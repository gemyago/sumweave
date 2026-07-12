package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
)

var ErrHandlerNotRegistered = errors.New("job handler not registered")

type typedHandler interface {
	jobType() JobType
	dispatchKind() appdispatch.ExecutionKind
	maxAttempts() int
	supportsCancel() bool
	supportsRetry() bool
	guardDuplicateDelivery() bool
	onScheduled(context.Context, Job) error
	execute(context.Context, Job, func(json.RawMessage) error) (json.RawMessage, error)
}

type Registry struct {
	mu               sync.RWMutex
	handlers         map[JobType]typedHandler
	dispatchHandlers map[appdispatch.ExecutionKind]typedHandler
}

func NewRegistry() *Registry {
	return &Registry{
		handlers:         map[JobType]typedHandler{},
		dispatchHandlers: map[appdispatch.ExecutionKind]typedHandler{},
	}
}

func (r *Registry) Register(handler typedHandler) error {
	if handler == nil {
		return errors.New("job handler is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[handler.jobType()]; exists {
		return fmt.Errorf("job handler already registered: %s", handler.jobType())
	}
	if _, exists := r.dispatchHandlers[handler.dispatchKind()]; exists {
		return fmt.Errorf("job dispatch handler already registered: %s", handler.dispatchKind())
	}
	r.handlers[handler.jobType()] = handler
	r.dispatchHandlers[handler.dispatchKind()] = handler
	return nil
}

func (r *Registry) Handler(jobType JobType) (typedHandler, error) { //nolint:ireturn
	if r == nil {
		return nil, ErrHandlerNotRegistered
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[jobType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrHandlerNotRegistered, jobType)
	}
	return handler, nil
}

func (r *Registry) HandlerByExecutionKind(kind appdispatch.ExecutionKind) (typedHandler, error) { //nolint:ireturn
	if r == nil {
		return nil, ErrHandlerNotRegistered
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.dispatchHandlers[kind]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrHandlerNotRegistered, kind)
	}
	return handler, nil
}

type TypedHandlerSpec[Input any, Result any, Progress any] struct {
	JobType                JobType
	DispatchKind           appdispatch.ExecutionKind
	MaxAttempts            int
	SupportsCancel         bool
	SupportsRetry          bool
	GuardDuplicateDelivery bool
	Run                    func(context.Context, Input, func(Progress) error) (Result, error)
	RunJob                 func(context.Context, Job, Input, func(Progress) error) (Result, error)
	OnScheduled            func(context.Context, Job) error
}

func RegisterTypedHandler[Input any, Result any, Progress any](
	registry *Registry,
	spec TypedHandlerSpec[Input, Result, Progress],
) error {
	if registry == nil {
		return errors.New("job registry is required")
	}
	if spec.Run == nil && spec.RunJob == nil {
		return errors.New("job handler run func is required")
	}
	return registry.Register(&registeredTypedHandler[Input, Result, Progress]{spec: spec})
}

type registeredTypedHandler[Input any, Result any, Progress any] struct {
	spec TypedHandlerSpec[Input, Result, Progress]
}

func (h *registeredTypedHandler[Input, Result, Progress]) jobType() JobType {
	return h.spec.JobType
}

func (h *registeredTypedHandler[Input, Result, Progress]) dispatchKind() appdispatch.ExecutionKind {
	if h.spec.DispatchKind != "" {
		return h.spec.DispatchKind
	}
	if h.spec.JobType == JobTypeHistoricalRawCandleBackfill {
		return DispatchKindHistoricalRawCandleBackfill
	}
	return appdispatch.ExecutionKind(h.spec.JobType)
}

func (h *registeredTypedHandler[Input, Result, Progress]) maxAttempts() int {
	if h.spec.MaxAttempts > 0 {
		return h.spec.MaxAttempts
	}
	return defaultWorkerMaxAttempts
}

func (h *registeredTypedHandler[Input, Result, Progress]) supportsCancel() bool {
	return h.spec.SupportsCancel
}

func (h *registeredTypedHandler[Input, Result, Progress]) supportsRetry() bool {
	return h.spec.SupportsRetry
}

func (h *registeredTypedHandler[Input, Result, Progress]) guardDuplicateDelivery() bool {
	return h.spec.GuardDuplicateDelivery
}

func (h *registeredTypedHandler[Input, Result, Progress]) onScheduled(ctx context.Context, job Job) error {
	if h.spec.OnScheduled == nil {
		return nil
	}
	return h.spec.OnScheduled(ctx, job)
}

func (h *registeredTypedHandler[Input, Result, Progress]) execute(
	ctx context.Context,
	job Job,
	setProgressJSON func(json.RawMessage) error,
) (json.RawMessage, error) {
	input, err := DecodeJobInput[Input](job)
	if err != nil {
		return nil, err
	}
	setProgress := func(progress Progress) error {
		payload, marshalErr := json.Marshal(progress)
		if marshalErr != nil {
			return fmt.Errorf("marshal job progress: %w", marshalErr)
		}
		return setProgressJSON(payload)
	}
	var result Result
	if h.spec.RunJob != nil {
		result, err = h.spec.RunJob(ctx, job, input, setProgress)
	} else {
		result, err = h.spec.Run(ctx, input, setProgress)
	}
	if err != nil {
		return nil, err
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal job result: %w", err)
	}
	return resultJSON, nil
}
