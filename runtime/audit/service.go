package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

// ErrValidation marks rejected inputs that fail audit-layer validation.
var ErrValidation = errors.New("audit validation failed")

// ErrTraceNotFound marks missing persisted decision traces.
var ErrTraceNotFound = errors.New("decision trace not found")

// ErrOrderIntentNotFound marks missing persisted order intents.
var ErrOrderIntentNotFound = errors.New("order intent not found")

// TraceQuery configures deterministic audit trace filtering.
type TraceQuery struct {
	StrategyID string
	Instrument *domain.Instrument
	Mode       *domain.DecisionMode
	TimeRange  *domain.TimeRange
}

// OrderIntentQuery configures deterministic audit intent filtering.
type OrderIntentQuery struct {
	StrategyID string
	Instrument *domain.Instrument
	Mode       *domain.DecisionMode
	TimeRange  *domain.TimeRange
}

// IntentContext captures the stable boundary passed to downstream governor work.
type IntentContext struct {
	Trace           domain.DecisionTrace
	Intent          domain.OrderIntent
	CandidateAction domain.CandidateAction
}

// Store captures durable audit persistence operations.
type Store interface {
	CreateTrace(ctx context.Context, trace domain.DecisionTrace) (domain.DecisionTrace, error)
	GetTrace(ctx context.Context, traceID string) (*domain.DecisionTrace, error)
	QueryTraces(ctx context.Context, query TraceQuery) ([]domain.DecisionTrace, error)
	UpdateTraceMetadata(
		ctx context.Context,
		traceID string,
		metadata map[string]string,
	) (domain.DecisionTrace, error)
	CreateOrderIntent(ctx context.Context, intent domain.OrderIntent) (domain.OrderIntent, error)
	GetOrderIntent(ctx context.Context, intentID string) (*domain.OrderIntent, error)
	QueryOrderIntents(ctx context.Context, query OrderIntentQuery) ([]domain.OrderIntent, error)
	UpdateOrderIntentStatus(
		ctx context.Context,
		intentID string,
		status domain.OrderIntentStatus,
	) (domain.OrderIntent, error)
	UpdateOrderIntentMetadata(
		ctx context.Context,
		intentID string,
		metadata map[string]string,
	) (domain.OrderIntent, error)
}

// Service coordinates audit validation and durable trace/intent behavior.
type Service struct {
	store Store
}

// NewService creates an audit service with required persistence.
func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("audit store is required")
	}

	return &Service{store: store}, nil
}

// RecordTrace canonicalizes and persists a durable decision trace.
func (s *Service) RecordTrace(
	ctx context.Context,
	trace domain.DecisionTrace,
) (domain.DecisionTrace, error) {
	if err := ctx.Err(); err != nil {
		return domain.DecisionTrace{}, err
	}

	canonicalizedTrace, err := canonicalTrace(trace)
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	persisted, err := s.store.CreateTrace(ctx, canonicalizedTrace)
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	return canonicalTrace(persisted)
}

// CreateOrderIntent canonicalizes, validates, and persists a durable order intent.
func (s *Service) CreateOrderIntent(
	ctx context.Context,
	intent domain.OrderIntent,
) (domain.OrderIntent, error) {
	if err := ctx.Err(); err != nil {
		return domain.OrderIntent{}, err
	}

	canonicalizedIntent, err := canonicalIntent(intent)
	if err != nil {
		return domain.OrderIntent{}, err
	}

	trace, err := s.store.GetTrace(ctx, string(canonicalizedIntent.TraceID))
	if err != nil {
		return domain.OrderIntent{}, err
	}
	if trace == nil {
		return domain.OrderIntent{}, fmt.Errorf(
			"%w: %s",
			ErrTraceNotFound,
			canonicalizedIntent.TraceID,
		)
	}

	persisted, err := s.store.CreateOrderIntent(ctx, canonicalizedIntent)
	if err != nil {
		return domain.OrderIntent{}, err
	}

	return canonicalIntent(persisted)
}

// UpdateOrderIntentStatus validates stable status transitions and persists them.
func (s *Service) UpdateOrderIntentStatus(
	ctx context.Context,
	intentID string,
	status domain.OrderIntentStatus,
) (domain.OrderIntent, error) {
	if err := ctx.Err(); err != nil {
		return domain.OrderIntent{}, err
	}

	canonicalStatus, err := domain.NewOrderIntentStatus(status.String())
	if err != nil {
		return domain.OrderIntent{}, validationError("order intent status is required")
	}

	current, err := s.store.GetOrderIntent(ctx, intentID)
	if err != nil {
		return domain.OrderIntent{}, err
	}
	if current == nil {
		return domain.OrderIntent{}, fmt.Errorf("%w: %s", ErrOrderIntentNotFound, intentID)
	}

	canonicalCurrent, err := canonicalIntent(*current)
	if err != nil {
		return domain.OrderIntent{}, err
	}

	if transitionErr := validateIntentTransition(canonicalCurrent.Status, canonicalStatus); transitionErr != nil {
		return domain.OrderIntent{}, transitionErr
	}

	persisted, err := s.store.UpdateOrderIntentStatus(ctx, intentID, canonicalStatus)
	if err != nil {
		return domain.OrderIntent{}, err
	}

	return canonicalIntent(persisted)
}

