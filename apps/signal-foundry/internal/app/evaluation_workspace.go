package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/runtime/audit"
	"github.com/gemyago/signal-foundry/runtime/backtest"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/execution"
	"github.com/gemyago/signal-foundry/runtime/flows"
	rtgovernor "github.com/gemyago/signal-foundry/runtime/governor"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"go.uber.org/dig"
)

const (
	defaultEvaluationEvidenceLimit      = 200
	evaluationRequestSourceTypeHuman    = "human"
	evaluationFailureReasonDataMissing  = "replay-data-unavailable"
	defaultGovernorPolicyVersion        = "v0"
	defaultGovernorPolicyArtifactIDName = "default-paper-governor-policy"
	zeroAssumptionModel                 = "zero"
	feeModelKey                         = "fee_model"
	slippageModelKey                    = "slippage_model"
	defaultExecutionSimulatorVersion    = "closed-candle-limit-v0"
	defaultGovernorPolicyPayloadText    = `{"mode":"paper","allowedActionKinds":["long","short"],"minimumQuality":"raw","maximumApprovedCount":50}`
)

type evaluationStrategyRegistry interface {
	GetVersion(ctx context.Context, strategyID string, version string) (*rtstrategy.Version, error)
}

type evaluationGovernorPolicyStore interface {
	CreateWithActivate(ctx context.Context, raw []byte) (rtgovernor.Artifact, error)
	Get(ctx context.Context, hash string) (*rtgovernor.Artifact, error)
	GetActive(ctx context.Context) (*rtgovernor.Artifact, error)
}

type evaluationBacktestFlow interface {
	Run(
		ctx context.Context,
		request flows.PaperBacktestRequest,
	) (flows.DurableBacktestResult, error)
}

type evaluationBacktestStore interface {
	GetBacktestRun(ctx context.Context, runID string) (*domain.BacktestRun, error)
	GetDatasetReference(ctx context.Context, datasetID string) (*domain.DatasetReference, error)
	QueryBacktestRuns(ctx context.Context, query backtest.RunQuery) ([]domain.BacktestRun, error)
	QueryEvaluationReports(
		ctx context.Context,
		query backtest.EvaluationReportQuery,
	) ([]domain.EvaluationReport, error)
	CreateDatasetReference(
		ctx context.Context,
		reference domain.DatasetReference,
	) (domain.DatasetReference, error)
	CreateBacktestRun(ctx context.Context, run domain.BacktestRun) (domain.BacktestRun, error)
}

type evaluationTraceReader interface {
	QueryTraces(ctx context.Context, query audit.TraceQuery) ([]domain.DecisionTrace, error)
	QueryOrderIntents(
		ctx context.Context,
		query audit.OrderIntentQuery,
	) ([]domain.OrderIntent, error)
}

type evaluationExecutionReader interface {
	GetCommand(ctx context.Context, commandID string) (*domain.ExecutionCommand, error)
	GetOrder(ctx context.Context, orderID string) (*domain.ExecutionOrder, error)
	GetFill(ctx context.Context, fillID string) (*domain.ExecutionFill, error)
	QueryPositionSnapshots(
		ctx context.Context,
		query execution.PositionSnapshotQuery,
	) ([]domain.PositionSnapshot, error)
	QueryPortfolioSnapshots(
		ctx context.Context,
		query execution.PortfolioSnapshotQuery,
	) ([]domain.PortfolioSnapshot, error)
}

type evaluationReplayReader interface {
	ReplayCandles(
		ctx context.Context,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
	) ([]data.ReplayCandle, error)
}

type EvaluationWorkspaceServiceDeps struct {
	dig.In

	IDGenerator         ident.Generator
	VersionRegistry     *rtstrategy.VersionRegistryService
	ArtifactStore       *rtstrategy.ArtifactDatabaseStore
	GovernorPolicyStore *rtgovernor.ArtifactDatabaseStore
	DurableBacktestFlow *flows.DurableBacktestFlow
	BacktestService     *backtest.Service
	BacktestStore       *backtest.DatabaseStore
	AuditStore          *audit.DatabaseStore
	ExecutionStore      *execution.DatabaseStore
	SnapshotService     *execution.SnapshotService
	DataReadService     *data.ReadService
}

type EvaluationWorkspaceService struct {
	idGenerator         ident.Generator
	versionRegistry     evaluationStrategyRegistry
	artifactStore       strategyArtifactLookup
	governorPolicyStore evaluationGovernorPolicyStore
	durableBacktestFlow evaluationBacktestFlow
	backtestService     *backtest.Service
	backtestStore       evaluationBacktestStore
	auditStore          evaluationTraceReader
	executionStore      evaluationExecutionReader
	replayReader        evaluationReplayReader
}

type evaluationGovernorPolicyReference struct {
	policyID      string
	policyVersion string
	policyHash    string
}

type CreateEvaluationParams struct {
	StrategyID         string
	StrategyVersion    string
	Start              time.Time
	End                time.Time
	Quantity           float64
	GovernorPolicyHash string
	Note               string
}

type ListEvaluationsParams struct {
	StrategyID string
	Status     string
}

type EvaluationMetricSummary struct {
	TradeCount                    *int
	BlockedGovernorDecisionCount  *int
	RejectedGovernorDecisionCount *int
	MaxDrawdown                   *float64
}

type EvaluationAIRenderMetadata struct {
	RequestSourceType   string
	StrategySourceType  string
	StrategySourceLabel string
	Note                string
	EvidenceCounts      EvaluationEvidenceCounts
}

type EvaluationEvidenceCounts struct {
	Traces             int
	OrderIntents       int
	GovernorDecisions  int
	ExecutionRecords   int
	PositionSnapshots  int
	PortfolioSnapshots int
}

