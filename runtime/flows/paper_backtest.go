package flows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/analytics"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/execution"
	"github.com/gemyago/signal-foundry/runtime/governor"
	"github.com/gemyago/signal-foundry/runtime/strategy"
)

// ErrValidation marks rejected paper backtest flow inputs.
var ErrValidation = errors.New("paper backtest flow validation failed")

// CandleReplayReader loads replay candles for deterministic paper runs.
type CandleReplayReader interface {
	ReplayCandles(
		ctx context.Context,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
	) ([]data.ReplayCandle, error)
}

// StrategyEvaluator evaluates deterministic strategy output for the flow.
type StrategyEvaluator interface {
	Evaluate(
		ctx context.Context,
		request strategy.EvaluateRequest,
	) (strategy.EvaluateResult, error)
}

// AnalyticsCalculator calculates deterministic analytics inputs for the flow.
type AnalyticsCalculator interface {
	CalculateCandles(
		ctx context.Context,
		request analytics.CalculateCandlesRequest,
	) (domain.AnalyticsSeries, error)
}

// GovernorEvaluator evaluates deterministic governor decisions for the flow.
type GovernorEvaluator interface {
	Evaluate(
		ctx context.Context,
		request governor.EvaluateRequest,
	) (governor.EvaluateResult, error)
}

// ExecutionRecorder creates local paper execution records.
type ExecutionRecorder interface {
	CreateCommand(
		ctx context.Context,
		request execution.CreateCommandRequest,
	) (domain.ExecutionCommand, error)
	RecordOrder(
		ctx context.Context,
		request execution.RecordOrderRequest,
	) (domain.ExecutionOrder, error)
	RecordFill(
		ctx context.Context,
		request execution.RecordFillRequest,
	) (domain.ExecutionFill, error)
	Reconcile(
		ctx context.Context,
		request execution.ReconcileRequest,
	) (domain.ExecutionReconciliation, error)
}

// PaperBacktestFlowDeps configures the deterministic paper backtest flow.
type PaperBacktestFlowDeps struct {
	CandleReplayReader  CandleReplayReader
	AnalyticsCalculator AnalyticsCalculator
	StrategyEvaluator   StrategyEvaluator
	GovernorEvaluator   GovernorEvaluator
	ExecutionRecorder   ExecutionRecorder
}

// PaperBacktestRequest defines one deterministic paper backtest run.
type PaperBacktestRequest struct {
	RunID              string
	Instrument         domain.Instrument
	Timeframe          domain.Timeframe
	TimeRange          domain.TimeRange
	StrategyParameters strategy.MovingAverageCrossoverParams
	GovernorPolicy     governor.Policy
	Quantity           float64
}

// PaperExecutionResult groups local paper execution records for one decision.
type PaperExecutionResult struct {
	ApprovedDecision domain.GovernorDecision
	ReconciliationID string
	Command          domain.ExecutionCommand
	Order            domain.ExecutionOrder
	Fill             domain.ExecutionFill
	Reconciliation   domain.ExecutionReconciliation
}

// PaperBacktestResult returns in-memory paper backtest outputs.
type PaperBacktestResult struct {
	RunID              string
	StrategyEvaluation strategy.EvaluateResult
	GovernorEvaluation governor.EvaluateResult
	PaperExecutions    []PaperExecutionResult
}

// PaperBacktestFlow keeps deterministic paper backtest orchestration thin.
type PaperBacktestFlow struct {
	candleReplayReader  CandleReplayReader
	analyticsCalculator AnalyticsCalculator
	strategyEvaluator   StrategyEvaluator
	governorEvaluator   GovernorEvaluator
	executionRecorder   ExecutionRecorder
}

// NewPaperBacktestFlow creates a paper backtest flow with required dependencies.
func NewPaperBacktestFlow(deps PaperBacktestFlowDeps) (*PaperBacktestFlow, error) {
	if deps.CandleReplayReader == nil {
		return nil, errors.New("candle replay reader is required")
	}
	if deps.AnalyticsCalculator == nil {
		return nil, errors.New("analytics calculator is required")
	}
	if deps.StrategyEvaluator == nil {
		return nil, errors.New("strategy evaluator is required")
	}
	if deps.GovernorEvaluator == nil {
		return nil, errors.New("governor evaluator is required")
	}
	if deps.ExecutionRecorder == nil {
		return nil, errors.New("execution recorder is required")
	}

	return &PaperBacktestFlow{
		candleReplayReader:  deps.CandleReplayReader,
		analyticsCalculator: deps.AnalyticsCalculator,
		strategyEvaluator:   deps.StrategyEvaluator,
		governorEvaluator:   deps.GovernorEvaluator,
		executionRecorder:   deps.ExecutionRecorder,
	}, nil
}

