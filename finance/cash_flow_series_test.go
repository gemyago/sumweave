package finance

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCashFlowSeriesContracts(t *testing.T) {
	makeParams := func(fake faker.Faker) CashFlowSeriesParams {
		start := time.Date(2026, time.July, fake.IntBetween(1, 10), 9, 30, 0, 0, time.FixedZone("series", 2*60*60))
		return CashFlowSeriesParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			StartDate:   start,
			EndDate:     start.AddDate(0, 0, 1),
			GroupBy:     CashFlowGroupByDay,
		}
	}
	makeTenant := func(fake faker.Faker, params CashFlowSeriesParams) domain.Tenant {
		return domain.Tenant{
			ID: params.TenantID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "EUR",
			CreatedAt: params.StartDate, UpdatedAt: params.StartDate,
		}
	}

	t.Run("validates required increasing ranges and supported bounded groups", func(t *testing.T) {
		fake := faker.New()
		params := makeParams(fake)
		require.NoError(t, ValidateCashFlowSeriesParams(params))

		params.StartDate = time.Time{}
		require.ErrorIs(t, ValidateCashFlowSeriesParams(params), ErrInvalidTimestampRange)

		params = makeParams(fake)
		params.EndDate = params.StartDate
		require.ErrorIs(t, ValidateCashFlowSeriesParams(params), ErrInvalidTimestampRange)

		params = makeParams(fake)
		params.GroupBy = CashFlowGroupBy("unsupported-" + fake.Lorem().Word())
		require.Error(t, ValidateCashFlowSeriesParams(params))

		params = makeParams(fake)
		params.EndDate = params.StartDate.AddDate(0, 0, 367)
		require.Error(t, ValidateCashFlowSeriesParams(params))

		monthStart := time.Date(2026, time.January, 31, 9, 30, 0, 0, time.FixedZone("monthly", 2*60*60))
		assert.Equal(t, 3, cashFlowBucketCount(monthStart, monthStart.AddDate(0, 2, 1), CashFlowGroupByMonth))

		originalAnchorStart := time.Date(2024, time.January, 31, 9, 30, 0, 0, time.FixedZone("monthly-anchor", 2*60*60))
		assert.Equal(t, 2, cashFlowBucketCount(
			originalAnchorStart,
			time.Date(2024, time.March, 30, 9, 30, 0, 0, originalAnchorStart.Location()),
			CashFlowGroupByMonth,
		))
		params = makeParams(fake)
		params.StartDate = originalAnchorStart
		params.EndDate = time.Date(2054, time.July, 31, 9, 30, 0, 0, originalAnchorStart.Location())
		params.GroupBy = CashFlowGroupByMonth
		require.NoError(t, ValidateCashFlowSeriesParams(params))
		params.EndDate = params.EndDate.Add(time.Nanosecond)
		require.Error(t, ValidateCashFlowSeriesParams(params))
	})

	cashFlowRegressionName := "matches PostgreSQL converted buckets with dashboard settled totals at reviewed FX boundaries"
	t.Run(cashFlowRegressionName, func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		provider := "provider-" + fake.UUID().V4()
		service := NewService(store, WithDefaultFXProvider(provider))
		reporting := NewReportingService(
			store,
			persistence.NewCashFlowSeriesStore(database),
			WithReportingServiceDefaultFXProvider(provider),
		)
		ownerID := "owner-" + fake.UUID().V4()
		start := time.Date(2026, time.July, 14, 9, 30, 0, 0, time.FixedZone("series", 2*60*60))
		end := start.AddDate(0, 0, 1)

		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "EUR",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		usdAccount, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerID,
			TenantID:    tenant.ID,
			Name:        "usd-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)
		gbpAccount, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerID,
			TenantID:    tenant.ID,
			Name:        "gbp-" + fake.Lorem().Word(),
			Currency:    "GBP",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)
		require.NoError(t, store.SaveCurrentFXRates(t.Context(), []domain.FXRate{
			{
				Provider: provider, BaseCurrency: "USD", QuoteCurrency: "EUR", Rate: 1.5,
				EffectiveAt: start, LastSuccessfulRefreshAt: start,
			},
			{
				Provider: provider, BaseCurrency: "GBP", QuoteCurrency: "EUR", Rate: 0.29,
				EffectiveAt: start, LastSuccessfulRefreshAt: start,
			},
		}))
		for _, transaction := range []RecordTransactionParams{
			{
				ActorUserID: ownerID, TenantID: tenant.ID, AccountID: usdAccount.ID,
				Source: domain.TransactionSourceManual, Status: domain.TransactionStatusBooked,
				Kind: domain.TransactionKindIncome, AmountMinor: 99, Currency: "USD",
				Description: "income-" + fake.Lorem().Word(), EffectiveAt: start.Add(time.Minute),
			},
			{
				ActorUserID: ownerID, TenantID: tenant.ID, AccountID: gbpAccount.ID,
				Source: domain.TransactionSourceManual, Status: domain.TransactionStatusBooked,
				Kind: domain.TransactionKindExpense, AmountMinor: -50, Currency: "GBP",
				Description: "expense-" + fake.Lorem().Word(), EffectiveAt: start.Add(2 * time.Minute),
			},
		} {
			_, recordErr := service.RecordTransaction(t.Context(), transaction)
			require.NoError(t, recordErr)
		}

		series, err := reporting.GetCashFlowSeries(t.Context(), CashFlowSeriesParams{
			ActorUserID: ownerID, TenantID: tenant.ID, StartDate: start, EndDate: end, GroupBy: CashFlowGroupByDay,
		})
		require.NoError(t, err)
		dashboard, err := reporting.GetDashboard(t.Context(), DashboardParams{
			ActorUserID: ownerID, TenantID: tenant.ID, StartDate: start, EndDate: end,
		})
		require.NoError(t, err)
		require.Len(t, series.Buckets, 1)
		assert.Equal(t, int64(149), series.Buckets[0].IncomeMinor)
		assert.Equal(t, int64(14), series.Buckets[0].ExpenseMinor)
		assert.Equal(t, dashboard.Settled.IncomeMinor, series.Buckets[0].IncomeMinor)
		assert.Equal(t, dashboard.Settled.ExpenseMinor, series.Buckets[0].ExpenseMinor)
	})

	t.Run("matches PostgreSQL monthly buckets and cap validation in the submitted offset calendar", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		seriesStore := persistence.NewCashFlowSeriesStore(database)
		start := time.Date(2024, time.January, 31, 0, 30, 0, 0, time.FixedZone("submitted", 2*60*60))
		acceptedEnd := cashFlowMonthBoundary(start, maxCashFlowBuckets)
		params := makeParams(fake)
		params.StartDate = start
		tenant := makeTenant(fake, params)
		_, err := store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)

		acceptedParams := CashFlowSeriesParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    tenant.ID,
			StartDate:   start,
			EndDate:     acceptedEnd,
			GroupBy:     CashFlowGroupByMonth,
		}
		require.Equal(t, maxCashFlowBuckets, cashFlowBucketCount(start, acceptedEnd, CashFlowGroupByMonth))
		require.NoError(t, ValidateCashFlowSeriesParams(acceptedParams))
		acceptedSeries, err := seriesStore.GetCashFlowSeries(t.Context(), persistence.CashFlowSeriesParams{
			TenantID: tenant.ID, StartDate: start, EndDate: acceptedEnd,
			GroupBy: CashFlowGroupByMonth, FXProvider: "provider-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		require.Len(t, acceptedSeries.Buckets, maxCashFlowBuckets)
		for index, bucket := range acceptedSeries.Buckets {
			require.True(t, cashFlowMonthBoundary(start, index).Equal(bucket.StartDate), "bucket %d", index)
		}

		overCapEnd := acceptedEnd.Add(time.Microsecond)
		overCapParams := acceptedParams
		overCapParams.EndDate = overCapEnd
		require.Equal(t, maxCashFlowBuckets+1, cashFlowBucketCount(start, overCapEnd, CashFlowGroupByMonth))
		require.Error(t, ValidateCashFlowSeriesParams(overCapParams))
		overCapSeries, err := seriesStore.GetCashFlowSeries(t.Context(), persistence.CashFlowSeriesParams{
			TenantID: tenant.ID, StartDate: start, EndDate: overCapEnd,
			GroupBy: CashFlowGroupByMonth, FXProvider: "provider-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		require.Len(t, overCapSeries.Buckets, maxCashFlowBuckets+1)
	})

	t.Run("validates named monthly calendars and DST bucket limits", func(t *testing.T) {
		fake := faker.New()
		params := makeParams(fake)
		params.GroupBy = CashFlowGroupByMonth
		params.TimeZone = "Europe/Warsaw"
		location, err := time.LoadLocation(params.TimeZone)
		require.NoError(t, err)
		// Calendar/DST boundaries are fixed regression inputs, not sample entity data.
		start := time.Date(2026, time.May, 1, 0, 0, 0, 0, location)
		end := time.Date(2026, time.November, 1, 0, 0, 0, 0, location)
		params.StartDate = start.In(time.FixedZone("wire", 0))
		params.EndDate = end.In(time.FixedZone("wire", 0))
		require.NoError(t, ValidateCashFlowSeriesParams(params))
		require.Equal(t, 6, cashFlowBucketCount(start, end, CashFlowGroupByMonth))
		params.EndDate = time.Date(2056, time.November, 1, 0, 0, 0, 0, location)
		require.NoError(t, ValidateCashFlowSeriesParams(params))
		params.EndDate = params.EndDate.Add(time.Microsecond)
		require.Error(t, ValidateCashFlowSeriesParams(params))
		params.EndDate = end
		for _, invalid := range []string{"Local", "invalid/" + fake.UUID().V4()} {
			params.TimeZone = invalid
			require.Error(t, ValidateCashFlowSeriesParams(params))
		}
		params.TimeZone = location.String()
		params.GroupBy = CashFlowGroupByDay
		require.Error(t, ValidateCashFlowSeriesParams(params))
	})

	t.Run("matches persistence at ambiguous and missing named-calendar cap anchors", func(t *testing.T) {
		for _, tc := range []struct {
			name        string
			start       string
			end         string
			capBoundary string
		}{
			{"LA reviewed fold cap", "2026-05-05T01:30:00-07:00", "2056-11-05T01:15:00-08:00", "2056-11-05T01:30:00-08:00"},
			{"LA missing cap anchor", "2025-09-12T02:30:00-07:00", "2056-03-12T03:15:00-07:00", "2056-03-12T03:30:00-07:00"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				fake := faker.New()
				location, err := time.LoadLocation("America/Los_Angeles")
				require.NoError(t, err)
				parse := func(value string) time.Time {
					at, parseErr := time.Parse(time.RFC3339, value)
					require.NoError(t, parseErr)
					return at.In(location)
				}
				params := makeParams(fake)
				params.StartDate, params.EndDate = parse(tc.start), parse(tc.end)
				params.GroupBy, params.TimeZone = CashFlowGroupByMonth, location.String()
				capBoundary := parse(tc.capBoundary)
				require.True(t, capBoundary.Equal(cashFlowMonthBoundary(params.StartDate, maxCashFlowBuckets)))
				database := openTestDatabase(t)
				store := persistence.NewStore(database)
				_, err = store.SaveTenant(t.Context(), makeTenant(fake, params))
				require.NoError(t, err)
				seriesStore := persistence.NewCashFlowSeriesStore(database)
				for _, end := range []time.Time{params.EndDate, capBoundary, capBoundary.Add(time.Microsecond)} {
					params.EndDate = end
					count := maxCashFlowBuckets
					if end.After(capBoundary) {
						count++
						require.Error(t, ValidateCashFlowSeriesParams(params))
					} else {
						require.NoError(t, ValidateCashFlowSeriesParams(params))
					}
					actual, queryErr := seriesStore.GetCashFlowSeries(t.Context(), persistence.CashFlowSeriesParams{
						TenantID: params.TenantID, StartDate: params.StartDate, EndDate: end,
						GroupBy: params.GroupBy, TimeZone: params.TimeZone, FXProvider: "provider-" + fake.UUID().V4(),
					})
					require.NoError(t, queryErr)
					require.Len(t, actual.Buckets, count)
					boundary := params.StartDate
					for _, bucket := range actual.Buckets {
						require.True(t, boundary.Equal(bucket.StartDate))
						require.True(t, bucket.StartDate.Before(bucket.EndDate))
						boundary = bucket.EndDate
					}
					require.True(t, end.Equal(boundary))
				}
			})
		}
	})

	t.Run("authorizes the tenant before delegating a valid request", func(t *testing.T) {
		fake := faker.New()
		params := makeParams(fake)
		params.GroupBy = CashFlowGroupByMonth
		params.TimeZone = "Europe/Warsaw"
		access := newMockreportingServiceStore(t)
		cashFlows := newMockcashFlowSeriesStore(t)
		access.EXPECT().IsTenantMember(mock.Anything, params.TenantID, params.ActorUserID).Return(true, nil).Once()
		expected := domain.CashFlowSeries{Complete: true}
		cashFlows.EXPECT().GetCashFlowSeries(mock.Anything, persistence.CashFlowSeriesParams{
			TenantID: params.TenantID, StartDate: params.StartDate, EndDate: params.EndDate,
			GroupBy: CashFlowGroupByMonth, FXProvider: FXProviderFrankfurter, TimeZone: params.TimeZone,
		}).Return(expected, nil).Once()

		actual, err := NewReportingService(
			access,
			cashFlows,
			WithReportingServiceAccountBalanceStore(newMockaccountBalanceReadStore(t)),
		).GetCashFlowSeries(t.Context(), params)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("rejects a non-member without querying the series store", func(t *testing.T) {
		fake := faker.New()
		params := makeParams(fake)
		access := newMockreportingServiceStore(t)
		access.EXPECT().IsTenantMember(mock.Anything, params.TenantID, params.ActorUserID).Return(false, nil).Once()

		_, err := NewReportingService(
			access,
			newMockcashFlowSeriesStore(t),
			WithReportingServiceAccountBalanceStore(newMockaccountBalanceReadStore(t)),
		).GetCashFlowSeries(t.Context(), params)
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})
}
