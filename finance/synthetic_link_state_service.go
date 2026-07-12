package finance

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/google/uuid"
)

var (
	ErrPendingSyntheticLinkStateNotFound          = errors.New("pending synthetic link state not found")
	ErrSyntheticConfiguredAccountNameRequired     = errors.New("synthetic configured account name is required")
	ErrSyntheticConfiguredAccountCurrencyRequired = errors.New("synthetic configured account currency is required")
)

type GetPendingSyntheticLinkStateParams struct {
	ActorUserID string
	TenantID    string
	State       string
}

type SavePendingSyntheticLinkStateParams struct {
	ActorUserID        string
	TenantID           string
	State              string
	ConfiguredAccounts []SyntheticLinkStateAccount
}

type SyntheticLinkStateAccount struct {
	Key      string
	Name     string
	Currency string
}

type PendingSyntheticLinkState struct {
	Provider           string
	State              string
	ConfiguredAccounts []SyntheticLinkStateAccount
	CanFinish          bool
}

type syntheticPendingStartLookup interface {
	GetPendingSyntheticStart(
		ctx context.Context,
		tenantID string,
		actorUserID string,
		state string,
		now time.Time,
	) (*domain.PendingBankConnectionLinkStart, error)
}

type syntheticProviderStateStore interface {
	SaveSyntheticProviderState(
		ctx context.Context,
		state domain.SyntheticProviderState,
	) (domain.SyntheticProviderState, error)
	GetSyntheticProviderState(
		ctx context.Context,
		providerReference string,
	) (*domain.SyntheticProviderState, error)
}

type SyntheticLinkStateService struct {
	access             *accessGuard
	pendingStartLookup syntheticPendingStartLookup
	stateStore         syntheticProviderStateStore
	now                func() time.Time
	newID              func() string
}

type SyntheticLinkStateServiceOption func(*SyntheticLinkStateService)

func WithSyntheticLinkStateServiceNow(now func() time.Time) SyntheticLinkStateServiceOption {
	return func(service *SyntheticLinkStateService) {
		service.now = now
	}
}

func WithSyntheticLinkStateServiceIDGenerator(newID func() string) SyntheticLinkStateServiceOption {
	return func(service *SyntheticLinkStateService) {
		service.newID = newID
	}
}

func NewSyntheticLinkStateService(
	store *persistence.Store,
	opts ...SyntheticLinkStateServiceOption,
) *SyntheticLinkStateService {
	service := &SyntheticLinkStateService{
		access:             newAccessGuard(store),
		pendingStartLookup: persistence.NewSyntheticPendingStartStoreFromStore(store),
		stateStore:         persistence.NewSyntheticProviderStateStoreFromStore(store),
		now:                time.Now,
		newID:              uuid.NewString,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *SyntheticLinkStateService) GetPendingSyntheticLinkState(
	ctx context.Context,
	params GetPendingSyntheticLinkStateParams,
) (PendingSyntheticLinkState, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return PendingSyntheticLinkState{}, err
	}
	pendingStart, providerState, err := s.resolvePendingSyntheticState(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.State,
	)
	if err != nil {
		return PendingSyntheticLinkState{}, err
	}
	return pendingSyntheticLinkStateFromDomain(*pendingStart, providerState), nil
}

func (s *SyntheticLinkStateService) SavePendingSyntheticLinkState(
	ctx context.Context,
	params SavePendingSyntheticLinkStateParams,
) (PendingSyntheticLinkState, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return PendingSyntheticLinkState{}, err
	}
	pendingStart, existingState, err := s.resolvePendingSyntheticState(
		ctx,
		params.TenantID,
		params.ActorUserID,
		params.State,
	)
	if err != nil {
		return PendingSyntheticLinkState{}, err
	}
	configuredAccounts, err := s.makeConfiguredAccounts(
		params.ConfiguredAccounts,
		existingConfiguredAccounts(existingState),
	)
	if err != nil {
		return PendingSyntheticLinkState{}, err
	}
	providerReference := syntheticProviderReference(*pendingStart)
	now := s.now()
	createdAt := now
	windowHistory := []domain.SyntheticWindowHistoryEntry{}
	sequenceCounters := []domain.SyntheticAccountInstantSequenceCounter{}
	if existingState != nil {
		createdAt = existingState.CreatedAt
		windowHistory = append([]domain.SyntheticWindowHistoryEntry{}, existingState.Envelope.WindowHistory...)
		sequenceCounters = append(
			[]domain.SyntheticAccountInstantSequenceCounter{},
			existingState.Envelope.SequenceCounters...,
		)
	}
	savedState, err := s.stateStore.SaveSyntheticProviderState(ctx, domain.SyntheticProviderState{
		ProviderReference: providerReference,
		Envelope: domain.SyntheticProviderStateEnvelope{
			Version:            domain.SyntheticProviderStateVersion1,
			ConfiguredAccounts: configuredAccounts,
			WindowHistory:      windowHistory,
			SequenceCounters:   sequenceCounters,
		},
		CreatedAt: createdAt,
		UpdatedAt: now,
	})
	if err != nil {
		return PendingSyntheticLinkState{}, fmt.Errorf("save pending synthetic link state: %w", err)
	}
	return pendingSyntheticLinkStateFromDomain(*pendingStart, &savedState), nil
}

