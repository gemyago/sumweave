package finance

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
)

type tenantServiceStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
	SaveTenant(ctx context.Context, tenant domain.Tenant) (domain.Tenant, error)
	SaveTenantMembership(
		ctx context.Context,
		membership domain.TenantMembership,
	) (domain.TenantMembership, error)
	ListTenantsForUser(ctx context.Context, userID string) ([]domain.TenantMembershipView, error)
	SaveTenantInvite(ctx context.Context, invite domain.TenantInvite) (domain.TenantInvite, error)
	GetTenantInviteByCode(ctx context.Context, code string) (*domain.TenantInvite, error)
	UpdateTenantInvite(ctx context.Context, invite domain.TenantInvite) (domain.TenantInvite, error)
	ListTenantInvites(ctx context.Context, tenantID string) ([]domain.TenantInvite, error)
	ListTenantMembers(ctx context.Context, tenantID string) ([]domain.TenantMember, error)
	GetTenant(ctx context.Context, tenantID string) (*domain.Tenant, error)
	SaveCategory(ctx context.Context, category domain.Category) (domain.Category, error)
	SaveTag(ctx context.Context, tag domain.Tag) (domain.Tag, error)
}

type TenantService struct {
	store  tenantServiceStore
	access *accessGuard
	now    func() time.Time
	newID  func() string
}

type TenantServiceOption func(*TenantService)

func WithTenantServiceNow(now func() time.Time) TenantServiceOption {
	return func(service *TenantService) {
		service.now = now
	}
}

func WithTenantServiceIDGenerator(newID func() string) TenantServiceOption {
	return func(service *TenantService) {
		service.newID = newID
	}
}

func NewTenantService(store tenantServiceStore, opts ...TenantServiceOption) *TenantService {
	service := &TenantService{
		store:  store,
		access: newAccessGuard(store),
		now:    time.Now,
		newID:  uuid.NewString,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *TenantService) CreateTenant(
	ctx context.Context,
	params CreateTenantParams,
) (domain.Tenant, error) {
	tenantName := strings.TrimSpace(params.Name)
	currency, err := normalizeTenantDisplayCurrency(params.DisplayCurrency)
	if err != nil {
		return domain.Tenant{}, err
	}
	userID := strings.TrimSpace(params.ActorUserID)
	if tenantName == "" || currency == "" || userID == "" {
		return domain.Tenant{}, errors.New(
			"tenant name, display currency, and actor user id are required",
		)
	}

	now := s.now()
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

	if _, saveTenantErr := s.store.SaveTenant(ctx, tenant); saveTenantErr != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant: %w", saveTenantErr)
	}
	if _, saveMembershipErr := s.store.SaveTenantMembership(ctx, membership); saveMembershipErr != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant: %w", saveMembershipErr)
	}
	if params.SeedDefaults {
		for _, seed := range defaultTenantCategorySeeds() {
			_, saveCategoryErr := s.store.SaveCategory(ctx, domain.Category{
				ID:            s.newID(),
				TenantID:      tenant.ID,
				Name:          seed.Name,
				Kind:          seed.Kind,
				SeededDefault: true,
				CreatedAt:     now,
				UpdatedAt:     now,
			})
			if saveCategoryErr != nil {
				return domain.Tenant{}, fmt.Errorf("create tenant: %w", saveCategoryErr)
			}
		}
		for _, seed := range defaultTenantTags() {
			_, saveTagErr := s.store.SaveTag(ctx, domain.Tag{
				ID:        s.newID(),
				TenantID:  tenant.ID,
				Name:      seed,
				CreatedAt: now,
				UpdatedAt: now,
			})
			if saveTagErr != nil {
				return domain.Tenant{}, fmt.Errorf("create tenant: %w", saveTagErr)
			}
		}
	}
	return tenant, nil
}

func (s *TenantService) UpdateTenant(
	ctx context.Context,
	params UpdateTenantParams,
) (domain.Tenant, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.Tenant{}, err
	}

	tenantName := strings.TrimSpace(params.Name)
	if tenantName == "" {
		return domain.Tenant{}, errors.New("tenant name is required")
	}

	currency, err := normalizeTenantDisplayCurrency(params.DisplayCurrency)
	if err != nil {
		return domain.Tenant{}, err
	}

	tenant, err := s.store.GetTenant(ctx, strings.TrimSpace(params.TenantID))
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("update tenant: %w", err)
	}

	now := s.now()
	tenant.Name = tenantName
	tenant.DisplayCurrency = currency
	tenant.UpdatedAt = now

	updated, err := s.store.SaveTenant(ctx, *tenant)
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("update tenant: %w", err)
	}

	return updated, nil
}

func (s *TenantService) ArchiveTenant(
	ctx context.Context,
	params ArchiveTenantParams,
) (domain.Tenant, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.Tenant{}, err
	}
	tenant, err := s.store.GetTenant(ctx, strings.TrimSpace(params.TenantID))
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("archive tenant: %w", err)
	}
	now := s.now()
	tenant.ArchivedAt = &now
	tenant.UpdatedAt = now
	archived, err := s.store.SaveTenant(ctx, *tenant)
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("archive tenant: %w", err)
	}
	return archived, nil
}

func (s *TenantService) ListTenantsForUser(
	ctx context.Context,
	userID string,
) ([]domain.TenantMembershipView, error) {
	views, err := s.store.ListTenantsForUser(ctx, strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list tenants for user: %w", err)
	}
	return views, nil
}

func (s *TenantService) CreateTenantInvite(
	ctx context.Context,
	params CreateTenantInviteParams,
) (domain.TenantInvite, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.TenantInvite{}, err
	}
	now := s.now()
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

func (s *TenantService) AcceptTenantInvite(
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
	now := s.now()
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

func (s *TenantService) ListTenantMembers(
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

func (s *TenantService) ListTenantInvites(
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

func normalizeTenantDisplayCurrency(displayCurrency string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(displayCurrency))
	if code == "" {
		return "", ErrInvalidTenantDisplayCurrency
	}
	if !slices.Contains(supportedTenantDisplayCurrencies(), code) {
		return "", ErrInvalidTenantDisplayCurrency
	}

	return code, nil
}
