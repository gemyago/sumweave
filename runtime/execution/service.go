package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

// ErrValidation marks rejected inputs that fail execution-layer validation.
var ErrValidation = errors.New("execution validation failed")

const reconciliationQuantityTolerance = 1e-9

// CreateCommandRequest configures approval-only execution command creation.
type CreateCommandRequest struct {
	ApprovedDecision domain.GovernorDecision
	Quantity         float64
	EventTime        time.Time
}

// RecordOrderRequest configures local execution order recording.
type RecordOrderRequest struct {
	Command       domain.ExecutionCommand
	Venue         domain.Venue
	ClientOrderID string
	Quantity      float64
	EventTime     time.Time
}

// RecordFillRequest configures local execution fill recording.
type RecordFillRequest struct {
	Order     domain.ExecutionOrder
	FillID    string
	Quantity  float64
	Price     float64
	EventTime time.Time
}

// ReconcileRequest configures deterministic local order reconciliation.
type ReconcileRequest struct {
	Order domain.ExecutionOrder
	Fills []domain.ExecutionFill
}

// Service provides local approval-only execution behavior.
type Service struct{}

// NewService creates a dependency-free execution service.
func NewService() *Service {
	return &Service{}
}

// CreateCommand creates a canonical execution command from an approved decision.
func (s *Service) CreateCommand(
	_ context.Context,
	request CreateCommandRequest,
) (domain.ExecutionCommand, error) {
	decision, err := canonicalApprovedDecision(request.ApprovedDecision)
	if err != nil {
		return domain.ExecutionCommand{}, err
	}

	command, err := domain.NewExecutionCommand(domain.ExecutionCommandParams{
		CommandID: stableID(
			"command",
			stableDecisionKey(decision),
			stableNumber(request.Quantity),
			stableTime(request.EventTime),
		),
		ApprovedDecision: decision,
		Status:           domain.ExecutionCommandStatusCreated,
		Quantity:         request.Quantity,
		EventTime:        request.EventTime,
	})
	if err != nil {
		return domain.ExecutionCommand{}, validationError(err.Error())
	}

	return command, nil
}

// RecordOrder records a canonical local order for an execution command.
func (s *Service) RecordOrder(
	_ context.Context,
	request RecordOrderRequest,
) (domain.ExecutionOrder, error) {
	command, err := canonicalCommand(request.Command)
	if err != nil {
		return domain.ExecutionOrder{}, err
	}

	order, err := domain.NewExecutionOrder(domain.ExecutionOrderParams{
		OrderID: stableID(
			"order",
			string(command.CommandID),
			request.Venue.String(),
			strings.TrimSpace(request.ClientOrderID),
			stableNumber(request.Quantity),
			stableTime(request.EventTime),
		),
		Command:       command,
		Venue:         request.Venue,
		ClientOrderID: request.ClientOrderID,
		Status:        domain.ExecutionOrderStatusOpen,
		Quantity:      request.Quantity,
		EventTime:     request.EventTime,
	})
	if err != nil {
		return domain.ExecutionOrder{}, validationError(err.Error())
	}

	return order, nil
}

// RecordFill records a canonical local fill for an execution order.
func (s *Service) RecordFill(
	_ context.Context,
	request RecordFillRequest,
) (domain.ExecutionFill, error) {
	order, err := canonicalOrder(request.Order)
	if err != nil {
		return domain.ExecutionFill{}, err
	}

	fill, err := domain.NewExecutionFill(domain.ExecutionFillParams{
		FillID:    request.FillID,
		Order:     order,
		Quantity:  request.Quantity,
		Price:     request.Price,
		EventTime: request.EventTime,
	})
	if err != nil {
		return domain.ExecutionFill{}, validationError(err.Error())
	}

	return fill, nil
}

// Reconcile deterministically reconciles an order against local fills.
func (s *Service) Reconcile(
	_ context.Context,
	request ReconcileRequest,
) (domain.ExecutionReconciliation, error) {
	order, err := canonicalOrder(request.Order)
	if err != nil {
		return domain.ExecutionReconciliation{}, err
	}

	canonicalFills := make([]domain.ExecutionFill, 0, len(request.Fills))
	for idx, fill := range request.Fills {
		canonicalFill, fillErr := canonicalFill(fill)
		if fillErr != nil {
			return domain.ExecutionReconciliation{}, validationError(
				fmt.Sprintf("execution reconciliation fill %d: %s", idx, fillErr.Error()),
			)
		}

		if canonicalFill.Order.OrderID != order.OrderID {
			return domain.ExecutionReconciliation{}, validationError(
				fmt.Sprintf("execution reconciliation fill %d order does not match", idx),
			)
		}

		canonicalFills = append(canonicalFills, canonicalFill)
	}

	slices.SortStableFunc(canonicalFills, func(left, right domain.ExecutionFill) int {
		if comparison := left.EventTime.Time().Compare(right.EventTime.Time()); comparison != 0 {
			return comparison
		}

		return strings.Compare(string(left.FillID), string(right.FillID))
	})

	filledQuantity := 0.0
	for _, fill := range canonicalFills {
		filledQuantity += fill.Quantity
	}

	var status domain.ExecutionOrderStatus
	switch {
	case len(canonicalFills) == 0:
		status = domain.ExecutionOrderStatusOpen
	case math.Abs(filledQuantity-order.Quantity) <= reconciliationQuantityTolerance:
		status = domain.ExecutionOrderStatusFilled
	case filledQuantity < order.Quantity:
		status = domain.ExecutionOrderStatusPartiallyFilled
	default:
		status = domain.ExecutionOrderStatusOverfilled
	}

	eventTime := order.EventTime.Time()
	if len(canonicalFills) > 0 {
		eventTime = canonicalFills[len(canonicalFills)-1].EventTime.Time()
	}

	reconciliation, err := domain.NewExecutionReconciliation(domain.ExecutionReconciliationParams{
		Order:          order,
		Fills:          canonicalFills,
		Status:         status,
		FilledQuantity: filledQuantity,
		EventTime:      eventTime,
	})
	if err != nil {
		return domain.ExecutionReconciliation{}, validationError(err.Error())
	}

	return reconciliation, nil
}

