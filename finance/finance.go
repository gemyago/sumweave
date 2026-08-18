package finance

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gemyago/sumweave/finance/domain"
	internalenablebanking "github.com/gemyago/sumweave/finance/internal/enablebanking"
	internalmonobank "github.com/gemyago/sumweave/finance/internal/monobank"
	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	internalsynthetic "github.com/gemyago/sumweave/finance/internal/synthetic"
	"github.com/gemyago/sumweave/finance/persistence"
)

type Finance struct {
	TenantService             *TenantService
	CatalogService            *CatalogService
	LedgerService             *LedgerService
	ReportingService          *ReportingService
	FXService                 *FXService
	CSVImportService          *CSVImportService
	BankConnectionService     *BankConnectionService
	SyntheticLinkStateService *SyntheticLinkStateService
	BankSyncService           *BankSyncService
	ProviderSnapshotService   *ProviderSnapshotService
	TransferDetailService     *TransferDetailService
}

func New(cfg *Config) (*Finance, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	store := persistence.NewStore(cfg.Database)
	currentFXRateStore := persistence.NewCurrentFXRateStore(cfg.Database)
	fxPairDiscoveryStore := persistence.NewFXPairDiscoveryStore(cfg.Database)
	transactionStore := persistence.NewTransactionTagStore(cfg.Database)
	csvImportStore := persistence.NewCSVImportStore(cfg.Database)
	providerSnapshotStore := persistence.NewProviderSnapshotStore(cfg.Database)
	transferCandidateStore := persistence.NewTransferCandidateStore(cfg.Database)
	connectors := newConnectors(cfg, store)
	connectorRegistry := internalproviders.NewStaticConnectorRegistry(connectors...)
	profileRegistry := newProviderProfileRegistry(cfg)
	windowPersistence := persistence.NewProviderWindowSyncPersistence(store)
	windowStore, err := internalproviders.NewProviderWindowSyncStore(
		windowPersistence,
		internalproviders.WithWindowSyncStoreIDGenerator(cfg.NewID),
		internalproviders.WithWindowSyncStoreNow(cfg.Now),
	)
	if err != nil { // coverage-ignore // Static production wireup always supplies persistence.
		return nil, fmt.Errorf("create provider window sync store: %w", err)
	}
	windowExecutor, err := internalproviders.NewWindowSyncExecutor(
		internalproviders.WithConnectorRegistry(connectorRegistry),
		internalproviders.WithWindowSyncStore(windowStore),
		internalproviders.WithRunIDGenerator(cfg.NewID),
		internalproviders.WithWindowSyncExecutorNow(cfg.Now),
	)
	if err != nil { // coverage-ignore // Static production wireup always supplies registry and store.
		return nil, fmt.Errorf("create provider window sync executor: %w", err)
	}
	syncOrchestrator, err := internalproviders.NewSyncOrchestrator(internalproviders.SyncOrchestratorParams{
		SyncStateJournal:   persistence.NewProviderSyncStateJournalStore(store),
		TargetWindowPolicy: internalproviders.NewCheckpointTargetWindowPolicy(),
		WindowChunkPolicy:  internalproviders.NewOldestFirstWindowChunkPolicy(),
		WindowExecutor:     windowExecutor,
		Logger:             cfg.Logger,
	}, internalproviders.WithNow(cfg.Now))
	if err != nil { // coverage-ignore // Static production wireup always supplies all required dependencies.
		return nil, fmt.Errorf("create provider sync orchestrator: %w", err)
	}
	services := newFocusedServices(
		store,
		transactionStore,
		csvImportStore,
		providerSnapshotStore,
		currentFXRateStore,
		fxPairDiscoveryStore,
		focusedServicesConfigFromConfig(cfg, syncOrchestrator),
	)

	bankConnectionService, err := newBankConnectionService(bankConnectionServiceArgs{
		Store:                   store,
		Logger:                  cfg.Logger,
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
		TenantService:         services.TenantService,
		CatalogService:        services.CatalogService,
		LedgerService:         services.LedgerService,
		ReportingService:      services.ReportingService,
		FXService:             services.FXService,
		CSVImportService:      services.CSVImportService,
		BankConnectionService: bankConnectionService,
		SyntheticLinkStateService: NewSyntheticLinkStateService(
			store,
			WithSyntheticLinkStateServiceNow(cfg.Now),
			WithSyntheticLinkStateServiceIDGenerator(cfg.NewID),
		),
		BankSyncService:         services.BankSyncService,
		ProviderSnapshotService: NewProviderSnapshotService(providerSnapshotStore),
		TransferDetailService:   NewTransferDetailService(transferCandidateStore),
	}, nil
}

func newConnectors(
	cfg *Config,
	store *persistence.Store,
) []internalproviders.Connector {
	connectors := []internalproviders.Connector{
		internalsynthetic.NewConnector(
			persistence.NewSyntheticProviderStateStoreFromStore(store),
			internalsynthetic.WithConnectorLogger(cfg.Logger),
			internalsynthetic.WithConnectorNow(cfg.Now),
			internalsynthetic.WithConnectorStateGenerator(cfg.NewID),
		),
		internalmonobank.NewConnector(internalmonobank.Args{
			BaseURL:    cfg.Monobank.BaseURL,
			HTTPClient: monobankHTTPClient(cfg),
			Logger:     cfg.Logger,
		}, internalmonobank.WithSecretTokenResolver(func(
			_ context.Context,
			secret domain.ConnectionSecret,
		) (string, error) {
			plaintext, err := cfg.ConnectionSecretCipher.OpenString(secret.Envelope)
			if err != nil {
				return "", fmt.Errorf("open monobank connection secret: %w", err)
			}
			return strings.TrimSpace(plaintext), nil
		})),
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
	return connectors
}

func monobankHTTPClient(cfg *Config) *http.Client {
	if cfg.Monobank.HTTPClient != nil {
		return cfg.Monobank.HTTPClient
	}
	return cfg.HTTPClient
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
