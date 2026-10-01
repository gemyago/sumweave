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

	t.Run("anchors twelve monthly buckets across February and daylight saving time", func(t *testing.T) {
		fake := faker.New()
		_, _, seriesStore, tenant, _, _ := makeFixture(t)
		location, err := time.LoadLocation("America/Los_Angeles")
		require.NoError(t, err)
		start := time.Date(2025, time.October, 31, 1, 30, 0, 0, location)
		end := time.Date(2026, time.October, 31, 1, 30, 0, 0, location)

		actual, err := seriesStore.GetCashFlowSeries(t.Context(), CashFlowSeriesParams{
			TenantID: tenant.ID, StartDate: start, EndDate: end,
			GroupBy: CashFlowGroupByMonth, FXProvider: "provider-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		require.Len(t, actual.Buckets, 12)
		assert.Equal(t, []int{31, 30, 31, 31, 28, 31, 30, 31, 30, 31, 31, 30}, func() []int {
			days := make([]int, len(actual.Buckets))
			for index, bucket := range actual.Buckets {
				days[index] = bucket.StartDate.UTC().Day()
			}
			return days
		}())
		assert.True(t, end.Equal(actual.Buckets[len(actual.Buckets)-1].EndDate))
	})

	t.Run("uses local calendar month bounds across DST and clips exact range edges", func(t *testing.T) {
		// Fixed calendar dates are regression inputs: month-end and DST are the behavior under test.
		for _, zone := range []string{"Europe/Warsaw", "America/Los_Angeles", "Asia/Kolkata"} {
			t.Run(zone, func(t *testing.T) {
				fake := faker.New()
				_, transactions, seriesStore, tenant, account, _ := makeFixture(t)
				location, err := time.LoadLocation(zone)
				require.NoError(t, err)
				start := time.Date(2026, time.May, 1, 0, 0, 0, 0, location)
				end := time.Date(2026, time.November, 1, 0, 0, 0, 0, location)
				income := int64(fake.IntBetween(100, 500))
				expense := int64(fake.IntBetween(100, 500))
				expected := make([]domain.CashFlowSeriesBucket, 6)
				for index := range expected {
					boundary := time.Date(2026, time.May+time.Month(index), 1, 0, 0, 0, 0, location)
					next := time.Date(2026, time.June+time.Month(index), 1, 0, 0, 0, 0, location)
					expected[index] = domain.CashFlowSeriesBucket{
						StartDate: boundary, EndDate: next, IncomeMinor: income, ExpenseMinor: expense,
					}
					for _, transaction := range []domain.Transaction{
						makeTransaction(fake, tenant.ID, account.ID, boundary, income, domain.TransactionKindRegular),
						makeTransaction(fake, tenant.ID, account.ID, next.Add(-time.Microsecond), -expense, domain.TransactionKindRegular),
					} {
						_, err = transactions.SaveTransaction(t.Context(), transaction)
						require.NoError(t, err)
					}
				}
				for _, at := range []time.Time{start.Add(-time.Microsecond), end} {
					_, err = transactions.SaveTransaction(t.Context(),
						makeTransaction(fake, tenant.ID, account.ID, at, income, domain.TransactionKindRegular))
					require.NoError(t, err)
				}
				params := CashFlowSeriesParams{
					TenantID: tenant.ID, StartDate: start, EndDate: end,
					GroupBy: CashFlowGroupByMonth, TimeZone: zone, FXProvider: "provider-" + fake.UUID().V4(),
				}
				assertBuckets := func(expected []domain.CashFlowSeriesBucket) {
					actual, queryErr := seriesStore.GetCashFlowSeries(t.Context(), params)
					require.NoError(t, queryErr)
					for index := range actual.Buckets {
						actual.Buckets[index].StartDate = actual.Buckets[index].StartDate.In(location)
						actual.Buckets[index].EndDate = actual.Buckets[index].EndDate.In(location)
					}
					require.Equal(t, expected, actual.Buckets)
				}
				assertBuckets(expected)
				// Twelve months cross both spring and fall DST, and February.
				params.StartDate = time.Date(2025, time.November, 1, 0, 0, 0, 0, location)
				twelveMonths := make([]domain.CashFlowSeriesBucket, 12)
				for index := range 6 {
					twelveMonths[index] = domain.CashFlowSeriesBucket{
						StartDate: time.Date(2025, time.November+time.Month(index), 1, 0, 0, 0, 0, location),
						EndDate:   time.Date(2025, time.December+time.Month(index), 1, 0, 0, 0, 0, location),
					}
				}
				twelveMonths[5].IncomeMinor = income // The pre-May edge row is inside April.
				copy(twelveMonths[6:], expected)
				assertBuckets(twelveMonths)
				// Reproduce the wire representation: Z is an instant, not the browser calendar.
				params.StartDate = start.In(time.FixedZone("wire", 0))
				params.EndDate = end.In(time.FixedZone("wire", 0))
				assertBuckets(expected)
				shift := 37*time.Minute + 123*time.Millisecond
				params.StartDate = start.Add(shift)
				params.EndDate = end.Add(-time.Hour)
				for index := range expected {
					expected[index].StartDate = expected[index].StartDate.Add(shift)
					expected[index].EndDate = expected[index].EndDate.Add(shift)
					expected[index].IncomeMinor = 0
					if index < len(expected)-1 {
						expected[index].IncomeMinor = income
					}
				}
				expected[len(expected)-1].EndDate = params.EndDate
				expected[len(expected)-1].ExpenseMinor = 0
				assertBuckets(expected)
			})
		}
	})

	t.Run("preserves fold instants and resolves subsequent month anchors consistently", func(t *testing.T) {
		// Explicit offsets distinguish both occurrences; fixed dates reproduce DST defects.
		for _, tc := range []struct {
			name   string
			zone   string
			bounds []string
		}{
			{"Warsaw earlier fold", "Europe/Warsaw", []string{"2026-10-25T02:30:00+02:00", "2026-11-25T02:30:00+01:00", "2026-12-01T00:00:00+01:00"}},
			{"Warsaw later fold", "Europe/Warsaw", []string{"2026-10-25T02:30:00+01:00", "2026-11-25T02:30:00+01:00", "2026-12-01T00:00:00+01:00"}},
			{"Warsaw short earlier fold", "Europe/Warsaw", []string{"2026-10-25T02:30:00+02:00", "2026-10-25T02:45:00+02:00"}},
			{"Warsaw short later fold", "Europe/Warsaw", []string{"2026-10-25T02:30:00+01:00", "2026-10-25T02:45:00+01:00"}},
			{"LA earlier fold", "America/Los_Angeles", []string{"2026-11-01T01:30:00-07:00", "2026-12-01T01:30:00-08:00", "2026-12-02T00:00:00-08:00"}},
			{"LA later fold", "America/Los_Angeles", []string{"2026-11-01T01:30:00-08:00", "2026-12-01T01:30:00-08:00", "2026-12-02T00:00:00-08:00"}},
			{"LA subsequent fold chooses later", "America/Los_Angeles", []string{"2026-10-01T01:30:00-07:00", "2026-11-01T01:30:00-08:00", "2026-12-01T01:30:00-08:00"}},
			{"LA missing time moves forward without shifting later anchors", "America/Los_Angeles", []string{"2026-02-08T02:30:00-08:00", "2026-03-08T03:30:00-07:00", "2026-04-08T02:30:00-07:00"}},
			{"Warsaw subsequent fold chooses later", "Europe/Warsaw", []string{"2026-09-25T02:30:00+02:00", "2026-10-25T02:30:00+01:00", "2026-11-25T02:30:00+01:00"}},
			{"Warsaw missing time moves forward", "Europe/Warsaw", []string{"2026-01-29T02:30:00+01:00", "2026-02-28T02:30:00+01:00", "2026-03-29T03:30:00+02:00", "2026-04-29T02:30:00+02:00"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				fake := faker.New()
				_, transactions, seriesStore, tenant, account, _ := makeFixture(t)
				location, err := time.LoadLocation(tc.zone)
				require.NoError(t, err)
				bounds := make([]time.Time, len(tc.bounds))
				for index, value := range tc.bounds {
					bounds[index], err = time.Parse(time.RFC3339, value)
					require.NoError(t, err)
					bounds[index] = bounds[index].In(location)
				}
				income := int64(fake.IntBetween(100, 500))
				expense := int64(fake.IntBetween(100, 500))
				expected := make([]domain.CashFlowSeriesBucket, len(bounds)-1)
				for index := range expected {
					expected[index] = domain.CashFlowSeriesBucket{
						StartDate: bounds[index], EndDate: bounds[index+1], IncomeMinor: income, ExpenseMinor: expense,
					}
					for _, transaction := range []domain.Transaction{
						makeTransaction(fake, tenant.ID, account.ID, bounds[index], income, domain.TransactionKindRegular),
						makeTransaction(fake, tenant.ID, account.ID, bounds[index+1].Add(-time.Microsecond), -expense, domain.TransactionKindRegular),
					} {
						_, err = transactions.SaveTransaction(t.Context(), transaction)
						require.NoError(t, err)
					}
				}
				for _, at := range []time.Time{bounds[0].Add(-time.Microsecond), bounds[len(bounds)-1]} {
					_, err = transactions.SaveTransaction(t.Context(),
						makeTransaction(fake, tenant.ID, account.ID, at, income, domain.TransactionKindRegular))
					require.NoError(t, err)
				}
				actual, err := seriesStore.GetCashFlowSeries(t.Context(), CashFlowSeriesParams{
					TenantID: tenant.ID, StartDate: bounds[0], EndDate: bounds[len(bounds)-1],
					GroupBy: CashFlowGroupByMonth, TimeZone: tc.zone, FXProvider: "provider-" + fake.UUID().V4(),
				})
				require.NoError(t, err)
				for index := range actual.Buckets {
					actual.Buckets[index].StartDate = actual.Buckets[index].StartDate.In(location)
					actual.Buckets[index].EndDate = actual.Buckets[index].EndDate.In(location)
					require.True(t, actual.Buckets[index].StartDate.Before(actual.Buckets[index].EndDate))
				}
				require.Equal(t,
					domain.CashFlowPeriod{StartDate: bounds[0], EndDate: bounds[len(bounds)-1]}, actual.Period)
				require.Equal(t, expected, actual.Buckets)
			})
		}
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
