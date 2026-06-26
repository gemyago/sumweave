package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
)

type tenantService struct {
	store             serviceStore
	access            *tenantAccessGuard
	now               func() time.Time
	newID             func() string
	defaultCategories []defaultCategorySeed
	defaultTags       []string
}

func newTenantService(
	store serviceStore,
	access *tenantAccessGuard,
	now func() time.Time,
	newID func() string,
	defaultCategories []defaultCategorySeed,
	defaultTags []string,
) *tenantService {
	return &tenantService{
		store:             store,
		access:            access,
		now:               now,
		newID:             newID,
		defaultCategories: append([]defaultCategorySeed{}, defaultCategories...),
		defaultTags:       append([]string{}, defaultTags...),
	}
}

func (s *tenantService) CreateTenant(
	ctx context.Context,
	params CreateTenantParams,
) (domain.Tenant, error) {
	tenantName := strings.TrimSpace(params.Name)
	currency := strings.ToUpper(strings.TrimSpace(params.DisplayCurrency))
	userID := strings.TrimSpace(params.ActorUserID)
	if tenantName == "" || currency == "" || userID == "" {
		return domain.Tenant{}, errors.New(
			"tenant name, display currency, and actor user id are required",
		)
	}

	now := s.now().UTC()
	tenant := domain.Tenant{
		ID:              s.newID(),
		Name:            tenantName,
		DisplayCurrency: currency,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	membership := domain.TenantMembership{
		TenantID:  tenant.ID,
		UserID:    userID,
		JoinedAt:  now,
		CreatedAt: now,
	}

	if _, err := s.store.SaveTenant(ctx, tenant); err != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant: %w", err)
	}
	if _, err := s.store.SaveTenantMembership(ctx, membership); err != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant: %w", err)
	}
	for _, seed := range s.defaultCategories {
		_, err := s.store.SaveCategory(ctx, domain.Category{
			ID:            s.newID(),
			TenantID:      tenant.ID,
			Name:          seed.Name,
			Kind:          seed.Kind,
			SeededDefault: true,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			return domain.Tenant{}, fmt.Errorf("create tenant: %w", err)
		}
	}
	for _, seed := range s.defaultTags {
		_, err := s.store.SaveTag(ctx, domain.Tag{
			ID:        s.newID(),
			TenantID:  tenant.ID,
			Name:      seed,
			CreatedAt: now,
			UpdatedAt: now,
		})
		if err != nil {
			return domain.Tenant{}, fmt.Errorf("create tenant: %w", err)
		}
	}
	return tenant, nil
}

func (s *tenantService) ListTenantsForUser(
	ctx context.Context,
	userID string,
) ([]domain.TenantMembershipView, error) {
	views, err := s.store.ListTenantsForUser(ctx, strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list tenants for user: %w", err)
	}
	return views, nil
}

func (s *tenantService) CreateTenantInvite(
	ctx context.Context,
	params CreateTenantInviteParams,
) (domain.TenantInvite, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.TenantInvite{}, err
	}
	now := s.now().UTC()
	invite := domain.TenantInvite{
		ID:              s.newID(),
		TenantID:        strings.TrimSpace(params.TenantID),
		Code:            s.newID(),
		Recipient:       strings.TrimSpace(params.Recipient),
		CreatedByUserID: strings.TrimSpace(params.ActorUserID),
		CreatedAt:       now,
	}
	created, err := s.store.SaveTenantInvite(ctx, invite)
	if err != nil {
		return domain.TenantInvite{}, fmt.Errorf("create tenant invite: %w", err)
	}
	return created, nil
}

func (s *tenantService) AcceptTenantInvite(
	ctx context.Context,
	params AcceptTenantInviteParams,
) (domain.TenantMembership, error) {
	invite, err := s.store.GetTenantInviteByCode(ctx, strings.TrimSpace(params.Code))
	if err != nil {
		if errors.Is(err, persistence.ErrTenantInviteNotFound) {
			return domain.TenantMembership{}, ErrInviteNotFound
		}
		return domain.TenantMembership{}, fmt.Errorf("accept tenant invite: %w", err)
	}
	if invite.AcceptedAt != nil {
		return domain.TenantMembership{}, ErrInviteAccepted
	}
	now := s.now().UTC()
	membership := domain.TenantMembership{
		TenantID:  invite.TenantID,
		UserID:    strings.TrimSpace(params.ActorUserID),
		JoinedAt:  now,
		CreatedAt: now,
	}
	acceptedByUserID := membership.UserID
	invite.AcceptedByUserID = &acceptedByUserID
	invite.AcceptedAt = &now

	_, err = s.store.SaveTenantMembership(ctx, membership)
	if err != nil {
		return domain.TenantMembership{}, fmt.Errorf("accept tenant invite: %w", err)
	}
	_, err = s.store.UpdateTenantInvite(ctx, *invite)
	if err != nil {
		return domain.TenantMembership{}, fmt.Errorf("accept tenant invite: %w", err)
	}
	return membership, nil
}

func (s *tenantService) ListTenantMembers(
	ctx context.Context,
	params ListTenantMembersParams,
) ([]domain.TenantMember, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	members, err := s.store.ListTenantMembers(ctx, strings.TrimSpace(params.TenantID))
	if err != nil {
		return nil, fmt.Errorf("list tenant members: %w", err)
	}
	return members, nil
}

func (s *tenantService) ListTenantInvites(
	ctx context.Context,
	params ListTenantInvitesParams,
) ([]domain.TenantInvite, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	invites, err := s.store.ListTenantInvites(ctx, strings.TrimSpace(params.TenantID))
	if err != nil {
		return nil, fmt.Errorf("list tenant invites: %w", err)
	}
	return invites, nil
}
