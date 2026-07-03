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

var (
	ErrTenantAccessDenied        = errors.New("tenant access denied")
	ErrInviteNotFound            = errors.New("tenant invite not found")
	ErrInviteAccepted            = errors.New("tenant invite already accepted")
	ErrAccountNotFound           = errors.New("account not found")
	ErrCategoryNotFound          = errors.New("category not found")
	ErrTagNotFound               = errors.New("tag not found")
	ErrTransactionNotFound       = errors.New("transaction not found")
	ErrCSVImportAlreadyConfirmed = errors.New("csv import already confirmed")
	ErrCSVImportAlreadyCompleted = errors.New("csv import already completed")
)

type serviceStore interface {
	SaveTenant(ctx context.Context, tenant domain.Tenant) (domain.Tenant, error)
	SaveTenantMembership(
		ctx context.Context,
		membership domain.TenantMembership,
	) (domain.TenantMembership, error)
	ListTenantsForUser(ctx context.Context, userID string) ([]domain.TenantMembershipView, error)
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
	SaveTenantInvite(ctx context.Context, invite domain.TenantInvite) (domain.TenantInvite, error)
	GetTenantInviteByCode(ctx context.Context, code string) (*domain.TenantInvite, error)
	UpdateTenantInvite(ctx context.Context, invite domain.TenantInvite) (domain.TenantInvite, error)
	ListTenantInvites(ctx context.Context, tenantID string) ([]domain.TenantInvite, error)
	ListTenantMembers(ctx context.Context, tenantID string) ([]domain.TenantMember, error)
	SaveAccount(ctx context.Context, account domain.Account) (domain.Account, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	ListAccounts(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Account, error)
	SaveCategory(ctx context.Context, category domain.Category) (domain.Category, error)
	GetCategory(ctx context.Context, categoryID string) (*domain.Category, error)
	ListCategories(
		ctx context.Context,
		tenantID string,
		includeHidden bool,
	) ([]domain.Category, error)
	SaveTag(ctx context.Context, tag domain.Tag) (domain.Tag, error)
	GetTag(ctx context.Context, tagID string) (*domain.Tag, error)
	ListTags(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Tag, error)
	SaveTransaction(ctx context.Context, transaction domain.Transaction) (domain.Transaction, error)
	SaveLinkedTransferPair(
		ctx context.Context,
		firstTransaction domain.Transaction,
		secondTransaction domain.Transaction,
	) error
	GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)
	ListTransactions(
		ctx context.Context,
		tenantID string,
		accountID string,
		source domain.TransactionSource,
		status domain.TransactionStatus,
		includeHidden bool,
	) ([]domain.Transaction, error)
	GetTenant(ctx context.Context, tenantID string) (*domain.Tenant, error)
	SaveFXRates(ctx context.Context, rates []domain.FXRate) error
	ListFXRates(ctx context.Context, params persistence.ListFXRatesParams) ([]domain.FXRate, error)
	SaveCSVImport(ctx context.Context, record domain.CSVImportRecord) (domain.CSVImportRecord, error)
	GetCSVImport(ctx context.Context, importID string) (*domain.CSVImportRecord, error)
}

