package finance

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBankSyncServiceProviderSnapshots(t *testing.T) {
	t.Run("persists distinct account balance and transaction documents with normalized records", func(t *testing.T) {
		fake := faker.New()
		store := persistence.NewStore(openTestDatabase(t))
		key := []byte("0123456789abcdef0123456789abcdef")
		cipher, err := credentials.NewAESGCMCipher(key, "snapshot-test")
		require.NoError(t, err)
		now := time.Date(2026, time.August, 14, 9, 0, 0, 0, time.UTC)
		service := NewService(
			store,
			WithConnectionSecretCipher(cipher),
			WithNow(func() time.Time { return now }),
		)
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID: "owner-" + fake.UUID().V4(), Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "PLN",
		})
		require.NoError(t, err)
		secret, err := service.encryptAndSaveConnectionSecret(t.Context(), "pko", "session-"+fake.UUID().V4(), "secret")
		require.NoError(t, err)
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID: "connection-" + fake.UUID().V4(), TenantID: tenant.ID, Provider: "pko",
			ConnectorID: domain.ProviderConnectorIDEnableBanking, SecretID: secret,
			ProviderReference: "session-" + fake.UUID().V4(), State: domain.BankConnectionStateActive,
			CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		providerTransactionFingerprint := "fingerprint-" + fake.UUID().V4()
		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
			ConnectionID: connection.ID,
			Result: ProviderSyncResult{
				Accounts: []ProviderNormalizedAccount{{
					ProviderAccountID: providerAccountID,
					Name:              "account-" + fake.Lorem().Word(),
					Currency:          "PLN",
				}},
				Transactions: []ProviderNormalizedTransaction{{
					ProviderAccountID: providerAccountID,
					Status:            domain.TransactionStatusBooked, AmountMinor: -100, Currency: "PLN",
					Description: "transaction-" + fake.Lorem().Word(), EffectiveAt: now,
					Fingerprint: providerTransactionFingerprint,
				}},
				Snapshots: []domain.ProviderSnapshotObservation{
					{
						Kind:              domain.ProviderSnapshotKindAccount,
						ProviderObjectID:  providerAccountID,
						ProviderAccountID: providerAccountID,
						DocumentJSON:      []byte(`{"account":"typed"}`),
						CapturedAt:        now,
					},
					{
						Kind:              domain.ProviderSnapshotKindAccountBalance,
						ProviderObjectID:  providerAccountID,
						ProviderAccountID: providerAccountID,
						DocumentJSON:      []byte(`{"balance":"typed"}`),
						CapturedAt:        now,
					},
					{
						Kind:              domain.ProviderSnapshotKindTransaction,
						ProviderObjectID:  providerTransactionFingerprint,
						ProviderAccountID: providerAccountID,
						DocumentJSON:      []byte(`{"transaction":"typed"}`),
						CapturedAt:        now,
					},
				},
			},
		})
		require.NoError(t, err)
		snapshots, err := persistence.NewProviderSnapshotStoreFromStore(store).
			ListProviderSnapshotsByConnection(t.Context(), connection.ID)
		require.NoError(t, err)
		require.Len(t, snapshots, 3)
		assert.ElementsMatch(t,
			[]domain.ProviderSnapshotKind{
				domain.ProviderSnapshotKindAccount,
				domain.ProviderSnapshotKindAccountBalance,
				domain.ProviderSnapshotKindTransaction,
			},
			[]domain.ProviderSnapshotKind{snapshots[0].Kind, snapshots[1].Kind, snapshots[2].Kind},
		)
		for _, snapshot := range snapshots {
			if snapshot.Kind == domain.ProviderSnapshotKindTransaction {
				assert.NotEmpty(t, snapshot.FinanceTransactionID)
				assert.Equal(t, providerTransactionFingerprint, snapshot.ProviderObjectID)
			}
		}
	})
}
