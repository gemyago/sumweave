package persistence

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTransferPairStore(t *testing.T) {
	makeTransaction := func(fake faker.Faker, tenantID string, accountID string, now time.Time) domain.Transaction {
		categoryID := "category-" + fake.UUID().V4()
		return domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    tenantID,
			AccountID:   accountID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -int64(fake.IntBetween(1, 10_000)),
			Currency:    "USD",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: now,
			CategoryID:  &categoryID,
			CreatedAt:   now,
			UpdatedAt:   now,
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -int64(fake.IntBetween(1, 10_000)),
				Currency:    "USD",
				Description: "provider-" + fake.Lorem().Word(),
				EffectiveAt: &now,
			},
		}
	}
	makeParams := func(fake faker.Faker, tenantID string, firstID string, secondID string, now time.Time) TransferPairLinkParams {
		return TransferPairLinkParams{
			TenantID:            tenantID,
			FirstTransactionID:  firstID,
			SecondTransactionID: secondID,
			TransferGroupID:     "group-" + fake.UUID().V4(),
			TransferMatchedAt:   now.Add(time.Minute),
			UpdatedAt:           now.Add(2 * time.Minute),
		}
	}
	makeStores := func(t *testing.T) (*Database, *Store, *TransactionTagStore, *TransferPairStore) {
		t.Helper()
		database := openTestDatabase(t)
		return database, NewStore(database), NewTransactionTagStore(database), NewTransferPairStore(database)
	}
	assertLedgerDataPreserved := func(t *testing.T, expected domain.Transaction, actual domain.Transaction) {
		t.Helper()
		assert.Equal(t, expected.ID, actual.ID)
		assert.Equal(t, expected.TenantID, actual.TenantID)
		assert.Equal(t, expected.AccountID, actual.AccountID)
		assert.Equal(t, expected.Source, actual.Source)
		assert.Equal(t, expected.Status, actual.Status)
		assert.Equal(t, expected.AmountMinor, actual.AmountMinor)
		assert.Equal(t, expected.Currency, actual.Currency)
		assert.Equal(t, expected.Description, actual.Description)
		assert.True(t, expected.EffectiveAt.Equal(actual.EffectiveAt))
		assert.Equal(t, expected.CategoryID, actual.CategoryID)
		assert.ElementsMatch(t, expected.TagIDs, actual.TagIDs)
		require.NotNil(t, expected.ProviderOriginal)
		require.NotNil(t, actual.ProviderOriginal)
		assert.Equal(t, expected.ProviderOriginal.AmountMinor, actual.ProviderOriginal.AmountMinor)
		assert.Equal(t, expected.ProviderOriginal.Currency, actual.ProviderOriginal.Currency)
		assert.Equal(t, expected.ProviderOriginal.Description, actual.ProviderOriginal.Description)
		require.NotNil(t, expected.ProviderOriginal.EffectiveAt)
		require.NotNil(t, actual.ProviderOriginal.EffectiveAt)
		assert.True(t, expected.ProviderOriginal.EffectiveAt.Equal(*actual.ProviderOriginal.EffectiveAt))
	}

	t.Run("links and unlinks existing tenant rows without changing ledger data", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.FixedZone("pairs", 2*60*60))
		_, coreStore, transactions, store := makeStores(t)
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
		first := makeTransaction(fake, tenantID, "account-first-"+fake.UUID().V4(), now)
		second := makeTransaction(fake, tenantID, "account-second-"+fake.UUID().V4(), now.Add(time.Hour))
		first.TagIDs = []string{firstTag.ID}
		second.TagIDs = []string{secondTag.ID}
		second.AmountMinor = -first.AmountMinor + 1
		second.Currency = "EUR"
		first.TransferMatchingExcluded = true
		for _, transaction := range []domain.Transaction{first, second} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		params := makeParams(fake, tenantID, first.ID, second.ID, now)
		require.NoError(t, store.LinkTransferPair(t.Context(), params))
		linkedFirst, err := transactions.GetTransaction(t.Context(), first.ID)
		require.NoError(t, err)
		linkedSecond, err := transactions.GetTransaction(t.Context(), second.ID)
		require.NoError(t, err)
		for _, linked := range []*domain.Transaction{linkedFirst, linkedSecond} {
			assert.Equal(t, domain.TransactionKindTransfer, linked.Kind)
			assert.Equal(t, params.TransferGroupID, *linked.TransferGroupID)
			assert.True(t, params.TransferMatchedAt.Equal(*linked.TransferMatchedAt))
			assert.True(t, params.UpdatedAt.Equal(linked.UpdatedAt))
		}
		assert.True(t, linkedFirst.TransferMatchingExcluded)
		assert.False(t, linkedSecond.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, first, *linkedFirst)
		assertLedgerDataPreserved(t, second, *linkedSecond)

		unlinkParams := TransferPairUnlinkParams{
			TenantID:            tenantID,
			FirstTransactionID:  first.ID,
			SecondTransactionID: second.ID,
			UpdatedAt:           now.Add(3 * time.Minute),
		}
		require.NoError(t, store.UnlinkTransferPair(t.Context(), unlinkParams))
		for _, transactionID := range []string{first.ID, second.ID} {
			unlinked, getErr := transactions.GetTransaction(t.Context(), transactionID)
			require.NoError(t, getErr)
			assert.Equal(t, domain.TransactionKindRegular, unlinked.Kind)
			assert.Nil(t, unlinked.TransferGroupID)
			assert.Nil(t, unlinked.TransferMatchedAt)
			assert.True(t, unlinked.TransferMatchingExcluded)
			assert.True(t, unlinkParams.UpdatedAt.Equal(unlinked.UpdatedAt))
			if transactionID == first.ID {
				assertLedgerDataPreserved(t, first, *unlinked)
			} else {
				assertLedgerDataPreserved(t, second, *unlinked)
			}
		}
	})

	t.Run("rolls back when either tenant-scoped row is missing", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.FixedZone("pairs", -3*60*60))
		_, _, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		first := makeTransaction(fake, tenantID, "account-first-"+fake.UUID().V4(), now)
		_, err := transactions.SaveTransaction(t.Context(), first)
		require.NoError(t, err)

		params := makeParams(fake, tenantID, first.ID, "missing-"+fake.UUID().V4(), now)
		require.ErrorIs(t, store.LinkTransferPair(t.Context(), params), ErrTransferPairTransactionNotFound)
		stored, err := transactions.GetTransaction(t.Context(), first.ID)
		require.NoError(t, err)
		assert.Equal(t, first.Kind, stored.Kind)
		assert.Nil(t, stored.TransferGroupID)
		assert.Nil(t, stored.TransferMatchedAt)
		assert.Equal(t, first.TransferMatchingExcluded, stored.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, first, *stored)
		_, err = transactions.GetTransaction(t.Context(), params.SecondTransactionID)
		require.ErrorIs(t, err, ErrTransactionNotFound)
	})

	t.Run("rejects duplicate transaction IDs without updating the row", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 13, 0, 0, 0, time.FixedZone("pairs", -2*60*60))
		_, _, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		transaction := makeTransaction(fake, tenantID, "account-"+fake.UUID().V4(), now)
		_, err := transactions.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)

		err = store.LinkTransferPair(t.Context(), makeParams(fake, tenantID, transaction.ID, transaction.ID, now))
		require.ErrorIs(t, err, ErrTransferPairTransactionIDsMustDiffer)
		stored, err := transactions.GetTransaction(t.Context(), transaction.ID)
		require.NoError(t, err)
		assert.Equal(t, transaction.Kind, stored.Kind)
		assert.Nil(t, stored.TransferGroupID)
		assert.Nil(t, stored.TransferMatchedAt)
		assert.Equal(t, transaction.TransferMatchingExcluded, stored.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, transaction, *stored)
	})

	t.Run("rolls back both legs when the second narrow update fails", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 14, 0, 0, 0, time.FixedZone("pairs", 5*60*60))
		database, _, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		first := makeTransaction(fake, tenantID, "account-first-"+fake.UUID().V4(), now)
		second := makeTransaction(fake, tenantID, "account-second-"+fake.UUID().V4(), now)
		for _, transaction := range []domain.Transaction{first, second} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}
		sentinel := errors.New("second narrow pair update failed")
		callbackName := "transfer-pair-" + fake.UUID().V4()
		var updates int
		require.NoError(
			t,
			database.db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
				if tx.Statement.Table == (transactionModel{}).TableName() {
					updates++
					if updates == 2 {
						tx.AddError(sentinel)
					}
				}
			}),
		)
		defer database.db.Callback().Update().Remove(callbackName)

		require.ErrorIs(
			t,
			store.LinkTransferPair(t.Context(), makeParams(fake, tenantID, first.ID, second.ID, now)),
			sentinel,
		)
		storedFirst, err := transactions.GetTransaction(t.Context(), first.ID)
		require.NoError(t, err)
		storedSecond, err := transactions.GetTransaction(t.Context(), second.ID)
		require.NoError(t, err)
		assert.Equal(t, first.Kind, storedFirst.Kind)
		assert.Nil(t, storedFirst.TransferGroupID)
		assert.Nil(t, storedFirst.TransferMatchedAt)
		assert.Equal(t, first.TransferMatchingExcluded, storedFirst.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, first, *storedFirst)
		assert.Equal(t, second.Kind, storedSecond.Kind)
		assert.Nil(t, storedSecond.TransferGroupID)
		assert.Nil(t, storedSecond.TransferMatchedAt)
		assert.Equal(t, second.TransferMatchingExcluded, storedSecond.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, second, *storedSecond)
	})
}
