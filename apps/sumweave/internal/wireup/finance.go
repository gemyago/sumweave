package wireup

import (
	"log/slog"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/financeapp"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

type financeModuleBuildDeps struct {
	Database          *persistence.Database
	Jobs              *jobspkg.Module
	HTTPClientFactory *apphttpclient.ClientFactory
	Logger            *slog.Logger
	JWTSigningKey     string
	Finance           config.Finance
}

func buildFinanceModule(deps financeModuleBuildDeps) (*financepkg.Finance, error) {
	providers := deps.Finance.Providers
	return financeapp.NewModule(financeapp.ModuleDeps{
		Database:                        deps.Database,
		Jobs:                            deps.Jobs.Service,
		JobsStore:                       deps.Jobs.Store,
		Registry:                        deps.Jobs.Registry,
		HTTPClientFactory:               deps.HTTPClientFactory,
		RootLogger:                      deps.Logger,
		JWTSigningKey:                   deps.JWTSigningKey,
		MonobankBaseURL:                 providers.Monobank.BaseURL,
		MonobankRetryAfterFallbackDelay: providers.Monobank.RetryAfterFallbackDelay,
		EnableBanking: financepkg.EnableBankingConfig{
			BaseURL: providers.EnableBanking.BaseURL, AppID: providers.EnableBanking.AppID,
			PrivateKeyPath: providers.EnableBanking.PrivateKeyPath,
			ASPSPs: []financepkg.EnableBankingASPSP{{
				ProviderID: domain.ProviderIDPKO, Name: providers.EnableBanking.ASPSPName,
				Country: providers.EnableBanking.Country, PSUType: providers.EnableBanking.PSUType,
				ValidDays: providers.EnableBanking.ValidDays,
			}},
		},
	})
}
