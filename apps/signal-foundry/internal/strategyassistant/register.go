package strategyassistant

import (
	"context"
	"errors"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/runtime/agent"
	rtdata "github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	toolNameDataListCandleAvailability      = "sf_data_list_candle_availability"
	toolNameDataGetCandles                  = "sf_data_get_candles"
	toolNameDataGetCandleEvidence           = "sf_data_get_candle_evidence"
	toolNameJobsStartHistoricalDataBackfill = "sf_jobs_start_historical_data_backfill"
	toolNameJobsList                        = "sf_jobs_list"
	toolNameJobsGet                         = "sf_jobs_get"
	toolNameStrategyListVersions            = "sf_strategy_list_versions"
	toolNameStrategyGetVersion              = "sf_strategy_get_version"
	toolNameStrategyValidateDefinition      = "sf_strategy_validate_definition"
	toolNameStrategyDuplicateVersion        = "sf_strategy_duplicate_version"
	toolNameStrategyCreateVersion           = "sf_strategy_create_version"
	toolNameEvaluationRunBacktest           = "sf_evaluation_run_backtest"
	toolNameEvaluationListBacktests         = "sf_evaluation_list_backtests"
	toolNameEvaluationGetDetail             = "sf_evaluation_get_backtest_detail"
	toolNameEvaluationGetReport             = "sf_evaluation_get_backtest_report"
	toolNameEvaluationGetEvidence           = "sf_evaluation_get_backtest_evidence"
)

type RegisterDeps struct {
	Registry            *agent.ToolsRegistry
	DataRead            candleReadService
	DataLineage         candleLineageService
	JobsService         jobsService
	StrategyWorkspace   strategyWorkspaceService
	EvaluationWorkspace evaluationWorkspaceService
}

type candleReadService interface {
	ListCandleAvailability(
		ctx context.Context,
		query rtdata.CandleAvailabilityListQuery,
	) (rtdata.CandleAvailabilityListResult, error)
	ReplayCandles(
		ctx context.Context,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
	) ([]rtdata.ReplayCandle, error)
}

type candleLineageService interface {
	ListCandleLinkedRawPayloadMetadata(
		ctx context.Context,
		query rtdata.CandleLinkedRawPayloadsQuery,
	) ([]rtdata.RawPayloadMetadata, error)
}

type jobsService interface {
	CreateHistoricalRawCandleBackfill(
		ctx context.Context,
		params jobspkg.CreateHistoricalRawCandleBackfillParams,
	) (*jobspkg.Job, error)
	List(ctx context.Context, params jobspkg.ListParams) (jobspkg.ListResult, error)
	Get(ctx context.Context, jobID string) (*jobspkg.Job, error)
}

type strategyWorkspaceService interface {
	ValidateDefinition(
		ctx context.Context,
		definition app.StrategyDefinitionInput,
	) (app.StrategyValidationResult, error)
	CreateVersion(
		ctx context.Context,
		params app.CreateStrategyVersionParams,
	) (*app.StrategyVersionRecord, error)
	ListVersions(ctx context.Context) ([]app.StrategyVersionRecord, error)
	GetVersion(
		ctx context.Context,
		strategyID string,
		version string,
	) (*app.StrategyVersionRecord, error)
	DuplicateVersion(
		ctx context.Context,
		strategyID string,
		version string,
	) (*app.StrategyVersionCandidate, error)
}

type evaluationWorkspaceService interface {
	CreateEvaluation(
		ctx context.Context,
		params app.CreateEvaluationParams,
	) (*app.EvaluationDetail, error)
	ListEvaluations(
		ctx context.Context,
		params app.ListEvaluationsParams,
	) ([]app.EvaluationListItem, error)
	GetEvaluation(ctx context.Context, runID string) (*app.EvaluationDetail, error)
	GetEvaluationReport(ctx context.Context, runID string) (*app.EvaluationReportView, error)
	GetEvaluationEvidence(ctx context.Context, runID string) (*app.EvaluationEvidenceView, error)
}

