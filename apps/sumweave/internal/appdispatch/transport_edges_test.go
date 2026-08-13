package appdispatch

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ThreeDotsLabs/watermill"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testTopic = "topic"

func TestTransportAdaptersCoverErrorAndLifecycleEdges(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	wmLogger := watermill.NewSlogLogger(logger)

	t.Run("covers message helpers and constructor validation", func(t *testing.T) {
		assert.Equal(
			t,
			TransportDriverPostgres,
			Config{DatabaseDSN: "postgres://%"}.Driver(),
		)
		message := wmmessage.NewMessage("message-id", []byte("payload"))
		message.Metadata.Set("traceId", "trace-id")
		converted := makeMessage(testTopic, message)
		assert.Equal(t, map[string]string{"traceId": "trace-id"}, converted.Metadata)
		assert.Empty(t, makeWatermillMessage(t.Context(), NewMessage(testTopic, nil)).Metadata)
		require.NoError(t, closeIfPresent(nil))

		_, err := newMessagePublisher(Config{}, struct{}{}, logger)
		require.EqualError(t, err, "db is nil")
		_, err = newSQLiteTransportSubscriber(Config{}, nil, "group", wmLogger)
		require.EqualError(t, err, "sqlite subscriber database is required")
		_, err = newMessageSubscriber(Config{}, &sql.DB{}, "", logger)
		require.EqualError(t, err, "consumer group is required")
		db, mockDB, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		assert.NotNil(t, asContextExecutor(db))
		mockDB.ExpectBegin()
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		assert.NotNil(t, asContextExecutor(tx))
		mockDB.ExpectRollback()
		require.NoError(t, tx.Rollback())
		require.NoError(t, rollbackSQLiteTx(nil))
	})

	t.Run("covers sqlite publisher boundary errors", func(t *testing.T) {
		db, mockDB, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		publisherValue, err := newMessagePublisher(Config{}, db, logger)
		require.NoError(t, err)
		publisher := publisherValue.(*wmsql.Publisher)

		writeErr := errors.New("write failed")
		mockDB.ExpectExec("INSERT INTO").WillReturnError(writeErr)
		require.ErrorIs(t, publisher.Publish(testTopic, wmmessage.NewMessage("id", nil)), writeErr)
		mockDB.ExpectBegin()
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		wrapper := &Publisher{config: Config{}, publisher: publisher, logger: logger}
		require.EqualError(t, wrapper.PublishInTx(t.Context(), tx, Message{Topic: testTopic}), "message id is required")
		mockDB.ExpectRollback()
		require.NoError(t, tx.Rollback())
		require.NoError(t, publisher.Close())
		require.ErrorIs(t, publisher.Publish(testTopic, wmmessage.NewMessage("id", nil)), wmsql.ErrPublisherClosed)
		require.NoError(t, mockDB.ExpectationsWereMet())
	})

	t.Run("covers sqlite subscriber setup and batch decoding errors", func(t *testing.T) {
		db, mockDB, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		config := Config{PollInterval: time.Millisecond}
		subscriberValue, err := newSQLiteTransportSubscriber(config, db, "group", wmLogger)
		require.NoError(t, err)
		subscriber := subscriberValue.(*sqliteTransportSubscriber)
		setupErr := errors.New("offset setup failed")
		mockDB.ExpectExec("INSERT INTO").WillReturnError(setupErr)
		_, err = subscriber.Subscribe(t.Context(), testTopic)
		require.ErrorIs(t, err, setupErr)
		require.NoError(t, subscriber.Close())
		_, err = subscriber.Subscribe(t.Context(), testTopic)
		require.EqualError(t, err, "sqlite subscriber is closed")

		mockDB.ExpectQuery("SELECT valid").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}).
				AddRow(1, "id", nil, []byte(`{"traceId":"trace-id"}`)),
		)
		rows, err := db.QueryContext(t.Context(), "SELECT valid")
		require.NoError(t, err)
		batch, err := buildSQLiteBatch(rows)
		require.NoError(t, err)
		require.Len(t, batch, 1)
		assert.Empty(t, batch[0].Payload)

		mockDB.ExpectQuery("SELECT invalid_metadata").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}).
				AddRow(1, "id", []byte("payload"), []byte("not-json")),
		)
		rows, err = db.QueryContext(t.Context(), "SELECT invalid_metadata")
		require.NoError(t, err)
		_, err = buildSQLiteBatch(rows)
		require.ErrorContains(t, err, "unable to parse metadata JSON")

		mockDB.ExpectQuery("SELECT invalid_offset").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}).
				AddRow("invalid", "id", []byte("payload"), []byte(`{}`)),
		)
		rows, err = db.QueryContext(t.Context(), "SELECT invalid_offset")
		require.NoError(t, err)
		_, err = buildSQLiteBatch(rows)
		require.Error(t, err)
		require.NoError(t, mockDB.ExpectationsWereMet())
	})

	t.Run("covers sqlite subscription database outcomes", func(t *testing.T) {
		t.Run("begin failure", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			beginErr := errors.New("begin failed")
			mockDB.ExpectBegin().WillReturnError(beginErr)
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			_, err = subscription.NextBatch(t.Context())
			require.ErrorIs(t, err, beginErr)
		})

		t.Run("unavailable lock", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mockDB.ExpectBegin()
			mockDB.ExpectQuery("UPDATE offsets").WillReturnRows(sqlmock.NewRows([]string{"offset_acked"}))
			mockDB.ExpectRollback()
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			batch, err := subscription.NextBatch(t.Context())
			require.NoError(t, err)
			assert.Empty(t, batch)
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("query failure", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			queryErr := errors.New("query failed")
			mockDB.ExpectBegin()
			mockDB.ExpectQuery("UPDATE offsets").WillReturnRows(
				sqlmock.NewRows([]string{"offset_acked"}).AddRow(0),
			)
			mockDB.ExpectQuery("SELECT messages").WillReturnError(queryErr)
			mockDB.ExpectRollback()
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			_, err = subscription.NextBatch(t.Context())
			require.ErrorIs(t, err, queryErr)
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("invalid lock offset", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mockDB.ExpectBegin()
			mockDB.ExpectQuery("UPDATE offsets").WillReturnRows(
				sqlmock.NewRows([]string{"offset_acked"}).AddRow("invalid"),
			)
			mockDB.ExpectRollback()
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			_, err = subscription.NextBatch(t.Context())
			require.ErrorContains(t, err, "unable to scan offset_acked value")
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("commit failure", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			commitErr := errors.New("commit failed")
			mockDB.ExpectBegin()
			mockDB.ExpectQuery("UPDATE offsets").WillReturnRows(
				sqlmock.NewRows([]string{"offset_acked"}).AddRow(0),
			)
			mockDB.ExpectQuery("SELECT messages").WillReturnRows(
				sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}).
					AddRow(1, "id", []byte("payload"), []byte(`{}`)),
			)
			mockDB.ExpectCommit().WillReturnError(commitErr)
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			_, err = subscription.NextBatch(t.Context())
			require.ErrorIs(t, err, commitErr)
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("empty batch", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mockDB.ExpectBegin()
			mockDB.ExpectQuery("UPDATE offsets").WillReturnRows(
				sqlmock.NewRows([]string{"offset_acked"}).AddRow(0),
			)
			mockDB.ExpectQuery("SELECT messages").WillReturnRows(
				sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}),
			)
			mockDB.ExpectRollback()
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			batch, err := subscription.NextBatch(t.Context())
			require.NoError(t, err)
			assert.Empty(t, batch)
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("extend release and send", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			extendErr := errors.New("extend failed")
			mockDB.ExpectQuery("UPDATE extend").WillReturnError(extendErr)
			require.ErrorIs(t, subscription.ExtendLock(t.Context()), extendErr)
			mockDB.ExpectQuery("UPDATE extend").WillReturnRows(
				sqlmock.NewRows([]string{"locked_until"}).AddRow(10),
			)
			require.NoError(t, subscription.ExtendLock(t.Context()))
			mockDB.ExpectExec("UPDATE acknowledge").WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, subscription.ReleaseLock(t.Context()))

			done := make(chan error, 1)
			go func() {
				done <- subscription.Send(t.Context(), sqliteRawMessage{Offset: 4, UUID: "id", Payload: []byte("payload")})
			}()
			message := <-subscription.destination
			message.Ack()
			require.NoError(t, <-done)
			assert.Equal(t, int64(4), subscription.lastAckedOffset)
			canceledCtx, cancel := context.WithCancel(t.Context())
			cancel()
			require.NoError(t, subscription.Send(canceledCtx, sqliteRawMessage{}))
			assert.True(t, subscription.runCycle(canceledCtx))
			require.NoError(t, mockDB.ExpectationsWereMet())
		})
	})

	t.Run("covers postgres schema and row adapters", func(t *testing.T) {
		config := Config{TablePrefix: "edge_"}
		schema := postgresSchema(config)
		assert.Equal(t, "BYTEA", schema.GeneratePayloadType(testTopic))
		first := wmmessage.NewMessage("first", []byte("one"))
		second := wmmessage.NewMessage("second", []byte("two"))
		insert, err := schema.InsertQuery(wmsql.InsertQueryParams{
			Topic: testTopic,
			Msgs:  wmmessage.Messages{first, second},
		})
		require.NoError(t, err)
		assert.Contains(t, insert.Query, "),(")
		_, err = schema.SelectQuery(wmsql.SelectQueryParams{})
		require.EqualError(t, err, "single-table postgres offsets adapter is required")

		db, mockDB, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mockDB.ExpectQuery("SELECT valid").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata", "transaction_id"}).
				AddRow(7, "id", []byte("payload"), []byte(`{"traceId":"trace-id"}`), 9),
		)
		row := mockQueryRow(t, db, "SELECT valid")
		decoded, err := schema.UnmarshalMessage(wmsql.UnmarshalMessageParams{Row: row})
		require.NoError(t, err)
		assert.Equal(t, "trace-id", decoded.Msg.Metadata.Get("traceId"))
		assert.Equal(t, wmsql.XID8(9), decoded.ExtraData["transaction_id"])

		mockDB.ExpectQuery("SELECT invalid_metadata").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata", "transaction_id"}).
				AddRow(7, "id", []byte("payload"), []byte("not-json"), 9),
		)
		row = mockQueryRow(t, db, "SELECT invalid_metadata")
		_, err = schema.UnmarshalMessage(wmsql.UnmarshalMessageParams{Row: row})
		require.ErrorContains(t, err, "unmarshal postgres message metadata")

		mockDB.ExpectQuery("SELECT invalid_offset").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata", "transaction_id"}).
				AddRow("invalid", "id", []byte("payload"), []byte(`{}`), 9),
		)
		row = mockQueryRow(t, db, "SELECT invalid_offset")
		_, err = schema.UnmarshalMessage(wmsql.UnmarshalMessageParams{Row: row})
		require.ErrorContains(t, err, "scan postgres message row")

		offsets := postgresOffsets(config)
		next, err := offsets.NextOffsetQuery(wmsql.NextOffsetQueryParams{Topic: testTopic, ConsumerGroup: "group"})
		require.NoError(t, err)
		assert.Equal(t, []any{testTopic, "group"}, next.Args)
		require.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestMigratorPostgresTransactions(t *testing.T) {
	config := Config{DatabaseDSN: "postgres://example.invalid/database", TablePrefix: "migration_"}

	t.Run("rejects missing database", func(t *testing.T) {
		require.EqualError(t, AutoMigrate(t.Context(), config, nil), "sql database is required")
	})

	t.Run("commits all schema queries", func(t *testing.T) {
		db, mockDB, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mockDB.ExpectBegin()
		mockDB.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
		mockDB.ExpectExec("CREATE INDEX IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
		mockDB.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
		mockDB.ExpectCommit()
		require.NoError(t, AutoMigrate(t.Context(), config, db))
		require.NoError(t, mockDB.ExpectationsWereMet())
	})

	t.Run("reports begin exec and commit failures", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mockDB.ExpectBegin().WillReturnError(errors.New("begin failed"))
			migrator, err := NewMigrator(config, db)
			require.NoError(t, err)
			require.ErrorContains(t, migrator.Migrate(t.Context()), "begin postgres transport migration")
		})

		t.Run("exec", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mockDB.ExpectBegin()
			mockDB.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnError(errors.New("exec failed"))
			mockDB.ExpectRollback()
			migrator, err := NewMigrator(config, db)
			require.NoError(t, err)
			require.ErrorContains(t, migrator.Migrate(t.Context()), "exec postgres transport migration query")
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("commit", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mockDB.ExpectBegin()
			mockDB.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
			mockDB.ExpectExec("CREATE INDEX IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
			mockDB.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
			mockDB.ExpectCommit().WillReturnError(errors.New("commit failed"))
			migrator, err := NewMigrator(config, db)
			require.NoError(t, err)
			require.ErrorContains(t, migrator.Migrate(t.Context()), "commit postgres transport migration")
		})
	})
}

