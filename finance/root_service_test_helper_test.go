package finance

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
)

type serviceStore interface {
	tenantServiceStore
	catalogServiceStore
	ledgerServiceStore
	reportingServiceStore
	csvImportFocusedStore
}

type Service struct {
	store                  serviceStore
	now                    func() time.Time
	newID                  func() string
	access                 *accessGuard
	tenants                *TenantService
	catalog                *CatalogService
	ledger                 *LedgerService
	reporting              *ReportingService
	fx                     *FXService
	currentFXRates         fxServiceStore
	csvImports             *CSVImportService
	fxProviders            map[string]FXRatesProvider
	defaultFXProvider      string
	fxJobEnqueuer          FXRefreshJobEnqueuer
	fxScheduleWriter       FXRefreshScheduleWriter
	connectionSecretCipher connectionSecretCipher
	bankProviders          map[string]BankConnectionProvider
	csvImportJobEnqueuer   CSVImportJobEnqueuer
	logger                 *slog.Logger
}

type ServiceOption func(*Service)

func WithNow(now func() time.Time) ServiceOption {
	return func(service *Service) { service.now = now }
}

func WithIDGenerator(newID func() string) ServiceOption {
	return func(service *Service) { service.newID = newID }
}

func WithFXProviders(providers ...FXRatesProvider) ServiceOption {
	return func(service *Service) {
		if service.fxProviders == nil {
			service.fxProviders = map[string]FXRatesProvider{}
		}
		for _, provider := range providers {
			if provider != nil {
				service.fxProviders[provider.Name()] = provider
			}
		}
	}
}

func WithDefaultFXProvider(name string) ServiceOption {
	return func(service *Service) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			service.defaultFXProvider = trimmed
		}
	}
}

func WithFXJobEnqueuer(enqueuer FXRefreshJobEnqueuer) ServiceOption {
	return func(service *Service) { service.fxJobEnqueuer = enqueuer }
}

func WithFXScheduleWriter(writer FXRefreshScheduleWriter) ServiceOption {
	return func(service *Service) { service.fxScheduleWriter = writer }
}

func WithCSVImportJobEnqueuer(enqueuer CSVImportJobEnqueuer) ServiceOption {
	return func(service *Service) { service.csvImportJobEnqueuer = enqueuer }
}

func WithConnectionSecretCipher(cipher connectionSecretCipher) ServiceOption {
	return func(service *Service) { service.connectionSecretCipher = cipher }
}

func WithBankProviders(providers ...BankConnectionProvider) ServiceOption {
	return func(service *Service) {
		if service.bankProviders == nil {
			service.bankProviders = map[string]BankConnectionProvider{}
		}
		for _, provider := range providers {
			if provider != nil {
				service.bankProviders[provider.Name()] = provider
			}
		}
	}
}

func WithLogger(logger *slog.Logger) ServiceOption {
	return func(service *Service) {
		if logger != nil {
			service.logger = logger
		}
	}
}

func NewService(store serviceStore, opts ...ServiceOption) *Service {
	service := &Service{
		store:             store,
		now:               func() time.Time { return time.Now().UTC() },
		newID:             uuid.NewString,
		fxProviders:       defaultFXProviders(),
		defaultFXProvider: FXProviderFrankfurter,
		bankProviders:     map[string]BankConnectionProvider{},
		logger:            defaultLogger(),
	}
	for _, opt := range opts {
		opt(service)
	}
	service.bindServices()
	return service
}

func (s *Service) bindServices() {
	if store, ok := s.store.(*persistence.Store); ok {
		s.currentFXRates = persistence.NewCurrentFXRateStoreFromStore(store)
	} else {
		s.currentFXRates, _ = s.store.(fxServiceStore)
	}
	s.access = newAccessGuard(s.store)
	s.tenants = NewTenantService(s.store, WithTenantServiceNow(s.now), WithTenantServiceIDGenerator(s.newID))
	s.catalog = NewCatalogService(s.store, WithCatalogServiceNow(s.now), WithCatalogServiceIDGenerator(s.newID))
	s.ledger = NewLedgerService(s.store, WithLedgerServiceNow(s.now), WithLedgerServiceIDGenerator(s.newID))
	s.reporting = NewReportingService(
		s.store,
		WithReportingServiceNow(s.now),
		WithReportingServiceDefaultFXProvider(s.defaultFXProvider),
		WithReportingServiceFXRateStore(s.currentFXRates),
	)
	s.fx = NewFXService(
		s.currentFXRates,
		WithFXServiceNow(s.now),
		WithFXServiceProviders(mapFXProviders(s.fxProviders)...),
		WithFXServiceDefaultProvider(s.defaultFXProvider),
		WithFXServiceJobEnqueuer(s.fxJobEnqueuer),
		WithFXServiceScheduleWriter(s.fxScheduleWriter),
	)
	s.csvImports = NewCSVImportService(
		s.store,
		s.catalog,
		s.ledger,
		WithCSVImportServiceNow(s.now),
		WithCSVImportServiceIDGenerator(s.newID),
		WithCSVImportServiceJobEnqueuer(s.csvImportJobEnqueuer),
	)
}

