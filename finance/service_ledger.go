package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/google/uuid"
)

type ledgerServiceStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	GetCategory(ctx context.Context, categoryID string) (*domain.Category, error)
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
}

type LedgerService struct {
	store  ledgerServiceStore
	access *accessGuard
	now    func() time.Time
	newID  func() string
}

type LedgerServiceOption func(*LedgerService)

func WithLedgerServiceNow(now func() time.Time) LedgerServiceOption {
	return func(service *LedgerService) {
		service.now = now
	}
}

func WithLedgerServiceIDGenerator(newID func() string) LedgerServiceOption {
	return func(service *LedgerService) {
		service.newID = newID
	}
}

func NewLedgerService(store ledgerServiceStore, opts ...LedgerServiceOption) *LedgerService {
	service := &LedgerService{
		store:  store,
		access: newAccessGuard(store),
		now:    func() time.Time { return time.Now().UTC() },
		newID:  uuid.NewString,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *LedgerService) requireTenantAccount(
	ctx context.Context,
	tenantID string,
	userID string,
	accountID string,
) (domain.Account, error) {
	if err := s.access.requireTenantMember(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(userID)); err != nil {
		return domain.Account{}, err
	}
	account, err := s.store.GetAccount(ctx, strings.TrimSpace(accountID))
	if err != nil {
		if errors.Is(err, persistence.ErrAccountNotFound) {
			return domain.Account{}, ErrAccountNotFound
		}
		return domain.Account{}, fmt.Errorf("get account: %w", err)
	}
	if account.TenantID != strings.TrimSpace(tenantID) {
		return domain.Account{}, ErrAccountNotFound
	}
	return *account, nil
}

func (s *LedgerService) requireTenantCategory(
	ctx context.Context,
	tenantID string,
	userID string,
	categoryID string,
) (domain.Category, error) {
	if err := s.access.requireTenantMember(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(userID)); err != nil {
		return domain.Category{}, err
	}
	category, err := s.store.GetCategory(ctx, strings.TrimSpace(categoryID))
	if err != nil {
		if errors.Is(err, persistence.ErrCategoryNotFound) {
			return domain.Category{}, ErrCategoryNotFound
		}
		return domain.Category{}, fmt.Errorf("get category: %w", err)
	}
	if category.TenantID != strings.TrimSpace(tenantID) {
		return domain.Category{}, ErrCategoryNotFound
	}
	return *category, nil
}

func (s *LedgerService) requireTenantTransaction(
	ctx context.Context,
	tenantID string,
	userID string,
	transactionID string,
) (domain.Transaction, error) {
	if err := s.access.requireTenantMember(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(userID)); err != nil {
		return domain.Transaction{}, err
	}
	txn, err := s.store.GetTransaction(ctx, strings.TrimSpace(transactionID))
	if err != nil {
		if errors.Is(err, persistence.ErrTransactionNotFound) {
			return domain.Transaction{}, ErrTransactionNotFound
		}
		return domain.Transaction{}, fmt.Errorf("get transaction: %w", err)
	}
	if txn.TenantID != strings.TrimSpace(tenantID) {
		return domain.Transaction{}, ErrTransactionNotFound
	}
	return *txn, nil
}

func (s *LedgerService) RecordTransaction(
	ctx context.Context,
	params RecordTransactionParams,
) (domain.Transaction, error) {
	account, err := s.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
	if err != nil {
		return domain.Transaction{}, err
	}
	var categoryID *string
	if trimmedCategoryID := strings.TrimSpace(params.CategoryID); trimmedCategoryID != "" {
		category, categoryErr := s.requireTenantCategory(
			ctx,
			params.TenantID,
			params.ActorUserID,
			trimmedCategoryID,
		)
		if categoryErr != nil {
			return domain.Transaction{}, categoryErr
		}
		categoryID = &category.ID
	}
	var transferGroupID *string
	if trimmedTransferGroupID := strings.TrimSpace(params.TransferGroupID); trimmedTransferGroupID != "" {
		transferGroupID = &trimmedTransferGroupID
	}
	now := s.now().UTC()
	txn := domain.Transaction{
		ID:               s.newID(),
		TenantID:         account.TenantID,
		AccountID:        account.ID,
		Source:           params.Source,
		Status:           params.Status,
		Kind:             params.Kind,
		AmountMinor:      params.AmountMinor,
		Currency:         strings.ToUpper(strings.TrimSpace(params.Currency)),
		Description:      strings.TrimSpace(params.Description),
		EffectiveAt:      params.EffectiveAt.UTC(),
		CategoryID:       categoryID,
		TransferGroupID:  transferGroupID,
		CreatedAt:        now,
		UpdatedAt:        now,
		ProviderOriginal: params.ProviderOriginal,
	}
	saved, err := s.store.SaveTransaction(ctx, txn)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("record transaction: %w", err)
	}
	return saved, nil
}

