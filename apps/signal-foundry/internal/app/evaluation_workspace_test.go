package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/runtime/audit"
	"github.com/gemyago/signal-foundry/runtime/backtest"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/execution"
	"github.com/gemyago/signal-foundry/runtime/flows"
	rtgovernor "github.com/gemyago/signal-foundry/runtime/governor"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type evaluationVersionRegistryStub struct {
	get func(context.Context, string, string) (*rtstrategy.Version, error)
}

func (s evaluationVersionRegistryStub) GetVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*rtstrategy.Version, error) {
	return s.get(ctx, strategyID, version)
}

type evaluationArtifactStoreStub struct {
	get func(context.Context, string) (*rtstrategy.Artifact, error)
}

func (s evaluationArtifactStoreStub) Get(
	ctx context.Context,
	hash string,
) (*rtstrategy.Artifact, error) {
	return s.get(ctx, hash)
}

type evaluationPolicyStoreStub struct {
	createWithActivate func(context.Context, []byte) (rtgovernor.Artifact, error)
	get                func(context.Context, string) (*rtgovernor.Artifact, error)
	getActive          func(context.Context) (*rtgovernor.Artifact, error)
}

func (s evaluationPolicyStoreStub) CreateWithActivate(
	ctx context.Context,
	raw []byte,
) (rtgovernor.Artifact, error) {
	return s.createWithActivate(ctx, raw)
}

func (s evaluationPolicyStoreStub) Get(
	ctx context.Context,
	hash string,
) (*rtgovernor.Artifact, error) {
	return s.get(ctx, hash)
}
func (s evaluationPolicyStoreStub) GetActive(ctx context.Context) (*rtgovernor.Artifact, error) {
	return s.getActive(ctx)
}

type evaluationBacktestFlowStub struct {
	request flows.PaperBacktestRequest
	run     func(context.Context, flows.PaperBacktestRequest) (flows.DurableBacktestResult, error)
}

func (s *evaluationBacktestFlowStub) Run(
	ctx context.Context,
	request flows.PaperBacktestRequest,
) (flows.DurableBacktestResult, error) {
	s.request = request
	return s.run(ctx, request)
}

type evaluationReplayReaderStub struct {
	replay func(context.Context, domain.Instrument, domain.Timeframe, domain.TimeRange) ([]data.ReplayCandle, error)
}

func (s evaluationReplayReaderStub) ReplayCandles(
	ctx context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]data.ReplayCandle, error) {
	return s.replay(ctx, instrument, timeframe, timeRange)
}

type evaluationBacktestStoreStub struct {
	runs      map[string]domain.BacktestRun
	reports   []domain.EvaluationReport
	datasets  map[string]domain.DatasetReference
	err       error
	reportErr error
}

func (s *evaluationBacktestStoreStub) CreateDatasetReference(
	_ context.Context,
	reference domain.DatasetReference,
) (domain.DatasetReference, error) {
	if s.err != nil {
		return domain.DatasetReference{}, s.err
	}
	if s.datasets == nil {
		s.datasets = map[string]domain.DatasetReference{}
	}
	s.datasets[reference.DatasetID.String()] = reference
	return reference, nil
}

func (s *evaluationBacktestStoreStub) CreateBacktestRun(
	_ context.Context,
	run domain.BacktestRun,
) (domain.BacktestRun, error) {
	if s.err != nil {
		return domain.BacktestRun{}, s.err
	}
	if s.runs == nil {
		s.runs = map[string]domain.BacktestRun{}
	}
	s.runs[run.RunID.String()] = run
	return run, nil
}

func (s *evaluationBacktestStoreStub) GetBacktestRun(
	_ context.Context,
	runID string,
) (*domain.BacktestRun, error) {
	if s.err != nil {
		return nil, s.err
	}
	run, ok := s.runs[runID]
	if !ok {
		return nil, backtest.ErrBacktestRunNotFound
	}
	return &run, nil
}

func (s *evaluationBacktestStoreStub) UpdateBacktestRun(
	_ context.Context,
	run domain.BacktestRun,
) (domain.BacktestRun, error) {
	if s.err != nil {
		return domain.BacktestRun{}, s.err
	}
	s.runs[run.RunID.String()] = run
	return run, nil
}

func (s *evaluationBacktestStoreStub) QueryBacktestRuns(
	_ context.Context,
	query backtest.RunQuery,
) ([]domain.BacktestRun, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make([]domain.BacktestRun, 0, len(s.runs))
	for _, run := range s.runs {
		if query.StrategyID != "" && run.StrategyID != query.StrategyID {
			continue
		}
		if query.Status != nil && run.Status != *query.Status {
			continue
		}
		result = append(result, run)
	}
	return result, nil
}

func (s *evaluationBacktestStoreStub) CreateEvaluationReport(
	_ context.Context,
	report domain.EvaluationReport,
) (domain.EvaluationReport, error) {
	if s.err != nil {
		return domain.EvaluationReport{}, s.err
	}
	s.reports = append(s.reports, report)
	return report, nil
}

func (s *evaluationBacktestStoreStub) QueryEvaluationReports(
	_ context.Context,
	query backtest.EvaluationReportQuery,
) ([]domain.EvaluationReport, error) {
	if s.reportErr != nil {
		return nil, s.reportErr
	}
	if s.err != nil {
		return nil, s.err
	}
	result := make([]domain.EvaluationReport, 0, len(s.reports))
	for _, report := range s.reports {
		if query.BacktestID != "" && report.BacktestRunID.String() != query.BacktestID {
			continue
		}
		result = append(result, report)
	}
	return result, nil
}

func (s *evaluationBacktestStoreStub) GetDatasetReference(
	_ context.Context,
	datasetID string,
) (*domain.DatasetReference, error) {
	if s.err != nil {
		return nil, s.err
	}
	reference, ok := s.datasets[datasetID]
	if !ok {
		return nil, backtest.ErrBacktestRunNotFound
	}
	return &reference, nil
}

