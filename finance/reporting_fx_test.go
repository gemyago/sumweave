package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type capturedFXSyncJobEnqueuer struct {
	request *FXSyncJobRequest
	jobRef  FXSyncJobRef
}

func (e *capturedFXSyncJobEnqueuer) EnqueueFXSync(
	_ context.Context,
	request FXSyncJobRequest,
) (FXSyncJobRef, error) {
	e.request = &request
	if e.jobRef.JobType == "" {
		e.jobRef = FXSyncJobRef{ID: "job-id", JobType: request.JobType}
	}
	return e.jobRef, nil
}

type capturedFXSyncScheduleWriter struct {
	schedule FXSyncSchedule
}

func (s *capturedFXSyncScheduleWriter) UpsertFXSyncSchedule(
	_ context.Context,
	schedule FXSyncSchedule,
) error {
	s.schedule = schedule
	return nil
}

func TestReportingAndFX(t *testing.T) {
	makeStore := func(t *testing.T) *persistence.Store {
		t.Helper()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		return store
	}

	t.Run(
		"dashboard reporting resolves periods and computes display currency summaries",
		func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
			idSequence := 0
			service := NewService(
				store,
				WithNow(func() time.Time { return now }),
				WithIDGenerator(func() string {
					idSequence++
					return fmt.Sprintf("id-%02d-%s", idSequence, fake.Lorem().Word())
				}),
			)

			ownerUserID := fmt.Sprintf("user-owner-%s", fake.Lorem().Word())
			tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
				ActorUserID:     ownerUserID,
				Name:            fmt.Sprintf("tenant-%s", fake.Company().Name()),
				DisplayCurrency: "PLN",
			})
			require.NoError(t, err)

			usdAccount, err := service.CreateAccount(t.Context(), CreateAccountParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Name:        fmt.Sprintf("usd-%s", fake.Lorem().Word()),
				Currency:    "USD",
				Kind:        domain.AccountKindManual,
			})
			require.NoError(t, err)
			plnAccount, err := service.CreateAccount(t.Context(), CreateAccountParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Name:        fmt.Sprintf("pln-%s", fake.Lorem().Word()),
				Currency:    "PLN",
				Kind:        domain.AccountKindManual,
			})
			require.NoError(t, err)
			eurAccount, err := service.CreateAccount(t.Context(), CreateAccountParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Name:        fmt.Sprintf("eur-%s", fake.Lorem().Word()),
				Currency:    "EUR",
				Kind:        domain.AccountKindManual,
			})
			require.NoError(t, err)
			gbpAccount, err := service.CreateAccount(t.Context(), CreateAccountParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Name:        fmt.Sprintf("gbp-%s", fake.Lorem().Word()),
				Currency:    "GBP",
				Kind:        domain.AccountKindManual,
			})
			require.NoError(t, err)

			salaryCategory, err := service.CreateCategory(t.Context(), CreateCategoryParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Name:        fmt.Sprintf("salary-%s", fake.Lorem().Word()),
				Kind:        domain.CategoryKindIncome,
			})
			require.NoError(t, err)
			groceriesCategory, err := service.CreateCategory(t.Context(), CreateCategoryParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Name:        fmt.Sprintf("groceries-%s", fake.Lorem().Word()),
				Kind:        domain.CategoryKindExpense,
			})
			require.NoError(t, err)

			require.NoError(t, store.SaveFXRates(t.Context(), []domain.FXRate{
				{
					Provider:      FXProviderFrankfurter,
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					RateDate:      time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC),
					Rate:          3.9,
				},
				{
					Provider:      FXProviderFrankfurter,
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					RateDate:      time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC),
					Rate:          4.0,
				},
				{
					Provider:      FXProviderFrankfurter,
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					RateDate:      time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC),
					Rate:          4.0,
				},
				{
					Provider:      FXProviderFrankfurter,
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					RateDate:      time.Date(2026, time.June, 4, 0, 0, 0, 0, time.UTC),
					Rate:          4.0,
				},
				{
					Provider:      FXProviderFrankfurter,
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					RateDate:      time.Date(2026, time.June, 5, 0, 0, 0, 0, time.UTC),
					Rate:          4.1,
				},
				{
					Provider:      FXProviderFrankfurter,
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					RateDate:      time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC),
					Rate:          4.2,
				},
			}))

			record := func(accountID string, categoryID string, status domain.TransactionStatus, kind domain.TransactionKind, amount int64, currency string, effectiveAt time.Time) domain.Transaction {
				t.Helper()

				transaction, recordErr := service.RecordTransaction(
					t.Context(),
					RecordTransactionParams{
						ActorUserID: ownerUserID,
						TenantID:    tenant.ID,
						AccountID:   accountID,
						Source:      domain.TransactionSourceManual,
						Status:      status,
						Kind:        kind,
						AmountMinor: amount,
						Currency:    currency,
						Description: fmt.Sprintf("transaction-%s", fake.Lorem().Word()),
						EffectiveAt: effectiveAt,
						CategoryID:  categoryID,
					},
				)
				require.NoError(t, recordErr)
				return transaction
			}

			record(
				gbpAccount.ID,
				salaryCategory.ID,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				30_00,
				"GBP",
				time.Date(2026, time.May, 15, 10, 0, 0, 0, time.UTC),
			)
			record(
				usdAccount.ID,
				salaryCategory.ID,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				100_00,
				"USD",
				time.Date(2026, time.May, 10, 10, 0, 0, 0, time.UTC),
			)
			record(
				usdAccount.ID,
				salaryCategory.ID,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				100_00,
				"USD",
				time.Date(2026, time.June, 2, 10, 0, 0, 0, time.UTC),
			)
			record(
				usdAccount.ID,
				groceriesCategory.ID,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-40_00,
				"USD",
				time.Date(2026, time.June, 3, 10, 0, 0, 0, time.UTC),
			)
			record(
				usdAccount.ID,
				groceriesCategory.ID,
				domain.TransactionStatusBooked,
				domain.TransactionKindRefund,
				10_00,
				"USD",
				time.Date(2026, time.June, 4, 10, 0, 0, 0, time.UTC),
			)
			record(
				usdAccount.ID,
				groceriesCategory.ID,
				domain.TransactionStatusPending,
				domain.TransactionKindRegular,
				-20_00,
				"USD",
				time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC),
			)
			usdTransferOut := record(
				usdAccount.ID,
				"",
				domain.TransactionStatusBooked,
				domain.TransactionKindTransfer,
				-15_00,
				"USD",
				time.Date(2026, time.June, 6, 10, 0, 0, 0, time.UTC),
			)
			plnTransferIn := record(
				plnAccount.ID,
				"",
				domain.TransactionStatusBooked,
				domain.TransactionKindTransfer,
				15_00,
				"PLN",
				time.Date(2026, time.June, 6, 10, 5, 0, 0, time.UTC),
			)
			require.NoError(t, service.LinkTransfers(t.Context(), LinkTransfersParams{
				ActorUserID:         ownerUserID,
				TenantID:            tenant.ID,
				FirstTransactionID:  usdTransferOut.ID,
				SecondTransactionID: plnTransferIn.ID,
			}))
			record(
				usdAccount.ID,
				"",
				domain.TransactionStatusBooked,
				domain.TransactionKindReconciliation,
				5_00,
				"USD",
				time.Date(2026, time.June, 7, 10, 0, 0, 0, time.UTC),
			)
			record(
				plnAccount.ID,
				groceriesCategory.ID,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-50_00,
				"PLN",
				time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC),
			)
			missingFXTransaction := record(
				eurAccount.ID,
				groceriesCategory.ID,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-70_00,
				"EUR",
				time.Date(2026, time.June, 9, 10, 0, 0, 0, time.UTC),
			)
			record(
				usdAccount.ID,
				salaryCategory.ID,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				200_00,
				"USD",
				time.Date(2026, time.July, 2, 10, 0, 0, 0, time.UTC),
			)

			dashboard, err := service.GetDashboard(t.Context(), DashboardParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)

			assert.Equal(t, DashboardPeriodPresetCurrentMonth, dashboard.Period.Preset)
			assert.Equal(
				t,
				time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
				dashboard.Period.StartDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.June, 30, 23, 59, 59, 999999999, time.UTC),
				dashboard.Period.EndDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
				dashboard.Period.Previous.StartDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.May, 31, 23, 59, 59, 999999999, time.UTC),
				dashboard.Period.Previous.EndDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
				dashboard.Period.Next.StartDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.July, 31, 23, 59, 59, 999999999, time.UTC),
				dashboard.Period.Next.EndDate,
			)

			assert.Equal(t, "PLN", dashboard.Settled.DisplayCurrency)
			assert.Equal(t, int64(420_00), dashboard.Settled.IncomeMinor)
			assert.Equal(t, int64(176_00), dashboard.Settled.ExpenseMinor)
			assert.Equal(t, int64(244_00), dashboard.Settled.NetMinor)
			assert.False(t, dashboard.Settled.Complete)

			assert.Equal(t, int64(84_00), dashboard.Pending.ExpenseMinor)
			assert.Equal(t, int64(-84_00), dashboard.Pending.NetMinor)
			assert.Equal(t, 1, dashboard.Pending.TransactionCount)

			require.Len(t, dashboard.CategoryBreakdowns, 2)
			assert.Equal(t, salaryCategory.ID, dashboard.CategoryBreakdowns[0].CategoryID)
			assert.Equal(t, int64(420_00), dashboard.CategoryBreakdowns[0].IncomeMinor)
			assert.Equal(t, groceriesCategory.ID, dashboard.CategoryBreakdowns[1].CategoryID)
			assert.Equal(t, int64(176_00), dashboard.CategoryBreakdowns[1].ExpenseMinor)

			require.Len(t, dashboard.AccountBalances, 4)
			assert.Equal(t, usdAccount.ID, dashboard.AccountBalances[0].AccountID)
			assert.Equal(t, int64(160_00), dashboard.AccountBalances[0].NativeBookedMinor)
			assert.Equal(t, int64(-20_00), dashboard.AccountBalances[0].NativePendingMinor)
			require.NotNil(t, dashboard.AccountBalances[0].DisplayBookedMinor)
			require.NotNil(t, dashboard.AccountBalances[0].DisplayPendingMinor)
			assert.Equal(t, int64(672_00), *dashboard.AccountBalances[0].DisplayBookedMinor)
			assert.Equal(t, int64(-84_00), *dashboard.AccountBalances[0].DisplayPendingMinor)
			assert.Equal(t, eurAccount.ID, dashboard.AccountBalances[2].AccountID)
			assert.True(t, dashboard.AccountBalances[2].MissingFX)
			assert.Nil(t, dashboard.AccountBalances[2].DisplayBookedMinor)
			assert.Equal(t, gbpAccount.ID, dashboard.AccountBalances[3].AccountID)
			assert.Equal(t, int64(30_00), dashboard.AccountBalances[3].NativeBookedMinor)
			assert.True(t, dashboard.AccountBalances[3].MissingFX)
			assert.Nil(t, dashboard.AccountBalances[3].DisplayBookedMinor)

			assert.ElementsMatch(
				t,
				[]string{"missing_fx_rates", "pending_transactions"},
				[]string{dashboard.Alerts[0].Code, dashboard.Alerts[1].Code},
			)
			assert.Equal(t, 3, dashboard.Alerts[0].Count)
			require.Len(t, dashboard.MissingFX, 3)
			assert.ElementsMatch(t, []DashboardMissingFXDiagnostic{
				{
					Source:        DashboardMissingFXSourceTransaction,
					TransactionID: missingFXTransaction.ID,
					AccountID:     eurAccount.ID,
					BaseCurrency:  "EUR",
					QuoteCurrency: "PLN",
					RateDate:      missingFXTransaction.EffectiveAt,
					Provider:      FXProviderFrankfurter,
				},
				{
					Source:        DashboardMissingFXSourceBalance,
					TransactionID: "",
					AccountID:     eurAccount.ID,
					BaseCurrency:  "EUR",
					QuoteCurrency: "PLN",
					RateDate:      dashboard.Period.EndDate,
					Provider:      FXProviderFrankfurter,
				},
				{
					Source:        DashboardMissingFXSourceBalance,
					TransactionID: "",
					AccountID:     gbpAccount.ID,
					BaseCurrency:  "GBP",
					QuoteCurrency: "PLN",
					RateDate:      dashboard.Period.EndDate,
					Provider:      FXProviderFrankfurter,
				},
			}, dashboard.MissingFX)

			assert.ElementsMatch(t, []DashboardCurrencyTotal{
				{Currency: "USD", IncomeMinor: 100_00, ExpenseMinor: 30_00, NetMinor: 70_00},
				{Currency: "PLN", IncomeMinor: 0, ExpenseMinor: 50_00, NetMinor: -50_00},
				{Currency: "EUR", IncomeMinor: 0, ExpenseMinor: 70_00, NetMinor: -70_00},
			}, dashboard.NativeSettledTotals)

			previousMonth, err := service.GetDashboard(t.Context(), DashboardParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Preset:      DashboardPeriodPresetPreviousMonth,
			})
			require.NoError(t, err)
			assert.Equal(
				t,
				time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
				previousMonth.Period.StartDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.May, 31, 23, 59, 59, 999999999, time.UTC),
				previousMonth.Period.EndDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
				previousMonth.Period.Previous.StartDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.April, 30, 23, 59, 59, 999999999, time.UTC),
				previousMonth.Period.Previous.EndDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
				previousMonth.Period.Next.StartDate,
			)
			assert.Equal(
				t,
				time.Date(2026, time.June, 30, 23, 59, 59, 999999999, time.UTC),
				previousMonth.Period.Next.EndDate,
			)
			assert.Equal(t, int64(420_00), previousMonth.Settled.IncomeMinor)
			assert.Equal(t, int64(420_00), previousMonth.Settled.NetMinor)

			customRange, err := service.GetDashboard(t.Context(), DashboardParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Preset:      DashboardPeriodPresetCustom,
				StartDate:   time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC),
				EndDate:     time.Date(2026, time.June, 4, 0, 0, 0, 0, time.UTC),
			})
			require.NoError(t, err)
			assert.Equal(t, int64(420_00), customRange.Settled.IncomeMinor)
			assert.Equal(t, int64(168_00), customRange.Settled.ExpenseMinor)
			assert.Equal(t, int64(252_00), customRange.Settled.NetMinor)
			assert.Equal(t, int64(160_00), customRange.AccountBalances[0].NativeBookedMinor)
			assert.Equal(t, int64(0), customRange.AccountBalances[0].NativePendingMinor)

			aggregateStore := persistence.NewAccountBalanceStoreFromStore(store)
			aggregateBalances, err := aggregateStore.ListAccountBalances(
				t.Context(),
				persistence.ListAccountBalancesParams{
					TenantID:              tenant.ID,
					AccountIDs:            []string{usdAccount.ID, plnAccount.ID, eurAccount.ID, gbpAccount.ID},
					EffectiveAtOnOrBefore: &dashboard.Period.EndDate,
				},
			)
			require.NoError(t, err)

			aggregateByAccountID := make(map[string]domain.AccountBalance, len(aggregateBalances))
			for _, balance := range aggregateBalances {
				aggregateByAccountID[balance.AccountID] = balance
			}
			for _, balance := range dashboard.AccountBalances {
				assert.Equal(t, aggregateByAccountID[balance.AccountID].BookedBalanceMinor, balance.NativeBookedMinor)
				assert.Equal(t, aggregateByAccountID[balance.AccountID].PendingBalanceMinor, balance.NativePendingMinor)
			}
		},
	)

	t.Run(
		"syncs persisted fx rates and exposes generic job seams and safe diagnostics",
		func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)

			provider := NewStaticFXProvider(
				"static-provider",
				[]domain.FXRate{{
					Provider:      "static-provider",
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					RateDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
					Rate:          4.12,
				}},
			)
			enqueuer := &capturedFXSyncJobEnqueuer{jobRef: FXSyncJobRef{
				ID:      fmt.Sprintf("job-%s", fake.Lorem().Word()),
				JobType: FXSyncJobType,
			}}
			scheduler := &capturedFXSyncScheduleWriter{}
			service := NewService(
				store,
				WithNow(func() time.Time { return now }),
				WithFXProviders(provider, NewNBPFXProvider(nil, ""), NewECBFXProvider(nil, "")),
				WithDefaultFXProvider(provider.Name()),
				WithFXJobEnqueuer(enqueuer),
				WithFXScheduleWriter(scheduler),
			)

			syncResult, err := service.SyncFXRates(t.Context(), SyncFXRatesParams{
				Provider:       provider.Name(),
				BaseCurrencies: []string{"USD"},
				QuoteCurrency:  "PLN",
				StartDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
				EndDate:        time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			})
			require.NoError(t, err)
			assert.Equal(t, provider.Name(), syncResult.Provider)
			assert.Equal(t, 1, syncResult.ImportedCount)

			storedRates, err := store.ListFXRates(t.Context(), persistence.ListFXRatesParams{
				Provider:      provider.Name(),
				BaseCurrency:  "USD",
				QuoteCurrency: "PLN",
			})
			require.NoError(t, err)
			require.Len(t, storedRates, 1)
			assert.InDelta(t, 4.12, storedRates[0].Rate, 0.00001)

			jobRef, err := service.TriggerFXSync(t.Context(), TriggerFXSyncParams{
				RequestedByUserID: fmt.Sprintf("admin-%s", fake.Lorem().Word()),
				Source:            FXSyncRequesterSourceOperator,
				Provider:          provider.Name(),
				BaseCurrencies:    []string{"USD", "EUR"},
				QuoteCurrency:     "PLN",
				StartDate:         time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
				EndDate:           time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			})
			require.NoError(t, err)
			assert.Equal(t, FXSyncJobType, jobRef.JobType)
			require.NotNil(t, enqueuer.request)
			assert.Equal(t, FXSyncJobType, enqueuer.request.JobType)
			assert.Equal(t, provider.Name(), enqueuer.request.Input.Provider)

			schedule, err := service.EnsureFXSyncSchedule(t.Context(), EnsureFXSyncScheduleParams{
				ScheduleID:      fmt.Sprintf("fx-daily-%s", fake.Lorem().Word()),
				Provider:        provider.Name(),
				BaseCurrencies:  []string{"USD", "EUR"},
				QuoteCurrency:   "PLN",
				Interval:        24 * time.Hour,
				RequestedByUser: "system",
			})
			require.NoError(t, err)
			assert.Equal(t, FXRefreshJobType, schedule.JobType)
			assert.Equal(t, FXRefreshJobType, scheduler.schedule.JobType)
			assert.Equal(t, provider.Name(), scheduler.schedule.Input.Provider)

			diagnostics, err := service.GetFXAdminDiagnostics(
				t.Context(),
				FXAdminDiagnosticsParams{},
			)
			require.NoError(t, err)
			assert.Equal(t, provider.Name(), diagnostics.DefaultProvider)
			assert.ElementsMatch(
				t,
				[]string{FXProviderFrankfurter, FXProviderNBP, FXProviderECB, provider.Name()},
				[]string{
					diagnostics.Providers[0].Name,
					diagnostics.Providers[1].Name,
					diagnostics.Providers[2].Name,
					diagnostics.Providers[3].Name,
				},
			)
			assert.Equal(t, 1, diagnostics.StoredRatesCount)

			encoded, err := json.Marshal(diagnostics)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), "token")
			assert.NotContains(t, string(encoded), "secret")
			assert.NotContains(t, string(encoded), "apiKey")
		},
	)

	t.Run("refreshes required pairs safely", func(t *testing.T) {
		makeRequiredPairs := func(t *testing.T, currencies ...string) (*persistence.Database, *persistence.CurrentFXRateStore) {
			t.Helper()
			database := openTestDatabase(t)
			store := persistence.NewStore(database)
			fake := faker.New()
			now := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)
			tenant := domain.Tenant{
				ID:              "tenant-" + fake.UUID().V4(),
				Name:            "tenant-" + fake.Company().Name(),
				DisplayCurrency: "PLN",
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			_, err := store.SaveTenant(t.Context(), tenant)
			require.NoError(t, err)
			for _, currency := range currencies {
				_, err = store.SaveAccount(t.Context(), domain.Account{
					ID:        "account-" + fake.UUID().V4(),
					TenantID:  tenant.ID,
					Name:      "account-" + fake.Lorem().Word(),
					Currency:  currency,
					Kind:      domain.AccountKindManual,
					CreatedAt: now,
					UpdatedAt: now,
				})
				require.NoError(t, err)
			}
			return database, persistence.NewCurrentFXRateStore(database)
		}

		t.Run("writes every required pair after successful responses", func(t *testing.T) {
			database, rateStore := makeRequiredPairs(t, "USD", "EUR")
			provider := NewMockFXRatesProvider(t)
			providerName := "provider-" + faker.New().Lorem().Word()
			refreshedAt := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)
			provider.EXPECT().Name().Return(providerName).Once()
			provider.EXPECT().FetchLatestRates(mock.Anything, mock.Anything).RunAndReturn(func(
				_ context.Context,
				query FXProviderQuery,
			) ([]domain.FXRate, error) {
				return []domain.FXRate{{
					Provider: providerName, BaseCurrency: query.BaseCurrency,
					QuoteCurrency: query.QuoteCurrencies[0], EffectiveAt: refreshedAt.Add(-time.Hour), Rate: 4.2,
				}}, nil
			}).Twice()
			service := NewFXService(
				rateStore,
				WithFXServiceNow(func() time.Time { return refreshedAt }),
				WithFXServiceProviders(provider),
				WithFXServiceDefaultProvider(providerName),
				WithFXServiceRequiredPairs(persistence.NewFXPairDiscoveryStore(database)),
			)

			result, err := service.RefreshRequiredFXRates(t.Context(), RefreshFXRatesParams{})
			require.NoError(t, err)
			assert.Equal(t, SyncFXRatesResult{Provider: providerName, ImportedCount: 2}, result)
			rates, err := rateStore.ListCurrentFXRates(t.Context(), persistence.ListCurrentFXRatesParams{
				Provider: providerName,
			})
			require.NoError(t, err)
			require.Len(t, rates, 2)
			assert.Equal(t, refreshedAt, rates[0].LastSuccessfulRefreshAt)
			assert.Equal(t, refreshedAt, rates[1].LastSuccessfulRefreshAt)
		})

		t.Run("keeps all last-good rates when a required pair fails", func(t *testing.T) {
			database, rateStore := makeRequiredPairs(t, "USD", "EUR")
			provider := NewMockFXRatesProvider(t)
			providerName := "provider-" + faker.New().Lorem().Word()
			lastGoodAt := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
			existing := []domain.FXRate{
				{
					Provider: providerName, BaseCurrency: "USD", QuoteCurrency: "PLN", EffectiveAt: lastGoodAt,
					LastSuccessfulRefreshAt: lastGoodAt, Rate: 4.1,
				},
				{
					Provider: providerName, BaseCurrency: "EUR", QuoteCurrency: "PLN", EffectiveAt: lastGoodAt,
					LastSuccessfulRefreshAt: lastGoodAt, Rate: 4.3,
				},
			}
			require.NoError(t, rateStore.SaveCurrentFXRates(t.Context(), existing))
			before, err := rateStore.ListCurrentFXRates(t.Context(), persistence.ListCurrentFXRatesParams{
				Provider: providerName,
			})
			require.NoError(t, err)
			provider.EXPECT().Name().Return(providerName).Once()
			provider.EXPECT().FetchLatestRates(mock.Anything, mock.Anything).RunAndReturn(func(
				_ context.Context,
				query FXProviderQuery,
			) ([]domain.FXRate, error) {
				if query.BaseCurrency == "EUR" {
					return nil, errors.New("provider unavailable")
				}
				return []domain.FXRate{{
					Provider:      providerName,
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					EffectiveAt:   lastGoodAt.Add(time.Hour),
					Rate:          4.2,
				}}, nil
			}).Once()
			service := NewFXService(
				rateStore,
				WithFXServiceProviders(provider),
				WithFXServiceDefaultProvider(providerName),
				WithFXServiceRequiredPairs(persistence.NewFXPairDiscoveryStore(database)),
			)

			_, err = service.RefreshRequiredFXRates(t.Context(), RefreshFXRatesParams{})
			require.Error(t, err)
			rates, listErr := rateStore.ListCurrentFXRates(t.Context(), persistence.ListCurrentFXRatesParams{
				Provider: providerName,
			})
			require.NoError(t, listErr)
			assert.Equal(t, before, rates)
		})

		t.Run("rejects malformed provider responses without replacing last-good rates", func(t *testing.T) {
			malformedRates := func(providerName string, effectiveAt time.Time) map[string][]domain.FXRate {
				makeRate := func(provider string, baseCurrency string, rate float64) domain.FXRate {
					return domain.FXRate{
						Provider: provider, BaseCurrency: baseCurrency, QuoteCurrency: "PLN",
						EffectiveAt: effectiveAt, Rate: rate,
					}
				}
				return map[string][]domain.FXRate{
					"empty":          nil,
					"wrong provider": {makeRate("other-provider", "USD", 4.2)},
					"wrong pair":     {makeRate(providerName, "EUR", 4.2)},
					"duplicate": {
						makeRate(providerName, "USD", 4.2),
						makeRate(providerName, "USD", 4.2),
					},
					"zero":     {makeRate(providerName, "USD", 0)},
					"negative": {makeRate(providerName, "USD", -4.2)},
					"NaN":      {makeRate(providerName, "USD", math.NaN())},
					"infinite": {makeRate(providerName, "USD", math.Inf(1))},
				}
			}
			for name, response := range malformedRates("provider", time.Date(2026, time.July, 14, 11, 0, 0, 0, time.UTC)) {
				t.Run(name, func(t *testing.T) {
					database, rateStore := makeRequiredPairs(t, "USD")
					provider := NewMockFXRatesProvider(t)
					providerName := "provider"
					lastGoodAt := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
					lastGood := domain.FXRate{
						Provider: providerName, BaseCurrency: "USD", QuoteCurrency: "PLN", EffectiveAt: lastGoodAt,
						LastSuccessfulRefreshAt: lastGoodAt, Rate: 4.1,
					}
					require.NoError(t, rateStore.SaveCurrentFXRates(t.Context(), []domain.FXRate{lastGood}))
					before, listErr := rateStore.ListCurrentFXRates(t.Context(), persistence.ListCurrentFXRatesParams{
						Provider: providerName,
					})
					require.NoError(t, listErr)
					provider.EXPECT().Name().Return(providerName).Once()
					provider.EXPECT().FetchLatestRates(mock.Anything, mock.Anything).Return(response, nil).Once()
					service := NewFXService(
						rateStore,
						WithFXServiceProviders(provider),
						WithFXServiceDefaultProvider(providerName),
						WithFXServiceRequiredPairs(persistence.NewFXPairDiscoveryStore(database)),
					)

					_, err := service.RefreshRequiredFXRates(t.Context(), RefreshFXRatesParams{})
					require.Error(t, err)
					rates, listErr := rateStore.ListCurrentFXRates(t.Context(), persistence.ListCurrentFXRatesParams{
						Provider: providerName,
					})
					require.NoError(t, listErr)
					assert.Equal(t, before, rates)
				})
			}
		})

		t.Run("succeeds without a provider call when no pairs are required", func(t *testing.T) {
			database := openTestDatabase(t)
			provider := NewMockFXRatesProvider(t)
			providerName := "provider-" + faker.New().Lorem().Word()
			provider.EXPECT().Name().Return(providerName).Once()
			service := NewFXService(
				persistence.NewCurrentFXRateStore(database),
				WithFXServiceProviders(provider),
				WithFXServiceDefaultProvider(providerName),
				WithFXServiceRequiredPairs(persistence.NewFXPairDiscoveryStore(database)),
			)
			result, err := service.RefreshRequiredFXRates(t.Context(), RefreshFXRatesParams{})
			require.NoError(t, err)
			assert.Equal(t, SyncFXRatesResult{Provider: providerName}, result)
		})
	})

	t.Run("returns only current FX metadata relevant to the dashboard tenant", func(t *testing.T) {
		store := makeStore(t)
		now := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)
		fake := faker.New()
		service := NewService(store, WithNow(func() time.Time { return now }))
		firstOwnerID := "owner-first-" + fake.UUID().V4()
		secondOwnerID := "owner-second-" + fake.UUID().V4()
		firstTenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID: firstOwnerID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "PLN",
		})
		require.NoError(t, err)
		secondTenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID: secondOwnerID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "PLN",
		})
		require.NoError(t, err)
		_, err = service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: firstOwnerID, TenantID: firstTenant.ID, Name: "account-" + fake.Lorem().Word(),
			Currency: "USD", Kind: domain.AccountKindManual,
		})
		require.NoError(t, err)
		_, err = service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: secondOwnerID, TenantID: secondTenant.ID, Name: "account-" + fake.Lorem().Word(),
			Currency: "EUR", Kind: domain.AccountKindManual,
		})
		require.NoError(t, err)
		require.NoError(t, store.SaveCurrentFXRates(t.Context(), []domain.FXRate{
			{
				Provider:                FXProviderFrankfurter,
				BaseCurrency:            "USD",
				QuoteCurrency:           "PLN",
				EffectiveAt:             now,
				LastSuccessfulRefreshAt: now, Rate: 4.2,
			},
			{
				Provider:                FXProviderFrankfurter,
				BaseCurrency:            "EUR",
				QuoteCurrency:           "PLN",
				EffectiveAt:             now.Add(-72 * time.Hour),
				LastSuccessfulRefreshAt: now.Add(-72 * time.Hour), Rate: 4.3,
			},
		}))

		dashboard, err := service.GetDashboard(t.Context(), DashboardParams{
			ActorUserID: firstOwnerID, TenantID: firstTenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, []DashboardFXRate{{
			Provider:                FXProviderFrankfurter,
			BaseCurrency:            "USD",
			QuoteCurrency:           "PLN",
			EffectiveAt:             now,
			LastSuccessfulRefreshAt: now, Stale: false,
		}}, dashboard.CurrentFXRates)
		assert.Empty(t, dashboard.Alerts)
	})

	t.Run(
		"parses frankfurter payloads and keeps nbp ecb provider seams available",
		func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/v2/rates", r.URL.Path)
					assert.Equal(t, "USD", r.URL.Query().Get("base"))
					assert.Equal(t, "PLN,EUR", r.URL.Query().Get("quotes"))
					assert.Empty(t, r.URL.Query().Get("from"))
					assert.Empty(t, r.URL.Query().Get("to"))
					_, err := w.Write([]byte(
						`[{"date":"2026-06-20","base":"USD","quote":"PLN","rate":4.1234},{"date":"2026-06-20","base":"USD","quote":"EUR","rate":0.8811}]`,
					))
					assert.NoError(t, err)
				}),
			)
			defer server.Close()

			provider := NewFrankfurterFXProvider(server.Client(), server.URL)
			rates, err := provider.FetchHistoricalRates(t.Context(), FXProviderQuery{
				BaseCurrency:    "USD",
				QuoteCurrencies: []string{"PLN", "EUR"},
				StartDate:       time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
				EndDate:         time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			})
			require.NoError(t, err)
			assert.ElementsMatch(t, []domain.FXRate{
				{
					Provider:      FXProviderFrankfurter,
					BaseCurrency:  "USD",
					QuoteCurrency: "PLN",
					RateDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.Local),
					Rate:          4.1234,
				},
				{
					Provider:      FXProviderFrankfurter,
					BaseCurrency:  "USD",
					QuoteCurrency: "EUR",
					RateDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.Local),
					Rate:          0.8811,
				},
			}, rates)

			assert.Equal(t, FXProviderNBP, NewNBPFXProvider(nil, "").Name())
			assert.Equal(t, FXProviderECB, NewECBFXProvider(nil, "").Name())
		},
	)

	t.Run("rejects frankfurter non-success and malformed latest responses", func(t *testing.T) {
		makeProvider := func(t *testing.T, status int, body string) *FrankfurterFXProvider {
			t.Helper()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, err := w.Write([]byte(body))
				assert.NoError(t, err)
			}))
			t.Cleanup(server.Close)
			return NewFrankfurterFXProvider(server.Client(), server.URL)
		}
		query := FXProviderQuery{BaseCurrency: "USD", QuoteCurrencies: []string{"PLN"}}

		_, err := makeProvider(t, http.StatusNotFound, `{"message":"not found"}`).FetchLatestRates(t.Context(), query)
		require.Error(t, err)

		for _, body := range []string{
			`{`,
			`[]`,
			`[{"date":"","base":"USD","quote":"PLN","rate":4.1}]`,
			`[{"date":"2026-06-20","base":"","quote":"PLN","rate":4.1}]`,
			`[{"date":"2026-06-20","base":"USD","quote":"","rate":4.1}]`,
			`[{"date":"2026-06-20","base":"EUR","quote":"PLN","rate":4.1}]`,
			`[{"date":"2026-06-20","base":"USD","quote":"EUR","rate":4.1}]`,
			`[{"date":"2026-06-20","base":"USD","quote":"PLN","rate":0}]`,
			`[{"date":"2026-06-20","base":"USD","quote":"PLN","rate":4.1,"unexpected":true}]`,
		} {
			_, fetchErr := makeProvider(t, http.StatusOK, body).FetchLatestRates(t.Context(), query)
			require.Error(t, fetchErr)
		}
	})
}
