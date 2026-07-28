package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/domain"
	financefixtures "github.com/gemyago/signal-foundry/finance/fixtures"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestFinanceCommand(t *testing.T) {
	fake := faker.New()
	makeRuntimeConfig := func(t *testing.T, dsn string) financeFixturesRuntimeConfig {
		t.Helper()
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		database, err := persistence.NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		jobsStore, err := jobspkg.NewStore(
			sqlDB,
			dsn,
			jobspkg.StoreOpts{TablePrefix: "signal_foundry_jobs_"},
		)
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

	t.Run("wires finance fixtures generate command", func(t *testing.T) {
		rootCmd, _ := makeRootCmd(t, financeFixturesCommandDeps{})
		financeCmd, _, err := rootCmd.Find([]string{"finance", "fixtures", "generate"})
		require.NoError(t, err)
		assert.Equal(t, "generate", financeCmd.Name())
	})

	t.Run("generate emits deterministic summary json", func(t *testing.T) {
		want := financefixtures.Summary{
			Seed:        42,
			Scenario:    "realistic",
			ScenarioIDs: []string{"realistic-core"},
		}
		wantConfig := makeRuntimeConfig(t, filepath.Join(t.TempDir(), fake.UUID().V4()+".db"))
		wantConfig.JWTSigningKey = fake.UUID().V4()
		wantConfig.MonobankBaseURL = "https://" + fake.Internet().Domain()
		rootCmd, stdout := makeRootCmd(t, financeFixturesCommandDeps{
			ResolveRuntimeConfig: func(*cobra.Command) (financeFixturesRuntimeConfig, error) {
				return wantConfig, nil
			},
			Generate: func(
				_ context.Context,
				runtimeConfig financeFixturesRuntimeConfig,
				params financeFixturesGenerateParams,
			) (financefixtures.Summary, error) {
				require.Equal(t, wantConfig, runtimeConfig)
				require.Equal(t, int64(42), params.Seed)
				require.Equal(t, "realistic", params.Scenario)
				require.Equal(t, "owner-1", params.OwnerUserID)
				require.Equal(t, "member-1", params.MemberUserID)
				require.Equal(t, "monobank", params.ConnectionProvider)
				return want, nil
			},
			Now: func() time.Time { return time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC) },
		})
		rootCmd.SetArgs([]string{
			"finance",
			"fixtures",
			"generate",
			"--seed",
			"42",
			"--scenario",
			"realistic",
			"--owner-user-id",
			"owner-1",
			"--member-user-id",
			"member-1",
			"--connection-provider",
			fixtureMonobankProviderName,
		})
		require.NoError(t, rootCmd.Execute())
		var got financefixtures.Summary
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, want, got)
	})

	t.Run("generate surfaces runner errors", func(t *testing.T) {
		rootCmd, _ := makeRootCmd(t, financeFixturesCommandDeps{
			Generate: func(context.Context, financeFixturesRuntimeConfig, financeFixturesGenerateParams) (financefixtures.Summary, error) {
				return financefixtures.Summary{}, assert.AnError
			},
			Now: func() time.Time { return time.Unix(int64(fake.Int()), 0).UTC() },
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

	t.Run("generate command falls back to default runtime generator", func(t *testing.T) {
		databasePath := filepath.Join(t.TempDir(), fake.UUID().V4()+".db")
		rootCmd, stdout := makeRootCmd(t, financeFixturesCommandDeps{
			ResolveRuntimeConfig: func(*cobra.Command) (financeFixturesRuntimeConfig, error) {
				return makeRuntimeConfig(t, databasePath), nil
			},
			Now: func() time.Time { return time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC) },
		})
		rootCmd.SetArgs([]string{"finance", "fixtures", "generate", "--seed", "11"})
		require.NoError(t, rootCmd.Execute())
		var got financefixtures.Summary
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(
			t,
			financefixtures.Summary{
				Seed:        11,
				Scenario:    realisticScenarioName,
				ScenarioIDs: []string{"realistic-core"},
			},
			got,
		)
	})

	t.Run("resolve finance fixtures runtime config uses app config wiring", func(t *testing.T) {
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), "application.db"))
		t.Setenv("APP_APPLICATION_DATABASE_TABLEPREFIX", "application_"+fake.UUID().V4()+"_")
		t.Setenv("APP_FINANCE_PROVIDERS_MONOBANK_BASEURL", "https://"+fake.Internet().Domain())
		t.Setenv("APP_AUTH_JWTSIGNINGKEY", fake.UUID().V4())

		rootCmd := newRootCmd()
		require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))

		runtimeConfig, err := resolveFinanceFixturesRuntimeConfig(rootCmd, dig.New())
		require.NoError(t, err)
		assert.NotNil(t, runtimeConfig.Database)
		assert.NotNil(t, runtimeConfig.JobsStore)
		assert.Equal(
			t,
			os.Getenv("APP_FINANCE_PROVIDERS_MONOBANK_BASEURL"),
			runtimeConfig.MonobankBaseURL,
		)
		assert.Equal(t, os.Getenv("APP_AUTH_JWTSIGNINGKEY"), runtimeConfig.JWTSigningKey)
	})

	t.Run(
		"resolve finance fixtures runtime config preserves default config values",
		func(t *testing.T) {
			t.Chdir(t.TempDir())
			rootCmd := newRootCmd()
			require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))

			runtimeConfig, err := resolveFinanceFixturesRuntimeConfig(rootCmd, dig.New())
			require.NoError(t, err)
			assert.NotNil(t, runtimeConfig.Database)
			assert.NotNil(t, runtimeConfig.JobsStore)
			assert.NotEmpty(t, runtimeConfig.JWTSigningKey)
		},
	)

	t.Run(
		"resolve finance fixtures runtime config surfaces engine bootstrap errors",
		func(t *testing.T) {
			_, err := resolveFinanceFixturesRuntimeConfig(&cobra.Command{}, dig.New())
			require.Error(t, err)
		},
	)

	t.Run("run finance fixtures generate rejects unsupported scenarios", func(t *testing.T) {
		runtimeConfig := makeRuntimeConfig(t, filepath.Join(t.TempDir(), "fixtures.db"))
		_, err := runFinanceFixturesGenerate(
			t.Context(),
			runtimeConfig,
			financeFixturesGenerateParams{
				Seed:     3,
				Scenario: "unsupported-" + fake.Lorem().Word(),
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
			},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported finance fixture scenario")
	})

	t.Run("run finance fixtures generate bootstraps realistic scenario", func(t *testing.T) {
		runtimeConfig := makeRuntimeConfig(t, filepath.Join(t.TempDir(), "fixtures.db"))
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		summary, err := runFinanceFixturesGenerate(
			t.Context(),
			runtimeConfig,
			financeFixturesGenerateParams{
				Seed:         5,
				Scenario:     realisticScenarioName,
				Now:          now,
				OwnerUserID:  "owner-e2e",
				MemberUserID: "member-e2e",
			},
		)
		require.NoError(t, err)
		assert.Equal(t, financefixtures.Summary{
			Seed:        5,
			Scenario:    realisticScenarioName,
			ScenarioIDs: []string{"realistic-core"},
		}, summary)

		require.NoError(t, persistence.NewMigrator(runtimeConfig.Database).Migrate(t.Context()))
		store := persistence.NewStore(runtimeConfig.Database)
		tenantService := financepkg.NewTenantService(
			store,
			financepkg.WithTenantServiceNow(func() time.Time { return now }),
		)
		reportingService := financepkg.NewReportingService(
			store,
			financepkg.WithReportingServiceNow(func() time.Time { return now }),
		)
		ownerTenants, err := tenantService.ListTenantsForUser(t.Context(), "owner-e2e")
		require.NoError(t, err)
		require.Len(t, ownerTenants, 1)

		dashboard, err := reportingService.GetDashboard(t.Context(), financepkg.DashboardParams{
			ActorUserID: "owner-e2e",
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

	t.Run("run finance fixtures generate never calls configured live monobank", func(t *testing.T) {
		liveCalls := 0
		liveServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			liveCalls++
			writer.WriteHeader(http.StatusForbidden)
		}))
		defer liveServer.Close()
		runtimeConfig := makeRuntimeConfig(t, filepath.Join(t.TempDir(), "fixture-safe.sqlite"))
		runtimeConfig.MonobankBaseURL = liveServer.URL
		_, err := runFinanceFixturesGenerate(t.Context(), runtimeConfig, financeFixturesGenerateParams{
			Seed: 7, Scenario: realisticScenarioName,
			Now:         time.Date(2026, time.July, 10, 22, 0, 0, 0, time.FixedZone("fixture", 2*60*60)),
			OwnerUserID: "owner-" + fake.UUID().V4(), MemberUserID: "member-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		assert.Zero(t, liveCalls)
	})

	t.Run("finance fixture monobank server rejects non-fixture paths", func(t *testing.T) {
		server := newFinanceFixturesMonobankServer()
		defer server.Close()
		response, err := server.Client().Get(server.URL + "/personal/statement")
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})

	t.Run("fixture provider exposes deterministic finance provider behavior", func(t *testing.T) {
		provider := financeFixturesProvider{}
		assert.Equal(t, "monobank", provider.Name())

		start, err := provider.StartLink(t.Context(), financepkg.ProviderStartLinkParams{})
		require.NoError(t, err)
		assert.Equal(t, financepkg.ProviderLinkStart{}, start)

		finish, err := provider.FinishLink(t.Context(), financepkg.ProviderFinishLinkParams{})
		require.NoError(t, err)
		assert.Equal(t, financepkg.ProviderLinkResult{}, finish)

		link, err := provider.LinkToken(t.Context(), financepkg.ProviderTokenLinkParams{})
		require.NoError(t, err)
		assert.Equal(t, financepkg.ProviderTokenLinkResult{
			DisplayName:       "Fixture Connection",
			ProviderReference: "fixture-reference",
			ExternalID:        "fixture-external",
			Secret:            "fixture-secret",
			State:             domain.BankConnectionStateActive,
		}, link)

		syncResult, err := provider.Sync(t.Context(), financepkg.ProviderSyncParams{})
		require.NoError(t, err)
		assert.Equal(t, financepkg.ProviderSyncResult{}, syncResult)

		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = provider.LinkToken(canceledCtx, financepkg.ProviderTokenLinkParams{})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("run finance fixtures generate writes to configured application database path", func(t *testing.T) {
		workdir := t.TempDir()
		t.Chdir(workdir)
		require.NoError(t, os.Mkdir(filepath.Join(workdir, "data"), 0o755))
		runtimeConfig := makeRuntimeConfig(t, filepath.Join("data", "application.db"))

		_, err := runFinanceFixturesGenerate(
			t.Context(),
			runtimeConfig,
			financeFixturesGenerateParams{
				Seed:     6,
				Scenario: realisticScenarioName,
				Now:      time.Date(2026, time.June, 21, 12, 0, 0, 0, time.UTC),
			},
		)
		require.NoError(t, err)
		_, statErr := os.Stat(filepath.Join(workdir, "data", "application.db"))
		require.NoError(t, statErr)
	})

	t.Run(
		"run finance fixtures generate rejects unsupported connection provider",
		func(t *testing.T) {
			runtimeConfig := makeRuntimeConfig(t, filepath.Join(t.TempDir(), "fixtures.db"))
			_, err := runFinanceFixturesGenerate(
				t.Context(),
				runtimeConfig,
				financeFixturesGenerateParams{
					Seed:               8,
					Scenario:           realisticScenarioName,
					Now:                time.Date(2026, time.June, 21, 12, 0, 0, 0, time.UTC),
					ConnectionProvider: "unsupported-" + fake.Lorem().Word(),
				},
			)
			require.ErrorContains(t, err, "unsupported finance fixture connection provider")
		},
	)

	t.Run("fixture schedule writer stores generic due schedule rows", func(t *testing.T) {
		dsn := filepath.Join(t.TempDir(), "jobs.db")
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()
		store, err := jobspkg.NewStore(
			sqlDB,
			dsn,
			jobspkg.StoreOpts{TablePrefix: "signal_foundry_jobs_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		writer := fixturesScheduleWriter{store: store}
		nextRunAt := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		require.NoError(
			t,
			writer.UpsertBankConnectionSyncSchedule(
				t.Context(),
				financepkg.BankConnectionSyncSchedule{
					ScheduleID:   "schedule-1",
					ConnectionID: "connection-1",
					ActorUserID:  "user-1",
					Interval:     time.Hour,
					NextRunAt:    &nextRunAt,
					Enabled:      true,
				},
			),
		)
		schedules, err := store.ListDueSchedules(t.Context(), nextRunAt.Add(time.Minute))
		require.NoError(t, err)
		require.Len(t, schedules, 1)
		assert.Equal(t, jobspkg.JobType(financepkg.BankConnectionSyncJobType), schedules[0].JobType)
	})
}
