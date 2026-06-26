package finance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

type catalogService struct {
	store  serviceStore
	access *tenantAccessGuard
	now    func() time.Time
	newID  func() string
}

func newCatalogService(
	store serviceStore,
	access *tenantAccessGuard,
	now func() time.Time,
	newID func() string,
) *catalogService {
	return &catalogService{store: store, access: access, now: now, newID: newID}
}

func (s *catalogService) CreateAccount(
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

func (s *catalogService) UpdateAccount(
	ctx context.Context,
	params UpdateAccountParams,
) (domain.Account, error) {
	account, err := s.access.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
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

func (s *catalogService) HideAccount(ctx context.Context, params HideAccountParams) error {
	account, err := s.access.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
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

func (s *catalogService) AttachLinkedAccount(
	ctx context.Context,
	params AttachLinkedAccountParams,
) (domain.Account, error) {
	account, err := s.access.requireTenantAccount(ctx, params.TenantID, params.ActorUserID, params.AccountID)
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

func (s *catalogService) ListAccounts(
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
	return accounts, nil
}

func (s *catalogService) CreateCategory(
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

func (s *catalogService) UpdateCategory(
	ctx context.Context,
	params UpdateCategoryParams,
) (domain.Category, error) {
	category, err := s.access.requireTenantCategory(ctx, params.TenantID, params.ActorUserID, params.CategoryID)
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

func (s *catalogService) HideCategory(ctx context.Context, params HideCategoryParams) error {
	category, err := s.access.requireTenantCategory(ctx, params.TenantID, params.ActorUserID, params.CategoryID)
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

func (s *catalogService) ListCategories(
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

func (s *catalogService) CreateTag(ctx context.Context, params CreateTagParams) (domain.Tag, error) {
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

func (s *catalogService) UpdateTag(ctx context.Context, params UpdateTagParams) (domain.Tag, error) {
	tag, err := s.access.requireTenantTag(ctx, params.TenantID, params.ActorUserID, params.TagID)
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

func (s *catalogService) HideTag(ctx context.Context, params HideTagParams) error {
	tag, err := s.access.requireTenantTag(ctx, params.TenantID, params.ActorUserID, params.TagID)
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

func (s *catalogService) ListTags(ctx context.Context, params ListTagsParams) ([]domain.Tag, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListTags(ctx, strings.TrimSpace(params.TenantID), params.IncludeHidden)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return items, nil
}