type evaluationTraceStoreStub struct {
	traces  []domain.DecisionTrace
	intents []domain.OrderIntent
	err     error
}

func (s evaluationTraceStoreStub) QueryTraces(
	context.Context,
	audit.TraceQuery,
) ([]domain.DecisionTrace, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.traces, nil
}

func (s evaluationTraceStoreStub) QueryOrderIntents(
	context.Context,
	audit.OrderIntentQuery,
) ([]domain.OrderIntent, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.intents, nil
}

type evaluationExecutionStoreStub struct {
	command    *domain.ExecutionCommand
	order      *domain.ExecutionOrder
	fill       *domain.ExecutionFill
	positions  []domain.PositionSnapshot
	portfolios []domain.PortfolioSnapshot
	err        error
}

func (s evaluationExecutionStoreStub) GetCommand(
	context.Context,
	string,
) (*domain.ExecutionCommand, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.command == nil {
		return nil, errors.New("missing")
	}
	return s.command, nil
}

func (s evaluationExecutionStoreStub) GetOrder(
	context.Context,
	string,
) (*domain.ExecutionOrder, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.order == nil {
		return nil, errors.New("missing")
	}
	return s.order, nil
}

func (s evaluationExecutionStoreStub) GetFill(
	context.Context,
	string,
) (*domain.ExecutionFill, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.fill == nil {
		return nil, errors.New("missing")
	}
	return s.fill, nil
}

func (s evaluationExecutionStoreStub) QueryPositionSnapshots(
	context.Context,
	execution.PositionSnapshotQuery,
) ([]domain.PositionSnapshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.positions, nil
}

func (s evaluationExecutionStoreStub) QueryPortfolioSnapshots(
	context.Context,
	execution.PortfolioSnapshotQuery,
) ([]domain.PortfolioSnapshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.portfolios, nil
}

