package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCashFlowSeriesStore(t *testing.T) {
	makeFixture := func(t *testing.T) (*Store, *TransactionTagStore, *CashFlowSeriesStore, domain.Tenant, domain.Account, time.Time) {
		t.Helper()
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 10, 30, 0, 0, time.FixedZone("series", 2*60*60))
		database := openTestDatabase(t)
		store := NewStore(database)
		tenant := domain.Tenant{
			ID:              "tenant-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "EUR",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_, err := store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)
		account := domain.Account{
			ID: "account-" + fake.UUID().V4(), TenantID: tenant.ID,
			Name: "account-" + fake.Lorem().Word(), Currency: "EUR", Kind: domain.AccountKindManual,
			CreatedAt: now, UpdatedAt: now,
		}
		_, err = store.SaveAccount(t.Context(), account)
		require.NoError(t, err)
		return store, NewTransactionTagStore(database), NewCashFlowSeriesStore(database), tenant, account, now
	}
	makeTransaction := func(fake faker.Faker, tenantID string, accountID string, at time.Time, amount int64, kind domain.TransactionKind) domain.Transaction {
		return domain.Transaction{
			ID: "transaction-" + fake.UUID().V4(), TenantID: tenantID, AccountID: accountID,
			Source: domain.TransactionSourceManual, Status: domain.TransactionStatusBooked, Kind: kind,
			AmountMinor: amount, Currency: "EUR", Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: at, CreatedAt: at, UpdatedAt: at,
		}
	}

	t.Run("generates anchored ordered daily buckets with clipped ends and settled contributions", func(t *testing.T) {
		fake := faker.New()
		_, transactions, seriesStore, tenant, account, now := makeFixture(t)
		start := now.Add(-time.Hour)
		end := start.AddDate(0, 0, 2).Add(15 * time.Minute)
		for _, transaction := range []domain.Transaction{
			makeTransaction(fake, tenant.ID, account.ID, start.Add(time.Minute), 101, domain.TransactionKindIncome),
			makeTransaction(fake, tenant.ID, account.ID, start.AddDate(0, 0, 1).Add(time.Minute), -55, domain.TransactionKindExpense),
		} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		actual, err := seriesStore.GetCashFlowSeries(t.Context(), CashFlowSeriesParams{
			TenantID:   tenant.ID,
			StartDate:  start,
			EndDate:    end,
			GroupBy:    CashFlowGroupByDay,
			FXProvider: "provider-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		assert.Equal(t, "EUR", actual.DisplayCurrency)
		assert.True(t, actual.Complete)
		require.Len(t, actual.Buckets, 3)
		assert.True(t, start.Equal(actual.Buckets[0].StartDate))
		assert.True(t, start.AddDate(0, 0, 1).Equal(actual.Buckets[0].EndDate))
		assert.Equal(t, int64(101), actual.Buckets[0].IncomeMinor)
		assert.True(t, start.AddDate(0, 0, 1).Equal(actual.Buckets[1].StartDate))
		assert.Equal(t, int64(55), actual.Buckets[1].ExpenseMinor)
		assert.True(t, start.AddDate(0, 0, 2).Equal(actual.Buckets[2].StartDate))
		assert.True(t, end.Equal(actual.Buckets[2].EndDate))
		assert.Zero(t, actual.Buckets[2].IncomeMinor)
		assert.Zero(t, actual.Buckets[2].ExpenseMinor)
	})

	t.Run("aggregates monthly reporting rules, rounding, and grouped missing FX", func(t *testing.T) {
		fake := faker.New()
		store, transactions, seriesStore, tenant, account, now := makeFixture(t)
		start := time.Date(2026, time.January, 15, 9, 30, 0, 0, time.FixedZone("monthly", 2*60*60))
		end := start.AddDate(0, 3, 1)
		hiddenAt := now
		foreignAccount := domain.Account{
			ID: "account-usd-" + fake.UUID().V4(), TenantID: tenant.ID,
			Name: "account-usd-" + fake.Lorem().Word(), Currency: "USD", Kind: domain.AccountKindManual,
			CreatedAt: now, UpdatedAt: now,
		}
		missingAccount := domain.Account{
			ID: "account-gbp-" + fake.UUID().V4(), TenantID: tenant.ID,
			Name: "account-gbp-" + fake.Lorem().Word(), Currency: "GBP", Kind: domain.AccountKindManual,
			CreatedAt: now, UpdatedAt: now,
		}
		hiddenAccount := domain.Account{
			ID: "account-hidden-" + fake.UUID().V4(), TenantID: tenant.ID,
			Name: "account-hidden-" + fake.Lorem().Word(), Currency: "EUR", Kind: domain.AccountKindManual,
			HiddenAt: &hiddenAt, CreatedAt: now, UpdatedAt: now,
		}
		for _, item := range []domain.Account{foreignAccount, missingAccount, hiddenAccount} {
			_, err := store.SaveAccount(t.Context(), item)
			require.NoError(t, err)
		}
		provider := "provider-" + fake.UUID().V4()
		require.NoError(t, NewCurrentFXRateStoreFromStore(store).SaveCurrentFXRates(t.Context(), []domain.FXRate{{
			Provider: provider, BaseCurrency: "USD", QuoteCurrency: "EUR", Rate: 1.5,
			EffectiveAt: now, LastSuccessfulRefreshAt: now,
		}}))
		makeAt := func(months int) time.Time { return start.AddDate(0, months, 0).Add(time.Minute) }
		included := []domain.Transaction{
			makeTransaction(fake, tenant.ID, account.ID, makeAt(0), 100, domain.TransactionKindIncome),
			makeTransaction(fake, tenant.ID, account.ID, makeAt(0), -50, domain.TransactionKindExpense),
			makeTransaction(fake, tenant.ID, account.ID, makeAt(0), 200, domain.TransactionKindRefund),
			makeTransaction(fake, tenant.ID, foreignAccount.ID, makeAt(0), -101, domain.TransactionKindRegular),
			makeTransaction(fake, tenant.ID, missingAccount.ID, makeAt(0), 100, domain.TransactionKindIncome),
			makeTransaction(fake, tenant.ID, missingAccount.ID, makeAt(1), -100, domain.TransactionKindExpense),
		}
		included[3].Currency = foreignAccount.Currency
		included[4].Currency = missingAccount.Currency
		included[5].Currency = missingAccount.Currency
		excluded := []domain.Transaction{
			makeTransaction(fake, tenant.ID, account.ID, makeAt(0), 999, domain.TransactionKindReconciliation),
			makeTransaction(fake, tenant.ID, account.ID, makeAt(0), 999, domain.TransactionKindOpeningBalance),
			makeTransaction(fake, tenant.ID, account.ID, makeAt(0), 999, domain.TransactionKindRegular),
			makeTransaction(fake, tenant.ID, hiddenAccount.ID, makeAt(0), 999, domain.TransactionKindIncome),
		}
		excluded[1].HiddenAt = &hiddenAt
		excluded[2].Status = domain.TransactionStatusPending
		matchedAt := makeAt(0)
		matchedTransfer := makeTransaction(fake, tenant.ID, account.ID, makeAt(0), 999, domain.TransactionKindTransfer)
		matchedTransfer.TransferMatchedAt = &matchedAt
		excluded = append(excluded, matchedTransfer)
		for _, transaction := range append(included, excluded...) {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		actual, err := seriesStore.GetCashFlowSeries(t.Context(), CashFlowSeriesParams{
			TenantID: tenant.ID, StartDate: start, EndDate: end, GroupBy: CashFlowGroupByMonth, FXProvider: provider,
		})
		require.NoError(t, err)
		require.Len(t, actual.Buckets, 4)
		for index, bucket := range actual.Buckets {
			assert.True(t, start.AddDate(0, index, 0).Equal(bucket.StartDate))
		}
		assert.Equal(t, int64(100), actual.Buckets[0].IncomeMinor)
		assert.Equal(t, int64(2), actual.Buckets[0].ExpenseMinor)
		assert.Zero(t, actual.Buckets[1].IncomeMinor)
		assert.Zero(t, actual.Buckets[1].ExpenseMinor)
		assert.True(t, end.Equal(actual.Buckets[3].EndDate))
		assert.False(t, actual.Complete)
		assert.Equal(t, []domain.CashFlowMissingFXDiagnostic{{
			Provider: provider, BaseCurrency: "GBP", QuoteCurrency: "EUR", AffectedTransactionCount: 2,
		}}, actual.MissingFX)
	})

	t.Run("rejects unsupported grouping and returns database failures", func(t *testing.T) {
		fake := faker.New()
		_, _, seriesStore, tenant, _, now := makeFixture(t)
		unsupported := CashFlowGroupBy("unsupported-" + fake.Lorem().Word())
		_, err := cashFlowSeriesQuery(unsupported)
		require.Error(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = seriesStore.GetCashFlowSeries(ctx, CashFlowSeriesParams{
			TenantID: tenant.ID, StartDate: now, EndDate: now.Add(time.Hour),
			GroupBy: CashFlowGroupByDay, FXProvider: "provider-" + fake.UUID().V4(),
		})
		require.Error(t, err)
	})
}