func (s *SyntheticLinkStateService) resolvePendingSyntheticState(
	ctx context.Context,
	tenantID string,
	actorUserID string,
	state string,
) (*domain.PendingBankConnectionLinkStart, *domain.SyntheticProviderState, error) {
	pendingStart, err := s.pendingStartLookup.GetPendingSyntheticStart(
		ctx,
		tenantID,
		actorUserID,
		state,
		s.now(),
	)
	if err != nil {
		if errors.Is(err, persistence.ErrPendingBankConnectionLinkStartNotFound) {
			return nil, nil, ErrPendingSyntheticLinkStateNotFound
		}
		return nil, nil, fmt.Errorf("get pending synthetic link state: %w", err)
	}
	providerState, err := s.stateStore.GetSyntheticProviderState(
		ctx,
		syntheticProviderReference(*pendingStart),
	)
	if err != nil {
		if errors.Is(err, persistence.ErrSyntheticProviderStateNotFound) {
			return pendingStart, nil, nil
		}
		return nil, nil, fmt.Errorf("get pending synthetic link state: %w", err)
	}
	return pendingStart, providerState, nil
}

func (s *SyntheticLinkStateService) makeConfiguredAccounts(
	accounts []SyntheticLinkStateAccount,
	existing []domain.SyntheticConfiguredAccount,
) ([]domain.SyntheticConfiguredAccount, error) {
	existingByKey := make(map[string]domain.SyntheticConfiguredAccount, len(existing))
	for _, account := range existing {
		existingByKey[strings.TrimSpace(account.Key)] = account
	}
	configuredAccounts := make([]domain.SyntheticConfiguredAccount, 0, len(accounts))
	usedKeys := map[string]struct{}{}
	for index, account := range accounts {
		name := strings.TrimSpace(account.Name)
		if name == "" {
			return nil, fmt.Errorf("configured account %d: %w", index, ErrSyntheticConfiguredAccountNameRequired)
		}
		currency := strings.TrimSpace(account.Currency)
		if currency == "" {
			return nil, fmt.Errorf("configured account %d: %w", index, ErrSyntheticConfiguredAccountCurrencyRequired)
		}
		key := strings.TrimSpace(account.Key)
		if existingAccount, ok := existingByKey[key]; ok {
			key = existingAccount.Key
		}
		if key == "" {
			key = syntheticAccountKey(s.newID(), index)
		}
		if _, exists := usedKeys[key]; exists {
			key = syntheticAccountKey(s.newID(), index)
		}
		usedKeys[key] = struct{}{}
		configuredAccounts = append(configuredAccounts, domain.SyntheticConfiguredAccount{
			Key:      key,
			Name:     name,
			Currency: currency,
		})
	}
	return configuredAccounts, nil
}

func pendingSyntheticLinkStateFromDomain(
	pendingStart domain.PendingBankConnectionLinkStart,
	providerState *domain.SyntheticProviderState,
) PendingSyntheticLinkState {
	result := PendingSyntheticLinkState{
		Provider: string(domain.ProviderIDSynthetic),
		State:    strings.TrimSpace(pendingStart.State),
	}
	if providerState == nil {
		return result
	}
	result.ConfiguredAccounts = make([]SyntheticLinkStateAccount, 0, len(providerState.Envelope.ConfiguredAccounts))
	for _, account := range providerState.Envelope.ConfiguredAccounts {
		result.ConfiguredAccounts = append(result.ConfiguredAccounts, SyntheticLinkStateAccount{
			Key:      account.Key,
			Name:     account.Name,
			Currency: account.Currency,
		})
	}
	result.CanFinish = len(result.ConfiguredAccounts) > 0
	return result
}

func existingConfiguredAccounts(
	state *domain.SyntheticProviderState,
) []domain.SyntheticConfiguredAccount {
	if state == nil {
		return nil
	}
	return append([]domain.SyntheticConfiguredAccount{}, state.Envelope.ConfiguredAccounts...)
}

func syntheticProviderReference(start domain.PendingBankConnectionLinkStart) string {
	return firstNonEmpty(start.ProviderReference, start.State)
}

func syntheticAccountKey(newID string, index int) string {
	trimmed := strings.TrimSpace(newID)
	if trimmed == "" {
		trimmed = strconv.Itoa(index + 1)
	}
	return "synthetic-account-" + trimmed
}
