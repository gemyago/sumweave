package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

var ErrHandlerNotRegistered = errors.New("job handler not registered")

type observedHandler interface {
	topic() string
	jobType() JobType
	metadata(json.RawMessage) (JobMetadata, error)
	execute(context.Context, Job, json.RawMessage) error
}
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]observedHandler
}

func NewRegistry() *Registry {
	return &Registry{handlers: map[string]observedHandler{}}
}
func (r *Registry) Register(handler observedHandler) error {
	if handler == nil {
		return errors.New("job handler is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[handler.topic()]; exists {
		return fmt.Errorf("observed job handler already registered for topic: %s", handler.topic())
	}
	r.handlers[handler.topic()] = handler
	return nil
}
func (r *Registry) Handler(topic string) (observedHandler, error) { //nolint:ireturn
	if r == nil {
		return nil, ErrHandlerNotRegistered
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[topic]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrHandlerNotRegistered, topic)
	}
	return handler, nil
}
func (r *Registry) Handlers() []observedHandler {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	handlers := make([]observedHandler, 0, len(r.handlers))
	for _, handler := range r.handlers {
		handlers = append(handlers, handler)
	}
	return handlers
}

type TypedHandlerSpec[Input any] struct {
	JobType  JobType
	Topic    string
	Metadata func(Input) (JobMetadata, error)
	Run      func(context.Context, Job, Input) error
}

func RegisterTypedHandler[Input any](registry *Registry, spec TypedHandlerSpec[Input]) error {
	if registry == nil {
		return errors.New("job registry is required")
	}
	if spec.Topic == "" {
		return errors.New("observed job handler topic is required")
	}
	if spec.JobType == "" {
		return errors.New("observed job type is required")
	}
	if spec.Metadata == nil {
		return errors.New("observed job metadata mapper is required")
	}
	if spec.Run == nil {
		return errors.New("job handler run func is required")
	}
	return registry.Register(&registeredTypedHandler[Input]{spec: spec})
}

type registeredTypedHandler[Input any] struct{ spec TypedHandlerSpec[Input] }

func (h *registeredTypedHandler[Input]) topic() string    { return h.spec.Topic }
func (h *registeredTypedHandler[Input]) jobType() JobType { return h.spec.JobType }
func (h *registeredTypedHandler[Input]) metadata(payload json.RawMessage) (JobMetadata, error) {
	var input Input
	if err := json.Unmarshal(payload, &input); err != nil {
		return JobMetadata{}, fmt.Errorf("decode observed job payload: %w", err)
	}
	return h.spec.Metadata(input)
}

func (h *registeredTypedHandler[Input]) execute(
	ctx context.Context,
	job Job,
	payload json.RawMessage,
) error {
	var input Input
	if err := json.Unmarshal(payload, &input); err != nil {
		return fmt.Errorf("decode job payload: %w", err)
	}
	return h.spec.Run(ctx, job, input)
}