type EvaluationListItem struct {
	RunID                string
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	SourceType           string
	SourceLabel          string
	Instrument           StrategyInstrumentInput
	Timeframe            string
	TestedRangeStart     time.Time
	TestedRangeEnd       time.Time
	Status               string
	Decision             *string
	Metrics              *EvaluationMetricSummary
	FailureReason        string
	FailureDetails       string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	AIRenderMetadata     EvaluationAIRenderMetadata
}

type EvaluationDatasetReference struct {
	DatasetID      string
	ReplayChecksum string
	CreatedAt      time.Time
}

type EvaluationPolicyReference struct {
	PolicyID      string
	PolicyVersion string
	PolicyHash    string
}

type EvaluationTraceRow struct {
	TraceID      string
	DecisionTime time.Time
	Result       string
	ReasonCodes  []string
	DataQuality  string
	RunReference string
}

type EvaluationOrderIntentRow struct {
	IntentID          string
	TraceID           string
	Status            string
	ActionKind        string
	RequestedQuantity float64
	RequestedNotional float64
	CreatedTime       time.Time
}

type EvaluationGovernorDecisionRow struct {
	DecisionID string
	IntentID   string
	Status     string
	Reason     string
	Reference  string
}

type EvaluationExecutionRow struct {
	CommandID string
	OrderID   string
	FillID    string
	Status    string
	EventTime *time.Time
}

type EvaluationPositionSnapshotRow struct {
	SnapshotID  string
	FillID      string
	Quantity    float64
	RealizedPnL float64
	EventTime   time.Time
}

type EvaluationPortfolioSnapshotRow struct {
	SnapshotID    string
	FillID        string
	GrossExposure float64
	NetExposure   float64
	RealizedPnL   float64
	EventTime     time.Time
}

type EvaluationDetail struct {
	RunID                string
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	SourceType           string
	SourceLabel          string
	StrategySourceType   string
	StrategySourceLabel  string
	Instrument           StrategyInstrumentInput
	Timeframe            string
	TestedRangeStart     time.Time
	TestedRangeEnd       time.Time
	Status               string
	Decision             *string
	FailureReason        string
	FailureDetails       string
	Metrics              *EvaluationMetricSummary
	DatasetReference     *EvaluationDatasetReference
	PolicyReference      EvaluationPolicyReference
	CreatedAt            time.Time
	UpdatedAt            time.Time
	AIRenderMetadata     EvaluationAIRenderMetadata
	Traces               []EvaluationTraceRow
	OrderIntents         []EvaluationOrderIntentRow
	GovernorDecisions    []EvaluationGovernorDecisionRow
	ExecutionRecords     []EvaluationExecutionRow
	PositionSnapshots    []EvaluationPositionSnapshotRow
	PortfolioSnapshots   []EvaluationPortfolioSnapshotRow
}

type EvaluationReportView struct {
	RunID            string
	Status           string
	Decision         *string
	FailureReason    string
	FailureDetails   string
	Metrics          *EvaluationMetricSummary
	DatasetReference *EvaluationDatasetReference
	PolicyReference  EvaluationPolicyReference
	AIRenderMetadata EvaluationAIRenderMetadata
}

type EvaluationEvidenceView struct {
	RunID              string
	Status             string
	AIRenderMetadata   EvaluationAIRenderMetadata
	Traces             []EvaluationTraceRow
	OrderIntents       []EvaluationOrderIntentRow
	GovernorDecisions  []EvaluationGovernorDecisionRow
	ExecutionRecords   []EvaluationExecutionRow
	PositionSnapshots  []EvaluationPositionSnapshotRow
	PortfolioSnapshots []EvaluationPortfolioSnapshotRow
}

func NewEvaluationWorkspaceService(
	deps EvaluationWorkspaceServiceDeps,
) (*EvaluationWorkspaceService, error) {
	if deps.IDGenerator == nil {
		return nil, errors.New("id generator is required")
	}
	if deps.VersionRegistry == nil {
		return nil, errors.New("strategy version registry is required")
	}
	if deps.ArtifactStore == nil {
		return nil, errors.New("strategy artifact store is required")
	}
	if deps.GovernorPolicyStore == nil {
		return nil, errors.New("governor policy artifact store is required")
	}
	if deps.DurableBacktestFlow == nil {
		return nil, errors.New("durable backtest flow is required")
	}
	if deps.BacktestStore == nil {
		return nil, errors.New("backtest store is required")
	}
	if deps.BacktestService == nil {
		return nil, errors.New("backtest service is required")
	}
	if deps.AuditStore == nil {
		return nil, errors.New("audit store is required")
	}
	if deps.ExecutionStore == nil {
		return nil, errors.New("execution store is required")
	}
	if deps.SnapshotService == nil {
		return nil, errors.New("snapshot service is required")
	}
	if deps.DataReadService == nil {
		return nil, errors.New("data read service is required")
	}

	service := &EvaluationWorkspaceService{
		idGenerator:         deps.IDGenerator,
		versionRegistry:     deps.VersionRegistry,
		artifactStore:       deps.ArtifactStore,
		governorPolicyStore: deps.GovernorPolicyStore,
		durableBacktestFlow: deps.DurableBacktestFlow,
		backtestService:     deps.BacktestService,
		backtestStore:       deps.BacktestStore,
		auditStore:          deps.AuditStore,
		executionStore: snapshotExecutionReader{
			store:     deps.ExecutionStore,
			snapshots: deps.SnapshotService,
		},
		replayReader: deps.DataReadService,
	}

	if _, err := service.ensureDefaultGovernorPolicy(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure default governor policy: %w", err)
	}

	return service, nil
}

type snapshotExecutionReader struct {
	store     *execution.DatabaseStore
	snapshots *execution.SnapshotService
}

