package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	financefixtures "github.com/gemyago/sumweave/finance/fixtures"
	"github.com/gemyago/sumweave/finance/persistence"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type financeFixturesProvider struct{}

func (financeFixturesProvider) Name() string { return fixtureMonobankProviderName }

func (financeFixturesProvider) StartLink(
	context.Context,
	financepkg.ProviderStartLinkParams,
) (financepkg.ProviderLinkStart, error) {
	return financepkg.ProviderLinkStart{}, nil
}

func (financeFixturesProvider) FinishLink(
	context.Context,
	financepkg.ProviderFinishLinkParams,
) (financepkg.ProviderLinkResult, error) {
	return financepkg.ProviderLinkResult{}, nil
}

func (financeFixturesProvider) LinkToken(
	ctx context.Context,
	_ financepkg.ProviderTokenLinkParams,
) (financepkg.ProviderTokenLinkResult, error) {
	if err := ctx.Err(); err != nil {
		return financepkg.ProviderTokenLinkResult{}, err
	}
	return financepkg.ProviderTokenLinkResult{
		DisplayName:       "Fixture Connection",
		ProviderReference: "fixture-reference",
		Secret:            "fixture-secret",
		State:             domain.BankConnectionStateActive,
	}, nil
}

func TestFinanceCommand(t *testing.T) {
	makeRuntimeConfig := func(t *testing.T) financeFixturesRuntimeConfig {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		sqlDB, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		database, err := persistence.NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		jobsStore, err := jobspkg.NewStore(sqlDB, dsn, jobspkg.StoreOpts{TablePrefix: "sumweave_jobs_"})
		require.NoError(t, err)
		return financeFixturesRuntimeConfig{Database: database, JobsStore: jobsStore}
	}
	makeRootCmd := func(t *testing.T) (*cobra.Command, *bytes.Buffer) {
		t.Helper()
		rootCmd := newRootCmd()
		stdout := &bytes.Buffer{}
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stdout)
		rootCmd.AddCommand(newFinanceCmd())
		return rootCmd, stdout
	}

	t.Run("wires finance fixtures generate command", func(t *testing.T) {
		rootCmd, _ := makeRootCmd(t)
		financeCmd, _, err := rootCmd.Find([]string{"finance", "fixtures", "generate"})
		require.NoError(t, err)
		assert.Equal(t, financeGenerateCommandName, financeCmd.Name())
	})

	t.Run("generate uses PostgreSQL schemas and scopes generated data to its owner", func(t *testing.T) {
		fake := faker.New()
		t.Chdir("../..")
		rootCmd, stdout := makeRootCmd(t)
		require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))
		ownerID := "owner-" + fake.UUID().V4()
		memberID := "member-" + fake.UUID().V4()
		seed := int64(1_000_000 + fake.Int())
		rootCmd.SetArgs([]string{
			"finance", "fixtures", "generate", "--seed", strconv.FormatInt(seed, 10),
			"--owner-user-id", ownerID, "--member-user-id", memberID,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		var got financefixtures.Summary
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, financefixtures.Summary{
			Seed: seed, Scenario: realisticScenarioName, ScenarioIDs: []string{"realistic-core"},
		}, got)

		runtimeConfig, err := resolveFinanceFixturesRuntimeConfig(rootCmd)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, closeFinanceFixturesRuntimeConfig(nil, runtimeConfig)) })
		store := persistence.NewStore(runtimeConfig.Database)
		tenantService := financepkg.NewTenantService(store)
		ownerTenants, err := tenantService.ListTenantsForUser(t.Context(), ownerID)
		require.NoError(t, err)
		require.Len(t, ownerTenants, 1)
		reportingService := financepkg.NewReportingService(store)
		dashboard, err := reportingService.GetDashboard(t.Context(), financepkg.DashboardParams{
			ActorUserID: ownerID,
			TenantID:    ownerTenants[0].Tenant.ID,
			Preset:      financepkg.DashboardPeriodPresetCustom,
			StartDate:   time.Now().AddDate(-5, 0, 0),
			EndDate:     time.Now().AddDate(1, 0, 0),
		})
		require.NoError(t, err)
		assert.True(t, dashboard.Settled.Complete)
		assert.True(t, dashboard.Pending.Complete)
		assert.Empty(t, dashboard.MissingFX)
	})

	t.Run("generation never calls a configured live monobank", func(t *testing.T) {
		fake := faker.New()
		liveCalls := 0
		liveServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { liveCalls++ }))
		defer liveServer.Close()
		runtimeConfig := makeRuntimeConfig(t)
		runtimeConfig.MonobankBaseURL = liveServer.URL
		_, err := runFinanceFixturesGenerate(t.Context(), runtimeConfig, financeFixturesGenerateParams{
			Seed: int64(1_000_000 + fake.Int()), Scenario: realisticScenarioName,
			Now:         time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
			OwnerUserID: "owner-" + fake.UUID().V4(), MemberUserID: "member-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		assert.Zero(t, liveCalls)
	})

	t.Run("resolve finance fixtures runtime config uses checked-in test configuration", func(t *testing.T) {
		fake := faker.New()
		t.Chdir("../..")
		t.Setenv("APP_FINANCE_PROVIDERS_MONOBANK_BASEURL", "https://"+fake.Internet().Domain())
		t.Setenv("APP_AUTH_JWTSIGNINGKEY", fake.UUID().V4())
		rootCmd := newRootCmd()
		require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))
		runtimeConfig, err := resolveFinanceFixturesRuntimeConfig(rootCmd)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, closeFinanceFixturesRuntimeConfig(nil, runtimeConfig)) })
		assert.NotNil(t, runtimeConfig.Database)
		assert.NotNil(t, runtimeConfig.JobsStore)
		assert.Equal(t, os.Getenv("APP_FINANCE_PROVIDERS_MONOBANK_BASEURL"), runtimeConfig.MonobankBaseURL)
		assert.Equal(t, os.Getenv("APP_AUTH_JWTSIGNINGKEY"), runtimeConfig.JWTSigningKey)
	})

	t.Run("resolve finance fixtures runtime config preserves test defaults", func(t *testing.T) {
		t.Chdir("../..")
		rootCmd := newRootCmd()
		require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))
		runtimeConfig, err := resolveFinanceFixturesRuntimeConfig(rootCmd)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, closeFinanceFixturesRuntimeConfig(nil, runtimeConfig)) })
		assert.NotNil(t, runtimeConfig.Database)
		assert.NotNil(t, runtimeConfig.JobsStore)
		assert.NotEmpty(t, runtimeConfig.JWTSigningKey)
	})

	t.Run("reports invalid runtime resolution and generation input", func(t *testing.T) {
		_, err := resolveFinanceFixturesRuntimeConfig(&cobra.Command{})
		require.Error(t, err)
		fake := faker.New()
		runtimeConfig := makeRuntimeConfig(t)
		_, err = runFinanceFixturesGenerate(t.Context(), runtimeConfig, financeFixturesGenerateParams{
			Seed: 3, Scenario: "unsupported-" + fake.Lorem().Word(), Now: time.Now(),
		})
		require.ErrorContains(t, err, "unsupported finance fixture scenario")
		_, err = runFinanceFixturesGenerate(t.Context(), runtimeConfig, financeFixturesGenerateParams{
			Seed: 8, Scenario: realisticScenarioName, Now: time.Now(),
			ConnectionProvider: "unsupported-" + fake.Lorem().Word(),
		})
		require.ErrorContains(t, err, "unsupported finance fixture connection provider")
	})

	t.Run("finance fixture module validates composition inputs", func(t *testing.T) {
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

	t.Run("finance fixture monobank server serves only fixture paths", func(t *testing.T) {
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

	t.Run("fixture provider exposes deterministic banking behavior", func(t *testing.T) {
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

	t.Run("joins fixture root close errors with command errors", func(t *testing.T) {
		closeErr := assert.AnError
		err := closeFinanceFixturesRuntimeConfig(
			closeErr,
			financeFixturesRuntimeConfig{close: func() error { return closeErr }},
		)
		require.ErrorIs(t, err, closeErr)
		require.NoError(t, closeFinanceFixturesRuntimeConfig(nil, financeFixturesRuntimeConfig{}))
	})
}
