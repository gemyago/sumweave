//go:build postgres_test

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

		replaced.TagIDs = []string{}
		cleared, err := transactionStore.SaveTransaction(t.Context(), replaced)
		require.NoError(t, err)
		assert.Empty(t, cleared.TagIDs)
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
