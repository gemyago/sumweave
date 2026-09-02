//go:build postgres_test

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
	financefixtures "github.com/gemyago/sumweave/finance/fixtures"
	"github.com/gemyago/sumweave/finance/persistence"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinanceCommandPostgres(t *testing.T) {
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
	makeRootCmd := func(t *testing.T, deps financeFixturesCommandDeps) (*cobra.Command, *bytes.Buffer) {
		t.Helper()
		rootCmd := newRootCmd()
		stdout := &bytes.Buffer{}
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stdout)
		rootCmd.AddCommand(newFinanceCmd(deps))
		return rootCmd, stdout
	}
	noOpPrepare := func(context.Context, financeFixturesRuntimeConfig) error { return nil }

	t.Run("generate uses prepared schemas without migration and scopes generated data to its owner", func(t *testing.T) {
		fake := faker.New()
		runtimeConfig := makeRuntimeConfig(t)
		runtimeConfig.JWTSigningKey = fake.UUID().V4()
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		rootCmd, stdout := makeRootCmd(t, financeFixturesCommandDeps{
			ResolveRuntimeConfig: func(*cobra.Command) (financeFixturesRuntimeConfig, error) { return runtimeConfig, nil },
			Prepare:              noOpPrepare,
			Now:                  func() time.Time { return now },
		})
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

		store := persistence.NewStore(runtimeConfig.Database)
		tenantService := financepkg.NewTenantService(
			store,
			financepkg.WithTenantServiceNow(func() time.Time { return now }),
		)
		ownerTenants, err := tenantService.ListTenantsForUser(t.Context(), ownerID)
		require.NoError(t, err)
		require.Len(t, ownerTenants, 1)

		reportingService := financepkg.NewReportingService(
			store,
			financepkg.WithReportingServiceNow(func() time.Time { return now }),
		)
		dashboard, err := reportingService.GetDashboard(t.Context(), financepkg.DashboardParams{
			ActorUserID: ownerID,
			TenantID:    ownerTenants[0].Tenant.ID,
			Preset:      financepkg.DashboardPeriodPresetCustom,
			StartDate:   time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC),
			EndDate:     time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC),
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
		seed := int64(1_000_000 + fake.Int())
		_, err := runFinanceFixturesGenerate(t.Context(), runtimeConfig, financeFixturesGenerateParams{
			Seed: seed, Scenario: realisticScenarioName,
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
}
