//go:build postgres_test

package finance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFocusedPublicServices(t *testing.T) {
	makeStore := func(t *testing.T) *persistence.Store {
		t.Helper()

		database := openTestDatabase(t)
		return persistence.NewStore(database)
	}

	t.Run("tenant service handles tenant workflows without root service", func(t *testing.T) {
		store := makeStore(t)
		service := NewTenantService(store)
		fake := faker.New()

		ownerUserID := "owner-" + fake.UUID().V4()
		memberUserID := "member-" + fake.UUID().V4()

		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		invite, err := service.CreateTenantInvite(t.Context(), CreateTenantInviteParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Recipient:   fmt.Sprintf("recipient-%s@example.com", fake.Internet().User()),
		})
		require.NoError(t, err)

		_, err = service.AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{
			ActorUserID: memberUserID,
			Code:        invite.Code,
		})
		require.NoError(t, err)

		_, err = service.ListTenantMembers(t.Context(), ListTenantMembersParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
	})

	t.Run("catalog service handles catalog workflows without root service", func(t *testing.T) {
		store := makeStore(t)
		tenantService := NewTenantService(store)
		service := NewCatalogService(store)
		fake := faker.New()

		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		account, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "account-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		_, err = service.AttachLinkedAccount(t.Context(), AttachLinkedAccountParams{
			ActorUserID:       ownerUserID,
			TenantID:          tenant.ID,
			AccountID:         account.ID,
			Provider:          "provider-" + fake.Lorem().Word(),
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
	})

	t.Run("ledger service handles ledger workflows without root service", func(t *testing.T) {
		store := makeStore(t)
		tenantService := NewTenantService(store)
		catalogService := NewCatalogService(store)
		service := NewLedgerService(store)
		fake := faker.New()

		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		account, err := catalogService.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "account-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		_, err = service.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: 100,
			Currency:    "USD",
			Description: "txn-" + fake.Lorem().Word(),
			EffectiveAt: time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
	})

	t.Run("reporting service handles dashboard workflows without root service", func(t *testing.T) {
		store := makeStore(t)
		tenantService := NewTenantService(store)
		catalogService := NewCatalogService(store)
		ledgerService := NewLedgerService(store)
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		fake := faker.New()
		provider := "provider-" + fake.UUID().V4()
		service := NewReportingService(
			store,
			WithReportingServiceNow(func() time.Time { return now }),
			WithReportingServiceDefaultFXProvider(provider),
		)

		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "PLN",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		account, err := catalogService.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "account-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		require.NoError(t, store.SaveFXRates(t.Context(), []domain.FXRate{{
			Provider:      provider,
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
			RateDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			Rate:          4.0,
		}}))

		_, err = ledgerService.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: 100_00,
			Currency:    "USD",
			Description: "txn-" + fake.Lorem().Word(),
			EffectiveAt: time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)

		dashboard, err := service.GetDashboard(t.Context(), DashboardParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(400_00), dashboard.Settled.IncomeMinor)
	})

	t.Run("fx service handles sync and diagnostics without root service", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		provider := NewStaticFXProvider("static-"+fake.Lorem().Word(), []domain.FXRate{{
			Provider:      "static",
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
			RateDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			Rate:          4.1,
		}})
		publisher := NewMockSemanticCommandPublisher(t)
		publisher.EXPECT().PublishSemanticCommand(mock.Anything, mock.Anything).Return(
			DispatchReference{MessageID: "message-" + fake.UUID().V4()}, nil,
		)
		service := NewFXService(
			store,
			WithFXServiceProviders(provider),
			WithFXServiceDefaultProvider(provider.Name()),
			WithFXServiceCommandPublisher(publisher),
		)

		err := store.SaveCurrentFXRates(t.Context(), []domain.FXRate{{
			Provider: provider.Name(), BaseCurrency: "USD", QuoteCurrency: "PLN",
			EffectiveAt: time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC), Rate: 4.1,
		}})
		require.NoError(t, err)

		_, err = service.TriggerFXRefresh(t.Context(), TriggerFXRefreshParams{
			RequestedByUserID: "admin-" + fake.UUID().V4(),
			Source:            CommandRequesterSourceOperator,
		})
		require.NoError(t, err)

		diagnostics, err := service.GetFXAdminDiagnostics(t.Context(), FXAdminDiagnosticsParams{})
		require.NoError(t, err)
		assert.Positive(t, diagnostics.StoredRatesCount)
	})

	t.Run("csv import service handles preview confirm run and audit without root service", func(t *testing.T) {
		store := makeStore(t)
		tenantService := NewTenantService(store)
		catalogService := NewCatalogService(store)
		ledgerService := NewLedgerService(store)
		publisher := NewMockSemanticCommandPublisher(t)
		publisher.EXPECT().PublishSemanticCommand(mock.Anything, mock.Anything).Return(
			DispatchReference{MessageID: "job-1"}, nil,
		)
		service := NewCSVImportService(
			store,
			catalogService,
			ledgerService,
			WithCSVImportServiceCommandPublisher(publisher),
		)
		fake := faker.New()

		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\n",
		})
		require.NoError(t, err)

		confirmed, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: ownerUserID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)

		_, err = service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{
			ImportID: preview.ImportID,
			JobID:    confirmed.JobID,
		})
		require.NoError(t, err)

		audit, err := service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)
		assert.Equal(t, CSVImportStatusCompleted, audit.Status)
	})

	t.Run("csv import surfaces preview store failures", func(t *testing.T) {
		fake := faker.New()
		csv := "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n29.05.26,wallet,,,1,,USD,purchase\n"
		denied := NewCSVImportService(stubStore{isTenantMemberFn: func(context.Context, string, string) (bool, error) {
			return false, nil
		}}, &CatalogService{}, &LedgerService{})
		_, err := denied.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			ImportType:  CSVImportTypeAccounts,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		legacy := NewCSVImportService(stubStore{isTenantMemberFn: func(context.Context, string, string) (bool, error) {
			return true, nil
		}}, &CatalogService{}, &LedgerService{})
		_, err = legacy.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			ImportType:  CSVImportTypeAccounts,
			CSV:         "name,currency,kind\naccount,USD,manual\n",
		})
		require.NoError(t, err)
		for _, makeFailureStore := range []func() stubStore{
			func() stubStore {
				return stubStore{isTenantMemberFn: func(context.Context, string, string) (bool, error) {
					return true, nil
				}, listAccountsFn: func(context.Context, string, bool) ([]domain.Account, error) {
					return nil, assert.AnError
				}}
			},
			func() stubStore {
				return stubStore{isTenantMemberFn: func(context.Context, string, string) (bool, error) {
					return true, nil
				}, listCategoriesFn: func(context.Context, string, bool) ([]domain.Category, error) {
					return nil, assert.AnError
				}}
			},
			func() stubStore {
				return stubStore{isTenantMemberFn: func(context.Context, string, string) (bool, error) {
					return true, nil
				}, listTagsFn: func(context.Context, string, bool) ([]domain.Tag, error) {
					return nil, assert.AnError
				}}
			},
			func() stubStore {
				return stubStore{isTenantMemberFn: func(context.Context, string, string) (bool, error) {
					return true, nil
				}, listTransactionsFn: func(
					context.Context,
					string,
					string,
					domain.TransactionSource,
					domain.TransactionStatus,
					bool,
				) ([]domain.Transaction, error) {
					return nil, assert.AnError
				}}
			},
		} {
			service := NewCSVImportService(makeFailureStore(), &CatalogService{}, &LedgerService{})
			_, previewErr := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
				ActorUserID: "actor-" + fake.UUID().V4(),
				TenantID:    "tenant-" + fake.UUID().V4(),
				ImportType:  CSVImportTypeTransactions,
				CSV:         csv,
			})
			require.ErrorIs(t, previewErr, assert.AnError)
		}
	})

	t.Run("rolls back journal cleanup when a later connection cleanup step fails", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		tenantService := NewTenantService(store)
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		connection := domain.BankConnection{
			ID:        "connection-" + fake.UUID().V4(),
			TenantID:  tenant.ID,
			Provider:  string(domain.ProviderIDMonobank),
			State:     domain.BankConnectionStateActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_, err = store.SaveBankConnection(t.Context(), connection)
		require.NoError(t, err)
		journalStore := persistence.NewProviderSyncStateJournalStore(store)
		attemptedAt := time.Now()
		state := domain.ProviderSyncState{
			Connection:  domain.ProviderConnectionRef{ConnectionID: connection.ID},
			AttemptedAt: &attemptedAt,
			Window: domain.ProviderSyncWindow{
				Start: attemptedAt.Add(-time.Hour),
				End:   attemptedAt,
			},
			JobID: "job-" + fake.UUID().V4(),
		}
		require.NoError(t, journalStore.AppendSyncState(t.Context(), state))
		snapshotDeleter := newMockproviderSnapshotConnectionDeleter(t)
		expectedErr := fmt.Errorf("snapshot-delete-%s", fake.UUID().V4())
		snapshotDeleter.EXPECT().
			DeleteProviderSnapshotsByConnection(mock.Anything, connection.ID).
			Once().
			Return(expectedErr)
		service := NewBankSyncService(
			store,
			newMockbankSyncOrchestrator(t),
			WithBankSyncServiceSyncStateJournalDeleter(journalStore),
			WithBankSyncServiceSnapshotDeleter(snapshotDeleter),
		)

		err = service.DeleteBankConnection(t.Context(), DeleteBankConnectionParams{
			ActorUserID:  ownerUserID,
			TenantID:     tenant.ID,
			ConnectionID: connection.ID,
		})
		require.ErrorIs(t, err, expectedErr)
		persistedState, err := journalStore.LoadLastState(t.Context(), state.Connection)
		require.NoError(t, err)
		require.NotNil(t, persistedState)
		assert.Equal(t, state.JobID, persistedState.JobID)
	})
}