func TestEvaluationWorkspaceService(t *testing.T) {
	fake := faker.New()
	rawStrategy := []byte(
		`{"kind":"moving-average-crossover","instrument":{"venue":"binance","symbol":"BTCUSDT","assetClass":"crypto","active":true},"timeframe":"1h","parameters":{"fastWindow":9,"slowWindow":21}}`,
	)
	artifact, err := rtstrategy.NewArtifactFromDSLV0(rawStrategy)
	require.NoError(t, err)
	policy, err := rtgovernor.NewArtifactFromPolicyV0(defaultGovernorPolicyPayload())
	require.NoError(t, err)
	version := &rtstrategy.Version{
		StrategyID:            "strategy-a",
		Version:               "v1",
		Status:                rtstrategy.VersionStatusReady,
		SourceType:            rtstrategy.VersionSourceTypeDemo,
		ArtifactHash:          artifact.Hash,
		ArtifactSchemaVersion: artifact.SchemaVersion,
	}

	makeService := func(t *testing.T, replayCount int, versionValue *rtstrategy.Version) (*EvaluationWorkspaceService, *evaluationBacktestFlowStub, *evaluationBacktestStoreStub) {
		t.Helper()
		store := &evaluationBacktestStoreStub{
			runs:     map[string]domain.BacktestRun{},
			datasets: map[string]domain.DatasetReference{},
		}
		backtestService, buildBacktestServiceErr := backtest.NewService(store)
		require.NoError(t, buildBacktestServiceErr)
		flowStub := &evaluationBacktestFlowStub{
			run: func(_ context.Context, request flows.PaperBacktestRequest) (flows.DurableBacktestResult, error) {
				run, runErr := domain.NewBacktestRun(
					domain.BacktestRunParams{
						RunID:                     request.RunID,
						StrategyID:                request.StrategyID,
						StrategyVersion:           request.StrategyVersion,
						StrategyArtifactHash:      request.StrategyArtifactHash,
						DatasetID:                 "dataset-1",
						GovernorPolicyID:          request.GovernorPolicyID,
						GovernorPolicyVersion:     request.GovernorPolicyVersion,
						GovernorPolicyHash:        request.GovernorPolicyHash,
						Mode:                      domain.DecisionModeBacktest,
						TestedRange:               request.TimeRange,
						FeeModelID:                "zero",
						FeeAssumptions:            map[string]string{"fee_model": "zero"},
						SlippageModelID:           "zero",
						SlippageAssumptions:       map[string]string{"slippage_model": "zero"},
						ExecutionSimulatorVersion: "closed-candle-limit-v0",
						Status:                    domain.BacktestRunStatusCompleted,
						CreatedAt:                 request.TimeRange.Start,
						UpdatedAt:                 request.TimeRange.End,
					},
				)
				require.NoError(t, runErr)
				store.runs[run.RunID.String()] = run
				report, reportErr := domain.NewEvaluationReport(
					domain.EvaluationReportParams{
						EvaluationID:         fake.UUID().V4(),
						StrategyID:           request.StrategyID,
						StrategyVersion:      request.StrategyVersion,
						StrategyArtifactHash: request.StrategyArtifactHash,
						BacktestRunID:        request.RunID,
						DatasetID:            "dataset-1",
						Decision:             domain.EvaluationDecisionNeedsReview,
						Notes:                request.ReportNotes,
						CreatedAt:            request.TimeRange.End,
					},
				)
				require.NoError(t, reportErr)
				store.reports = []domain.EvaluationReport{report}
				return flows.DurableBacktestResult{}, nil
			},
		}
		service := &EvaluationWorkspaceService{
			idGenerator: ident.NewDefaultGenerator(),
			versionRegistry: evaluationVersionRegistryStub{
				get: func(context.Context, string, string) (*rtstrategy.Version, error) { return versionValue, nil },
			},
			artifactStore: evaluationArtifactStoreStub{
				get: func(context.Context, string) (*rtstrategy.Artifact, error) { return &artifact, nil },
			},
			governorPolicyStore: evaluationPolicyStoreStub{
				createWithActivate: func(context.Context, []byte) (rtgovernor.Artifact, error) { return policy, nil },
				get:                func(context.Context, string) (*rtgovernor.Artifact, error) { return &policy, nil },
				getActive:          func(context.Context) (*rtgovernor.Artifact, error) { return &policy, nil },
			},
			durableBacktestFlow: flowStub,
			backtestService:     backtestService,
			backtestStore:       store,
			auditStore:          evaluationTraceStoreStub{},
			executionStore:      evaluationExecutionStoreStub{},
			replayReader: evaluationReplayReaderStub{
				replay: func(context.Context, domain.Instrument, domain.Timeframe, domain.TimeRange) ([]data.ReplayCandle, error) {
					rows := make([]data.ReplayCandle, 0, replayCount)
					for i := range replayCount {
						provenance, provErr := domain.NewSourceProvenance("venue", "id")
						require.NoError(t, provErr)
						candle, candleErr := domain.NewCandle(
							domain.CandleParams{
								Instrument: artifact.Strategy.Instrument,
								Timeframe:  artifact.Strategy.Timeframe,
								TimeRange: domain.TimeRange{
									Start: time.Date(2026, time.June, 15, 10+i, 0, 0, 0, time.UTC),
									End:   time.Date(2026, time.June, 15, 11+i, 0, 0, 0, time.UTC),
								},
								Open:       1,
								High:       1,
								Low:        1,
								Close:      1,
								Volume:     1,
								Quality:    domain.DataQualityRaw,
								Provenance: provenance,
							},
						)
						require.NoError(t, candleErr)
						rows = append(
							rows,
							data.ReplayCandle{Identity: uint64(i + 1), Candle: candle},
						)
					}
					return rows, nil
				},
			},
		}
		return service, flowStub, store
	}

	t.Run("create derives runtime inputs from artifact and selected policy", func(t *testing.T) {
		service, flowStub, _ := makeService(t, 1, version)
		start := time.Date(2026, time.June, 15, 11, 0, 0, 0, time.UTC)
		end := start.Add(time.Hour)
		detail, createErr := service.CreateEvaluation(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:         "strategy-a",
				StrategyVersion:    "v1",
				Start:              start,
				End:                end,
				Quantity:           1,
				GovernorPolicyHash: policy.Hash,
				Note:               "operator note",
			},
		)
		require.NoError(t, createErr)
		require.Equal(t, artifact.Strategy.Instrument, flowStub.request.Instrument)
		require.Equal(t, artifact.Strategy.Timeframe, flowStub.request.Timeframe)
		require.Equal(t, artifact.Strategy.Parameters, flowStub.request.StrategyParameters)
		require.Equal(t, policy.Hash, flowStub.request.GovernorPolicyHash)
		require.Equal(t, policy.Hash, flowStub.request.GovernorPolicyID)
		require.Equal(t, policy.SchemaVersion, flowStub.request.GovernorPolicyVersion)
		require.Equal(t, "operator note", detail.AIRenderMetadata.Note)
		require.Equal(t, policy.Hash, detail.PolicyReference.PolicyID)
		require.Equal(t, policy.SchemaVersion, detail.PolicyReference.PolicyVersion)
	})

	t.Run("create rejects non-ready versions before running flow", func(t *testing.T) {
		archived := *version
		archived.Status = rtstrategy.VersionStatusArchived
		service, flowStub, _ := makeService(t, 1, &archived)
		_, createErr := service.CreateEvaluation(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:      "strategy-a",
				StrategyVersion: "v1",
				Start:           time.Now().UTC(),
				End:             time.Now().UTC().Add(time.Hour),
				Quantity:        1,
			},
		)
		require.EqualError(
			t,
			createErr,
			"invalid input for field 'strategyVersion': strategy version status must be ready",
		)
		require.Empty(t, flowStub.request.RunID)
	})

	t.Run(
		"create persists replay data unavailable failures without running flow",
		func(t *testing.T) {
			service, flowStub, store := makeService(t, 0, version)
			detail, createErr := service.CreateEvaluation(
				t.Context(),
				CreateEvaluationParams{
					StrategyID:      "strategy-a",
					StrategyVersion: "v1",
					Start:           time.Date(2026, time.June, 15, 11, 0, 0, 0, time.UTC),
					End:             time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
					Quantity:        1,
				},
			)
			require.NoError(t, createErr)
			require.Equal(t, evaluationFailureReasonDataMissing, detail.FailureReason)
			require.Empty(t, flowStub.request.RunID)
			require.Len(t, store.runs, 1)
			require.Equal(t, defaultGovernorPolicyArtifactIDName, detail.PolicyReference.PolicyID)
		},
	)

	t.Run(
		"create persists explicit policy reference on replay data unavailable failures",
		func(t *testing.T) {
			service, _, store := makeService(t, 0, version)
			detail, createErr := service.CreateEvaluation(
				t.Context(),
				CreateEvaluationParams{
					StrategyID:         "strategy-a",
					StrategyVersion:    "v1",
					Start:              time.Date(2026, time.June, 15, 11, 0, 0, 0, time.UTC),
					End:                time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
					Quantity:           1,
					GovernorPolicyHash: policy.Hash,
				},
			)
			require.NoError(t, createErr)
			require.Len(t, store.runs, 1)
			require.Equal(t, policy.Hash, detail.PolicyReference.PolicyID)
			require.Equal(t, policy.SchemaVersion, detail.PolicyReference.PolicyVersion)
			require.Equal(t, policy.Hash, detail.PolicyReference.PolicyHash)
		},
	)

	t.Run("constructor validates dependencies and ensures default policy", func(t *testing.T) {
		governorDSN := t.TempDir() + "/governor.db"
		governorSQLDB, storeErr := sqlconn.Open(governorDSN)
		require.NoError(t, storeErr)
		defer func() { require.NoError(t, governorSQLDB.Close()) }()
		governorStore, storeErr := rtgovernor.NewArtifactDatabaseStore(
			governorSQLDB,
			governorDSN,
			rtgovernor.ArtifactDatabaseStoreOpts{},
		)
		require.NoError(t, storeErr)
		require.NoError(t, governorStore.AutoMigrate())
		validDeps := EvaluationWorkspaceServiceDeps{
			IDGenerator:         ident.NewDefaultGenerator(),
			VersionRegistry:     &rtstrategy.VersionRegistryService{},
			ArtifactStore:       &rtstrategy.ArtifactDatabaseStore{},
			GovernorPolicyStore: governorStore,
			DurableBacktestFlow: &flows.DurableBacktestFlow{},
			BacktestService:     &backtest.Service{},
			BacktestStore:       &backtest.DatabaseStore{},
			AuditStore:          &audit.DatabaseStore{},
			ExecutionStore:      &execution.DatabaseStore{},
			SnapshotService:     &execution.SnapshotService{},
			DataReadService:     &data.ReadService{},
		}
		cases := []struct {
			name    string
			mutate  func(*EvaluationWorkspaceServiceDeps)
			message string
		}{
			{
				name:    "missing id generator",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.IDGenerator = nil },
				message: "id generator is required",
			},
			{
				name:    "missing version registry",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.VersionRegistry = nil },
				message: "strategy version registry is required",
			},
			{
				name:    "missing artifact store",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.ArtifactStore = nil },
				message: "strategy artifact store is required",
			},
			{
				name:    "missing governor store",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.GovernorPolicyStore = nil },
				message: "governor policy artifact store is required",
			},
			{
				name:    "missing durable flow",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.DurableBacktestFlow = nil },
				message: "durable backtest flow is required",
			},
			{
				name:    "missing backtest store",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.BacktestStore = nil },
				message: "backtest store is required",
			},
			{
				name:    "missing backtest service",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.BacktestService = nil },
				message: "backtest service is required",
			},
			{
				name:    "missing audit store",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.AuditStore = nil },
				message: "audit store is required",
			},
			{
				name:    "missing execution store",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.ExecutionStore = nil },
				message: "execution store is required",
			},
			{
				name:    "missing snapshot service",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.SnapshotService = nil },
				message: "snapshot service is required",
			},
			{
				name:    "missing data read service",
				mutate:  func(deps *EvaluationWorkspaceServiceDeps) { deps.DataReadService = nil },
				message: "data read service is required",
			},
		}
		for _, tc := range cases {
			deps := validDeps
			tc.mutate(&deps)
			_, buildErr := NewEvaluationWorkspaceService(deps)
			require.EqualError(t, buildErr, tc.message)
		}

		service, buildServiceErr := NewEvaluationWorkspaceService(validDeps)
		require.NoError(t, buildServiceErr)
		require.NotNil(t, service)
	})

	t.Run("resolveCreateInputs maps missing resources and policy errors", func(t *testing.T) {
		service, _, _ := makeService(t, 1, version)
		_, _, _, _, resolveErr := service.resolveCreateInputs(
			t.Context(),
			CreateEvaluationParams{
				Start:    time.Now().UTC(),
				End:      time.Now().UTC().Add(time.Hour),
				Quantity: 1,
			},
		)
		require.EqualError(
			t,
			resolveErr,
			"invalid input for field 'strategyId': strategy id is required",
		)

		service.versionRegistry = evaluationVersionRegistryStub{
			get: func(context.Context, string, string) (*rtstrategy.Version, error) {
				return nil, rtstrategy.ErrStrategyVersionNotFound
			},
		}
		_, _, _, _, resolveErr = service.resolveCreateInputs(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:      "strategy-a",
				StrategyVersion: "v1",
				Start:           time.Now().UTC(),
				End:             time.Now().UTC().Add(time.Hour),
				Quantity:        1,
			},
		)
		require.EqualError(t, resolveErr, "strategy version not found: strategy-a/v1")

		service.versionRegistry = evaluationVersionRegistryStub{
			get: func(context.Context, string, string) (*rtstrategy.Version, error) { return version, nil },
		}
		service.artifactStore = evaluationArtifactStoreStub{
			get: func(context.Context, string) (*rtstrategy.Artifact, error) {
				return nil, rtstrategy.ErrArtifactNotFound
			},
		}
		_, _, _, _, resolveErr = service.resolveCreateInputs(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:      "strategy-a",
				StrategyVersion: "v1",
				Start:           time.Now().UTC(),
				End:             time.Now().UTC().Add(time.Hour),
				Quantity:        1,
			},
		)
		require.EqualError(t, resolveErr, "strategy artifact not found: "+artifact.Hash)

		service.artifactStore = evaluationArtifactStoreStub{
			get: func(context.Context, string) (*rtstrategy.Artifact, error) {
				copyArtifact := artifact
				copyArtifact.Hash = "other-hash"
				return &copyArtifact, nil
			},
		}
		_, _, _, _, resolveErr = service.resolveCreateInputs(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:      "strategy-a",
				StrategyVersion: "v1",
				Start:           time.Now().UTC(),
				End:             time.Now().UTC().Add(time.Hour),
				Quantity:        1,
			},
		)
		require.EqualError(
			t,
			resolveErr,
			"invalid input for field 'strategyVersion': strategy version does not resolve to the expected artifact",
		)

		service.artifactStore = evaluationArtifactStoreStub{
			get: func(context.Context, string) (*rtstrategy.Artifact, error) { return &artifact, nil },
		}
		service.governorPolicyStore = evaluationPolicyStoreStub{
			createWithActivate: func(context.Context, []byte) (rtgovernor.Artifact, error) { return policy, nil },
			get: func(context.Context, string) (*rtgovernor.Artifact, error) {
				return nil, rtgovernor.ErrArtifactNotFound
			},
			getActive: func(context.Context) (*rtgovernor.Artifact, error) { return &policy, nil },
		}
		_, _, _, _, resolveErr = service.resolveCreateInputs(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:         "strategy-a",
				StrategyVersion:    "v1",
				Start:              time.Now().UTC(),
				End:                time.Now().UTC().Add(time.Hour),
				Quantity:           1,
				GovernorPolicyHash: "missing-policy",
			},
		)
		require.EqualError(t, resolveErr, "governor policy not found: missing-policy")
	})

	t.Run(
		"list get report and evidence expose persisted evidence and metadata",
		func(t *testing.T) {
			service, _, store := makeService(t, 1, version)
			runTime := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
			run, runErr := domain.NewBacktestRun(
				domain.BacktestRunParams{
					RunID:                 "run-1",
					StrategyID:            version.StrategyID,
					StrategyVersion:       version.Version,
					StrategyArtifactHash:  artifact.Hash,
					DatasetID:             "dataset-1",
					GovernorPolicyID:      defaultGovernorPolicyArtifactIDName,
					GovernorPolicyVersion: defaultGovernorPolicyVersion,
					GovernorPolicyHash:    policy.Hash,
					Mode:                  domain.DecisionModeBacktest,
					TestedRange: domain.TimeRange{
						Start: runTime.Add(-time.Hour),
						End:   runTime,
					},
					FeeModelID:      zeroAssumptionModel,
					FeeAssumptions:  map[string]string{"fee_model": zeroAssumptionModel},
					SlippageModelID: zeroAssumptionModel,
					SlippageAssumptions: map[string]string{
						"slippage_model": zeroAssumptionModel,
					},
					ExecutionSimulatorVersion: "closed-candle-limit-v0",
					Status:                    domain.BacktestRunStatusCompleted,
					CreatedAt:                 runTime.Add(-time.Hour),
					UpdatedAt:                 runTime,
					Metrics:                   nil,
				},
			)
			require.NoError(t, runErr)
			store.runs[run.RunID.String()] = run
			dataset, datasetErr := domain.NewDatasetReference(
				domain.DatasetReferenceParams{
					DatasetID:      "dataset-1",
					EntityTypes:    []string{"candles"},
					Instruments:    []domain.Instrument{artifact.Strategy.Instrument},
					Timeframes:     []domain.Timeframe{artifact.Strategy.Timeframe},
					TimeRange:      run.TestedRange,
					ReplayChecksum: "checksum-1",
					CreatedAt:      runTime,
				},
			)
			require.NoError(t, datasetErr)
			store.datasets[dataset.DatasetID.String()] = dataset
			tradeCount := 2
			report, reportErr := domain.NewEvaluationReport(
				domain.EvaluationReportParams{
					EvaluationID:         fake.UUID().V4(),
					StrategyID:           version.StrategyID,
					StrategyVersion:      version.Version,
					StrategyArtifactHash: artifact.Hash,
					BacktestRunID:        run.RunID.String(),
					DatasetID:            dataset.DatasetID.String(),
					Decision:             domain.EvaluationDecisionNeedsReview,
					Metrics: &domain.VersionedMetrics{
						SchemaVersion: "evaluation-report.v1",
						TradeCount:    &tradeCount,
					},
					Notes:     "operator note",
					CreatedAt: runTime,
				},
			)
			require.NoError(t, reportErr)
			store.reports = []domain.EvaluationReport{report}
			trace, traceErr := domain.NewDecisionTrace(
				domain.DecisionTraceParams{
					TraceID:              "trace-1",
					Mode:                 domain.DecisionModeBacktest,
					DecisionTime:         runTime,
					StrategyID:           run.StrategyID,
					StrategyVersion:      run.StrategyVersion,
					StrategyArtifactHash: run.StrategyArtifactHash,
					Instrument:           artifact.Strategy.Instrument,
					Timeframe:            artifact.Strategy.Timeframe,
					DatasetReference:     dataset.DatasetID.String(),
					RunReference:         run.RunID.String(),
					InputRange:           run.TestedRange,
					AnalyticsReference:   "analytics-1",
					DataQuality:          domain.DataQualityRaw,
					EvaluatorName:        "eval",
					EvaluatorVersion:     "v1",
					Result:               domain.DecisionTraceResultIntentCreated,
					ReasonCodes:          []string{"CROSSOVER"},
					Metadata:             map[string]string{"backtest_run_id": run.RunID.String()},
				},
			)
			require.NoError(t, traceErr)
			intent, intentErr := domain.NewOrderIntent(
				domain.OrderIntentParams{
					IntentID:                 "intent-1",
					TraceID:                  string(trace.TraceID),
					StrategyID:               run.StrategyID,
					StrategyVersion:          run.StrategyVersion,
					StrategyArtifactHash:     run.StrategyArtifactHash,
					Mode:                     domain.DecisionModeBacktest,
					Instrument:               artifact.Strategy.Instrument,
					Timeframe:                artifact.Strategy.Timeframe,
					ActionKind:               domain.CandidateActionKindLong,
					OrderType:                domain.OrderTypeLimit,
					RequestedQuantity:        1,
					RequestedNotional:        100,
					RequestedLimitPrice:      pointer(100.0),
					ReduceOnly:               false,
					SourceReasonCode:         "CROSSOVER",
					CandidateActionReference: "action-1",
					CreatedTime:              runTime,
					Status:                   domain.OrderIntentStatusExecutionCreated,
					Metadata: map[string]string{
						"backtest_run_id":             run.RunID.String(),
						"governor_decision_reference": "decision-1",
						"governor_decision_status":    "approved",
						"governor_decision_reason":    "ok",
						"execution_command_id":        "cmd-1",
						"execution_order_id":          "order-1",
						"execution_fill_id":           "fill-1",
					},
				},
			)
			require.NoError(t, intentErr)
			service.auditStore = evaluationTraceStoreStub{
				traces:  []domain.DecisionTrace{trace},
				intents: []domain.OrderIntent{intent},
			}
			commandEventTime, commandErr := domain.NewExecutionEventTime(runTime)
			require.NoError(t, commandErr)
			command := &domain.ExecutionCommand{
				CommandID: "cmd-1",
				Status:    domain.ExecutionCommandStatusCreated,
				EventTime: commandEventTime,
			}
			orderEventTime, orderErr := domain.NewExecutionEventTime(runTime)
			require.NoError(t, orderErr)
			order := &domain.ExecutionOrder{
				OrderID:   "order-1",
				Status:    domain.ExecutionOrderStatusFilled,
				EventTime: orderEventTime,
			}
			fillEventTime, fillErr := domain.NewExecutionEventTime(runTime)
			require.NoError(t, fillErr)
			fill := &domain.ExecutionFill{FillID: "fill-1", EventTime: fillEventTime}
			positionSnapshot, positionErr := domain.NewPositionSnapshot(
				domain.PositionSnapshotParams{
					SnapshotID:           "pos-1",
					SourceFillID:         "fill-1",
					Mode:                 domain.DecisionModeBacktest,
					StrategyID:           run.StrategyID,
					StrategyVersion:      run.StrategyVersion,
					StrategyArtifactHash: run.StrategyArtifactHash,
					Instrument:           artifact.Strategy.Instrument,
					Quantity:             1,
					AverageEntryPrice:    pointer(100.0),
					RealizedPnL:          0,
					ExposureNotional:     100,
					EventTime:            runTime,
					Metadata:             map[string]string{},
				},
			)
			require.NoError(t, positionErr)
			positionSnapshotOtherRun, positionOtherErr := domain.NewPositionSnapshot(
				domain.PositionSnapshotParams{
					SnapshotID:           "pos-2",
					SourceFillID:         "fill-2",
					Mode:                 domain.DecisionModeBacktest,
					StrategyID:           run.StrategyID,
					StrategyVersion:      run.StrategyVersion,
					StrategyArtifactHash: run.StrategyArtifactHash,
					Instrument:           artifact.Strategy.Instrument,
					Quantity:             2,
					AverageEntryPrice:    pointer(101.0),
					RealizedPnL:          1,
					ExposureNotional:     202,
					EventTime:            runTime.Add(5 * time.Minute),
					Metadata:             map[string]string{},
				},
			)
			require.NoError(t, positionOtherErr)
			portfolioSnapshot, portfolioErr := domain.NewPortfolioSnapshot(
				domain.PortfolioSnapshotParams{
					SnapshotID:    "portfolio-1",
					SourceFillID:  "fill-1",
					Mode:          domain.DecisionModeBacktest,
					GrossExposure: 100,
					NetExposure:   100,
					RealizedPnL:   0,
					EventTime:     runTime,
					Metadata:      map[string]string{},
				},
			)
			require.NoError(t, portfolioErr)
			portfolioSnapshotOtherRun, portfolioOtherErr := domain.NewPortfolioSnapshot(
				domain.PortfolioSnapshotParams{
					SnapshotID:    "portfolio-2",
					SourceFillID:  "fill-2",
					Mode:          domain.DecisionModeBacktest,
					GrossExposure: 200,
					NetExposure:   200,
					RealizedPnL:   1,
					EventTime:     runTime.Add(5 * time.Minute),
					Metadata:      map[string]string{},
				},
			)
			require.NoError(t, portfolioOtherErr)
			service.executionStore = evaluationExecutionStoreStub{
				command:    command,
				order:      order,
				fill:       fill,
				positions:  []domain.PositionSnapshot{positionSnapshot, positionSnapshotOtherRun},
				portfolios: []domain.PortfolioSnapshot{portfolioSnapshot, portfolioSnapshotOtherRun},
			}

			items, listErr := service.ListEvaluations(
				t.Context(),
				ListEvaluationsParams{StrategyID: run.StrategyID, Status: run.Status.String()},
			)
			require.NoError(t, listErr)
			require.Len(t, items, 1)
			require.Equal(t, "operator note", items[0].AIRenderMetadata.Note)

			detail, getErr := service.GetEvaluation(t.Context(), run.RunID.String())
			require.NoError(t, getErr)
			require.Len(t, detail.Traces, 1)
			require.Len(t, detail.ExecutionRecords, 1)
			require.Equal(t, "filled", detail.ExecutionRecords[0].Status)
			require.Len(t, detail.PositionSnapshots, 1)
			require.Equal(t, "pos-1", detail.PositionSnapshots[0].SnapshotID)
			require.Len(t, detail.PortfolioSnapshots, 1)
			require.Equal(t, "portfolio-1", detail.PortfolioSnapshots[0].SnapshotID)

			reportView, reportViewErr := service.GetEvaluationReport(
				t.Context(),
				run.RunID.String(),
			)
			require.NoError(t, reportViewErr)
			require.Equal(t, "checksum-1", reportView.DatasetReference.ReplayChecksum)

			evidenceView, evidenceErr := service.GetEvaluationEvidence(
				t.Context(),
				run.RunID.String(),
			)
			require.NoError(t, evidenceErr)
			require.Len(t, evidenceView.GovernorDecisions, 1)
			require.Len(t, evidenceView.PositionSnapshots, 1)
			require.Len(t, evidenceView.PortfolioSnapshots, 1)
		},
	)

	t.Run("error paths stay deterministic", func(t *testing.T) {
		service, _, _ := makeService(t, 1, version)
		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		_, createErr := service.CreateEvaluation(canceledCtx, CreateEvaluationParams{})
		require.ErrorIs(t, createErr, context.Canceled)

		_, listErr := service.ListEvaluations(
			t.Context(),
			ListEvaluationsParams{Status: "bad-status"},
		)
		require.EqualError(
			t,
			listErr,
			"invalid input for field 'status': invalid backtest run status \"bad-status\"",
		)

		_, getErr := service.GetEvaluation(t.Context(), "missing-run")
		require.EqualError(t, getErr, "evaluation run not found: missing-run")

		_, hasReport, reportErr := service.reportForRun(t.Context(), "missing-run")
		require.NoError(t, reportErr)
		require.False(t, hasReport)

		service.auditStore = evaluationTraceStoreStub{err: errors.New("audit boom")}
		_, _, _, _, _, _, _, evidenceErr := service.loadEvidence(
			t.Context(),
			domain.BacktestRun{
				RunID:      "run-1",
				StrategyID: "strategy-a",
				TestedRange: domain.TimeRange{
					Start: time.Now().UTC(),
					End:   time.Now().UTC().Add(time.Hour),
				},
			},
		)
		require.EqualError(t, evidenceErr, "query evaluation traces: audit boom")

		service.auditStore = evaluationTraceStoreStub{}
		service.executionStore = evaluationExecutionStoreStub{err: errors.New("execution boom")}
		_, _, _, _, _, _, _, evidenceErr = service.loadEvidence(
			t.Context(),
			domain.BacktestRun{
				RunID:      "run-1",
				StrategyID: "strategy-a",
				TestedRange: domain.TimeRange{
					Start: time.Now().UTC(),
					End:   time.Now().UTC().Add(time.Hour),
				},
			},
		)
		require.EqualError(t, evidenceErr, "query position snapshots: execution boom")

		service.executionStore = evaluationExecutionStoreStub{}
		unavailable := service.mapExecutionRow(
			t.Context(),
			domain.OrderIntent{Metadata: map[string]string{"execution_command_id": "cmd-1"}},
		)
		require.Equal(t, "unavailable", unavailable.Status)

		service, flowStub, _ := makeService(t, 1, version)
		flowStub.run = func(context.Context, flows.PaperBacktestRequest) (flows.DurableBacktestResult, error) {
			return flows.DurableBacktestResult{}, errors.New("flow boom")
		}
		_, createErr = service.CreateEvaluation(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:      "strategy-a",
				StrategyVersion: "v1",
				Start:           time.Date(2026, time.June, 15, 11, 0, 0, 0, time.UTC),
				End:             time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
				Quantity:        1,
			},
		)
		require.EqualError(t, createErr, "read failed evaluation run: backtest run not found")

		service, flowStub, failedStore := makeService(t, 1, version)
		flowStub.run = func(_ context.Context, request flows.PaperBacktestRequest) (flows.DurableBacktestResult, error) {
			run, runErr := domain.NewBacktestRun(
				domain.BacktestRunParams{
					RunID:                 request.RunID,
					StrategyID:            request.StrategyID,
					StrategyVersion:       request.StrategyVersion,
					StrategyArtifactHash:  request.StrategyArtifactHash,
					DatasetID:             "dataset-1",
					GovernorPolicyID:      request.GovernorPolicyID,
					GovernorPolicyVersion: request.GovernorPolicyVersion,
					GovernorPolicyHash:    request.GovernorPolicyHash,
					Mode:                  domain.DecisionModeBacktest,
					TestedRange:           request.TimeRange,
					FeeModelID:            zeroAssumptionModel,
					FeeAssumptions:        map[string]string{feeModelKey: zeroAssumptionModel},
					SlippageModelID:       zeroAssumptionModel,
					SlippageAssumptions: map[string]string{
						slippageModelKey: zeroAssumptionModel,
					},
					ExecutionSimulatorVersion: defaultExecutionSimulatorVersion,
					Status:                    domain.BacktestRunStatusFailed,
					FailureReason:             "strategy-failed",
					FailureDetails:            "flow boom",
					CreatedAt:                 request.TimeRange.Start,
					UpdatedAt:                 request.TimeRange.End,
				},
			)
			require.NoError(t, runErr)
			failedStore.runs[run.RunID.String()] = run
			return flows.DurableBacktestResult{}, errors.New("flow boom")
		}
		failedDetail, createErr := service.CreateEvaluation(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:      "strategy-a",
				StrategyVersion: "v1",
				Start:           time.Date(2026, time.June, 15, 11, 0, 0, 0, time.UTC),
				End:             time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
				Quantity:        1,
			},
		)
		require.NoError(t, createErr)
		require.Equal(t, "strategy-failed", failedDetail.FailureReason)

		service, flowStub, store := makeService(t, 1, version)
		flowStub.run = func(context.Context, flows.PaperBacktestRequest) (flows.DurableBacktestResult, error) {
			store.runs = map[string]domain.BacktestRun{}
			return flows.DurableBacktestResult{}, nil
		}
		_, createErr = service.CreateEvaluation(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:      "strategy-a",
				StrategyVersion: "v1",
				Start:           time.Date(2026, time.June, 15, 11, 0, 0, 0, time.UTC),
				End:             time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
				Quantity:        1,
			},
		)
		require.EqualError(t, createErr, "read completed evaluation run: backtest run not found")

		service, _, _ = makeService(t, 1, version)
		canceledListCtx, cancelList := context.WithCancel(t.Context())
		cancelList()
		_, listErr = service.ListEvaluations(canceledListCtx, ListEvaluationsParams{})
		require.ErrorIs(t, listErr, context.Canceled)

		service.backtestStore = &evaluationBacktestStoreStub{}
		service.versionRegistry = evaluationVersionRegistryStub{
			get: func(context.Context, string, string) (*rtstrategy.Version, error) {
				return nil, errors.New("version boom")
			},
		}
		_, getErr = service.GetEvaluation(t.Context(), "run-1")
		require.EqualError(t, getErr, "evaluation run not found: run-1")

		service, _, store = makeService(t, 1, version)
		store.err = errors.New("store boom")
		_, listErr = service.ListEvaluations(t.Context(), ListEvaluationsParams{})
		require.EqualError(t, listErr, "list evaluation runs: store boom")
		_, getErr = service.GetEvaluation(t.Context(), "run-1")
		require.EqualError(t, getErr, "read evaluation run: store boom")

		service, _, store = makeService(t, 1, version)
		runTime := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
		run, runErr := domain.NewBacktestRun(domain.BacktestRunParams{
			RunID:                 "run-report-error",
			StrategyID:            version.StrategyID,
			StrategyVersion:       version.Version,
			StrategyArtifactHash:  artifact.Hash,
			DatasetID:             "dataset-report-error",
			GovernorPolicyID:      defaultGovernorPolicyArtifactIDName,
			GovernorPolicyVersion: defaultGovernorPolicyVersion,
			GovernorPolicyHash:    policy.Hash,
			Mode:                  domain.DecisionModeBacktest,
			TestedRange: domain.TimeRange{
				Start: runTime.Add(-time.Hour),
				End:   runTime,
			},
			FeeModelID:                zeroAssumptionModel,
			FeeAssumptions:            map[string]string{feeModelKey: zeroAssumptionModel},
			SlippageModelID:           zeroAssumptionModel,
			SlippageAssumptions:       map[string]string{slippageModelKey: zeroAssumptionModel},
			ExecutionSimulatorVersion: defaultExecutionSimulatorVersion,
			Status:                    domain.BacktestRunStatusCompleted,
			CreatedAt:                 runTime.Add(-time.Hour),
			UpdatedAt:                 runTime,
		})
		require.NoError(t, runErr)
		store.runs[run.RunID.String()] = run
		store.reportErr = errors.New("report boom")
		_, listErr = service.ListEvaluations(t.Context(), ListEvaluationsParams{})
		require.EqualError(
			t,
			listErr,
			"read evaluation report for run run-report-error: report boom",
		)
		_, getErr = service.GetEvaluation(t.Context(), run.RunID.String())
		require.EqualError(
			t,
			getErr,
			"read evaluation report for run run-report-error: report boom",
		)
	})

	t.Run("wrapper readers and helper branches stay deterministic", func(t *testing.T) {
		executionDSN := t.TempDir() + "/execution.db"
		executionSQLDB, executionStoreErr := sqlconn.Open(executionDSN)
		require.NoError(t, executionStoreErr)
		defer func() { require.NoError(t, executionSQLDB.Close()) }()
		executionStore, executionStoreErr := execution.NewDatabaseStore(
			executionSQLDB,
			executionDSN,
			execution.DatabaseStoreOpts{},
		)
		require.NoError(t, executionStoreErr)
		require.NoError(t, executionStore.AutoMigrate())
		snapshotService, snapshotServiceErr := execution.NewSnapshotService(executionStore)
		require.NoError(t, snapshotServiceErr)
		reader := snapshotExecutionReader{store: executionStore, snapshots: snapshotService}
		_, commandErr := reader.GetCommand(t.Context(), "missing")
		require.Error(t, commandErr)
		_, orderErr := reader.GetOrder(t.Context(), "missing")
		require.Error(t, orderErr)
		_, fillErr := reader.GetFill(t.Context(), "missing")
		require.Error(t, fillErr)
		positionRows, positionErr := reader.QueryPositionSnapshots(
			t.Context(),
			execution.PositionSnapshotQuery{},
		)
		require.NoError(t, positionErr)
		require.Empty(t, positionRows)
		portfolioRows, portfolioErr := reader.QueryPortfolioSnapshots(
			t.Context(),
			execution.PortfolioSnapshotQuery{},
		)
		require.NoError(t, portfolioErr)
		require.Empty(t, portfolioRows)

		service, _, _ := makeService(t, 1, version)
		service.replayReader = evaluationReplayReaderStub{
			replay: func(context.Context, domain.Instrument, domain.Timeframe, domain.TimeRange) ([]data.ReplayCandle, error) {
				return nil, errors.New("replay boom")
			},
		}
		detail, createErr := service.CreateEvaluation(
			t.Context(),
			CreateEvaluationParams{
				StrategyID:      "strategy-a",
				StrategyVersion: "v1",
				Start:           time.Date(2026, time.June, 15, 11, 0, 0, 0, time.UTC),
				End:             time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
				Quantity:        1,
			},
		)
		require.NoError(t, createErr)
		require.Equal(t, "replay boom", detail.FailureDetails)

		require.Equal(t, 1, boundedCount(1))
		require.Equal(
			t,
			defaultEvaluationEvidenceLimit,
			boundedCount(defaultEvaluationEvidenceLimit+1),
		)
		require.Nil(t, reportPointer(domain.EvaluationReport{}, false))
		require.NotEmpty(t, defaultGovernorPolicyPayload())
		require.Len(
			t,
			limitDecisionTraces(make([]domain.DecisionTrace, defaultEvaluationEvidenceLimit+1)),
			defaultEvaluationEvidenceLimit,
		)
		require.Len(
			t,
			limitOrderIntents(make([]domain.OrderIntent, defaultEvaluationEvidenceLimit+1)),
			defaultEvaluationEvidenceLimit,
		)
		require.Len(
			t,
			limitPositionSnapshots(
				make([]domain.PositionSnapshot, defaultEvaluationEvidenceLimit+1),
			),
			defaultEvaluationEvidenceLimit,
		)
		require.Len(
			t,
			limitPortfolioSnapshots(
				make([]domain.PortfolioSnapshot, defaultEvaluationEvidenceLimit+1),
			),
			defaultEvaluationEvidenceLimit,
		)
	})
}
