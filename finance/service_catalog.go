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

type catalogServiceStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
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
}

type CatalogService struct {
	store        catalogServiceStore
	balanceStore accountBalanceReadStore
	access       *accessGuard
	now          func() time.Time
	newID        func() string
}

type CatalogServiceOption func(*CatalogService)

func WithCatalogServiceAccountBalanceStore(store accountBalanceReadStore) CatalogServiceOption {
	return func(service *CatalogService) {
		service.balanceStore = store
	}
}

func WithCatalogServiceNow(now func() time.Time) CatalogServiceOption {
	return func(service *CatalogService) {
		service.now = now
	}
}

func WithCatalogServiceIDGenerator(newID func() string) CatalogServiceOption {
	return func(service *CatalogService) {
		service.newID = newID
	}
}

func NewCatalogService(store catalogServiceStore, opts ...CatalogServiceOption) *CatalogService {
	service := &CatalogService{
		store:  store,
		access: newAccessGuard(store),
		now:    func() time.Time { return time.Now().UTC() },
		newID:  uuid.NewString,
	}
	for _, opt := range opts {
		opt(service)
	}
	assignAccountBalanceReadStore(store, &service.balanceStore)
	return service
}

func (s *CatalogService) GetAccount(
	ctx context.Context,
	params GetAccountParams,
) (domain.Account, error) {
	account, err := s.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
	if err != nil {
		return domain.Account{}, err
	}
	balances, err := s.balanceStore.ListAccountBalances(ctx, persistence.ListAccountBalancesParams{
		TenantID:   account.TenantID,
		AccountIDs: []string{account.ID},
	})
	if err != nil {
		return domain.Account{}, fmt.Errorf("get account balance aggregate: %w", err)
	}
	if len(balances) > 0 {
		account.BookedBalanceMinor = balances[0].BookedBalanceMinor
		account.PendingBalanceMinor = balances[0].PendingBalanceMinor
	}
	return account, nil
}

func (s *CatalogService) requireTenantAccount(
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

func (s *CatalogService) requireTenantCategory(
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

func (s *CatalogService) requireTenantTag(
	ctx context.Context,
	tenantID string,
	userID string,
	tagID string,
) (domain.Tag, error) {
	if err := s.access.requireTenantMember(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(userID)); err != nil {
		return domain.Tag{}, err
	}
	tag, err := s.store.GetTag(ctx, strings.TrimSpace(tagID))
	if err != nil {
		if errors.Is(err, persistence.ErrTagNotFound) {
			return domain.Tag{}, ErrTagNotFound
		}
		return domain.Tag{}, fmt.Errorf("get tag: %w", err)
	}
	if tag.TenantID != strings.TrimSpace(tenantID) {
		return domain.Tag{}, ErrTagNotFound
	}
	return *tag, nil
}

func (s *CatalogService) CreateAccount(
	ctx context.Context,
	params CreateAccountParams,
) (domain.Account, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.Account{}, err
	}
	now := s.now().UTC()
	account := domain.Account{
		ID:        s.newID(),
		TenantID:  strings.TrimSpace(params.TenantID),
		Name:      strings.TrimSpace(params.Name),
		Currency:  strings.ToUpper(strings.TrimSpace(params.Currency)),
		Kind:      params.Kind,
		CreatedAt: now,
		UpdatedAt: now,
	}
	saved, err := s.store.SaveAccount(ctx, account)
	if err != nil {
		return domain.Account{}, fmt.Errorf("create account: %w", err)
	}
	return saved, nil
}

func (s *CatalogService) UpdateAccount(
	ctx context.Context,
	params UpdateAccountParams,
) (domain.Account, error) {
	account, err := s.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
	if err != nil {
		return domain.Account{}, err
	}
	account.Name = strings.TrimSpace(params.Name)
	account.UpdatedAt = s.now().UTC()
	saved, err := s.store.SaveAccount(ctx, account)
	if err != nil {
		return domain.Account{}, fmt.Errorf("update account: %w", err)
	}
	return saved, nil
}

func (s *CatalogService) HideAccount(ctx context.Context, params HideAccountParams) error {
	account, err := s.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	account.HiddenAt = &now
	account.UpdatedAt = now
	_, err = s.store.SaveAccount(ctx, account)
	if err != nil {
		return fmt.Errorf("hide account: %w", err)
	}
	return nil
}

func (s *CatalogService) AttachLinkedAccount(
	ctx context.Context,
	params AttachLinkedAccountParams,
) (domain.Account, error) {
	account, err := s.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
	if err != nil {
		return domain.Account{}, err
	}
	account.Kind = domain.AccountKindLinked
	account.LinkedAccount = &domain.LinkedAccount{
		Provider:          strings.TrimSpace(params.Provider),
		ProviderAccountID: strings.TrimSpace(params.ProviderAccountID),
	}
	account.UpdatedAt = s.now().UTC()
	saved, err := s.store.SaveAccount(ctx, account)
	if err != nil {
		return domain.Account{}, fmt.Errorf("attach linked account: %w", err)
	}
	return saved, nil
}

func (s *CatalogService) ListAccounts(
	ctx context.Context,
	params ListAccountsParams,
) ([]domain.Account, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	accounts, err := s.store.ListAccounts(ctx, strings.TrimSpace(params.TenantID), params.IncludeHidden)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	balances, err := s.balanceStore.ListAccountBalances(ctx, persistence.ListAccountBalancesParams{
		TenantID:   strings.TrimSpace(params.TenantID),
		AccountIDs: accountIDs(accounts),
	})
	if err != nil {
		return nil, fmt.Errorf("list account balances: %w", err)
	}
	applyAccountBalances(accounts, balances)
	return accounts, nil
}

func accountIDs(items []domain.Account) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func applyAccountBalances(accounts []domain.Account, balances []domain.AccountBalance) {
	balanceByAccountID := make(map[string]domain.AccountBalance, len(balances))
	for _, balance := range balances {
		balanceByAccountID[balance.AccountID] = balance
	}
	for index := range accounts {
		balance := balanceByAccountID[accounts[index].ID]
		accounts[index].BookedBalanceMinor = balance.BookedBalanceMinor
		accounts[index].PendingBalanceMinor = balance.PendingBalanceMinor
	}
}

func (s *CatalogService) CreateCategory(
	ctx context.Context,
	params CreateCategoryParams,
) (domain.Category, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.Category{}, err
	}
	now := s.now().UTC()
	category := domain.Category{
		ID:        s.newID(),
		TenantID:  strings.TrimSpace(params.TenantID),
		Name:      strings.TrimSpace(params.Name),
		Kind:      params.Kind,
		CreatedAt: now,
		UpdatedAt: now,
	}
	saved, err := s.store.SaveCategory(ctx, category)
	if err != nil {
		return domain.Category{}, fmt.Errorf("create category: %w", err)
	}
	return saved, nil
}

