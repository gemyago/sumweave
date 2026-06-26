package finance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	return f(request)
}

type failingReadCloser struct{}

func (failingReadCloser) Read(p []byte) (int, error) {
	_ = p
	return 0, errors.New("read failed")
}

func (failingReadCloser) Close() error { return nil }

func TestReportingAndFXInternals(t *testing.T) {
	t.Run("covers fx provider helpers and service error paths", func(t *testing.T) {
		fake := faker.New()
		sentinel := errors.New("sentinel")

		staticProvider := NewStaticFXProvider("static", []domain.FXRate{
			{
				Provider:      "static",
				BaseCurrency:  "USD",
				QuoteCurrency: "PLN",
				RateDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
				Rate:          4.1,
			},
			{
				Provider:      "static",
				BaseCurrency:  "USD",
				QuoteCurrency: "EUR",
				RateDate:      time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC),
				Rate:          0.9,
			},
		})
		rates, err := staticProvider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "usd",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		require.Len(t, rates, 1)
		emptyRates, err := staticProvider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "EUR",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 22, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 23, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		assert.Empty(t, emptyRates)

		_, err = NewNBPFXProvider(nil, "").FetchHistoricalRates(t.Context(), FXProviderQuery{})
		require.ErrorIs(t, err, ErrFXProviderNotImplemented)
		_, err = NewECBFXProvider(nil, "").FetchHistoricalRates(t.Context(), FXProviderQuery{})
		require.ErrorIs(t, err, ErrFXProviderNotImplemented)

		service := NewService(stubStore{})
		_, err = service.TriggerFXSync(t.Context(), TriggerFXSyncParams{})
		require.Error(t, err)
		_, err = service.EnsureFXSyncSchedule(t.Context(), EnsureFXSyncScheduleParams{})
		require.Error(t, err)

		service = NewService(
			stubStore{
				saveFXRatesFn: func(_ context.Context, _ []domain.FXRate) error { return sentinel },
			},
			WithFXProviders(staticProvider),
			WithDefaultFXProvider(staticProvider.Name()),
		)
		_, err = service.SyncFXRates(t.Context(), SyncFXRatesParams{
			BaseCurrencies: []string{"USD"},
			QuoteCurrency:  "PLN",
			StartDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:        time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.ErrorIs(t, err, sentinel)

		service = NewService(stubStore{listFXRatesFn: func(
			_ context.Context,
			_ persistence.ListFXRatesParams,
		) ([]domain.FXRate, error) {
			return nil, sentinel
		}})
		_, err = service.GetFXAdminDiagnostics(t.Context(), FXAdminDiagnosticsParams{})
		require.ErrorIs(t, err, sentinel)

		service = NewService(
			stubStore{},
			WithFXProviders(staticProvider),
			WithDefaultFXProvider(staticProvider.Name()),
		)
		_, err = service.SyncFXRates(t.Context(), SyncFXRatesParams{
			Provider: fmt.Sprintf("missing-%s", fake.Lorem().Word()),
		})
		require.Error(t, err)

		assert.Equal(
			t,
			[]string{"USD", "PLN"},
			canonicalizeCurrencies([]string{"usd", "PLN", "usd"}),
		)
		assert.Equal(
			t,
			time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			startOfDay(time.Date(2026, time.June, 20, 12, 0, 0, 0, time.FixedZone("x", 3600))),
		)
	})

	t.Run("covers frankfurter range and error responses", func(t *testing.T) {
		errorServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, err := w.Write([]byte("boom"))
				assert.NoError(t, err)
			}),
		)
		defer errorServer.Close()

		provider := NewFrankfurterFXProvider(errorServer.Client(), errorServer.URL)
		_, err := provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "USD",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.Error(t, err)

		rangeServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/2026-06-20..2026-06-21", r.URL.Path)
				_, writeErr := w.Write(
					[]byte(
						`{"base":"USD","rates":{"2026-06-20":{"PLN":4.1},"2026-06-21":{"PLN":4.2}}}`,
					),
				)
				assert.NoError(t, writeErr)
			}),
		)
		defer rangeServer.Close()

		provider = NewFrankfurterFXProvider(rangeServer.Client(), rangeServer.URL)
		rates, err := provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "USD",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		require.Len(t, rates, 2)

		provider = NewFrankfurterFXProvider(rangeServer.Client(), "http://[::1")
		_, err = provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "USD",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.Error(t, err)

		provider = NewFrankfurterFXProvider(
			&http.Client{
				Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					_ = request
					return nil, errors.New("network failed")
				}),
			},
			"https://example.com",
		)
		_, err = provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "USD",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.Error(t, err)

		provider = NewFrankfurterFXProvider(
			&http.Client{
				Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					_ = request
					return &http.Response{StatusCode: http.StatusOK, Body: failingReadCloser{}}, nil
				}),
			},
			"https://example.com",
		)
		_, err = provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "USD",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.Error(t, err)

		badSingleServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, writeErr := io.WriteString(
					w,
					`{"base":"USD","date":"bad-date","rates":{"PLN":4.1}}`,
				)
				assert.NoError(t, writeErr)
			}),
		)
		defer badSingleServer.Close()
		provider = NewFrankfurterFXProvider(badSingleServer.Client(), badSingleServer.URL)
		_, err = provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "USD",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.Error(t, err)

		badRangeServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, writeErr := io.WriteString(w, `{`)
				assert.NoError(t, writeErr)
			}),
		)
		defer badRangeServer.Close()
		provider = NewFrankfurterFXProvider(badRangeServer.Client(), badRangeServer.URL)
		_, err = provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "USD",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC),
		})
		require.Error(t, err)
	})

	t.Run("covers dashboard helper branches", func(t *testing.T) {
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		assert.Equal(
			t,
			DashboardPeriodPresetCurrentMonth,
			resolveDashboardPeriod(now, DashboardParams{}).Preset,
		)
		assert.Equal(
			t,
			time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			resolveDashboardPeriod(
				now,
				DashboardParams{Preset: DashboardPeriodPresetLast3Months},
			).StartDate,
		)
		assert.Equal(
			t,
			time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			resolveDashboardPeriod(
				now,
				DashboardParams{Preset: DashboardPeriodPresetLast6Months},
			).StartDate,
		)
		assert.Equal(
			t,
			time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			resolveDashboardPeriod(
				now,
				DashboardParams{Preset: DashboardPeriodPresetThisYear},
			).StartDate,
		)
		assert.Equal(
			t,
			time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			resolveDashboardPeriod(
				now,
				DashboardParams{Preset: DashboardPeriodPresetPreviousYear},
			).StartDate,
		)
		currentMonth := resolveDashboardPeriod(
			now,
			DashboardParams{Preset: DashboardPeriodPresetCurrentMonth},
		)
		assert.Equal(
			t,
			time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC),
			currentMonth.Previous.EndDate,
		)
		assert.Equal(
			t,
			time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC),
			currentMonth.Next.EndDate,
		)
		lastThreeMonths := resolveDashboardPeriod(
			now,
			DashboardParams{Preset: DashboardPeriodPresetLast3Months},
		)
		assert.Equal(
			t,
			time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			lastThreeMonths.Previous.StartDate,
		)
		assert.Equal(
			t,
			time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
			lastThreeMonths.Previous.EndDate,
		)
		assert.Equal(
			t,
			time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
			lastThreeMonths.Next.StartDate,
		)
		assert.Equal(
			t,
			time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC),
			lastThreeMonths.Next.EndDate,
		)
		thisYear := resolveDashboardPeriod(
			now,
			DashboardParams{Preset: DashboardPeriodPresetThisYear},
		)
		assert.Equal(
			t,
			time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			thisYear.Previous.StartDate,
		)
		assert.Equal(
			t,
			time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
			thisYear.Previous.EndDate,
		)
		assert.Equal(
			t,
			time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
			thisYear.Next.StartDate,
		)
		assert.Equal(
			t,
			time.Date(2027, time.December, 31, 0, 0, 0, 0, time.UTC),
			thisYear.Next.EndDate,
		)
		previousStart, previousEnd, nextStart, nextEnd := shiftPeriodWindow(
			time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC),
			time.Date(2026, time.June, 12, 0, 0, 0, 0, time.UTC),
			0,
			0,
		)
		assert.Equal(t, time.Date(2026, time.June, 7, 0, 0, 0, 0, time.UTC), previousStart)
		assert.Equal(t, time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC), nextEnd)
		assert.Equal(t, time.Date(2026, time.June, 9, 0, 0, 0, 0, time.UTC), previousEnd)
		assert.Equal(t, time.Date(2026, time.June, 13, 0, 0, 0, 0, time.UTC), nextStart)

		income, expense, ok := reportingContribution(
			domain.Transaction{Kind: domain.TransactionKindOpeningBalance},
		)
		assert.False(t, ok)
		assert.Zero(t, income)
		assert.Zero(t, expense)
		_, _, ok = reportingContribution(
			domain.Transaction{Kind: domain.TransactionKindReconciliation},
		)
		assert.False(t, ok)
		income, expense, ok = reportingContribution(domain.Transaction{
			Kind:        domain.TransactionKindRefund,
			AmountMinor: 5_00,
		})
		assert.True(t, ok)
		assert.Equal(t, int64(0), income)
		assert.Equal(t, int64(-5_00), expense)
		income, expense, ok = reportingContribution(domain.Transaction{
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -7_00,
		})
		assert.True(t, ok)
		assert.Equal(t, int64(0), income)
		assert.Equal(t, int64(7_00), expense)
		matchedAt := time.Now().UTC()
		_, _, ok = reportingContribution(domain.Transaction{
			Kind:              domain.TransactionKindTransfer,
			AmountMinor:       -7_00,
			Status:            domain.TransactionStatusBooked,
			TransferMatchedAt: &matchedAt,
		})
		assert.False(t, ok)

		lookup := newFXRateLookup([]domain.FXRate{{
			Provider:      FXProviderFrankfurter,
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
			RateDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			Rate:          4.1,
		}})
		_, ok = convertBalanceAmount(10_00, "EUR", "PLN", now, FXProviderFrankfurter, lookup)
		assert.False(t, ok)
		_, ok = convertBalanceAmount(
			10_00,
			"USD",
			"PLN",
			time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC),
			FXProviderFrankfurter,
			lookup,
		)
		assert.False(t, ok)

		categoryBreakdowns := map[string]*DashboardCategoryBreakdown{}
		addDashboardCategoryContribution(
			categoryBreakdowns,
			map[string]domain.Category{},
			domain.Transaction{},
			1,
			2,
		)
		assert.Empty(t, categoryBreakdowns)
		alerts := buildDashboardAlerts([]DashboardMissingFXDiagnostic{{}}, 2)
		require.Len(t, alerts, 2)

		settled := &DashboardMoneySummary{Complete: true}
		pending := &DashboardMoneySummary{Complete: true}
		markIncompleteDashboardSummary(domain.TransactionStatusBooked, settled, pending)
		assert.False(t, settled.Complete)
		markIncompleteDashboardSummary(domain.TransactionStatusPending, settled, pending)
		assert.False(t, pending.Complete)

		categoryItems := sortCategoryBreakdowns(map[string]*DashboardCategoryBreakdown{
			"b": {CategoryID: "b", CategoryName: "B", Kind: domain.CategoryKindExpense},
			"a": {CategoryID: "a", CategoryName: "A", Kind: domain.CategoryKindIncome},
		})
		assert.Equal(t, "a", categoryItems[0].CategoryID)
		currencyItems := sortCurrencyTotals(map[string]*DashboardCurrencyTotal{
			"USD": {Currency: "USD"},
			"EUR": {Currency: "EUR"},
		})
		assert.Equal(t, "EUR", currencyItems[0].Currency)
	})

	t.Run("covers dashboard data load failures", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		service := NewService(
			stubStore{getTenantFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
				return nil, sentinel
			}},
		)
		_, err := service.loadDashboardData(t.Context(), "tenant-1", DashboardParams{})
		require.ErrorIs(t, err, sentinel)

		service = NewService(stubStore{
			getTenantFn: func(_ context.Context, tenantID string) (*domain.Tenant, error) {
				return &domain.Tenant{ID: tenantID, DisplayCurrency: "PLN"}, nil
			},
			listTransactionsFn: func(
				_ context.Context,
				_ string,
				_ string,
				_ domain.TransactionSource,
				_ domain.TransactionStatus,
				_ bool,
			) ([]domain.Transaction, error) {
				return nil, sentinel
			},
		})
		_, err = service.loadDashboardData(t.Context(), "tenant-1", DashboardParams{})
		require.ErrorIs(t, err, sentinel)
	})
}
