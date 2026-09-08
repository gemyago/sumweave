package persistence

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionTagStore(t *testing.T) {
	makeTransaction := func(fake faker.Faker, tenantID string, now time.Time) domain.Transaction {
		return domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    tenantID,
			AccountID:   "account-" + fake.UUID().V4(),
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -123,
			Currency:    "USD",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	t.Run("replaces assignments and preserves hidden historic tag IDs on reads", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.FixedZone("test", 3*60*60))
		database := openTestDatabase(t)
		store := NewStore(database)
		transactionStore := NewTransactionTagStore(database)
		tenantID := "tenant-" + fake.UUID().V4()
		firstTag := domain.Tag{
			ID:        "tag-z-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		secondTag := domain.Tag{
			ID:        "tag-a-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		thirdTag := domain.Tag{
			ID:        "tag-c-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		for _, tag := range []domain.Tag{firstTag, secondTag, thirdTag} {
			_, err := store.SaveTag(t.Context(), tag)
			require.NoError(t, err)
		}

		transaction := makeTransaction(fake, tenantID, now)
		transaction.TagIDs = []string{firstTag.ID, secondTag.ID}
		saved, err := transactionStore.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)
		assert.Equal(t, []string{secondTag.ID, firstTag.ID}, saved.TagIDs)

		secondTag.HiddenAt = &now
		_, err = store.SaveTag(t.Context(), secondTag)
		require.NoError(t, err)
		loaded, err := transactionStore.GetTransaction(t.Context(), transaction.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{secondTag.ID, firstTag.ID}, loaded.TagIDs)

		loaded.TagIDs = []string{thirdTag.ID}
		loaded.Description = "replacement-" + fake.Lorem().Word()
		replaced, err := transactionStore.SaveTransaction(t.Context(), *loaded)
		require.NoError(t, err)
		assert.Equal(t, []string{thirdTag.ID}, replaced.TagIDs)

		listed, err := transactionStore.ListTransactions(t.Context(), tenantID, "", "", "", true)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		assert.Equal(t, []string{thirdTag.ID}, listed[0].TagIDs)

		filtered, err := transactionStore.ListTransactions(
			t.Context(),
			tenantID,
			"",
			"",
			"",
			true,
			ListTransactionsPage{
				Kind:      domain.TransactionKindRegular,
				StartDate: now,
				EndDate:   now.Add(time.Minute),
			},
		)
		require.NoError(t, err)
		require.Len(t, filtered, 1)
		assert.Equal(t, transaction.ID, filtered[0].ID)

		replaced.TagIDs = []string{}
		cleared, err := transactionStore.SaveTransaction(t.Context(), replaced)
		require.NoError(t, err)
		assert.Empty(t, cleared.TagIDs)
	})

	t.Run("applies ascending order before the requested offset page", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.FixedZone("test", 3*60*60))
		transactionStore := NewTransactionTagStore(openTestDatabase(t))
		tenantID := "tenant-" + fake.UUID().V4()
		earlier := makeTransaction(fake, tenantID, now.Add(-time.Minute))
		later := makeTransaction(fake, tenantID, now)
		_, err := transactionStore.SaveTransaction(t.Context(), earlier)
		require.NoError(t, err)
		_, err = transactionStore.SaveTransaction(t.Context(), later)
		require.NoError(t, err)

		page, err := transactionStore.ListTransactions(
			t.Context(),
			tenantID,
			"",
			"",
			"",
			true,
			ListTransactionsPage{Limit: 1, Offset: 1, SortAscending: true},
		)
		require.NoError(t, err)
		require.Len(t, page, 1)
		assert.Equal(t, later.ID, page[0].ID)
	})

	t.Run("rolls back the transaction and associations when assignment validation fails", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 12, 13, 0, 0, 0, time.FixedZone("test", -4*60*60))
		database := openTestDatabase(t)
		store := NewStore(database)
		transactionStore := NewTransactionTagStore(database)
		tenantID := "tenant-" + fake.UUID().V4()
		foreignTenantID := "tenant-foreign-" + fake.UUID().V4()
		validTag := domain.Tag{
			ID:        "tag-valid-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		foreignTag := domain.Tag{
			ID:        "tag-foreign-" + fake.UUID().V4(),
			TenantID:  foreignTenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		for _, tag := range []domain.Tag{validTag, foreignTag} {
			_, err := store.SaveTag(t.Context(), tag)
			require.NoError(t, err)
		}

		transaction := makeTransaction(fake, tenantID, now)
		transaction.TagIDs = []string{validTag.ID}
		_, err := transactionStore.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)

		transaction.Description = "must-not-save-" + fake.Lorem().Word()
		transaction.TagIDs = []string{foreignTag.ID}
		_, err = transactionStore.SaveTransaction(t.Context(), transaction)
		require.ErrorIs(t, err, ErrTagNotFound)

		loaded, err := transactionStore.GetTransaction(t.Context(), transaction.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{validTag.ID}, loaded.TagIDs)
		assert.NotEqual(t, transaction.Description, loaded.Description)
	})

	t.Run("preserves pair-owned state while replacing ordinary transaction data", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 13, 13, 0, 0, 0, time.FixedZone("test", -4*60*60))
		database := openTestDatabase(t)
		coreStore := NewStore(database)
		transactionStore := NewTransactionTagStore(database)
		tenantID := "tenant-" + fake.UUID().V4()
		firstTag := domain.Tag{
			ID:        "tag-first-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		secondTag := domain.Tag{
			ID:        "tag-second-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		for _, tag := range []domain.Tag{firstTag, secondTag} {
			_, err := coreStore.SaveTag(t.Context(), tag)
			require.NoError(t, err)
		}

		groupID := "group-" + fake.UUID().V4()
		matchedAt := now.Add(-time.Minute)
		linked := makeTransaction(fake, tenantID, now)
		linked.Kind = domain.TransactionKindTransfer
		linked.TransferGroupID = &groupID
		linked.TransferMatchedAt = &matchedAt
		linked.TransferMatchingExcluded = true
		linked.TagIDs = []string{firstTag.ID}
		linked.ProviderOriginal = &domain.ProviderTransactionOriginal{
			AmountMinor: linked.AmountMinor,
			Currency:    linked.Currency,
			Description: linked.Description,
			EffectiveAt: &linked.EffectiveAt,
		}
		_, err := transactionStore.SaveTransaction(t.Context(), linked)
		require.NoError(t, err)

		refresh := linked
		refresh.Kind = domain.TransactionKindRegular
		refresh.TransferGroupID = nil
		refresh.TransferMatchedAt = nil
		refresh.TransferMatchingExcluded = false
		refresh.Description = "provider-refresh-" + fake.Lorem().Word()
		refresh.TagIDs = []string{secondTag.ID}
		refresh.UpdatedAt = now.Add(time.Minute)
		refresh.ProviderOriginal = &domain.ProviderTransactionOriginal{
			AmountMinor: refresh.AmountMinor,
			Currency:    refresh.Currency,
			Description: refresh.Description,
			EffectiveAt: &refresh.EffectiveAt,
		}

		saved, err := transactionStore.SaveTransaction(t.Context(), refresh)
		require.NoError(t, err)
		assert.Equal(t, domain.TransactionKindTransfer, saved.Kind)
		assert.Equal(t, linked.TransferGroupID, saved.TransferGroupID)
		require.NotNil(t, linked.TransferMatchedAt)
		require.NotNil(t, saved.TransferMatchedAt)
		assert.True(t, linked.TransferMatchedAt.Equal(*saved.TransferMatchedAt))
		assert.True(t, saved.TransferMatchingExcluded)
		assert.Equal(t, refresh.Description, saved.Description)
		assert.Equal(t, []string{secondTag.ID}, saved.TagIDs)
		require.NotNil(t, saved.ProviderOriginal)
		require.NotNil(t, refresh.ProviderOriginal)
		assert.Equal(t, refresh.ProviderOriginal.AmountMinor, saved.ProviderOriginal.AmountMinor)
		assert.Equal(t, refresh.ProviderOriginal.Currency, saved.ProviderOriginal.Currency)
		assert.Equal(t, refresh.ProviderOriginal.Description, saved.ProviderOriginal.Description)
		require.NotNil(t, refresh.ProviderOriginal.EffectiveAt)
		require.NotNil(t, saved.ProviderOriginal.EffectiveAt)
		assert.True(t, refresh.ProviderOriginal.EffectiveAt.Equal(*saved.ProviderOriginal.EffectiveAt))

		stored, err := transactionStore.GetTransaction(t.Context(), linked.ID)
		require.NoError(t, err)
		assert.Equal(t, saved.Kind, stored.Kind)
		assert.Equal(t, saved.TransferGroupID, stored.TransferGroupID)
		require.NotNil(t, saved.TransferMatchedAt)
		require.NotNil(t, stored.TransferMatchedAt)
		assert.True(t, saved.TransferMatchedAt.Equal(*stored.TransferMatchedAt))
		assert.Equal(t, saved.TransferMatchingExcluded, stored.TransferMatchingExcluded)
		assert.Equal(t, saved.Description, stored.Description)
		assert.Equal(t, saved.TagIDs, stored.TagIDs)
		require.NotNil(t, saved.ProviderOriginal)
		require.NotNil(t, stored.ProviderOriginal)
		assert.Equal(t, saved.ProviderOriginal.AmountMinor, stored.ProviderOriginal.AmountMinor)
		assert.Equal(t, saved.ProviderOriginal.Currency, stored.ProviderOriginal.Currency)
		assert.Equal(t, saved.ProviderOriginal.Description, stored.ProviderOriginal.Description)
		require.NotNil(t, saved.ProviderOriginal.EffectiveAt)
		require.NotNil(t, stored.ProviderOriginal.EffectiveAt)
		assert.True(t, saved.ProviderOriginal.EffectiveAt.Equal(*stored.ProviderOriginal.EffectiveAt))
	})

	t.Run("rejects duplicate IDs and handles absent or empty transaction reads", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 12, 16, 0, 0, 0, time.FixedZone("test", 5*60*60))
		database := openTestDatabase(t)
		transactionStore := NewTransactionTagStore(database)
		transaction := makeTransaction(fake, "tenant-"+fake.UUID().V4(), now)
		duplicateTagID := "tag-" + fake.UUID().V4()
		transaction.TagIDs = []string{duplicateTagID, duplicateTagID}
		_, err := transactionStore.SaveTransaction(t.Context(), transaction)
		require.ErrorIs(t, err, ErrDuplicateTransactionTag)

		_, err = transactionStore.GetTransaction(t.Context(), "missing-"+fake.UUID().V4())
		require.ErrorIs(t, err, ErrTransactionNotFound)
		items, err := transactionStore.ListTransactions(
			t.Context(),
			transaction.TenantID,
			"account-"+fake.UUID().V4(),
			domain.TransactionSourceManual,
			domain.TransactionStatusBooked,
			false,
			ListTransactionsPage{Limit: 1, Offset: 1},
		)
		require.NoError(t, err)
		assert.Empty(t, items)
	})
}
