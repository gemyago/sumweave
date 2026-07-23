package finance

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/google/uuid"
)

type ledgerServiceStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	ListAccounts(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Account, error)
	GetCategory(ctx context.Context, categoryID string) (*domain.Category, error)
	GetTag(ctx context.Context, tagID string) (*domain.Tag, error)
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
		page ...persistence.ListTransactionsPage,
	) ([]domain.Transaction, error)
}

type ledgerTransactionStore interface {
	SaveTransaction(ctx context.Context, transaction domain.Transaction) (domain.Transaction, error)
	GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)
	ListTransactions(
		ctx context.Context,
		tenantID string,
		accountID string,
		source domain.TransactionSource,
		status domain.TransactionStatus,
		includeHidden bool,
		page ...persistence.ListTransactionsPage,
	) ([]domain.Transaction, error)
}

type LedgerService struct {
	store        ledgerServiceStore
	transactions ledgerTransactionStore
	balanceStore accountBalanceReadStore
	access       *accessGuard
	now          func() time.Time
	newID        func() string
}

type LedgerServiceOption func(*LedgerService)

func WithLedgerServiceAccountBalanceStore(store accountBalanceReadStore) LedgerServiceOption {
	return func(service *LedgerService) {
		service.balanceStore = store
	}
}

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

func WithLedgerServiceTransactionStore(store ledgerTransactionStore) LedgerServiceOption {
	return func(service *LedgerService) {
		service.transactions = store
	}
}

