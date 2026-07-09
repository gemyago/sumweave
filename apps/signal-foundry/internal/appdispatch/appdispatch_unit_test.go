package appdispatch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHandler struct{}

func (stubHandler) kind() ExecutionKind                    { return ExecutionKind("stub.kind") }
func (stubHandler) handle(context.Context, Envelope) error { return nil }

func TestAppDispatchUnits(t *testing.T) {
	fake := faker.New()
	makeSQLiteMemoryDSN := func(prefix string) string {
		return fmt.Sprintf("file:%s?mode=memory&cache=shared", prefix+"-"+fake.UUID().V4())
	}

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

	t.Run("builds sqlite table generators and migration queries for sqlite and postgres", func(t *testing.T) {
		cfg := Config{TablePrefix: "signal_foundry_data_"}
		generators := sqliteTableNameGenerators(cfg)
		assert.Equal(t, cfg.MessagesTable(), generators.Topic(DispatchTopicExecution))
		assert.Equal(t, cfg.OffsetsTable(), generators.Offsets(DispatchTopicExecution))

		sqliteQueries := buildSQLiteMigrationQueries(cfg)
		require.Len(t, sqliteQueries, 2)
		assert.Contains(t, sqliteQueries[0], cfg.MessagesTable())
		assert.Contains(t, sqliteQueries[1], cfg.OffsetsTable())
		assert.Contains(t, sqliteQueries[1], "locked_until")

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

	t.Run("covers close helpers", func(t *testing.T) {
		assert.NoError(t, closeIfPresent(nil))
		assert.NoError(t, closeDB(nil))
		assert.NoError(t, (*Publisher)(nil).Close())
		assert.NoError(t, (*Consumer)(nil).Close())
	})

	t.Run("covers explicit migrator behavior on sqlite and postgres", func(t *testing.T) {
		hasSQLiteColumn := func(t *testing.T, db *sql.DB, tableName string, columnName string) bool {
			t.Helper()
			rows, err := db.QueryContext(t.Context(), `PRAGMA table_info(`+quoteIdentifier(tableName)+`)`)
			require.NoError(t, err)
			defer func() { require.NoError(t, rows.Close()) }()

			for rows.Next() {
				var (
					cid        int
					name       string
					columnType string
					notNull    int
					defaultVal sql.NullString
					pk         int
				)
				require.NoError(t, rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk))
				if name == columnName {
					return true
				}
			}
			require.NoError(t, rows.Err())
			return false
		}

		t.Run("constructor validates required db", func(t *testing.T) {
			_, err := NewMigrator(Config{}, nil)
			require.EqualError(t, err, "sql database is required")
		})

		t.Run("sqlite migration creates schema and stays idempotent", func(t *testing.T) {
			cfg := Config{DatabaseDSN: makeSQLiteMemoryDSN("sqlite-migrator"), TablePrefix: "signal_foundry_data_"}
			db, err := sqlconn.Open(cfg.DatabaseDSN)
			require.NoError(t, err)
			defer func() { require.NoError(t, db.Close()) }()

			migrator, err := NewMigrator(cfg, db)
			require.NoError(t, err)
			require.NoError(t, migrator.Migrate(t.Context()))
			require.NoError(t, migrator.Migrate(t.Context()))
			require.True(t, hasSQLiteColumn(t, db, cfg.OffsetsTable(), "locked_until"))
		})

		t.Run("sqlite migration surfaces write failures", func(t *testing.T) {
			// File-backed on purpose: read-only SQLite opens need a real database file.
			readOnlyPath := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
			require.NoError(t, os.WriteFile(readOnlyPath, []byte{}, 0o600))
			readOnlyDB, err := sqlconn.Open("file:" + readOnlyPath + "?mode=ro")
			require.NoError(t, err)
			defer func() { require.NoError(t, readOnlyDB.Close()) }()

			migrator, err := NewMigrator(Config{TablePrefix: "signal_foundry_data_"}, readOnlyDB)
			require.NoError(t, err)
			err = migrator.Migrate(t.Context())
			require.ErrorContains(t, err, "migrate sqlite app dispatch transport")
		})

		t.Run("sqlite migration helper surfaces constructor and inspection failures", func(t *testing.T) {
			err := migrateSQLite(t.Context(), nil, Config{})
			require.EqualError(t, err, "sql database is required")

			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			mock.ExpectQuery(regexp.QuoteMeta(`PRAGMA table_info("signal_foundry_data_app_dispatch_offsets")`)).
				WillReturnError(errors.New("inspect boom"))
			require.ErrorContains(
				t,
				migrateSQLite(t.Context(), mockDB, Config{TablePrefix: "signal_foundry_data_"}),
				"inspect sqlite app dispatch offsets schema",
			)
			mock.ExpectClose()
			require.NoError(t, mockDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})

		t.Run("sqlite migration helper surfaces legacy table scan and reset failures", func(t *testing.T) {
			cfg := Config{TablePrefix: "signal_foundry_data_"}

			scanDB, scanMock, err := sqlmock.New()
			require.NoError(t, err)
			scanMock.ExpectQuery(regexp.QuoteMeta(`PRAGMA table_info("signal_foundry_data_app_dispatch_offsets")`)).
				WillReturnRows(sqlmock.NewRows([]string{"cid", "name"}).AddRow(1, "offset_acked"))
			require.ErrorContains(t, migrateSQLite(t.Context(), scanDB, cfg), "scan sqlite app dispatch offsets schema")
			scanMock.ExpectClose()
			require.NoError(t, scanDB.Close())
			require.NoError(t, scanMock.ExpectationsWereMet())

			resetDB, resetMock, err := sqlmock.New()
			require.NoError(t, err)
			resetMock.ExpectQuery(regexp.QuoteMeta(`PRAGMA table_info("signal_foundry_data_app_dispatch_offsets")`)).
				WillReturnRows(sqlmock.NewRows([]string{"cid", "name", "type", "notnull", "dflt_value", "pk"}).
					AddRow(0, "consumer_group", "TEXT", 1, nil, 1))
			resetMock.ExpectExec(regexp.QuoteMeta(`DROP TABLE "signal_foundry_data_app_dispatch_offsets"`)).
				WillReturnError(errors.New("drop boom"))
			require.ErrorContains(
				t,
				migrateSQLite(t.Context(), resetDB, cfg),
				"reset sqlite app dispatch offsets schema",
			)
			resetMock.ExpectClose()
			require.NoError(t, resetDB.Close())
			require.NoError(t, resetMock.ExpectationsWereMet())
		})

		t.Run("legacy sqlite offsets repair runs only in the migrator path", func(t *testing.T) {
			cfg := Config{DatabaseDSN: makeSQLiteMemoryDSN("legacy-offsets"), TablePrefix: "signal_foundry_data_"}
			legacyDB, err := sqlconn.Open(cfg.DatabaseDSN)
			require.NoError(t, err)
			defer func() { require.NoError(t, legacyDB.Close()) }()

			_, err = legacyDB.ExecContext(
				t.Context(),
				`CREATE TABLE `+quoteIdentifier(cfg.OffsetsTable())+` (`+
					`consumer_group TEXT NOT NULL PRIMARY KEY, `+
					`offset_acked INTEGER NOT NULL, `+
					`offset_consumed INTEGER NOT NULL)`,
			)
			require.NoError(t, err)

			publisher, err := NewPublisher(cfg, legacyDB, slog.New(slog.DiscardHandler))
			require.NoError(t, err)
			defer func() { require.NoError(t, publisher.Close()) }()
			consumer, err := NewConsumer(cfg, legacyDB, NewHandlerRegistry(), slog.New(slog.DiscardHandler))
			require.NoError(t, err)
			defer func() { require.NoError(t, consumer.Close()) }()
			require.False(t, hasSQLiteColumn(t, legacyDB, cfg.OffsetsTable(), "locked_until"))

			migrator, err := NewMigrator(cfg, legacyDB)
			require.NoError(t, err)
			require.NoError(t, migrator.Migrate(t.Context()))
			require.True(t, hasSQLiteColumn(t, legacyDB, cfg.OffsetsTable(), "locked_until"))
		})

		t.Run("postgres migration uses explicit schema queries", func(t *testing.T) {
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

			migrator, err := NewMigrator(Config{TablePrefix: "signal_foundry_data_"}, postgresDB)
			require.NoError(t, err)
			require.NoError(t, migrator.migratePostgres(t.Context()))
			require.NoError(t, postgresDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})

		t.Run("postgres migration helper surfaces constructor, exec, and commit failures", func(t *testing.T) {
			err := migratePostgres(t.Context(), nil, Config{})
			require.EqualError(t, err, "sql database is required")

			cfg := Config{
				DatabaseDSN: "postgres://signal-foundry:secret@example.invalid:5432/signal_foundry?sslmode=disable",
				TablePrefix: "signal_foundry_data_",
			}
			queries, err := buildPostgresMigrationQueries(cfg)
			require.NoError(t, err)

			execDB, execMock, err := sqlmock.New()
			require.NoError(t, err)
			execMock.ExpectBegin()
			execMock.ExpectExec(regexp.QuoteMeta(queries[0].Query)).WillReturnError(errors.New("exec boom"))
			execMock.ExpectRollback()
			require.ErrorContains(
				t,
				migratePostgres(t.Context(), execDB, cfg),
				"exec postgres transport migration query",
			)
			execMock.ExpectClose()
			require.NoError(t, execDB.Close())
			require.NoError(t, execMock.ExpectationsWereMet())

			commitDB, commitMock, err := sqlmock.New()
			require.NoError(t, err)
			commitMock.ExpectBegin()
			for _, query := range queries {
				commitMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			commitMock.ExpectCommit().WillReturnError(errors.New("commit boom"))
			require.ErrorContains(
				t,
				migratePostgres(t.Context(), commitDB, cfg),
				"commit postgres transport migration",
			)
			commitMock.ExpectClose()
			require.NoError(t, commitDB.Close())
			require.NoError(t, commitMock.ExpectationsWereMet())
		})
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

		handled := false
		registry = NewHandlerRegistry()
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[map[string]string]{
			Kind: ExecutionKind("handler.canceled.ctx"),
			Run: func(ctx context.Context, _ Envelope, _ map[string]string) error {
				handled = true
				require.NoError(t, ctx.Err())
				return nil
			},
		}))
		canceledMsgCtx, cancelMsgCtx := context.WithCancel(t.Context())
		cancelMsgCtx()
		msg = wmmessage.NewMessageWithContext(canceledMsgCtx, fake.UUID().V4(), mustMarshalJSON(t, Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("handler.canceled.ctx"),
			Payload: mustMarshalJSON(t, map[string]string{"id": fake.UUID().V4()}),
		}))
		messageCh = make(chan *wmmessage.Message, 1)
		messageCh <- msg
		close(messageCh)
		consumer = &Consumer{
			subscriber: subscriberStub{messages: messageCh},
			registry:   registry,
			logger:     logger,
		}
		require.NoError(t, consumer.Run(t.Context()))
		require.True(t, handled)
	})

	t.Run("covers publisher and migrate error branches", func(t *testing.T) {
		cfg := Config{
			DatabaseDSN: makeSQLiteMemoryDSN("publish-errors"),
			TablePrefix: "signal_foundry_data_",
		}
		db, err := sqlconn.Open(cfg.DatabaseDSN)
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()
		require.NoError(t, AutoMigrate(t.Context(), cfg, db))

		publisher, err := NewPublisher(cfg, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		defer func() { require.NoError(t, publisher.Close()) }()

		err = publisher.Publish(t.Context(), Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("broken.publish"),
			Payload: json.RawMessage(`{"broken"`),
		})
		require.ErrorContains(t, err, "marshal dispatch envelope")

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
			DatabaseDSN: makeSQLiteMemoryDSN("publish-no-schema"),
			TablePrefix: "signal_foundry_data_",
		}
		dbNoSchema, err := sqlconn.Open(cfgNoSchema.DatabaseDSN)
		require.NoError(t, err)
		defer func() { require.NoError(t, dbNoSchema.Close()) }()
		publisherNoSchema, err := NewPublisher(cfgNoSchema, dbNoSchema, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		defer func() { require.NoError(t, publisherNoSchema.Close()) }()
		txNoSchema, err := dbNoSchema.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		err = publisherNoSchema.PublishInTx(t.Context(), txNoSchema, Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("missing.schema"),
			Payload: mustMarshalJSON(t, map[string]string{"id": fake.UUID().V4()}),
		})
		require.ErrorContains(t, err, "publish dispatch envelope in tx")
		require.NoError(t, txNoSchema.Rollback())

		err = AutoMigrate(t.Context(), Config{}, nil)
		require.EqualError(t, err, "sql database is required")

		postgresDB, err := sqlconn.Open(
			"postgres://signal-foundry:secret@example.invalid:5432/signal_foundry?sslmode=disable",
		)
		require.NoError(t, err)
		require.NoError(t, postgresDB.Close())
	})

	t.Run("uses sqlite and postgres helpers for concrete handles", func(t *testing.T) {
		cfg := Config{DatabaseDSN: ":memory:"}
		db, err := sqlconn.Open(cfg.DatabaseDSN)
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()

		var busyTimeout int
		require.NoError(t, db.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&busyTimeout))
		require.Equal(t, sqlconn.SQLiteBusyTimeoutMillis, busyTimeout)

		require.NoError(t, AutoMigrate(t.Context(), cfg, db))
		publisher, err := newMessagePublisher(cfg, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		require.NoError(t, publisher.Close())

		subscriber, err := newMessageSubscriber(Config{
			DatabaseDSN:  cfg.DatabaseDSN,
			ConsumerName: "consumer-" + fake.UUID().V4(),
			PollInterval: 10 * time.Millisecond,
		}, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		require.NoError(t, subscriber.Close())

		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		require.NotNil(t, asContextExecutor(db))
		require.NotNil(t, asContextExecutor(tx))

		txPublisher, err := newMessagePublisher(cfg, tx, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		require.NoError(t, txPublisher.Close())
	})

	t.Run("uses WAL mode for file-backed sqlite handles", func(t *testing.T) {
		// File-backed on purpose: WAL mode only applies to on-disk SQLite databases.
		cfg := Config{DatabaseDSN: filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")}
		db, err := sqlconn.Open(cfg.DatabaseDSN)
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
