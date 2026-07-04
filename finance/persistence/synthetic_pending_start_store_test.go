package persistence

import (
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyntheticPendingStartStore(t *testing.T) {
	makeStore := func(t *testing.T) (*Store, *SyntheticPendingStartStore) {
		t.Helper()
		database := openTestDatabase(t)
		store := NewStore(database)
		return store, NewSyntheticPendingStartStore(database)
	}

	t.Run("returns only matching active synthetic pending starts", func(t *testing.T) {
		fake := faker.New()
		store, pendingStore := makeStore(t)
		now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)

		active := domain.PendingBankConnectionLinkStart{
			ID:               "pending-" + fake.UUID().V4(),
			TenantID:         "tenant-" + fake.UUID().V4(),
			ActorUserID:      "actor-" + fake.UUID().V4(),
			Provider:         string(domain.ProviderIDSynthetic),
			ConnectorID:      domain.ProviderConnectorIDSynthetic,
			State:            "state-" + fake.UUID().V4(),
			CallbackURL:      "http://localhost:5173/#/finance/connections",
			AuthorizationURL: "#/finance/connections/synthetic?state=" + fake.UUID().V4(),
			ExpiresAt:        now.Add(10 * time.Minute),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		_, err := store.SavePendingBankConnectionLinkStart(t.Context(), active)
		require.NoError(t, err)

		wrongActor := active
		wrongActor.ID = "pending-other-actor-" + fake.UUID().V4()
		wrongActor.ActorUserID = "actor-other-" + fake.UUID().V4()
		wrongActor.State = "state-other-actor-" + fake.UUID().V4()
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), wrongActor)
		require.NoError(t, err)

		nonSynthetic := active
		nonSynthetic.ID = "pending-pko-" + fake.UUID().V4()
		nonSynthetic.Provider = string(domain.ProviderIDPKO)
		nonSynthetic.ConnectorID = domain.ProviderConnectorIDEnableBanking
		nonSynthetic.State = "state-pko-" + fake.UUID().V4()
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), nonSynthetic)
		require.NoError(t, err)

		expired := active
		expired.ID = "pending-expired-" + fake.UUID().V4()
		expired.State = "state-expired-" + fake.UUID().V4()
		expired.ExpiresAt = now.Add(-time.Minute)
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), expired)
		require.NoError(t, err)

		consumedAt := now.Add(-time.Minute)
		consumed := active
		consumed.ID = "pending-consumed-" + fake.UUID().V4()
		consumed.State = "state-consumed-" + fake.UUID().V4()
		consumed.ConsumedAt = &consumedAt
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), consumed)
		require.NoError(t, err)

		resolved, err := pendingStore.GetPendingSyntheticStart(
			t.Context(),
			active.TenantID,
			active.ActorUserID,
			active.State,
			now,
		)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, active.ID, resolved.ID)
		assert.Equal(t, active.Provider, resolved.Provider)
		assert.Equal(t, active.ConnectorID, resolved.ConnectorID)

		missingCases := []struct {
			name        string
			tenantID    string
			actorUserID string
			state       string
		}{
			{
				name:        "wrong tenant",
				tenantID:    "tenant-other-" + fake.UUID().V4(),
				actorUserID: active.ActorUserID,
				state:       active.State,
			},
			{
				name:        "wrong actor",
				tenantID:    active.TenantID,
				actorUserID: wrongActor.ActorUserID,
				state:       active.State,
			},
			{
				name:        "wrong state",
				tenantID:    active.TenantID,
				actorUserID: active.ActorUserID,
				state:       "state-missing-" + fake.UUID().V4(),
			},
			{
				name:        "non synthetic provider",
				tenantID:    nonSynthetic.TenantID,
				actorUserID: nonSynthetic.ActorUserID,
				state:       nonSynthetic.State,
			},
			{name: "expired", tenantID: expired.TenantID, actorUserID: expired.ActorUserID, state: expired.State},
			{name: "consumed", tenantID: consumed.TenantID, actorUserID: consumed.ActorUserID, state: consumed.State},
		}

		for _, testCase := range missingCases {
			t.Run(testCase.name, func(t *testing.T) {
				missing, lookupErr := pendingStore.GetPendingSyntheticStart(
					t.Context(),
					testCase.tenantID,
					testCase.actorUserID,
					testCase.state,
					now,
				)
				require.ErrorIs(t, lookupErr, ErrPendingBankConnectionLinkStartNotFound)
				assert.Nil(t, missing)
			})
		}
	})

	t.Run("handles constructor nil and database failures", func(t *testing.T) {
		assert.Nil(t, NewSyntheticPendingStartStoreFromStore(nil))

		_, pendingStore := makeStore(t)
		sqlDB, err := pendingStore.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		resolved, err := pendingStore.GetPendingSyntheticStart(
			t.Context(),
			"tenant",
			"actor",
			"state",
			time.Date(2026, time.July, 4, 10, 30, 0, 0, time.UTC),
		)
		require.ErrorContains(t, err, "get pending synthetic start")
		assert.Nil(t, resolved)
	})
}
