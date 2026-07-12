package execution

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
)

const zeroAssumptionValue = "zero"

// ExecuteApprovedIntentRequest configures durable paper/backtest execution.
type ExecuteApprovedIntentRequest struct {
	Intent           domain.OrderIntent
	ApprovedDecision domain.GovernorDecision
	ReplayCandles    []data.ReplayCandle
}

// ExecuteApprovedIntentResult groups durable execution ledger outputs.
type ExecuteApprovedIntentResult struct {
	Command        domain.ExecutionCommand
	Order          domain.ExecutionOrder
	Fill           *domain.ExecutionFill
	Reconciliation domain.ExecutionReconciliation
}

// PaperService persists deterministic paper/backtest execution ledger records.
type PaperService struct {
	store     executionStore
	service   *Service
	simulator *LimitFillSimulator
}

// NewPaperService creates a durable paper execution service.
func NewPaperService(store executionStore) (*PaperService, error) {
	if store == nil {
		return nil, errors.New("execution store is required")
	}

	return &PaperService{
		store:     store,
		service:   NewService(),
		simulator: NewLimitFillSimulator(),
	}, nil
}

// ExecuteApprovedIntent persists command/order records and simulates a limit fill.
func (s *PaperService) ExecuteApprovedIntent(
	ctx context.Context,
	request ExecuteApprovedIntentRequest,
) (ExecuteApprovedIntentResult, error) {
	if err := ctx.Err(); err != nil {
		return ExecuteApprovedIntentResult{}, err
	}

	intent, decision, err := canonicalPaperExecutionRequest(request)
	if err != nil {
		return ExecuteApprovedIntentResult{}, err
	}

	command, err := s.store.CreateCommand(ctx, buildPaperExecutionCommand(intent, decision))
	if err != nil {
		return ExecuteApprovedIntentResult{}, err
	}

	order, err := s.store.CreateOrder(ctx, buildPaperExecutionOrder(command))
	if err != nil {
		return ExecuteApprovedIntentResult{}, err
	}

	simulatedFill, hasFill, err := s.simulator.Simulate(ctx, SimulateLimitFillRequest{
		Order:         order,
		ReplayCandles: request.ReplayCandles,
	})
	if err != nil {
		return ExecuteApprovedIntentResult{}, err
	}

	var persistedFill *domain.ExecutionFill
	if hasFill {
		fill, fillErr := s.store.CreateFill(ctx, simulatedFill)
		if fillErr != nil {
			return ExecuteApprovedIntentResult{}, fillErr
		}
		persistedFill = &fill

		order, err = s.store.UpdateOrderStatus(ctx, string(order.OrderID), domain.ExecutionOrderStatusFilled)
		if err != nil {
			return ExecuteApprovedIntentResult{}, err
		}
	}

	fills, err := s.store.ListFillsByOrder(ctx, string(order.OrderID))
	if err != nil {
		return ExecuteApprovedIntentResult{}, err
	}

	reconciliation, err := s.service.Reconcile(ctx, ReconcileRequest{Order: order, Fills: fills})
	if err != nil {
		return ExecuteApprovedIntentResult{}, err
	}

	return ExecuteApprovedIntentResult{
		Command:        command,
		Order:          order,
		Fill:           persistedFill,
		Reconciliation: reconciliation,
	}, nil
}

// LimitFillSimulator deterministically simulates closed-candle limit fills.
type LimitFillSimulator struct{}

// NewLimitFillSimulator creates the deterministic v0 paper fill simulator.
func NewLimitFillSimulator() *LimitFillSimulator {
	return &LimitFillSimulator{}
}

// SimulateLimitFillRequest configures deterministic limit fill simulation.
type SimulateLimitFillRequest struct {
	Order         domain.ExecutionOrder
	ReplayCandles []data.ReplayCandle
}