func (r snapshotExecutionReader) GetCommand(
	ctx context.Context,
	commandID string,
) (*domain.ExecutionCommand, error) {
	return r.store.GetCommand(ctx, commandID)
}

func (r snapshotExecutionReader) GetOrder(
	ctx context.Context,
	orderID string,
) (*domain.ExecutionOrder, error) {
	return r.store.GetOrder(ctx, orderID)
}

func (r snapshotExecutionReader) GetFill(
	ctx context.Context,
	fillID string,
) (*domain.ExecutionFill, error) {
	return r.store.GetFill(ctx, fillID)
}

func (r snapshotExecutionReader) QueryPositionSnapshots(
	ctx context.Context,
	query execution.PositionSnapshotQuery,
) ([]domain.PositionSnapshot, error) {
	return r.snapshots.QueryPositionSnapshots(ctx, query)
}

func (r snapshotExecutionReader) QueryPortfolioSnapshots(
	ctx context.Context,
	query execution.PortfolioSnapshotQuery,
) ([]domain.PortfolioSnapshot, error) {
	return r.snapshots.QueryPortfolioSnapshots(ctx, query)
}

func (s *EvaluationWorkspaceService) CreateEvaluation(
	ctx context.Context,
	params CreateEvaluationParams,
) (*EvaluationDetail, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	version, artifact, policyArtifact, timeRange, err := s.resolveCreateInputs(ctx, params)
	if err != nil {
		return nil, err
	}

	runID := s.idGenerator.MustNewV7().String()
	replayedCandles, replayErr := s.replayReader.ReplayCandles(
		ctx,
		artifact.Strategy.Instrument,
		artifact.Strategy.Timeframe,
		timeRange,
	)
	if replayErr != nil || len(replayedCandles) == 0 {
		failedRun, persistErr := s.persistDataUnavailableFailure(
			ctx,
			runID,
			version,
			artifact,
			policyArtifact,
			params.GovernorPolicyHash,
			timeRange,
			replayErr,
		)
		if persistErr != nil {
			return nil, persistErr
		}

		return s.buildDetail(ctx, *version, *artifact, failedRun)
	}

	policyReference := governorPolicyReference(params.GovernorPolicyHash, policyArtifact)

	_, flowErr := s.durableBacktestFlow.Run(ctx, flows.PaperBacktestRequest{
		RunID:                 runID,
		Mode:                  domain.DecisionModeBacktest,
		StrategyID:            version.StrategyID,
		StrategyVersion:       version.Version,
		StrategyArtifactHash:  artifact.Hash,
		GovernorPolicyID:      policyReference.policyID,
		GovernorPolicyVersion: policyReference.policyVersion,
		GovernorPolicyHash:    policyReference.policyHash,
		Instrument:            artifact.Strategy.Instrument,
		Timeframe:             artifact.Strategy.Timeframe,
		TimeRange:             timeRange,
		StrategyParameters:    artifact.Strategy.Parameters,
		GovernorPolicy:        policyArtifact.Policy,
		Quantity:              params.Quantity,
		ReportNotes:           strings.TrimSpace(params.Note),
	})
	if flowErr != nil {
		persistedRun, readErr := s.backtestStore.GetBacktestRun(ctx, runID)
		if readErr != nil {
			return nil, fmt.Errorf("read failed evaluation run: %w", readErr)
		}

		return s.buildDetail(ctx, *version, *artifact, *persistedRun)
	}

	persistedRun, err := s.backtestStore.GetBacktestRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("read completed evaluation run: %w", err)
	}

	return s.buildDetail(ctx, *version, *artifact, *persistedRun)
}

