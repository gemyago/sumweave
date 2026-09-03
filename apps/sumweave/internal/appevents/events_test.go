package appevents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEvent struct {
	ID        string `json:"id"`
	TopicName string `json:"-"`
}

func (event testEvent) Topic() string {
	if event.TopicName != "" {
		return event.TopicName
	}
	return "test.event.v1"
}

type emptyTopicEvent struct{}

func (emptyTopicEvent) Topic() string { return "" }

type invalidJSONEvent struct {
	Value func()
}

func (invalidJSONEvent) Topic() string { return "invalid.event.v1" }

func TestEvents(t *testing.T) {
	fake := faker.New()
	logger := slog.New(slog.DiscardHandler)
	config := appdispatch.Config{
		DatabaseDSN:  os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN"),
		TablePrefix:  "sumweave_",
		PollInterval: 10 * time.Millisecond,
	}
	require.NotEmpty(t, config.DatabaseDSN)
	db, err := sql.Open("pgx", config.DatabaseDSN)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	rawPublisher, err := appdispatch.NewPublisher(config, db, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rawPublisher.Close()) })
	publisher, err := NewPublisher(rawPublisher)
	require.NoError(t, err)

	t.Run("publishes and dispatches typed events", func(t *testing.T) {
		factory, factoryErr := appdispatch.NewRouterFactory(config, db, rawPublisher, logger)
		require.NoError(t, factoryErr)
		router, routerErr := factory.NewRouter("events-group-" + fake.UUID().V4())
		require.NoError(t, routerErr)
		received := make(chan testEvent, 1)
		topic := "test.event." + fake.UUID().V4()
		require.NoError(t, RegisterHandler(
			router,
			testEvent{TopicName: topic},
			func(_ context.Context, event testEvent) error {
				received <- event
				return nil
			},
		))
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(func() {
			cancel()
			require.NoError(t, router.Close())
		})
		go func() { _ = router.Run(ctx) }()

		expected := testEvent{ID: fake.UUID().V4(), TopicName: topic}
		require.NoError(t, publisher.Publish(t.Context(), expected))
		select {
		case actual := <-received:
			assert.Equal(t, expected.ID, actual.ID)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for typed event")
		}
	})

	t.Run("supports transaction-bound event publication", func(t *testing.T) {
		tx, txErr := db.BeginTx(t.Context(), nil)
		require.NoError(t, txErr)
		event := testEvent{ID: fake.UUID().V4()}
		require.NoError(t, publisher.PublishInTx(t.Context(), tx, event))
		require.NoError(t, tx.Rollback())
	})

	t.Run("rejects invalid event contracts and payloads", func(t *testing.T) {
		_, constructorErr := NewPublisher(nil)
		require.EqualError(t, constructorErr, "message publisher is required")
		require.EqualError(t, publisher.Publish(t.Context(), emptyTopicEvent{}), "domain event topic is required")
		require.ErrorContains(
			t,
			publisher.Publish(t.Context(), invalidJSONEvent{Value: func() {}}),
			"marshal domain event",
		)
		_, handlerErr := MakeHandler(emptyTopicEvent{}, func(context.Context, emptyTopicEvent) error { return nil })
		require.EqualError(t, handlerErr, "domain event topic is required")
		_, handlerErr = MakeHandler(testEvent{}, nil)
		require.EqualError(t, handlerErr, "event handler run func is required")
		require.EqualError(t, RegisterHandler[testEvent](nil, testEvent{}, func(context.Context, testEvent) error {
			return nil
		}), "event router is required")
	})

	t.Run("fails malformed typed payloads", func(t *testing.T) {
		malformedTopic := "malformed.event." + fake.UUID().V4()
		handler, handlerErr := MakeHandler(
			testEvent{TopicName: malformedTopic},
			func(context.Context, testEvent) error {
				return errors.New("must not run")
			},
		)
		require.NoError(t, handlerErr)
		factory, factoryErr := appdispatch.NewRouterFactory(config, db, rawPublisher, logger)
		require.NoError(t, factoryErr)
		router, routerErr := factory.NewRouter("malformed-group-" + fake.UUID().V4())
		require.NoError(t, routerErr)
		require.NoError(t, router.Handle(handler))
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(func() {
			cancel()
			require.NoError(t, router.Close())
		})
		go func() { _ = router.Run(ctx) }()
		require.Eventually(t, func() bool {
			message := appdispatch.NewMessage(malformedTopic, []byte("bad-json"))
			if rawPublisher.Publish(t.Context(), message) != nil {
				return false
			}
			var count int
			queryErr := db.QueryRowContext(
				t.Context(),
				`SELECT COUNT(*) FROM sumweave_app_dispatch_messages WHERE topic=$1 AND metadata->>$2=$3`,
				appdispatch.DeadLetterTopic,
				middleware.PoisonedTopicKey,
				malformedTopic,
			).Scan(&count)
			return queryErr == nil && count > 0
		}, 8*time.Second, 50*time.Millisecond)
	})

	t.Run("reports transport publication failures", func(t *testing.T) {
		isolatedRawPublisher, publisherErr := appdispatch.NewPublisher(config, db, logger)
		require.NoError(t, publisherErr)
		isolatedPublisher, publisherErr := NewPublisher(isolatedRawPublisher)
		require.NoError(t, publisherErr)
		var nilEvent Event
		require.EqualError(t, isolatedPublisher.Publish(t.Context(), nilEvent), "domain event is required")

		require.NoError(t, isolatedRawPublisher.Close())
		require.ErrorContains(
			t,
			isolatedPublisher.Publish(t.Context(), testEvent{ID: fake.UUID().V4()}),
			"publish domain event",
		)

		tx, txErr := db.BeginTx(t.Context(), nil)
		require.NoError(t, txErr)
		require.NoError(t, tx.Rollback())
		require.ErrorContains(
			t,
			publisher.PublishInTx(t.Context(), tx, testEvent{ID: fake.UUID().V4()}),
			"publish domain event",
		)
	})

	t.Run("keeps the publisher interface transaction-aware", func(t *testing.T) {
		var _ interface {
			Publish(context.Context, appdispatch.Message) error
			PublishInTx(context.Context, *sql.Tx, appdispatch.Message) error
		} = rawPublisher
		assert.NotEmpty(t, fmt.Sprint(config.PollInterval))
	})
}
