package persistence

import (
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassificationTransactionStore(t *testing.T) {
	makeTransaction := func(fake faker.Faker, tenantID string, source domain.TransactionSource, kind domain.TransactionKind, status domain.TransactionStatus, at time.Time) domain.Transaction {
		return domain.Transaction{
			ID: "transaction-" + fake.UUID().V4(), TenantID: tenantID, AccountID: "account-" + fake.UUID().V4(),
			Source: source, Status: status, Kind: kind, AmountMinor: -1, Currency: "USD",
			Description: "description-" + fake.Lorem().Word(), EffectiveAt: at, CreatedAt: at, UpdatedAt: at,
		}
	}

	t.Run(
		"selects only eligible sources, tenants, kinds, and range rows then conditionally assigns",
		func(t *testing.T) {
			fake := faker.New()
			database := openTestDatabase(t)
			core := NewStore(database)
			store := NewClassificationTransactionStore(database)
			now := time.Date(2026, time.September, 6, 14, 0, 0, 0, time.FixedZone("test", 2*60*60))
			tenantID := "tenant-" + fake.UUID().V4()
			for _, transaction := range []domain.Transaction{
				makeTransaction(fake, tenantID, domain.TransactionSourceManual, domain.TransactionKindRegular, domain.TransactionStatusBooked, now),
				makeTransaction(fake, tenantID, domain.TransactionSourceCSV, domain.TransactionKindExpense, domain.TransactionStatusBooked, now),
				makeTransaction(fake, tenantID, domain.TransactionSourceProvider, domain.TransactionKindRefund, domain.TransactionStatusBooked, now),
				makeTransaction(fake, tenantID, domain.TransactionSourceProvider, domain.TransactionKindTransfer, domain.TransactionStatusBooked, now),
				makeTransaction(fake, tenantID, domain.TransactionSourceProvider, domain.TransactionKindIncome, domain.TransactionStatusPending, now),
				makeTransaction(fake, "tenant-other-"+fake.UUID().V4(), domain.TransactionSourceManual, domain.TransactionKindRegular, domain.TransactionStatusBooked, now),
				makeTransaction(fake, tenantID, domain.TransactionSourceManual, domain.TransactionKindRegular, domain.TransactionStatusBooked, now.Add(-48*time.Hour)),
			} {
				_, err := core.SaveTransaction(t.Context(), transaction)
				require.NoError(t, err)
			}
			selected, err := store.ListEligibleClassificationTransactions(
				t.Context(),
				ListEligibleClassificationTransactionsParams{
					TenantID: tenantID, RangeStart: now.Add(-time.Hour), RangeEndExclusive: now.Add(time.Hour),
				},
			)
			require.NoError(t, err)
			require.Len(t, selected, 3)
			assigned, err := store.AssignClassificationCategory(t.Context(), AssignClassificationCategoryParams{
				TenantID:      tenantID,
				TransactionID: selected[0].ID,
				CategoryID:    "category-" + fake.UUID().V4(),
				UpdatedAt:     now.Add(time.Minute),
			})
			require.NoError(t, err)
			assert.True(t, assigned)
			assigned, err = store.AssignClassificationCategory(t.Context(), AssignClassificationCategoryParams{
				TenantID:      tenantID,
				TransactionID: selected[0].ID,
				CategoryID:    "category-other-" + fake.UUID().V4(),
				UpdatedAt:     now.Add(2 * time.Minute),
			})
			require.NoError(t, err)
			assert.False(t, assigned)
		},
	)

	t.Run("uses a 200-row ID keyset and excludes the complete ineligible matrix", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		core := NewStore(database)
		store := NewClassificationTransactionStore(database)
		now := time.Date(2026, time.September, 7, 10, 0, 0, 0, time.FixedZone("test", -4*60*60))
		tenantID := "tenant-" + fake.UUID().V4()
		batchPrefix := "transaction-" + fake.UUID().V4() + "-"
		for index := range classificationBatchSize + 1 {
			transaction := makeTransaction(
				fake,
				tenantID,
				[]domain.TransactionSource{
					domain.TransactionSourceManual,
					domain.TransactionSourceCSV,
					domain.TransactionSourceProvider,
				}[index%3],
				[]domain.TransactionKind{
					domain.TransactionKindRegular,
					domain.TransactionKindExpense,
					domain.TransactionKindIncome,
					domain.TransactionKindRefund,
				}[index%4],
				domain.TransactionStatusBooked,
				now,
			)
			transaction.ID = fmt.Sprintf("%s%03d", batchPrefix, index)
			_, err := core.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		categoryID := "category-" + fake.UUID().V4()
		hiddenAt := now.Add(-time.Minute)
		ineligible := []domain.Transaction{
			makeTransaction(
				fake,
				tenantID,
				domain.TransactionSourceManual,
				domain.TransactionKindRegular,
				domain.TransactionStatusPending,
				now,
			),
			makeTransaction(
				fake,
				tenantID,
				domain.TransactionSourceCSV,
				domain.TransactionKindTransfer,
				domain.TransactionStatusBooked,
				now,
			),
			makeTransaction(
				fake,
				tenantID,
				domain.TransactionSourceProvider,
				domain.TransactionKindReconciliation,
				domain.TransactionStatusBooked,
				now,
			),
			makeTransaction(
				fake,
				tenantID,
				domain.TransactionSourceManual,
				domain.TransactionKindOpeningBalance,
				domain.TransactionStatusBooked,
				now,
			),
			makeTransaction(
				fake,
				tenantID,
				domain.TransactionSourceProvider,
				domain.TransactionKindIncome,
				domain.TransactionStatusBooked,
				now.Add(-time.Minute),
			),
			makeTransaction(
				fake,
				tenantID,
				domain.TransactionSourceCSV,
				domain.TransactionKindRefund,
				domain.TransactionStatusBooked,
				now.Add(time.Minute),
			),
			makeTransaction(
				fake,
				"tenant-other-"+fake.UUID().V4(),
				domain.TransactionSourceManual,
				domain.TransactionKindRegular,
				domain.TransactionStatusBooked,
				now,
			),
		}
		categorized := makeTransaction(
			fake,
			tenantID,
			domain.TransactionSourceManual,
			domain.TransactionKindRegular,
			domain.TransactionStatusBooked,
			now,
		)
		categorized.CategoryID = &categoryID
		ineligible = append(ineligible, categorized)
		hidden := makeTransaction(
			fake,
			tenantID,
			domain.TransactionSourceProvider,
			domain.TransactionKindExpense,
			domain.TransactionStatusBooked,
			now,
		)
		hidden.HiddenAt = &hiddenAt
		ineligible = append(ineligible, hidden)
		for _, transaction := range ineligible {
			_, err := core.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		params := ListEligibleClassificationTransactionsParams{
			TenantID: tenantID, RangeStart: now, RangeEndExclusive: now.Add(time.Minute),
		}
		firstBatch, err := store.ListEligibleClassificationTransactions(t.Context(), params)
		require.NoError(t, err)
		require.Len(t, firstBatch, classificationBatchSize)
		assert.Equal(t, fmt.Sprintf("%s%03d", batchPrefix, 0), firstBatch[0].ID)
		assert.Equal(
			t,
			fmt.Sprintf("%s%03d", batchPrefix, classificationBatchSize-1),
			firstBatch[classificationBatchSize-1].ID,
		)

		params.AfterID = firstBatch[len(firstBatch)-1].ID
		secondBatch, err := store.ListEligibleClassificationTransactions(t.Context(), params)
		require.NoError(t, err)
		require.Len(t, secondBatch, 1)
		assert.Equal(t, fmt.Sprintf("%s%03d", batchPrefix, classificationBatchSize), secondBatch[0].ID)

		assigned, err := store.AssignClassificationCategory(t.Context(), AssignClassificationCategoryParams{
			TenantID: tenantID, TransactionID: hidden.ID, CategoryID: "category-" + fake.UUID().V4(), UpdatedAt: now,
		})
		require.NoError(t, err)
		assert.False(t, assigned)
		storedHidden, err := core.GetTransaction(t.Context(), hidden.ID)
		require.NoError(t, err)
		assert.Nil(t, storedHidden.CategoryID)
	})

	t.Run("wraps selection and assignment database failures", func(t *testing.T) {
		fake := faker.New()
		makeClosedStore := func(t *testing.T) *ClassificationTransactionStore {
			t.Helper()
			database := openTestDatabase(t)
			sqlDB, err := database.db.DB()
			require.NoError(t, err)
			require.NoError(t, sqlDB.Close())
			return NewClassificationTransactionStore(database)
		}
		now := time.Date(2026, time.September, 6, 16, 0, 0, 0, time.FixedZone("test", 2*60*60))
		params := ListEligibleClassificationTransactionsParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now.Add(-time.Hour), RangeEndExclusive: now,
		}
		_, err := makeClosedStore(t).ListEligibleClassificationTransactions(t.Context(), params)
		require.Error(t, err)
		_, err = makeClosedStore(t).AssignClassificationCategory(t.Context(), AssignClassificationCategoryParams{
			TenantID: params.TenantID, TransactionID: "transaction-" + fake.UUID().V4(),
			CategoryID: "category-" + fake.UUID().V4(), UpdatedAt: now,
		})
		require.Error(t, err)
	})
}
