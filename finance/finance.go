package finance

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gemyago/sumweave/finance/credentials"
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
		focusedServicesConfigFromConfig(cfg, connectors, syncOrchestrator),
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

type connectorBankSyncProvider struct {
	name      string
	connector internalproviders.Connector
}

func newConnectorBankSyncProvider(
	connector internalproviders.Connector,
) (connectorBankSyncProvider, bool) {
	if connector == nil || !connector.Capabilities().SupportsFetch {
		return connectorBankSyncProvider{}, false
	}
	return connectorBankSyncProvider{
		name:      strings.TrimSpace(string(connector.ConnectorID())),
		connector: connector,
	}, true
}

func (p connectorBankSyncProvider) Name() string { return p.name }

func (p connectorBankSyncProvider) StartLink(context.Context, ProviderStartLinkParams) (ProviderLinkStart, error) {
	return ProviderLinkStart{}, ErrUnsupportedBankLinkingMethod
}

func (p connectorBankSyncProvider) FinishLink(context.Context, ProviderFinishLinkParams) (ProviderLinkResult, error) {
	return ProviderLinkResult{}, ErrUnsupportedBankLinkingMethod
}

func (p connectorBankSyncProvider) LinkToken(
	context.Context,
	ProviderTokenLinkParams,
) (ProviderTokenLinkResult, error) {
	return ProviderTokenLinkResult{}, ErrUnsupportedBankLinkingMethod
}

func (p connectorBankSyncProvider) Sync(
	ctx context.Context,
	params ProviderSyncParams,
) (ProviderSyncResult, error) {
	batch, err := p.connector.Fetch(ctx, internalproviders.FetchRequest{
		Connection: domain.ProviderConnectionRef{
			ConnectionID:      strings.TrimSpace(params.ConnectionID),
			ProviderID:        domain.ProviderID(strings.TrimSpace(p.name)),
			ConnectorID:       p.connector.ConnectorID(),
			ProviderReference: strings.TrimSpace(params.ProviderReference),
		},
		Secret: domain.ConnectionSecret{Envelope: credentialsEnvelopeFromPlaintext(params.Secret)},
		RequestedWindow: domain.ProviderSyncWindow{
			Start: params.WindowStart,
			End:   params.WindowEnd,
		},
	})
	if err != nil {
		return ProviderSyncResult{}, err
	}
	return providerSyncResultFromBatch(batch), nil
}

func credentialsEnvelopeFromPlaintext(secret string) credentials.Envelope {
	return credentials.Envelope{Ciphertext: strings.TrimSpace(secret)}
}

func providerSyncResultFromBatch(batch domain.ProviderSyncBatch) ProviderSyncResult {
	result := ProviderSyncResult{
		Accounts:     make([]ProviderNormalizedAccount, 0, len(batch.Accounts)),
		Transactions: make([]ProviderNormalizedTransaction, 0, len(batch.Transactions)),
		Snapshots:    append([]domain.ProviderSnapshotObservation(nil), batch.Snapshots...),
	}
	balanceByAccountID := make(map[string]domain.ProviderBalanceObservation, len(batch.Balances))
	for _, balance := range batch.Balances {
		balanceByAccountID[strings.TrimSpace(balance.ProviderAccountID)] = balance
	}
	for _, account := range batch.Accounts {
		var currentBalanceMinor *int64
		var availableBalanceMinor *int64
		if balance, ok := balanceByAccountID[strings.TrimSpace(account.ProviderAccountID)]; ok {
			current := balance.CurrentBalanceMinor
			currentBalanceMinor = &current
			availableBalanceMinor = balance.AvailableBalanceMinor
		}
		mapped := ProviderNormalizedAccount{
			ProviderAccountID:     strings.TrimSpace(account.ProviderAccountID),
			Name:                  account.Name,
			Currency:              account.Currency,
			IBAN:                  account.IBAN,
			MaskedPAN:             account.MaskedPAN,
			CurrentBalanceMinor:   currentBalanceMinor,
			AvailableBalanceMinor: availableBalanceMinor,
		}
		result.Accounts = append(result.Accounts, mapped)
	}
	for _, item := range batch.Transactions {
		result.Transactions = append(result.Transactions, ProviderNormalizedTransaction{
			ProviderAccountID:     strings.TrimSpace(item.ProviderAccountID),
			ProviderTransactionID: strings.TrimSpace(item.ProviderTransactionID),
			Status:                item.Status,
			AmountMinor:           item.AmountMinor,
			Currency:              item.Currency,
			Description:           item.Description,
			EffectiveAt:           item.EffectiveAt,
			Fingerprint:           item.Fingerprint,
			ProviderOriginal:      item.ProviderOriginal,
		})
	}
	return result
}
