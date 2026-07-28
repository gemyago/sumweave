package internal

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/auth"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestApplicationComposition(t *testing.T) {
	makeMigrationDeps := func(t *testing.T) DatabaseMigrationDeps {
		t.Helper()
		dsn := filepath.Join(t.TempDir(), "application.sqlite")
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		users, err := auth.NewUserStore(
			auth.UserStoreDeps{
				SQLDB:       db,
				DatabaseDSN: dsn,
				TablePrefix: "app_auth_",
				IDGen:       ident.NewDefaultGenerator(),
				Logger:      slog.Default(),
			},
		)
		require.NoError(t, err)
		refresh, err := auth.NewRefreshTokenStore(
			auth.RefreshTokenStoreDeps{
				SQLDB:       db,
				DatabaseDSN: dsn,
				TablePrefix: "app_auth_",
				Logger:      slog.Default(),
			},
		)
		require.NoError(t, err)
		return DatabaseMigrationDeps{
			RootLogger:                      telemetry.RootTestLogger(),
			AgentRuntimeStorageType:         storageTypeDatabase,
			AgentRuntimeDatabaseDSN:         filepath.Join(t.TempDir(), "agent.sqlite"),
			AgentRuntimeDatabaseTablePrefix: "agent_",
			ApplicationDatabaseDSN:          dsn,
			ApplicationDatabaseTablePrefix:  "app_",
			ApplicationSQLDB:                db,
			AuthUsers:                       users,
			AuthRefreshTokens:               refresh,
		}
	}
	makeRuntimeDeps := func(t *testing.T, storage string) RuntimeDeps {
		t.Helper()
		dataDir, platformDir := t.TempDir(), t.TempDir()
		return RuntimeDeps{
			RootLogger:                      telemetry.RootTestLogger(),
			DataDir:                         dataDir,
			PlatformAgentsPath:              platformDir,
			AgentRuntimeStorageType:         storage,
			AgentRuntimeDatabaseDSN:         filepath.Join(t.TempDir(), "agent.sqlite"),
			AgentRuntimeDatabaseTablePrefix: "agent_",
			SkillsMaxSkillBytes:             4096,
			SkillsMaxCatalogEntries:         10,
			ToolsRegistry:                   agent.NewToolsRegistry(),
		}
	}

	t.Run(
		"database migration composes agent auth dispatch jobs and finance schemas",
		func(t *testing.T) {
			deps := makeMigrationDeps(t)
			migrator := newDatabaseMigrator(deps)
			require.NoError(t, migrator.Migrate(t.Context()))
			require.NoError(t, migrator.Migrate(t.Context()))
			for _, table := range []string{"app_auth_auth_users", "app_auth_auth_refresh_tokens", appdispatch.Config{TablePrefix: "app_"}.MessagesTable(), "app_jobs_jobs", "app_jobs_job_schedules", "finance_tenants"} {
				var name string
				require.NoError(
					t,
					deps.ApplicationSQLDB.QueryRowContext(t.Context(), `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).
						Scan(&name),
				)
				assert.Equal(t, table, name)
			}
			err := migrator.runStep(
				t.Context(),
				"finance",
				func(context.Context) error { return errors.New("failed") },
			)
			require.ErrorIs(t, err, errors.Unwrap(err))
			assert.ErrorContains(t, err, "migrate finance schema")
		},
	)

	t.Run(
		"agent runtime supports file and database persistence with workspace and skills",
		func(t *testing.T) {
			fileDeps := makeRuntimeDeps(t, "file")
			runtime, err := newRuntime(fileDeps)
			require.NoError(t, err)
			assert.NotNil(t, runtime.HTTPHandler)
			assert.NotNil(t, runtime.Runner)
			assert.Same(t, fileDeps.ToolsRegistry, runtime.ToolsRegistry)
			_, err = os.Stat(filepath.Join(fileDeps.DataDir, "agent-temp"))
			require.NoError(t, err)
			providers, err := newProvidersConfigService(fileDeps)
			require.NoError(t, err)
			assert.NotNil(t, providers)
			profiles, err := newAgentProfilesService(fileDeps)
			require.NoError(t, err)
			assert.NotNil(t, profiles)
			opts, err := workspacefsRegisterOptions(fileDeps)
			require.NoError(t, err)
			assert.Len(t, opts, 2)
			fileDeps.SkillsEnabled = true
			skillDir := filepath.Join(t.TempDir(), "finance-skill")
			require.NoError(t, os.MkdirAll(skillDir, 0o755))
			require.NoError(
				t,
				os.WriteFile(
					filepath.Join(skillDir, "SKILL.md"),
					[]byte("---\nname: finance-skill\ndescription: finance\n---\n"),
					0o644,
				),
			)
			fileDeps.SkillsPaths = []string{filepath.Dir(skillDir)}
			_, err = buildRunnerOpts(fileDeps, agent.NewToolsRegistry())
			require.NoError(t, err)
			databaseDeps := makeRuntimeDeps(t, storageTypeDatabase)
			_, err = newRuntime(databaseDeps)
			require.NoError(t, err)
			services, err := newRuntimeServices(databaseDeps)
			require.NoError(t, err)
			assert.NotNil(t, services.agentProfilesSvc)
			badDataDir := filepath.Join(t.TempDir(), "not-a-directory")
			require.NoError(t, os.WriteFile(badDataDir, []byte("x"), 0o600))
			badDeps := makeRuntimeDeps(t, "file")
			badDeps.DataDir = badDataDir
			_, err = workspacefsRegisterOptions(badDeps)
			require.Error(t, err)
			badDeps = makeRuntimeDeps(t, storageTypeDatabase)
			badDeps.AgentRuntimeDatabaseDSN = ""
			_, err = newProvidersConfigService(badDeps)
			require.Error(t, err)
			_, err = newAgentProfilesService(badDeps)
			require.Error(t, err)
		},
	)

	t.Run("engine options configure application database lifecycle", func(t *testing.T) {
		cfg, container := viper.New(), dig.New()
		engineCfg := &EngineCfg{}
		engineCfg.Apply(
			WithEngineConfig(cfg),
			WithEngineContainer(container),
			WithEngineJobsWorkerAutoStart(false),
		)
		assert.Same(t, cfg, engineCfg.Config)
		assert.Same(t, container, engineCfg.Container)
		require.NotNil(t, engineCfg.JobsWorkerAutoStart)
		hooks := lifecycle.NewTestShutdownHooks()
		db, err := newApplicationSQLDB(
			applicationDatabaseDeps{
				DatabaseDSN:   filepath.Join(t.TempDir(), "application.sqlite"),
				ShutdownHooks: hooks,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, db)
	})

	t.Run("application database constructor returns open errors", func(t *testing.T) {
		_, err := newApplicationSQLDB(
			applicationDatabaseDeps{
				DatabaseDSN:   "",
				ShutdownHooks: lifecycle.NewTestShutdownHooks(),
			},
		)
		require.Error(t, err)
	})
}