// Run orchestrates deterministic replay, analytics, strategy, and governor stages.
func (f *PaperBacktestFlow) Run(
	ctx context.Context,
	request PaperBacktestRequest,
) (PaperBacktestResult, error) {
	canonicalRequest, err := canonicalizePaperBacktestRequest(request)
	if err != nil {
		return PaperBacktestResult{}, err
	}

	replayedCandles, err := f.candleReplayReader.ReplayCandles(
		ctx,
		canonicalRequest.instrument,
		canonicalRequest.timeframe,
		canonicalRequest.timeRange,
	)
	if err != nil {
		return PaperBacktestResult{}, fmt.Errorf("replay candles: %w", err)
	}

	analyticsErr := f.runAnalyticsStage(ctx, canonicalRequest)
	if analyticsErr != nil {
		return PaperBacktestResult{}, analyticsErr
	}

	strategyEvaluation, err := f.strategyEvaluator.Evaluate(ctx, strategy.EvaluateRequest{
		Instrument:   canonicalRequest.instrument,
		Timeframe:    canonicalRequest.timeframe,
		TimeRange:    canonicalRequest.timeRange,
		StrategyKind: domain.StrategyKindMovingAverageCrossover,
		Parameters:   canonicalRequest.strategyParameters,
	})
	if err != nil {
		return PaperBacktestResult{}, fmt.Errorf("evaluate strategy: %w", err)
	}

	governorEvaluation, err := f.governorEvaluator.Evaluate(ctx, governor.EvaluateRequest{
		CandidateActions: strategyEvaluation.Actions,
		Policy:           canonicalRequest.governorPolicy,
	})
	if err != nil {
		return PaperBacktestResult{}, fmt.Errorf("evaluate governor: %w", err)
	}

	paperExecutions, err := f.runExecutionStage(
		ctx,
		canonicalRequest,
		governorEvaluation.Decisions,
		replayedCandles,
	)
	if err != nil {
		return PaperBacktestResult{}, err
	}

	return PaperBacktestResult{
		RunID:              canonicalRequest.runID,
		StrategyEvaluation: strategyEvaluation,
		GovernorEvaluation: governorEvaluation,
		PaperExecutions:    paperExecutions,
	}, nil
}

func (f *PaperBacktestFlow) runAnalyticsStage(
	ctx context.Context,
	request canonicalPaperBacktestRequest,
) error {
	if _, err := f.analyticsCalculator.CalculateCandles(ctx, analytics.CalculateCandlesRequest{
		Instrument:    request.instrument,
		Timeframe:     request.timeframe,
		TimeRange:     request.timeRange,
		IndicatorKind: domain.IndicatorKindMovingAverage,
		IndicatorParams: domain.IndicatorParams{
			Window: request.strategyParameters.FastWindow,
		},
	}); err != nil {
		return fmt.Errorf("calculate fast moving average analytics: %w", err)
	}

	if _, err := f.analyticsCalculator.CalculateCandles(ctx, analytics.CalculateCandlesRequest{
		Instrument:    request.instrument,
		Timeframe:     request.timeframe,
		TimeRange:     request.timeRange,
		IndicatorKind: domain.IndicatorKindMovingAverage,
		IndicatorParams: domain.IndicatorParams{
			Window: request.strategyParameters.SlowWindow,
		},
	}); err != nil {
		return fmt.Errorf("calculate slow moving average analytics: %w", err)
	}

	return nil
}

func (f *PaperBacktestFlow) runExecutionStage(
	ctx context.Context,
	request canonicalPaperBacktestRequest,
	decisions []domain.GovernorDecision,
	replayedCandles []data.ReplayCandle,
) ([]PaperExecutionResult, error) {
	approvedExecutions := make([]PaperExecutionResult, 0, len(decisions))
	replayClosePrices := make(map[time.Time]float64, len(replayedCandles))
	for _, replayedCandle := range replayedCandles {
		replayClosePrices[replayedCandle.Candle.TimeRange.End.UTC()] = replayedCandle.Candle.Close
	}

	approvedDecisionOrder := 0
	for idx, decision := range decisions {
		if decision.Status != domain.GovernorDecisionStatusApproved {
			continue
		}

		decisionTime := decision.DecisionTime.Time().UTC()
		fillPrice, ok := replayClosePrices[decisionTime]
		if !ok {
			return nil, fmt.Errorf(
				"paper execution approved decision %d fill price candle: replay candle close price is required at decision time",
				idx,
			)
		}

		command, err := f.executionRecorder.CreateCommand(ctx, execution.CreateCommandRequest{
			ApprovedDecision: decision,
			Quantity:         request.quantity,
			EventTime:        decisionTime,
		})
		if err != nil {
			return nil, fmt.Errorf("paper execution approved decision %d create command: %w", idx, err)
		}

		clientOrderID := flowStableID("client-order", request.runID, strconv.Itoa(approvedDecisionOrder))
		order, err := f.executionRecorder.RecordOrder(ctx, execution.RecordOrderRequest{
			Command:       command,
			Venue:         request.instrument.Venue,
			ClientOrderID: clientOrderID,
			Quantity:      request.quantity,
			EventTime:     decisionTime,
		})
		if err != nil {
			return nil, fmt.Errorf("paper execution approved decision %d record order: %w", idx, err)
		}

		fill, err := f.executionRecorder.RecordFill(ctx, execution.RecordFillRequest{
			Order:     order,
			FillID:    flowStableID("fill", request.runID, strconv.Itoa(approvedDecisionOrder)),
			Quantity:  request.quantity,
			Price:     fillPrice,
			EventTime: decisionTime,
		})
		if err != nil {
			return nil, fmt.Errorf("paper execution approved decision %d record fill: %w", idx, err)
		}

		reconciliation, err := f.executionRecorder.Reconcile(ctx, execution.ReconcileRequest{
			Order: order,
			Fills: []domain.ExecutionFill{fill},
		})
		if err != nil {
			return nil, fmt.Errorf("paper execution approved decision %d reconcile: %w", idx, err)
		}

		approvedExecutions = append(approvedExecutions, PaperExecutionResult{
			ApprovedDecision: decision,
			ReconciliationID: flowStableID("reconciliation", request.runID, strconv.Itoa(approvedDecisionOrder)),
			Command:          command,
			Order:            order,
			Fill:             fill,
			Reconciliation:   reconciliation,
		})
		approvedDecisionOrder++
	}

	return approvedExecutions, nil
}

