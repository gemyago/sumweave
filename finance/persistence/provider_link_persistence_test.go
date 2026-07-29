package persistence

import (
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProviderLinkPersistence(t *testing.T) {
	t.Run("stores parallel pending starts per connector and consumes matching connector only", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.June, 29, 18, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		actorUserID := "actor-" + fake.UUID().V4()
		providerID := string(domain.ProviderIDPKO)
		state := "state-" + fake.UUID().V4()

		first, err := persistence.SavePendingStart(t.Context(), domain.PendingBankConnectionLinkStart{
			ID:          "pending-a-" + fake.UUID().V4(),
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			Provider:    providerID,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			CallbackURL: "http://localhost:5173/#/finance/connections",
			ExpiresAt:   now.Add(15 * time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		second, err := persistence.SavePendingStart(t.Context(), domain.PendingBankConnectionLinkStart{
			ID:          "pending-b-" + fake.UUID().V4(),
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			Provider:    providerID,
			ConnectorID: domain.ProviderConnectorIDMonobank,
			State:       state,
			CallbackURL: "http://localhost:5173/#/finance/connections",
			ExpiresAt:   now.Add(15 * time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		consumed, err := persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDSynthetic,
			State:       state,
			ConsumedAt:  now.Add(time.Minute),
		})
		require.ErrorIs(t, err, providers.ErrPendingStartNotFound)
		assert.Nil(t, consumed)

		consumed, err = persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ConsumedAt:  now.Add(2 * time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, consumed)
		assert.Equal(t, first.ID, consumed.ID)
		assert.Equal(t, first.ConnectorID, consumed.ConnectorID)

		consumed, err = persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDMonobank,
			State:       state,
			ConsumedAt:  now.Add(3 * time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, consumed)
		assert.Equal(t, second.ID, consumed.ID)
		assert.Equal(t, second.ConnectorID, consumed.ConnectorID)
	})

	t.Run("restores consumed pending start and reuses it", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.June, 29, 19, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		actorUserID := "actor-" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()

		_, err := persistence.SavePendingStart(t.Context(), domain.PendingBankConnectionLinkStart{
			ID:          "pending-" + fake.UUID().V4(),
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			Provider:    string(domain.ProviderIDPKO),
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			CallbackURL: "http://localhost:5173/#/finance/connections",
			ExpiresAt:   now.Add(15 * time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		consumed, err := persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ConsumedAt:  now.Add(time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, consumed)

		require.NoError(
			t,
			persistence.RestorePendingStart(t.Context(), providers.RestorePendingStartRequest{
				TenantID:    tenantID,
				ActorUserID: actorUserID,
				ProviderID:  domain.ProviderIDPKO,
				ConnectorID: domain.ProviderConnectorIDEnableBanking,
				State:       state,
				RestoredAt:  now.Add(2 * time.Minute),
			}),
		)

		restored, err := persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ConsumedAt:  now.Add(3 * time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, restored)
		assert.Equal(t, consumed.ID, restored.ID)
	})

	t.Run("returns not found when restoring unknown pending start", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)

		err := persistence.RestorePendingStart(t.Context(), providers.RestorePendingStartRequest{
			TenantID:    "tenant-" + fake.UUID().V4(),
			ActorUserID: "actor-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       "state-" + fake.UUID().V4(),
			RestoredAt:  time.Now().UTC(),
		})
		require.ErrorIs(t, err, providers.ErrPendingStartNotFound)
	})

	t.Run("delegates bank connection and payload operations", func(t *testing.T) {
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)
		fake := faker.New()

		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		rawPayloadID := "payload-" + fake.UUID().V4()
		observedAt := time.Now().UTC()

		savedConnection, err := persistence.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:        connectionID,
			TenantID:  tenantID,
			Provider:  string(domain.ProviderIDPKO),
			CreatedAt: observedAt,
			UpdatedAt: observedAt,
		})
		require.NoError(t, err)
		require.Equal(t, connectionID, savedConnection.ID)

		connections, err := persistence.ListBankConnections(t.Context(), tenantID)
		require.NoError(t, err)
		require.Len(t, connections, 1)
		assert.Equal(t, connectionID, connections[0].ID)

		payload := domain.RawPayload{
			ID:               rawPayloadID,
			ConnectionID:     connectionID,
			Scope:            domain.RawPayloadScopeTransaction,
			ProviderObjectID: "obj-" + fake.UUID().V4(),
			PayloadJSON:      []byte(`{"scope":"transaction"}`),
			CapturedAt:       time.Now().UTC(),
		}
		savedPayload, err := persistence.SaveRawPayload(t.Context(), payload)
		require.NoError(t, err)
		assert.Equal(t, payload.ID, savedPayload.ID)
	})

	t.Run("returns persistence errors when tables are unavailable", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		require.NoError(t, database.db.WithContext(t.Context()).Migrator().DropTable(
			&bankConnectionModel{},
			&pendingBankConnectionLinkStartModel{},
			&rawPayloadModel{},
		))
		persistence := NewProviderLinkPersistence(NewStore(database))

		_, err := persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    "tenant-" + fake.UUID().V4(),
			ActorUserID: "actor-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       "state-" + fake.UUID().V4(),
			ConsumedAt:  time.Now().UTC(),
		})
		require.Error(t, err)

		err = persistence.RestorePendingStart(t.Context(), providers.RestorePendingStartRequest{
			TenantID:    "tenant-" + fake.UUID().V4(),
			ActorUserID: "actor-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       "state-" + fake.UUID().V4(),
			RestoredAt:  time.Now().UTC(),
		})
		require.Error(t, err)

		_, err = persistence.SaveBankConnection(t.Context(), domain.BankConnection{ID: "missing-" + fake.UUID().V4()})
		require.Error(t, err)

		_, err = persistence.ListBankConnections(t.Context(), "tenant-"+fake.UUID().V4())
		require.Error(t, err)

		_, err = persistence.SaveRawPayload(t.Context(), domain.RawPayload{ID: "missing-payload-" + fake.UUID().V4()})
		require.Error(t, err)
	})

	t.Run("returns not found when consumed row disappears before read", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := NewStore(database)
		persistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.June, 29, 20, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		actorUserID := "actor-" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()

		_, err := persistence.SavePendingStart(t.Context(), domain.PendingBankConnectionLinkStart{
			ID:          "pending-" + fake.UUID().V4(),
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			Provider:    string(domain.ProviderIDPKO),
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ExpiresAt:   now.Add(15 * time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		callbackName := fmt.Sprintf("consume-consumed-not-found-%s", fake.UUID().V4())
		callbackCalled := false
		require.NoError(
			t,
			database.db.Callback().
				Query().
				Before("gorm:query").
				Register(callbackName, func(tx *gorm.DB) {
					if callbackCalled {
						return
					}
					if tx.Statement.Table != (pendingBankConnectionLinkStartModel{}).TableName() {
						return
					}
					tx.AddError(gorm.ErrRecordNotFound)
					callbackCalled = true
				}),
		)
		defer func() {
			require.NoError(t, database.db.Callback().Query().Remove(callbackName))
		}()

		_, err = persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ConsumedAt:  now.Add(time.Minute),
		})
		require.ErrorIs(t, err, providers.ErrPendingStartNotFound)
		require.True(t, callbackCalled)
	})
}