// UpdateTraceMetadata validates and persists stable trace metadata updates.
func (s *Service) UpdateTraceMetadata(
	ctx context.Context,
	traceID string,
	metadata map[string]string,
) (domain.DecisionTrace, error) {
	if err := ctx.Err(); err != nil {
		return domain.DecisionTrace{}, err
	}

	current, err := s.store.GetTrace(ctx, traceID)
	if err != nil {
		return domain.DecisionTrace{}, err
	}
	if current == nil {
		return domain.DecisionTrace{}, fmt.Errorf("%w: %s", ErrTraceNotFound, traceID)
	}

	updated, err := canonicalTrace(domain.DecisionTrace{
		TraceID:              current.TraceID,
		Mode:                 current.Mode,
		DecisionTime:         current.DecisionTime,
		StrategyID:           current.StrategyID,
		StrategyVersion:      current.StrategyVersion,
		StrategyArtifactHash: current.StrategyArtifactHash,
		Instrument:           current.Instrument,
		Timeframe:            current.Timeframe,
		DatasetReference:     current.DatasetReference,
		RunReference:         current.RunReference,
		InputRange:           current.InputRange,
		AnalyticsReference:   current.AnalyticsReference,
		DataQuality:          current.DataQuality,
		EvaluatorName:        current.EvaluatorName,
		EvaluatorVersion:     current.EvaluatorVersion,
		Result:               current.Result,
		ReasonCodes:          current.ReasonCodes,
		Metadata:             metadata,
	})
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	persisted, err := s.store.UpdateTraceMetadata(ctx, traceID, updated.Metadata)
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	return canonicalTrace(persisted)
}

// UpdateOrderIntentMetadata validates and persists stable intent metadata updates.
func (s *Service) UpdateOrderIntentMetadata(
	ctx context.Context,
	intentID string,
	metadata map[string]string,
) (domain.OrderIntent, error) {
	if err := ctx.Err(); err != nil {
		return domain.OrderIntent{}, err
	}

	current, err := s.store.GetOrderIntent(ctx, intentID)
	if err != nil {
		return domain.OrderIntent{}, err
	}
	if current == nil {
		return domain.OrderIntent{}, fmt.Errorf("%w: %s", ErrOrderIntentNotFound, intentID)
	}

	updated, err := canonicalIntent(domain.OrderIntent{
		IntentID:                 current.IntentID,
		TraceID:                  current.TraceID,
		StrategyID:               current.StrategyID,
		StrategyVersion:          current.StrategyVersion,
		StrategyArtifactHash:     current.StrategyArtifactHash,
		Mode:                     current.Mode,
		Instrument:               current.Instrument,
		Timeframe:                current.Timeframe,
		ActionKind:               current.ActionKind,
		OrderType:                current.OrderType,
		RequestedQuantity:        current.RequestedQuantity,
		RequestedNotional:        current.RequestedNotional,
		RequestedLimitPrice:      current.RequestedLimitPrice,
		ReduceOnly:               current.ReduceOnly,
		SourceReasonCode:         current.SourceReasonCode,
		CandidateActionReference: current.CandidateActionReference,
		CreatedTime:              current.CreatedTime,
		Status:                   current.Status,
		Metadata:                 metadata,
	})
	if err != nil {
		return domain.OrderIntent{}, err
	}

	persisted, err := s.store.UpdateOrderIntentMetadata(ctx, intentID, updated.Metadata)
	if err != nil {
		return domain.OrderIntent{}, err
	}

	return canonicalIntent(persisted)
}

// GetTrace reads a persisted trace by stable id.
func (s *Service) GetTrace(
	ctx context.Context,
	traceID string,
) (*domain.DecisionTrace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	trace, err := s.store.GetTrace(ctx, traceID)
	if err != nil || trace == nil {
		return trace, err
	}

	canonical, canonicalErr := canonicalTrace(*trace)
	if canonicalErr != nil {
		return nil, canonicalErr
	}

	return &canonical, nil
}

// ListTraces returns deterministic filtered trace records.
func (s *Service) ListTraces(
	ctx context.Context,
	query TraceQuery,
) ([]domain.DecisionTrace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	traces, err := s.store.QueryTraces(ctx, query)
	if err != nil {
		return nil, err
	}

	canonical := make([]domain.DecisionTrace, 0, len(traces))
	for _, trace := range traces {
		canonicalTrace, canonicalErr := canonicalTrace(trace)
		if canonicalErr != nil {
			return nil, canonicalErr
		}
		canonical = append(canonical, canonicalTrace)
	}

	return canonical, nil
}

