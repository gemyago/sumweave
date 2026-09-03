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

func TestLedgerServiceTransactionTagAssignments(t *testing.T) {
	t.Run("records reads lists replaces clears and validates tenant-local tags atomically", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		tenantService := NewTenantService(store)
		catalogService := NewCatalogService(store)
		ledgerService := NewLedgerService(
			store,
			WithLedgerServiceAccountBalanceStore(persistence.NewAccountBalanceStore(database)),
			WithLedgerServiceTransactionStore(persistence.NewTransactionTagStore(database)),
		)
		actorUserID := "user-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		account, err := catalogService.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			Name:        "account-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)
		createTag := func(t *testing.T, tenantID string) domain.Tag {
			t.Helper()
			tag, createErr := catalogService.CreateTag(t.Context(), CreateTagParams{
				ActorUserID: actorUserID,
				TenantID:    tenantID,
				Name:        "tag-" + fake.Lorem().Word(),
			})
			require.NoError(t, createErr)
			return tag
		}
		firstTag := createTag(t, tenant.ID)
		secondTag := createTag(t, tenant.ID)
		thirdTag := createTag(t, tenant.ID)
		foreignTenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "foreign-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		foreignTag := createTag(t, foreignTenant.ID)

		effectiveAt := time.Date(2026, time.July, 12, 14, 0, 0, 0, time.FixedZone("test", 2*60*60))
		recordParams := RecordTransactionParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -987,
			Currency:    "USD",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: effectiveAt,
			TagIDs:      []string{firstTag.ID, secondTag.ID},
		}
		recorded, err := ledgerService.RecordTransaction(t.Context(), recordParams)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{firstTag.ID, secondTag.ID}, recorded.TagIDs)

		loaded, err := ledgerService.GetTransaction(t.Context(), GetTransactionParams{
			ActorUserID: actorUserID, TenantID: tenant.ID, TransactionID: recorded.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, recorded.TagIDs, loaded.TagIDs)
		listed, err := ledgerService.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: actorUserID, TenantID: tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, listed, 1)
		assert.Equal(t, recorded.TagIDs, listed[0].TagIDs)

		updated, err := ledgerService.UpdateTransaction(t.Context(), UpdateTransactionParams{
			ActorUserID: actorUserID, TenantID: tenant.ID, TransactionID: recorded.ID,
			Description: "updated-" + fake.Lorem().Word(), AmountMinor: -654, TagIDs: []string{thirdTag.ID},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{thirdTag.ID}, updated.TagIDs)

		require.NoError(t, catalogService.HideTag(t.Context(), HideTagParams{
			ActorUserID: actorUserID, TenantID: tenant.ID, TagID: thirdTag.ID,
		}))
		historic, err := ledgerService.GetTransaction(t.Context(), GetTransactionParams{
			ActorUserID: actorUserID, TenantID: tenant.ID, TransactionID: recorded.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{thirdTag.ID}, historic.TagIDs)

		for _, tagIDs := range [][]string{
			{firstTag.ID, firstTag.ID},
			{"missing-" + fake.UUID().V4()},
			{thirdTag.ID},
			{foreignTag.ID},
		} {
			_, updateErr := ledgerService.UpdateTransaction(t.Context(), UpdateTransactionParams{
				ActorUserID: actorUserID, TenantID: tenant.ID, TransactionID: recorded.ID,
				Description: "invalid-" + fake.Lorem().Word(), AmountMinor: -321, TagIDs: tagIDs,
			})
			require.Error(t, updateErr)
		}
		unchanged, err := ledgerService.GetTransaction(t.Context(), GetTransactionParams{
			ActorUserID: actorUserID, TenantID: tenant.ID, TransactionID: recorded.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{thirdTag.ID}, unchanged.TagIDs)
		assert.Equal(t, updated.Description, unchanged.Description)
		assert.Equal(t, updated.AmountMinor, unchanged.AmountMinor)

		cleared, err := ledgerService.UpdateTransaction(t.Context(), UpdateTransactionParams{
			ActorUserID: actorUserID, TenantID: tenant.ID, TransactionID: recorded.ID,
			Description: updated.Description, AmountMinor: updated.AmountMinor, TagIDs: []string{},
		})
		require.NoError(t, err)
		assert.Empty(t, cleared.TagIDs)
	})
}
