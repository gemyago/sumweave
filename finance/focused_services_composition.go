package finance

import (
	"log/slog"
	"strings"
	"time"

	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
)

type focusedServices struct {
	TenantService    *TenantService
	CatalogService   *CatalogService
	LedgerService    *LedgerService
	ReportingService *ReportingService
	FXService        *FXService
	CSVImportService *CSVImportService
	BankSyncService  *BankSyncService
}

type focusedServicesConfig struct {
	now                    func() time.Time
	newID                  func() string
	fxProviders            []FXRatesProvider
	defaultFXProvider      string
	fxJobEnqueuer          FXRefreshJobEnqueuer
	fxScheduleWriter       FXRefreshScheduleWriter
	csvImportJobEnqueuer   CSVImportJobEnqueuer
	connectionSecretCipher connectionSecretCipher
	bankProviders          []BankConnectionProvider
	bankSyncJobEnqueuer    BankConnectionSyncJobEnqueuer
	bankSyncScheduleWriter BankConnectionSyncScheduleWriter
	logger                 *slog.Logger
}

func defaultFocusedServicesConfig() focusedServicesConfig {
	return focusedServicesConfig{
		now:               time.Now,
		newID:             uuid.NewString,
		defaultFXProvider: FXProviderFrankfurter,
	}
}

func focusedServicesConfigFromConfig(
	cfg *Config,
	connectors []internalproviders.Connector,
) focusedServicesConfig {
	serviceConfig := defaultFocusedServicesConfig()
	serviceConfig.now = cfg.Now
	serviceConfig.newID = cfg.NewID
	serviceConfig.fxProviders = append(serviceConfig.fxProviders, cfg.FXProviders...)
	serviceConfig.fxJobEnqueuer = cfg.FXJobEnqueuer
	serviceConfig.fxScheduleWriter = cfg.FXScheduleWriter
	serviceConfig.csvImportJobEnqueuer = cfg.CSVImportJobEnqueuer
	serviceConfig.connectionSecretCipher = cfg.ConnectionSecretCipher
	serviceConfig.bankSyncJobEnqueuer = cfg.BankSyncJobEnqueuer
	serviceConfig.bankSyncScheduleWriter = cfg.BankSyncScheduleWriter
	serviceConfig.logger = cfg.Logger
	if trimmed := strings.TrimSpace(cfg.DefaultFXProvider); trimmed != "" {
		serviceConfig.defaultFXProvider = trimmed
	}
	for _, connector := range connectors {
		if provider, ok := newConnectorBankSyncProvider(connector); ok {
			serviceConfig.bankProviders = append(serviceConfig.bankProviders, provider)
		}
	}
	return serviceConfig
}

func newFocusedServices(
	store *persistence.Store,
	transactionStore *persistence.TransactionTagStore,
	values ...any,
) focusedServices {
	var csvImportStore *persistence.CSVImportStore
	var providerEvidenceStore *persistence.ProviderEvidenceStore
	var currentFXRateStore *persistence.CurrentFXRateStore
	var fxPairDiscoveryStore *persistence.FXPairDiscoveryStore
	var cfg focusedServicesConfig
	for _, value := range values {
		switch typed := value.(type) {
		case *persistence.CSVImportStore:
			csvImportStore = typed
		case *persistence.ProviderEvidenceStore:
			providerEvidenceStore = typed
		case *persistence.CurrentFXRateStore:
			currentFXRateStore = typed
		case *persistence.FXPairDiscoveryStore:
			fxPairDiscoveryStore = typed
		case focusedServicesConfig:
			cfg = typed
		}
	}
	tenantOpts := []TenantServiceOption{
		WithTenantServiceNow(cfg.now),
		WithTenantServiceIDGenerator(cfg.newID),
	}
	catalogOpts := []CatalogServiceOption{
		WithCatalogServiceNow(cfg.now),
		WithCatalogServiceIDGenerator(cfg.newID),
	}
	ledgerOpts := []LedgerServiceOption{
		WithLedgerServiceNow(cfg.now),
		WithLedgerServiceIDGenerator(cfg.newID),
		WithLedgerServiceTransactionStore(transactionStore),
	}
	reportingOpts := []ReportingServiceOption{
		WithReportingServiceNow(cfg.now),
		WithReportingServiceDefaultFXProvider(cfg.defaultFXProvider),
		WithReportingServiceFXRateStore(currentFXRateStore),
	}
	fxOpts := []FXServiceOption{
		WithFXServiceNow(cfg.now),
		WithFXServiceDefaultProvider(cfg.defaultFXProvider),
		WithFXServiceProviders(cfg.fxProviders...),
	}
	if cfg.fxJobEnqueuer != nil {
		fxOpts = append(fxOpts, WithFXServiceJobEnqueuer(cfg.fxJobEnqueuer))
	}
	if cfg.fxScheduleWriter != nil {
		fxOpts = append(fxOpts, WithFXServiceScheduleWriter(cfg.fxScheduleWriter))
	}
	tenantService := NewTenantService(store, tenantOpts...)
	catalogService := NewCatalogService(store, catalogOpts...)
	ledgerService := NewLedgerService(store, ledgerOpts...)
	reportingService := NewReportingService(store, reportingOpts...)
	fxOpts = append(fxOpts, WithFXServiceRequiredPairs(fxPairDiscoveryStore))
	fxService := NewFXService(currentFXRateStore, fxOpts...)
	csvImportOpts := []CSVImportServiceOption{
		WithCSVImportServiceNow(cfg.now),
		WithCSVImportServiceIDGenerator(cfg.newID),
		WithCSVImportServiceRowStore(csvImportStore),
	}
	if cfg.csvImportJobEnqueuer != nil {
		csvImportOpts = append(csvImportOpts, WithCSVImportServiceJobEnqueuer(cfg.csvImportJobEnqueuer))
	}
	csvImportService := NewCSVImportService(store, catalogService, ledgerService, csvImportOpts...)
	bankSyncOpts := []BankSyncServiceOption{
		WithBankSyncServiceNow(cfg.now),
		WithBankSyncServiceIDGenerator(cfg.newID),
		WithBankSyncServiceConnectionSecretCipher(cfg.connectionSecretCipher),
		WithBankSyncServiceProviders(cfg.bankProviders...),
		WithBankSyncServiceEvidenceWriter(providerEvidenceStore),
	}
	if cfg.bankSyncJobEnqueuer != nil {
		bankSyncOpts = append(bankSyncOpts, WithBankSyncServiceJobEnqueuer(cfg.bankSyncJobEnqueuer))
	}
	if cfg.bankSyncScheduleWriter != nil {
		bankSyncOpts = append(bankSyncOpts, WithBankSyncServiceScheduleWriter(cfg.bankSyncScheduleWriter))
	}
	if cfg.logger != nil {
		bankSyncOpts = append(bankSyncOpts, WithBankSyncServiceLogger(cfg.logger))
	}
	return focusedServices{
		TenantService:    tenantService,
		CatalogService:   catalogService,
		LedgerService:    ledgerService,
		ReportingService: reportingService,
		FXService:        fxService,
		CSVImportService: csvImportService,
		BankSyncService:  NewBankSyncService(store, bankSyncOpts...),
	}
}
