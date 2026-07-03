package finance

import (
	"context"
	"fmt"
)

type accessGuardStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
}

type accessGuard struct {
	store accessGuardStore
}

func newAccessGuard(
	store accessGuardStore,
) *accessGuard {
	return &accessGuard{store: store}
}

func (g *accessGuard) requireTenantMember(
	ctx context.Context,
	tenantID string,
	userID string,
) error {
	allowed, err := g.store.IsTenantMember(
		ctx,
		tenantID,
		userID,
	)
	if err != nil {
		return fmt.Errorf("check tenant membership: %w", err)
	}
	if !allowed {
		return ErrTenantAccessDenied
	}
	return nil
}