func RegisterTools(deps RegisterDeps) error {
	if deps.Registry == nil {
		return errors.New("tools registry is required")
	}

	deps.Registry.AddTools(
		newListCandleAvailabilityTool(deps),
		newGetCandlesTool(deps),
		newGetCandleEvidenceTool(deps),
		newStartHistoricalDataBackfillTool(deps),
		newListJobsTool(deps),
		newGetJobTool(deps),
		newListStrategyVersionsTool(deps),
		newGetStrategyVersionTool(deps),
		newValidateStrategyDefinitionTool(deps),
		newDuplicateStrategyVersionTool(deps),
		newCreateStrategyVersionTool(deps),
		newRunBacktestTool(deps),
		newListBacktestsTool(deps),
		newGetBacktestDetailTool(deps),
		newGetBacktestReportTool(deps),
		newGetBacktestEvidenceTool(deps),
	)

	return nil
}

func internalAlphaBoundedDescription(summary string) string {
	return "Internal alpha only. Bounded product-service tool for strategy assistant chat. " + summary
}

func newListCandleAvailabilityTool(
	deps RegisterDeps,
) agent.ToolDef[ListCandleAvailabilityRequest, ListCandleAvailabilityResponse] {
	return agent.NewToolDef(
		toolNameDataListCandleAvailability,
		internalAlphaBoundedDescription(
			"Lists compact candle availability without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input ListCandleAvailabilityRequest) (ListCandleAvailabilityResponse, error) {
			return handleListCandleAvailabilityTool(ctx, deps, input)
		},
	)
}

func newGetCandlesTool(deps RegisterDeps) agent.ToolDef[GetCandlesRequest, GetCandlesResponse] {
	return agent.NewToolDef(
		toolNameDataGetCandles,
		internalAlphaBoundedDescription(
			"Returns deterministic candle slices for analysis without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input GetCandlesRequest) (GetCandlesResponse, error) {
			return handleGetCandlesTool(ctx, deps, input)
		},
	)
}

func newGetCandleEvidenceTool(deps RegisterDeps) agent.ToolDef[GetCandleEvidenceRequest, GetCandleEvidenceResponse] {
	return agent.NewToolDef(
		toolNameDataGetCandleEvidence,
		internalAlphaBoundedDescription(
			"Returns bounded candle-linked evidence metadata without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input GetCandleEvidenceRequest) (GetCandleEvidenceResponse, error) {
			return handleGetCandleEvidenceTool(ctx, deps, input)
		},
	)
}

func newStartHistoricalDataBackfillTool(
	deps RegisterDeps,
) agent.ToolDef[StartHistoricalDataBackfillRequest, StartHistoricalDataBackfillResponse] {
	return agent.NewToolDef(
		toolNameJobsStartHistoricalDataBackfill,
		internalAlphaBoundedDescription(
			"Starts bounded durable historical data backfill jobs without live trading, shell access, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input StartHistoricalDataBackfillRequest) (StartHistoricalDataBackfillResponse, error) {
			return handleStartHistoricalDataBackfillTool(ctx, deps, input)
		},
	)
}

func newListJobsTool(deps RegisterDeps) agent.ToolDef[ListJobsRequest, ListJobsResponse] {
	return agent.NewToolDef(
		toolNameJobsList,
		internalAlphaBoundedDescription(
			"Lists bounded durable historical data jobs without live trading, shell access, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input ListJobsRequest) (ListJobsResponse, error) {
			return handleListJobsTool(ctx, deps, input)
		},
	)
}

func newGetJobTool(deps RegisterDeps) agent.ToolDef[GetJobRequest, GetJobResponse] {
	return agent.NewToolDef(
		toolNameJobsGet,
		internalAlphaBoundedDescription(
			"Reads one bounded durable historical data job without live trading, shell access, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input GetJobRequest) (GetJobResponse, error) {
			return handleGetJobTool(ctx, deps, input)
		},
	)
}

func newListStrategyVersionsTool(
	deps RegisterDeps,
) agent.ToolDef[ListStrategyVersionsRequest, ListStrategyVersionsResponse] {
	return agent.NewToolDef(
		toolNameStrategyListVersions,
		internalAlphaBoundedDescription(
			"Lists compact saved strategy versions without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input ListStrategyVersionsRequest) (ListStrategyVersionsResponse, error) {
			return handleListStrategyVersionsTool(ctx, deps, input)
		},
	)
}