type Service struct {
	store                  serviceStore
	now                    func() time.Time
	newID                  func() string
	defaultCategories      []defaultCategorySeed
	defaultTags            []string
	access                 *tenantAccessGuard
	tenants                *tenantService
	catalog                *catalogService
	ledger                 *ledgerService
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

type defaultCategorySeed struct {
	Name string
	Kind domain.CategoryKind
}

func WithNow(now func() time.Time) ServiceOption {
	return func(service *Service) {
		service.now = now
	}
}

func WithIDGenerator(newID func() string) ServiceOption {
	return func(service *Service) {
		service.newID = newID
	}
}

func NewService(store serviceStore, opts ...ServiceOption) *Service {
	service := &Service{
		store: store,
		now:   func() time.Time { return time.Now().UTC() },
		newID: uuid.NewString,
		defaultCategories: []defaultCategorySeed{
			{Name: "Paycheck", Kind: domain.CategoryKindIncome},
			{Name: "Bonus", Kind: domain.CategoryKindIncome},
			{Name: "Interest & Dividends", Kind: domain.CategoryKindIncome},
			{Name: "Business Income", Kind: domain.CategoryKindIncome},
			{Name: "Other Income", Kind: domain.CategoryKindIncome},
			{Name: "Housing", Kind: domain.CategoryKindExpense},
			{Name: "Utilities", Kind: domain.CategoryKindExpense},
			{Name: "Groceries", Kind: domain.CategoryKindExpense},
			{Name: "Dining & Coffee", Kind: domain.CategoryKindExpense},
			{Name: "Transportation", Kind: domain.CategoryKindExpense},
			{Name: "Health & Medical", Kind: domain.CategoryKindExpense},
			{Name: "Insurance", Kind: domain.CategoryKindExpense},
			{Name: "Education & Childcare", Kind: domain.CategoryKindExpense},
			{Name: "Pets", Kind: domain.CategoryKindExpense},
			{Name: "Personal Care", Kind: domain.CategoryKindExpense},
			{Name: "Entertainment", Kind: domain.CategoryKindExpense},
			{Name: "Shopping", Kind: domain.CategoryKindExpense},
			{Name: "Home Improvement & Furnishings", Kind: domain.CategoryKindExpense},
			{Name: "Travel & Vacation", Kind: domain.CategoryKindExpense},
			{Name: "Gifts & Donations", Kind: domain.CategoryKindExpense},
			{Name: "Taxes & Fees", Kind: domain.CategoryKindExpense},
			{Name: "Debt Payments", Kind: domain.CategoryKindExpense},
			{Name: "Miscellaneous", Kind: domain.CategoryKindExpense},
		},
		defaultTags: []string{
			"Tax",
			"Reimburse",
			"Split",
			"Business",
			"Subscription",
			"Travel",
		},
		fxProviders:       defaultFXProviders(),
		defaultFXProvider: FXProviderFrankfurter,
		bankProviders:     map[string]BankConnectionProvider{},
		logger:            slog.New(slog.DiscardHandler),
	}
	for _, opt := range opts {
		opt(service)
	}
	service.bindCoreServices()
	return service
}

func (s *Service) bindCoreServices() {
	s.access = newTenantAccessGuard(s.store)
	s.tenants = newTenantService(
		s.store,
		s.access,
		s.now,
		s.newID,
		s.defaultCategories,
		s.defaultTags,
	)
	s.catalog = newCatalogService(s.store, s.access, s.now, s.newID)
	s.ledger = newLedgerService(s.store, s.access, s.now, s.newID)
}

type CreateTenantParams struct {
	ActorUserID     string
	Name            string
	DisplayCurrency string
}

type ArchiveTenantParams struct {
	ActorUserID string
	TenantID    string
}

type CreateTenantInviteParams struct {
	ActorUserID string
	TenantID    string
	Recipient   string
}

type AcceptTenantInviteParams struct {
	ActorUserID string
	Code        string
}

type ListTenantMembersParams struct {
	ActorUserID string
	TenantID    string
}

type ListTenantInvitesParams struct {
	ActorUserID string
	TenantID    string
}

type CreateAccountParams struct {
	ActorUserID string
	TenantID    string
	Name        string
	Currency    string
	Kind        domain.AccountKind
}

type UpdateAccountParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
	Name        string
}

type HideAccountParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
}

type AttachLinkedAccountParams struct {
	ActorUserID       string
	TenantID          string
	AccountID         string
	Provider          string
	ProviderAccountID string
}

type ListAccountsParams struct {
	ActorUserID   string
	TenantID      string
	IncludeHidden bool
}

type CreateCategoryParams struct {
	ActorUserID string
	TenantID    string
	Name        string
	Kind        domain.CategoryKind
}

type UpdateCategoryParams struct {
	ActorUserID string
	TenantID    string
	CategoryID  string
	Name        string
}

type HideCategoryParams struct {
	ActorUserID string
	TenantID    string
	CategoryID  string
}

type ListCategoriesParams struct {
	ActorUserID   string
	TenantID      string
	IncludeHidden bool
}

type CreateTagParams struct {
	ActorUserID string
	TenantID    string
	Name        string
}

type UpdateTagParams struct {
	ActorUserID string
	TenantID    string
	TagID       string
	Name        string
}

type HideTagParams struct {
	ActorUserID string
	TenantID    string
	TagID       string
}

