package app

import (
	"fmt"

	"github.com/gemyago/signal-foundry/runtime/analytics"
	"github.com/gemyago/signal-foundry/runtime/audit"
	"github.com/gemyago/signal-foundry/runtime/backtest"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/execution"
	"github.com/gemyago/signal-foundry/runtime/flows"
	rtgovernor "github.com/gemyago/signal-foundry/runtime/governor"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
)

func newEvaluationGovernorPolicyStore(
	deps strategyWorkspaceStoreDeps,
) (*rtgovernor.ArtifactDatabaseStore, error) {
	store, err := rtgovernor.NewArtifactDatabaseStore(
		deps.DatabaseDSN,
		rtgovernor.ArtifactDatabaseStoreOpts{TablePrefix: deps.DatabaseTablePrefix + "evaluation_"},
	)
	if err != nil {
		return nil, fmt.Errorf("create governor policy artifact store: %w", err)
	}

	return store, nil
}

func newEvaluationAuditStore(deps strategyWorkspaceStoreDeps) (*audit.DatabaseStore, error) {
	store, err := audit.NewDatabaseStore(
		deps.DatabaseDSN,
		audit.DatabaseStoreOpts{TablePrefix: deps.DatabaseTablePrefix + "evaluation_"},
	)
	if err != nil {
		return nil, fmt.Errorf("create evaluation audit store: %w", err)
	}

	return store, nil
}

func newEvaluationExecutionStore(
	deps strategyWorkspaceStoreDeps,
) (*execution.DatabaseStore, error) {
	store, err := execution.NewDatabaseStore(
		deps.DatabaseDSN,
		execution.DatabaseStoreOpts{TablePrefix: deps.DatabaseTablePrefix + "evaluation_"},
	)
	if err != nil {
		return nil, fmt.Errorf("create evaluation execution store: %w", err)
	}

	return store, nil
}

func newEvaluationBacktestStore(deps strategyWorkspaceStoreDeps) (*backtest.DatabaseStore, error) {
	store, err := backtest.NewDatabaseStore(
		deps.DatabaseDSN,
		backtest.DatabaseStoreOpts{TablePrefix: deps.DatabaseTablePrefix + "evaluation_"},
	)
	if err != nil {
		return nil, fmt.Errorf("create evaluation backtest store: %w", err)
	}

	return store, nil
}

func newEvaluationAnalyticsService(readService *data.ReadService) (*analytics.Service, error) {
	return analytics.NewService(analytics.ServiceDeps{CandleReplayReader: readService})
}

func newEvaluationStrategyService(
	analyticsService *analytics.Service,
) (*rtstrategy.Service, error) {
	return rtstrategy.NewService(rtstrategy.ServiceDeps{AnalyticsCalculator: analyticsService})
}

func newEvaluationAuditService(store *audit.DatabaseStore) (*audit.Service, error) {
	return audit.NewService(store)
}

func newEvaluationBacktestService(store *backtest.DatabaseStore) (*backtest.Service, error) {
	return backtest.NewService(store)
}

func newEvaluationPaperService(store *execution.DatabaseStore) (*execution.PaperService, error) {
	return execution.NewPaperService(store)
}

func newEvaluationSnapshotService(
	store *execution.DatabaseStore,
) (*execution.SnapshotService, error) {
	return execution.NewSnapshotService(store)
}

func newDurableBacktestFlow(
	readService *data.ReadService,
	analyticsService *analytics.Service,
	strategyService *rtstrategy.Service,
	auditService *audit.Service,
	paperService *execution.PaperService,
	snapshotService *execution.SnapshotService,
	backtestService *backtest.Service,
) (*flows.DurableBacktestFlow, error) {
	return flows.NewDurableBacktestFlow(flows.DurableBacktestFlowDeps{
		CandleReplayReader:  readService,
		AnalyticsCalculator: analyticsService,
		StrategyEvaluator:   strategyService,
		AuditRecorder:       auditService,
		GovernorEvaluator:   rtgovernor.NewService(),
		PaperExecutor:       paperService,
		SnapshotProjector:   snapshotService,
		BacktestRecorder:    backtestService,
	})
}
