package finance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type syntheticLinkStateAccessStoreStub struct {
	allowed bool
	err     error
}

func (s syntheticLinkStateAccessStoreStub) IsTenantMember(
	context.Context,
	string,
	string,
) (bool, error) {
	return s.allowed, s.err
}

type syntheticLinkStatePendingLookupStub struct {
	start *domain.PendingBankConnectionLinkStart
	err   error
}

func (s syntheticLinkStatePendingLookupStub) GetPendingSyntheticStart(
	context.Context,
	string,
	string,
	string,
	time.Time,
) (*domain.PendingBankConnectionLinkStart, error) {
	return s.start, s.err
}

type syntheticLinkStateProviderStateStoreStub struct {
	state   *domain.SyntheticProviderState
	getErr  error
	saveErr error
}

func (s syntheticLinkStateProviderStateStoreStub) SaveSyntheticProviderState(
	context.Context,
	domain.SyntheticProviderState,
) (domain.SyntheticProviderState, error) {
	if s.saveErr != nil {
		return domain.SyntheticProviderState{}, s.saveErr
	}
	if s.state == nil {
		return domain.SyntheticProviderState{}, nil
	}
	return *s.state, nil
}

func (s syntheticLinkStateProviderStateStoreStub) GetSyntheticProviderState(
	context.Context,
	string,
) (*domain.SyntheticProviderState, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.state, nil
}

