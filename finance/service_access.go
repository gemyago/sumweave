package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
)

type tenantAccessGuard struct {
	store serviceStore
}

func newTenantAccessGuard(store serviceStore) *tenantAccessGuard {
	return &tenantAccessGuard{store: store}
}

func (g *tenantAccessGuard) requireTenantMember(ctx context.Context, tenantID string, userID string) error {
	allowed, err := g.store.IsTenantMember(
		ctx,
		strings.TrimSpace(tenantID),
		strings.TrimSpace(userID),
	)
	if err != nil {
		return fmt.Errorf("check tenant membership: %w", err)
	}
	if !allowed {
		return ErrTenantAccessDenied
	}
	return nil
}

func (g *tenantAccessGuard) requireTenantAccount(
	ctx context.Context,
	tenantID string,
	userID string,
	accountID string,
) (domain.Account, error) {
	if err := g.requireTenantMember(ctx, tenantID, userID); err != nil {
		return domain.Account{}, err
	}
	account, err := g.store.GetAccount(ctx, strings.TrimSpace(accountID))
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

func (g *tenantAccessGuard) requireTenantCategory(
	ctx context.Context,
	tenantID string,
	userID string,
	categoryID string,
) (domain.Category, error) {
	if err := g.requireTenantMember(ctx, tenantID, userID); err != nil {
		return domain.Category{}, err
	}
	category, err := g.store.GetCategory(ctx, strings.TrimSpace(categoryID))
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

func (g *tenantAccessGuard) requireTenantTag(
	ctx context.Context,
	tenantID string,
	userID string,
	tagID string,
) (domain.Tag, error) {
	if err := g.requireTenantMember(ctx, tenantID, userID); err != nil {
		return domain.Tag{}, err
	}
	tag, err := g.store.GetTag(ctx, strings.TrimSpace(tagID))
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

func (g *tenantAccessGuard) requireTenantTransaction(
	ctx context.Context,
	tenantID string,
	userID string,
	transactionID string,
) (domain.Transaction, error) {
	if err := g.requireTenantMember(ctx, tenantID, userID); err != nil {
		return domain.Transaction{}, err
	}
	txn, err := g.store.GetTransaction(ctx, strings.TrimSpace(transactionID))
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
