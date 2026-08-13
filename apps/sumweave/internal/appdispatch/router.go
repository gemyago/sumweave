package appdispatch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmmessage "github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
)

const (
	routerMaxRetries      = 3
	routerInitialInterval = 100 * time.Millisecond
)

// Handler binds one raw transport topic to one application callback.
type Handler struct {
	topic string
	run   func(context.Context, Message) error
}

func NewHandler(topic string, run func(context.Context, Message) error) (Handler, error) {
	if topic == "" {
		return Handler{}, errors.New("handler topic is required")
	}
	if run == nil {
		return Handler{}, errors.New("handler run func is required")
	}
	return Handler{topic: topic, run: run}, nil
}

// RouterFactory constructs named routers without starting subscriptions.
type RouterFactory struct {
	config    Config
	db        *sql.DB
	publisher *Publisher
	logger    *slog.Logger
}

func NewRouterFactory(config Config, db *sql.DB, publisher *Publisher, logger *slog.Logger) (*RouterFactory, error) {
	if db == nil {
		return nil, errors.New("sql database is required")
	}
	if publisher == nil {
		return nil, errors.New("publisher is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	return &RouterFactory{config: config.normalize(), db: db, publisher: publisher, logger: logger}, nil
}

func (f *RouterFactory) NewRouter(consumerGroup string) (*Router, error) {
	if consumerGroup == "" {
		return nil, errors.New("consumer group is required")
	}
	logger := f.logger.WithGroup("appdispatchRouter").With(slog.String("consumerGroup", consumerGroup))
	subscriber, err := newMessageSubscriber(f.config, f.db, consumerGroup, logger)
	if err != nil {
		return nil, fmt.Errorf("create subscriber for consumer group %s: %w", consumerGroup, err)
	}
	wmLogger := watermill.NewSlogLogger(logger)
	router, err := wmmessage.NewRouter(wmmessage.RouterConfig{}, wmLogger)
	if err != nil {
		_ = subscriber.Close()
		return nil, fmt.Errorf("create router for consumer group %s: %w", consumerGroup, err)
	}
	poisonQueue, err := middleware.PoisonQueue(f.publisher.publisher, DeadLetterTopic)
	if err != nil {
		_ = subscriber.Close()
		_ = router.Close()
		return nil, fmt.Errorf("create dead-letter middleware: %w", err)
	}
	router.AddMiddleware(
		poisonQueue,
		middleware.Retry{
			MaxRetries:      routerMaxRetries,
			InitialInterval: routerInitialInterval,
			Logger:          wmLogger,
		}.Middleware,
		middleware.Recoverer,
	)
	return &Router{
		consumerGroup: consumerGroup,
		router:        router,
		subscriber:    newLifecycleSubscriber(subscriber),
		logger:        logger,
		handlerTopics: make(map[string]struct{}),
		closeDone:     make(chan struct{}),
	}, nil
}

// Router owns one consumer group's subscriptions and failure policy.
type Router struct {
	consumerGroup string
	router        *wmmessage.Router
	subscriber    *lifecycleSubscriber
	logger        *slog.Logger

	mu              sync.Mutex
	handlerTopics   map[string]struct{}
	started         bool
	closed          bool
	runCancel       context.CancelFunc
	closeDone       chan struct{}
	finishCloseOnce sync.Once
	closeErr        error
}

func (r *Router) Handle(handler Handler) error {
	if handler.topic == "" || handler.run == nil {
		return errors.New("valid handler is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlerTopics[handler.topic]; exists {
		return fmt.Errorf("handler already registered for topic: %s", handler.topic)
	}
	r.handlerTopics[handler.topic] = struct{}{}
	r.router.AddConsumerHandler(
		"handler_"+handler.topic,
		handler.topic,
		r.subscriber,
		func(message *wmmessage.Message) error {
			return handler.run(message.Context(), makeMessage(handler.topic, message))
		},
	)
	return nil
}

// Run starts subscriptions and blocks until cancellation or closure.
func (r *Router) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(err, r.Close())
	}

	r.mu.Lock()
	r.ensureCloseDone()
	if r.closed {
		r.mu.Unlock()
		return errors.New("message router is closed")
	}
	if r.started {
		r.mu.Unlock()
		return errors.New("run message router: router is already running")
	}
	// External cancellation is coordinated through Close so startup subscription
	// errors can be drained through Watermill's handler lifecycle.
	runCtx, runCancel := context.WithCancel(context.WithoutCancel(ctx))
	r.started = true
	r.runCancel = runCancel
	r.mu.Unlock()
	r.logger.InfoContext(ctx, "starting message router")
	runDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			r.requestClose()
		case <-runDone:
		}
	}()
	runErr := r.router.Run(runCtx)
	close(runDone)
	r.requestClose()
	r.finishClose()
	closeErr := r.Close()
	subscribeErr := r.subscriber.SubscribeError()
	if runErr != nil {
		return errors.Join(fmt.Errorf("run message router: %w", runErr), subscribeErr, closeErr)
	}
	if subscribeErr != nil {
		return errors.Join(fmt.Errorf("subscribe message router: %w", subscribeErr), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func (r *Router) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	r.ensureCloseDone()
	closeDone := r.closeDone
	r.mu.Unlock()
	started := r.requestClose()
	if !started {
		r.finishClose()
	}
	<-closeDone
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeErr
}

func (r *Router) requestClose() bool {
	r.mu.Lock()
	r.closed = true
	started := r.started
	hasHandlers := len(r.handlerTopics) > 0
	runCancel := r.runCancel
	r.mu.Unlock()
	r.subscriber.BeginClose()
	if runCancel != nil {
		runCancel()
	}
	// Watermill does not observe Run context cancellation while an empty router
	// waits for a handler. No subscriber can be in Subscribe without a handler,
	// so closing Watermill here unblocks that special case without reintroducing
	// the Close-vs-Subscribe overlap for configured routers.
	if started && !hasHandlers {
		_ = r.router.Close()
	}
	return started
}

func (r *Router) finishClose() {
	r.finishCloseOnce.Do(func() {
		closeErr := r.subscriber.Close()
		r.mu.Lock()
		r.ensureCloseDone()
		r.closeErr = closeErr
		closeDone := r.closeDone
		r.mu.Unlock()
		close(closeDone)
	})
}

// ensureCloseDone must be called with r.mu held. It supports package tests that
// assemble a Router around a controlled Watermill subscriber.
func (r *Router) ensureCloseDone() {
	if r.closeDone == nil {
		r.closeDone = make(chan struct{})
	}
}

type lifecycleSubscriber struct {
	subscriber wmmessage.Subscriber

	mu           sync.Mutex
	closing      bool
	subscribeErr error
	closeOnce    sync.Once
	closeErr     error
}

func newLifecycleSubscriber(subscriber wmmessage.Subscriber) *lifecycleSubscriber {
	return &lifecycleSubscriber{subscriber: subscriber}
}

func (s *lifecycleSubscriber) Subscribe(ctx context.Context, topic string) (<-chan *wmmessage.Message, error) {
	s.mu.Lock()
	if s.closing || s.subscribeErr != nil {
		s.mu.Unlock()
		return s.drainedSubscription()
	}
	s.mu.Unlock()

	messages, err := s.subscriber.Subscribe(ctx, topic)
	if err == nil {
		return messages, nil
	}
	s.rememberSubscribeError(err)
	// Watermill accounts for a handler only after Subscribe succeeds. Returning
	// a closed channel lets every registered handler start and stop, so router
	// shutdown cannot wait forever after a startup subscription error.
	return s.drainedSubscription()
}

func (*lifecycleSubscriber) drainedSubscription() (<-chan *wmmessage.Message, error) {
	return closedMessageChannel(), nil
}

func (s *lifecycleSubscriber) rememberSubscribeError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subscribeErr == nil {
		s.subscribeErr = err
	}
}

func (s *lifecycleSubscriber) BeginClose() {
	s.mu.Lock()
	s.closing = true
	s.mu.Unlock()
}

func (s *lifecycleSubscriber) SubscribeError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subscribeErr
}

func (s *lifecycleSubscriber) Close() error {
	s.closeOnce.Do(func() {
		s.BeginClose()
		s.closeErr = s.subscriber.Close()
	})
	return s.closeErr
}

func closedMessageChannel() <-chan *wmmessage.Message {
	messages := make(chan *wmmessage.Message)
	close(messages)
	return messages
}