// Simulate deterministically returns one full fill or no fill.
func (s *LimitFillSimulator) Simulate(
	_ context.Context,
	request SimulateLimitFillRequest,
) (domain.ExecutionFill, bool, error) {
	order, err := canonicalOrder(request.Order)
	if err != nil {
		return domain.ExecutionFill{}, false, err
	}
	if order.OrderType != domain.OrderTypeLimit {
		return domain.ExecutionFill{}, false, validationError("execution order type is unsupported")
	}
	if order.LimitPrice == nil {
		return domain.ExecutionFill{}, false, validationError("execution limit price is required for limit orders")
	}

	replayCandles := slices.Clone(request.ReplayCandles)
	slices.SortStableFunc(replayCandles, func(left, right data.ReplayCandle) int {
		if comparison := left.Candle.TimeRange.End.Compare(right.Candle.TimeRange.End); comparison != 0 {
			return comparison
		}

		return strings.Compare(strconv.FormatUint(left.Identity, 10), strconv.FormatUint(right.Identity, 10))
	})

	for _, replayCandle := range replayCandles {
		if replayCandle.Candle.Instrument != order.Instrument {
			continue
		}
		if !replayCandle.Candle.TimeRange.End.After(order.EventTime.Time()) {
			continue
		}

		var shouldFill bool
		switch order.Command.ActionKind {
		case domain.CandidateActionKindLong:
			shouldFill = replayCandle.Candle.Low <= *order.LimitPrice
		case domain.CandidateActionKindShort:
			shouldFill = replayCandle.Candle.High >= *order.LimitPrice
		default:
			return domain.ExecutionFill{}, false, validationError("execution action kind is unsupported")
		}
		if !shouldFill {
			continue
		}

		fill, fillErr := domain.NewExecutionFill(domain.ExecutionFillParams{
			FillID: stableID(
				"fill",
				string(order.OrderID),
				strconv.FormatUint(replayCandle.Identity, 10),
				stableTime(replayCandle.Candle.TimeRange.End),
			),
			Order:                     order,
			SourceMarketDataReference: "replay-candle:" + strconv.FormatUint(replayCandle.Identity, 10),
			FeeAmount:                 0,
			SlippageAmount:            0,
			Metadata: map[string]string{
				"fee_model":          zeroAssumptionValue,
				"slippage_model":     zeroAssumptionValue,
				simulatorMetadataKey: closedCandleLimitSimulatorName,
			},
			Quantity:  order.Quantity,
			Price:     *order.LimitPrice,
			EventTime: replayCandle.Candle.TimeRange.End,
		})
		if fillErr != nil {
			return domain.ExecutionFill{}, false, validationError(fillErr.Error())
		}

		return fill, true, nil
	}

	return domain.ExecutionFill{}, false, nil
}

