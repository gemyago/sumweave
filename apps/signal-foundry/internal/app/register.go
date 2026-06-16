package app

import (
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"go.uber.org/dig"
)

func Register(container *dig.Container) error {
	return di.ProvideAll(
		container,
		newStrategyArtifactStore,
		newStrategyVersionRegistryService,
		NewStrategyWorkspaceService,
		newEvaluationGovernorPolicyStore,
		newEvaluationAuditStore,
		newEvaluationExecutionStore,
		newEvaluationBacktestStore,
		newEvaluationAnalyticsService,
		newEvaluationStrategyService,
		newEvaluationAuditService,
		newEvaluationBacktestService,
		newEvaluationPaperService,
		newEvaluationSnapshotService,
		newDurableBacktestFlow,
		NewEvaluationWorkspaceService,
	)
}