func TestRouterValidationAndClosedLifecycle(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	require.EqualError(t, func() error {
		_, err := NewHandler("", func(context.Context, Message) error { return nil })
		return err
	}(), "handler topic is required")
	_, err := NewHandler(testTopic, nil)
	require.EqualError(t, err, "handler run func is required")
	_, err = NewRouterFactory(Config{}, &sql.DB{}, nil, logger)
	require.EqualError(t, err, "publisher is required")
	_, err = NewRouterFactory(Config{}, &sql.DB{}, &Publisher{}, nil)
	require.EqualError(t, err, "logger is required")

	db, mockDB, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	publisher, err := NewPublisher(Config{}, db, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, publisher.Close()) })
	factory, err := NewRouterFactory(Config{}, db, publisher, logger)
	require.NoError(t, err)
	_, err = factory.NewRouter("")
	require.EqualError(t, err, "consumer group is required")
	badFactory := &RouterFactory{config: Config{}, db: db, publisher: &Publisher{}, logger: logger}
	badRouter, err := badFactory.NewRouter("bad-publisher-group")
	require.NoError(t, err)
	require.NoError(t, badRouter.Close())
	router, err := factory.NewRouter("group")
	require.NoError(t, err)
	require.EqualError(t, router.Handle(Handler{}), "valid handler is required")
	handler, err := NewHandler(testTopic, func(context.Context, Message) error { return nil })
	require.NoError(t, err)
	require.NoError(t, router.Handle(handler))
	require.EqualError(t, router.Handle(handler), "handler already registered for topic: topic")
	require.NoError(t, router.Close())
	require.EqualError(t, router.Run(t.Context()), "message router is closed")
	require.NoError(t, mockDB.ExpectationsWereMet())

	canceledRouter, err := factory.NewRouter("canceled-group")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, canceledRouter.Run(ctx), context.Canceled)

	t.Run("closes running empty routers after direct Close or external cancellation", func(t *testing.T) {
		makeEmptyRouter := func(t *testing.T, closeErr error) (*Router, *wmmessage.Router) {
			t.Helper()
			subscriber := NewMockSubscriber(t)
			subscriber.EXPECT().Close().Return(closeErr).Once()
			watermillRouter, routerErr := wmmessage.NewRouter(
				wmmessage.RouterConfig{},
				watermill.NewSlogLogger(logger),
			)
			require.NoError(t, routerErr)
			return &Router{
				router:        watermillRouter,
				subscriber:    newLifecycleSubscriber(subscriber),
				logger:        logger,
				handlerTopics: make(map[string]struct{}),
			}, watermillRouter
		}
		waitForRunning := func(t *testing.T, watermillRouter *wmmessage.Router) {
			t.Helper()
			select {
			case <-watermillRouter.Running():
			case <-time.After(time.Second):
				t.Fatal("empty router did not start")
			}
		}
		waitForResult := func(t *testing.T, result <-chan error) error {
			t.Helper()
			select {
			case resultErr := <-result:
				return resultErr
			case <-time.After(time.Second):
				t.Fatal("empty router shutdown did not complete")
				return nil
			}
		}

		t.Run("direct Close", func(t *testing.T) {
			closeErr := errors.New("empty subscriber close failed")
			emptyRouter, watermillRouter := makeEmptyRouter(t, closeErr)
			runResult := make(chan error, 1)
			go func() { runResult <- emptyRouter.Run(t.Context()) }()
			waitForRunning(t, watermillRouter)
			closeResult := make(chan error, 1)
			go func() { closeResult <- emptyRouter.Close() }()
			require.ErrorIs(t, waitForResult(t, closeResult), closeErr)
			require.ErrorIs(t, waitForResult(t, runResult), closeErr)
		})

		t.Run("external cancellation", func(t *testing.T) {
			emptyRouter, watermillRouter := makeEmptyRouter(t, nil)
			runCtx, cancelRun := context.WithCancel(t.Context())
			runResult := make(chan error, 1)
			go func() { runResult <- emptyRouter.Run(runCtx) }()
			waitForRunning(t, watermillRouter)
			cancelRun()
			require.ErrorIs(t, waitForResult(t, runResult), context.Canceled)
			require.NoError(t, emptyRouter.Close())
		})
	})

	t.Run("joins pre-canceled Run and subscriber close failures", func(t *testing.T) {
		closeErr := errors.New("pre-canceled subscriber close failed")
		subscriber := NewMockSubscriber(t)
		subscriber.EXPECT().Close().Return(closeErr).Once()
		preCanceledRouter := &Router{subscriber: newLifecycleSubscriber(subscriber)}
		preCanceledCtx, cancelPreCanceled := context.WithCancel(t.Context())
		cancelPreCanceled()
		runErr := preCanceledRouter.Run(preCanceledCtx)
		require.ErrorIs(t, runErr, context.Canceled)
		require.ErrorIs(t, runErr, closeErr)
	})

	realConfig := Config{DatabaseDSN: filepath.Join(t.TempDir(), "router.sqlite"), PollInterval: time.Millisecond}
	realDB, err := sqlconn.Open(realConfig.DatabaseDSN)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, realDB.Close()) })
	require.NoError(t, AutoMigrate(t.Context(), realConfig, realDB))
	realPublisher, err := NewPublisher(realConfig, realDB, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, realPublisher.Close()) })
	realFactory, err := NewRouterFactory(realConfig, realDB, realPublisher, logger)
	require.NoError(t, err)
	failingRouter, err := realFactory.NewRouter("concurrent-run-group")
	require.NoError(t, err)
	require.NoError(t, failingRouter.Handle(handler))
	firstCtx, cancelFirst := context.WithCancel(t.Context())
	defer cancelFirst()
	firstRun := make(chan error, 1)
	go func() { firstRun <- failingRouter.Run(firstCtx) }()
	<-failingRouter.router.Running()
	require.ErrorContains(t, failingRouter.Run(t.Context()), "run message router")
	cancelFirst()
	<-firstRun

	t.Run("returns the subscriber close failure after handlers stop", func(t *testing.T) {
		subscriber := NewMockSubscriber(t)
		topic := "topic-" + faker.New().UUID().V4()
		messages := make(chan *wmmessage.Message)
		closeErr := errors.New("subscriber close failed")
		var closeMessages sync.Once
		subscriber.EXPECT().Subscribe(mock.Anything, topic).Run(func(ctx context.Context, _ string) {
			go func() {
				<-ctx.Done()
				closeMessages.Do(func() { close(messages) })
			}()
		}).Return(messages, nil).Once()
		subscriber.EXPECT().Close().Run(func() {
			closeMessages.Do(func() { close(messages) })
		}).Return(closeErr).Once()
		watermillRouter, routerErr := wmmessage.NewRouter(wmmessage.RouterConfig{}, watermill.NewSlogLogger(logger))
		require.NoError(t, routerErr)
		closingRouter := &Router{
			router:        watermillRouter,
			subscriber:    newLifecycleSubscriber(subscriber),
			logger:        logger,
			handlerTopics: make(map[string]struct{}),
		}
		closingHandler, handlerErr := NewHandler(topic, func(context.Context, Message) error { return nil })
		require.NoError(t, handlerErr)
		require.NoError(t, closingRouter.Handle(closingHandler))

		runResult := make(chan error, 1)
		go func() { runResult <- closingRouter.Run(t.Context()) }()
		<-watermillRouter.Running()
		require.ErrorIs(t, closingRouter.Close(), closeErr)
		require.ErrorIs(t, <-runResult, closeErr)
	})

	t.Run("cancellation does not call Close until Subscribe returns", func(t *testing.T) {
		subscriber := NewMockSubscriber(t)
		topic := "topic-" + faker.New().UUID().V4()
		messages := make(chan *wmmessage.Message)
		subscribeStarted := make(chan struct{})
		subscribeCanceled := make(chan struct{})
		allowSubscribeReturn := make(chan struct{})
		subscribeReturned := make(chan struct{})
		closeBeforeSubscribeReturned := make(chan struct{}, 1)
		var closeMessages sync.Once
		subscriber.EXPECT().Subscribe(mock.Anything, topic).RunAndReturn(
			func(ctx context.Context, _ string) (<-chan *wmmessage.Message, error) {
				close(subscribeStarted)
				<-ctx.Done()
				close(subscribeCanceled)
				closeMessages.Do(func() { close(messages) })
				<-allowSubscribeReturn
				close(subscribeReturned)
				return messages, nil
			},
		).Once()
		subscriber.EXPECT().Close().Run(func() {
			select {
			case <-subscribeReturned:
			default:
				closeBeforeSubscribeReturned <- struct{}{}
				t.Error("subscriber Close entered before Subscribe returned")
			}
			closeMessages.Do(func() {
				close(messages)
			})
		}).Return(nil).Once()
		watermillRouter, routerErr := wmmessage.NewRouter(wmmessage.RouterConfig{}, watermill.NewSlogLogger(logger))
		require.NoError(t, routerErr)
		startupRouter := &Router{
			router:        watermillRouter,
			subscriber:    newLifecycleSubscriber(subscriber),
			logger:        logger,
			handlerTopics: make(map[string]struct{}),
		}
		startupHandler, handlerErr := NewHandler(topic, func(context.Context, Message) error { return nil })
		require.NoError(t, handlerErr)
		require.NoError(t, startupRouter.Handle(startupHandler))

		startupCtx, cancelStartup := context.WithCancel(t.Context())
		t.Cleanup(cancelStartup)
		runResult := make(chan error, 1)
		go func() { runResult <- startupRouter.Run(startupCtx) }()
		<-subscribeStarted
		cancelStartup()
		<-subscribeCanceled
		select {
		case <-closeBeforeSubscribeReturned:
			t.Fatal("subscriber Close entered before Subscribe was released")
		default:
		}
		close(allowSubscribeReturn)
		<-subscribeReturned
		require.ErrorIs(t, <-runResult, context.Canceled)
	})

	t.Run("drains Watermill handlers when cancellation interrupts subscription startup", func(t *testing.T) {
		subscriber := NewMockSubscriber(t)
		topic := "topic-" + faker.New().UUID().V4()
		subscribeStarted := make(chan struct{})
		subscriber.EXPECT().Subscribe(mock.Anything, topic).RunAndReturn(
			func(ctx context.Context, _ string) (<-chan *wmmessage.Message, error) {
				close(subscribeStarted)
				<-ctx.Done()
				return nil, ctx.Err()
			},
		).Once()
		subscriber.EXPECT().Close().Return(nil).Once()
		watermillRouter, routerErr := wmmessage.NewRouter(
			wmmessage.RouterConfig{CloseTimeout: 100 * time.Millisecond},
			watermill.NewSlogLogger(logger),
		)
		require.NoError(t, routerErr)
		startupRouter := &Router{
			router:        watermillRouter,
			subscriber:    newLifecycleSubscriber(subscriber),
			logger:        logger,
			handlerTopics: make(map[string]struct{}),
		}
		startupHandler, handlerErr := NewHandler(topic, func(context.Context, Message) error { return nil })
		require.NoError(t, handlerErr)
		require.NoError(t, startupRouter.Handle(startupHandler))

		startupCtx, cancelStartup := context.WithCancel(t.Context())
		runResult := make(chan error, 1)
		go func() { runResult <- startupRouter.Run(startupCtx) }()
		<-subscribeStarted
		cancelStartup()
		select {
		case runErr := <-runResult:
			require.ErrorIs(t, runErr, context.Canceled)
			require.NotErrorIs(t, runErr, context.DeadlineExceeded)
			require.NotContains(t, runErr.Error(), "router close timeout")
		case <-time.After(time.Second):
			t.Fatal("router shutdown did not complete")
		}
	})

	t.Run("returns startup subscription and close failures without a shutdown timeout", func(t *testing.T) {
		subscriber := NewMockSubscriber(t)
		topic := "topic-" + faker.New().UUID().V4()
		subscribeErr := errors.New("subscribe failed")
		closeErr := errors.New("subscriber close failed")
		subscriber.EXPECT().Subscribe(mock.Anything, topic).Return(nil, subscribeErr).Once()
		subscriber.EXPECT().Close().Return(closeErr).Once()
		watermillRouter, routerErr := wmmessage.NewRouter(
			wmmessage.RouterConfig{CloseTimeout: 100 * time.Millisecond},
			watermill.NewSlogLogger(logger),
		)
		require.NoError(t, routerErr)
		startupRouter := &Router{
			router:        watermillRouter,
			subscriber:    newLifecycleSubscriber(subscriber),
			logger:        logger,
			handlerTopics: make(map[string]struct{}),
		}
		startupHandler, handlerErr := NewHandler(topic, func(context.Context, Message) error { return nil })
		require.NoError(t, handlerErr)
		require.NoError(t, startupRouter.Handle(startupHandler))

		runErr := startupRouter.Run(t.Context())
		require.ErrorIs(t, runErr, subscribeErr)
		require.ErrorIs(t, runErr, closeErr)
		require.NotContains(t, runErr.Error(), "router close timeout")
	})
}

func makeTestSQLiteSubscription(db sqliteDatabase, logger watermill.LoggerAdapter) *sqliteSubscription {
	return &sqliteSubscription{
		db:                     db,
		pollTicker:             time.NewTicker(time.Hour),
		lockTicker:             time.NewTicker(time.Hour),
		lockDuration:           time.Hour,
		lockTimeoutSeconds:     1,
		topic:                  testTopic,
		consumerGroup:          "group",
		sqlLockConsumerGroup:   "UPDATE offsets",
		sqlExtendLock:          "UPDATE extend",
		sqlNextMessageBatch:    "SELECT messages",
		sqlAcknowledgeMessages: "UPDATE acknowledge",
		destination:            make(chan *wmmessage.Message),
		logger:                 logger,
	}
}

func mockQueryRow(t *testing.T, db *sql.DB, query string) *sql.Row {
	t.Helper()
	return db.QueryRowContext(t.Context(), query)
}
