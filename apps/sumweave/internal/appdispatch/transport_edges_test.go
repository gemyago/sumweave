package appdispatch

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ThreeDotsLabs/watermill"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const transportTestTopic = "topic"

func TestPostgresTransportEdges(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	fake := faker.New()

	t.Run("migrates PostgreSQL transport through a transaction", func(t *testing.T) {
		config := Config{TablePrefix: "migration_"}
		queries, err := buildPostgresMigrationQueries(config)
		require.NoError(t, err)

		makeMigrator := func(t *testing.T) (*Migrator, sqlmock.Sqlmock) {
			t.Helper()
			db, databaseMock, databaseErr := sqlmock.New()
			require.NoError(t, databaseErr)
			t.Cleanup(func() { _ = db.Close() })
			migrator, migratorErr := NewMigrator(config, db)
			require.NoError(t, migratorErr)
			return migrator, databaseMock
		}
		expectSuccess := func(databaseMock sqlmock.Sqlmock) {
			databaseMock.ExpectBegin()
			for _, query := range queries {
				databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			databaseMock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
			databaseMock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 0))
			databaseMock.ExpectExec("CREATE UNIQUE INDEX").WillReturnResult(sqlmock.NewResult(0, 0))
		}

		_, err = NewMigrator(config, nil)
		require.EqualError(t, err, "sql database is required")

		migrator, databaseMock := makeMigrator(t)
		expectSuccess(databaseMock)
		databaseMock.ExpectCommit()
		require.NoError(t, migrator.Migrate(t.Context()))
		require.NoError(t, databaseMock.ExpectationsWereMet())

		migrator, databaseMock = makeMigrator(t)
		beginErr := errors.New(fake.UUID().V4())
		databaseMock.ExpectBegin().WillReturnError(beginErr)
		require.ErrorIs(t, migrator.Migrate(t.Context()), beginErr)

		migrator, databaseMock = makeMigrator(t)
		databaseMock.ExpectBegin()
		databaseMock.ExpectExec(regexp.QuoteMeta(queries[0].Query)).WillReturnError(beginErr)
		databaseMock.ExpectRollback()
		require.ErrorIs(t, migrator.Migrate(t.Context()), beginErr)

		migrator, databaseMock = makeMigrator(t)
		databaseMock.ExpectBegin()
		for _, query := range queries {
			databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
		}
		databaseMock.ExpectExec("ALTER TABLE").WillReturnError(beginErr)
		databaseMock.ExpectRollback()
		require.ErrorIs(t, migrator.Migrate(t.Context()), beginErr)

		migrator, databaseMock = makeMigrator(t)
		databaseMock.ExpectBegin()
		for _, query := range queries {
			databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
		}
		databaseMock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
		databaseMock.ExpectExec("DELETE FROM").WillReturnError(beginErr)
		databaseMock.ExpectRollback()
		require.ErrorIs(t, migrator.Migrate(t.Context()), beginErr)

		migrator, databaseMock = makeMigrator(t)
		expectSuccess(databaseMock)
		databaseMock.ExpectCommit().WillReturnError(beginErr)
		require.ErrorIs(t, migrator.Migrate(t.Context()), beginErr)

		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		databaseMock.ExpectBegin()
		for _, query := range queries {
			databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
		}
		databaseMock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
		databaseMock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 0))
		databaseMock.ExpectExec("CREATE UNIQUE INDEX").WillReturnResult(sqlmock.NewResult(0, 0))
		databaseMock.ExpectCommit()
		require.NoError(t, AutoMigrate(t.Context(), config, db))
	})

	t.Run("adapts PostgreSQL rows and validates transport constructors", func(t *testing.T) {
		config := Config{TablePrefix: "edge_"}
		schema := postgresSchema(config)
		assert.Equal(t, "BYTEA", schema.GeneratePayloadType(transportTestTopic))
		_, err := schema.SelectQuery(wmsql.SelectQueryParams{})
		require.EqualError(t, err, "single-table postgres offsets adapter is required")
		selected, err := schema.SelectQuery(wmsql.SelectQueryParams{
			Topic: transportTestTopic, ConsumerGroup: "group", OffsetsAdapter: postgresOffsets(config),
		})
		require.NoError(t, err)
		assert.Equal(t, []any{transportTestTopic, "group"}, selected.Args)

		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		databaseMock.ExpectQuery("SELECT valid").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata", "transaction_id"}).
				AddRow(7, "id", []byte("payload"), []byte(`{"traceId":"trace-id"}`), 9),
		)
		decoded, err := schema.UnmarshalMessage(wmsql.UnmarshalMessageParams{
			Row: db.QueryRowContext(t.Context(), "SELECT valid"),
		})
		require.NoError(t, err)
		assert.Equal(t, "trace-id", decoded.Msg.Metadata.Get("traceId"))
		databaseMock.ExpectQuery("SELECT invalid").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata", "transaction_id"}).
				AddRow("invalid", "id", []byte("payload"), []byte(`{}`), 9),
		)
		_, err = schema.UnmarshalMessage(wmsql.UnmarshalMessageParams{
			Row: db.QueryRowContext(t.Context(), "SELECT invalid"),
		})
		require.ErrorContains(t, err, "scan postgres message row")
		require.NoError(t, databaseMock.ExpectationsWereMet())

		_, err = NewPublisher(Config{}, nil, logger)
		require.EqualError(t, err, "sql database is required")
		_, err = NewPublisher(Config{}, &sql.DB{}, nil)
		require.EqualError(t, err, "logger is required")
		_, err = newMessageSubscriber(Config{}, &sql.DB{}, "", logger)
		require.EqualError(t, err, "consumer group is required")
		assert.Equal(t, "$1", (&Publisher{}).publicationPlaceholder(1))
		assert.True(t, isDuplicateMessageIDError(errors.New("duplicate key value violates unique constraint")))
		assert.False(t, isDuplicateMessageIDError(errors.New("temporary transport error")))

		db, databaseMock, err = sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		publisher, err := NewPublisher(Config{}, db, logger)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		publishErr := errors.New(fake.UUID().V4())
		databaseMock.ExpectExec("INSERT INTO").WillReturnError(publishErr)
		require.ErrorIs(
			t,
			publisher.Publish(t.Context(), NewMessage(transportTestTopic, []byte(fake.UUID().V4()))),
			publishErr,
		)
		databaseMock.ExpectBegin().WillReturnError(publishErr)
		_, err = publisher.PublishRequest(t.Context(), PublicationRequest{
			Topic: transportTestTopic, Payload: []byte(fake.UUID().V4()), IdempotencyKey: fake.UUID().V4(),
		})
		require.ErrorIs(t, err, publishErr)
		require.EqualError(t, publisher.PublishInTx(t.Context(), nil, Message{}), "publish transaction is required")
		_, err = publisher.PublishRequestInTx(t.Context(), nil, PublicationRequest{})
		require.EqualError(t, err, "publish transaction is required")
		_, err = publisher.PublishRequest(t.Context(), PublicationRequest{})
		require.EqualError(t, err, "publication topic is required")

		databaseMock.ExpectBegin()
		tx, transactionErr := db.BeginTx(t.Context(), nil)
		require.NoError(t, transactionErr)
		databaseMock.ExpectExec("INSERT INTO").WillReturnError(publishErr)
		require.ErrorIs(
			t,
			publisher.PublishInTx(t.Context(), tx, NewMessage(transportTestTopic, []byte(fake.UUID().V4()))),
			publishErr,
		)
		databaseMock.ExpectRollback()
		require.NoError(t, tx.Rollback())

		databaseMock.ExpectBegin()
		tx, transactionErr = db.BeginTx(t.Context(), nil)
		require.NoError(t, transactionErr)
		request := PublicationRequest{
			Topic: transportTestTopic, Payload: []byte(fake.UUID().V4()), IdempotencyKey: fake.UUID().V4(),
		}
		databaseMock.ExpectExec("INSERT INTO.*app_dispatch_publications").WillReturnError(publishErr)
		_, err = publisher.PublishRequestInTx(t.Context(), tx, request)
		require.ErrorIs(t, err, publishErr)
		databaseMock.ExpectRollback()
		require.NoError(t, tx.Rollback())

		databaseMock.ExpectBegin()
		tx, transactionErr = db.BeginTx(t.Context(), nil)
		require.NoError(t, transactionErr)
		databaseMock.ExpectQuery("SELECT message_id").WithArgs(request.IdempotencyKey).WillReturnError(publishErr)
		_, err = publisher.existingPublication(t.Context(), tx, request)
		require.ErrorIs(t, err, publishErr)
		databaseMock.ExpectRollback()
		require.NoError(t, tx.Rollback())
	})

	t.Run("validates router construction and retry lifecycle", func(t *testing.T) {
		_, err := NewHandler("", func(context.Context, Message) error { return nil })
		require.EqualError(t, err, "handler topic is required")
		_, err = NewHandler(transportTestTopic, nil)
		require.EqualError(t, err, "handler run func is required")
		_, err = NewRouterFactory(Config{}, nil, &Publisher{}, logger)
		require.EqualError(t, err, "sql database is required")
		_, err = NewRouterFactory(Config{}, &sql.DB{}, nil, logger)
		require.EqualError(t, err, "publisher is required")
		_, err = NewRouterFactory(Config{}, &sql.DB{}, &Publisher{}, nil)
		require.EqualError(t, err, "logger is required")
		factory := &RouterFactory{config: Config{}, db: &sql.DB{}, publisher: &Publisher{}, logger: logger}
		_, err = factory.NewRouter("")
		require.EqualError(t, err, "consumer group is required")
		router := &Router{retryLifecycle: &retryLifecycleState{}}
		require.NoError(t, router.SetRetryLifecycle(RetryLifecycle{}))
		router.started = true
		require.EqualError(
			t,
			router.SetRetryLifecycle(RetryLifecycle{}),
			"cannot set retry lifecycle after router starts",
		)
		require.EqualError(t, router.Handle(Handler{}), "valid handler is required")

		state := &retryLifecycleState{}
		retried := make(chan string, 1)
		exhausted := make(chan string, 1)
		state.set(RetryLifecycle{
			OnRetry:            func(messageID string) { retried <- messageID },
			OnRetriesExhausted: func(messageID string) { exhausted <- messageID },
		})
		message := wmmessage.NewMessage(fake.UUID().V4(), []byte(fake.UUID().V4()))
		failure := errors.New(fake.UUID().V4())
		handler := retryLifecycleMiddleware(state)(func(*wmmessage.Message) ([]*wmmessage.Message, error) {
			return nil, failure
		})
		_, err = handler(message)
		require.ErrorIs(t, err, failure)
		assert.Equal(t, message.UUID, <-retried)
		var lifecycleErr retryLifecycleError
		require.ErrorAs(t, err, &lifecycleErr)
		require.NoError(t, lifecycleErr.OnRetriesExhausted())
		assert.Equal(t, message.UUID, <-exhausted)

		subscriber := NewMockSubscriber(t)
		subscriber.EXPECT().Subscribe(mock.Anything, transportTestTopic).Return(nil, failure).Once()
		subscriber.EXPECT().Close().Return(nil).Once()
		lifecycle := newLifecycleSubscriber(subscriber)
		messages, err := lifecycle.Subscribe(t.Context(), transportTestTopic)
		require.NoError(t, err)
		_, open := <-messages
		assert.False(t, open)
		require.ErrorIs(t, lifecycle.SubscribeError(), failure)
		require.NoError(t, lifecycle.Close())
	})

	t.Run("closes an empty router", func(t *testing.T) {
		subscriber := NewMockSubscriber(t)
		subscriber.EXPECT().Close().Return(nil).Once()
		router, err := wmmessage.NewRouter(wmmessage.RouterConfig{}, watermill.NewSlogLogger(logger))
		require.NoError(t, err)
		dispatchRouter := &Router{
			router:        router,
			subscriber:    newLifecycleSubscriber(subscriber),
			logger:        logger,
			handlerTopics: make(map[string]struct{}),
		}
		runResult := make(chan error, 1)
		go func() { runResult <- dispatchRouter.Run(t.Context()) }()
		<-router.Running()
		require.NoError(t, dispatchRouter.Close())
		require.NoError(t, <-runResult)
	})

	t.Run("does not leak Watermill into domain or runtime packages", func(t *testing.T) {
		for _, root := range []string{
			filepath.Clean(filepath.Join("..", "..", "..", "..", "finance")),
			filepath.Clean(filepath.Join("..", "..", "..", "..", "runtime")),
		} {
			require.NoError(t, assertNoWatermillImports(root))
		}
	})
}