func TestSyntheticLinkStateService(t *testing.T) {
	makeStore := func(t *testing.T) *persistence.Store {
		t.Helper()
		return persistence.NewStore(openTestDatabase(t))
	}

	makePendingStart := func(fake faker.Faker, tenantID string, actorUserID string, now time.Time) domain.PendingBankConnectionLinkStart {
		return domain.PendingBankConnectionLinkStart{
			ID:               "pending-" + fake.UUID().V4(),
			TenantID:         tenantID,
			ActorUserID:      actorUserID,
			Provider:         string(domain.ProviderIDSynthetic),
			ConnectorID:      domain.ProviderConnectorIDSynthetic,
			State:            "state-" + fake.UUID().V4(),
			CallbackURL:      "http://localhost:5173/#/finance/connections",
			AuthorizationURL: "#/finance/connections/synthetic?state=" + fake.UUID().V4(),
			ExpiresAt:        now.Add(15 * time.Minute),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}

	t.Run("refreshes pending synthetic state and preserves stable account keys", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		tenantService := NewTenantService(store)
		now := time.Date(2026, time.July, 4, 11, 0, 0, 0, time.UTC)
		actorUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)

		pendingStart := makePendingStart(fake, tenant.ID, actorUserID, now)
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), pendingStart)
		require.NoError(t, err)

		generatedIDs := []string{"account-a", "account-b"}
		service := NewSyntheticLinkStateService(
			store,
			WithSyntheticLinkStateServiceNow(func() time.Time { return now }),
			WithSyntheticLinkStateServiceIDGenerator(func() string {
				if len(generatedIDs) == 0 {
					return ""
				}
				value := generatedIDs[0]
				generatedIDs = generatedIDs[1:]
				return value
			}),
		)

		initial, err := service.GetPendingSyntheticLinkState(t.Context(), GetPendingSyntheticLinkStateParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			State:       pendingStart.State,
		})
		require.NoError(t, err)
		assert.Equal(t, string(domain.ProviderIDSynthetic), initial.Provider)
		assert.Equal(t, pendingStart.State, initial.State)
		assert.Empty(t, initial.ConfiguredAccounts)
		assert.False(t, initial.CanFinish)

		duplicateName := "wallet-" + fake.Lorem().Word()
		updated, err := service.SavePendingSyntheticLinkState(t.Context(), SavePendingSyntheticLinkStateParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			State:       pendingStart.State,
			ConfiguredAccounts: []SyntheticLinkStateAccount{{
				Name:     duplicateName,
				Currency: "USD",
			}, {
				Name:     duplicateName,
				Currency: "USD",
			}},
		})
		require.NoError(t, err)
		require.Len(t, updated.ConfiguredAccounts, 2)
		assert.Equal(t, "synthetic-account-account-a", updated.ConfiguredAccounts[0].Key)
		assert.Equal(t, "synthetic-account-account-b", updated.ConfiguredAccounts[1].Key)
		assert.NotEqual(t, updated.ConfiguredAccounts[0].Key, updated.ConfiguredAccounts[1].Key)
		assert.True(t, updated.CanFinish)

		refreshed, err := service.GetPendingSyntheticLinkState(t.Context(), GetPendingSyntheticLinkStateParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			State:       pendingStart.State,
		})
		require.NoError(t, err)
		assert.Equal(t, updated, refreshed)

		reSaved, err := service.SavePendingSyntheticLinkState(t.Context(), SavePendingSyntheticLinkStateParams{
			ActorUserID:        actorUserID,
			TenantID:           tenant.ID,
			State:              pendingStart.State,
			ConfiguredAccounts: refreshed.ConfiguredAccounts,
		})
		require.NoError(t, err)
		assert.Equal(t, updated.ConfiguredAccounts, reSaved.ConfiguredAccounts)
	})

	t.Run("enforces tenant actor provider and state authorization without leaking ownership", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		tenantService := NewTenantService(store)
		now := time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC)

		ownerUserID := "owner-" + fake.UUID().V4()
		memberUserID := "member-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)
		_, err = store.SaveTenantMembership(t.Context(), domain.TenantMembership{
			TenantID:  tenant.ID,
			UserID:    memberUserID,
			JoinedAt:  now,
			CreatedAt: now,
		})
		require.NoError(t, err)

		pendingStart := makePendingStart(fake, tenant.ID, ownerUserID, now)
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), pendingStart)
		require.NoError(t, err)

		nonSynthetic := pendingStart
		nonSynthetic.ID = "pending-pko-" + fake.UUID().V4()
		nonSynthetic.Provider = string(domain.ProviderIDPKO)
		nonSynthetic.ConnectorID = domain.ProviderConnectorIDEnableBanking
		nonSynthetic.State = "state-pko-" + fake.UUID().V4()
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), nonSynthetic)
		require.NoError(t, err)

		service := NewSyntheticLinkStateService(
			store,
			WithSyntheticLinkStateServiceNow(func() time.Time { return now }),
		)

		_, err = service.GetPendingSyntheticLinkState(t.Context(), GetPendingSyntheticLinkStateParams{
			ActorUserID: "outsider-" + fake.UUID().V4(),
			TenantID:    tenant.ID,
			State:       pendingStart.State,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		_, err = service.GetPendingSyntheticLinkState(t.Context(), GetPendingSyntheticLinkStateParams{
			ActorUserID: memberUserID,
			TenantID:    tenant.ID,
			State:       pendingStart.State,
		})
		require.ErrorIs(t, err, ErrPendingSyntheticLinkStateNotFound)

		_, err = service.GetPendingSyntheticLinkState(t.Context(), GetPendingSyntheticLinkStateParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			State:       nonSynthetic.State,
		})
		require.ErrorIs(t, err, ErrPendingSyntheticLinkStateNotFound)

		_, err = service.GetPendingSyntheticLinkState(t.Context(), GetPendingSyntheticLinkStateParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			State:       "missing-state-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrPendingSyntheticLinkStateNotFound)
	})

	t.Run("validates configured accounts before saving synthetic state", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		tenantService := NewTenantService(store)
		now := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
		actorUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)

		pendingStart := makePendingStart(fake, tenant.ID, actorUserID, now)
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), pendingStart)
		require.NoError(t, err)

		service := NewSyntheticLinkStateService(
			store,
			WithSyntheticLinkStateServiceNow(func() time.Time { return now }),
		)

		_, err = service.SavePendingSyntheticLinkState(t.Context(), SavePendingSyntheticLinkStateParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			State:       pendingStart.State,
			ConfiguredAccounts: []SyntheticLinkStateAccount{{
				Name:     " ",
				Currency: "USD",
			}},
		})
		require.ErrorIs(t, err, ErrSyntheticConfiguredAccountNameRequired)

		_, err = service.SavePendingSyntheticLinkState(t.Context(), SavePendingSyntheticLinkStateParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			State:       pendingStart.State,
			ConfiguredAccounts: []SyntheticLinkStateAccount{{
				Name:     "wallet-" + fake.Lorem().Word(),
				Currency: " ",
			}},
		})
		require.ErrorIs(t, err, ErrSyntheticConfiguredAccountCurrencyRequired)
	})

	t.Run("covers helper and error branches", func(t *testing.T) {
		nowService := NewSyntheticLinkStateService(makeStore(t))
		assert.False(t, nowService.now().IsZero())

		pendingStart := &domain.PendingBankConnectionLinkStart{State: "state-1"}
		stateErr := fmt.Errorf("state-store-%s", faker.New().UUID().V4())
		lookupErr := fmt.Errorf("pending-lookup-%s", faker.New().UUID().V4())

		service := &SyntheticLinkStateService{
			access:             newAccessGuard(syntheticLinkStateAccessStoreStub{allowed: true}),
			pendingStartLookup: syntheticLinkStatePendingLookupStub{start: pendingStart},
			stateStore:         syntheticLinkStateProviderStateStoreStub{getErr: stateErr},
			now:                func() time.Time { return time.Date(2026, time.July, 4, 14, 0, 0, 0, time.UTC) },
			newID:              func() string { return "" },
		}

		_, err := service.GetPendingSyntheticLinkState(t.Context(), GetPendingSyntheticLinkStateParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			State:       pendingStart.State,
		})
		require.ErrorContains(t, err, "get pending synthetic link state")

		service.pendingStartLookup = syntheticLinkStatePendingLookupStub{err: lookupErr}
		_, err = service.GetPendingSyntheticLinkState(t.Context(), GetPendingSyntheticLinkStateParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			State:       pendingStart.State,
		})
		require.ErrorContains(t, err, "get pending synthetic link state")

		service.pendingStartLookup = syntheticLinkStatePendingLookupStub{start: pendingStart}
		service.stateStore = syntheticLinkStateProviderStateStoreStub{saveErr: stateErr}
		_, err = service.SavePendingSyntheticLinkState(t.Context(), SavePendingSyntheticLinkStateParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			State:       pendingStart.State,
			ConfiguredAccounts: []SyntheticLinkStateAccount{{
				Name:     "wallet-a",
				Currency: "USD",
			}},
		})
		require.ErrorContains(t, err, "save pending synthetic link state")

		service.access = newAccessGuard(syntheticLinkStateAccessStoreStub{allowed: false})
		_, err = service.SavePendingSyntheticLinkState(t.Context(), SavePendingSyntheticLinkStateParams{
			ActorUserID: "actor-1",
			TenantID:    "tenant-1",
			State:       pendingStart.State,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		service.access = newAccessGuard(syntheticLinkStateAccessStoreStub{allowed: true})
		accounts, err := service.makeConfiguredAccounts([]SyntheticLinkStateAccount{{
			Key:      "same-key",
			Name:     "wallet-a",
			Currency: "USD",
		}, {
			Key:      "same-key",
			Name:     "wallet-b",
			Currency: "EUR",
		}}, nil)
		require.NoError(t, err)
		require.Len(t, accounts, 2)
		assert.Equal(t, "same-key", accounts[0].Key)
		assert.Equal(t, "synthetic-account-2", accounts[1].Key)
		assert.Equal(t, "synthetic-account-1", syntheticAccountKey("", 0))
	})
}
