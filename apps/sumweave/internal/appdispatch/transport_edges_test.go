package appdispatch

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ThreeDotsLabs/watermill"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
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

	t.Run("validates publication requests and transport-only message behavior", func(t *testing.T) {
		fake := faker.New()
		assert.Equal(t, TransportDriverPostgres, Config{DatabaseDSN: "postgresql://example.invalid"}.Driver())
		assert.Equal(t, TransportDriverSQLite, Config{DatabaseDSN: "file:dispatch.db"}.Driver())
		assert.Equal(t, TransportDriverSQLite, Config{DatabaseDSN: "sqlite:dispatch.db"}.Driver())
		assert.Equal(t, "prefix_app_dispatch_messages", Config{TablePrefix: "prefix_"}.MessagesTable())
		assert.Equal(t, "prefix_app_dispatch_offsets", Config{TablePrefix: "prefix_"}.OffsetsTable())
		assert.Equal(t, "prefix_app_dispatch_publications", Config{TablePrefix: "prefix_"}.PublicationsTable())
		require.EqualError(t, Message{Topic: testTopic}.validate(), "message id is required")
		require.EqualError(t, Message{ID: fake.UUID().V4()}.validate(), "message topic is required")
		require.EqualError(t, PublicationRequest{}.validate(), "publication topic is required")
		message := publicationMessage(PublicationRequest{Topic: testTopic, Payload: []byte(`{"b":2,"a":1}`)})
		assert.NotEmpty(t, message.ID)
		assert.Equal(
			t,
			canonicalPayloadHash([]byte(`{"a":1,"b":2}`)),
			message.Metadata[transportPayloadHashMetadataKey],
		)
		assert.Equal(t, hashPayload([]byte(`trailing payload`)), canonicalPayloadHash([]byte(`trailing payload`)))
		assert.NotEqual(
			t,
			canonicalPayloadHash([]byte(`{"id":9007199254740993}`)),
			canonicalPayloadHash([]byte(`{"id":9007199254740992}`)),
		)
		assert.True(t, isDuplicateMessageIDError(errors.New("UNIQUE constraint failed: messages.uuid")))
		assert.True(t, isDuplicateMessageIDError(errors.New("duplicate key value violates unique constraint")))
		assert.False(t, isDuplicateMessageIDError(errors.New("temporary transport error")))
	})

	t.Run("publishes and claims semantic messages with SQL mock boundaries", func(t *testing.T) {
		fake := faker.New()
		newPublisher := func(t *testing.T) (*Publisher, sqlmock.Sqlmock) {
			t.Helper()
			db, databaseMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			publisher, err := NewPublisher(Config{}, db, logger)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, publisher.Close()) })
			return publisher, databaseMock
		}

		t.Run("returns transport publication errors and detects duplicates", func(t *testing.T) {
			publisher, databaseMock := newPublisher(t)
			writeErr := errors.New(fake.Lorem().Sentence(3))
			databaseMock.ExpectExec("INSERT INTO").WillReturnError(writeErr)
			require.ErrorIs(
				t,
				publisher.Publish(t.Context(), NewMessage(testTopic, []byte(fake.UUID().V4()))),
				writeErr,
			)
			databaseMock.ExpectExec("INSERT INTO").WillReturnError(
				errors.New("duplicate key value violates unique constraint"),
			)
			require.ErrorIs(
				t,
				publisher.Publish(t.Context(), NewMessage(testTopic, []byte(fake.UUID().V4()))),
				ErrDuplicateMessageID,
			)
			require.EqualError(t, publisher.Publish(t.Context(), Message{}), "message id is required")
			require.NoError(t, databaseMock.ExpectationsWereMet())
		})

		t.Run("publishes unkeyed and idempotent requests", func(t *testing.T) {
			publisher, databaseMock := newPublisher(t)
			unkeyed := PublicationRequest{Topic: testTopic, Payload: []byte(fake.UUID().V4())}
			databaseMock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(1, 1))
			reference, err := publisher.PublishRequest(t.Context(), unkeyed)
			require.NoError(t, err)
			assert.NotEmpty(t, reference.MessageID)

			request := PublicationRequest{
				Topic:          testTopic,
				Payload:        []byte(`{"value":"` + fake.UUID().V4() + `"}`),
				IdempotencyKey: fake.UUID().V4(),
			}
			databaseMock.ExpectBegin()
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_publications").WillReturnResult(sqlmock.NewResult(1, 1))
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_messages").WillReturnResult(sqlmock.NewResult(1, 1))
			databaseMock.ExpectCommit()
			reference, err = publisher.PublishRequest(t.Context(), request)
			require.NoError(t, err)
			assert.NotEmpty(t, reference.MessageID)
			require.NoError(t, databaseMock.ExpectationsWereMet())
		})

		t.Run("returns transactional publication boundary errors", func(t *testing.T) {
			publisher, databaseMock := newPublisher(t)
			request := PublicationRequest{
				Topic:          testTopic,
				Payload:        []byte(fake.UUID().V4()),
				IdempotencyKey: fake.UUID().V4(),
			}
			beginErr := errors.New(fake.Lorem().Sentence(3))
			databaseMock.ExpectBegin().WillReturnError(beginErr)
			_, err := publisher.PublishRequest(t.Context(), request)
			require.ErrorIs(t, err, beginErr)

			databaseMock.ExpectBegin()
			tx, err := publisher.db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_messages").WillReturnResult(sqlmock.NewResult(1, 1))
			_, err = publisher.PublishRequestInTx(
				t.Context(),
				tx,
				PublicationRequest{Topic: testTopic, Payload: []byte(fake.UUID().V4())},
			)
			require.NoError(t, err)
			databaseMock.ExpectRollback()
			require.NoError(t, tx.Rollback())
			require.EqualError(t, func() error {
				_, publishErr := publisher.PublishRequestInTx(t.Context(), nil, request)
				return publishErr
			}(), "publish transaction is required")
			require.NoError(t, databaseMock.ExpectationsWereMet())
		})

		t.Run("returns transaction write and commit errors", func(t *testing.T) {
			publisher, databaseMock := newPublisher(t)
			writeErr := errors.New(fake.Lorem().Sentence(3))
			databaseMock.ExpectBegin()
			tx, err := publisher.db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			databaseMock.ExpectExec("INSERT INTO").WillReturnError(writeErr)
			require.ErrorIs(
				t,
				publisher.PublishInTx(t.Context(), tx, NewMessage(testTopic, []byte(fake.UUID().V4()))),
				writeErr,
			)
			databaseMock.ExpectRollback()
			require.NoError(t, tx.Rollback())

			request := PublicationRequest{
				Topic:          testTopic,
				Payload:        []byte(fake.UUID().V4()),
				IdempotencyKey: fake.UUID().V4(),
			}
			commitErr := errors.New(fake.Lorem().Sentence(3))
			databaseMock.ExpectBegin()
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_publications").WillReturnResult(sqlmock.NewResult(1, 1))
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_messages").WillReturnResult(sqlmock.NewResult(1, 1))
			databaseMock.ExpectCommit().WillReturnError(commitErr)
			_, err = publisher.PublishRequest(t.Context(), request)
			require.ErrorIs(t, err, commitErr)
			require.NoError(t, databaseMock.ExpectationsWereMet())
		})

		t.Run("returns existing references and semantic conflicts", func(t *testing.T) {
			publisher, databaseMock := newPublisher(t)
			request := PublicationRequest{
				Topic:          testTopic,
				Payload:        []byte(`{"value":"` + fake.UUID().V4() + `"}`),
				IdempotencyKey: fake.UUID().V4(),
			}
			messageID := fake.UUID().V4()
			databaseMock.ExpectBegin()
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_publications").WillReturnResult(sqlmock.NewResult(0, 0))
			databaseMock.ExpectQuery("SELECT message_id").WithArgs(request.IdempotencyKey).WillReturnRows(
				sqlmock.NewRows([]string{"message_id", "topic", "payload_hash"}).AddRow(
					messageID,
					request.Topic,
					canonicalPayloadHash(request.Payload),
				),
			)
			databaseMock.ExpectCommit()
			reference, err := publisher.PublishRequest(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(t, PublicationReference{MessageID: messageID}, reference)

			databaseMock.ExpectBegin()
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_publications").WillReturnResult(sqlmock.NewResult(0, 0))
			databaseMock.ExpectQuery("SELECT message_id").WithArgs(request.IdempotencyKey).WillReturnRows(
				sqlmock.NewRows([]string{"message_id", "topic", "payload_hash"}).AddRow(
					messageID,
					request.Topic,
					fake.UUID().V4(),
				),
			)
			databaseMock.ExpectRollback()
			_, err = publisher.PublishRequest(t.Context(), request)
			require.ErrorIs(t, err, ErrPublicationConflict)
			require.NoError(t, databaseMock.ExpectationsWereMet())
		})

		t.Run("returns idempotency storage errors", func(t *testing.T) {
			publisher, databaseMock := newPublisher(t)
			request := PublicationRequest{
				Topic:          testTopic,
				Payload:        []byte(fake.UUID().V4()),
				IdempotencyKey: fake.UUID().V4(),
			}
			storageErr := errors.New(fake.Lorem().Sentence(3))
			databaseMock.ExpectBegin()
			tx, err := publisher.db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_publications").WillReturnError(storageErr)
			_, err = publisher.PublishRequestInTx(t.Context(), tx, request)
			require.ErrorIs(t, err, storageErr)
			databaseMock.ExpectRollback()
			require.NoError(t, tx.Rollback())

			rowsErr := errors.New(fake.Lorem().Sentence(3))
			databaseMock.ExpectBegin()
			tx, err = publisher.db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			databaseMock.ExpectExec("INSERT INTO.*app_dispatch_publications").WillReturnResult(
				sqlmock.NewErrorResult(rowsErr),
			)
			_, err = publisher.claimPublication(t.Context(), tx, request, fake.UUID().V4())
			require.ErrorIs(t, err, rowsErr)
			databaseMock.ExpectRollback()
			require.NoError(t, tx.Rollback())

			databaseMock.ExpectBegin()
			tx, err = publisher.db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			databaseMock.ExpectQuery("SELECT message_id").WithArgs(request.IdempotencyKey).WillReturnError(
				storageErr,
			)
			_, err = publisher.existingPublication(t.Context(), tx, request)
			require.ErrorIs(t, err, storageErr)
			databaseMock.ExpectRollback()
			require.NoError(t, tx.Rollback())
			require.NoError(t, databaseMock.ExpectationsWereMet())
		})

		t.Run("filters private transport metadata", func(t *testing.T) {
			metadata := wmmessage.Metadata{
				"traceId":                       fake.UUID().V4(),
				transportPayloadHashMetadataKey: fake.UUID().V4(),
			}
			filtered := transportMessageMetadata(metadata)
			assert.Len(t, filtered, 1)
			assert.Empty(t, transportMessagePayloadHash(&wmmessage.Message{}))
			message := wmmessage.NewMessage(fake.UUID().V4(), nil)
			message.Metadata = metadata
			assert.NotEmpty(t, transportMessagePayloadHash(message))
		})

		t.Run("closes injected publisher errors and builds a PostgreSQL subscriber", func(t *testing.T) {
			transport := newMockmessagePublisher(t)
			closeErr := errors.New(fake.Lorem().Sentence(3))
			transport.EXPECT().Close().Return(closeErr).Once()
			publisher := &Publisher{publisher: transport}
			require.ErrorIs(t, publisher.Close(), closeErr)
			require.ErrorIs(t, publisher.Close(), closeErr)
			assert.Equal(t, "?", (&Publisher{config: Config{}}).publicationPlaceholder(1))

			db, subscriberMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			subscriber, err := newMessageSubscriber(
				Config{DatabaseDSN: "postgres://example.invalid/dispatch"},
				db,
				"group-"+fake.UUID().V4(),
				logger,
			)
			require.NoError(t, err)
			require.NoError(t, subscriber.Close())
			require.NoError(t, subscriberMock.ExpectationsWereMet())
		})
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

	t.Run("keeps the SQLite compatibility subscriber lifecycle database-free", func(t *testing.T) {
		db, mockDB, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		config := Config{TablePrefix: "compat_", PollInterval: time.Millisecond}
		subscriberValue, err := newSQLiteTransportSubscriber(config, db, "group", wmLogger)
		require.NoError(t, err)
		subscriber := subscriberValue.(*sqliteTransportSubscriber)

		mockDB.ExpectExec("INSERT INTO").WithArgs(testTopic, "group").WillReturnResult(sqlmock.NewResult(0, 1))
		messages, err := subscriber.Subscribe(t.Context(), testTopic)
		require.NoError(t, err)
		require.NotNil(t, messages)
		require.NoError(t, subscriber.Close())
		require.NoError(t, mockDB.ExpectationsWereMet())
	})

	t.Run("builds SQLite subscription defaults and handles transactional outcomes", func(t *testing.T) {
		db, mockDB, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		config := Config{TablePrefix: "compat_", PollInterval: time.Millisecond}
		subscription := newSQLiteSubscription(config, db, "group", testTopic, wmLogger)
		t.Cleanup(func() {
			subscription.pollTicker.Stop()
			subscription.lockTicker.Stop()
		})
		assert.Equal(t, testTopic, subscription.topic)
		assert.Equal(t, "group", subscription.consumerGroup)

		mockDB.ExpectBegin()
		mockDB.ExpectQuery("UPDATE.*compat_").WillReturnError(errors.New("lock failed"))
		mockDB.ExpectRollback()
		_, err = subscription.NextBatch(t.Context())
		require.ErrorContains(t, err, "unable to acquire row lock")

		mockDB.ExpectBegin()
		mockDB.ExpectQuery("UPDATE.*compat_").WillReturnRows(sqlmock.NewRows([]string{"offset_acked"}).AddRow(0))
		mockDB.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}).
				AddRow("bad", "id", []byte("payload"), []byte(`{}`)),
		)
		mockDB.ExpectRollback()
		_, err = subscription.NextBatch(t.Context())
		require.Error(t, err)

		mockDB.ExpectBegin()
		mockDB.ExpectQuery("UPDATE.*compat_").WillReturnRows(sqlmock.NewRows([]string{"offset_acked"}).AddRow(0))
		mockDB.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}).
				AddRow(1, "id", []byte("payload"), []byte(`{}`)).
				RowError(0, errors.New("read failed")),
		)
		mockDB.ExpectRollback()
		_, err = subscription.NextBatch(t.Context())
		require.ErrorContains(t, err, "read failed")

		mockDB.ExpectBegin()
		mockDB.ExpectQuery("UPDATE.*compat_").WillReturnRows(sqlmock.NewRows([]string{"offset_acked"}).AddRow(0))
		mockDB.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}).
				AddRow(1, "id", []byte("payload"), []byte(`{}`)),
		)
		mockDB.ExpectCommit()
		batch, err := subscription.NextBatch(t.Context())
		require.NoError(t, err)
		require.Len(t, batch, 1)
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
			mockDB.ExpectQuery("UPDATE extend").WillReturnRows(sqlmock.NewRows([]string{"locked_until"}))
			require.ErrorIs(t, subscription.ExtendLock(t.Context()), errSQLiteDeliveryLeaseLost)
			mockDB.ExpectExec("UPDATE acknowledge").WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, subscription.ReleaseLock(t.Context()))
			rowsAffectedErr := errors.New("rows affected failed")
			mockDB.ExpectExec("UPDATE acknowledge").WillReturnResult(sqlmock.NewErrorResult(rowsAffectedErr))
			require.ErrorIs(t, subscription.ReleaseLock(t.Context()), rowsAffectedErr)

			done := make(chan error, 1)
			go func() {
				done <- subscription.Send(t.Context(), sqliteRawMessage{Offset: 4, UUID: "id", Payload: []byte("payload")})
			}()
			message := <-subscription.destination
			message.Ack()
			require.NoError(t, <-done)
			assert.Equal(t, int64(4), subscription.lastAckedOffset)
			nacked := make(chan error, 1)
			go func() {
				nacked <- subscription.Send(t.Context(), sqliteRawMessage{UUID: "nacked", Payload: []byte("payload")})
			}()
			(<-subscription.destination).Nack()
			require.ErrorIs(t, <-nacked, errSQLiteMessageNacked)
			canceledCtx, cancel := context.WithCancel(t.Context())
			cancel()
			require.NoError(t, subscription.Send(canceledCtx, sqliteRawMessage{}))
			assert.True(t, subscription.runCycle(canceledCtx))
			subscription.Run(canceledCtx)
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("expires a pre-delivery lease without advancing the batch offset", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			lockTicks := make(chan time.Time)
			subscription.lockTick = lockTicks
			mockDB.ExpectExec("UPDATE acknowledge").
				WithArgs(int64(0), testTopic, "group", int64(0), "lease").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mockDB.ExpectExec("UPDATE acknowledge").
				WithArgs(int64(0), testTopic, "group", int64(0), "lease").
				WillReturnResult(sqlmock.NewResult(0, 1))

			done := make(chan struct{})
			go func() {
				subscription.processBatch(t.Context(), []sqliteRawMessage{
					{Offset: 1, UUID: "first"},
					{Offset: 2, UUID: "second"},
				})
				close(done)
			}()
			lockTicks <- time.Time{}
			<-done
			assert.Zero(t, subscription.lastAckedOffset)
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("rejects a stale lease release", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			mockDB.ExpectExec("UPDATE acknowledge").
				WithArgs(int64(0), testTopic, "group", int64(0), "lease").
				WillReturnResult(sqlmock.NewResult(0, 0))
			require.ErrorIs(t, subscription.ReleaseLock(t.Context()), errSQLiteDeliveryLeaseLost)
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("extends a delivery lease while awaiting acknowledgement", func(t *testing.T) {
			db, mockDB, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			subscription := makeTestSQLiteSubscription(db, wmLogger)
			lockTicks := make(chan time.Time)
			subscription.lockTick = lockTicks
			mockDB.ExpectQuery("UPDATE extend").WillReturnRows(
				sqlmock.NewRows([]string{"locked_until"}).AddRow(10),
			)

			done := make(chan error, 1)
			go func() {
				done <- subscription.Send(t.Context(), sqliteRawMessage{Offset: 4, UUID: "id"})
			}()
			message := <-subscription.destination
			lockTicks <- time.Time{}
			message.Ack()
			require.NoError(t, <-done)
			require.NoError(t, mockDB.ExpectationsWereMet())
		})

		t.Run("runs batch and poll branches from injected ticks", func(t *testing.T) {
			t.Run("continues after database errors and empty batches", func(t *testing.T) {
				db, mockDB, err := sqlmock.New()
				require.NoError(t, err)
				t.Cleanup(func() { _ = db.Close() })
				subscription := makeTestSQLiteSubscription(db, wmLogger)

				mockDB.ExpectBegin().WillReturnError(errors.New("begin failed"))
				require.False(t, subscription.runCycle(t.Context()))
				mockDB.ExpectBegin()
				mockDB.ExpectQuery("UPDATE offsets").WillReturnRows(
					sqlmock.NewRows([]string{"offset_acked"}).AddRow(0),
				)
				mockDB.ExpectQuery("SELECT messages").WillReturnRows(
					sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}),
				)
				mockDB.ExpectRollback()
				require.False(t, subscription.runCycle(t.Context()))
				require.NoError(t, mockDB.ExpectationsWereMet())
			})

			t.Run("processes a batch and wakes a running poll loop", func(t *testing.T) {
				db, mockDB, err := sqlmock.New()
				require.NoError(t, err)
				t.Cleanup(func() { _ = db.Close() })
				subscription := makeTestSQLiteSubscription(db, wmLogger)
				mockDB.ExpectBegin()
				mockDB.ExpectQuery("UPDATE offsets").WillReturnRows(
					sqlmock.NewRows([]string{"offset_acked"}).AddRow(0),
				)
				mockDB.ExpectQuery("SELECT messages").WillReturnRows(
					sqlmock.NewRows([]string{"offset", "uuid", "payload", "metadata"}).
						AddRow(4, "id", []byte("payload"), []byte(`{}`)),
				)
				mockDB.ExpectCommit()
				mockDB.ExpectExec("UPDATE acknowledge").
					WithArgs(int64(4), testTopic, "group", int64(0), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))

				cycleDone := make(chan bool, 1)
				go func() { cycleDone <- subscription.runCycle(t.Context()) }()
				(<-subscription.destination).Ack()
				require.False(t, <-cycleDone)
				assert.Equal(t, int64(4), subscription.lastAckedOffset)

				pollTicks := make(chan time.Time)
				subscription.pollTick = pollTicks
				runCtx, cancelRun := context.WithCancel(t.Context())
				runDone := make(chan struct{})
				go func() {
					subscription.Run(runCtx)
					close(runDone)
				}()
				pollTicks <- time.Time{}
				cancelRun()
				<-runDone
				require.NoError(t, mockDB.ExpectationsWereMet())
			})
		})
	})

	t.Run("covers postgres schema and row adapters", func(t *testing.T) {
		config := Config{TablePrefix: "edge_"}
		schema := postgresSchema(config)
		assert.Equal(t, "BYTEA", schema.GeneratePayloadType(testTopic))
		first := wmmessage.NewMessage("first", []byte("one"))
		second := wmmessage.NewMessage("second", []byte("two"))
		_, err := schema.InsertQuery(wmsql.InsertQueryParams{
			Topic: testTopic,
			Msgs:  wmmessage.Messages{first, second},
		})
		require.NoError(t, err)
		_, err = schema.SelectQuery(wmsql.SelectQueryParams{})
		require.EqualError(t, err, "single-table postgres offsets adapter is required")
		selectQuery, err := schema.SelectQuery(wmsql.SelectQueryParams{
			Topic:          testTopic,
			ConsumerGroup:  "group",
			OffsetsAdapter: postgresOffsets(config),
		})
		require.NoError(t, err)
		assert.Equal(t, []any{testTopic, "group"}, selectQuery.Args)

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

func TestRouterValidationAndClosedLifecycle(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	t.Run("validates router factory and lifecycle state before startup", func(t *testing.T) {
		_, err := NewRouterFactory(Config{}, nil, &Publisher{}, logger)
		require.EqualError(t, err, "sql database is required")
		state := &retryLifecycleState{}
		router := &Router{retryLifecycle: state}
		callbackID := "message-" + faker.New().UUID().V4()
		require.NoError(t, router.SetRetryLifecycle(RetryLifecycle{OnRetry: func(messageID string) {
			assert.Equal(t, callbackID, messageID)
		}}))
		state.retry(callbackID)
		router.started = true
		require.EqualError(
			t,
			router.SetRetryLifecycle(RetryLifecycle{}),
			"cannot set retry lifecycle after router starts",
		)
		require.EqualError(t, router.Run(t.Context()), "run message router: router is already running")

		subscriber := NewMockSubscriber(t)
		subscriber.EXPECT().Close().Return(nil).Once()
		lifecycle := newLifecycleSubscriber(subscriber)
		lifecycle.BeginClose()
		messages, err := lifecycle.Subscribe(t.Context(), "topic-"+faker.New().UUID().V4())
		require.NoError(t, err)
		_, open := <-messages
		assert.False(t, open)
		require.NoError(t, lifecycle.Close())
	})
	t.Run("routes retry lifecycle and dead-letter metadata through message boundaries", func(t *testing.T) {
		fake := faker.New()
		state := &retryLifecycleState{}
		retried := make(chan string, 1)
		exhausted := make(chan string, 1)
		state.set(RetryLifecycle{
			OnRetry:            func(messageID string) { retried <- messageID },
			OnRetriesExhausted: func(messageID string) { exhausted <- messageID },
		})
		message := wmmessage.NewMessage(fake.UUID().V4(), []byte(fake.UUID().V4()))
		failure := errors.New(fake.Lorem().Sentence(3))
		handler := retryLifecycleMiddleware(state)(func(*wmmessage.Message) ([]*wmmessage.Message, error) {
			return nil, failure
		})
		_, err := handler(message)
		require.ErrorIs(t, err, failure)
		assert.Equal(t, message.UUID, <-retried)
		var lifecycleErr retryLifecycleError
		require.ErrorAs(t, err, &lifecycleErr)
		assert.Equal(t, failure.Error(), lifecycleErr.Error())
		require.ErrorIs(t, lifecycleErr.Unwrap(), failure)
		require.NoError(t, lifecycleErr.OnRetriesExhausted())
		assert.Equal(t, message.UUID, <-exhausted)
		nestedLifecycleErr := retryLifecycleError{
			err: lifecycleErr, messageID: message.UUID, state: &retryLifecycleState{},
		}
		require.NoError(t, nestedLifecycleErr.OnRetriesExhausted())
		assert.Equal(t, message.UUID, <-exhausted)

		publisher := newMockmessagePublisher(t)
		deadLetters := deadLetterPublisher{publisher: publisher}
		forwardTopic := "topic." + fake.UUID().V4()
		publisher.EXPECT().Publish(forwardTopic, message).Return(nil).Once()
		require.NoError(t, deadLetters.Publish(forwardTopic, message))
		message.Metadata.Set("traceId", fake.UUID().V4())
		publisher.EXPECT().Publish(DeadLetterTopic, mock.MatchedBy(func(messages *wmmessage.Message) bool {
			return messages.UUID != message.UUID &&
				messages.Metadata.Get(originalMessageIDMetadataKey) == message.UUID &&
				messages.Metadata.Get("traceId") == message.Metadata.Get("traceId")
		})).Return(nil).Once()
		require.NoError(t, deadLetters.Publish(DeadLetterTopic, message))
		publisher.EXPECT().Close().Return(nil).Once()
		require.NoError(t, deadLetters.Close())

		successHandler := retryLifecycleMiddleware(state)(func(*wmmessage.Message) ([]*wmmessage.Message, error) {
			return wmmessage.Messages{message}, nil
		})
		produced, err := successHandler(message)
		require.NoError(t, err)
		assert.Equal(t, []*wmmessage.Message{message}, produced)

		emptyPublisher := newMockmessagePublisher(t)
		emptyPublisher.EXPECT().Publish(DeadLetterTopic).Return(nil).Once()
		require.NoError(t, deadLetterPublisher{publisher: emptyPublisher}.Publish(DeadLetterTopic))
	})

	t.Run("records subscriber errors and passes successful subscriptions through", func(t *testing.T) {
		subscriber := NewMockSubscriber(t)
		topic := "topic-" + faker.New().UUID().V4()
		messages := make(chan *wmmessage.Message)
		close(messages)
		subscriber.EXPECT().Subscribe(mock.Anything, topic).Return(messages, nil).Once()
		subscriber.EXPECT().Close().Return(nil).Once()
		lifecycle := newLifecycleSubscriber(subscriber)
		got, err := lifecycle.Subscribe(t.Context(), topic)
		require.NoError(t, err)
		assert.Equal(t, (<-chan *wmmessage.Message)(messages), got)
		require.NoError(t, lifecycle.Close())

		failedSubscriber := NewMockSubscriber(t)
		failedTopic := "topic-" + faker.New().UUID().V4()
		subscribeErr := errors.New("subscription failure")
		failedSubscriber.EXPECT().Subscribe(mock.Anything, failedTopic).Return(nil, subscribeErr).Once()
		failedSubscriber.EXPECT().Close().Return(nil).Once()
		failedLifecycle := newLifecycleSubscriber(failedSubscriber)
		_, err = failedLifecycle.Subscribe(t.Context(), failedTopic)
		require.ErrorIs(t, failedLifecycle.SubscribeError(), subscribeErr)
		require.NoError(t, err)
		require.NoError(t, failedLifecycle.Close())
	})

	t.Run("runs the factory middleware around a successful generated subscription", func(t *testing.T) {
		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		publisher, err := NewPublisher(Config{}, db, logger)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		factory, err := NewRouterFactory(Config{}, db, publisher, logger)
		require.NoError(t, err)
		router, err := factory.NewRouter("group-" + faker.New().UUID().V4())
		require.NoError(t, err)

		subscriber := NewMockSubscriber(t)
		messages := make(chan *wmmessage.Message)
		var closeMessages sync.Once
		subscriber.EXPECT().Subscribe(mock.Anything, testTopic).Run(func(ctx context.Context, _ string) {
			go func() {
				<-ctx.Done()
				closeMessages.Do(func() { close(messages) })
			}()
		}).Return(messages, nil).Once()
		subscriber.EXPECT().Close().Run(func() {
			closeMessages.Do(func() { close(messages) })
		}).Return(nil).Once()
		router.subscriber = newLifecycleSubscriber(subscriber)
		processed := make(chan struct{})
		handler, err := NewHandler(testTopic, func(context.Context, Message) error {
			close(processed)
			return nil
		})
		require.NoError(t, err)
		require.NoError(t, router.Handle(handler))

		ctx, cancel := context.WithCancel(t.Context())
		runResult := make(chan error, 1)
		go func() { runResult <- router.Run(ctx) }()
		<-router.router.Running()
		messages <- wmmessage.NewMessage(faker.New().UUID().V4(), []byte(faker.New().UUID().V4()))
		<-processed
		cancel()
		require.ErrorIs(t, <-runResult, context.Canceled)
		require.NoError(t, databaseMock.ExpectationsWereMet())
	})
	require.EqualError(t, func() error {
		_, err := NewHandler("", func(context.Context, Message) error { return nil })
		return err
	}(), "handler topic is required")
	require.NoError(t, (*Router)(nil).Close())
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

	t.Run("cancels generic active handlers while the router drains", func(t *testing.T) {
		subscriber := NewMockSubscriber(t)
		topic := "topic-" + faker.New().UUID().V4()
		messages := make(chan *wmmessage.Message)
		subscriptionContexts := make(chan context.Context, 1)
		var closeMessages sync.Once
		subscriber.EXPECT().Subscribe(mock.Anything, topic).Run(func(ctx context.Context, _ string) {
			subscriptionContexts <- ctx
			go func() {
				<-ctx.Done()
				closeMessages.Do(func() { close(messages) })
			}()
		}).Return(messages, nil).Once()
		subscriber.EXPECT().Close().Run(func() {
			closeMessages.Do(func() { close(messages) })
		}).Return(nil).Once()
		watermillRouter, routerErr := wmmessage.NewRouter(wmmessage.RouterConfig{}, watermill.NewSlogLogger(logger))
		require.NoError(t, routerErr)
		shutdownRouter := &Router{
			router:        watermillRouter,
			subscriber:    newLifecycleSubscriber(subscriber),
			logger:        logger,
			handlerTopics: make(map[string]struct{}),
		}
		handlerStarted := make(chan struct{})
		finishHandler := make(chan struct{})
		handlerContextErr := make(chan error, 1)
		shutdownHandler, handlerErr := NewHandler(topic, func(ctx context.Context, _ Message) error {
			close(handlerStarted)
			<-finishHandler
			handlerContextErr <- ctx.Err()
			return nil
		})
		require.NoError(t, handlerErr)
		require.NoError(t, shutdownRouter.Handle(shutdownHandler))

		runCtx, cancelRun := context.WithCancel(t.Context())
		runResult := make(chan error, 1)
		go func() { runResult <- shutdownRouter.Run(runCtx) }()
		<-watermillRouter.Running()
		messages <- wmmessage.NewMessageWithContext(<-subscriptionContexts, faker.New().UUID().V4(), nil)
		<-handlerStarted
		cancelRun()
		close(finishHandler)
		require.ErrorIs(t, <-handlerContextErr, context.Canceled)
		require.ErrorIs(t, <-runResult, context.Canceled)
	})

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
	pollTicker := time.NewTicker(time.Hour)
	lockTicker := time.NewTicker(time.Hour)
	return &sqliteSubscription{
		db:                     db,
		pollTicker:             pollTicker,
		lockTicker:             lockTicker,
		pollTick:               pollTicker.C,
		lockTick:               lockTicker.C,
		lockDuration:           time.Hour,
		lockTimeoutSeconds:     1,
		topic:                  testTopic,
		consumerGroup:          "group",
		leaseID:                "lease",
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
