package finance

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBankSyncServiceOrchestration(t *testing.T) {
	fake := faker.New()

	makeFixture := func(t *testing.T, connectorID domain.ProviderConnectorID) (
		*persistence.Store,
		*BankSyncService,
		*mockbankSyncOrchestrator,
		domain.BankConnection,
		domain.ConnectionSecret,
		time.Time,
	) {
		t.Helper()
		store := persistence.NewStore(openTestDatabase(t))
		now := time.Date(2026, time.August, 18, 10, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
		ownerID := "owner-" + fake.UUID().V4()
		tenant, err := NewTenantService(store).CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "PLN",
		})
		require.NoError(t, err)
		cipherKey := make([]byte, 32)
		for index := range cipherKey {
			cipherKey[index] = byte(index + 1)
		}
		cipher, err := credentials.NewAESGCMCipher(cipherKey, "bank-sync-orchestrator")
		require.NoError(t, err)
		envelope, err := cipher.SealString("secret-" + fake.UUID().V4())
		require.NoError(t, err)
		secret, err := store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "secret-" + fake.UUID().V4(),
			Provider:  string(domain.ProviderIDPKO),
			Reference: "secret-reference-" + fake.UUID().V4(),
			Envelope:  envelope,
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err)
		persistedSecret, err := store.GetConnectionSecret(t.Context(), secret.ID)
		require.NoError(t, err)
		require.NotNil(t, persistedSecret)
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-" + fake.UUID().V4(),
			TenantID:          tenant.ID,
			Provider:          string(domain.ProviderIDPKO),
			ConnectorID:       connectorID,
			ProviderReference: "session-" + fake.UUID().V4(),
			SecretID:          secret.ID,
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now.Add(-time.Hour),
			UpdatedAt:         now.Add(-time.Hour),
		})
		require.NoError(t, err)
		orchestrator := newMockbankSyncOrchestrator(t)
		service := NewBankSyncService(
			store,
			orchestrator,
			WithBankSyncServiceNow(func() time.Time { return now }),
			WithBankSyncServiceLogger(slog.New(slog.DiscardHandler)),
		)
		return store, service, orchestrator, connection, *persistedSecret, now
	}

	t.Run("requires an orchestrator at construction", func(t *testing.T) {
		store := persistence.NewStore(openTestDatabase(t))
		require.PanicsWithValue(t, "bank sync orchestrator is required", func() {
			NewBankSyncService(store, nil)
		})
	})

	t.Run("passes persisted PKO Enable Banking identity encrypted secret and bounds", func(t *testing.T) {
		store, service, orchestrator, connection, secret, now := makeFixture(
			t,
			domain.ProviderConnectorIDEnableBanking,
		)
		windowStart := now.AddDate(0, 0, -5)
		windowEnd := now.Add(-time.Hour)
		expectedRequest := internalproviders.SyncOrchestrationRequest{
			Connection: domain.ProviderConnectionRef{
				ConnectionID:      connection.ID,
				ProviderID:        domain.ProviderIDPKO,
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				ProviderReference: connection.ProviderReference,
			},
			Secret:      secret,
			JobID:       "job-" + fake.UUID().V4(),
			Reason:      BankConnectionSyncReasonManual,
			WindowStart: &windowStart,
			WindowEnd:   &windowEnd,
		}
		orchestrator.EXPECT().Orchestrate(mock.Anything, expectedRequest).Return(
			internalproviders.SyncOrchestrationResult{Stats: domain.ProviderSyncStats{
				CreatedAccounts:     2,
				CreatedTransactions: 3,
				UpdatedTransactions: 4,
			}}, nil,
		)

		result, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        expectedRequest.JobID,
			Reason:       expectedRequest.Reason,
			WindowStart:  expectedRequest.WindowStart,
			WindowEnd:    expectedRequest.WindowEnd,
		})
		require.NoError(t, err)
		assert.Equal(t, BankConnectionSyncResult{
			ImportedAccounts: 2, ImportedTransactions: 3, UpdatedTransactions: 4,
		}, result)
		persisted, err := store.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, err)
		require.NotNil(t, persisted)
		assert.Equal(t, domain.BankConnectionStateActive, persisted.State)
		assert.Equal(t, expectedRequest.JobID, persisted.LastSyncJobID)
		require.NotNil(t, persisted.LastSyncStartedAt)
		assert.True(t, persisted.LastSyncStartedAt.Equal(now))
		require.NotNil(t, persisted.LastSuccessfulSyncAt)
		assert.True(t, persisted.LastSuccessfulSyncAt.Equal(now))
		assert.Empty(t, persisted.LastSyncError)
	})

	t.Run("records an unknown persisted connector orchestration failure", func(t *testing.T) {
		store, service, orchestrator, connection, secret, now := makeFixture(
			t,
			domain.ProviderConnectorID("unknown-"+fake.UUID().V4()),
		)
		orchestrationErr := errors.New("resolve sync connector: connector not configured")
		orchestrator.EXPECT().Orchestrate(mock.Anything, mock.MatchedBy(
			func(request internalproviders.SyncOrchestrationRequest) bool {
				return request.Connection.ConnectorID == connection.ConnectorID && request.Secret == secret
			},
		)).Return(internalproviders.SyncOrchestrationResult{}, orchestrationErr)

		_, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-" + fake.UUID().V4(),
			Reason:       BankConnectionSyncReasonManual,
		})
		require.ErrorIs(t, err, orchestrationErr)
		persisted, loadErr := store.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, loadErr)
		require.NotNil(t, persisted)
		require.NotNil(t, persisted.LastSyncStartedAt)
		assert.True(t, persisted.LastSyncStartedAt.Equal(now))
		assert.Empty(t, persisted.LastSuccessfulSyncAt)
		assert.Contains(t, persisted.LastSyncError, orchestrationErr.Error())
	})
}
