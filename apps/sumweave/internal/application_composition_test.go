package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/agent"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationComposition(t *testing.T) {
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

	t.Run("application database constructor returns open errors", func(t *testing.T) {
		_, err := NewApplicationSQLDB("", nil)
		require.Error(t, err)
	})

	t.Run("application database rejects SQLite configuration", func(t *testing.T) {
		_, err := OpenApplicationSQLDB(":memory:")
		require.Error(t, err)
	})

	t.Run("application composition validates direct migration dependencies", func(t *testing.T) {
		migrator := &DatabaseMigrator{rootLogger: telemetry.RootTestLogger()}
		require.NoError(t, migrator.migrateAgentRuntime(t.Context()))
		require.Error(t, migrator.migrateAuthentication(t.Context()))
		require.Error(t, migrator.migrateAppDispatch(t.Context()))
		require.Error(t, migrator.migrateJobs(t.Context()))
		require.Error(t, migrator.migrateFinance(t.Context()))
		migrator.agentRuntimeStorageType = storageTypeDatabase
		require.Error(t, migrator.migrateAgentRuntime(t.Context()))
		require.Error(t, migrator.Migrate(t.Context()))
		expectedErr := errors.New(faker.New().UUID().V4())
		err := migrator.runStep(t.Context(), "finance", func(context.Context) error { return expectedErr })
		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "migrate finance schema")

		invalidDatabaseDeps := makeRuntimeDeps(t, storageTypeDatabase)
		invalidDatabaseDeps.AgentRuntimeDatabaseDSN = ":memory:"
		_, err = newProvidersConfigService(invalidDatabaseDeps)
		require.Error(t, err)
	})

	t.Run("application database opens the prepared runtime-role schema and runs migrations", func(t *testing.T) {
		values, err := config.LoadValues(config.ValuesLoadInput{Environment: "test"})
		require.NoError(t, err)
		rootConfig, err := values.HTTPRoot("test")
		require.NoError(t, err)
		migrationDSN := strings.Replace(
			rootConfig.Application.Database.DSN,
			"sumweave_runtime:sumweave_runtime_local",
			"sumweave_migrator:sumweave_migrator_local",
			1,
		)
		deps := RuntimeDeps{
			RootLogger:                      telemetry.RootTestLogger(),
			DataDir:                         t.TempDir(),
			PlatformAgentsPath:              t.TempDir(),
			AgentRuntimeStorageType:         rootConfig.AgentRuntime.Storage.Type,
			AgentRuntimeDatabaseDSN:         migrationDSN,
			AgentRuntimeDatabaseTablePrefix: rootConfig.AgentRuntime.Database.TablePrefix,
			SkillsMaxSkillBytes:             4096,
			SkillsMaxCatalogEntries:         10,
			ToolsRegistry:                   agent.NewToolsRegistry(),
		}
		runtime, err := newRuntime(deps)
		require.NoError(t, err)
		assert.NotNil(t, runtime.Runner)
		services, err := newRuntimeServices(deps)
		require.NoError(t, err)
		assert.NotNil(t, services.agentProfilesSvc)
		hooks := lifecycle.NewTestShutdownHooks()
		database, err := NewApplicationSQLDB(migrationDSN, hooks)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, hooks.PerformShutdown(context.WithoutCancel(t.Context()))) })
		require.NotNil(t, database)
		users, err := auth.NewUserStore(auth.UserStoreDeps{
			SQLDB:       database,
			DatabaseDSN: migrationDSN,
			TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_",
			IDGen:       ident.NewDefaultGenerator(),
			Logger:      telemetry.RootTestLogger(),
		})
		require.NoError(t, err)
		refreshTokens, err := auth.NewRefreshTokenStore(auth.RefreshTokenStoreDeps{
			SQLDB:       database,
			DatabaseDSN: migrationDSN,
			TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_",
			Logger:      telemetry.RootTestLogger(),
		})
		require.NoError(t, err)
		migrator := NewDatabaseMigrator(DatabaseMigrationDeps{
			RootLogger:                      telemetry.RootTestLogger(),
			AgentRuntimeStorageType:         rootConfig.AgentRuntime.Storage.Type,
			AgentRuntimeDatabaseDSN:         migrationDSN,
			AgentRuntimeDatabaseTablePrefix: rootConfig.AgentRuntime.Database.TablePrefix,
			ApplicationDatabaseDSN:          migrationDSN,
			ApplicationDatabaseTablePrefix:  rootConfig.Application.Database.TablePrefix,
			ApplicationSQLDB:                database,
			AuthUsers:                       users,
			AuthRefreshTokens:               refreshTokens,
		})
		require.NoError(t, migrator.Migrate(t.Context()))
		canceledContext, cancel := context.WithCancel(t.Context())
		cancel()
		require.Error(t, migrator.migrateAppDispatch(canceledContext))
		require.Error(t, migrator.migrateFinance(canceledContext))
		runtimeRoleMigrator := &DatabaseMigrator{
			rootLogger:                      telemetry.RootTestLogger(),
			agentRuntimeStorageType:         storageTypeDatabase,
			agentRuntimeDatabaseDSN:         rootConfig.AgentRuntime.Database.DSN,
			agentRuntimeDatabaseTablePrefix: rootConfig.AgentRuntime.Database.TablePrefix,
		}
		require.Error(t, runtimeRoleMigrator.migrateAgentRuntime(t.Context()))
		usersDatabase, err := OpenApplicationSQLDB(migrationDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, usersDatabase.Close()) })
		refreshTokensDatabase, err := OpenApplicationSQLDB(migrationDSN)
		require.NoError(t, err)
		users, err = auth.NewUserStore(auth.UserStoreDeps{
			SQLDB:       usersDatabase,
			DatabaseDSN: migrationDSN,
			TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_",
			IDGen:       ident.NewDefaultGenerator(),
			Logger:      telemetry.RootTestLogger(),
		})
		require.NoError(t, err)
		refreshTokens, err = auth.NewRefreshTokenStore(auth.RefreshTokenStoreDeps{
			SQLDB:       refreshTokensDatabase,
			DatabaseDSN: migrationDSN,
			TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_",
			Logger:      telemetry.RootTestLogger(),
		})
		require.NoError(t, err)
		require.NoError(t, refreshTokensDatabase.Close())
		migrator = NewDatabaseMigrator(DatabaseMigrationDeps{
			RootLogger: telemetry.RootTestLogger(), AuthUsers: users, AuthRefreshTokens: refreshTokens,
		})
		require.Error(t, migrator.migrateAuthentication(t.Context()))
		require.NoError(t, database.Close())
		require.Error(t, migrator.migrateAuthentication(t.Context()))
		require.Error(t, migrator.migrateAppDispatch(t.Context()))
		require.Error(t, migrator.migrateJobs(t.Context()))
		require.Error(t, migrator.migrateFinance(t.Context()))
	})
}
