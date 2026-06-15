package flows

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/audit"
	"github.com/gemyago/signal-foundry/runtime/backtest"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/execution"
	"github.com/gemyago/signal-foundry/runtime/governor"
	"github.com/gemyago/signal-foundry/runtime/strategy"
)

const linkedZeroAssumptionValue = "zero"

type durableAuditRecorder interface {
	AuditRecorder
	UpdateTraceMetadata(
		ctx context.Context,
		traceID string,
		metadata map[string]string,
	) (domain.DecisionTrace, error)
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

type approvedIntentExecutor interface {
	ExecuteApprovedIntent(
		ctx context.Context,
		request execution.ExecuteApprovedIntentRequest,
	) (execution.ExecuteApprovedIntentResult, error)
}

type snapshotProjector interface {
	RecordPositionSnapshots(
		ctx context.Context,
		fills []domain.ExecutionFill,
	) ([]domain.PositionSnapshot, error)
	RecordPortfolioSnapshots(
		ctx context.Context,
		request execution.ProjectPortfolioSnapshotsRequest,
	) ([]domain.PortfolioSnapshot, error)
}

type backtestRecorder interface {
	CreateDatasetReference(
		ctx context.Context,
		reference domain.DatasetReference,
	) (domain.DatasetReference, error)
	CreateBacktestRun(ctx context.Context, run domain.BacktestRun) (domain.BacktestRun, error)
	StartBacktestRun(
		ctx context.Context,
		runID string,
		startedAt domain.BacktestRunTime,
	) (domain.BacktestRun, error)
	CompleteBacktestRun(
		ctx context.Context,
		request backtest.CompleteBacktestRunRequest,
	) (domain.BacktestRun, error)
	FailBacktestRun(
		ctx context.Context,
		request backtest.FailBacktestRunRequest,
	) (domain.BacktestRun, error)
	CreateEvaluationReport(
		ctx context.Context,
		request backtest.CreateEvaluationReportRequest,
	) (domain.EvaluationReport, error)
}

// DurableBacktestFlowDeps configures the linked durable backtest coordinator.
type DurableBacktestFlowDeps struct {
	CandleReplayReader  CandleReplayReader
	AnalyticsCalculator AnalyticsCalculator
	StrategyEvaluator   StrategyEvaluator
	AuditRecorder       durableAuditRecorder
	GovernorEvaluator   GovernorEvaluator
	PaperExecutor       approvedIntentExecutor
	SnapshotProjector   snapshotProjector
	BacktestRecorder    backtestRecorder
}

// DurableBacktestResult returns linked durable backtest records.
type DurableBacktestResult struct {
	RunID              string
	DatasetReference   domain.DatasetReference
	BacktestRun        domain.BacktestRun
	StrategyEvaluation strategy.EvaluateResult
	IntentContexts     []audit.IntentContext
	GovernorEvaluation governor.EvaluateResult
	PaperExecutions    []execution.ExecuteApprovedIntentResult
	PositionSnapshots  []domain.PositionSnapshot
	PortfolioSnapshots []domain.PortfolioSnapshot
	EvaluationReport   domain.EvaluationReport
}

// DurableBacktestFlow coordinates audit, execution, snapshot, and report linkage.
type DurableBacktestFlow struct {
	candleReplayReader  CandleReplayReader
	analyticsCalculator AnalyticsCalculator
	strategyEvaluator   StrategyEvaluator
	auditRecorder       durableAuditRecorder
	governorEvaluator   GovernorEvaluator
	paperExecutor       approvedIntentExecutor
	snapshotProjector   snapshotProjector
	backtestRecorder    backtestRecorder
}

// NewDurableBacktestFlow creates a linked durable backtest flow.
func NewDurableBacktestFlow(deps DurableBacktestFlowDeps) (*DurableBacktestFlow, error) {
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
	if deps.PaperExecutor == nil {
		return nil, errors.New("paper executor is required")
	}
	if deps.SnapshotProjector == nil {
		return nil, errors.New("snapshot projector is required")
	}
	if deps.BacktestRecorder == nil {
		return nil, errors.New("backtest recorder is required")
	}

	return &DurableBacktestFlow{
		candleReplayReader:  deps.CandleReplayReader,
		analyticsCalculator: deps.AnalyticsCalculator,
		strategyEvaluator:   deps.StrategyEvaluator,
		auditRecorder:       deps.AuditRecorder,
		governorEvaluator:   deps.GovernorEvaluator,
		paperExecutor:       deps.PaperExecutor,
		snapshotProjector:   deps.SnapshotProjector,
		backtestRecorder:    deps.BacktestRecorder,
	}, nil
}

// Run coordinates the durable backtest linkage path.
func (f *DurableBacktestFlow) Run(
	ctx context.Context,
	request PaperBacktestRequest,
) (DurableBacktestResult, error) {
	canonicalRequest, err := canonicalizePaperBacktestRequest(request)
	if err != nil {
		return DurableBacktestResult{}, err
	}
	if canonicalRequest.mode != domain.DecisionModeBacktest {
		return DurableBacktestResult{}, validationError("durable backtest flow mode must be backtest")
	}

	replayedCandles, err := f.candleReplayReader.ReplayCandles(
		ctx,
		canonicalRequest.instrument,
		canonicalRequest.timeframe,
		canonicalRequest.timeRange,
	)
	if err != nil {
		return DurableBacktestResult{}, fmt.Errorf("replay candles: %w", err)
	}

	datasetReference, err := buildDatasetReference(canonicalRequest, replayedCandles)
	if err != nil {
		return DurableBacktestResult{}, err
	}
	datasetReference, backtestRun, err := f.initializeBacktest(ctx, canonicalRequest, datasetReference)
	if err != nil {
		return DurableBacktestResult{}, err
	}

	strategyEvaluation, intentContexts, governorEvaluation, paperExecutions, fills, err := f.evaluateBacktest(
		ctx,
		canonicalRequest,
		replayedCandles,
		datasetReference,
	)
	if err != nil {
		return DurableBacktestResult{}, f.failBacktestRun(
			ctx,
			backtestRun.RunID.String(),
			err,
			canonicalRequest.timeRange.End.UTC(),
		)
	}

	intentContexts, positionSnapshots, portfolioSnapshots, report, err := f.projectAndReport(
		ctx,
		canonicalRequest,
		intentContexts,
		paperExecutions,
		backtestRun.RunID.String(),
		datasetReference.DatasetID.String(),
		governorEvaluation.Decisions,
		fills,
	)
	if err != nil {
		return DurableBacktestResult{}, f.failBacktestRun(
			ctx,
			backtestRun.RunID.String(),
			err,
			linkedReportTime(canonicalRequest, governorEvaluation.Decisions, portfolioSnapshots, fills),
		)
	}
	reportCreatedAt := report.CreatedAt.Time()

	backtestRun, err = f.backtestRecorder.CompleteBacktestRun(ctx, backtest.CompleteBacktestRunRequest{
		RunID:   backtestRun.RunID.String(),
		Metrics: report.Metrics,
		EndedAt: domain.BacktestRunTime(reportCreatedAt),
	})
	if err != nil {
		return DurableBacktestResult{}, f.failBacktestRun(
			ctx,
			backtestRun.RunID.String(),
			fmt.Errorf("complete backtest run: %w", err),
			reportCreatedAt,
		)
	}

	return DurableBacktestResult{
		RunID:              canonicalRequest.runID,
		DatasetReference:   datasetReference,
		BacktestRun:        backtestRun,
		StrategyEvaluation: strategyEvaluation,
		IntentContexts:     intentContexts,
		GovernorEvaluation: governorEvaluation,
		PaperExecutions:    paperExecutions,
		PositionSnapshots:  positionSnapshots,
		PortfolioSnapshots: portfolioSnapshots,
		EvaluationReport:   report,
	}, nil
}

func (f *DurableBacktestFlow) initializeBacktest(
	ctx context.Context,
	request canonicalPaperBacktestRequest,
	datasetReference domain.DatasetReference,
) (domain.DatasetReference, domain.BacktestRun, error) {
	persistedDataset, err := f.backtestRecorder.CreateDatasetReference(ctx, datasetReference)
	if err != nil {
		return domain.DatasetReference{}, domain.BacktestRun{}, fmt.Errorf("create dataset reference: %w", err)
	}

	backtestRun, err := buildBacktestRun(request, persistedDataset)
	if err != nil {
		return domain.DatasetReference{}, domain.BacktestRun{}, err
	}
	backtestRun, err = f.backtestRecorder.CreateBacktestRun(ctx, backtestRun)
	if err != nil {
		return domain.DatasetReference{}, domain.BacktestRun{}, fmt.Errorf("create backtest run: %w", err)
	}

	backtestRun, err = f.backtestRecorder.StartBacktestRun(
		ctx,
		backtestRun.RunID.String(),
		domain.BacktestRunTime(backtestRun.CreatedAt.Time()),
	)
	if err != nil {
		return domain.DatasetReference{}, domain.BacktestRun{}, fmt.Errorf("start backtest run: %w", err)
	}

	return persistedDataset, backtestRun, nil
}

func (f *DurableBacktestFlow) evaluateBacktest(
	ctx context.Context,
	request canonicalPaperBacktestRequest,
	replayedCandles []data.ReplayCandle,
	datasetReference domain.DatasetReference,
) (
	strategy.EvaluateResult,
	[]audit.IntentContext,
	governor.EvaluateResult,
	[]execution.ExecuteApprovedIntentResult,
	[]domain.ExecutionFill,
	error,
) {
	prepareFlow := PaperBacktestFlow{
		analyticsCalculator: f.analyticsCalculator,
		auditRecorder:       f.auditRecorder,
	}

	if err := prepareFlow.runAnalyticsStage(ctx, request); err != nil {
		return strategy.EvaluateResult{}, nil, governor.EvaluateResult{}, nil, nil, err
	}

	strategyEvaluation, err := f.strategyEvaluator.Evaluate(ctx, strategy.EvaluateRequest{
		Instrument:   request.instrument,
		Timeframe:    request.timeframe,
		TimeRange:    request.timeRange,
		StrategyKind: domain.StrategyKindMovingAverageCrossover,
		Parameters:   request.strategyParameters,
	})
	if err != nil {
		return strategy.EvaluateResult{}, nil, governor.EvaluateResult{}, nil, nil, fmt.Errorf(
			"evaluate strategy: %w",
			err,
		)
	}

	replayClosePrices := make(map[time.Time]float64, len(replayedCandles))
	for _, replayedCandle := range replayedCandles {
		replayClosePrices[replayedCandle.Candle.TimeRange.End.UTC()] = replayedCandle.Candle.Close
	}

	intentContexts, err := prepareLinkedIntentContexts(
		ctx,
		f.auditRecorder,
		request,
		strategyEvaluation.Actions,
		replayClosePrices,
		datasetReference,
	)
	if err != nil {
		return strategy.EvaluateResult{}, nil, governor.EvaluateResult{}, nil, nil, err
	}

	intentContexts, err = f.markIntentsSentToGovernor(ctx, intentContexts)
	if err != nil {
		return strategy.EvaluateResult{}, nil, governor.EvaluateResult{}, nil, nil, err
	}

	governorEvaluation, err := f.governorEvaluator.Evaluate(ctx, governor.EvaluateRequest{
		IntentInputs: buildGovernorIntentInputs(request, intentContexts),
		Policy:       request.governorPolicy,
	})
	if err != nil {
		return strategy.EvaluateResult{}, nil, governor.EvaluateResult{}, nil, nil, fmt.Errorf(
			"evaluate governor: %w",
			err,
		)
	}

	paperExecutions, fills, err := f.executeGovernorDecisions(
		ctx,
		intentContexts,
		governorEvaluation.Decisions,
		replayedCandles,
	)
	if err != nil {
		return strategy.EvaluateResult{}, nil, governor.EvaluateResult{}, nil, nil, err
	}

	return strategyEvaluation, intentContexts, governorEvaluation, paperExecutions, fills, nil
}

func (f *DurableBacktestFlow) markIntentsSentToGovernor(
	ctx context.Context,
	intentContexts []audit.IntentContext,
) ([]audit.IntentContext, error) {
	for idx := range intentContexts {
		updatedIntent, err := f.auditRecorder.UpdateOrderIntentStatus(
			ctx,
			string(intentContexts[idx].Intent.IntentID),
			domain.OrderIntentStatusSentToGovernor,
		)
		if err != nil {
			return nil, fmt.Errorf("update order intent %d status sent_to_governor: %w", idx, err)
		}
		intentContexts[idx].Intent = updatedIntent
	}

	return intentContexts, nil
}

func (f *DurableBacktestFlow) executeGovernorDecisions(
	ctx context.Context,
	intentContexts []audit.IntentContext,
	decisions []domain.GovernorDecision,
	replayedCandles []data.ReplayCandle,
) ([]execution.ExecuteApprovedIntentResult, []domain.ExecutionFill, error) {
	paperExecutions := make([]execution.ExecuteApprovedIntentResult, 0, len(decisions))
	fills := make([]domain.ExecutionFill, 0, len(decisions))

	for idx, decision := range decisions {
		var err error
		intentContexts[idx], err = f.writeAuditReferences(
			ctx,
			intentContexts[idx],
			map[string]string{
				"governor_decision_reference": governorDecisionReference(decision),
				"governor_decision_status":    decision.Status.String(),
				"governor_decision_reason":    decision.Reason.String(),
			},
		)
		if err != nil {
			return nil, nil, fmt.Errorf("write order intent %d governor references: %w", idx, err)
		}

		updatedIntent, err := f.auditRecorder.UpdateOrderIntentStatus(
			ctx,
			string(intentContexts[idx].Intent.IntentID),
			intentStatusForDecision(decision),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("update order intent %d status after governor: %w", idx, err)
		}
		intentContexts[idx].Intent = updatedIntent

		if decision.Status != domain.GovernorDecisionStatusApproved {
			continue
		}

		paperExecution, err := f.paperExecutor.ExecuteApprovedIntent(ctx, execution.ExecuteApprovedIntentRequest{
			Intent:           updatedIntent,
			ApprovedDecision: decision,
			ReplayCandles:    replayedCandles,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("execute approved intent %d: %w", idx, err)
		}
		paperExecutions = append(paperExecutions, paperExecution)

		intentContexts[idx], err = f.writeAuditReferences(
			ctx,
			intentContexts[idx],
			map[string]string{
				"execution_command_id": string(paperExecution.Command.CommandID),
				"execution_order_id":   string(paperExecution.Order.OrderID),
			},
		)
		if err != nil {
			return nil, nil, fmt.Errorf("write order intent %d execution references: %w", idx, err)
		}

		if paperExecution.Fill != nil {
			intentContexts[idx], err = f.writeAuditReferences(
				ctx,
				intentContexts[idx],
				map[string]string{
					"execution_fill_id": string(paperExecution.Fill.FillID),
				},
			)
			if err != nil {
				return nil, nil, fmt.Errorf("write order intent %d fill references: %w", idx, err)
			}
		}

		updatedIntent, err = f.auditRecorder.UpdateOrderIntentStatus(
			ctx,
			string(updatedIntent.IntentID),
			domain.OrderIntentStatusExecutionCreated,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("update order intent %d status execution_created: %w", idx, err)
		}
		intentContexts[idx].Intent = updatedIntent

		if paperExecution.Fill != nil {
			fills = append(fills, *paperExecution.Fill)
		}
	}

	return paperExecutions, fills, nil
}

func (f *DurableBacktestFlow) projectAndReport(
	ctx context.Context,
	request canonicalPaperBacktestRequest,
	intentContexts []audit.IntentContext,
	paperExecutions []execution.ExecuteApprovedIntentResult,
	runID string,
	datasetID string,
	decisions []domain.GovernorDecision,
	fills []domain.ExecutionFill,
) ([]audit.IntentContext, []domain.PositionSnapshot, []domain.PortfolioSnapshot, domain.EvaluationReport, error) {
	positionSnapshots, err := f.snapshotProjector.RecordPositionSnapshots(ctx, fills)
	if err != nil {
		return nil, nil, nil, domain.EvaluationReport{}, fmt.Errorf("record position snapshots: %w", err)
	}

	portfolioSnapshots, err := f.snapshotProjector.RecordPortfolioSnapshots(
		ctx,
		execution.ProjectPortfolioSnapshotsRequest{
			PositionSnapshots: positionSnapshots,
		},
	)
	if err != nil {
		return nil, nil, nil, domain.EvaluationReport{}, fmt.Errorf("record portfolio snapshots: %w", err)
	}

	reportCreatedAt := linkedReportTime(request, decisions, portfolioSnapshots, fills)
	report, err := f.backtestRecorder.CreateEvaluationReport(ctx, backtest.CreateEvaluationReportRequest{
		EvaluationID:         flowStableID("evaluation-report", request.runID),
		StrategyID:           request.strategyID,
		StrategyVersion:      request.strategyVersion,
		StrategyArtifactHash: request.strategyArtifactHash,
		BacktestRunID:        runID,
		DatasetID:            datasetID,
		Decision:             domain.EvaluationDecisionNeedsReview,
		FailureReasons:       collectGovernorFailureReasons(decisions),
		Notes:                "flow-linked durable backtest evidence",
		CreatedAt:            domain.EvaluationReportTime(reportCreatedAt),
		Fills:                fills,
		GovernorDecisions:    decisions,
		PortfolioSnapshots:   portfolioSnapshots,
	})
	if err != nil {
		return nil, nil, nil, domain.EvaluationReport{}, fmt.Errorf("create evaluation report: %w", err)
	}

	updatedIntentContexts, err := f.writeDownstreamAuditReferences(
		ctx,
		intentContexts,
		paperExecutions,
		positionSnapshots,
		portfolioSnapshots,
		report,
	)
	if err != nil {
		return nil, nil, nil, domain.EvaluationReport{}, err
	}

	return updatedIntentContexts, positionSnapshots, portfolioSnapshots, report, nil
}

func (f *DurableBacktestFlow) failBacktestRun(
	ctx context.Context,
	runID string,
	cause error,
	endedAt time.Time,
) error {
	_, failErr := f.backtestRecorder.FailBacktestRun(ctx, backtest.FailBacktestRunRequest{
		RunID:          runID,
		FailureReason:  "flow-linkage-error",
		FailureDetails: cause.Error(),
		EndedAt:        domain.BacktestRunTime(endedAt.UTC()),
	})
	if failErr != nil {
		return fmt.Errorf("%w (fail backtest run: %s)", cause, failErr.Error())
	}

	return cause
}

func prepareLinkedIntentContexts(
	ctx context.Context,
	auditRecorder durableAuditRecorder,
	request canonicalPaperBacktestRequest,
	actions []domain.CandidateAction,
	replayClosePrices map[time.Time]float64,
	datasetReference domain.DatasetReference,
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
		traceToRecord, err = linkTrace(traceToRecord, datasetReference)
		if err != nil {
			return nil, fmt.Errorf("link decision trace %d: %w", idx, err)
		}

		trace, err := auditRecorder.RecordTrace(ctx, traceToRecord)
		if err != nil {
			return nil, fmt.Errorf("record decision trace %d: %w", idx, err)
		}

		intentToCreate, err := buildOrderIntent(request, action, trace, limitPrice, idx)
		if err != nil {
			return nil, fmt.Errorf("build order intent %d: %w", idx, err)
		}
		intentToCreate, err = linkIntent(intentToCreate, datasetReference)
		if err != nil {
			return nil, fmt.Errorf("link order intent %d: %w", idx, err)
		}

		intent, err := auditRecorder.CreateOrderIntent(ctx, intentToCreate)
		if err != nil {
			return nil, fmt.Errorf("create order intent %d: %w", idx, err)
		}

		traceMetadata := copyMetadata(trace.Metadata)
		traceMetadata["intent_id"] = string(intent.IntentID)
		trace, err = auditRecorder.UpdateTraceMetadata(ctx, string(trace.TraceID), traceMetadata)
		if err != nil {
			return nil, fmt.Errorf("update decision trace %d intent reference: %w", idx, err)
		}

		contexts = append(contexts, audit.IntentContext{
			Trace:           trace,
			Intent:          intent,
			CandidateAction: action,
		})
	}

	return contexts, nil
}

func buildDatasetReference(
	request canonicalPaperBacktestRequest,
	replayedCandles []data.ReplayCandle,
) (domain.DatasetReference, error) {
	createdAt := request.timeRange.End.UTC()
	if len(replayedCandles) > 0 {
		createdAt = replayedCandles[len(replayedCandles)-1].Candle.TimeRange.End.UTC()
	}
	replayChecksum := replayChecksum(request, replayedCandles)

	reference, err := domain.NewDatasetReference(domain.DatasetReferenceParams{
		DatasetID:      flowStableID("dataset", replayChecksum),
		EntityTypes:    []string{"candles"},
		Instruments:    []domain.Instrument{request.instrument},
		Timeframes:     []domain.Timeframe{request.timeframe},
		TimeRange:      request.timeRange,
		ReplayChecksum: replayChecksum,
		CreatedAt:      createdAt,
		Metadata:       map[string]string{},
	})
	if err != nil {
		return domain.DatasetReference{}, validationError(err.Error())
	}

	return reference, nil
}

func buildBacktestRun(
	request canonicalPaperBacktestRequest,
	datasetReference domain.DatasetReference,
) (domain.BacktestRun, error) {
	run, err := domain.NewBacktestRun(domain.BacktestRunParams{
		RunID:                 request.runID,
		StrategyID:            request.strategyID,
		StrategyVersion:       request.strategyVersion,
		StrategyArtifactHash:  request.strategyArtifactHash,
		DatasetID:             datasetReference.DatasetID.String(),
		GovernorPolicyID:      flowStableID("governor-policy", request.runID),
		GovernorPolicyVersion: "v0",
		GovernorPolicyHash:    flowStableID("governor-policy-hash", request.runID),
		Mode:                  request.mode,
		TestedRange:           request.timeRange,
		FeeAssumptions: map[string]string{
			"fee_model": linkedZeroAssumptionValue,
		},
		SlippageAssumptions: map[string]string{
			"slippage_model": linkedZeroAssumptionValue,
		},
		ExecutionSimulatorVersion: "closed-candle-limit-v0",
		Status:                    domain.BacktestRunStatusPending,
		CreatedAt:                 request.timeRange.Start.UTC(),
		UpdatedAt:                 request.timeRange.Start.UTC(),
	})
	if err != nil {
		return domain.BacktestRun{}, validationError(err.Error())
	}

	return run, nil
}

func linkTrace(
	trace domain.DecisionTrace,
	datasetReference domain.DatasetReference,
) (domain.DecisionTrace, error) {
	metadata := copyMetadata(trace.Metadata)
	metadata["dataset_id"] = datasetReference.DatasetID.String()
	metadata["backtest_run_id"] = trace.RunReference

	linked, err := domain.NewDecisionTrace(domain.DecisionTraceParams{
		TraceID:              string(trace.TraceID),
		Mode:                 trace.Mode,
		DecisionTime:         trace.DecisionTime.Time(),
		StrategyID:           trace.StrategyID,
		StrategyVersion:      trace.StrategyVersion,
		StrategyArtifactHash: trace.StrategyArtifactHash,
		Instrument:           trace.Instrument,
		Timeframe:            trace.Timeframe,
		DatasetReference:     datasetReference.DatasetID.String(),
		RunReference:         trace.RunReference,
		InputRange:           trace.InputRange,
		AnalyticsReference:   trace.AnalyticsReference,
		DataQuality:          trace.DataQuality,
		EvaluatorName:        trace.EvaluatorName,
		EvaluatorVersion:     trace.EvaluatorVersion,
		Result:               trace.Result,
		ReasonCodes:          trace.ReasonCodes,
		Metadata:             metadata,
	})
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	return linked, nil
}

func linkIntent(
	intent domain.OrderIntent,
	datasetReference domain.DatasetReference,
) (domain.OrderIntent, error) {
	metadata := copyMetadata(intent.Metadata)
	metadata["dataset_id"] = datasetReference.DatasetID.String()
	metadata["backtest_run_id"] = metadata["run_id"]

	linked, err := domain.NewOrderIntent(domain.OrderIntentParams{
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
		Metadata:                 metadata,
	})
	if err != nil {
		return domain.OrderIntent{}, err
	}

	return linked, nil
}

func intentStatusForDecision(decision domain.GovernorDecision) domain.OrderIntentStatus {
	switch decision.Status {
	case domain.GovernorDecisionStatusApproved:
		return domain.OrderIntentStatusApproved
	case domain.GovernorDecisionStatusRejected:
		return domain.OrderIntentStatusRejected
	case domain.GovernorDecisionStatusBlocked:
		return domain.OrderIntentStatusBlocked
	}

	return domain.OrderIntentStatusBlocked
}

func collectGovernorFailureReasons(decisions []domain.GovernorDecision) []string {
	reasons := make([]string, 0, len(decisions))
	seen := map[string]struct{}{}
	for _, decision := range decisions {
		switch decision.Status {
		case domain.GovernorDecisionStatusApproved:
			continue
		case domain.GovernorDecisionStatusRejected, domain.GovernorDecisionStatusBlocked:
		}
		reason := decision.Reason.String()
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		reasons = append(reasons, reason)
	}

	return reasons
}

func replayChecksum(
	request canonicalPaperBacktestRequest,
	replayedCandles []data.ReplayCandle,
) string {
	parts := []string{
		"replay-checksum",
		request.instrument.Venue.String(),
		request.instrument.Symbol.String(),
		request.instrument.AssetClass.String(),
		request.timeframe.String(),
		request.timeRange.Start.UTC().Format(time.RFC3339Nano),
		request.timeRange.End.UTC().Format(time.RFC3339Nano),
	}
	for _, replayedCandle := range replayedCandles {
		parts = append(parts,
			strconv.FormatUint(replayedCandle.Identity, 10),
			replayedCandle.Candle.TimeRange.End.UTC().Format(time.RFC3339Nano),
			strconv.FormatFloat(replayedCandle.Candle.Open, 'g', -1, 64),
			strconv.FormatFloat(replayedCandle.Candle.High, 'g', -1, 64),
			strconv.FormatFloat(replayedCandle.Candle.Low, 'g', -1, 64),
			strconv.FormatFloat(replayedCandle.Candle.Close, 'g', -1, 64),
		)
	}

	return flowStableID(parts...)
}

func (f *DurableBacktestFlow) writeDownstreamAuditReferences(
	ctx context.Context,
	intentContexts []audit.IntentContext,
	paperExecutions []execution.ExecuteApprovedIntentResult,
	positionSnapshots []domain.PositionSnapshot,
	portfolioSnapshots []domain.PortfolioSnapshot,
	report domain.EvaluationReport,
) ([]audit.IntentContext, error) {
	contextsByIntentID := indexAuditContextsByIntentID(intentContexts)
	positionSnapshotsByFillID := indexPositionSnapshotsByFillID(positionSnapshots)
	portfolioSnapshotsByFillID := indexPortfolioSnapshotsByFillID(portfolioSnapshots)

	for _, paperExecution := range paperExecutions {
		intentContext, ok := contextsByIntentID[string(paperExecution.Command.IntentID)]
		if !ok {
			continue
		}

		references := snapshotAndReportReferences(
			paperExecution,
			positionSnapshotsByFillID,
			portfolioSnapshotsByFillID,
			report,
		)

		updatedIntentContext, err := f.writeAuditReferences(ctx, intentContext, references)
		if err != nil {
			return nil, fmt.Errorf(
				"write order intent %s snapshot/report references: %w",
				intentContext.Intent.IntentID,
				err,
			)
		}
		contextsByIntentID[string(intentContext.Intent.IntentID)] = updatedIntentContext
	}

	updatedIntentContexts := make([]audit.IntentContext, 0, len(intentContexts))
	for _, intentContext := range intentContexts {
		updatedIntentContext, ok := contextsByIntentID[string(intentContext.Intent.IntentID)]
		if !ok {
			updatedIntentContext = intentContext
		}
		if updatedIntentContext.Intent.IntentID != "" {
			updated, err := f.writeAuditReferences(ctx, updatedIntentContext, map[string]string{
				"evaluation_report_id": report.EvaluationID.String(),
			})
			if err != nil {
				return nil, fmt.Errorf(
					"write order intent %s evaluation report reference: %w",
					updatedIntentContext.Intent.IntentID,
					err,
				)
			}
			updatedIntentContext = updated
		}
		updatedIntentContexts = append(updatedIntentContexts, updatedIntentContext)
	}

	return updatedIntentContexts, nil
}

func indexAuditContextsByIntentID(
	intentContexts []audit.IntentContext,
) map[string]audit.IntentContext {
	contextsByIntentID := make(map[string]audit.IntentContext, len(intentContexts))
	for _, intentContext := range intentContexts {
		contextsByIntentID[string(intentContext.Intent.IntentID)] = intentContext
	}

	return contextsByIntentID
}

func indexPositionSnapshotsByFillID(
	positionSnapshots []domain.PositionSnapshot,
) map[string]domain.PositionSnapshot {
	positionSnapshotsByFillID := make(map[string]domain.PositionSnapshot, len(positionSnapshots))
	for _, snapshot := range positionSnapshots {
		positionSnapshotsByFillID[string(snapshot.SourceFillID)] = snapshot
	}

	return positionSnapshotsByFillID
}

func indexPortfolioSnapshotsByFillID(
	portfolioSnapshots []domain.PortfolioSnapshot,
) map[string]domain.PortfolioSnapshot {
	portfolioSnapshotsByFillID := make(map[string]domain.PortfolioSnapshot, len(portfolioSnapshots))
	for _, snapshot := range portfolioSnapshots {
		portfolioSnapshotsByFillID[string(snapshot.SourceFillID)] = snapshot
	}

	return portfolioSnapshotsByFillID
}

func snapshotAndReportReferences(
	paperExecution execution.ExecuteApprovedIntentResult,
	positionSnapshotsByFillID map[string]domain.PositionSnapshot,
	portfolioSnapshotsByFillID map[string]domain.PortfolioSnapshot,
	report domain.EvaluationReport,
) map[string]string {
	references := map[string]string{
		"evaluation_report_id": report.EvaluationID.String(),
	}
	if paperExecution.Fill == nil {
		return references
	}

	fillID := string(paperExecution.Fill.FillID)
	positionSnapshot, hasPositionSnapshot := positionSnapshotsByFillID[fillID]
	if hasPositionSnapshot {
		references["position_snapshot_id"] = positionSnapshot.SnapshotID.String()
	}
	portfolioSnapshot, hasPortfolioSnapshot := portfolioSnapshotsByFillID[fillID]
	if hasPortfolioSnapshot {
		references["portfolio_snapshot_id"] = portfolioSnapshot.SnapshotID.String()
	}

	return references
}

func (f *DurableBacktestFlow) writeAuditReferences(
	ctx context.Context,
	intentContext audit.IntentContext,
	references map[string]string,
) (audit.IntentContext, error) {
	traceMetadata := mergeAuditMetadata(intentContext.Trace.Metadata, references)
	trace, err := f.auditRecorder.UpdateTraceMetadata(
		ctx,
		string(intentContext.Trace.TraceID),
		traceMetadata,
	)
	if err != nil {
		return audit.IntentContext{}, err
	}

	intentMetadata := mergeAuditMetadata(intentContext.Intent.Metadata, references)
	intent, err := f.auditRecorder.UpdateOrderIntentMetadata(
		ctx,
		string(intentContext.Intent.IntentID),
		intentMetadata,
	)
	if err != nil {
		return audit.IntentContext{}, err
	}

	intentContext.Trace = trace
	intentContext.Intent = intent

	return intentContext, nil
}

func mergeAuditMetadata(base map[string]string, additions map[string]string) map[string]string {
	merged := copyMetadata(base)
	for key, value := range additions {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		merged[key] = trimmedValue
	}

	return merged
}

func governorDecisionReference(decision domain.GovernorDecision) string {
	action := decision.CandidateAction
	strategyIdentity := action.Strategy
	instrument := strategyIdentity.Instrument
	inputRange := action.InputRange

	return flowStableID(
		"governor-decision",
		strings.Join([]string{
			instrument.Venue.String(),
			instrument.Symbol.String(),
			instrument.AssetClass.String(),
			strconv.FormatBool(instrument.Active),
			strategyIdentity.Timeframe.String(),
			strategyIdentity.Kind.String(),
			action.Kind.String(),
			action.DecisionTime.Time().UTC().Format(time.RFC3339Nano),
			inputRange.Start.UTC().Format(time.RFC3339Nano),
			inputRange.End.UTC().Format(time.RFC3339Nano),
			action.Quality.String(),
			decision.Status.String(),
			decision.Reason.String(),
			decision.DecisionTime.Time().UTC().Format(time.RFC3339Nano),
		}, "|"),
	)
}

func linkedReportTime(
	request canonicalPaperBacktestRequest,
	decisions []domain.GovernorDecision,
	portfolioSnapshots []domain.PortfolioSnapshot,
	fills []domain.ExecutionFill,
) time.Time {
	latest := request.timeRange.End.UTC()
	for _, decision := range decisions {
		if decision.DecisionTime.Time().After(latest) {
			latest = decision.DecisionTime.Time()
		}
	}
	for _, fill := range fills {
		if fill.EventTime.Time().After(latest) {
			latest = fill.EventTime.Time()
		}
	}
	for _, snapshot := range portfolioSnapshots {
		if snapshot.EventTime.Time().After(latest) {
			latest = snapshot.EventTime.Time()
		}
	}

	return latest.UTC()
}

func copyMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return map[string]string{}
	}
	clone := make(map[string]string, len(metadata))
	maps.Copy(clone, metadata)

	return clone
}
