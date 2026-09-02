package appdispatch

import (
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAppDispatchRoutineEdges(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	fake := faker.New()

	t.Run("validates publisher construction and transaction messages", func(t *testing.T) {
		_, err := NewPublisher(Config{}, nil, logger)
		require.EqualError(t, err, "sql database is required")
		_, err = NewPublisher(Config{}, &sql.DB{}, nil)
		require.EqualError(t, err, "logger is required")

		publisher := &Publisher{logger: logger}
		require.EqualError(
			t,
			publisher.PublishInTx(t.Context(), nil, Message{ID: fake.UUID().V4()}),
			"publish transaction is required",
		)

		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		databaseMock.ExpectBegin()
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		require.EqualError(
			t,
			publisher.PublishInTx(t.Context(), tx, Message{ID: fake.UUID().V4()}),
			"message topic is required",
		)
		databaseMock.ExpectRollback()
		require.NoError(t, tx.Rollback())
		require.NoError(t, databaseMock.ExpectationsWereMet())

		require.NoError(t, (*Publisher)(nil).Close())
	})

	t.Run("returns unkeyed publication failures", func(t *testing.T) {
		publishErr := errors.New(fake.Lorem().Sentence(3))
		transport := newMockmessagePublisher(t)
		transport.EXPECT().Publish(
			transportTestTopic,
			mock.MatchedBy(func(message *wmmessage.Message) bool {
				return message != nil && message.UUID != ""
			}),
		).Return(publishErr).Once()
		publisher := &Publisher{publisher: transport, logger: logger}

		_, err := publisher.PublishRequest(t.Context(), PublicationRequest{
			Topic:   transportTestTopic,
			Payload: []byte(fake.UUID().V4()),
		})
		require.ErrorIs(t, err, publishErr)
	})

	t.Run("validates transactional publication requests", func(t *testing.T) {
		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		publisher := &Publisher{config: Config{TablePrefix: "routine_"}, logger: logger}

		databaseMock.ExpectBegin()
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		_, err = publisher.PublishRequestInTx(t.Context(), tx, PublicationRequest{})
		require.EqualError(t, err, "publication topic is required")
		databaseMock.ExpectRollback()
		require.NoError(t, tx.Rollback())
		require.NoError(t, databaseMock.ExpectationsWereMet())

		_, err = (&Publisher{}).PublishRequest(t.Context(), PublicationRequest{})
		require.EqualError(t, err, "publication topic is required")
	})

	t.Run("returns transactional publication failures", func(t *testing.T) {
		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		publisher := &Publisher{config: Config{TablePrefix: "routine_"}, logger: logger}

		publishErr := errors.New(fake.Lorem().Sentence(3))
		databaseMock.ExpectBegin()
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		databaseMock.ExpectExec("INSERT INTO").WillReturnError(publishErr)
		_, err = publisher.PublishRequestInTx(t.Context(), tx, PublicationRequest{
			Topic:   transportTestTopic,
			Payload: []byte(fake.UUID().V4()),
		})
		require.ErrorIs(t, err, publishErr)
		databaseMock.ExpectRollback()
		require.NoError(t, tx.Rollback())

		databaseMock.ExpectBegin()
		tx, err = db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		databaseMock.ExpectExec("INSERT INTO").WillReturnError(
			errors.New("duplicate key value violates unique constraint"),
		)
		require.ErrorIs(
			t,
			publisher.PublishInTx(t.Context(), tx, NewMessage(transportTestTopic, []byte(fake.UUID().V4()))),
			ErrDuplicateMessageID,
		)
		databaseMock.ExpectRollback()
		require.NoError(t, tx.Rollback())

		claimErr := errors.New(fake.Lorem().Sentence(3))
		databaseMock.ExpectBegin()
		tx, err = db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		databaseMock.ExpectExec("INSERT INTO.*app_dispatch_publications").WillReturnResult(
			sqlmock.NewResult(1, 1),
		)
		databaseMock.ExpectExec("INSERT INTO.*app_dispatch_messages").WillReturnError(claimErr)
		_, err = publisher.PublishRequestInTx(t.Context(), tx, PublicationRequest{
			Topic:          transportTestTopic,
			Payload:        []byte(fake.UUID().V4()),
			IdempotencyKey: fake.UUID().V4(),
		})
		require.ErrorIs(t, err, claimErr)
		databaseMock.ExpectRollback()
		require.NoError(t, tx.Rollback())
		require.NoError(t, databaseMock.ExpectationsWereMet())
	})

	t.Run("filters private transport metadata in domain messages", func(t *testing.T) {
		message := wmmessage.NewMessage(fake.UUID().V4(), []byte(fake.UUID().V4()))
		message.Metadata.Set("traceId", fake.UUID().V4())
		message.Metadata.Set(transportPayloadHashMetadataKey, fake.UUID().V4())

		converted := makeMessage(transportTestTopic, message)
		assert.Equal(t, transportTestTopic, converted.Topic)
		assert.Len(t, converted.Metadata, 1)
		assert.NotEmpty(t, converted.Metadata["traceId"])
	})
}