func mapFXProviders(values map[string]FXRatesProvider) []FXRatesProvider {
	items := make([]FXRatesProvider, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

func (s *Service) requireTenantMember(ctx context.Context, tenantID string, userID string) error {
	return s.access.requireTenantMember(ctx, tenantID, userID)
}

func (s *Service) CreateTenant(
	ctx context.Context,
	params CreateTenantParams,
) (domain.Tenant, error) {
	return s.tenants.CreateTenant(ctx, params)
}

func (s *Service) UpdateTenant(
	ctx context.Context,
	params UpdateTenantParams,
) (domain.Tenant, error) {
	return s.tenants.UpdateTenant(ctx, params)
}

func (s *Service) ArchiveTenant(
	ctx context.Context,
	params ArchiveTenantParams,
) (domain.Tenant, error) {
	return s.tenants.ArchiveTenant(ctx, params)
}

func (s *Service) ListTenantsForUser(ctx context.Context, userID string) ([]domain.TenantMembershipView, error) {
	return s.tenants.ListTenantsForUser(ctx, userID)
}

func (s *Service) CreateTenantInvite(
	ctx context.Context,
	params CreateTenantInviteParams,
) (domain.TenantInvite, error) {
	return s.tenants.CreateTenantInvite(ctx, params)
}

func (s *Service) AcceptTenantInvite(
	ctx context.Context,
	params AcceptTenantInviteParams,
) (domain.TenantMembership, error) {
	return s.tenants.AcceptTenantInvite(ctx, params)
}

func (s *Service) ListTenantMembers(
	ctx context.Context,
	params ListTenantMembersParams,
) ([]domain.TenantMember, error) {
	return s.tenants.ListTenantMembers(ctx, params)
}

func (s *Service) ListTenantInvites(
	ctx context.Context,
	params ListTenantInvitesParams,
) ([]domain.TenantInvite, error) {
	return s.tenants.ListTenantInvites(ctx, params)
}

func (s *Service) CreateAccount(
	ctx context.Context,
	params CreateAccountParams,
) (domain.Account, error) {
	return s.catalog.CreateAccount(ctx, params)
}

func (s *Service) UpdateAccount(
	ctx context.Context,
	params UpdateAccountParams,
) (domain.Account, error) {
	return s.catalog.UpdateAccount(ctx, params)
}

func (s *Service) HideAccount(
	ctx context.Context,
	params HideAccountParams,
) error {
	return s.catalog.HideAccount(ctx, params)
}

func (s *Service) AttachLinkedAccount(
	ctx context.Context,
	params AttachLinkedAccountParams,
) (domain.Account, error) {
	return s.catalog.AttachLinkedAccount(ctx, params)
}

func (s *Service) GetAccount(
	ctx context.Context,
	params GetAccountParams,
) (domain.Account, error) {
	return s.catalog.GetAccount(ctx, params)
}

func (s *Service) ListAccounts(
	ctx context.Context,
	params ListAccountsParams,
) ([]domain.Account, error) {
	return s.catalog.ListAccounts(ctx, params)
}

func (s *Service) CreateCategory(
	ctx context.Context,
	params CreateCategoryParams,
) (domain.Category, error) {
	return s.catalog.CreateCategory(ctx, params)
}

func (s *Service) UpdateCategory(
	ctx context.Context,
	params UpdateCategoryParams,
) (domain.Category, error) {
	return s.catalog.UpdateCategory(ctx, params)
}

func (s *Service) HideCategory(
	ctx context.Context,
	params HideCategoryParams,
) error {
	return s.catalog.HideCategory(ctx, params)
}

func (s *Service) ListCategories(
	ctx context.Context,
	params ListCategoriesParams,
) ([]domain.Category, error) {
	return s.catalog.ListCategories(ctx, params)
}

func (s *Service) CreateTag(
	ctx context.Context,
	params CreateTagParams,
) (domain.Tag, error) {
	return s.catalog.CreateTag(ctx, params)
}

func (s *Service) UpdateTag(
	ctx context.Context,
	params UpdateTagParams,
) (domain.Tag, error) {
	return s.catalog.UpdateTag(ctx, params)
}

func (s *Service) HideTag(
	ctx context.Context,
	params HideTagParams,
) error {
	return s.catalog.HideTag(ctx, params)
}

func (s *Service) ListTags(
	ctx context.Context,
	params ListTagsParams,
) ([]domain.Tag, error) {
	return s.catalog.ListTags(ctx, params)
}

func (s *Service) RecordTransaction(
	ctx context.Context,
	params RecordTransactionParams,
) (domain.Transaction, error) {
	return s.ledger.RecordTransaction(ctx, params)
}

func (s *Service) UpdateTransaction(
	ctx context.Context,
	params UpdateTransactionParams,
) (domain.Transaction, error) {
	return s.ledger.UpdateTransaction(ctx, params)
}

func (s *Service) GetTransaction(
	ctx context.Context,
	params GetTransactionParams,
) (domain.Transaction, error) {
	return s.ledger.GetTransaction(ctx, params)
}

func (s *Service) HideTransaction(
	ctx context.Context,
	params HideTransactionParams,
) error {
	return s.ledger.HideTransaction(ctx, params)
}

func (s *Service) LinkTransfers(
	ctx context.Context,
	params LinkTransfersParams,
) error {
	return s.ledger.LinkTransfers(ctx, params)
}

func (s *Service) UnlinkTransfers(
	ctx context.Context,
	params UnlinkTransfersParams,
) error {
	return s.ledger.UnlinkTransfers(ctx, params)
}

func (s *Service) ListTransactions(
	ctx context.Context,
	params ListTransactionsParams,
) ([]domain.Transaction, error) {
	return s.ledger.ListTransactions(ctx, params)
}

func (s *Service) GetAccountBalance(
	ctx context.Context,
	params GetAccountBalanceParams,
) (domain.AccountBalance, error) {
	return s.ledger.GetAccountBalance(ctx, params)
}

func (s *Service) SummarizeTransactions(
	ctx context.Context,
	params SummarizeTransactionsParams,
) (domain.TransactionSummary, error) {
	return s.ledger.SummarizeTransactions(ctx, params)
}

func (s *Service) GetDashboard(
	ctx context.Context,
	params DashboardParams,
) (Dashboard, error) {
	return s.reporting.GetDashboard(ctx, params)
}

func (s *Service) loadDashboardData(
	ctx context.Context,
	tenantID string,
	params DashboardParams,
) (dashboardData, error) {
	return s.reporting.loadDashboardData(ctx, tenantID, params)
}

func (s *Service) TriggerFXRefresh(
	ctx context.Context,
	params TriggerFXRefreshParams,
) (FXRefreshJobRef, error) {
	return s.fx.TriggerFXRefresh(ctx, params)
}

func (s *Service) EnsureFXRefreshSchedule(
	ctx context.Context,
	params EnsureFXRefreshScheduleParams,
) (FXRefreshSchedule, error) {
	return s.fx.EnsureFXRefreshSchedule(ctx, params)
}

func (s *Service) GetFXAdminDiagnostics(
	ctx context.Context,
	params FXAdminDiagnosticsParams,
) (FXAdminDiagnostics, error) {
	return s.fx.GetFXAdminDiagnostics(ctx, params)
}

func (s *Service) PreviewCSVImport(
	ctx context.Context,
	params PreviewCSVImportParams,
) (CSVImportPreview, error) {
	return s.csvImports.PreviewCSVImport(ctx, params)
}

func (s *Service) ConfirmCSVImport(
	ctx context.Context,
	params ConfirmCSVImportParams,
) (CSVImportConfirmation, error) {
	return s.csvImports.ConfirmCSVImport(ctx, params)
}

func (s *Service) RunCSVImportJob(
	ctx context.Context,
	params RunCSVImportJobParams,
) (CSVImportRunResult, error) {
	return s.csvImports.RunCSVImportJob(ctx, params)
}

func (s *Service) GetCSVImportAudit(
	ctx context.Context,
	params GetCSVImportAuditParams,
) (CSVImportAudit, error) {
	return s.csvImports.GetCSVImportAudit(ctx, params)
}

func (s *Service) LinkTokenBankConnection(
	ctx context.Context,
	params LinkTokenBankConnectionParams,
) (domain.BankConnection, error) {
	if err := s.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.BankConnection{}, err
	}
	provider, err := s.bankProviderForLink(params.Provider, bankLinkMethodToken)
	if err != nil {
		return domain.BankConnection{}, err
	}
	result, err := provider.LinkToken(ctx, ProviderTokenLinkParams{Token: strings.TrimSpace(params.Token)})
	if err != nil {
		return domain.BankConnection{}, errors.New("link token bank connection: " + err.Error())
	}
	connection, err := s.saveLinkedBankConnection(
		ctx,
		params.TenantID,
		provider.bankID,
		domain.ProviderConnectorID(strings.TrimSpace(provider.Name())),
		result,
	)
	if err != nil {
		return domain.BankConnection{}, err
	}
	s.logger.InfoContext(ctx, "linked bank connection", "connection_id", connection.ID, "provider", connection.Provider)
	return connection, nil
}

func (s *Service) bankProviderForLink(bankID string, method bankLinkMethod) (*bankProviderRef, error) {
	trimmedBankID := strings.TrimSpace(bankID)
	providerName, err := configuredBankProviderName(trimmedBankID, method)
	if err != nil {
		return nil, err
	}
	provider, ok := s.bankProviders[providerName]
	if !ok {
		return nil, bankProviderNotConfiguredForBankError(trimmedBankID, providerName)
	}
	return &bankProviderRef{BankConnectionProvider: provider, bankID: trimmedBankID}, nil
}

func (s *Service) bankSyncStore() (*bankSyncStoreRef, error) {
	syncStore, ok := s.store.(bankSyncStore)
	if !ok {
		return nil, errors.New("bank sync store is required")
	}
	return &bankSyncStoreRef{bankSyncStore: syncStore}, nil
}

func (s *Service) connectionSecretsStore() (*connectionSecretsStoreRef, error) {
	secretStore, ok := s.store.(connectionSecretStore)
	if !ok {
		return nil, errors.New("connection secret store is required")
	}
	return &connectionSecretsStoreRef{connectionSecretStore: secretStore}, nil
}

func (s *Service) encryptAndSaveConnectionSecret(
	ctx context.Context,
	providerName string,
	reference string,
	plaintext string,
) (string, error) {
	store, err := s.connectionSecretsStore()
	if err != nil {
		return "", err
	}
	if s.connectionSecretCipher == nil {
		return "", errors.New("connection secret cipher is required")
	}
	secretWriter := newBankConnectionSecretWriter(store, s.connectionSecretCipher, s.now, s.newID)
	return secretWriter.SaveConnectionSecret(ctx, providerName, reference, plaintext)
}

func (s *Service) saveLinkedBankConnection(
	ctx context.Context,
	tenantID string,
	providerName string,
	connectorID domain.ProviderConnectorID,
	result ProviderLinkResult,
) (domain.BankConnection, error) {
	secretID, err := s.encryptAndSaveConnectionSecret(ctx, providerName, result.ProviderReference, result.Secret)
	if err != nil {
		return domain.BankConnection{}, err
	}
	syncStore, err := s.bankSyncStore()
	if err != nil {
		return domain.BankConnection{}, err
	}
	now := s.now().UTC()
	connection := domain.BankConnection{
		ID:                s.newID(),
		TenantID:          strings.TrimSpace(tenantID),
		Provider:          providerName,
		ConnectorID:       connectorID,
		DisplayName:       strings.TrimSpace(result.DisplayName),
		ProviderReference: strings.TrimSpace(result.ProviderReference),
		SecretID:          secretID,
		State:             result.State,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	saved, err := syncStore.SaveBankConnection(ctx, connection)
	if err != nil {
		return domain.BankConnection{}, errors.New("save bank connection: " + err.Error())
	}
	return saved, nil
}
