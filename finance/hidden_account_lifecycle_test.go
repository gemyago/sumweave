package finance

import (
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHiddenAccountLifecycle(t *testing.T) {
	makeFixture := func(t *testing.T) (*CatalogService, *LedgerService, *ReportingService, *CSVImportService, domain.Tenant, string) {
		t.Helper()

		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		ownerID := "owner-" + fake.UUID().V4()
		tenants := NewTenantService(store)
		tenant, err := tenants.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		catalog := NewCatalogService(store)
		ledger := NewLedgerService(store)
		reporting := NewReportingService(
			store,
			WithReportingServiceNow(func() time.Time {
				return time.Date(2026, time.July, 17, 10, 0, 0, 0, time.Local)
			}),
			WithReportingServiceFXRateStore(persistence.NewCurrentFXRateStore(database)),
		)
		imports := NewCSVImportService(
			store,
			catalog,
			ledger,
			WithCSVImportServiceRowStore(persistence.NewCSVImportStore(database)),
		)
		return catalog, ledger, reporting, imports, tenant, ownerID
	}

	makeAccount := func(
		t *testing.T,
		catalog *CatalogService,
		tenant domain.Tenant,
		ownerID string,
		currency string,
	) domain.Account {
		t.Helper()
		account, err := catalog.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerID,
			TenantID:    tenant.ID,
			Name:        "account-" + faker.New().Lorem().Word(),
			Currency:    currency,
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)
		return account
	}

	makeRecordParams := func(
		tenantID string,
		ownerID string,
		accountID string,
		amountMinor int64,
		currency string,
	) RecordTransactionParams {
		return RecordTransactionParams{
			ActorUserID: ownerID,
			TenantID:    tenantID,
			AccountID:   accountID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: amountMinor,
			Currency:    currency,
			Description: "transaction-" + faker.New().Lorem().Word(),
			EffectiveAt: time.Date(2026, time.July, 10, 12, 0, 0, 0, time.Local),
		}
	}

	t.Run("hides restores and preserves historical reads", func(t *testing.T) {
		catalog, ledger, _, _, tenant, ownerID := makeFixture(t)
		account := makeAccount(t, catalog, tenant, ownerID, "USD")
		_, err := ledger.RecordTransaction(t.Context(), makeRecordParams(tenant.ID, ownerID, account.ID, 500, "USD"))
		require.NoError(t, err)
		require.NoError(t, catalog.HideAccount(t.Context(), HideAccountParams{
			ActorUserID: ownerID, TenantID: tenant.ID, AccountID: account.ID,
		}))

		visible, err := catalog.ListAccounts(t.Context(), ListAccountsParams{ActorUserID: ownerID, TenantID: tenant.ID})
		require.NoError(t, err)
		assert.Empty(t, visible)
		historical, err := catalog.GetAccount(t.Context(), GetAccountParams{
			ActorUserID: ownerID, TenantID: tenant.ID, AccountID: account.ID,
		})
		require.NoError(t, err)
		assert.NotNil(t, historical.HiddenAt)
		assert.Equal(t, int64(500), historical.BookedBalanceMinor)
		transactions, err := ledger.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: ownerID, TenantID: tenant.ID,
		})
		require.NoError(t, err)
		assert.Len(t, transactions, 1)

		_, err = ledger.RecordTransaction(t.Context(), makeRecordParams(tenant.ID, ownerID, account.ID, 100, "USD"))
		require.ErrorIs(t, err, ErrHiddenAccount)
		require.NoError(t, catalog.UnhideAccount(t.Context(), UnhideAccountParams{
			ActorUserID: ownerID, TenantID: tenant.ID, AccountID: account.ID,
		}))
		visible, err = catalog.ListAccounts(t.Context(), ListAccountsParams{ActorUserID: ownerID, TenantID: tenant.ID})
		require.NoError(t, err)
		require.Len(t, visible, 1)
		assert.Nil(t, visible[0].HiddenAt)
		_, err = ledger.RecordTransaction(t.Context(), makeRecordParams(tenant.ID, ownerID, account.ID, 100, "USD"))
		require.NoError(t, err)
	})

	t.Run("excludes hidden account activity from current reporting and FX coverage", func(t *testing.T) {
		catalog, ledger, reporting, _, tenant, ownerID := makeFixture(t)
		active := makeAccount(t, catalog, tenant, ownerID, "USD")
		hidden := makeAccount(t, catalog, tenant, ownerID, "EUR")
		_, err := ledger.RecordTransaction(t.Context(), makeRecordParams(tenant.ID, ownerID, active.ID, 500, "USD"))
		require.NoError(t, err)
		_, err = ledger.RecordTransaction(t.Context(), makeRecordParams(tenant.ID, ownerID, hidden.ID, -200, "EUR"))
		require.NoError(t, err)
		require.NoError(t, catalog.HideAccount(t.Context(), HideAccountParams{
			ActorUserID: ownerID, TenantID: tenant.ID, AccountID: hidden.ID,
		}))

		summary, err := ledger.SummarizeTransactions(t.Context(), SummarizeTransactionsParams{
			ActorUserID: ownerID, TenantID: tenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, domain.TransactionSummary{IncomeMinor: 500, NetMinor: 500}, summary)
		dashboard, err := reporting.GetDashboard(t.Context(), DashboardParams{
			ActorUserID: ownerID, TenantID: tenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(500), dashboard.Settled.IncomeMinor)
		assert.Zero(t, dashboard.Settled.ExpenseMinor)
		assert.Len(t, dashboard.AccountBalances, 1)
		assert.Equal(t, active.ID, dashboard.AccountBalances[0].AccountID)
		assert.Equal(t, []DashboardCurrencyTotal{
			{Currency: "USD", IncomeMinor: 500, NetMinor: 500},
		}, dashboard.NativeSettledTotals)
		assert.Empty(t, dashboard.FXCoverage)
		assert.Empty(t, dashboard.MissingFX)
	})

	t.Run("rejects hidden accounts in preview and durable transaction row import", func(t *testing.T) {
		catalog, _, _, imports, tenant, ownerID := makeFixture(t)
		account := makeAccount(t, catalog, tenant, ownerID, "USD")
		require.NoError(t, catalog.HideAccount(t.Context(), HideAccountParams{
			ActorUserID: ownerID, TenantID: tenant.ID, AccountID: account.ID,
		}))
		csv := fmt.Sprintf(
			"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,%s,,,1,,USD,purchase\n",
			account.Name,
		)
		preview, err := imports.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: ownerID, TenantID: tenant.ID, ImportType: CSVImportTypeTransactions, CSV: csv,
		})
		require.NoError(t, err)
		assert.Zero(t, preview.ImportableCount)
		require.Len(t, preview.RejectedRows, 1)
		assert.Equal(t, fmt.Sprintf("account %q is hidden", account.Name), preview.RejectedRows[0].Reason)

		require.NoError(t, catalog.UnhideAccount(t.Context(), UnhideAccountParams{
			ActorUserID: ownerID, TenantID: tenant.ID, AccountID: account.ID,
		}))
		preview, err = imports.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: ownerID, TenantID: tenant.ID, ImportType: CSVImportTypeTransactions, CSV: csv,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, preview.ImportableCount)
		require.NoError(t, catalog.HideAccount(t.Context(), HideAccountParams{
			ActorUserID: ownerID, TenantID: tenant.ID, AccountID: account.ID,
		}))
		imports.csvImportJobEnqueuer = &recordingCSVJobEnqueuer{
			jobID: "job-" + faker.New().UUID().V4(), jobType: CSVImportJobTypeTransactions,
		}
		confirmed, err := imports.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: ownerID, ImportID: preview.ImportID,
		})
		require.NoError(t, err)
		result, err := imports.RunCSVImportJob(t.Context(), RunCSVImportJobParams{
			ImportID: preview.ImportID, JobID: confirmed.JobID,
		})
		require.NoError(t, err)
		assert.Zero(t, result.ImportedCount)
		require.Len(t, result.RejectedRows, 1)
		assert.Contains(t, result.RejectedRows[0].Reason, "is hidden")
	})
}
