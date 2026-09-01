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

	originalMessageIDMetadataKey = "originalMessageId"
)

// Handler binds one raw transport topic to one application callback.
type Handler struct {
	topic string
	run   func(context.Context, Message) error
}

// NonRetryable marks a handler error that must be sent directly to the durable
// dead-letter flow instead of being retried by the generic router.
type NonRetryable interface {
	error
	NonRetryable()
}

// RetryExhaustionAware marks a delivery error that needs a final action before
// the router dead-letters a message after its retry budget is exhausted.
type RetryExhaustionAware interface {
	error
	OnRetriesExhausted() error
}

// RetryLifecycle observes retryable delivery failures and their final outcome.
// Callbacks run synchronously on the router's delivery path.
type RetryLifecycle struct {
	OnRetry            func(messageID string)
	OnRetriesExhausted func(messageID string)
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
	lifecycle := &retryLifecycleState{}
	router, err := wmmessage.NewRouter(wmmessage.RouterConfig{}, wmLogger)
	if err != nil {
		_ = subscriber.Close()
		return nil, fmt.Errorf("create router for consumer group %s: %w", consumerGroup, err)
	}
	poisonQueue, err := middleware.PoisonQueueWithFilter(
		deadLetterPublisher{publisher: f.publisher.publisher},
		DeadLetterTopic,
		func(err error) bool {
			var nonRetryable NonRetryable
			return !errors.As(err, &nonRetryable)
		},
	)
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
			OnRetriesExhausted: func(params middleware.RetriesExhaustedParams) {
				var aware RetryExhaustionAware
				if !errors.As(params.Err, &aware) {
					return
				}
				if finalizeErr := aware.OnRetriesExhausted(); finalizeErr != nil {
					logger.Error("finalize exhausted message retries failed", "error", finalizeErr)
				}
			},
			ShouldRetry: func(params middleware.RetryParams) bool {
				var nonRetryable NonRetryable
				return !errors.As(params.Err, &nonRetryable)
			},
		}.Middleware,
		retryLifecycleMiddleware(lifecycle),
		middleware.Recoverer,
		func(h wmmessage.HandlerFunc) wmmessage.HandlerFunc {
			return func(msg *wmmessage.Message) ([]*wmmessage.Message, error) {
				logger.InfoContext(
					msg.Context(),
					"received message",
					slog.String("id", msg.UUID),
					slog.Any("metadata", msg.Metadata),
				)
				res, handlerErr := h(msg)
				if handlerErr != nil {
					logger.ErrorContext(msg.Context(), "error handling message", "error", handlerErr)
				}
				return res, handlerErr
			}
		},
	)
	return &Router{
		consumerGroup:  consumerGroup,
		router:         router,
		subscriber:     newLifecycleSubscriber(subscriber),
		logger:         logger,
		handlerTopics:  make(map[string]struct{}),
		closeDone:      make(chan struct{}),
		retryLifecycle: lifecycle,
	}, nil
}

type retryLifecycleState struct {
	mu        sync.Mutex
	lifecycle RetryLifecycle
}

func (s *retryLifecycleState) set(lifecycle RetryLifecycle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lifecycle = lifecycle
}

func (s *retryLifecycleState) retry(messageID string) {
	s.mu.Lock()
	onRetry := s.lifecycle.OnRetry
	s.mu.Unlock()
	if onRetry != nil {
		onRetry(messageID)
	}
}

func (s *retryLifecycleState) retriesExhausted(messageID string) {
	s.mu.Lock()
	onRetriesExhausted := s.lifecycle.OnRetriesExhausted
	s.mu.Unlock()
	if onRetriesExhausted != nil {
		onRetriesExhausted(messageID)
	}
}

func retryLifecycleMiddleware(state *retryLifecycleState) wmmessage.HandlerMiddleware {
	return func(handler wmmessage.HandlerFunc) wmmessage.HandlerFunc {
		return func(message *wmmessage.Message) ([]*wmmessage.Message, error) {
			produced, err := handler(message)
			if err == nil {
				return produced, nil
			}
			var nonRetryable NonRetryable
			if errors.As(err, &nonRetryable) {
				return produced, err
			}
			state.retry(message.UUID)
			return produced, retryLifecycleError{err: err, messageID: message.UUID, state: state}
		}
	}
}

type retryLifecycleError struct {
	err       error
	messageID string
	state     *retryLifecycleState
}

func (e retryLifecycleError) Error() string { return e.err.Error() }

func (e retryLifecycleError) Unwrap() error { return e.err }

func (e retryLifecycleError) OnRetriesExhausted() error {
	defer e.state.retriesExhausted(e.messageID)
	var aware RetryExhaustionAware
	if errors.As(e.err, &aware) {
		return aware.OnRetriesExhausted()
	}
	return nil
}

type deadLetterPublisher struct {
	publisher wmmessage.Publisher
}

func (p deadLetterPublisher) Publish(topic string, messages ...*wmmessage.Message) error {
	if topic != DeadLetterTopic {
		return p.publisher.Publish(topic, messages...)
	}
	deadLetters := make(wmmessage.Messages, 0, len(messages))
	for _, message := range messages {
		deadLetter := wmmessage.NewMessageWithContext(message.Context(), watermill.NewUUID(), message.Payload)
		deadLetter.Metadata = make(wmmessage.Metadata, len(message.Metadata)+1)
		for key, value := range message.Metadata {
			deadLetter.Metadata.Set(key, value)
		}
		deadLetter.Metadata.Set(originalMessageIDMetadataKey, message.UUID)
		deadLetters = append(deadLetters, deadLetter)
	}
	return p.publisher.Publish(topic, deadLetters...)
}

func (p deadLetterPublisher) Close() error {
	return p.publisher.Close()
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
	retryLifecycle  *retryLifecycleState
}

// SetRetryLifecycle sets callbacks for retryable delivery failures. It must be
// called before Run.
func (r *Router) SetRetryLifecycle(lifecycle RetryLifecycle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return errors.New("cannot set retry lifecycle after router starts")
	}
	r.retryLifecycle.set(lifecycle)
	return nil
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
	runCtx, runCancel := context.WithCancel(ctx)
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
