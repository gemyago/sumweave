package finance

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
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
	t.Run("keeps dashboard period validation while current FX ignores historical ranges", func(t *testing.T) {
		validDate := time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)
		laterDate := time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC)
		provider := NewStaticFXProvider("static", []domain.FXRate{{
			Provider:      "static",
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
			RateDate:      validDate,
			Rate:          4.1,
		}})

		_, err := provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency: "USD", QuoteCurrencies: []string{"PLN"}, EndDate: validDate,
		})
		require.NoError(t, err)

		_, err = provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency: "USD", QuoteCurrencies: []string{"PLN"}, StartDate: laterDate, EndDate: validDate,
		})
		require.NoError(t, err)

		service := NewService(
			stubStore{isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil }},
			WithFXProviders(provider),
			WithDefaultFXProvider(provider.Name()),
		)
		_, err = service.TriggerFXRefresh(t.Context(), TriggerFXRefreshParams{})
		require.Error(t, err)
		_, err = service.TriggerFXRefresh(t.Context(), TriggerFXRefreshParams{})
		require.Error(t, err)

		_, validationErr := validateProviderFXRates("provider", "USD", "PLN", []domain.FXRate{{
			Provider: "provider", BaseCurrency: "USD", QuoteCurrency: "PLN", Rate: 4.1,
		}})
		require.ErrorIs(t, validationErr, ErrInvalidTimestampRange)

		_, err = service.GetDashboard(t.Context(), DashboardParams{
			ActorUserID: "user",
			TenantID:    "tenant",
			StartDate:   laterDate,
			EndDate:     validDate,
		})
		require.Error(t, err)
	})

	t.Run("validates required timestamp range endpoints", func(t *testing.T) {
		timestamp := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)
		require.ErrorIs(t, ValidateRequiredTimestampRange(time.Time{}, timestamp), ErrInvalidTimestampRange)
		require.ErrorIs(t, ValidateRequiredTimestampRange(timestamp, time.Time{}), ErrInvalidTimestampRange)
		require.ErrorIs(
			t,
			ValidateRequiredTimestampRange(timestamp, timestamp.Add(-time.Nanosecond)),
			ErrInvalidTimestampRange,
		)
		require.NoError(t, ValidateRequiredTimestampRange(timestamp, timestamp))
	})

	t.Run("covers fx provider helpers and service error paths", func(t *testing.T) {
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

		providerQuery := FXProviderQuery{
			StartDate: time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		}
		_, err = NewNBPFXProvider(nil, "").FetchHistoricalRates(t.Context(), providerQuery)
		require.ErrorIs(t, err, ErrFXProviderNotImplemented)
		_, err = NewECBFXProvider(nil, "").FetchHistoricalRates(t.Context(), providerQuery)
		require.ErrorIs(t, err, ErrFXProviderNotImplemented)

		service := NewService(stubStore{})
		_, err = service.TriggerFXRefresh(t.Context(), TriggerFXRefreshParams{})
		require.Error(t, err)
		service = NewService(stubStore{listCurrentFXRatesFn: func(
			_ context.Context,
			_ persistence.ListCurrentFXRatesParams,
		) ([]domain.FXRate, error) {
			return nil, sentinel
		}})
		_, err = service.GetFXAdminDiagnostics(t.Context(), FXAdminDiagnosticsParams{})
		require.ErrorIs(t, err, sentinel)

		assert.Equal(
			t,
			[]string{"USD", "PLN"},
			canonicalizeCurrencies([]string{"usd", "PLN", "usd"}),
		)
	})

	t.Run("groups missing valuations by provider and currency pair", func(t *testing.T) {
		missingUSDTransaction := DashboardMissingFXDiagnostic{
			Source:        DashboardMissingFXSourceTransaction,
			Provider:      "provider-a",
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
		}
		missingUSDBalance := DashboardMissingFXDiagnostic{
			Source:        DashboardMissingFXSourceBalance,
			Provider:      "provider-a",
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
		}
		missingEURTransaction := DashboardMissingFXDiagnostic{
			Source:        DashboardMissingFXSourceTransaction,
			Provider:      "provider-a",
			BaseCurrency:  "EUR",
			QuoteCurrency: "PLN",
		}
		coverage := buildDashboardFXCoverage([]DashboardMissingFXDiagnostic{
			missingUSDTransaction,
			missingUSDTransaction,
			missingUSDTransaction,
			missingUSDBalance,
			missingUSDBalance,
			missingEURTransaction,
		})
		assert.Equal(t, []DashboardFXCoverage{
			{
				Provider:                 "provider-a",
				BaseCurrency:             "EUR",
				QuoteCurrency:            "PLN",
				AffectedTransactionCount: 1,
			},
			{
				Provider:                 "provider-a",
				BaseCurrency:             "USD",
				QuoteCurrency:            "PLN",
				AffectedTransactionCount: 3,
				AffectedAccountCount:     2,
			},
		}, coverage)
	})

	t.Run("covers frankfurter latest-rate and error responses", func(t *testing.T) {
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

		latestServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v2/rates", r.URL.Path)
				_, writeErr := w.Write(
					[]byte(`[{"base":"USD","date":"2026-06-21","quote":"PLN","rate":4.2}]`),
				)
				assert.NoError(t, writeErr)
			}),
		)
		defer latestServer.Close()

		provider = NewFrankfurterFXProvider(latestServer.Client(), latestServer.URL)
		rates, err := provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
			BaseCurrency:    "USD",
			QuoteCurrencies: []string{"PLN"},
			StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		require.Len(t, rates, 1)

		provider = NewFrankfurterFXProvider(latestServer.Client(), "http://[::1")
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

	t.Run("uses the requested dashboard range without server-side period derivation", func(t *testing.T) {
		startDate := time.Date(2026, time.June, 10, 0, 0, 0, 0, time.FixedZone("UTC+02", 2*60*60))
		endDate := time.Date(2026, time.June, 12, 23, 59, 59, 999999999, time.FixedZone("UTC+02", 2*60*60))
		require.NoError(t, ValidateDashboardParams(DashboardParams{StartDate: startDate, EndDate: endDate}))
		require.ErrorIs(
			t,
			ValidateDashboardParams(DashboardParams{StartDate: endDate, EndDate: startDate}),
			ErrInvalidTimestampRange,
		)
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
		_, ok = convertBalanceAmount(10_00, "EUR", "PLN", FXProviderFrankfurter, lookup)
		assert.False(t, ok)
		_, ok = convertBalanceAmount(10_00, "USD", "PLN", FXProviderFrankfurter, lookup)
		assert.True(t, ok)

		categoryBreakdowns := map[string]*DashboardCategoryBreakdown{}
		addDashboardCategoryContribution(
			categoryBreakdowns,
			map[string]domain.Category{},
			domain.Transaction{},
			1,
			2,
		)
		assert.Empty(t, categoryBreakdowns)
		alerts := buildDashboardAlerts([]DashboardFXCoverage{{}}, 2)
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
