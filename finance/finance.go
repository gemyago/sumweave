package finance

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	internalenablebanking "github.com/gemyago/signal-foundry/finance/internal/enablebanking"
	internalmonobank "github.com/gemyago/signal-foundry/finance/internal/monobank"
	internalproviders "github.com/gemyago/signal-foundry/finance/internal/providers"
	internalsynthetic "github.com/gemyago/signal-foundry/finance/internal/synthetic"
	"github.com/gemyago/signal-foundry/finance/persistence"
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
}

func New(cfg *Config) (*Finance, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	store := persistence.NewStore(cfg.Database)
	connectors := newConnectors(cfg, store)
	connectorRegistry := internalproviders.NewStaticConnectorRegistry(connectors...)
	profileRegistry := newProviderProfileRegistry(cfg)
	services := newFocusedServices(store, focusedServicesConfigFromConfig(cfg, connectors))

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
		BankSyncService: services.BankSyncService,
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
			HTTPClient: cfg.HTTPClient,
			Logger:     cfg.Logger,
		}, internalmonobank.WithSecretTokenResolver(func(
			_ context.Context,
			secret domain.ConnectionSecret,
		) (string, error) {
			return strings.TrimSpace(secret.Envelope.Ciphertext), nil
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
			ExternalID:        strings.TrimSpace(params.ExternalID),
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
		RawPayloads:  make([]ProviderRawPayload, 0, len(batch.RawPayloads)),
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
	for _, item := range batch.RawPayloads {
		result.RawPayloads = append(result.RawPayloads, ProviderRawPayload{
			Scope:            item.Scope,
			ProviderObjectID: item.ProviderObjectID,
			PayloadJSON:      item.PayloadJSON,
		})
	}
	return result
}
