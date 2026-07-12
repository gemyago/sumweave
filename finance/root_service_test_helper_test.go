package finance

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/google/uuid"
)

type serviceStore interface {
	tenantServiceStore
	catalogServiceStore
	ledgerServiceStore
	reportingServiceStore
	fxServiceStore
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
	csvImports             *CSVImportService
	bankSync               *BankSyncService
	fxProviders            map[string]FXRatesProvider
	defaultFXProvider      string
	fxJobEnqueuer          FXSyncJobEnqueuer
	fxScheduleWriter       FXSyncScheduleWriter
	connectionSecretCipher connectionSecretCipher
	bankProviders          map[string]BankConnectionProvider
	bankSyncJobEnqueuer    BankConnectionSyncJobEnqueuer
	bankSyncScheduleWriter BankConnectionSyncScheduleWriter
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

func WithFXJobEnqueuer(enqueuer FXSyncJobEnqueuer) ServiceOption {
	return func(service *Service) { service.fxJobEnqueuer = enqueuer }
}

func WithFXScheduleWriter(writer FXSyncScheduleWriter) ServiceOption {
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

func WithBankSyncJobEnqueuer(enqueuer BankConnectionSyncJobEnqueuer) ServiceOption {
	return func(service *Service) { service.bankSyncJobEnqueuer = enqueuer }
}

func WithBankConnectionSyncScheduleWriter(writer BankConnectionSyncScheduleWriter) ServiceOption {
	return func(service *Service) { service.bankSyncScheduleWriter = writer }
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
	s.access = newAccessGuard(s.store)
	s.tenants = NewTenantService(s.store, WithTenantServiceNow(s.now), WithTenantServiceIDGenerator(s.newID))
	s.catalog = NewCatalogService(s.store, WithCatalogServiceNow(s.now), WithCatalogServiceIDGenerator(s.newID))
	s.ledger = NewLedgerService(s.store, WithLedgerServiceNow(s.now), WithLedgerServiceIDGenerator(s.newID))
	s.reporting = NewReportingService(
		s.store,
		WithReportingServiceNow(s.now),
		WithReportingServiceDefaultFXProvider(s.defaultFXProvider),
	)
	s.fx = NewFXService(
		s.store,
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
	if bankSyncStore, ok := s.store.(bankSyncFocusedStore); ok {
		s.bankSync = NewBankSyncService(
			bankSyncStore,
			WithBankSyncServiceNow(s.now),
			WithBankSyncServiceIDGenerator(s.newID),
			WithBankSyncServiceConnectionSecretCipher(s.connectionSecretCipher),
			WithBankSyncServiceProviders(mapBankProviders(s.bankProviders)...),
			WithBankSyncServiceJobEnqueuer(s.bankSyncJobEnqueuer),
			WithBankSyncServiceScheduleWriter(s.bankSyncScheduleWriter),
			WithBankSyncServiceLogger(s.logger),
		)
	}
}

func mapFXProviders(values map[string]FXRatesProvider) []FXRatesProvider {
	items := make([]FXRatesProvider, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

func mapBankProviders(values map[string]BankConnectionProvider) []BankConnectionProvider {
	items := make([]BankConnectionProvider, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

func (s *Service) bankSyncService() (*BankSyncService, error) {
	if s.bankSync == nil {
		return nil, errors.New("bank sync store is required")
	}
	return s.bankSync, nil
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

func (s *Service) SyncFXRates(
	ctx context.Context,
	params SyncFXRatesParams,
) (SyncFXRatesResult, error) {
	return s.fx.SyncFXRates(ctx, params)
}

func (s *Service) TriggerFXSync(
	ctx context.Context,
	params TriggerFXSyncParams,
) (FXSyncJobRef, error) {
	return s.fx.TriggerFXSync(ctx, params)
}

func (s *Service) EnsureFXSyncSchedule(
	ctx context.Context,
	params EnsureFXSyncScheduleParams,
) (FXSyncSchedule, error) {
	return s.fx.EnsureFXSyncSchedule(ctx, params)
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

func (s *Service) UpsertBankConnectionSchedule(
	ctx context.Context,
	params UpsertBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	service, err := s.bankSyncService()
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	return service.UpsertBankConnectionSchedule(ctx, params)
}

func (s *Service) PauseBankConnectionSchedule(
	ctx context.Context,
	params PauseBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	service, err := s.bankSyncService()
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	return service.PauseBankConnectionSchedule(ctx, params)
}

func (s *Service) ResumeBankConnectionSchedule(
	ctx context.Context,
	params ResumeBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	service, err := s.bankSyncService()
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	return service.ResumeBankConnectionSchedule(ctx, params)
}

func (s *Service) writeBankConnectionSyncSchedule(ctx context.Context, schedule BankConnectionSyncSchedule) error {
	if s.bankSyncScheduleWriter == nil {
		return nil
	}
	if err := s.bankSyncScheduleWriter.UpsertBankConnectionSyncSchedule(ctx, schedule); err != nil {
		return errors.New("write bank connection sync schedule: " + err.Error())
	}
	return nil
}

func (s *Service) TriggerBankConnectionSync(
	ctx context.Context,
	params TriggerBankConnectionSyncParams,
) (BankConnectionSyncJobRef, error) {
	service, err := s.bankSyncService()
	if err != nil {
		return BankConnectionSyncJobRef{}, err
	}
	return service.TriggerBankConnectionSync(ctx, params)
}

func (s *Service) DeleteBankConnection(
	ctx context.Context,
	params DeleteBankConnectionParams,
) error {
	service, err := s.bankSyncService()
	if err != nil {
		return err
	}
	return service.DeleteBankConnection(ctx, params)
}

func (s *Service) RunBankConnectionSync(
	ctx context.Context,
	params RunBankConnectionSyncParams,
) (BankConnectionSyncResult, error) {
	service, err := s.bankSyncService()
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	return service.RunBankConnectionSync(ctx, params)
}

func (s *Service) RecordBankConnectionSyncScheduled(
	ctx context.Context,
	params RecordBankConnectionSyncScheduledParams,
) (domain.BankConnectionSchedule, error) {
	service, err := s.bankSyncService()
	if err != nil {
		return domain.BankConnectionSchedule{}, err
	}
	return service.RecordBankConnectionSyncScheduled(ctx, params)
}

func (s *Service) ApplyProviderSyncResult(
	ctx context.Context,
	params ApplyProviderSyncResultParams,
) (BankConnectionSyncResult, error) {
	service, err := s.bankSyncService()
	if err != nil {
		return BankConnectionSyncResult{}, err
	}
	return service.ApplyProviderSyncResult(ctx, params)
}

func (s *Service) ListBankConnections(
	ctx context.Context,
	params ListBankConnectionsParams,
) ([]BankConnectionView, error) {
	service, err := s.bankSyncService()
	if err != nil {
		return nil, err
	}
	return service.ListBankConnections(ctx, params)
}

func (s *Service) bankProvider(name string) (*bankProviderRef, error) {
	trimmedName := strings.TrimSpace(name)
	provider, ok := s.bankProviders[trimmedName]
	if !ok {
		return nil, bankProviderNotConfiguredError(trimmedName)
	}
	return &bankProviderRef{BankConnectionProvider: provider, bankID: trimmedName}, nil
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

func (s *Service) decryptConnectionSecret(
	ctx context.Context,
	secretID string,
) (string, error) {
	store, err := s.connectionSecretsStore()
	if err != nil {
		return "", err
	}
	if s.connectionSecretCipher == nil {
		return "", errors.New("connection secret cipher is required")
	}
	secret, err := store.GetConnectionSecret(ctx, secretID)
	if err != nil {
		return "", errors.New("get connection secret: " + err.Error())
	}
	plaintext, err := s.connectionSecretCipher.OpenString(secret.Envelope)
	if err != nil {
		return "", errors.New("open connection secret: " + err.Error())
	}
	return plaintext, nil
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
	reusedConnection := domain.BankConnection{}
	if strings.TrimSpace(providerName) == bankProviderPKO {
		connections, listErr := syncStore.ListBankConnections(ctx, tenantID)
		if listErr != nil {
			return domain.BankConnection{}, errors.New("list bank connections: " + listErr.Error())
		}
		for _, existingConnection := range connections {
			if existingConnection.Provider == providerName {
				reusedConnection = existingConnection
				break
			}
		}
	}
	now := s.now().UTC()
	connection := domain.BankConnection{
		ID:                s.newID(),
		TenantID:          strings.TrimSpace(tenantID),
		Provider:          providerName,
		ConnectorID:       connectorID,
		DisplayName:       strings.TrimSpace(result.DisplayName),
		ProviderReference: strings.TrimSpace(result.ProviderReference),
		ExternalID:        strings.TrimSpace(result.ExternalID),
		SecretID:          secretID,
		State:             result.State,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if reusedConnection.ID != "" {
		connection.ID = reusedConnection.ID
		connection.CreatedAt = reusedConnection.CreatedAt
	}
	saved, err := syncStore.SaveBankConnection(ctx, connection)
	if err != nil {
		return domain.BankConnection{}, errors.New("save bank connection: " + err.Error())
	}
	for _, payload := range result.RawPayloads {
		_, rawErr := syncStore.SaveRawPayload(ctx, domain.RawPayload{
			ID:               s.newID(),
			ConnectionID:     saved.ID,
			Scope:            payload.Scope,
			ProviderObjectID: payload.ProviderObjectID,
			PayloadJSON:      payload.PayloadJSON,
			CapturedAt:       now,
		})
		if rawErr != nil {
			return domain.BankConnection{}, errors.New("save raw payload: " + rawErr.Error())
		}
	}
	return saved, nil
}

func (s *Service) upsertProviderAccount(
	ctx context.Context,
	connection domain.BankConnection,
	item ProviderNormalizedAccount,
	now time.Time,
) (domain.ConnectionProviderAccount, error) {
	if s.bankSync == nil {
		return domain.ConnectionProviderAccount{}, errors.New("bank sync store is required")
	}
	return s.bankSync.upsertProviderAccount(ctx, connection, item, now)
}

func (s *Service) findOrCreateFinanceAccountForProviderAccount(
	ctx context.Context,
	connection domain.BankConnection,
	item ProviderNormalizedAccount,
	existingProviderAccount *domain.ConnectionProviderAccount,
	now time.Time,
) (domain.Account, error) {
	if s.bankSync == nil {
		return domain.Account{}, errors.New("bank sync store is required")
	}
	return s.bankSync.findOrCreateFinanceAccountForProviderAccount(ctx, connection, item, existingProviderAccount, now)
}

//nolint:unparam // Test-only compatibility wrapper keeps the legacy bool result for existing assertions.
func (s *Service) applyProviderTransaction(
	ctx context.Context,
	connection domain.BankConnection,
	providerAccount domain.ConnectionProviderAccount,
	item ProviderNormalizedTransaction,
	now time.Time,
) (bool, error) {
	if s.bankSync == nil {
		return false, errors.New("bank sync store is required")
	}
	return s.bankSync.applyProviderTransaction(ctx, connection, providerAccount, item, now)
}

func (s *Service) deleteBankConnectionOwnedMetadata(ctx context.Context, connection domain.BankConnection) error {
	if s.bankSync == nil {
		return errors.New("bank sync store is required")
	}
	return s.bankSync.deleteBankConnectionOwnedMetadata(ctx, connection)
}

func (s *Service) syncRunAlreadyApplied(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connectionID string,
	syncKey string,
) (bool, error) {
	if syncStore == nil {
		return false, nil
	}
	if strings.TrimSpace(syncKey) == "" {
		return false, nil
	}
	existing, err := syncStore.GetBankConnectionSyncRun(ctx, connectionID, syncKey)
	if err != nil {
		return false, errors.New("apply provider sync result: " + err.Error())
	}
	return existing != nil, nil
}

func (s *Service) claimSyncRun(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connectionID string,
	syncKey string,
	jobID string,
	now time.Time,
) (bool, error) {
	if syncStore == nil {
		return true, nil
	}
	if strings.TrimSpace(syncKey) == "" {
		return true, nil
	}
	newID := s.newID
	if newID == nil {
		newID = uuid.NewString
	}
	claimed, err := syncStore.ClaimBankConnectionSyncRun(ctx, domain.BankConnectionSyncRun{
		ID:           newID(),
		ConnectionID: connectionID,
		SyncKey:      strings.TrimSpace(syncKey),
		JobID:        strings.TrimSpace(jobID),
		CreatedAt:    now,
	})
	if err != nil {
		return false, errors.New("apply provider sync result: " + err.Error())
	}
	return claimed, nil
}

func (s *Service) makeScheduledRunMetadata(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connection domain.BankConnection,
	params RunBankConnectionSyncParams,
	now time.Time,
) (*ProviderScheduledRunMetadata, bool, error) {
	if strings.TrimSpace(params.Reason) != BankConnectionSyncReasonScheduled {
		return nil, false, nil
	}
	metadata := &ProviderScheduledRunMetadata{ScheduledAt: now}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
	if errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return metadata, true, nil
	}
	if err != nil {
		return nil, false, errors.New("prepare bank connection sync schedule: " + err.Error())
	}
	if schedule.Enabled && schedule.Interval > 0 {
		nextRunAt := now.Add(schedule.Interval).UTC()
		metadata.NextRunAt = &nextRunAt
	}
	return metadata, true, nil
}

func (s *Service) markBankConnectionSyncStarted(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connection *domain.BankConnection,
	params RunBankConnectionSyncParams,
	now time.Time,
	scheduledRun *ProviderScheduledRunMetadata,
) error {
	connection.LastSyncJobID = strings.TrimSpace(params.JobID)
	connection.LastSyncStartedAt = &now
	connection.LastSyncError = ""
	connection.UpdatedAt = now
	if _, err := syncStore.SaveBankConnection(ctx, *connection); err != nil {
		return err
	}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil || schedule == nil {
		return err
	}
	schedule.LastStartedAt = &now
	schedule.LastJobID = strings.TrimSpace(params.JobID)
	if scheduledRun != nil {
		schedule.LastScheduledAt = &scheduledRun.ScheduledAt
		schedule.NextRunAt = scheduledRun.NextRunAt
	}
	schedule.UpdatedAt = now
	_, err = syncStore.SaveBankConnectionSchedule(ctx, *schedule)
	return err
}

func (s *Service) recordBankConnectionSyncFailure(
	ctx context.Context,
	syncStore *bankSyncStoreRef,
	connection *domain.BankConnection,
	params RunBankConnectionSyncParams,
	startedAt time.Time,
	scheduledRun *ProviderScheduledRunMetadata,
	syncErr error,
) error {
	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	connection.LastSyncJobID = strings.TrimSpace(params.JobID)
	connection.LastSyncStartedAt = &startedAt
	connection.LastSyncError = strings.TrimSpace(syncErr.Error())
	connection.UpdatedAt = nowFn().UTC()
	if _, err := syncStore.SaveBankConnection(ctx, *connection); err != nil {
		return err
	}
	schedule, err := syncStore.GetBankConnectionSchedule(ctx, connection.ID)
	if err != nil || schedule == nil {
		return errors.New("run bank connection sync: " + syncErr.Error())
	}
	schedule.LastStartedAt = &startedAt
	schedule.LastJobID = strings.TrimSpace(params.JobID)
	if scheduledRun != nil {
		schedule.LastScheduledAt = &scheduledRun.ScheduledAt
		schedule.NextRunAt = scheduledRun.NextRunAt
	}
	schedule.UpdatedAt = nowFn().UTC()
	if _, err = syncStore.SaveBankConnectionSchedule(ctx, *schedule); err != nil {
		return err
	}
	return errors.New("run bank connection sync: " + syncErr.Error())
}