type ListTagsParams struct {
	ActorUserID   string
	TenantID      string
	IncludeHidden bool
}

type RecordTransactionParams struct {
	ActorUserID      string
	TenantID         string
	AccountID        string
	Source           domain.TransactionSource
	Status           domain.TransactionStatus
	Kind             domain.TransactionKind
	AmountMinor      int64
	Currency         string
	Description      string
	EffectiveAt      time.Time
	CategoryID       string
	TransferGroupID  string
	ProviderOriginal *domain.ProviderTransactionOriginal
}

type UpdateTransactionParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
	Description   string
	AmountMinor   int64
	EffectiveAt   time.Time
	CategoryID    string
}

type HideTransactionParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
}

type GetTransactionParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
}

type LinkTransfersParams struct {
	ActorUserID         string
	TenantID            string
	FirstTransactionID  string
	SecondTransactionID string
}

type ListTransactionsParams struct {
	ActorUserID   string
	TenantID      string
	AccountID     string
	Source        domain.TransactionSource
	Status        domain.TransactionStatus
	IncludeHidden bool
}

type SummarizeTransactionsParams struct {
	ActorUserID string
	TenantID    string
}

type GetAccountBalanceParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
}

func (s *Service) CreateTenant(ctx context.Context, params CreateTenantParams) (domain.Tenant, error) {
	return s.tenants.CreateTenant(ctx, params)
}

func (s *Service) ArchiveTenant(ctx context.Context, params ArchiveTenantParams) (domain.Tenant, error) {
	return s.tenants.ArchiveTenant(ctx, params)
}

func (s *Service) ListTenantsForUser(
	ctx context.Context,
	userID string,
) ([]domain.TenantMembershipView, error) {
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

func (s *Service) HideAccount(ctx context.Context, params HideAccountParams) error {
	return s.catalog.HideAccount(ctx, params)
}

func (s *Service) AttachLinkedAccount(
	ctx context.Context,
	params AttachLinkedAccountParams,
) (domain.Account, error) {
	return s.catalog.AttachLinkedAccount(ctx, params)
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

func (s *Service) HideCategory(ctx context.Context, params HideCategoryParams) error {
	return s.catalog.HideCategory(ctx, params)
}

func (s *Service) ListCategories(
	ctx context.Context,
	params ListCategoriesParams,
) ([]domain.Category, error) {
	return s.catalog.ListCategories(ctx, params)
}

func (s *Service) CreateTag(ctx context.Context, params CreateTagParams) (domain.Tag, error) {
	return s.catalog.CreateTag(ctx, params)
}

func (s *Service) UpdateTag(ctx context.Context, params UpdateTagParams) (domain.Tag, error) {
	return s.catalog.UpdateTag(ctx, params)
}

func (s *Service) HideTag(ctx context.Context, params HideTagParams) error {
	return s.catalog.HideTag(ctx, params)
}

func (s *Service) ListTags(ctx context.Context, params ListTagsParams) ([]domain.Tag, error) {
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

func (s *Service) HideTransaction(ctx context.Context, params HideTransactionParams) error {
	return s.ledger.HideTransaction(ctx, params)
}

func (s *Service) LinkTransfers(ctx context.Context, params LinkTransfersParams) error {
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

func bookedMatchedTransfer(item domain.Transaction) bool {
	if item.HiddenAt != nil ||
		item.Status != domain.TransactionStatusBooked ||
		item.Kind != domain.TransactionKindTransfer {
		return false
	}
	if item.TransferMatchedAt == nil || item.TransferMatchedAt.IsZero() {
		return false
	}
	return item.AmountMinor != 0
}

func existingTransferGroupID(
	firstTransaction domain.Transaction,
	secondTransaction domain.Transaction,
) string {
	if firstTransaction.TransferGroupID != nil {
		if groupID := strings.TrimSpace(*firstTransaction.TransferGroupID); groupID != "" {
			return groupID
		}
	}
	if secondTransaction.TransferGroupID != nil {
		if groupID := strings.TrimSpace(*secondTransaction.TransferGroupID); groupID != "" {
			return groupID
		}
	}
	return ""
}

func (s *Service) requireTenantMember(ctx context.Context, tenantID string, userID string) error {
	return s.access.requireTenantMember(ctx, tenantID, userID)
}
