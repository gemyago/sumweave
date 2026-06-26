package appdispatch

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatermillTransport(t *testing.T) {
	fake := faker.New()

	type examplePayload struct {
		RunID string `json:"runId"`
	}

	makeConfig := func(t *testing.T) Config {
		t.Helper()
		return Config{
			DatabaseDSN:  filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			TablePrefix:  "signal_foundry_data_",
			ConsumerName: "consumer-" + fake.UUID().V4(),
		}
	}

	openDB := func(t *testing.T, dsn string) *sql.DB {
		t.Helper()
		db, err := sql.Open("sqlite", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return db
	}

	requireNoTable := func(t *testing.T, db *sql.DB, tableName string) {
		t.Helper()
		var found string
		err := db.QueryRowContext(
			t.Context(),
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			tableName,
		).Scan(&found)
		require.ErrorIs(t, err, sql.ErrNoRows)
	}

	requireTable := func(t *testing.T, db *sql.DB, tableName string) {
		t.Helper()
		var found string
		err := db.QueryRowContext(
			t.Context(),
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			tableName,
		).Scan(&found)
		require.NoError(t, err)
		require.Equal(t, tableName, found)
	}

	t.Run("publishes and consumes the durable execution topic through the app abstraction", func(t *testing.T) {
		cfg := makeConfig(t)
		require.NoError(t, AutoMigrate(t.Context(), cfg))

		publisher, err := NewPublisher(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })

		registry := NewHandlerRegistry()
		var (
			mu       sync.Mutex
			received []examplePayload
		)
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[examplePayload]{
			Kind: ExecutionKind("example.dispatch"),
			Run: func(_ context.Context, envelope Envelope, payload examplePayload) error {
				mu.Lock()
				defer mu.Unlock()
				received = append(received, payload)
				assert.Equal(t, EnvelopeVersionV1, envelope.Version)
				assert.Equal(t, DispatchTopicExecution, envelope.Topic())
				return nil
			},
		}))

		consumer, err := NewConsumer(cfg, registry)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, consumer.Close()) })

		envelope := Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("example.dispatch"),
			Payload: mustMarshalJSON(t, examplePayload{RunID: "run-" + fake.UUID().V4()}),
		}
		require.NoError(t, publisher.Publish(t.Context(), envelope))

		consumeCtx, cancel := context.WithCancel(t.Context())
		defer cancel()
		consumeErr := make(chan error, 1)
		go func() {
			consumeErr <- consumer.Run(consumeCtx)
		}()

		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(received) == 1
		}, 5*time.Second, 50*time.Millisecond)

		cancel()
		require.ErrorIs(t, <-consumeErr, context.Canceled)

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, received, 1)
		assert.NotEmpty(t, received[0].RunID)
	})

	t.Run("uses the repo sqlite-first database shape and supports transactional publish", func(t *testing.T) {
		cfg := makeConfig(t)
		require.Equal(t, TransportDriverSQLite, cfg.Driver())
		require.Equal(t, "signal_foundry_data_app_dispatch_messages", cfg.MessagesTable())
		require.Equal(t, "signal_foundry_data_app_dispatch_offsets", cfg.OffsetsTable())
		require.NoError(t, AutoMigrate(t.Context(), cfg))

		db := openDB(t, cfg.DatabaseDSN)
		_, err := db.ExecContext(t.Context(), `CREATE TABLE app_owned_records (id TEXT PRIMARY KEY)`)
		require.NoError(t, err)

		publisher, err := NewPublisher(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })

		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(
			t.Context(),
			`INSERT INTO app_owned_records (id) VALUES (?)`,
			"record-"+fake.UUID().V4(),
		)
		require.NoError(t, err)
		require.NoError(t, publisher.PublishInTx(t.Context(), tx, Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("example.dispatch"),
			Payload: mustMarshalJSON(t, examplePayload{RunID: "run-" + fake.UUID().V4()}),
		}))
		require.NoError(t, tx.Commit())

		var recordCount int
		err = db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM app_owned_records`).Scan(&recordCount)
		require.NoError(t, err)
		assert.Equal(t, 1, recordCount)

		var messageCount int
		err = db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM signal_foundry_data_app_dispatch_messages`,
		).Scan(&messageCount)
		require.NoError(t, err)
		assert.Equal(t, 1, messageCount)
	})

	t.Run("does not leak watermill imports outside the app module", func(t *testing.T) {
		for _, root := range []string{
			filepath.Clean(filepath.Join("..", "..", "..", "..", "finance")),
			filepath.Clean(filepath.Join("..", "..", "..", "..", "runtime")),
		} {
			entries, err := os.ReadDir(root)
			require.NoError(t, err)
			_ = entries
			require.NoError(t, assertNoWatermillImports(root))
		}
	})

	t.Run("prepares transport schema only when explicitly migrated", func(t *testing.T) {
		cfg := makeConfig(t)
		db := openDB(t, cfg.DatabaseDSN)
		requireNoTable(t, db, cfg.MessagesTable())
		requireNoTable(t, db, cfg.OffsetsTable())

		publisher, err := NewPublisher(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		require.Error(t, publisher.Publish(t.Context(), Envelope{
			Version: EnvelopeVersionV1,
			Kind:    ExecutionKind("example.dispatch"),
			Payload: mustMarshalJSON(t, examplePayload{RunID: "run-" + fake.UUID().V4()}),
		}))
		requireNoTable(t, db, cfg.MessagesTable())
		requireNoTable(t, db, cfg.OffsetsTable())

		require.NoError(t, AutoMigrate(t.Context(), cfg))
		requireTable(t, db, cfg.MessagesTable())
		requireTable(t, db, cfg.OffsetsTable())
	})

	t.Run("keeps a postgres-compatible transport shape available", func(t *testing.T) {
		cfg := Config{
			DatabaseDSN:  "postgres://signal-foundry:secret@example.invalid:5432/signal_foundry?sslmode=disable",
			TablePrefix:  "signal_foundry_data_",
			ConsumerName: "consumer-" + fake.UUID().V4(),
		}

		require.Equal(t, TransportDriverPostgres, cfg.Driver())
		require.Equal(t, "signal_foundry_data_app_dispatch_messages", cfg.MessagesTable())
		require.Equal(t, "signal_foundry_data_app_dispatch_offsets", cfg.OffsetsTable())
	})

	t.Run("covers validation and error edges for the app-owned seam", func(t *testing.T) {
		_, err := EncodePayload(func() {})
		require.Error(t, err)

		cfg := Config{}
		_, err = NewPublisher(cfg)
		require.EqualError(t, err, "database dsn is required")

		_, err = NewConsumer(makeConfig(t), nil)
		require.EqualError(t, err, "handler registry is required")

		registry := NewHandlerRegistry()
		err = RegisterTypedHandler(registry, TypedHandlerSpec[examplePayload]{})
		require.EqualError(t, err, "handler run func is required")

		err = RegisterTypedHandler[examplePayload](nil, TypedHandlerSpec[examplePayload]{
			Kind: ExecutionKind("nil.registry"),
			Run:  func(context.Context, Envelope, examplePayload) error { return nil },
		})
		require.EqualError(t, err, "handler registry is required")

		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[examplePayload]{
			Kind: ExecutionKind("duplicate.kind"),
			Run:  func(context.Context, Envelope, examplePayload) error { return nil },
		}))
		err = RegisterTypedHandler(registry, TypedHandlerSpec[examplePayload]{
			Kind: ExecutionKind("duplicate.kind"),
			Run:  func(context.Context, Envelope, examplePayload) error { return nil },
		})
		require.EqualError(t, err, "handler already registered: duplicate.kind")

		err = registry.Handle(t.Context(), Envelope{Kind: ExecutionKind("missing.kind")})
		require.EqualError(t, err, "handler not registered: missing.kind")

		var nilRegistry *HandlerRegistry
		err = nilRegistry.Handle(t.Context(), Envelope{Kind: ExecutionKind("missing.kind")})
		require.EqualError(t, err, "handler registry is required")
	})

	t.Run("surfaces publish and consume failures without schema side effects", func(t *testing.T) {
		cfg := makeConfig(t)
		publisher, err := NewPublisher(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })

		err = publisher.Publish(t.Context(), Envelope{})
		require.EqualError(t, err, "unsupported envelope version: ")

		err = publisher.PublishInTx(t.Context(), nil, Envelope{Version: EnvelopeVersionV1})
		require.EqualError(t, err, "publish transaction is required")

		db := openDB(t, cfg.DatabaseDSN)
		require.NoError(t, AutoMigrate(t.Context(), cfg))
		_, err = db.ExecContext(
			t.Context(),
			`INSERT INTO signal_foundry_data_app_dispatch_messages (uuid, created_at, payload, metadata) VALUES (?, CURRENT_TIMESTAMP, ?, '{}')`,
			"msg-"+fake.UUID().V4(),
			[]byte(`not-json`),
		)
		require.NoError(t, err)

		consumer, err := NewConsumer(cfg, NewHandlerRegistry())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, consumer.Close()) })

		consumeCtx, cancel := context.WithCancel(t.Context())
		defer cancel()
		err = consumer.Run(consumeCtx)
		require.ErrorContains(t, err, "decode dispatch envelope")
	})

	t.Run("handles postgres wiring and migration failures explicitly", func(t *testing.T) {
		cfg := Config{
			DatabaseDSN:  "postgres://signal-foundry:secret@example.invalid:5432/signal_foundry?sslmode=disable",
			TablePrefix:  "signal_foundry_data_",
			ConsumerName: "consumer-" + fake.UUID().V4(),
		}

		publisher, err := NewPublisher(cfg)
		require.NoError(t, err)
		require.NoError(t, publisher.Close())

		consumer, err := NewConsumer(cfg, NewHandlerRegistry())
		require.NoError(t, err)
		require.NoError(t, consumer.Close())

		err = AutoMigrate(t.Context(), cfg)
		require.ErrorContains(t, err, "begin postgres transport migration")
	})
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := EncodePayload(value)
	require.NoError(t, err)
	return payload
}
