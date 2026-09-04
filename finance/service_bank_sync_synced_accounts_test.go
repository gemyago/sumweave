package finance

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBankSyncServiceListBankConnectionSyncedAccounts(t *testing.T) {
	makeFixture := func(t *testing.T) (*persistence.Store, *BankSyncService, domain.Tenant, string, domain.BankConnection) {
		t.Helper()
		fake := faker.New()
		store := persistence.NewStore(openTestDatabase(t))
		ownerID := "user-" + fake.UUID().V4()
		tenant, err := NewService(store).CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		now := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.FixedZone("", 2*60*60))
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID: "connection-" + fake.UUID().V4(), TenantID: tenant.ID, Provider: "provider-" + fake.Lorem().Word(),
			State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		return store, NewBankSyncService(store, newMockbankSyncOrchestrator(t)), tenant, ownerID, connection
	}

	t.Run("returns resolved rows in authoritative stable order", func(t *testing.T) {
		fake := faker.New()
		store, service, tenant, ownerID, connection := makeFixture(t)
		firstSyncedAt := time.Date(2026, time.July, 14, 9, 0, 0, 0, time.FixedZone("", 2*60*60))
		makeAccount := func(id, financeAccountID string, createdAt time.Time) domain.ConnectionProviderAccount {
			return domain.ConnectionProviderAccount{
				ID: id, ConnectionID: connection.ID, ProviderAccountID: "provider-account-" + fake.UUID().V4(),
				FinanceAccountID: financeAccountID, Name: "account-" + fake.Lorem().Word(), Currency: "USD",
				IBAN: "iban-" + fake.UUID().V4(), MaskedPAN: "pan-" + fake.UUID().V4(),
				LastSuccessfulSyncAt: &firstSyncedAt, CreatedAt: createdAt, UpdatedAt: createdAt,
			}
		}
		first := makeAccount("row-a-"+fake.UUID().V4(), "finance-a-"+fake.UUID().V4(), firstSyncedAt)
		unresolved := makeAccount("row-b-"+fake.UUID().V4(), "", firstSyncedAt.Add(time.Minute))
		second := makeAccount(
			"row-c-"+fake.UUID().V4(),
			"finance-c-"+fake.UUID().V4(),
			firstSyncedAt.Add(2*time.Minute),
		)
		second.LastSuccessfulSyncAt = nil
		for _, account := range []domain.ConnectionProviderAccount{first, unresolved, second} {
			_, err := store.SaveConnectionProviderAccount(t.Context(), account)
			require.NoError(t, err)
		}

		actual, err := service.ListBankConnectionSyncedAccounts(t.Context(), ListBankConnectionSyncedAccountsParams{
			ActorUserID: ownerID, TenantID: tenant.ID, ConnectionID: connection.ID,
		})
		require.NoError(t, err)
		require.Len(t, actual, 2)
		assert.Equal(t, first.FinanceAccountID, actual[0].FinanceAccountID)
		assert.Equal(t, first.Name, actual[0].Name)
		assert.Equal(t, first.Currency, actual[0].Currency)
		assert.Equal(t, second.FinanceAccountID, actual[1].FinanceAccountID)
		assert.Equal(t, second.Name, actual[1].Name)
		assert.Equal(t, second.Currency, actual[1].Currency)
		assert.Nil(t, actual[1].LastSuccessfulSyncAt)
		require.NotNil(t, actual[0].LastSuccessfulSyncAt)
		require.NotNil(t, first.LastSuccessfulSyncAt)
		assert.True(t, first.LastSuccessfulSyncAt.Equal(*actual[0].LastSuccessfulSyncAt))
	})

	t.Run("enforces membership tenant ownership and not found semantics", func(t *testing.T) {
		fake := faker.New()
		store, service, tenant, ownerID, connection := makeFixture(t)
		otherTenant, err := NewService(store).CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID: ownerID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "USD", SeedDefaults: true,
		})
		require.NoError(t, err)
		for _, params := range []ListBankConnectionSyncedAccountsParams{
			{ActorUserID: "outsider-" + fake.UUID().V4(), TenantID: tenant.ID, ConnectionID: connection.ID},
			{ActorUserID: ownerID, TenantID: otherTenant.ID, ConnectionID: connection.ID},
			{ActorUserID: ownerID, TenantID: tenant.ID, ConnectionID: "missing-" + fake.UUID().V4()},
		} {
			_, resultErr := service.ListBankConnectionSyncedAccounts(t.Context(), params)
			if params.ActorUserID != ownerID {
				require.ErrorIs(t, resultErr, ErrTenantAccessDenied)
				continue
			}
			require.ErrorIs(t, resultErr, ErrBankConnectionNotFound)
		}
	})
}
