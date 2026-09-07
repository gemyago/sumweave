package appdispatch

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRouterRunOnce(t *testing.T) {
	fake := faker.New()
	logger := slog.New(slog.DiscardHandler)

	makeRouter := func(t *testing.T, subscriber *MockSubscriber) *Router {
		t.Helper()
		watermillRouter, err := wmmessage.NewRouter(wmmessage.RouterConfig{}, watermill.NewSlogLogger(logger))
		require.NoError(t, err)
		return &Router{
			router:         watermillRouter,
			subscriber:     newLifecycleSubscriber(subscriber),
			logger:         logger,
			handlerTopics:  make(map[string]struct{}),
			retryLifecycle: &retryLifecycleState{},
		}
	}

	t.Run("waits for subscription attempts before one-shot idle polling", func(t *testing.T) {
		topic := "router." + fake.UUID().V4()
		pollInterval := 10 * time.Millisecond
		subscriber := NewMockSubscriber(t)
		subscribeStarted := make(chan struct{}, 1)
		releaseSubscription := make(chan struct{})
		messages := make(chan *wmmessage.Message, 1)
		message := wmmessage.NewMessage(fake.UUID().V4(), []byte(fake.UUID().V4()))
		messages <- message
		close(messages)
		subscriber.EXPECT().
			Subscribe(mock.Anything, topic).
			RunAndReturn(func(ctx context.Context, _ string) (<-chan *wmmessage.Message, error) {
				subscribeStarted <- struct{}{}
				select {
				case <-releaseSubscription:
					return messages, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}).
			Once()
		subscriber.EXPECT().Close().Return(nil).Once()
		router := makeRouter(t, subscriber)
		delivered := make(chan struct{}, 1)
		handler, err := NewHandler(topic, func(context.Context, Message) error {
			delivered <- struct{}{}
			return nil
		})
		require.NoError(t, err)
		require.NoError(t, router.Handle(handler))

		runDone := make(chan error, 1)
		go func() { runDone <- router.RunOnce(t.Context(), pollInterval) }()
		select {
		case <-subscribeStarted:
		case <-time.After(time.Second):
			t.Fatal("router did not start its subscription")
		}
		select {
		case runErr := <-runDone:
			t.Fatalf("router declared idle before subscription startup: %v", runErr)
		case <-time.After(3 * pollInterval):
		}

		close(releaseSubscription)
		select {
		case <-delivered:
		case <-time.After(time.Second):
			t.Fatal("router did not deliver the queued message")
		}
		require.NoError(t, <-runDone)
	})
}