func (s *EvaluationWorkspaceService) ListEvaluations(
	ctx context.Context,
	params ListEvaluationsParams,
) ([]EvaluationListItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	query := backtest.RunQuery{StrategyID: strings.TrimSpace(params.StrategyID)}
	if strings.TrimSpace(params.Status) != "" {
		status, err := domain.NewBacktestRunStatus(params.Status)
		if err != nil {
			return nil, NewErrInvalidInput("status", err.Error())
		}
		query.Status = &status
	}

	runs, err := s.backtestStore.QueryBacktestRuns(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list evaluation runs: %w", err)
	}

	items := make([]EvaluationListItem, 0, len(runs))
	for _, run := range runs {
		versionValue, artifactValue, resolveErr := s.resolveRunStrategy(ctx, run)
		if resolveErr != nil {
			return nil, resolveErr
		}
		report, hasReport, reportErr := s.reportForRun(ctx, run.RunID.String())
		if reportErr != nil {
			return nil, fmt.Errorf(
				"read evaluation report for run %s: %w",
				run.RunID.String(),
				reportErr,
			)
		}
		item := EvaluationListItem{
			RunID:                run.RunID.String(),
			StrategyID:           run.StrategyID,
			StrategyVersion:      run.StrategyVersion,
			StrategyArtifactHash: run.StrategyArtifactHash,
			SourceType:           string(versionValue.SourceType),
			SourceLabel:          sourceLabel(versionValue.SourceType),
			Instrument:           mapEvaluationInstrument(artifactValue.Strategy.Instrument),
			Timeframe:            artifactValue.Strategy.Timeframe.String(),
			TestedRangeStart:     run.TestedRange.Start.UTC(),
			TestedRangeEnd:       run.TestedRange.End.UTC(),
			Status:               run.Status.String(),
			FailureReason:        run.FailureReason,
			FailureDetails:       run.FailureDetails,
			CreatedAt:            run.CreatedAt.Time(),
			UpdatedAt:            run.UpdatedAt.Time(),
			AIRenderMetadata: s.makeAIRenderMetadata(
				versionValue,
				reportPointer(report, hasReport),
				EvaluationEvidenceCounts{},
			),
		}
		if hasReport {
			decision := report.Decision.String()
			item.Decision = &decision
			item.Metrics = mapMetrics(report.Metrics)
			item.AIRenderMetadata = s.makeAIRenderMetadata(
				versionValue,
				&report,
				EvaluationEvidenceCounts{},
			)
		} else {
			item.Metrics = mapMetrics(run.Metrics)
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *EvaluationWorkspaceService) GetEvaluation(
	ctx context.Context,
	runID string,
) (*EvaluationDetail, error) {
	run, err := s.backtestStore.GetBacktestRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		if errors.Is(err, backtest.ErrBacktestRunNotFound) {
			return nil, NewErrNotFound("evaluation run", runID)
		}
		return nil, fmt.Errorf("read evaluation run: %w", err)
	}
	version, artifact, err := s.resolveRunStrategy(ctx, *run)
	if err != nil {
		return nil, err
	}

	return s.buildDetail(ctx, *version, *artifact, *run)
}

func (s *EvaluationWorkspaceService) GetEvaluationReport(
	ctx context.Context,
	runID string,
) (*EvaluationReportView, error) {
	detail, err := s.GetEvaluation(ctx, runID)
	if err != nil {
		return nil, err
	}

	return &EvaluationReportView{
		RunID:            detail.RunID,
		Status:           detail.Status,
		Decision:         detail.Decision,
		FailureReason:    detail.FailureReason,
		FailureDetails:   detail.FailureDetails,
		Metrics:          detail.Metrics,
		DatasetReference: detail.DatasetReference,
		PolicyReference:  detail.PolicyReference,
		AIRenderMetadata: detail.AIRenderMetadata,
	}, nil
}

func (s *EvaluationWorkspaceService) GetEvaluationEvidence(
	ctx context.Context,
	runID string,
) (*EvaluationEvidenceView, error) {
	detail, err := s.GetEvaluation(ctx, runID)
	if err != nil {
		return nil, err
	}

	return &EvaluationEvidenceView{
		RunID:              detail.RunID,
		Status:             detail.Status,
		AIRenderMetadata:   detail.AIRenderMetadata,
		Traces:             detail.Traces,
		OrderIntents:       detail.OrderIntents,
		GovernorDecisions:  detail.GovernorDecisions,
		ExecutionRecords:   detail.ExecutionRecords,
		PositionSnapshots:  detail.PositionSnapshots,
		PortfolioSnapshots: detail.PortfolioSnapshots,
	}, nil
}

func (s *EvaluationWorkspaceService) resolveCreateInputs(
	ctx context.Context,
	params CreateEvaluationParams,
) (*rtstrategy.Version, *rtstrategy.Artifact, *rtgovernor.Artifact, domain.TimeRange, error) {
	strategyID := strings.TrimSpace(params.StrategyID)
	if strategyID == "" {
		return nil, nil, nil, domain.TimeRange{}, NewErrInvalidInput(
			"strategyId",
			"strategy id is required",
		)
	}
	strategyVersion := strings.TrimSpace(params.StrategyVersion)
	if strategyVersion == "" {
		return nil, nil, nil, domain.TimeRange{}, NewErrInvalidInput(
			"strategyVersion",
			"strategy version is required",
		)
	}
	if params.Quantity <= 0 {
		return nil, nil, nil, domain.TimeRange{}, NewErrInvalidInput(
			"quantity",
			"quantity must be positive",
		)
	}
	timeRange, err := domain.NewTimeRange(params.Start, params.End)
	if err != nil {
		return nil, nil, nil, domain.TimeRange{}, NewErrInvalidInput("timeRange", err.Error())
	}

	version, err := s.versionRegistry.GetVersion(ctx, strategyID, strategyVersion)
	if err != nil {
		if errors.Is(err, rtstrategy.ErrStrategyVersionNotFound) {
			return nil, nil, nil, domain.TimeRange{}, NewErrNotFound(
				"strategy version",
				strategyID+"/"+strategyVersion,
			)
		}
		return nil, nil, nil, domain.TimeRange{}, fmt.Errorf("read strategy version: %w", err)
	}
	if version.Status != rtstrategy.VersionStatusReady {
		return nil, nil, nil, domain.TimeRange{}, NewErrInvalidInput(
			"strategyVersion",
			"strategy version status must be ready",
		)
	}

	artifact, err := s.artifactStore.Get(ctx, version.ArtifactHash)
	if err != nil {
		if errors.Is(err, rtstrategy.ErrArtifactNotFound) {
			return nil, nil, nil, domain.TimeRange{}, NewErrNotFound(
				"strategy artifact",
				version.ArtifactHash,
			)
		}
		return nil, nil, nil, domain.TimeRange{}, fmt.Errorf("read strategy artifact: %w", err)
	}
	if artifact.Hash != version.ArtifactHash ||
		artifact.SchemaVersion != version.ArtifactSchemaVersion {
		return nil, nil, nil, domain.TimeRange{}, NewErrInvalidInput(
			"strategyVersion",
			"strategy version does not resolve to the expected artifact",
		)
	}

	policyArtifact, err := s.resolveGovernorPolicy(ctx, params.GovernorPolicyHash)
	if err != nil {
		return nil, nil, nil, domain.TimeRange{}, err
	}

	return version, artifact, policyArtifact, timeRange, nil
}

func (s *EvaluationWorkspaceService) resolveGovernorPolicy(
	ctx context.Context,
	policyHash string,
) (*rtgovernor.Artifact, error) {
	trimmedHash := strings.TrimSpace(policyHash)
	if trimmedHash == "" {
		return s.ensureDefaultGovernorPolicy(ctx)
	}

	artifact, err := s.governorPolicyStore.Get(ctx, trimmedHash)
	if err != nil {
		if errors.Is(err, rtgovernor.ErrArtifactNotFound) {
			return nil, NewErrNotFound("governor policy", trimmedHash)
		}
		return nil, fmt.Errorf("read governor policy artifact: %w", err)
	}

	return artifact, nil
}

func (s *EvaluationWorkspaceService) ensureDefaultGovernorPolicy(
	ctx context.Context,
) (*rtgovernor.Artifact, error) {
	created, err := s.governorPolicyStore.CreateWithActivate(ctx, defaultGovernorPolicyPayload())
	if err != nil {
		return nil, fmt.Errorf("create default governor policy artifact: %w", err)
	}

	return &created, nil
}

func (s *EvaluationWorkspaceService) persistDataUnavailableFailure(
	ctx context.Context,
	runID string,
	version *rtstrategy.Version,
	artifact *rtstrategy.Artifact,
	policyArtifact *rtgovernor.Artifact,
	requestedPolicyHash string,
	timeRange domain.TimeRange,
	replayErr error,
) (domain.BacktestRun, error) {
	policyReference := governorPolicyReference(requestedPolicyHash, policyArtifact)

	reference, err := domain.NewDatasetReference(domain.DatasetReferenceParams{
		DatasetID:   stableEvaluationID("dataset", runID),
		EntityTypes: []string{"candles"},
		Instruments: []domain.Instrument{artifact.Strategy.Instrument},
		Timeframes:  []domain.Timeframe{artifact.Strategy.Timeframe},
		TimeRange:   timeRange,
		ReplayChecksum: stableEvaluationID(
			"replay",
			runID,
			artifact.Hash,
			timeRange.Start.UTC().Format(time.RFC3339Nano),
			timeRange.End.UTC().Format(time.RFC3339Nano),
		),
		CreatedAt: timeRange.End.UTC(),
		Metadata: map[string]string{
			"failure_reason": evaluationFailureReasonDataMissing,
		},
	})
	if err != nil {
		return domain.BacktestRun{}, fmt.Errorf("build failed dataset reference: %w", err)
	}
	reference, err = s.backtestStore.CreateDatasetReference(ctx, reference)
	if err != nil {
		return domain.BacktestRun{}, fmt.Errorf("create failed dataset reference: %w", err)
	}

	createdAt := timeRange.Start.UTC()
	run, err := domain.NewBacktestRun(domain.BacktestRunParams{
		RunID:                     runID,
		StrategyID:                version.StrategyID,
		StrategyVersion:           version.Version,
		StrategyArtifactHash:      artifact.Hash,
		DatasetID:                 reference.DatasetID.String(),
		GovernorPolicyID:          policyReference.policyID,
		GovernorPolicyVersion:     policyReference.policyVersion,
		GovernorPolicyHash:        policyReference.policyHash,
		Mode:                      domain.DecisionModeBacktest,
		TestedRange:               timeRange,
		FeeModelID:                zeroAssumptionModel,
		FeeAssumptions:            map[string]string{feeModelKey: zeroAssumptionModel},
		SlippageModelID:           zeroAssumptionModel,
		SlippageAssumptions:       map[string]string{slippageModelKey: zeroAssumptionModel},
		ExecutionSimulatorVersion: defaultExecutionSimulatorVersion,
		Status:                    domain.BacktestRunStatusPending,
		CreatedAt:                 createdAt,
		UpdatedAt:                 createdAt,
	})
	if err != nil {
		return domain.BacktestRun{}, fmt.Errorf("build failed backtest run: %w", err)
	}
	if _, err = s.backtestStore.CreateBacktestRun(ctx, run); err != nil {
		return domain.BacktestRun{}, fmt.Errorf("create failed backtest run: %w", err)
	}

	failureDetails := "no local replay candles were available for the requested strategy artifact and time range"
	if replayErr != nil {
		failureDetails = replayErr.Error()
	}

	run, err = s.backtestService.FailBacktestRun(ctx, backtest.FailBacktestRunRequest{
		RunID:          runID,
		FailureReason:  evaluationFailureReasonDataMissing,
		FailureDetails: failureDetails,
		EndedAt:        domain.BacktestRunTime(timeRange.End.UTC()),
	})
	if err != nil {
		return domain.BacktestRun{}, fmt.Errorf("fail backtest run: %w", err)
	}

	return run, nil
}

func (s *EvaluationWorkspaceService) resolveRunStrategy(
	ctx context.Context,
	run domain.BacktestRun,
) (*rtstrategy.Version, *rtstrategy.Artifact, error) {
	version, err := s.versionRegistry.GetVersion(ctx, run.StrategyID, run.StrategyVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("read strategy version for evaluation run: %w", err)
	}
	artifact, err := s.artifactStore.Get(ctx, run.StrategyArtifactHash)
	if err != nil {
		return nil, nil, fmt.Errorf("read strategy artifact for evaluation run: %w", err)
	}

	return version, artifact, nil
}

func (s *EvaluationWorkspaceService) buildDetail(
	ctx context.Context,
	version rtstrategy.Version,
	artifact rtstrategy.Artifact,
	run domain.BacktestRun,
) (*EvaluationDetail, error) {
	report, hasReport, reportErr := s.reportForRun(ctx, run.RunID.String())
	if reportErr != nil {
		return nil, fmt.Errorf(
			"read evaluation report for run %s: %w",
			run.RunID.String(),
			reportErr,
		)
	}
	counts, traces, intents, decisions, executions, positionSnapshots, portfolioSnapshots, err := s.loadEvidence(
		ctx,
		run,
	)
	if err != nil {
		return nil, err
	}
	var datasetReference *EvaluationDatasetReference
	if dataset, datasetErr := s.backtestStore.GetDatasetReference(ctx, run.DatasetID.String()); datasetErr == nil &&
		dataset != nil {
		datasetReference = &EvaluationDatasetReference{
			DatasetID:      dataset.DatasetID.String(),
			ReplayChecksum: dataset.ReplayChecksum,
			CreatedAt:      dataset.CreatedAt.Time(),
		}
	}
	var decision *string
	metrics := mapMetrics(run.Metrics)
	if hasReport {
		decided := report.Decision.String()
		decision = &decided
		metrics = mapMetrics(report.Metrics)
	}
	metadata := s.makeAIRenderMetadata(&version, reportPointer(report, hasReport), counts)

	return &EvaluationDetail{
		RunID:                run.RunID.String(),
		StrategyID:           run.StrategyID,
		StrategyVersion:      run.StrategyVersion,
		StrategyArtifactHash: run.StrategyArtifactHash,
		SourceType:           string(version.SourceType),
		SourceLabel:          sourceLabel(version.SourceType),
		StrategySourceType:   string(version.SourceType),
		StrategySourceLabel:  sourceLabel(version.SourceType),
		Instrument:           mapEvaluationInstrument(artifact.Strategy.Instrument),
		Timeframe:            artifact.Strategy.Timeframe.String(),
		TestedRangeStart:     run.TestedRange.Start.UTC(),
		TestedRangeEnd:       run.TestedRange.End.UTC(),
		Status:               run.Status.String(),
		Decision:             decision,
		FailureReason:        run.FailureReason,
		FailureDetails:       run.FailureDetails,
		Metrics:              metrics,
		DatasetReference:     datasetReference,
		PolicyReference: EvaluationPolicyReference{
			PolicyID:      run.GovernorPolicyID,
			PolicyVersion: run.GovernorPolicyVersion,
			PolicyHash:    run.GovernorPolicyHash,
		},
		CreatedAt:          run.CreatedAt.Time(),
		UpdatedAt:          run.UpdatedAt.Time(),
		AIRenderMetadata:   metadata,
		Traces:             traces,
		OrderIntents:       intents,
		GovernorDecisions:  decisions,
		ExecutionRecords:   executions,
		PositionSnapshots:  positionSnapshots,
		PortfolioSnapshots: portfolioSnapshots,
	}, nil
}

func (s *EvaluationWorkspaceService) reportForRun(
	ctx context.Context,
	runID string,
) (domain.EvaluationReport, bool, error) {
	reports, err := s.backtestStore.QueryEvaluationReports(
		ctx,
		backtest.EvaluationReportQuery{BacktestID: runID},
	)
	if err != nil {
		return domain.EvaluationReport{}, false, err
	}
	if len(reports) == 0 {
		return domain.EvaluationReport{}, false, nil
	}

	return reports[len(reports)-1], true, nil
}

func (s *EvaluationWorkspaceService) loadEvidence(
	ctx context.Context,
	run domain.BacktestRun,
) (EvaluationEvidenceCounts, []EvaluationTraceRow, []EvaluationOrderIntentRow, []EvaluationGovernorDecisionRow, []EvaluationExecutionRow, []EvaluationPositionSnapshotRow, []EvaluationPortfolioSnapshotRow, error) {
	tracesDomain, intentsDomain, positionDomain, portfolioDomain, err := s.queryEvidence(ctx, run)
	if err != nil {
		return EvaluationEvidenceCounts{}, nil, nil, nil, nil, nil, nil, err
	}

	tracesRows := buildTraceRows(tracesDomain)
	intentsRows, decisionRows, executionRows := s.buildIntentEvidenceRows(ctx, intentsDomain)
	positionRows := buildPositionSnapshotRows(positionDomain)
	portfolioRows := buildPortfolioSnapshotRows(portfolioDomain)

	counts := EvaluationEvidenceCounts{
		Traces:             len(tracesRows),
		OrderIntents:       len(intentsRows),
		GovernorDecisions:  len(decisionRows),
		ExecutionRecords:   len(executionRows),
		PositionSnapshots:  len(positionRows),
		PortfolioSnapshots: len(portfolioRows),
	}

	return counts, tracesRows, intentsRows, decisionRows, executionRows, positionRows, portfolioRows, nil
}

func (s *EvaluationWorkspaceService) queryEvidence(
	ctx context.Context,
	run domain.BacktestRun,
) ([]domain.DecisionTrace, []domain.OrderIntent, []domain.PositionSnapshot, []domain.PortfolioSnapshot, error) {
	tracesDomain, err := s.auditStore.QueryTraces(
		ctx,
		audit.TraceQuery{
			StrategyID: run.StrategyID,
			Mode:       pointer(domain.DecisionModeBacktest),
			TimeRange:  &run.TestedRange,
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("query evaluation traces: %w", err)
	}
	intentsDomain, err := s.auditStore.QueryOrderIntents(
		ctx,
		audit.OrderIntentQuery{
			StrategyID: run.StrategyID,
			Mode:       pointer(domain.DecisionModeBacktest),
			TimeRange:  &run.TestedRange,
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("query evaluation intents: %w", err)
	}
	positionDomain, err := s.executionStore.QueryPositionSnapshots(
		ctx,
		execution.PositionSnapshotQuery{
			StrategyID: run.StrategyID,
			Mode:       pointer(domain.DecisionModeBacktest),
			TimeRange:  &run.TestedRange,
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("query position snapshots: %w", err)
	}
	portfolioDomain, err := s.executionStore.QueryPortfolioSnapshots(
		ctx,
		execution.PortfolioSnapshotQuery{
			Mode:      pointer(domain.DecisionModeBacktest),
			TimeRange: &run.TestedRange,
		},
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("query portfolio snapshots: %w", err)
	}

	tracesDomain = filterTracesByRun(tracesDomain, run.RunID.String())
	intentsDomain = filterIntentsByRun(intentsDomain, run.RunID.String())

	return tracesDomain,
		intentsDomain,
		filterPositionSnapshotsByRun(positionDomain, intentsDomain, run.RunID.String()),
		filterPortfolioSnapshotsByRun(portfolioDomain, intentsDomain, run.RunID.String()),
		nil
}

func buildTraceRows(tracesDomain []domain.DecisionTrace) []EvaluationTraceRow {
	tracesRows := make([]EvaluationTraceRow, 0, boundedCount(len(tracesDomain)))
	for _, trace := range limitDecisionTraces(tracesDomain) {
		tracesRows = append(
			tracesRows,
			EvaluationTraceRow{
				TraceID:      string(trace.TraceID),
				DecisionTime: trace.DecisionTime.Time(),
				Result:       trace.Result.String(),
				ReasonCodes:  domainReasonCodes(trace.ReasonCodes),
				DataQuality:  trace.DataQuality.String(),
				RunReference: trace.RunReference,
			},
		)
	}

	return tracesRows
}

func (s *EvaluationWorkspaceService) buildIntentEvidenceRows(
	ctx context.Context,
	intentsDomain []domain.OrderIntent,
) ([]EvaluationOrderIntentRow, []EvaluationGovernorDecisionRow, []EvaluationExecutionRow) {
	intentsRows := make([]EvaluationOrderIntentRow, 0, boundedCount(len(intentsDomain)))
	decisionRows := make([]EvaluationGovernorDecisionRow, 0, boundedCount(len(intentsDomain)))
	executionRows := make([]EvaluationExecutionRow, 0, boundedCount(len(intentsDomain)))
	for _, intent := range limitOrderIntents(intentsDomain) {
		intentsRows = append(
			intentsRows,
			EvaluationOrderIntentRow{
				IntentID:          string(intent.IntentID),
				TraceID:           string(intent.TraceID),
				Status:            intent.Status.String(),
				ActionKind:        intent.ActionKind.String(),
				RequestedQuantity: intent.RequestedQuantity,
				RequestedNotional: intent.RequestedNotional,
				CreatedTime:       intent.CreatedTime.Time(),
			},
		)
		decisionRows = append(
			decisionRows,
			EvaluationGovernorDecisionRow{
				DecisionID: intent.Metadata["governor_decision_reference"],
				IntentID:   string(intent.IntentID),
				Status:     intent.Metadata["governor_decision_status"],
				Reason:     intent.Metadata["governor_decision_reason"],
				Reference:  intent.Metadata["governor_decision_reference"],
			},
		)
		executionRows = append(executionRows, s.mapExecutionRow(ctx, intent))
	}

	return intentsRows, decisionRows, executionRows
}

func buildPositionSnapshotRows(
	positionDomain []domain.PositionSnapshot,
) []EvaluationPositionSnapshotRow {
	positionRows := make([]EvaluationPositionSnapshotRow, 0, boundedCount(len(positionDomain)))
	for _, snapshot := range limitPositionSnapshots(positionDomain) {
		positionRows = append(
			positionRows,
			EvaluationPositionSnapshotRow{
				SnapshotID:  snapshot.SnapshotID.String(),
				FillID:      string(snapshot.SourceFillID),
				Quantity:    snapshot.Quantity,
				RealizedPnL: snapshot.RealizedPnL,
				EventTime:   snapshot.EventTime.Time(),
			},
		)
	}

	return positionRows
}

func buildPortfolioSnapshotRows(
	portfolioDomain []domain.PortfolioSnapshot,
) []EvaluationPortfolioSnapshotRow {
	portfolioRows := make([]EvaluationPortfolioSnapshotRow, 0, boundedCount(len(portfolioDomain)))
	for _, snapshot := range limitPortfolioSnapshots(portfolioDomain) {
		portfolioRows = append(
			portfolioRows,
			EvaluationPortfolioSnapshotRow{
				SnapshotID:    snapshot.SnapshotID.String(),
				FillID:        string(snapshot.SourceFillID),
				GrossExposure: snapshot.GrossExposure,
				NetExposure:   snapshot.NetExposure,
				RealizedPnL:   snapshot.RealizedPnL,
				EventTime:     snapshot.EventTime.Time(),
			},
		)
	}

	return portfolioRows
}

func (s *EvaluationWorkspaceService) mapExecutionRow(
	ctx context.Context,
	intent domain.OrderIntent,
) EvaluationExecutionRow {
	row := EvaluationExecutionRow{
		CommandID: intent.Metadata["execution_command_id"],
		OrderID:   intent.Metadata["execution_order_id"],
		FillID:    intent.Metadata["execution_fill_id"],
		Status:    "unavailable",
	}
	if row.CommandID != "" {
		if command, err := s.executionStore.GetCommand(ctx, row.CommandID); err == nil &&
			command != nil {
			eventTime := command.EventTime.Time()
			row.EventTime = &eventTime
			row.Status = command.Status.String()
		}
	}
	if row.OrderID != "" {
		if order, err := s.executionStore.GetOrder(ctx, row.OrderID); err == nil && order != nil {
			row.Status = order.Status.String()
		}
	}
	if row.FillID != "" {
		if fill, err := s.executionStore.GetFill(ctx, row.FillID); err == nil && fill != nil {
			eventTime := fill.EventTime.Time()
			row.EventTime = &eventTime
			row.Status = "filled"
		}
	}

	return row
}

func (s *EvaluationWorkspaceService) makeAIRenderMetadata(
	version *rtstrategy.Version,
	report *domain.EvaluationReport,
	counts EvaluationEvidenceCounts,
) EvaluationAIRenderMetadata {
	note := ""
	if report != nil {
		note = report.Notes
	}

	return EvaluationAIRenderMetadata{
		RequestSourceType:   evaluationRequestSourceTypeHuman,
		StrategySourceType:  string(version.SourceType),
		StrategySourceLabel: sourceLabel(version.SourceType),
		Note:                note,
		EvidenceCounts:      counts,
	}
}

func stableEvaluationID(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(hash[:])
}

func mapMetrics(metrics *domain.VersionedMetrics) *EvaluationMetricSummary {
	if metrics == nil {
		return nil
	}

	return &EvaluationMetricSummary{
		TradeCount:                    metrics.TradeCount,
		BlockedGovernorDecisionCount:  metrics.BlockedGovernorDecisionCount,
		RejectedGovernorDecisionCount: metrics.RejectedGovernorDecisionCount,
		MaxDrawdown:                   metrics.MaxDrawdown,
	}
}

func mapEvaluationInstrument(instrument domain.Instrument) StrategyInstrumentInput {
	return StrategyInstrumentInput{
		Venue:      instrument.Venue.String(),
		Symbol:     instrument.Symbol.String(),
		AssetClass: instrument.AssetClass.String(),
		Active:     instrument.Active,
	}
}

func pointer[T any](value T) *T { return &value }

func filterTracesByRun(rows []domain.DecisionTrace, runID string) []domain.DecisionTrace {
	filtered := make([]domain.DecisionTrace, 0, len(rows))
	for _, row := range rows {
		if row.RunReference == runID || row.Metadata["backtest_run_id"] == runID {
			filtered = append(filtered, row)
		}
	}

	return filtered
}

func filterIntentsByRun(rows []domain.OrderIntent, runID string) []domain.OrderIntent {
	filtered := make([]domain.OrderIntent, 0, len(rows))
	for _, row := range rows {
		if row.Metadata["backtest_run_id"] == runID {
			filtered = append(filtered, row)
		}
	}

	return filtered
}

func filterPositionSnapshotsByRun(
	rows []domain.PositionSnapshot,
	intents []domain.OrderIntent,
	runID string,
) []domain.PositionSnapshot {
	fillIDs := snapshotFillIDsByRun(intents)
	filtered := make([]domain.PositionSnapshot, 0, len(rows))
	for _, row := range rows {
		if row.Metadata["backtest_run_id"] == runID {
			filtered = append(filtered, row)
			continue
		}
		if _, ok := fillIDs[string(row.SourceFillID)]; ok {
			filtered = append(filtered, row)
		}
	}

	return filtered
}

func filterPortfolioSnapshotsByRun(
	rows []domain.PortfolioSnapshot,
	intents []domain.OrderIntent,
	runID string,
) []domain.PortfolioSnapshot {
	fillIDs := snapshotFillIDsByRun(intents)
	filtered := make([]domain.PortfolioSnapshot, 0, len(rows))
	for _, row := range rows {
		if row.Metadata["backtest_run_id"] == runID {
			filtered = append(filtered, row)
			continue
		}
		if _, ok := fillIDs[string(row.SourceFillID)]; ok {
			filtered = append(filtered, row)
		}
	}

	return filtered
}

func snapshotFillIDsByRun(intents []domain.OrderIntent) map[string]struct{} {
	fillIDs := make(map[string]struct{}, len(intents))
	for _, intent := range intents {
		fillID := strings.TrimSpace(intent.Metadata["execution_fill_id"])
		if fillID == "" {
			continue
		}
		fillIDs[fillID] = struct{}{}
	}

	return fillIDs
}

func limitDecisionTraces(rows []domain.DecisionTrace) []domain.DecisionTrace {
	if len(rows) <= defaultEvaluationEvidenceLimit {
		return rows
	}

	return rows[:defaultEvaluationEvidenceLimit]
}

func limitOrderIntents(rows []domain.OrderIntent) []domain.OrderIntent {
	if len(rows) <= defaultEvaluationEvidenceLimit {
		return rows
	}

	return rows[:defaultEvaluationEvidenceLimit]
}

func limitPositionSnapshots(rows []domain.PositionSnapshot) []domain.PositionSnapshot {
	if len(rows) <= defaultEvaluationEvidenceLimit {
		return rows
	}

	return rows[:defaultEvaluationEvidenceLimit]
}

func limitPortfolioSnapshots(rows []domain.PortfolioSnapshot) []domain.PortfolioSnapshot {
	if len(rows) <= defaultEvaluationEvidenceLimit {
		return rows
	}

	return rows[:defaultEvaluationEvidenceLimit]
}

func boundedCount(size int) int {
	if defaultEvaluationEvidenceLimit < size {
		return defaultEvaluationEvidenceLimit
	}

	return size
}

func domainReasonCodes(values []string) []string {
	return append([]string(nil), values...)
}

func reportPointer(report domain.EvaluationReport, ok bool) *domain.EvaluationReport {
	if !ok {
		return nil
	}

	return &report
}

func governorPolicyReference(
	requestedHash string,
	artifact *rtgovernor.Artifact,
) evaluationGovernorPolicyReference {
	if strings.TrimSpace(requestedHash) == "" {
		return evaluationGovernorPolicyReference{
			policyID:      defaultGovernorPolicyArtifactIDName,
			policyVersion: defaultGovernorPolicyVersion,
			policyHash:    artifact.Hash,
		}
	}

	return evaluationGovernorPolicyReference{
		policyID:      artifact.Hash,
		policyVersion: artifact.SchemaVersion,
		policyHash:    artifact.Hash,
	}
}

func defaultGovernorPolicyPayload() []byte {
	return []byte(defaultGovernorPolicyPayloadText)
}
