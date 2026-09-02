package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	financefixtures "github.com/gemyago/sumweave/finance/fixtures"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFinanceCommand(t *testing.T) {
	makeRootCmd := func(t *testing.T, deps financeFixturesCommandDeps) (*cobra.Command, *bytes.Buffer) {
		t.Helper()
		rootCmd := newRootCmd()
		stdout := &bytes.Buffer{}
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stdout)
		rootCmd.AddCommand(newFinanceCmd(deps))
		return rootCmd, stdout
	}

	t.Run("wires finance fixtures generate command", func(t *testing.T) {
		rootCmd, _ := makeRootCmd(t, financeFixturesCommandDeps{})
		financeCmd, _, err := rootCmd.Find([]string{"finance", "fixtures", "generate"})
		require.NoError(t, err)
		assert.Equal(t, financeGenerateCommandName, financeCmd.Name())
	})

	t.Run("defaults production preparation wiring without preparing a schema", func(t *testing.T) {
		deps := defaultFinanceFixturesCommandDeps(financeFixturesCommandDeps{})
		require.NotNil(t, deps.Generate)
		require.NotNil(t, deps.Prepare)
	})

	t.Run("generate orchestrates injected preparation and emits summary json", func(t *testing.T) {
		fake := faker.New()
		want := financefixtures.Summary{
			Seed:        42,
			Scenario:    "realistic",
			ScenarioIDs: []string{"realistic-core"},
		}
		wantConfig := financeFixturesRuntimeConfig{
			JWTSigningKey:   fake.UUID().V4(),
			MonobankBaseURL: "https://" + fake.Internet().Domain(),
		}
		ownerID := "owner-" + fake.UUID().V4()
		memberID := "member-" + fake.UUID().V4()
		prepared := false
		rootCmd, stdout := makeRootCmd(t, financeFixturesCommandDeps{
			ResolveRuntimeConfig: func(*cobra.Command) (financeFixturesRuntimeConfig, error) {
				return wantConfig, nil
			},
			Prepare: func(_ context.Context, runtimeConfig financeFixturesRuntimeConfig) error {
				require.Equal(t, wantConfig, runtimeConfig)
				prepared = true
				return nil
			},
			Generate: func(
				_ context.Context,
				runtimeConfig financeFixturesRuntimeConfig,
				params financeFixturesGenerateParams,
			) (financefixtures.Summary, error) {
				require.True(t, prepared)
				require.Equal(t, wantConfig, runtimeConfig)
				require.Equal(t, int64(42), params.Seed)
				require.Equal(t, "realistic", params.Scenario)
				require.Equal(t, ownerID, params.OwnerUserID)
				require.Equal(t, memberID, params.MemberUserID)
				require.Equal(t, fixtureMonobankProviderName, params.ConnectionProvider)
				return want, nil
			},
			Now: func() time.Time { return time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC) },
		})
		rootCmd.SetArgs([]string{
			"finance", "fixtures", "generate", "--seed", "42", "--scenario", "realistic",
			"--owner-user-id", ownerID, "--member-user-id", memberID,
			"--connection-provider", fixtureMonobankProviderName,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		var got financefixtures.Summary
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, want, got)
	})

	t.Run("generate surfaces runner errors after preparation", func(t *testing.T) {
		prepared := false
		rootCmd, _ := makeRootCmd(t, financeFixturesCommandDeps{
			Prepare: func(context.Context, financeFixturesRuntimeConfig) error {
				prepared = true
				return nil
			},
			Generate: func(context.Context, financeFixturesRuntimeConfig, financeFixturesGenerateParams) (financefixtures.Summary, error) {
				require.True(t, prepared)
				return financefixtures.Summary{}, assert.AnError
			},
		})
		rootCmd.SetArgs([]string{"finance", "fixtures", "generate", "--seed", "7"})
		require.ErrorIs(t, rootCmd.Execute(), assert.AnError)
	})

	t.Run("generate surfaces runtime config resolution errors", func(t *testing.T) {
		rootCmd, _ := makeRootCmd(t, financeFixturesCommandDeps{
			ResolveRuntimeConfig: func(*cobra.Command) (financeFixturesRuntimeConfig, error) {
				return financeFixturesRuntimeConfig{}, assert.AnError
			},
		})
		rootCmd.SetArgs([]string{"finance", "fixtures", "generate", "--seed", "7"})
		require.ErrorIs(t, rootCmd.Execute(), assert.AnError)
	})

	t.Run("generate surfaces preparation failures without running generation", func(t *testing.T) {
		rootCmd, _ := makeRootCmd(t, financeFixturesCommandDeps{
			Prepare: func(context.Context, financeFixturesRuntimeConfig) error { return assert.AnError },
			Generate: func(context.Context, financeFixturesRuntimeConfig, financeFixturesGenerateParams) (financefixtures.Summary, error) {
				t.Fatal("generation must not run after preparation fails")
				return financefixtures.Summary{}, nil
			},
		})
		rootCmd.SetArgs([]string{"finance", "fixtures", "generate"})
		require.ErrorIs(t, rootCmd.Execute(), assert.AnError)
	})

	t.Run("resolve finance fixtures runtime config reports command configuration errors", func(t *testing.T) {
		_, err := resolveFinanceFixturesRuntimeConfig(&cobra.Command{})
		require.Error(t, err)
	})

	t.Run("run finance fixtures generate rejects unsupported scenarios without persistence", func(t *testing.T) {
		fake := faker.New()
		_, err := runFinanceFixturesGenerate(
			t.Context(),
			financeFixturesRuntimeConfig{JWTSigningKey: fake.UUID().V4()},
			financeFixturesGenerateParams{
				Seed: 3, Scenario: "unsupported-" + fake.Lorem().Word(), Now: time.Now(),
			},
		)
		require.ErrorContains(t, err, "unsupported finance fixture scenario")
	})

	t.Run("run finance fixtures generate rejects unsupported providers without persistence", func(t *testing.T) {
		fake := faker.New()
		_, err := runFinanceFixturesGenerate(
			t.Context(),
			financeFixturesRuntimeConfig{JWTSigningKey: fake.UUID().V4()},
			financeFixturesGenerateParams{
				Seed: 8, Scenario: realisticScenarioName, Now: time.Now(),
				ConnectionProvider: "unsupported-" + fake.Lorem().Word(),
			},
		)
		require.ErrorContains(t, err, "unsupported finance fixture connection provider")
	})

	t.Run("finance fixture module validates its composition inputs", func(t *testing.T) {
		fake := faker.New()
		cipher, err := credentials.NewAESGCMCipher([]byte("12345678901234567890123456789012"), fake.UUID().V4())
		require.NoError(t, err)
		_, err = newFinanceFixturesModule(
			nil,
			financeFixturesGenerateParams{Now: time.Now()},
			cipher,
			http.DefaultClient,
			"https://"+fake.Internet().Domain(),
		)
		require.Error(t, err)
	})

	t.Run("finance fixture composition surfaces storage failures through SQLMock", func(t *testing.T) {
		sqlDB, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			databaseMock.ExpectClose()
			require.NoError(t, sqlDB.Close())
		})
		database, err := persistence.NewDatabase(sqlDB, "postgres://example.invalid/fixtures")
		require.NoError(t, err)
		storageErr := assert.AnError
		databaseMock.ExpectBegin().WillReturnError(storageErr)
		_, err = runFinanceFixturesGenerate(
			t.Context(),
			financeFixturesRuntimeConfig{Database: database, JWTSigningKey: faker.New().UUID().V4()},
			financeFixturesGenerateParams{
				Seed:               9,
				Scenario:           realisticScenarioName,
				Now:                time.Now(),
				ConnectionProvider: fixtureScenarioProviderName,
			},
		)
		require.ErrorIs(t, err, storageErr)
		require.NoError(t, databaseMock.ExpectationsWereMet())
	})

	t.Run("finance fixture monobank server rejects non-fixture paths", func(t *testing.T) {
		server := newFinanceFixturesMonobankServer()
		defer server.Close()
		response, err := server.Client().Get(server.URL + "/personal/statement")
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
		response, err = server.Client().Get(server.URL + "/personal/client-info")
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("fixture FX provider supplies every requested shared-schema pair", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		provider := financeFixturesFXProvider{now: func() time.Time { return now }}
		baseCurrency := fake.Currency().Code()
		quoteCurrency := fake.Currency().Code()
		rates, err := provider.FetchLatestRates(t.Context(), financepkg.FXProviderQuery{
			BaseCurrency: baseCurrency, QuoteCurrencies: []string{quoteCurrency},
		})
		require.NoError(t, err)
		assert.Equal(t, []domain.FXRate{{
			Provider: financepkg.FXProviderFrankfurter, BaseCurrency: strings.ToUpper(baseCurrency),
			QuoteCurrency: strings.ToUpper(quoteCurrency), RateDate: now, Rate: 1.1,
		}}, rates)

		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = provider.FetchLatestRates(canceledCtx, financepkg.FXProviderQuery{})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("fixture provider exposes deterministic finance provider behavior", func(t *testing.T) {
		provider := financeFixturesProvider{}
		assert.Equal(t, fixtureMonobankProviderName, provider.Name())
		start, err := provider.StartLink(t.Context(), financepkg.ProviderStartLinkParams{})
		require.NoError(t, err)
		assert.Equal(t, financepkg.ProviderLinkStart{}, start)
		finish, err := provider.FinishLink(t.Context(), financepkg.ProviderFinishLinkParams{})
		require.NoError(t, err)
		assert.Equal(t, financepkg.ProviderLinkResult{}, finish)
		link, err := provider.LinkToken(t.Context(), financepkg.ProviderTokenLinkParams{})
		require.NoError(t, err)
		assert.Equal(t, financepkg.ProviderTokenLinkResult{
			DisplayName: "Fixture Connection", ProviderReference: "fixture-reference", Secret: "fixture-secret",
			State: domain.BankConnectionStateActive,
		}, link)
		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = provider.LinkToken(canceledCtx, financepkg.ProviderTokenLinkParams{})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("fixture scenario adapter delegates every operation to its service boundary", func(t *testing.T) {
		tenants := newMockfinanceFixturesTenantService(t)
		catalog := newMockfinanceFixturesCatalogService(t)
		ledger := newMockfinanceFixturesLedgerService(t)
		csvImports := newMockfinanceFixturesCSVImportService(t)
		connections := newMockfinanceFixturesBankConnectionService(t)
		bankSync := newMockfinanceFixturesBankSyncService(t)
		fx := newMockfinanceFixturesFXService(t)
		service := financeFixturesScenarioService{
			tenantService: tenants, catalogService: catalog, ledgerService: ledger,
			csvImportService: csvImports, bankConnectionService: connections, bankSyncService: bankSync,
			fxService: fx,
		}

		tenants.EXPECT().CreateTenant(mock.Anything, mock.Anything).Return(domain.Tenant{}, nil)
		_, err := service.CreateTenant(t.Context(), financepkg.CreateTenantParams{})
		require.NoError(t, err)
		tenants.EXPECT().CreateTenantInvite(mock.Anything, mock.Anything).Return(domain.TenantInvite{}, nil)
		_, err = service.CreateTenantInvite(t.Context(), financepkg.CreateTenantInviteParams{})
		require.NoError(t, err)
		tenants.EXPECT().AcceptTenantInvite(mock.Anything, mock.Anything).Return(domain.TenantMembership{}, nil)
		_, err = service.AcceptTenantInvite(t.Context(), financepkg.AcceptTenantInviteParams{})
		require.NoError(t, err)

		catalog.EXPECT().CreateAccount(mock.Anything, mock.Anything).Return(domain.Account{}, nil)
		_, err = service.CreateAccount(t.Context(), financepkg.CreateAccountParams{})
		require.NoError(t, err)
		catalog.EXPECT().ListCategories(mock.Anything, mock.Anything).Return([]domain.Category{}, nil)
		_, err = service.ListCategories(t.Context(), financepkg.ListCategoriesParams{})
		require.NoError(t, err)
		catalog.EXPECT().ListTags(mock.Anything, mock.Anything).Return([]domain.Tag{}, nil)
		_, err = service.ListTags(t.Context(), financepkg.ListTagsParams{})
		require.NoError(t, err)

		ledger.EXPECT().RecordTransaction(mock.Anything, mock.Anything).Return(domain.Transaction{}, nil)
		_, err = service.RecordTransaction(t.Context(), financepkg.RecordTransactionParams{})
		require.NoError(t, err)
		ledger.EXPECT().HideTransaction(mock.Anything, mock.Anything).Return(nil)
		require.NoError(t, service.HideTransaction(t.Context(), financepkg.HideTransactionParams{}))
		ledger.EXPECT().LinkTransfers(mock.Anything, mock.Anything).Return(nil)
		require.NoError(t, service.LinkTransfers(t.Context(), financepkg.LinkTransfersParams{}))

		csvImports.EXPECT().PreviewCSVImport(mock.Anything, mock.Anything).Return(financepkg.CSVImportPreview{}, nil)
		_, err = service.PreviewCSVImport(t.Context(), financepkg.PreviewCSVImportParams{})
		require.NoError(t, err)
		connections.EXPECT().LinkTokenBankConnection(mock.Anything, mock.Anything).Return(domain.BankConnection{}, nil)
		_, err = service.LinkTokenBankConnection(t.Context(), financepkg.LinkTokenBankConnectionParams{})
		require.NoError(t, err)
		bankSync.EXPECT().UpsertBankConnectionSchedule(
			mock.Anything,
			mock.Anything,
		).Return(domain.BankConnectionSchedule{}, nil)
		_, err = service.UpsertBankConnectionSchedule(t.Context(), financepkg.UpsertBankConnectionScheduleParams{})
		require.NoError(t, err)
		fx.EXPECT().RefreshRequiredFXRates(mock.Anything, mock.Anything).Return(financepkg.RefreshFXRatesResult{}, nil)
		_, err = service.RefreshRequiredFXRates(t.Context(), financepkg.RefreshFXRatesParams{})
		require.NoError(t, err)
	})

	t.Run("joins fixture root close errors with command errors", func(t *testing.T) {
		closeErr := assert.AnError
		err := closeFinanceFixturesRuntimeConfig(
			closeErr,
			financeFixturesRuntimeConfig{close: func() error { return closeErr }},
		)
		require.ErrorIs(t, err, closeErr)
		require.NoError(t, closeFinanceFixturesRuntimeConfig(nil, financeFixturesRuntimeConfig{}))
	})

	t.Run("closes a resolved fixture root after a successful generation", func(t *testing.T) {
		closed := false
		rootCmd, _ := makeRootCmd(t, financeFixturesCommandDeps{
			ResolveRuntimeConfig: func(*cobra.Command) (financeFixturesRuntimeConfig, error) {
				return financeFixturesRuntimeConfig{close: func() error {
					closed = true
					return nil
				}}, nil
			},
			Prepare: func(context.Context, financeFixturesRuntimeConfig) error { return nil },
			Generate: func(context.Context, financeFixturesRuntimeConfig, financeFixturesGenerateParams) (financefixtures.Summary, error) {
				return financefixtures.Summary{}, nil
			},
		})
		rootCmd.SetArgs([]string{"finance", "fixtures", "generate"})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		assert.True(t, closed)
	})
}
