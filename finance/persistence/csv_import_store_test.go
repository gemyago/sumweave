package persistence

import (
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSVImportStore(t *testing.T) {
	makeRow := func(fake faker.Faker) domain.CSVImportTransactionRow {
		return domain.CSVImportTransactionRow{
			ImportID:     "import-" + fake.UUID().V4(),
			RowNumber:    2,
			TenantID:     "tenant-" + fake.UUID().V4(),
			ActorUserID:  "actor-" + fake.UUID().V4(),
			AccountName:  "account-" + fake.Lorem().Word(),
			CategoryName: "category-" + fake.Lorem().Word(),
			TagNames:     []string{"tag-" + fake.Lorem().Word()},
			Currency:     "USD",
			Description:  "description-" + fake.Lorem().Word(),
			AmountMinor:  -100,
			EffectiveAt:  time.Date(2026, time.May, 29, 0, 0, 0, 0, time.Local),
		}
	}

	t.Run("reuses durable outcomes and records rejected ambiguous rows", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := NewCSVImportStore(database)
		row := makeRow(fake)
		outcome, err := store.ImportTransactionRow(t.Context(), row)
		require.NoError(t, err)
		assert.Equal(t, domain.CSVImportRowOutcomeImported, outcome.Status)
		retried, err := store.ImportTransactionRow(t.Context(), row)
		require.NoError(t, err)
		assert.Equal(t, outcome.ImportID, retried.ImportID)
		assert.Equal(t, outcome.RowNumber, retried.RowNumber)
		assert.Equal(t, outcome.Status, retried.Status)
		assert.Equal(t, outcome.TransactionID, retried.TransactionID)
		outcomes, err := store.ListCSVImportRowOutcomes(t.Context(), row.ImportID)
		require.NoError(t, err)
		require.Len(t, outcomes, 1)
		assert.Equal(t, outcome.ImportID, outcomes[0].ImportID)
		assert.Equal(t, outcome.TransactionID, outcomes[0].TransactionID)

		ambiguous := makeRow(fake)
		for range 2 {
			require.NoError(t, database.db.WithContext(t.Context()).Create(&accountModel{
				ID:        fake.UUID().V4(),
				TenantID:  ambiguous.TenantID,
				Name:      ambiguous.AccountName,
				Currency:  ambiguous.Currency,
				Kind:      string(domain.AccountKindManual),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).Error)
		}
		rejected, err := store.ImportTransactionRow(t.Context(), ambiguous)
		require.NoError(t, err)
		assert.Equal(t, domain.CSVImportRowOutcomeRejected, rejected.Status)
		assert.Contains(t, rejected.Reason, "ambiguous")
	})

	t.Run("persists the normalized fallback description", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := NewCSVImportStore(database)
		row := makeRow(fake)
		row.Description = "n/a"
		outcome, err := store.ImportTransactionRow(t.Context(), row)
		require.NoError(t, err)
		var transaction transactionModel
		require.NoError(t, database.db.WithContext(t.Context()).
			First(&transaction, "id = ?", outcome.TransactionID).Error)
		assert.Equal(t, "n/a", transaction.Description)
	})

	t.Run("surfaces database failures from outcome lookup and listing", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := NewCSVImportStore(database)
		sqlDB, err := database.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		_, err = store.ImportTransactionRow(t.Context(), makeRow(fake))
		require.Error(t, err)
		_, err = store.ListCSVImportRowOutcomes(t.Context(), "import-"+fake.UUID().V4())
		require.Error(t, err)
		_, err = createImportTransaction(database.db, makeRow(fake), "account-"+fake.UUID().V4(), nil, time.Now())
		require.Error(t, err)
		_, err = resolveImportAccount(database.db, makeRow(fake), time.Now())
		require.Error(t, err)
		_, _, err = resolveImportCategory(database.db, makeRow(fake), time.Now())
		require.Error(t, err)
		_, err = resolveImportTags(database.db, makeRow(fake), time.Now())
		require.Error(t, err)
		_, _, err = getCSVImportRowOutcome(database.db, "import-"+fake.UUID().V4(), 2)
		require.Error(t, err)
		_, _, err = importCategoryByName(database.db, "tenant-"+fake.UUID().V4(), "category")
		require.Error(t, err)
		_, _, err = importTagByName(database.db, "tenant-"+fake.UUID().V4(), "tag")
		require.Error(t, err)
	})

	t.Run("rejects an existing account with the wrong currency", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		row := makeRow(fake)
		now := time.Now()
		require.NoError(t, database.db.WithContext(t.Context()).Create(&accountModel{
			ID:        fake.UUID().V4(),
			TenantID:  row.TenantID,
			Name:      row.AccountName,
			Currency:  "EUR",
			Kind:      string(domain.AccountKindManual),
			CreatedAt: now,
			UpdatedAt: now,
		}).Error)
		_, err := resolveImportAccount(database.db, row, now)
		require.Error(t, err)
		row.CategoryName = ""
		_, found, err := resolveImportCategory(database.db, row, now)
		require.NoError(t, err)
		assert.False(t, found)
		row.TagNames = nil
		tags, err := resolveImportTags(database.db, row, now)
		require.NoError(t, err)
		assert.Empty(t, tags)
		_, found, err = getCSVImportRowOutcome(database.db, "missing-"+fake.UUID().V4(), 2)
		require.NoError(t, err)
		assert.False(t, found)
		for range 2 {
			require.NoError(t, database.db.WithContext(t.Context()).Create(&categoryModel{
				ID:        fake.UUID().V4(),
				TenantID:  row.TenantID,
				Name:      "duplicate-category",
				Kind:      string(domain.CategoryKindExpense),
				CreatedAt: now,
				UpdatedAt: now,
			}).Error)
		}
		_, _, err = importCategoryByName(database.db, row.TenantID, "duplicate-category")
		require.Error(t, err)
		for range 2 {
			require.NoError(t, database.db.WithContext(t.Context()).Create(&tagModel{
				ID:        fake.UUID().V4(),
				TenantID:  row.TenantID,
				Name:      "duplicate-tag",
				CreatedAt: now,
				UpdatedAt: now,
			}).Error)
		}
		_, _, err = importTagByName(database.db, row.TenantID, "duplicate-tag")
		require.Error(t, err)
	})
}