func (s *LedgerService) UpdateTransaction(
	ctx context.Context,
	params UpdateTransactionParams,
) (domain.Transaction, error) {
	txn, err := s.requireTenantTransaction(ctx, params.TenantID, params.ActorUserID, params.TransactionID)
	if err != nil {
		return domain.Transaction{}, err
	}
	var categoryID *string
	if trimmedCategoryID := strings.TrimSpace(params.CategoryID); trimmedCategoryID != "" {
		category, categoryErr := s.requireTenantCategory(
			ctx,
			params.TenantID,
			params.ActorUserID,
			trimmedCategoryID,
		)
		if categoryErr != nil {
			return domain.Transaction{}, categoryErr
		}
		categoryID = &category.ID
	}
	txn.Description = strings.TrimSpace(params.Description)
	txn.AmountMinor = params.AmountMinor
	txn.EffectiveAt = params.EffectiveAt.UTC()
	txn.CategoryID = categoryID
	txn.UpdatedAt = s.now().UTC()
	saved, err := s.store.SaveTransaction(ctx, txn)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("update transaction: %w", err)
	}
	return saved, nil
}

func (s *LedgerService) GetTransaction(
	ctx context.Context,
	params GetTransactionParams,
) (domain.Transaction, error) {
	txn, err := s.requireTenantTransaction(ctx, params.TenantID, params.ActorUserID, params.TransactionID)
	if err != nil {
		return domain.Transaction{}, err
	}
	return txn, nil
}

func (s *LedgerService) HideTransaction(ctx context.Context, params HideTransactionParams) error {
	txn, err := s.requireTenantTransaction(ctx, params.TenantID, params.ActorUserID, params.TransactionID)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	txn.HiddenAt = &now
	txn.UpdatedAt = now
	_, err = s.store.SaveTransaction(ctx, txn)
	if err != nil {
		return fmt.Errorf("hide transaction: %w", err)
	}
	return nil
}

func (s *LedgerService) LinkTransfers(ctx context.Context, params LinkTransfersParams) error {
	firstTransaction, err := s.requireTenantTransaction(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.FirstTransactionID,
	)
	if err != nil {
		return err
	}
	secondTransaction, err := s.requireTenantTransaction(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.SecondTransactionID,
	)
	if err != nil {
		return err
	}

	transferGroupID := existingTransferGroupID(firstTransaction, secondTransaction)
	if transferGroupID == "" {
		transferGroupID = s.newID()
	}

	now := s.now().UTC()
	firstTransaction.TransferGroupID = &transferGroupID
	firstTransaction.TransferMatchedAt = &now
	firstTransaction.UpdatedAt = now
	secondTransaction.TransferGroupID = &transferGroupID
	secondTransaction.TransferMatchedAt = &now
	secondTransaction.UpdatedAt = now
	if saveErr := s.store.SaveLinkedTransferPair(ctx, firstTransaction, secondTransaction); saveErr != nil {
		return fmt.Errorf("link transfers: %w", saveErr)
	}

	return nil
}

func (s *LedgerService) ListTransactions(
	ctx context.Context,
	params ListTransactionsParams,
) ([]domain.Transaction, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListTransactions(
		ctx,
		strings.TrimSpace(params.TenantID),
		strings.TrimSpace(params.AccountID),
		params.Source,
		params.Status,
		params.IncludeHidden,
	)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	return items, nil
}

func (s *LedgerService) GetAccountBalance(
	ctx context.Context,
	params GetAccountBalanceParams,
) (domain.AccountBalance, error) {
	account, err := s.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
	if err != nil {
		return domain.AccountBalance{}, err
	}
	items, err := s.store.ListTransactions(ctx, account.TenantID, account.ID, "", "", false)
	if err != nil {
		return domain.AccountBalance{}, fmt.Errorf("get account balance: %w", err)
	}
	balance := domain.AccountBalance{AccountID: account.ID, Currency: account.Currency}
	for _, item := range items {
		if item.HiddenAt != nil {
			continue
		}
		if item.Status == domain.TransactionStatusBooked {
			balance.BookedBalanceMinor += item.AmountMinor
			continue
		}
		balance.PendingBalanceMinor += item.AmountMinor
	}
	return balance, nil
}

func (s *LedgerService) SummarizeTransactions(
	ctx context.Context,
	params SummarizeTransactionsParams,
) (domain.TransactionSummary, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.TransactionSummary{}, err
	}
	items, err := s.store.ListTransactions(ctx, strings.TrimSpace(params.TenantID), "", "", "", false)
	if err != nil {
		return domain.TransactionSummary{}, fmt.Errorf("summarize transactions: %w", err)
	}
	return summarizeBookedTransactions(items), nil
}

func summarizeBookedTransactions(items []domain.Transaction) domain.TransactionSummary {
	summary := domain.TransactionSummary{}
	for _, item := range items {
		if item.HiddenAt != nil || item.Status != domain.TransactionStatusBooked {
			continue
		}
		switch item.Kind {
		case domain.TransactionKindTransfer:
			if bookedMatchedTransfer(item) {
				continue
			}
			if item.AmountMinor > 0 {
				summary.IncomeMinor += item.AmountMinor
			} else if item.AmountMinor < 0 {
				summary.ExpenseMinor += -item.AmountMinor
			}
		case domain.TransactionKindReconciliation,
			domain.TransactionKindOpeningBalance:
			continue
		case domain.TransactionKindRefund:
			summary.ExpenseMinor -= item.AmountMinor
		case domain.TransactionKindRegular:
			if item.AmountMinor > 0 {
				summary.IncomeMinor += item.AmountMinor
			} else if item.AmountMinor < 0 {
				summary.ExpenseMinor += -item.AmountMinor
			}
		}
	}
	summary.NetMinor = summary.IncomeMinor - summary.ExpenseMinor
	return summary
}