func NewLedgerService(store ledgerServiceStore, opts ...LedgerServiceOption) *LedgerService {
	service := &LedgerService{
		store:        store,
		transactions: store,
		access:       newAccessGuard(store),
		now:          time.Now,
		newID:        uuid.NewString,
	}
	for _, opt := range opts {
		opt(service)
	}
	assignAccountBalanceReadStore(store, &service.balanceStore)
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
	txn, err := s.transactions.GetTransaction(ctx, strings.TrimSpace(transactionID))
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

func (s *LedgerService) requireTenantTagIDs(
	ctx context.Context,
	tenantID string,
	tagIDs []string,
) ([]string, error) {
	result := make([]string, 0, len(tagIDs))
	seen := make(map[string]struct{}, len(tagIDs))
	for _, tagID := range tagIDs {
		trimmedTagID := strings.TrimSpace(tagID)
		if _, ok := seen[trimmedTagID]; ok {
			return nil, ErrDuplicateTagID
		}
		seen[trimmedTagID] = struct{}{}
		tag, err := s.store.GetTag(ctx, trimmedTagID)
		if err != nil {
			if errors.Is(err, persistence.ErrTagNotFound) {
				return nil, ErrTagNotAssignable
			}
			return nil, fmt.Errorf("get tag: %w", err)
		}
		if tag.TenantID != strings.TrimSpace(tenantID) || tag.HiddenAt != nil {
			return nil, ErrTagNotAssignable
		}
		result = append(result, tag.ID)
	}
	sort.Strings(result)
	return result, nil
}

func (s *LedgerService) RecordTransaction(
	ctx context.Context,
	params RecordTransactionParams,
) (domain.Transaction, error) {
	if err := validateLedgerTimestamp(params.EffectiveAt); err != nil {
		return domain.Transaction{}, err
	}
	account, err := s.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
	if err != nil {
		return domain.Transaction{}, err
	}
	if account.HiddenAt != nil {
		return domain.Transaction{}, ErrHiddenAccount
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
	tagIDs, err := s.requireTenantTagIDs(ctx, params.TenantID, params.TagIDs)
	if err != nil {
		return domain.Transaction{}, err
	}
	now := s.now()
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
		EffectiveAt:      params.EffectiveAt,
		CategoryID:       categoryID,
		TagIDs:           tagIDs,
		TransferGroupID:  transferGroupID,
		CreatedAt:        now,
		UpdatedAt:        now,
		ProviderOriginal: params.ProviderOriginal,
	}
	saved, err := s.transactions.SaveTransaction(ctx, txn)
	if err != nil {
		if errors.Is(err, persistence.ErrTagNotFound) || errors.Is(err, persistence.ErrDuplicateTransactionTag) {
			return domain.Transaction{}, ErrTagNotAssignable
		}
		return domain.Transaction{}, fmt.Errorf("record transaction: %w", err)
	}
	return saved, nil
}

func (s *LedgerService) UpdateTransaction(
	ctx context.Context,
	params UpdateTransactionParams,
) (domain.Transaction, error) {
	if params.EffectiveAt != nil {
		if err := validateLedgerTimestamp(*params.EffectiveAt); err != nil {
			return domain.Transaction{}, err
		}
	}
	txn, err := s.requireTenantTransaction(ctx, params.TenantID, params.ActorUserID, params.TransactionID)
	if err != nil {
		return domain.Transaction{}, err
	}
	if params.ClearCategory {
		txn.CategoryID = nil
	} else if params.CategoryID != "" {
		category, categoryErr := s.requireTenantCategory(
			ctx,
			params.TenantID,
			params.ActorUserID,
			params.CategoryID,
		)
		if categoryErr != nil {
			return domain.Transaction{}, categoryErr
		}
		txn.CategoryID = &category.ID
	}
	tagIDs, err := s.requireTenantTagIDs(ctx, params.TenantID, params.TagIDs)
	if err != nil {
		return domain.Transaction{}, err
	}
	txn.Description = strings.TrimSpace(params.Description)
	txn.AmountMinor = params.AmountMinor
	txn.TagIDs = tagIDs
	if params.EffectiveAt != nil {
		txn.EffectiveAt = *params.EffectiveAt
	}
	txn.UpdatedAt = s.now()
	saved, err := s.transactions.SaveTransaction(ctx, txn)
	if err != nil {
		if errors.Is(err, persistence.ErrTagNotFound) || errors.Is(err, persistence.ErrDuplicateTransactionTag) {
			return domain.Transaction{}, ErrTagNotAssignable
		}
		return domain.Transaction{}, fmt.Errorf("update transaction: %w", err)
	}
	return saved, nil
}

func validateLedgerTimestamp(value time.Time) error {
	if value.IsZero() {
		return errors.New("effectiveAt must be non-zero")
	}
	return nil
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
	now := s.now()
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

	if validationErr := validateTransferPair(firstTransaction, secondTransaction); validationErr != nil {
		return validationErr
	}

	transferGroupID := s.newID()
	now := s.now()
	firstTransaction.Kind = domain.TransactionKindTransfer
	firstTransaction.TransferGroupID = &transferGroupID
	firstTransaction.TransferMatchedAt = &now
	firstTransaction.UpdatedAt = now
	secondTransaction.Kind = domain.TransactionKindTransfer
	secondTransaction.TransferGroupID = &transferGroupID
	secondTransaction.TransferMatchedAt = &now
	secondTransaction.UpdatedAt = now
	if saveErr := s.store.SaveLinkedTransferPair(ctx, firstTransaction, secondTransaction); saveErr != nil {
		return fmt.Errorf("link transfers: %w", saveErr)
	}

	return nil
}

func (s *LedgerService) UnlinkTransfers(ctx context.Context, params UnlinkTransfersParams) error {
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

	if !linkedTransferPair(firstTransaction, secondTransaction) {
		return ErrTransferNotLinked
	}

	now := s.now()
	firstTransaction.Kind = domain.TransactionKindRegular
	firstTransaction.TransferGroupID = nil
	firstTransaction.TransferMatchedAt = nil
	firstTransaction.UpdatedAt = now
	secondTransaction.Kind = domain.TransactionKindRegular
	secondTransaction.TransferGroupID = nil
	secondTransaction.TransferMatchedAt = nil
	secondTransaction.UpdatedAt = now
	if saveErr := s.store.SaveLinkedTransferPair(ctx, firstTransaction, secondTransaction); saveErr != nil {
		return fmt.Errorf("unlink transfers: %w", saveErr)
	}

	return nil
}

func validateTransferPair(firstTransaction domain.Transaction, secondTransaction domain.Transaction) error {
	if firstTransaction.ID == secondTransaction.ID ||
		firstTransaction.AccountID == secondTransaction.AccountID ||
		firstTransaction.Status != domain.TransactionStatusBooked ||
		secondTransaction.Status != domain.TransactionStatusBooked ||
		firstTransaction.AmountMinor == 0 ||
		secondTransaction.AmountMinor == 0 ||
		(firstTransaction.AmountMinor > 0) == (secondTransaction.AmountMinor > 0) ||
		firstTransaction.TransferGroupID != nil ||
		secondTransaction.TransferGroupID != nil ||
		firstTransaction.TransferMatchedAt != nil ||
		secondTransaction.TransferMatchedAt != nil {
		return ErrInvalidTransferPair
	}
	return nil
}

func linkedTransferPair(firstTransaction domain.Transaction, secondTransaction domain.Transaction) bool {
	if firstTransaction.ID == secondTransaction.ID ||
		firstTransaction.Kind != domain.TransactionKindTransfer ||
		secondTransaction.Kind != domain.TransactionKindTransfer ||
		firstTransaction.TransferGroupID == nil ||
		secondTransaction.TransferGroupID == nil ||
		firstTransaction.TransferMatchedAt == nil ||
		secondTransaction.TransferMatchedAt == nil {
		return false
	}
	return *firstTransaction.TransferGroupID == *secondTransaction.TransferGroupID
}

func (s *LedgerService) ListTransactions(
	ctx context.Context,
	params ListTransactionsParams,
) ([]domain.Transaction, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.transactions.ListTransactions(
		ctx,
		strings.TrimSpace(params.TenantID),
		strings.TrimSpace(params.AccountID),
		params.Source,
		params.Status,
		params.IncludeHidden,
		persistence.ListTransactionsPage{Limit: params.Limit, Offset: params.Offset},
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
	items, err := s.balanceStore.ListAccountBalances(ctx, persistence.ListAccountBalancesParams{
		TenantID:   account.TenantID,
		AccountIDs: []string{account.ID},
	})
	if err != nil {
		return domain.AccountBalance{}, fmt.Errorf("get account balance: %w", err)
	}
	balance := domain.AccountBalance{AccountID: account.ID, Currency: account.Currency}
	if len(items) > 0 {
		balance.BookedBalanceMinor = items[0].BookedBalanceMinor
		balance.PendingBalanceMinor = items[0].PendingBalanceMinor
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
	accounts, err := s.store.ListAccounts(ctx, strings.TrimSpace(params.TenantID), false)
	if err != nil {
		return domain.TransactionSummary{}, fmt.Errorf("list active accounts for transaction summary: %w", err)
	}
	return summarizeBookedTransactions(transactionsForAccounts(items, accounts)), nil
}

func transactionsForAccounts(items []domain.Transaction, accounts []domain.Account) []domain.Transaction {
	activeAccountIDs := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		activeAccountIDs[account.ID] = struct{}{}
	}
	filtered := make([]domain.Transaction, 0, len(items))
	for _, item := range items {
		if _, active := activeAccountIDs[item.AccountID]; active {
			filtered = append(filtered, item)
		}
	}
	return filtered
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
		case domain.TransactionKindExpense,
			domain.TransactionKindIncome,
			domain.TransactionKindRegular:
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
