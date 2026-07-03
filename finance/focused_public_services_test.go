package finance

import (
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
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
		service := NewReportingService(store, WithReportingServiceNow(func() time.Time { return now }))
		fake := faker.New()

		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "PLN",
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
			Provider:      FXProviderFrankfurter,
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
		enqueuer := &capturedFXSyncJobEnqueuer{}
		scheduleWriter := &capturedFXSyncScheduleWriter{}
		service := NewFXService(
			store,
			WithFXServiceProviders(provider),
			WithFXServiceDefaultProvider(provider.Name()),
			WithFXServiceJobEnqueuer(enqueuer),
			WithFXServiceScheduleWriter(scheduleWriter),
		)

		_, err := service.SyncFXRates(t.Context(), SyncFXRatesParams{
			BaseCurrencies: []string{"USD"},
			QuoteCurrency:  "PLN",
			StartDate:      time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:        time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)

		_, err = service.TriggerFXSync(t.Context(), TriggerFXSyncParams{
			RequestedByUserID: "admin-" + fake.UUID().V4(),
			Source:            FXSyncRequesterSourceOperator,
			BaseCurrencies:    []string{"USD"},
			QuoteCurrency:     "PLN",
			StartDate:         time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			EndDate:           time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)

		_, err = service.EnsureFXSyncSchedule(t.Context(), EnsureFXSyncScheduleParams{
			ScheduleID:      "schedule-" + fake.UUID().V4(),
			BaseCurrencies:  []string{"USD"},
			QuoteCurrency:   "PLN",
			Interval:        time.Hour,
			RequestedByUser: "system",
		})
		require.NoError(t, err)

		diagnostics, err := service.GetFXAdminDiagnostics(t.Context(), FXAdminDiagnosticsParams{})
		require.NoError(t, err)
		assert.Equal(t, 1, diagnostics.StoredRatesCount)
	})

	t.Run("csv import service handles preview confirm run and audit without root service", func(t *testing.T) {
		store := makeStore(t)
		tenantService := NewTenantService(store)
		catalogService := NewCatalogService(store)
		ledgerService := NewLedgerService(store)
		enqueuer := &recordingCSVJobEnqueuer{jobID: "job-1", jobType: CSVImportJobTypeAccounts}
		service := NewCSVImportService(
			store,
			catalogService,
			ledgerService,
			WithCSVImportServiceJobEnqueuer(enqueuer),
		)
		fake := faker.New()

		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
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
			Mapping:     preview.Mapping,
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

	t.Run("bank sync service handles sync workflows while bank link stays separate", func(t *testing.T) {
		store := makeStore(t)
		tenantService := NewTenantService(store)
		fake := faker.New()
		cipher, err := credentials.NewAESGCMCipher(
			[]byte("0123456789abcdef0123456789abcdef"),
			"test-key",
		)
		require.NoError(t, err)
		provider := &stubBankProvider{
			name: "monobank",
			syncResults: []ProviderSyncResult{{
				SyncKey: "sync-" + fake.UUID().V4(),
				Accounts: []ProviderNormalizedAccount{{
					ProviderAccountID: "provider-account-" + fake.UUID().V4(),
					Name:              "main",
					Currency:          "USD",
				}},
			}},
		}
		enqueuer := &capturedBankSyncJobEnqueuer{}
		scheduleWriter := &capturedBankSyncScheduleWriter{}
		service := NewBankSyncService(
			store,
			WithBankSyncServiceConnectionSecretCipher(cipher),
			WithBankSyncServiceProviders(provider),
			WithBankSyncServiceJobEnqueuer(enqueuer),
			WithBankSyncServiceScheduleWriter(scheduleWriter),
		)
		linkService := NewService(store, WithConnectionSecretCipher(cipher), WithBankProviders(provider))

		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := tenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)

		secretID, err := linkService.encryptAndSaveConnectionSecret(
			t.Context(),
			provider.Name(),
			"ref-"+fake.UUID().V4(),
			"secret-"+fake.UUID().V4(),
		)
		require.NoError(t, err)
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-" + fake.UUID().V4(),
			TenantID:          tenant.ID,
			Provider:          provider.Name(),
			ConnectorID:       domain.ProviderConnectorIDMonobank,
			DisplayName:       "Connection " + fake.Company().Name(),
			ProviderReference: "ref-" + fake.UUID().V4(),
			ExternalID:        "external-" + fake.UUID().V4(),
			SecretID:          secretID,
			State:             domain.BankConnectionStateActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		})
		require.NoError(t, err)

		_, err = service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID:  ownerUserID,
			TenantID:     tenant.ID,
			ConnectionID: connection.ID,
			Interval:     time.Hour,
			NextRunAt:    time.Now().UTC(),
		})
		require.NoError(t, err)
		_, err = service.TriggerBankConnectionSync(t.Context(), TriggerBankConnectionSyncParams{
			ActorUserID:  ownerUserID,
			TenantID:     tenant.ID,
			ConnectionID: connection.ID,
			Reason:       BankConnectionSyncReasonManual,
		})
		require.NoError(t, err)
		_, err = service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		_, err = service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-1",
		})
		require.NoError(t, err)
		require.NoError(t, service.DeleteBankConnection(t.Context(), DeleteBankConnectionParams{
			ActorUserID:  ownerUserID,
			TenantID:     tenant.ID,
			ConnectionID: connection.ID,
		}))
	})
}