func canonicalApprovedDecision(decision domain.GovernorDecision) (domain.GovernorDecision, error) {
	if decision.Status != domain.GovernorDecisionStatusApproved {
		return domain.GovernorDecision{}, validationError("approved governor decision is required")
	}

	canonicalDecision, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
		CandidateAction: decision.CandidateAction,
		Status:          decision.Status,
		Reason:          decision.Reason,
		DecisionTime:    decision.DecisionTime.Time(),
	})
	if err != nil {
		return domain.GovernorDecision{}, validationError(
			fmt.Sprintf("approved governor decision: %s", err.Error()),
		)
	}
	if canonicalDecision.Status != domain.GovernorDecisionStatusApproved {
		return domain.GovernorDecision{}, validationError("approved governor decision is required")
	}

	return canonicalDecision, nil
}

func canonicalCommand(command domain.ExecutionCommand) (domain.ExecutionCommand, error) {
	canonicalCommand, err := domain.NewExecutionCommand(domain.ExecutionCommandParams{
		CommandID:                 string(command.CommandID),
		TraceID:                   string(command.TraceID),
		IntentID:                  string(command.IntentID),
		Mode:                      command.Mode,
		StrategyID:                command.StrategyID,
		StrategyVersion:           command.StrategyVersion,
		StrategyArtifactHash:      command.StrategyArtifactHash,
		Venue:                     command.Venue,
		Instrument:                command.Instrument,
		ActionKind:                command.ActionKind,
		OrderType:                 command.OrderType,
		LimitPrice:                command.LimitPrice,
		ReduceOnly:                command.ReduceOnly,
		GovernorDecisionReference: command.GovernorDecisionReference,
		ApprovedDecision:          command.ApprovedDecision,
		Status:                    command.Status,
		Quantity:                  command.Quantity,
		Notional:                  command.Notional,
		EventTime:                 command.EventTime.Time(),
	})
	if err != nil {
		return domain.ExecutionCommand{}, validationError(
			fmt.Sprintf("execution order command: %s", err.Error()),
		)
	}

	return canonicalCommand, nil
}

func canonicalOrder(order domain.ExecutionOrder) (domain.ExecutionOrder, error) {
	canonicalOrder, err := domain.NewExecutionOrder(domain.ExecutionOrderParams{
		OrderID:              string(order.OrderID),
		Command:              order.Command,
		Mode:                 order.Mode,
		StrategyID:           order.StrategyID,
		StrategyVersion:      order.StrategyVersion,
		StrategyArtifactHash: order.StrategyArtifactHash,
		Venue:                order.Venue,
		Instrument:           order.Instrument,
		OrderType:            order.OrderType,
		TimeInForce:          order.TimeInForce,
		ReduceOnly:           order.ReduceOnly,
		ClientOrderID:        order.ClientOrderID,
		Status:               order.Status,
		Quantity:             order.Quantity,
		Notional:             order.Notional,
		LimitPrice:           order.LimitPrice,
		EventTime:            order.EventTime.Time(),
	})
	if err != nil {
		return domain.ExecutionOrder{}, validationError(
			fmt.Sprintf("execution fill order: %s", err.Error()),
		)
	}

	return canonicalOrder, nil
}

func canonicalFill(fill domain.ExecutionFill) (domain.ExecutionFill, error) {
	canonicalFill, err := domain.NewExecutionFill(domain.ExecutionFillParams{
		FillID:                    string(fill.FillID),
		Order:                     fill.Order,
		SourceMarketDataReference: fill.SourceMarketDataReference,
		FeeAmount:                 fill.FeeAmount,
		SlippageAmount:            fill.SlippageAmount,
		Metadata:                  fill.Metadata,
		Quantity:                  fill.Quantity,
		Price:                     fill.Price,
		EventTime:                 fill.EventTime.Time(),
	})
	if err != nil {
		return domain.ExecutionFill{}, err
	}

	return canonicalFill, nil
}

func stableDecisionKey(decision domain.GovernorDecision) string {
	action := decision.CandidateAction
	strategy := action.Strategy
	instrument := strategy.Instrument
	inputRange := action.InputRange

	return strings.Join([]string{
		string(instrument.Venue),
		string(instrument.Symbol),
		string(instrument.AssetClass),
		strconv.FormatBool(instrument.Active),
		string(strategy.Timeframe),
		string(strategy.Kind),
		string(action.Kind),
		stableTime(action.DecisionTime.Time()),
		stableTime(inputRange.Start),
		stableTime(inputRange.End),
		string(action.Quality),
		string(decision.Status),
		string(decision.Reason),
		stableTime(decision.DecisionTime.Time()),
	}, "|")
}

func stableID(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(hash[:16])
}

func stableNumber(value float64) string {
	return fmt.Sprintf("%.12f", value)
}

func stableTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
