package appdispatch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHandler struct{}

func (stubHandler) kind() ExecutionKind                    { return ExecutionKind("stub.kind") }
func (stubHandler) handle(context.Context, Envelope) error { return nil }

func TestAppDispatchUnits(t *testing.T) {
	fake := faker.New()

	t.Run("validates envelopes and payload encoding", func(t *testing.T) {
		payload := mustMarshalJSON(t, map[string]string{"id": fake.UUID().V4()})
		require.NoError(t, (Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("valid.kind"),
			Payload: payload,
		}).validate())

		require.EqualError(t, (Envelope{}).validate(), "unsupported envelope version: ")
		require.EqualError(
			t,
			(Envelope{Version: EnvelopeVersionV1}).validate(),
			"execution kind is required",
		)
		require.EqualError(
			t,
			(Envelope{Version: EnvelopeVersionV1, Kind: ExecutionKind("kind")}).validate(),
			"execution payload is required",
		)

		msg, err := envelopeMessage(t.Context(), Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("kind"),
			Payload: payload,
		})
		require.NoError(t, err)
		decoded, err := decodeEnvelope(msg.Payload)
		require.NoError(t, err)
		assert.Equal(t, ExecutionKind("kind"), decoded.Kind)

		_, err = envelopeMessage(t.Context(), Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("kind"),
			Payload: json.RawMessage(`{"broken"`),
		})
		require.ErrorContains(t, err, "marshal dispatch envelope")

		_, err = decodeEnvelope([]byte(`not-json`))
		require.ErrorContains(t, err, "decode dispatch envelope")

		_, err = decodeEnvelope([]byte(`{"version":"v1"}`))
		require.EqualError(t, err, "execution kind is required")
	})

	t.Run("normalizes config and detects drivers", func(t *testing.T) {
		normalized := (Config{}).normalize()
		assert.Equal(t, "signal-foundry-app-dispatch", normalized.ConsumerName)
		assert.Equal(t, 100*time.Millisecond, normalized.PollInterval)

		assert.Equal(t, TransportDriverSQLite, Config{DatabaseDSN: ":memory:"}.Driver())
		assert.Equal(t, TransportDriverSQLite, Config{DatabaseDSN: "file:/tmp/test.sqlite"}.Driver())
		assert.Equal(
			t,
			TransportDriverPostgres,
			Config{DatabaseDSN: "postgresql://user:pass@example.invalid/db"}.Driver(),
		)
		assert.Equal(t, TransportDriverPostgres, Config{DatabaseDSN: "postgres://%zz"}.Driver())
		assert.Equal(t, `"quoted"`, quoteIdentifier(`quoted`))
	})

	t.Run("covers registry and typed handler behavior", func(t *testing.T) {
		registry := NewHandlerRegistry()

		type handlerPayload struct {
			ID string `json:"id"`
		}

		require.NoError(t, registry.register(stubHandler{}))
		err := registry.register(nil)
		require.EqualError(t, err, "handler is required")

		th := typedHandler[handlerPayload]{spec: TypedHandlerSpec[handlerPayload]{
			Kind: ExecutionKind("typed.kind"),
			Run: func(_ context.Context, envelope Envelope, payload handlerPayload) error {
				assert.Equal(t, ExecutionKind("typed.kind"), envelope.Kind)
				assert.NotEmpty(t, payload.ID)
				return nil
			},
		}}
		require.Equal(t, ExecutionKind("typed.kind"), th.kind())
		require.NoError(t, th.handle(t.Context(), Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("typed.kind"),
			Payload: mustMarshalJSON(t, handlerPayload{ID: fake.UUID().V4()}),
		}))
		err = th.handle(t.Context(), Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("typed.kind"),
			Payload: []byte(`bad-json`),
		})
		require.ErrorContains(t, err, "decode envelope payload")
	})

	t.Run("builds schema and offset queries for sqlite and postgres", func(t *testing.T) {
		cfg := Config{TablePrefix: "signal_foundry_data_"}
		schema := sqliteSchema{config: cfg}
		offsets := sqliteOffsetsAdapter{config: cfg}

		queries, err := schema.SchemaInitializingQueries(
			wmsql.SchemaInitializingQueriesParams{Topic: DispatchTopicExecution},
		)
		require.NoError(t, err)
		require.Len(t, queries, 1)
		assert.Contains(t, queries[0].Query, cfg.MessagesTable())

		insertQuery, err := schema.InsertQuery(wmsql.InsertQueryParams{
			Topic: DispatchTopicExecution,
			Msgs:  []*wmmessage.Message{wmmessage.NewMessage(fake.UUID().V4(), []byte(`{"ok":true}`))},
		})
		require.NoError(t, err)
		assert.Contains(t, insertQuery.Query, cfg.MessagesTable())
		assert.Len(t, insertQuery.Args, 3)

		selectQuery, err := schema.SelectQuery(wmsql.SelectQueryParams{
			Topic:          DispatchTopicExecution,
			ConsumerGroup:  "consumer-" + fake.UUID().V4(),
			OffsetsAdapter: offsets,
		})
		require.NoError(t, err)
		assert.Contains(t, selectQuery.Query, cfg.MessagesTable())
		assert.Contains(t, selectQuery.Query, cfg.OffsetsTable())

		offsetQueries, err := offsets.SchemaInitializingQueries(
			wmsql.OffsetsSchemaInitializingQueriesParams{Topic: DispatchTopicExecution},
		)
		require.NoError(t, err)
		require.Len(t, offsetQueries, 1)
		assert.Contains(t, offsetQueries[0].Query, cfg.OffsetsTable())

		ackQuery, err := offsets.AckMessageQuery(wmsql.AckMessageQueryParams{
			Topic:         DispatchTopicExecution,
			ConsumerGroup: "consumer-" + fake.UUID().V4(),
			LastRow:       wmsql.Row{Offset: 7},
		})
		require.NoError(t, err)
		assert.Contains(t, ackQuery.Query, "ON CONFLICT")

		consumedQuery, err := offsets.ConsumedMessageQuery(wmsql.ConsumedMessageQueryParams{
			Topic:         DispatchTopicExecution,
			ConsumerGroup: "consumer-" + fake.UUID().V4(),
			Row:           wmsql.Row{Offset: 9},
		})
		require.NoError(t, err)
		assert.Contains(t, consumedQuery.Query, cfg.OffsetsTable())

		nextOffsetQuery, err := offsets.NextOffsetQuery(wmsql.NextOffsetQueryParams{
			Topic:         DispatchTopicExecution,
			ConsumerGroup: "consumer-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		assert.Contains(t, nextOffsetQuery.Query, cfg.OffsetsTable())

		beforeQueries, err := offsets.BeforeSubscribingQueries(wmsql.BeforeSubscribingQueriesParams{
			Topic:         DispatchTopicExecution,
			ConsumerGroup: "consumer-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		require.Len(t, beforeQueries, 1)
		assert.Equal(t, sql.LevelSerializable, schema.SubscribeIsolationLevel())
		assert.Equal(t, quoteIdentifier(cfg.MessagesTable()), schema.MessagesTable(DispatchTopicExecution))
		assert.Equal(t, quoteIdentifier(cfg.OffsetsTable()), offsets.MessagesOffsetsTable(DispatchTopicExecution))

		postgresCfg := Config{TablePrefix: "signal_foundry_data_"}
		assert.Contains(
			t,
			postgresSchema(postgresCfg).MessagesTable(DispatchTopicExecution),
			postgresCfg.MessagesTable(),
		)
		assert.Contains(
			t,
			postgresOffsets(postgresCfg).MessagesOffsetsTable(DispatchTopicExecution),
			postgresCfg.OffsetsTable(),
		)

		postgresQueries, err := buildPostgresMigrationQueries(postgresCfg)
		require.NoError(t, err)
		require.Len(t, postgresQueries, 3)
	})

	t.Run("covers close helpers and default insert args", func(t *testing.T) {
		assert.NoError(t, closeIfPresent(nil))
		assert.NoError(t, closeDB(nil))
		assert.NoError(t, (*Publisher)(nil).Close())
		assert.NoError(t, (*Consumer)(nil).Close())

		args, err := defaultInsertArgs([]*wmmessage.Message{
			wmmessage.NewMessage(fake.UUID().V4(), []byte(`{"ok":true}`)),
		})
		require.NoError(t, err)
		assert.Len(t, args, 3)
	})

	t.Run("covers migration helpers on sqlite and postgres", func(t *testing.T) {
		readOnlyPath := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		require.NoError(t, os.WriteFile(readOnlyPath, []byte{}, 0o600))
		readOnlyDB, err := sql.Open("sqlite", "file:"+readOnlyPath+"?mode=ro")
		require.NoError(t, err)
		defer func() { require.NoError(t, readOnlyDB.Close()) }()
		err = migrateSQLite(t.Context(), readOnlyDB, Config{TablePrefix: "signal_foundry_data_"})
		require.ErrorContains(t, err, "migrate sqlite app dispatch transport")

		postgresDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		queries, err := buildPostgresMigrationQueries(Config{TablePrefix: "signal_foundry_data_"})
		require.NoError(t, err)
		mock.ExpectBegin()
		for _, query := range queries {
			mock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
		}
		mock.ExpectCommit()
		mock.ExpectClose()
		require.NoError(t, migratePostgres(t.Context(), postgresDB, Config{TablePrefix: "signal_foundry_data_"}))
		require.NoError(t, postgresDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("covers consumer run branches with stubs", func(t *testing.T) {
		logger := slog.New(slog.DiscardHandler)
		consumer := &Consumer{
			subscriber: subscriberStub{err: errors.New("subscribe boom")},
			registry:   NewHandlerRegistry(),
			logger:     logger,
		}
		err := consumer.Run(t.Context())
		require.EqualError(t, err, "subscribe dispatch topic: subscribe boom")

		closedMessages := make(chan *wmmessage.Message)
		close(closedMessages)
		consumer = &Consumer{
			subscriber: subscriberStub{messages: closedMessages},
			registry:   NewHandlerRegistry(),
			logger:     logger,
		}
		require.NoError(t, consumer.Run(t.Context()))

		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		closedMessages = make(chan *wmmessage.Message)
		close(closedMessages)
		consumer = &Consumer{
			subscriber: subscriberStub{messages: closedMessages},
			registry:   NewHandlerRegistry(),
			logger:     logger,
		}
		require.ErrorIs(t, consumer.Run(canceledCtx), context.Canceled)

		handlerErr := errors.New("handler boom")
		registry := NewHandlerRegistry()
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[map[string]string]{
			Kind: ExecutionKind("handler.err"),
			Run:  func(context.Context, Envelope, map[string]string) error { return handlerErr },
		}))
		msg := wmmessage.NewMessage(fake.UUID().V4(), mustMarshalJSON(t, Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("handler.err"),
			Payload: mustMarshalJSON(t, map[string]string{"id": fake.UUID().V4()}),
		}))
		messageCh := make(chan *wmmessage.Message, 1)
		messageCh <- msg
		close(messageCh)
		consumer = &Consumer{
			subscriber: subscriberStub{messages: messageCh},
			registry:   registry,
			logger:     logger,
		}
		err = consumer.Run(t.Context())
		require.EqualError(t, err, handlerErr.Error())
	})

	t.Run("covers publisher and migrate error branches", func(t *testing.T) {
		cfg := Config{
			DatabaseDSN: filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			TablePrefix: "signal_foundry_data_",
		}
		require.NoError(t, AutoMigrate(t.Context(), cfg))

		publisher, err := NewPublisher(cfg, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		defer func() { require.NoError(t, publisher.Close()) }()

		err = publisher.Publish(t.Context(), Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("broken.publish"),
			Payload: json.RawMessage(`{"broken"`),
		})
		require.ErrorContains(t, err, "marshal dispatch envelope")

		db, err := openDatabase(cfg)
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		err = publisher.PublishInTx(t.Context(), tx, Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("broken.publish.in.tx"),
			Payload: json.RawMessage(`{"broken"`),
		})
		require.ErrorContains(t, err, "marshal dispatch envelope")
		require.NoError(t, tx.Rollback())

		cfgNoSchema := Config{
			DatabaseDSN: filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			TablePrefix: "signal_foundry_data_",
		}
		publisherNoSchema, err := NewPublisher(cfgNoSchema, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		defer func() { require.NoError(t, publisherNoSchema.Close()) }()
		dbNoSchema, err := openDatabase(cfgNoSchema)
		require.NoError(t, err)
		defer func() { require.NoError(t, dbNoSchema.Close()) }()
		txNoSchema, err := dbNoSchema.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		err = publisherNoSchema.PublishInTx(t.Context(), txNoSchema, Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("missing.schema"),
			Payload: mustMarshalJSON(t, map[string]string{"id": fake.UUID().V4()}),
		})
		require.ErrorContains(t, err, "publish dispatch envelope in tx")
		require.NoError(t, txNoSchema.Rollback())

		err = AutoMigrate(t.Context(), Config{})
		require.EqualError(t, err, "database dsn is required")

		postgresDB, err := openDatabase(Config{
			DatabaseDSN: "postgres://signal-foundry:secret@example.invalid:5432/signal_foundry?sslmode=disable",
		})
		require.NoError(t, err)
		require.NoError(t, postgresDB.Close())
	})

	t.Run("covers sqlite schema edge helpers", func(t *testing.T) {
		schema := sqliteSchema{config: Config{TablePrefix: "signal_foundry_data_"}}
		_, err := schema.SelectQuery(wmsql.SelectQueryParams{
			OffsetsAdapter: failingOffsetsAdapter{err: errors.New("offset boom")},
		})
		require.EqualError(t, err, "offset boom")

		_, err = schema.UnmarshalMessage(wmsql.UnmarshalMessageParams{Row: scannerStub{err: errors.New("scan boom")}})
		require.ErrorContains(t, err, "could not scan sqlite message row")

		_, err = schema.UnmarshalMessage(wmsql.UnmarshalMessageParams{
			Row: scannerStub{values: []any{int64(1), "uuid", []byte(`{"ok":true}`), []byte(`bad-json`)}},
		})
		require.ErrorContains(t, err, "could not unmarshal sqlite metadata")

		row, err := schema.UnmarshalMessage(wmsql.UnmarshalMessageParams{
			Row: scannerStub{values: []any{int64(2), "uuid", []byte(`{"ok":true}`), []byte(`{"k":"v"}`)}},
		})
		require.NoError(t, err)
		assert.EqualValues(t, 2, row.Offset)
	})

	t.Run("uses sql helpers for concrete handles", func(t *testing.T) {
		cfg := Config{DatabaseDSN: ":memory:"}
		db, err := openDatabase(cfg)
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()

		var busyTimeout int
		require.NoError(t, db.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&busyTimeout))
		require.Equal(t, sqliteBusyTimeoutMillis, busyTimeout)

		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		require.NotNil(t, asContextExecutor(db))
		require.NotNil(t, asContextExecutor(tx))
	})

	t.Run("uses WAL mode for file-backed sqlite handles", func(t *testing.T) {
		cfg := Config{DatabaseDSN: filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")}
		db, err := openDatabase(cfg)
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()

		var journalMode string
		require.NoError(t, db.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journalMode))
		require.Equal(t, "wal", journalMode)
	})

	t.Run("joins close errors from publisher and consumer wrappers", func(t *testing.T) {
		errBoom := errors.New("boom")
		closer := &closeErrStub{err: errBoom}
		assert.ErrorIs(t, closeIfPresent(closer), errBoom)
	})
}

type closeErrStub struct {
	err error
}

func (s *closeErrStub) Close() error {
	return s.err
}

type subscriberStub struct {
	messages <-chan *wmmessage.Message
	err      error
}

func (s subscriberStub) Subscribe(context.Context, string) (<-chan *wmmessage.Message, error) {
	return s.messages, s.err
}

func (subscriberStub) Close() error { return nil }

type scannerStub struct {
	values []any
	err    error
}

func (s scannerStub) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	for idx := range dest {
		switch ptr := dest[idx].(type) {
		case *int64:
			*ptr = s.values[idx].(int64)
		case *[]byte:
			switch value := s.values[idx].(type) {
			case []byte:
				*ptr = value
			case string:
				*ptr = []byte(value)
			}
		case *string:
			*ptr = s.values[idx].(string)
		}
	}
	return nil
}

type failingOffsetsAdapter struct {
	err error
}

func (a failingOffsetsAdapter) AckMessageQuery(wmsql.AckMessageQueryParams) (wmsql.Query, error) {
	return wmsql.Query{}, nil
}
func (a failingOffsetsAdapter) ConsumedMessageQuery(wmsql.ConsumedMessageQueryParams) (wmsql.Query, error) {
	return wmsql.Query{}, nil
}
func (a failingOffsetsAdapter) NextOffsetQuery(wmsql.NextOffsetQueryParams) (wmsql.Query, error) {
	return wmsql.Query{}, a.err
}
func (a failingOffsetsAdapter) SchemaInitializingQueries(
	wmsql.OffsetsSchemaInitializingQueriesParams,
) ([]wmsql.Query, error) {
	return nil, nil
}
func (a failingOffsetsAdapter) BeforeSubscribingQueries(wmsql.BeforeSubscribingQueriesParams) ([]wmsql.Query, error) {
	return nil, nil
}
