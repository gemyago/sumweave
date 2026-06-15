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
	"github.com/gemyago/signal-foundry/runtime/audit"
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

// AuditRecorder persists durable trace and order intent records for the flow.
type AuditRecorder interface {
	RecordTrace(ctx context.Context, trace domain.DecisionTrace) (domain.DecisionTrace, error)
	CreateOrderIntent(ctx context.Context, intent domain.OrderIntent) (domain.OrderIntent, error)
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
	AuditRecorder       AuditRecorder
	GovernorEvaluator   GovernorEvaluator
	ExecutionRecorder   ExecutionRecorder
}

// PaperBacktestRequest defines one deterministic paper backtest run.
type PaperBacktestRequest struct {
	RunID                string
	Mode                 domain.DecisionMode
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	Instrument           domain.Instrument
	Timeframe            domain.Timeframe
	TimeRange            domain.TimeRange
	StrategyParameters   strategy.MovingAverageCrossoverParams
	GovernorPolicy       governor.Policy
	Quantity             float64
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
	IntentContexts     []audit.IntentContext
	GovernorEvaluation governor.EvaluateResult
	PaperExecutions    []PaperExecutionResult
}

// PaperBacktestFlow keeps deterministic paper backtest orchestration thin.
type PaperBacktestFlow struct {
	candleReplayReader  CandleReplayReader
	analyticsCalculator AnalyticsCalculator
	strategyEvaluator   StrategyEvaluator
	auditRecorder       AuditRecorder
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
	if deps.AuditRecorder == nil {
		return nil, errors.New("audit recorder is required")
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
		auditRecorder:       deps.AuditRecorder,
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

	replayClosePrices := make(map[time.Time]float64, len(replayedCandles))
	for _, replayedCandle := range replayedCandles {
		replayClosePrices[replayedCandle.Candle.TimeRange.End.UTC()] = replayedCandle.Candle.Close
	}

	intentContexts, err := f.prepareIntentContexts(
		ctx,
		canonicalRequest,
		strategyEvaluation.Actions,
		replayClosePrices,
	)
	if err != nil {
		return PaperBacktestResult{}, err
	}

	governorEvaluation, err := f.governorEvaluator.Evaluate(ctx, governor.EvaluateRequest{
		IntentInputs: buildGovernorIntentInputs(canonicalRequest, intentContexts),
		Policy:       canonicalRequest.governorPolicy,
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
		IntentContexts:     intentContexts,
		GovernorEvaluation: governorEvaluation,
		PaperExecutions:    paperExecutions,
	}, nil
}

func (f *PaperBacktestFlow) prepareIntentContexts(
	ctx context.Context,
	request canonicalPaperBacktestRequest,
	actions []domain.CandidateAction,
	replayClosePrices map[time.Time]float64,
) ([]audit.IntentContext, error) {
	contexts := make([]audit.IntentContext, 0, len(actions))

	for idx, action := range actions {
		decisionTime := action.DecisionTime.Time().UTC()
		limitPrice, ok := replayClosePrices[decisionTime]
		if !ok {
			return nil, fmt.Errorf(
				"prepare order intent %d limit price: replay candle close price is required at decision time",
				idx,
			)
		}

		traceToRecord, err := buildDecisionTrace(request, action, idx)
		if err != nil {
			return nil, fmt.Errorf("build decision trace %d: %w", idx, err)
		}

		trace, err := f.auditRecorder.RecordTrace(ctx, traceToRecord)
		if err != nil {
			return nil, fmt.Errorf("record decision trace %d: %w", idx, err)
		}

		intentToCreate, err := buildOrderIntent(
			request,
			action,
			trace,
			limitPrice,
			idx,
		)
		if err != nil {
			return nil, fmt.Errorf("build order intent %d: %w", idx, err)
		}

		intent, err := f.auditRecorder.CreateOrderIntent(ctx, intentToCreate)
		if err != nil {
			return nil, fmt.Errorf("create order intent %d: %w", idx, err)
		}

		contexts = append(contexts, audit.IntentContext{
			Trace:           trace,
			Intent:          intent,
			CandidateAction: action,
		})
	}

	return contexts, nil
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
	runID                string
	mode                 domain.DecisionMode
	strategyID           string
	strategyVersion      string
	strategyArtifactHash string
	instrument           domain.Instrument
	timeframe            domain.Timeframe
	timeRange            domain.TimeRange
	strategyParameters   strategy.MovingAverageCrossoverParams
	governorPolicy       governor.Policy
	quantity             float64
}

func canonicalizePaperBacktestRequest(
	request PaperBacktestRequest,
) (canonicalPaperBacktestRequest, error) {
	runID := strings.TrimSpace(request.RunID)
	if runID == "" {
		return canonicalPaperBacktestRequest{}, validationError("run id is required")
	}

	mode, err := domain.NewDecisionMode(request.Mode.String())
	if err != nil {
		return canonicalPaperBacktestRequest{}, validationError(err.Error())
	}

	strategyID := strings.TrimSpace(request.StrategyID)
	if strategyID == "" {
		return canonicalPaperBacktestRequest{}, validationError("strategy id is required")
	}
	strategyVersion := strings.TrimSpace(request.StrategyVersion)
	if strategyVersion == "" {
		return canonicalPaperBacktestRequest{}, validationError("strategy version is required")
	}
	strategyArtifactHash := strings.TrimSpace(request.StrategyArtifactHash)
	if strategyArtifactHash == "" {
		return canonicalPaperBacktestRequest{}, validationError("strategy artifact hash is required")
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
		runID:                runID,
		mode:                 mode,
		strategyID:           strategyID,
		strategyVersion:      strategyVersion,
		strategyArtifactHash: strategyArtifactHash,
		instrument:           strategyIdentity.Instrument,
		timeframe:            strategyIdentity.Timeframe,
		timeRange:            timeRange,
		strategyParameters:   strategyParameters,
		governorPolicy:       governorPolicy,
		quantity:             request.Quantity,
	}, nil
}

func canonicalizeGovernorPolicy(policy governor.Policy) (governor.Policy, error) {
	canonicalAllowedModes, err := canonicalizeGovernorModes(policy.AllowedModes)
	if err != nil {
		return governor.Policy{}, err
	}

	canonicalAllowedVenues, err := canonicalizeGovernorVenues(policy.AllowedVenues)
	if err != nil {
		return governor.Policy{}, err
	}

	canonicalAllowedInstruments, err := canonicalizeGovernorInstruments(policy.AllowedInstruments)
	if err != nil {
		return governor.Policy{}, err
	}

	canonicalAllowedStrategyIDs, err := canonicalizeGovernorStrategyIDs(policy.AllowedStrategyIDs)
	if err != nil {
		return governor.Policy{}, err
	}

	canonicalAllowedActionKinds, err := canonicalizeGovernorActionKinds(policy.AllowedActionKinds)
	if err != nil {
		return governor.Policy{}, err
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

	if thresholdErr := validateGovernorPolicyThresholds(policy); thresholdErr != nil {
		return governor.Policy{}, thresholdErr
	}

	return governor.Policy{
		AllowedModes:                      canonicalAllowedModes,
		AllowedVenues:                     canonicalAllowedVenues,
		AllowedInstruments:                canonicalAllowedInstruments,
		AllowedStrategyIDs:                canonicalAllowedStrategyIDs,
		AllowedActionKinds:                canonicalAllowedActionKinds,
		MinimumQuality:                    minimumQuality,
		BlockNewRisk:                      policy.BlockNewRisk,
		MaximumOrderNotional:              policy.MaximumOrderNotional,
		MaximumStrategyExposureNotional:   policy.MaximumStrategyExposureNotional,
		MaximumInstrumentExposureNotional: policy.MaximumInstrumentExposureNotional,
		MaximumApprovedCount:              policy.MaximumApprovedCount,
	}, nil
}

func canonicalizeGovernorModes(modes []domain.DecisionMode) ([]domain.DecisionMode, error) {
	canonicalModes := make([]domain.DecisionMode, 0, len(modes))
	for _, mode := range modes {
		canonicalMode, err := domain.NewDecisionMode(mode.String())
		if err != nil {
			return nil, validationError(fmt.Sprintf("unsupported allowed mode %q", mode))
		}

		canonicalModes = append(canonicalModes, canonicalMode)
	}

	return canonicalModes, nil
}

func canonicalizeGovernorVenues(venues []domain.Venue) ([]domain.Venue, error) {
	canonicalVenues := make([]domain.Venue, 0, len(venues))
	for _, venue := range venues {
		canonicalVenue, err := domain.NewVenue(venue.String())
		if err != nil {
			return nil, validationError(fmt.Sprintf("unsupported allowed venue %q", venue))
		}

		canonicalVenues = append(canonicalVenues, canonicalVenue)
	}

	return canonicalVenues, nil
}

func canonicalizeGovernorInstruments(instruments []domain.Instrument) ([]domain.Instrument, error) {
	canonicalInstruments := make([]domain.Instrument, 0, len(instruments))
	for _, instrument := range instruments {
		canonicalInstrument, err := domain.NewInstrument(domain.InstrumentParams(instrument))
		if err != nil {
			return nil, validationError(
				fmt.Sprintf("unsupported allowed instrument %q/%q", instrument.Venue, instrument.Symbol),
			)
		}

		canonicalInstruments = append(canonicalInstruments, canonicalInstrument)
	}

	return canonicalInstruments, nil
}

func canonicalizeGovernorStrategyIDs(strategyIDs []string) ([]string, error) {
	canonicalStrategyIDs := make([]string, 0, len(strategyIDs))
	for _, strategyID := range strategyIDs {
		normalizedStrategyID := strings.TrimSpace(strategyID)
		if normalizedStrategyID == "" {
			return nil, validationError("allowed strategy ids must not be empty")
		}

		canonicalStrategyIDs = append(canonicalStrategyIDs, normalizedStrategyID)
	}

	return canonicalStrategyIDs, nil
}

func canonicalizeGovernorActionKinds(
	actionKinds []domain.CandidateActionKind,
) ([]domain.CandidateActionKind, error) {
	if len(actionKinds) == 0 {
		return nil, validationError("allowed action kinds are required")
	}

	canonicalActionKinds := make([]domain.CandidateActionKind, 0, len(actionKinds))
	for _, actionKind := range actionKinds {
		canonicalActionKind, err := domain.NewCandidateActionKind(actionKind.String())
		if err != nil {
			return nil, validationError(
				fmt.Sprintf("unsupported allowed action kind %q", actionKind),
			)
		}

		canonicalActionKinds = append(canonicalActionKinds, canonicalActionKind)
	}

	return canonicalActionKinds, nil
}

func validateGovernorPolicyThresholds(policy governor.Policy) error {
	if policy.MaximumApprovedCount < 0 {
		return validationError("maximum approved action count must be zero or greater")
	}
	if !isNonNegativeFinite(policy.MaximumOrderNotional) {
		return validationError("maximum order notional must be finite and zero or greater")
	}
	if !isNonNegativeFinite(policy.MaximumStrategyExposureNotional) {
		return validationError(
			"maximum strategy exposure notional must be finite and zero or greater",
		)
	}
	if !isNonNegativeFinite(policy.MaximumInstrumentExposureNotional) {
		return validationError(
			"maximum instrument exposure notional must be finite and zero or greater",
		)
	}

	return nil
}

func isNonNegativeFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

func flowStableID(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(hash[:16])
}

func buildDecisionTrace(
	request canonicalPaperBacktestRequest,
	action domain.CandidateAction,
	index int,
) (domain.DecisionTrace, error) {
	trace, err := domain.NewDecisionTrace(domain.DecisionTraceParams{
		TraceID:              flowStableID("trace", request.runID, strconv.Itoa(index)),
		Mode:                 request.mode,
		DecisionTime:         action.DecisionTime.Time(),
		StrategyID:           request.strategyID,
		StrategyVersion:      request.strategyVersion,
		StrategyArtifactHash: request.strategyArtifactHash,
		Instrument:           request.instrument,
		Timeframe:            request.timeframe,
		RunReference:         request.runID,
		InputRange:           action.InputRange,
		AnalyticsReference:   flowStableID("analytics", request.runID, strconv.Itoa(index)),
		DataQuality:          action.Quality,
		EvaluatorName:        "paper-backtest-flow",
		EvaluatorVersion:     "v0",
		Result:               domain.DecisionTraceResultIntentCreated,
		ReasonCodes:          []string{"OK"},
		Metadata: map[string]string{
			"action_kind": action.Kind.String(),
		},
	})
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	return trace, nil
}

func buildOrderIntent(
	request canonicalPaperBacktestRequest,
	action domain.CandidateAction,
	trace domain.DecisionTrace,
	limitPrice float64,
	index int,
) (domain.OrderIntent, error) {
	intent, err := domain.NewOrderIntent(domain.OrderIntentParams{
		IntentID:                 flowStableID("intent", request.runID, strconv.Itoa(index)),
		TraceID:                  string(trace.TraceID),
		StrategyID:               request.strategyID,
		StrategyVersion:          request.strategyVersion,
		StrategyArtifactHash:     request.strategyArtifactHash,
		Mode:                     request.mode,
		Instrument:               request.instrument,
		Timeframe:                request.timeframe,
		ActionKind:               action.Kind,
		OrderType:                domain.OrderTypeLimit,
		RequestedQuantity:        request.quantity,
		RequestedNotional:        request.quantity * limitPrice,
		RequestedLimitPrice:      &limitPrice,
		SourceReasonCode:         "OK",
		CandidateActionReference: flowStableID("candidate-action", request.runID, strconv.Itoa(index)),
		CreatedTime:              action.DecisionTime.Time(),
		Status:                   domain.OrderIntentStatusCreated,
		Metadata: map[string]string{
			"run_id": request.runID,
		},
	})
	if err != nil {
		return domain.OrderIntent{}, err
	}

	return intent, nil
}

func buildGovernorIntentInputs(
	request canonicalPaperBacktestRequest,
	contexts []audit.IntentContext,
) []governor.IntentInput {
	inputs := make([]governor.IntentInput, 0, len(contexts))
	policyID := flowStableID("governor-policy", request.runID)
	policyVersion := "v0"
	policyHash := flowStableID("governor-policy-hash", request.runID)

	for _, intentContext := range contexts {
		inputs = append(inputs, governor.IntentInput{
			CandidateAction:                   intentContext.CandidateAction,
			Intent:                            intentContext.Intent,
			CurrentStrategyExposureNotional:   0,
			CurrentInstrumentExposureNotional: 0,
			GovernorPolicyID:                  policyID,
			GovernorPolicyVersion:             policyVersion,
			GovernorPolicyHash:                policyHash,
		})
	}

	return inputs
}
