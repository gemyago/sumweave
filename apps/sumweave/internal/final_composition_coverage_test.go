package internal

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/agent"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestFinalAgentRuntimeCoverage(t *testing.T) {
	fake := faker.New()
	makeDeps := func(t *testing.T) RuntimeDeps {
		t.Helper()
		return RuntimeDeps{
			RootLogger:                      telemetry.RootTestLogger(),
			DataDir:                         t.TempDir(),
			PlatformAgentsPath:              t.TempDir(),
			AgentRuntimeStorageType:         "file",
			AgentRuntimeDatabaseTablePrefix: "agent_",
			SkillsMaxSkillBytes:             1024,
			SkillsMaxCatalogEntries:         4,
			ToolsRegistry:                   agent.NewToolsRegistry(),
		}
	}
	t.Run("registers optional execution tools and stops before invalid file services", func(t *testing.T) {
		deps := makeDeps(t)
		deps.ExecEnabled, deps.ExecMaxOutputBytes, deps.ExecDefaultTimeout, deps.ExecMaxConcurrentJobs = true, 1024, time.Second, 1
		opts, err := workspacefsRegisterOptions(deps)
		require.NoError(t, err)
		require.Len(t, opts, 3)
		deps.DataDir = filepath.Join(t.TempDir(), fake.UUID().V4())
		require.NoError(t, os.WriteFile(deps.DataDir, []byte(fake.UUID().V4()), 0o600))
		_, err = newRuntime(deps)
		require.Error(t, err)
	})
	t.Run(
		"builds a catalog from an absent optional skills root and rejects invalid database runtime",
		func(t *testing.T) {
			deps := makeDeps(t)
			deps.SkillsEnabled = true
			deps.SkillsPaths = []string{filepath.Join(t.TempDir(), fake.UUID().V4())}
			_, err := buildRunnerOpts(deps, agent.NewToolsRegistry())
			require.NoError(t, err)
			deps = makeDeps(t)
			deps.AgentRuntimeStorageType = storageTypeDatabase
			_, err = newRuntime(deps)
			require.Error(t, err)
			filePath := filepath.Join(t.TempDir(), fake.UUID().V4())
			require.NoError(t, os.WriteFile(filePath, []byte(fake.UUID().V4()), 0o600))
			fileDeps := makeDeps(t)
			fileDeps.DataDir = filePath
			_, err = newProvidersConfigService(fileDeps)
			require.Error(t, err)
			_, err = newAgentProfilesService(fileDeps)
			require.Error(t, err)
		},
	)
}

func TestFinalDatabaseMigratorCoverage(t *testing.T) {
	fake := faker.New()
	makeMigrator := func(t *testing.T) *DatabaseMigrator {
		t.Helper()
		dsn := filepath.Join(t.TempDir(), "application.sqlite")
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		users, err := auth.NewUserStore(auth.UserStoreDeps{
			SQLDB: db, DatabaseDSN: dsn, IDGen: ident.NewDefaultGenerator(), Logger: slog.Default(),
		})
		require.NoError(t, err)
		refresh, err := auth.NewRefreshTokenStore(auth.RefreshTokenStoreDeps{
			SQLDB: db, DatabaseDSN: dsn, Logger: slog.Default(),
		})
		require.NoError(t, err)
		return newDatabaseMigrator(DatabaseMigrationDeps{
			RootLogger:                     slog.Default(),
			AgentRuntimeStorageType:        "file",
			ApplicationDatabaseDSN:         dsn,
			ApplicationDatabaseTablePrefix: "final_",
			ApplicationSQLDB:               db,
			AuthUsers:                      users,
			AuthRefreshTokens:              refresh,
		})
	}
	t.Run("skips file agent schema and reports component errors", func(t *testing.T) {
		migrator := makeMigrator(t)
		require.NoError(t, migrator.migrateAgentRuntime(t.Context()))
		migrator.authUsers = nil
		require.Error(t, migrator.migrateAuthentication(t.Context()))
		migrator.authUsers, migrator.authRefreshTokens = nil, nil
		err := migrator.Migrate(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "migrate authentication schema")
	})

	t.Run("reports unavailable database agent persistence", func(t *testing.T) {
		migrator := &DatabaseMigrator{rootLogger: slog.Default(), agentRuntimeStorageType: storageTypeDatabase}
		require.Error(t, migrator.migrateAgentRuntime(t.Context()))
	})

	t.Run("wraps agent session migration failures on readonly persistence", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "agent-readonly.sqlite")
		require.NoError(t, os.WriteFile(path, nil, 0o600))
		migrator := &DatabaseMigrator{
			rootLogger:                      slog.Default(),
			agentRuntimeStorageType:         storageTypeDatabase,
			agentRuntimeDatabaseDSN:         "file:" + path + "?mode=ro",
			agentRuntimeDatabaseTablePrefix: "agent_",
		}
		require.Error(t, migrator.migrateAgentRuntime(t.Context()))
	})
	t.Run("wraps app dispatch and jobs construction errors", func(t *testing.T) {
		migrator := &DatabaseMigrator{}
		migrator.rootLogger = slog.Default()
		migrator.applicationDatabaseDSN = " "
		migrator.applicationSQLDB = nil
		migrator.applicationDatabaseTablePrefix = fake.UUID().V4()
		require.Error(t, migrator.migrateAppDispatch(t.Context()))
		require.Error(t, migrator.migrateJobs(t.Context()))
		require.Error(t, migrator.migrateFinance(t.Context()))
	})

	t.Run("propagates concrete storage migration failures", func(t *testing.T) {
		migrator := makeMigrator(t)
		require.NoError(t, migrator.applicationSQLDB.Close())
		require.Error(t, migrator.migrateAuthentication(t.Context()))
		require.Error(t, migrator.migrateAppDispatch(t.Context()))
		require.Error(t, migrator.migrateJobs(t.Context()))
		require.Error(t, migrator.migrateFinance(t.Context()))
	})

	t.Run("wraps durable jobs and finance migration execution failures", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "readonly.sqlite")
		require.NoError(t, os.WriteFile(path, nil, 0o600))
		dsn := "file:" + path + "?mode=ro"
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		migrator := &DatabaseMigrator{
			rootLogger: slog.Default(), applicationSQLDB: db, applicationDatabaseDSN: dsn,
		}
		require.Error(t, migrator.migrateJobs(t.Context()))
		require.Error(t, migrator.migrateFinance(t.Context()))
	})

	t.Run("reports refresh migration failures after users are migrated", func(t *testing.T) {
		usersDSN := filepath.Join(t.TempDir(), "users.sqlite")
		usersDB, err := sqlconn.Open(usersDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, usersDB.Close()) })
		users, err := auth.NewUserStore(auth.UserStoreDeps{
			SQLDB: usersDB, DatabaseDSN: usersDSN, IDGen: ident.NewDefaultGenerator(), Logger: slog.Default(),
		})
		require.NoError(t, err)
		refreshDSN := filepath.Join(t.TempDir(), "refresh.sqlite")
		refreshDB, err := sqlconn.Open(refreshDSN)
		require.NoError(t, err)
		refresh, err := auth.NewRefreshTokenStore(auth.RefreshTokenStoreDeps{
			SQLDB: refreshDB, DatabaseDSN: refreshDSN, Logger: slog.Default(),
		})
		require.NoError(t, err)
		require.NoError(t, refreshDB.Close())
		migrator := &DatabaseMigrator{
			rootLogger: slog.Default(), authUsers: users, authRefreshTokens: refresh,
		}
		require.Error(t, migrator.migrateAuthentication(t.Context()))
	})
}