func canonicalPaperExecutionRequest(
	request ExecuteApprovedIntentRequest,
) (domain.OrderIntent, domain.GovernorDecision, error) {
	intent, err := domain.NewOrderIntent(domain.OrderIntentParams{
		IntentID:                 string(request.Intent.IntentID),
		TraceID:                  string(request.Intent.TraceID),
		StrategyID:               request.Intent.StrategyID,
		StrategyVersion:          request.Intent.StrategyVersion,
		StrategyArtifactHash:     request.Intent.StrategyArtifactHash,
		Mode:                     request.Intent.Mode,
		Instrument:               request.Intent.Instrument,
		Timeframe:                request.Intent.Timeframe,
		ActionKind:               request.Intent.ActionKind,
		OrderType:                request.Intent.OrderType,
		RequestedQuantity:        request.Intent.RequestedQuantity,
		RequestedNotional:        request.Intent.RequestedNotional,
		RequestedLimitPrice:      request.Intent.RequestedLimitPrice,
		ReduceOnly:               request.Intent.ReduceOnly,
		SourceReasonCode:         request.Intent.SourceReasonCode,
		CandidateActionReference: request.Intent.CandidateActionReference,
		CreatedTime:              request.Intent.CreatedTime.Time(),
		Status:                   request.Intent.Status,
		Metadata:                 request.Intent.Metadata,
	})
	if err != nil {
		return domain.OrderIntent{}, domain.GovernorDecision{}, validationError(err.Error())
	}
	if intent.Mode == domain.DecisionModeLive {
		return domain.OrderIntent{}, domain.GovernorDecision{}, validationError("live mode is unsupported")
	}
	if intent.OrderType != domain.OrderTypeLimit {
		return domain.OrderIntent{}, domain.GovernorDecision{}, validationError(
			"execution order type is unsupported",
		)
	}
	if intent.RequestedQuantity <= 0 {
		return domain.OrderIntent{}, domain.GovernorDecision{}, validationError(
			"execution command quantity must be positive",
		)
	}

	decision, err := canonicalApprovedDecision(request.ApprovedDecision)
	if err != nil {
		return domain.OrderIntent{}, domain.GovernorDecision{}, err
	}

	action := decision.CandidateAction
	if action.Kind != intent.ActionKind {
		return domain.OrderIntent{}, domain.GovernorDecision{}, validationError(
			"approved decision action kind does not match order intent",
		)
	}
	if action.Strategy.Instrument != intent.Instrument {
		return domain.OrderIntent{}, domain.GovernorDecision{}, validationError(
			"approved decision instrument does not match order intent",
		)
	}
	if action.Strategy.Timeframe != intent.Timeframe {
		return domain.OrderIntent{}, domain.GovernorDecision{}, validationError(
			"approved decision timeframe does not match order intent",
		)
	}

	return intent, decision, nil
}

func buildPaperExecutionCommand(
	intent domain.OrderIntent,
	decision domain.GovernorDecision,
) domain.ExecutionCommand {
	command, _ := domain.NewExecutionCommand(domain.ExecutionCommandParams{
		CommandID: stableID(
			"paper-command",
			string(intent.IntentID),
			stableGovernorDecisionReference(decision),
		),
		TraceID:                   string(intent.TraceID),
		IntentID:                  string(intent.IntentID),
		GovernorDecisionReference: stableGovernorDecisionReference(decision),
		Mode:                      intent.Mode,
		StrategyID:                intent.StrategyID,
		StrategyVersion:           intent.StrategyVersion,
		StrategyArtifactHash:      intent.StrategyArtifactHash,
		Venue:                     intent.Instrument.Venue,
		Instrument:                intent.Instrument,
		ActionKind:                intent.ActionKind,
		OrderType:                 intent.OrderType,
		LimitPrice:                intent.RequestedLimitPrice,
		ReduceOnly:                intent.ReduceOnly,
		ApprovedDecision:          decision,
		Status:                    domain.ExecutionCommandStatusCreated,
		Quantity:                  intent.RequestedQuantity,
		Notional:                  intent.RequestedNotional,
		EventTime:                 decision.DecisionTime.Time(),
	})

	return command
}

func buildPaperExecutionOrder(command domain.ExecutionCommand) domain.ExecutionOrder {
	clientOrderID := stableID("paper-client-order", string(command.CommandID))
	order, _ := domain.NewExecutionOrder(domain.ExecutionOrderParams{
		OrderID:              stableID("paper-order", string(command.CommandID), clientOrderID),
		Command:              command,
		Mode:                 command.Mode,
		StrategyID:           command.StrategyID,
		StrategyVersion:      command.StrategyVersion,
		StrategyArtifactHash: command.StrategyArtifactHash,
		Venue:                command.Venue,
		Instrument:           command.Instrument,
		OrderType:            command.OrderType,
		TimeInForce:          domain.TimeInForceGTC,
		ReduceOnly:           command.ReduceOnly,
		ClientOrderID:        clientOrderID,
		Status:               domain.ExecutionOrderStatusOpen,
		Quantity:             command.Quantity,
		Notional:             command.Notional,
		LimitPrice:           command.LimitPrice,
		EventTime:            command.EventTime.Time(),
	})

	return order
}

func stableGovernorDecisionReference(decision domain.GovernorDecision) string {
	return stableID("governor-decision", stableDecisionKey(decision))
}