func (s *CatalogService) UpdateCategory(
	ctx context.Context,
	params UpdateCategoryParams,
) (domain.Category, error) {
	category, err := s.requireTenantCategory(ctx, params.TenantID, params.ActorUserID, params.CategoryID)
	if err != nil {
		return domain.Category{}, err
	}
	category.Name = strings.TrimSpace(params.Name)
	category.UpdatedAt = s.now().UTC()
	saved, err := s.store.SaveCategory(ctx, category)
	if err != nil {
		return domain.Category{}, fmt.Errorf("update category: %w", err)
	}
	return saved, nil
}

func (s *CatalogService) HideCategory(ctx context.Context, params HideCategoryParams) error {
	category, err := s.requireTenantCategory(ctx, params.TenantID, params.ActorUserID, params.CategoryID)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	category.HiddenAt = &now
	category.UpdatedAt = now
	_, err = s.store.SaveCategory(ctx, category)
	if err != nil {
		return fmt.Errorf("hide category: %w", err)
	}
	return nil
}

func (s *CatalogService) ListCategories(
	ctx context.Context,
	params ListCategoriesParams,
) ([]domain.Category, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListCategories(ctx, strings.TrimSpace(params.TenantID), params.IncludeHidden)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return items, nil
}

func (s *CatalogService) CreateTag(ctx context.Context, params CreateTagParams) (domain.Tag, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.Tag{}, err
	}
	now := s.now().UTC()
	tag := domain.Tag{
		ID:        s.newID(),
		TenantID:  strings.TrimSpace(params.TenantID),
		Name:      strings.TrimSpace(params.Name),
		CreatedAt: now,
		UpdatedAt: now,
	}
	saved, err := s.store.SaveTag(ctx, tag)
	if err != nil {
		return domain.Tag{}, fmt.Errorf("create tag: %w", err)
	}
	return saved, nil
}

func (s *CatalogService) UpdateTag(ctx context.Context, params UpdateTagParams) (domain.Tag, error) {
	tag, err := s.requireTenantTag(ctx, params.TenantID, params.ActorUserID, params.TagID)
	if err != nil {
		return domain.Tag{}, err
	}
	tag.Name = strings.TrimSpace(params.Name)
	tag.UpdatedAt = s.now().UTC()
	saved, err := s.store.SaveTag(ctx, tag)
	if err != nil {
		return domain.Tag{}, fmt.Errorf("update tag: %w", err)
	}
	return saved, nil
}

func (s *CatalogService) HideTag(ctx context.Context, params HideTagParams) error {
	tag, err := s.requireTenantTag(ctx, params.TenantID, params.ActorUserID, params.TagID)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	tag.HiddenAt = &now
	tag.UpdatedAt = now
	_, err = s.store.SaveTag(ctx, tag)
	if err != nil {
		return fmt.Errorf("hide tag: %w", err)
	}
	return nil
}

func (s *CatalogService) ListTags(ctx context.Context, params ListTagsParams) ([]domain.Tag, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListTags(ctx, strings.TrimSpace(params.TenantID), params.IncludeHidden)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return items, nil
}