func newGetStrategyVersionTool(
	deps RegisterDeps,
) agent.ToolDef[GetStrategyVersionRequest, GetStrategyVersionResponse] {
	return agent.NewToolDef(
		toolNameStrategyGetVersion,
		internalAlphaBoundedDescription(
			"Reads one bounded saved strategy version detail without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input GetStrategyVersionRequest) (GetStrategyVersionResponse, error) {
			return handleGetStrategyVersionTool(ctx, deps, input)
		},
	)
}

func newValidateStrategyDefinitionTool(
	deps RegisterDeps,
) agent.ToolDef[ValidateStrategyDefinitionRequest, ValidateStrategyDefinitionResponse] {
	return agent.NewToolDef(
		toolNameStrategyValidateDefinition,
		internalAlphaBoundedDescription(
			"Validates bounded Strategy DSL inputs without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input ValidateStrategyDefinitionRequest) (ValidateStrategyDefinitionResponse, error) {
			return handleValidateStrategyDefinitionTool(ctx, deps, input)
		},
	)
}

func newDuplicateStrategyVersionTool(
	deps RegisterDeps,
) agent.ToolDef[DuplicateStrategyVersionRequest, DuplicateStrategyVersionResponse] {
	return agent.NewToolDef(
		toolNameStrategyDuplicateVersion,
		internalAlphaBoundedDescription(
			"Creates a bounded editable strategy candidate without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input DuplicateStrategyVersionRequest) (DuplicateStrategyVersionResponse, error) {
			return handleDuplicateStrategyVersionTool(ctx, deps, input)
		},
	)
}

func newCreateStrategyVersionTool(
	deps RegisterDeps,
) agent.ToolDef[CreateStrategyVersionRequest, CreateStrategyVersionResponse] {
	return agent.NewToolDef(
		toolNameStrategyCreateVersion,
		internalAlphaBoundedDescription(
			"Creates a bounded immutable strategy version without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input CreateStrategyVersionRequest) (CreateStrategyVersionResponse, error) {
			return handleCreateStrategyVersionTool(ctx, deps, input)
		},
	)
}

func newRunBacktestTool(deps RegisterDeps) agent.ToolDef[RunBacktestRequest, RunBacktestResponse] {
	return agent.NewToolDef(
		toolNameEvaluationRunBacktest,
		internalAlphaBoundedDescription(
			"Runs bounded deterministic backtests without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input RunBacktestRequest) (RunBacktestResponse, error) {
			return handleRunBacktestTool(ctx, deps, input)
		},
	)
}

func newListBacktestsTool(deps RegisterDeps) agent.ToolDef[ListBacktestsRequest, ListBacktestsResponse] {
	return agent.NewToolDef(
		toolNameEvaluationListBacktests,
		internalAlphaBoundedDescription(
			"Lists bounded evaluation history without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input ListBacktestsRequest) (ListBacktestsResponse, error) {
			return handleListBacktestsTool(ctx, deps, input)
		},
	)
}

func newGetBacktestDetailTool(deps RegisterDeps) agent.ToolDef[GetBacktestDetailRequest, GetBacktestDetailResponse] {
	return agent.NewToolDef(
		toolNameEvaluationGetDetail,
		internalAlphaBoundedDescription(
			"Reads bounded evaluation detail without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input GetBacktestDetailRequest) (GetBacktestDetailResponse, error) {
			return handleGetBacktestDetailTool(ctx, deps, input)
		},
	)
}

func newGetBacktestReportTool(deps RegisterDeps) agent.ToolDef[GetBacktestReportRequest, GetBacktestReportResponse] {
	return agent.NewToolDef(
		toolNameEvaluationGetReport,
		internalAlphaBoundedDescription(
			"Returns bounded evaluation summaries without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input GetBacktestReportRequest) (GetBacktestReportResponse, error) {
			return handleGetBacktestReportTool(ctx, deps, input)
		},
	)
}

func newGetBacktestEvidenceTool(
	deps RegisterDeps,
) agent.ToolDef[GetBacktestEvidenceRequest, GetBacktestEvidenceResponse] {
	return agent.NewToolDef(
		toolNameEvaluationGetEvidence,
		internalAlphaBoundedDescription(
			"Returns bounded evaluation evidence rows without live trading, manual orders, or raw SQL access.",
		),
		func(ctx *agent.ToolContext, input GetBacktestEvidenceRequest) (GetBacktestEvidenceResponse, error) {
			return handleGetBacktestEvidenceTool(ctx, deps, input)
		},
	)
}
