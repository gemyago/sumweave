package appevents

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type unitTestEvent struct {
	ID string `json:"id"`
}

func (unitTestEvent) Topic() string { return "unit.event.v1" }

type unitEmptyTopicEvent struct{}

func (unitEmptyTopicEvent) Topic() string { return "" }

type unitInvalidJSONEvent struct{ Value func() }

func (unitInvalidJSONEvent) Topic() string { return "unit.invalid.v1" }

func TestEventsUnit(t *testing.T) {
	fake := faker.New()

	t.Run("validates event contracts without transport persistence", func(t *testing.T) {
		_, err := NewPublisher(nil)
		require.EqualError(t, err, "message publisher is required")
		_, err = makeMessage(unitEmptyTopicEvent{})
		require.EqualError(t, err, "domain event topic is required")
		_, err = makeMessage(unitInvalidJSONEvent{Value: func() {}})
		require.ErrorContains(t, err, "marshal domain event")
		_, err = MakeHandler(unitEmptyTopicEvent{}, func(context.Context, unitEmptyTopicEvent) error { return nil })
		require.EqualError(t, err, "domain event topic is required")
		_, err = MakeHandler(unitTestEvent{}, nil)
		require.EqualError(t, err, "event handler run func is required")
		require.EqualError(
			t,
			RegisterHandler[unitTestEvent](
				nil,
				unitTestEvent{},
				func(context.Context, unitTestEvent) error { return nil },
			),
			"event router is required",
		)
		var event Event
		_, err = makeMessage(event)
		require.EqualError(t, err, "domain event is required")
	})

	t.Run("publishes and handles typed events without persistence", func(t *testing.T) {
		expected := unitTestEvent{ID: fake.UUID().V4()}
		message, err := makeMessage(expected)
		require.NoError(t, err)
		assert.Equal(t, expected.Topic(), message.Topic)
		matchesExpected := mock.MatchedBy(func(actual appdispatch.Message) bool {
			return actual.ID != "" && actual.Topic == message.Topic && string(actual.Payload) == string(message.Payload)
		})

		publisher := newMockmessagePublisher(t)
		eventPublisher, err := NewPublisher(publisher)
		require.NoError(t, err)
		publisher.EXPECT().Publish(mock.Anything, matchesExpected).Return(nil).Once()
		require.NoError(t, eventPublisher.Publish(t.Context(), expected))
		publishErr := errors.New(fake.UUID().V4())
		publisher.EXPECT().Publish(mock.Anything, matchesExpected).Return(publishErr).Once()
		require.ErrorIs(t, eventPublisher.Publish(t.Context(), expected), publishErr)

		publisher.EXPECT().PublishInTx(mock.Anything, (*sql.Tx)(nil), matchesExpected).Return(nil).Once()
		require.NoError(t, eventPublisher.PublishInTx(t.Context(), nil, expected))
		publisher.EXPECT().PublishInTx(mock.Anything, (*sql.Tx)(nil), matchesExpected).Return(publishErr).Once()
		require.ErrorIs(t, eventPublisher.PublishInTx(t.Context(), nil, expected), publishErr)
		invalidEvent := unitInvalidJSONEvent{Value: func() {}}
		require.ErrorContains(t, eventPublisher.Publish(t.Context(), invalidEvent), "marshal domain event")
		require.ErrorContains(t, eventPublisher.PublishInTx(t.Context(), nil, invalidEvent), "marshal domain event")

		called := false
		handlerErr := makeEventHandler(expected.Topic(), func(_ context.Context, actual unitTestEvent) error {
			called = true
			assert.Equal(t, expected, actual)
			return nil
		})(t.Context(), message)
		require.NoError(t, handlerErr)
		assert.True(t, called)
		require.ErrorContains(
			t,
			makeEventHandler(
				expected.Topic(),
				func(context.Context, unitTestEvent) error { return nil },
			)(t.Context(), appdispatch.Message{Payload: []byte("{")}),
			"decode domain event",
		)

		registrar := newMockhandlerRegistrar(t)
		registrar.EXPECT().Handle(mock.Anything).Return(nil).Once()
		require.NoError(
			t,
			registerHandler(
				registrar,
				unitTestEvent{},
				func(context.Context, unitTestEvent) error { return nil },
			),
		)
		registerErr := errors.New(fake.UUID().V4())
		registrar.EXPECT().Handle(mock.Anything).Return(registerErr).Once()
		require.ErrorIs(
			t,
			registerHandler(
				registrar,
				unitTestEvent{},
				func(context.Context, unitTestEvent) error { return nil },
			),
			registerErr,
		)
		require.Error(
			t,
			registerHandler(
				registrar,
				unitEmptyTopicEvent{},
				func(context.Context, unitEmptyTopicEvent) error { return nil },
			),
		)
	})
}
