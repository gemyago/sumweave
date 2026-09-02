package internal

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationComposition(t *testing.T) {
	makeMigrationDeps := func(t *testing.T) DatabaseMigrationDeps {
		t.Helper()
		dsn := filepath.Join(t.TempDir(), "application.sqlite")
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		users, err := auth.NewUserStore(auth.UserStoreDeps{
			SQLDB:       db,
			DatabaseDSN: dsn,
			TablePrefix: "app_auth_",
			IDGen:       ident.NewDefaultGenerator(),
			Logger:      slog.Default(),
		})
		require.NoError(t, err)
		refresh, err := auth.NewRefreshTokenStore(auth.RefreshTokenStoreDeps{
			SQLDB: db, DatabaseDSN: dsn, TablePrefix: "app_auth_", Logger: slog.Default(),
		})
		require.NoError(t, err)
		return DatabaseMigrationDeps{
			RootLogger: telemetry.RootTestLogger(), AgentRuntimeStorageType: "file", ApplicationDatabaseDSN: dsn,
			ApplicationDatabaseTablePrefix: "app_", ApplicationSQLDB: db, AuthUsers: users, AuthRefreshTokens: refresh,
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
			AgentRuntimeDatabaseDSN:         "",
			AgentRuntimeDatabaseTablePrefix: "agent_",
			SkillsMaxSkillBytes:             4096,
			SkillsMaxCatalogEntries:         10,
			ToolsRegistry:                   agent.NewToolsRegistry(),
		}
	}

	t.Run("application migration composes application schemas with file runtime storage", func(t *testing.T) {
		deps := makeMigrationDeps(t)
		migrator := NewDatabaseMigrator(deps)
		require.NoError(t, migrator.Migrate(t.Context()))
		err := migrator.runStep(t.Context(), "finance", func(context.Context) error { return errors.New("failed") })
		require.ErrorIs(t, err, errors.Unwrap(err))
		require.ErrorContains(t, err, "migrate finance schema")
		assert.NotEmpty(t, appdispatch.Config{TablePrefix: "app_"}.MessagesTable())
	})

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
			services, err := newRuntimeServices(fileDeps)
			require.NoError(t, err)
			assert.NotNil(t, services.agentProfilesSvc)
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
			databaseOptsDeps := makeRuntimeDeps(t, storageTypeDatabase)
			databaseOptsDeps.AgentRuntimeDatabaseDSN = "database-runtime"
			_, err = buildRunnerOpts(databaseOptsDeps, agent.NewToolsRegistry())
			require.NoError(t, err)
			databaseServicesDeps := makeRuntimeDeps(t, storageTypeDatabase)
			databaseServicesDeps.AgentRuntimeDatabaseDSN = "database-runtime"
			databaseFactory := newMockdatabaseRuntimeServiceFactory(t)
			databaseProviders, err := agent.NewFileProvidersConfigService(t.TempDir(), telemetry.RootTestLogger())
			require.NoError(t, err)
			databaseProfiles, err := agent.NewFileAgentProfilesService(t.TempDir(), telemetry.RootTestLogger())
			require.NoError(t, err)
			databaseFactory.EXPECT().
				NewProvidersConfigService(
					databaseServicesDeps.AgentRuntimeDatabaseDSN,
					databaseServicesDeps.RootLogger,
					databaseServicesDeps.AgentRuntimeDatabaseTablePrefix,
				).
				Return(databaseProviders, nil).
				Once()
			databaseFactory.EXPECT().
				NewAgentProfilesService(
					databaseServicesDeps.AgentRuntimeDatabaseDSN,
					databaseServicesDeps.RootLogger,
					databaseServicesDeps.AgentRuntimeDatabaseTablePrefix,
				).
				Return(databaseProfiles, nil).
				Once()
			services, err = newRuntimeServicesWithFactory(databaseServicesDeps, databaseFactory)
			require.NoError(t, err)
			assert.Same(t, databaseProviders, services.providersConfigSvc)
			assert.Same(t, databaseProfiles, services.agentProfilesSvc)
			databaseFactory = newMockdatabaseRuntimeServiceFactory(t)
			databaseFactory.EXPECT().
				NewProvidersConfigService(
					databaseServicesDeps.AgentRuntimeDatabaseDSN,
					databaseServicesDeps.RootLogger,
					databaseServicesDeps.AgentRuntimeDatabaseTablePrefix,
				).
				Return(nil, errors.New("providers failure")).
				Once()
			_, err = newProvidersConfigServiceWithFactory(databaseServicesDeps, databaseFactory)
			require.Error(t, err)
			databaseFactory = newMockdatabaseRuntimeServiceFactory(t)
			databaseFactory.EXPECT().
				NewAgentProfilesService(
					databaseServicesDeps.AgentRuntimeDatabaseDSN,
					databaseServicesDeps.RootLogger,
					databaseServicesDeps.AgentRuntimeDatabaseTablePrefix,
				).
				Return(nil, errors.New("profiles failure")).
				Once()
			_, err = newAgentProfilesServiceWithFactory(databaseServicesDeps, databaseFactory)
			require.Error(t, err)
			_, err = NewRuntime(fileDeps)
			require.NoError(t, err)
			invalidWorkspaceDeps := makeRuntimeDeps(t, "file")
			invalidWorkspaceDeps.PlatformAgentsPath = filepath.Join(t.TempDir(), "missing-platform-agents")
			_, err = NewRuntime(invalidWorkspaceDeps)
			require.Error(t, err)
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

	t.Run("application database registers lifecycle cleanup", func(t *testing.T) {
		hooks := lifecycle.NewTestShutdownHooks()
		db, err := NewApplicationSQLDB(filepath.Join(t.TempDir(), "application.sqlite"), hooks)
		require.NoError(t, err)
		require.NotNil(t, db)
	})

	t.Run("application database constructor returns open errors", func(t *testing.T) {
		_, err := NewApplicationSQLDB("", lifecycle.NewTestShutdownHooks())
		require.Error(t, err)
	})
}