type canonicalPaperBacktestRequest struct {
	runID              string
	instrument         domain.Instrument
	timeframe          domain.Timeframe
	timeRange          domain.TimeRange
	strategyParameters strategy.MovingAverageCrossoverParams
	governorPolicy     governor.Policy
	quantity           float64
}

func canonicalizePaperBacktestRequest(
	request PaperBacktestRequest,
) (canonicalPaperBacktestRequest, error) {
	runID := strings.TrimSpace(request.RunID)
	if runID == "" {
		return canonicalPaperBacktestRequest{}, validationError("run id is required")
	}

	strategyIdentity, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
		Instrument: request.Instrument,
		Timeframe:  request.Timeframe,
		Kind:       domain.StrategyKindMovingAverageCrossover,
	})
	if err != nil {
		return canonicalPaperBacktestRequest{}, validationError(err.Error())
	}

	timeRange, err := domain.NewTimeRange(request.TimeRange.Start, request.TimeRange.End)
	if err != nil {
		return canonicalPaperBacktestRequest{}, validationError(err.Error())
	}

	strategyParameters, err := strategy.NewMovingAverageCrossoverParams(request.StrategyParameters)
	if err != nil {
		return canonicalPaperBacktestRequest{}, validationError(err.Error())
	}

	governorPolicy, err := canonicalizeGovernorPolicy(request.GovernorPolicy)
	if err != nil {
		return canonicalPaperBacktestRequest{}, err
	}

	if math.IsNaN(request.Quantity) || math.IsInf(request.Quantity, 0) || request.Quantity <= 0 {
		return canonicalPaperBacktestRequest{}, validationError("quantity must be positive")
	}

	return canonicalPaperBacktestRequest{
		runID:              runID,
		instrument:         strategyIdentity.Instrument,
		timeframe:          strategyIdentity.Timeframe,
		timeRange:          timeRange,
		strategyParameters: strategyParameters,
		governorPolicy:     governorPolicy,
		quantity:           request.Quantity,
	}, nil
}

func canonicalizeGovernorPolicy(policy governor.Policy) (governor.Policy, error) {
	if len(policy.AllowedActionKinds) == 0 {
		return governor.Policy{}, validationError("allowed action kinds are required")
	}

	canonicalAllowedActionKinds := make([]domain.CandidateActionKind, 0, len(policy.AllowedActionKinds))
	for _, actionKind := range policy.AllowedActionKinds {
		canonicalActionKind, err := domain.NewCandidateActionKind(actionKind.String())
		if err != nil {
			return governor.Policy{}, validationError(
				fmt.Sprintf("unsupported allowed action kind %q", actionKind),
			)
		}

		canonicalAllowedActionKinds = append(canonicalAllowedActionKinds, canonicalActionKind)
	}

	switch policy.MinimumQuality {
	case domain.DataQualityRaw, domain.DataQualityValidated:
		// accepted as-is after enum normalization below.
	case domain.DataQualitySuspect:
		return governor.Policy{}, validationError("unsupported minimum quality \"suspect\"")
	default:
		return governor.Policy{}, validationError(
			fmt.Sprintf("unsupported minimum quality %q", policy.MinimumQuality),
		)
	}

	minimumQuality, err := domain.NewDataQuality(policy.MinimumQuality.String())
	if err != nil {
		return governor.Policy{}, validationError(err.Error())
	}

	if policy.MaximumApprovedCount < 0 {
		return governor.Policy{}, validationError("maximum approved action count must be zero or greater")
	}

	return governor.Policy{
		AllowedActionKinds:   canonicalAllowedActionKinds,
		MinimumQuality:       minimumQuality,
		MaximumApprovedCount: policy.MaximumApprovedCount,
	}, nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

func flowStableID(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(hash[:16])
}