// GetOrderIntent reads a persisted order intent by stable id.
func (s *Service) GetOrderIntent(
	ctx context.Context,
	intentID string,
) (*domain.OrderIntent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	intent, err := s.store.GetOrderIntent(ctx, intentID)
	if err != nil || intent == nil {
		return intent, err
	}

	canonical, canonicalErr := canonicalIntent(*intent)
	if canonicalErr != nil {
		return nil, canonicalErr
	}

	return &canonical, nil
}

// ListOrderIntents returns deterministic filtered order intent records.
func (s *Service) ListOrderIntents(
	ctx context.Context,
	query OrderIntentQuery,
) ([]domain.OrderIntent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	intents, err := s.store.QueryOrderIntents(ctx, query)
	if err != nil {
		return nil, err
	}

	canonical := make([]domain.OrderIntent, 0, len(intents))
	for _, intent := range intents {
		canonicalIntent, canonicalErr := canonicalIntent(intent)
		if canonicalErr != nil {
			return nil, canonicalErr
		}
		canonical = append(canonical, canonicalIntent)
	}

	return canonical, nil
}

func canonicalTrace(trace domain.DecisionTrace) (domain.DecisionTrace, error) {
	return domain.NewDecisionTrace(domain.DecisionTraceParams{
		TraceID:              string(trace.TraceID),
		Mode:                 trace.Mode,
		DecisionTime:         trace.DecisionTime.Time(),
		StrategyID:           trace.StrategyID,
		StrategyVersion:      trace.StrategyVersion,
		StrategyArtifactHash: trace.StrategyArtifactHash,
		Instrument:           trace.Instrument,
		Timeframe:            trace.Timeframe,
		DatasetReference:     trace.DatasetReference,
		RunReference:         trace.RunReference,
		InputRange:           trace.InputRange,
		AnalyticsReference:   trace.AnalyticsReference,
		DataQuality:          trace.DataQuality,
		EvaluatorName:        trace.EvaluatorName,
		EvaluatorVersion:     trace.EvaluatorVersion,
		Result:               trace.Result,
		ReasonCodes:          trace.ReasonCodes,
		Metadata:             trace.Metadata,
	})
}

func canonicalIntent(intent domain.OrderIntent) (domain.OrderIntent, error) {
	return domain.NewOrderIntent(domain.OrderIntentParams{
		IntentID:                 string(intent.IntentID),
		TraceID:                  string(intent.TraceID),
		StrategyID:               intent.StrategyID,
		StrategyVersion:          intent.StrategyVersion,
		StrategyArtifactHash:     intent.StrategyArtifactHash,
		Mode:                     intent.Mode,
		Instrument:               intent.Instrument,
		Timeframe:                intent.Timeframe,
		ActionKind:               intent.ActionKind,
		OrderType:                intent.OrderType,
		RequestedQuantity:        intent.RequestedQuantity,
		RequestedNotional:        intent.RequestedNotional,
		RequestedLimitPrice:      intent.RequestedLimitPrice,
		ReduceOnly:               intent.ReduceOnly,
		SourceReasonCode:         intent.SourceReasonCode,
		CandidateActionReference: intent.CandidateActionReference,
		CreatedTime:              intent.CreatedTime.Time(),
		Status:                   intent.Status,
		Metadata:                 intent.Metadata,
	})
}

func validateIntentTransition(
	from domain.OrderIntentStatus,
	to domain.OrderIntentStatus,
) error {
	if from == to {
		return nil
	}

	allowed := map[domain.OrderIntentStatus]map[domain.OrderIntentStatus]struct{}{
		domain.OrderIntentStatusCreated: {
			domain.OrderIntentStatusSentToGovernor: {},
			domain.OrderIntentStatusApproved:       {},
			domain.OrderIntentStatusRejected:       {},
			domain.OrderIntentStatusBlocked:        {},
		},
		domain.OrderIntentStatusSentToGovernor: {
			domain.OrderIntentStatusApproved: {},
			domain.OrderIntentStatusRejected: {},
			domain.OrderIntentStatusBlocked:  {},
		},
		domain.OrderIntentStatusApproved: {
			domain.OrderIntentStatusExecutionCreated: {},
		},
		domain.OrderIntentStatusRejected:         {},
		domain.OrderIntentStatusBlocked:          {},
		domain.OrderIntentStatusExecutionCreated: {},
	}

	if _, ok := allowed[from][to]; !ok {
		return validationError(
			fmt.Sprintf("invalid order intent status transition %q -> %q", from, to),
		)
	}

	return nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
