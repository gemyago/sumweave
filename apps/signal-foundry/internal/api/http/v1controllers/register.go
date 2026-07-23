package v1controllers

import (
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/auth"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/runtime/data"
	"go.uber.org/dig"
)

func Register(container *dig.Container) error {
	return di.ProvideAll(container,
		di.ProvideValue(&HealthController{}),
		di.ProvideImplementation[*auth.AuthService, AuthenticatingService],
		di.ProvideImplementation[*app.UserDirectory, userDirectory],
		di.ProvideImplementation[*app.StrategyWorkspaceService, strategyWorkspaceService],
		di.ProvideImplementation[*app.EvaluationWorkspaceService, evaluationWorkspaceService],
		di.ProvideImplementation[*jobspkg.Service, jobsService],
		di.ProvideImplementation[*data.ReadService, replayReadService],
		di.ProvideImplementation[*data.LineageService, lineageBrowserService],
		di.ProvideImplementation[*financepkg.TenantService, tenantService],
		di.ProvideImplementation[*financepkg.CatalogService, catalogService],
		di.ProvideImplementation[*financepkg.LedgerService, ledgerService],
		di.ProvideImplementation[*financepkg.TransferDetailService, transferDetailService],
		di.ProvideImplementation[*financepkg.ReportingService, reportingService],
		di.ProvideImplementation[*financepkg.FXService, fxService],
		di.ProvideImplementation[*financepkg.ProviderEvidenceService, providerEvidenceService],
		di.ProvideImplementation[*financepkg.CSVImportService, csvImportService],
		di.ProvideImplementation[*financepkg.BankSyncService, bankSyncService],
		di.ProvideImplementation[*financepkg.BankConnectionService, bankConnectionService],
		di.ProvideImplementation[*financepkg.SyntheticLinkStateService, syntheticLinkStateService],
		NewAuthController,
		NewDataController,
		NewJobsController,
		NewFinanceController,
		NewEvaluationsController,
		NewStrategiesController,
	)
}
