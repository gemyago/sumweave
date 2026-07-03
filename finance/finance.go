package finance

import (
	"fmt"

	"github.com/gemyago/signal-foundry/finance/domain"
	internalenablebanking "github.com/gemyago/signal-foundry/finance/internal/enablebanking"
	internalmonobank "github.com/gemyago/signal-foundry/finance/internal/monobank"
	internalproviders "github.com/gemyago/signal-foundry/finance/internal/providers"
	internalsynthetic "github.com/gemyago/signal-foundry/finance/internal/synthetic"
	"github.com/gemyago/signal-foundry/finance/persistence"
)

type Finance struct {
	BankConnectionService *BankConnectionService
}

func New(cfg *Config) (*Finance, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	store := persistence.NewStore(cfg.Database)
	connectorRegistry := newConnectorRegistry(cfg, store)
	profileRegistry := newProviderProfileRegistry(cfg)

	bankConnectionService, err := newBankConnectionService(bankConnectionServiceArgs{
		Store:                   store,
		ConnectionSecretCipher:  cfg.ConnectionSecretCipher,
		ConnectorRegistry:       connectorRegistry,
		ProviderProfileRegistry: profileRegistry,
		Now:                     cfg.Now,
		NewID:                   cfg.NewID,
	})

	if err != nil { // coverage-ignore // Config-validated internal wireup failure is not practically reachable.
		return nil, fmt.Errorf("create bank connection service: %w", err)
	}

	return &Finance{
		BankConnectionService: bankConnectionService,
	}, nil
}

func newConnectorRegistry(
	cfg *Config,
	store *persistence.Store,
) *internalproviders.StaticConnectorRegistry {
	connectors := []internalproviders.Connector{
		internalsynthetic.NewConnector(
			persistence.NewSyntheticProviderStateStoreFromStore(store),
			internalsynthetic.WithConnectorLogger(cfg.Logger),
			internalsynthetic.WithConnectorNow(cfg.Now),
		),
		internalmonobank.NewConnector(internalmonobank.Args{
			BaseURL:    cfg.Monobank.BaseURL,
			HTTPClient: cfg.HTTPClient,
			Logger:     cfg.Logger,
		}),
	}
	enableBankingASPSP := cfg.EnableBanking.ASPSPs[0]
	connectors = append(connectors, internalenablebanking.NewConnector(internalenablebanking.Args{
		BaseURL:        cfg.EnableBanking.BaseURL,
		HTTPClient:     cfg.HTTPClient,
		Logger:         cfg.Logger,
		AppID:          cfg.EnableBanking.AppID,
		PrivateKeyPath: cfg.EnableBanking.PrivateKeyPath,
		ASPSPName:      enableBankingASPSP.Name,
		Country:        enableBankingASPSP.Country,
		PSUType:        enableBankingASPSP.PSUType,
		ValidDays:      enableBankingASPSP.ValidDays,
		Now:            cfg.Now,
	}))
	return internalproviders.NewStaticConnectorRegistry(connectors...)
}

func newProviderProfileRegistry(cfg *Config) *internalproviders.StaticProviderProfileRegistry {
	profiles := []internalproviders.ProviderProfile{
		internalmonobank.Profile(),
		internalsynthetic.Profile(),
	}
	for _, aspsp := range cfg.EnableBanking.ASPSPs {
		profiles = append(profiles, internalproviders.ProviderProfile{
			ProviderID:    aspsp.ProviderID,
			ConnectorID:   domain.ProviderConnectorIDEnableBanking,
			DisplayName:   aspsp.Name,
			CountryCode:   aspsp.Country,
			MarketSegment: aspsp.PSUType,
		})
	}
	return internalproviders.NewStaticProviderProfileRegistry(profiles...)
}
