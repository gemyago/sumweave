package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/auth"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/gemyago/signal-foundry/runtime/audit"
	"github.com/gemyago/signal-foundry/runtime/backtest"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/execution"
	rtgovernor "github.com/gemyago/signal-foundry/runtime/governor"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

//nolint:gochecknoglobals // Shared migrated template avoids repeated full schema setup in cmd package tests.
var appDatabaseTemplate struct {
	once sync.Once
	path string
	err  error
}

func testLogFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.log")
}

func runDatabaseMigrateCommand(t *testing.T) {
	t.Helper()
	rootCmd := setupCommands()
	rootCmd.SetArgs([]string{"db-migrate", "-e", "test", "--logs-file", testLogFile(t)})
	require.NoError(t, rootCmd.ExecuteContext(t.Context()))
}

func migrateAppDatabaseForTests(t *testing.T, dsn string) {
	t.Helper()
	templatePath := appDatabaseTemplatePath(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(dsn), 0o755))
	require.NoError(t, os.RemoveAll(dsn))

	source, err := os.Open(templatePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, source.Close())
	})

	target, err := os.Create(dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, target.Close())
	})
	_, err = io.Copy(target, source)
	require.NoError(t, err)
	require.NoError(t, target.Sync())
}

func appDatabaseTemplatePath(t *testing.T) string {
	t.Helper()
	appDatabaseTemplate.once.Do(func() {
		//nolint:usetesting // Template must outlive the first test that initializes it.
		templateDir, err := os.MkdirTemp("", "signal-foundry-cmd-test-db-*")
		if err != nil {
			appDatabaseTemplate.err = err
			return
		}
		appDatabaseTemplate.path = filepath.Join(templateDir, "template.sqlite")
		appDatabaseTemplate.err = initializeAppDatabaseTemplate(appDatabaseTemplate.path)
	})
	require.NoError(t, appDatabaseTemplate.err)
	return appDatabaseTemplate.path
}

func initializeAppDatabaseTemplate(dsn string) error {
	sqlDB, err := sqlconn.Open(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	if err = appdispatch.AutoMigrate(context.Background(), appdispatch.Config{
		DatabaseDSN: dsn,
		TablePrefix: "signal_foundry_data_",
	}, sqlDB); err != nil {
		return err
	}

	if migrateErr := runAppDatabaseMigrations(dsn); migrateErr != nil {
		return migrateErr
	}

	return checkpointSQLiteTemplate(dsn)
}

func checkpointSQLiteTemplate(dsn string) error {
	db, err := sqlconn.Open(dsn)
	if err != nil {
		return fmt.Errorf("open sqlite template checkpoint handle: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	if _, err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint sqlite template wal: %w", err)
	}

	return nil
}

func runAppDatabaseMigrations(dsn string) error {
	sqlDB, err := sqlconn.Open(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	if err = appdispatch.AutoMigrate(context.Background(), appdispatch.Config{
		DatabaseDSN: dsn,
		TablePrefix: "signal_foundry_data_",
	}, sqlDB); err != nil {
		return err
	}

	dataStore, err := data.NewDatabaseStore(sqlDB, dsn, data.DatabaseStoreOpts{
		TablePrefix: "signal_foundry_data_",
	})
	if err != nil {
		return err
	}
	migrateErr := dataStore.AutoMigrate()
	if migrateErr != nil {
		return migrateErr
	}
	authUsers, err := auth.NewUserStore(auth.UserStoreDeps{
		SQLDB: sqlDB, DatabaseDSN: dsn, TablePrefix: "signal_foundry_data_auth_",
		IDGen: ident.NewDefaultGenerator(), Logger: slog.Default(),
	})
	if err != nil {
		return err
	}
	if err = authUsers.AutoMigrate(); err != nil {
		return err
	}
	authRefreshTokens, err := auth.NewRefreshTokenStore(auth.RefreshTokenStoreDeps{
		SQLDB: sqlDB, DatabaseDSN: dsn, TablePrefix: "signal_foundry_data_auth_", Logger: slog.Default(),
	})
	if err != nil {
		return err
	}
	if err = authRefreshTokens.AutoMigrate(); err != nil {
		return err
	}

	jobsStore, err := jobspkg.NewStore(sqlDB, dsn, jobspkg.StoreOpts{TablePrefix: "signal_foundry_data_jobs_"})
	if err != nil {
		return err
	}
	migrateErr = jobsStore.AutoMigrate()
	if migrateErr != nil {
		return migrateErr
	}

	financeDatabase, err := persistence.NewDatabase(sqlDB, dsn)
	if err != nil {
		return err
	}
	migrateErr = persistence.NewMigrator(financeDatabase).Migrate(context.Background())
	if migrateErr != nil {
		return migrateErr
	}

	strategyStore, err := rtstrategy.NewArtifactDatabaseStore(sqlDB, dsn, rtstrategy.ArtifactDatabaseStoreOpts{
		TablePrefix: "signal_foundry_data_strategy_",
	})
	if err != nil {
		return err
	}
	migrateErr = strategyStore.AutoMigrate()
	if migrateErr != nil {
		return migrateErr
	}

	strategyRegistry, err := rtstrategy.NewVersionRegistryService(sqlDB, dsn, rtstrategy.VersionRegistryServiceDeps{
		ArtifactStore: strategyStore,
		TablePrefix:   "signal_foundry_data_strategy_",
	})
	if err != nil {
		return err
	}
	migrateErr = strategyRegistry.AutoMigrate()
	if migrateErr != nil {
		return migrateErr
	}

	governorStore, err := rtgovernor.NewArtifactDatabaseStore(sqlDB, dsn, rtgovernor.ArtifactDatabaseStoreOpts{
		TablePrefix: "signal_foundry_data_evaluation_",
	})
	if err != nil {
		return err
	}
	migrateErr = governorStore.AutoMigrate()
	if migrateErr != nil {
		return migrateErr
	}

	auditStore, err := audit.NewDatabaseStore(sqlDB, dsn, audit.DatabaseStoreOpts{
		TablePrefix: "signal_foundry_data_evaluation_",
	})
	if err != nil {
		return err
	}
	migrateErr = auditStore.AutoMigrate()
	if migrateErr != nil {
		return migrateErr
	}

	executionStore, err := execution.NewDatabaseStore(sqlDB, dsn, execution.DatabaseStoreOpts{
		TablePrefix: "signal_foundry_data_evaluation_",
	})
	if err != nil {
		return err
	}
	migrateErr = executionStore.AutoMigrate()
	if migrateErr != nil {
		return migrateErr
	}

	backtestStore, err := backtest.NewDatabaseStore(sqlDB, dsn, backtest.DatabaseStoreOpts{
		TablePrefix: "signal_foundry_data_evaluation_",
	})
	if err != nil {
		return err
	}
	return backtestStore.AutoMigrate()
}

// chdirModuleRoot keeps command tests in the supported app working directory.
func chdirModuleRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	moduleRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	t.Chdir(moduleRoot)
}

func TestMain(t *testing.T) {
	chdirModuleRoot(t)
	fake := faker.New()
	t.Run("start", func(t *testing.T) {
		t.Run("should initialize app", func(t *testing.T) {
			t.Setenv("APP_DATADIR", filepath.Join(t.TempDir(), "data"))
			dsn := filepath.Join(t.TempDir(), "signal-foundry.sqlite")
			t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)
			migrateAppDatabaseForTests(t, dsn)
			rootCmd := setupCommands()
			rootCmd.SetArgs([]string{"start", "-e", "test", "--noop", "--logs-file", testLogFile(t)})
			require.NoError(t, rootCmd.Execute())
		})
		t.Run("should fail if bad log level", func(t *testing.T) {
			rootCmd := setupCommands()
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.SetArgs(
				[]string{"start", "-e", "test", "--noop", "-l", fake.Lorem().Word(), "--logs-file", testLogFile(t)},
			)
			assert.Error(t, rootCmd.Execute())
		})
		t.Run("should not expose deprecated ui-location flag on start command", func(t *testing.T) {
			rootCmd := setupCommands()
			startCmd := findStartCmd(t, rootCmd)
			assert.Nil(t, startCmd.Flags().Lookup("ui-location"))
		})

		t.Run("does not run durable jobs inline", func(t *testing.T) {
			dsn := filepath.Join(t.TempDir(), "start-no-inline.sqlite")
			t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)
			t.Setenv("APP_JOBS_WORKER_ENABLED", "true")
			runDatabaseMigrateCommand(t)

			store, jobID := makeQueuedUnknownJob(t, dsn)

			container := dig.New()
			rootCmd := newRootCmd()
			rootCmd.AddCommand(newStartServerCmd(container))
			rootCmd.SetArgs([]string{"start", "-e", "test", "--noop", "--logs-file", testLogFile(t)})
			require.NoError(t, rootCmd.Execute())

			persisted, err := store.Get(t.Context(), jobID)
			require.NoError(t, err)
			assert.Equal(t, jobspkg.JobStatusQueued, persisted.Status)
			assert.Empty(t, persisted.WorkerID)
		})

		t.Run("db-migrate command is discoverable and inherits root flags", func(t *testing.T) {
			rootCmd := setupCommands()
			dbMigrateCmd := findRootCommandByName(t, rootCmd, dbMigrateCommandName)

			require.NotNil(t, dbMigrateCmd.InheritedFlags().Lookup("env"))
			require.NotNil(t, dbMigrateCmd.InheritedFlags().Lookup("log-level"))
			require.NotNil(t, dbMigrateCmd.InheritedFlags().Lookup("json-logs"))
			require.NotNil(t, dbMigrateCmd.InheritedFlags().Lookup("logs-file"))
		})

		t.Run("db-migrate command reports contextual errors without unsafe output", func(t *testing.T) {
			rootCmd := newRootCmd()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.AddCommand(newDatabaseMigrateCmdWithResolver(
				dig.New(),
				func(*cobra.Command, *dig.Container) (databaseMigrationRunner, error) {
					return migrationRunnerStub{
						err: secretiveMigrationError{
							safe:   "migrate finance schema",
							secret: "postgres://user:password@example.invalid/signal-foundry",
						},
					}, nil
				},
			))
			rootCmd.SetArgs([]string{"db-migrate"})

			err := rootCmd.ExecuteContext(t.Context())
			require.EqualError(t, err, "run database migrations: migrate finance schema")
			require.NotContains(t, err.Error(), "password@example.invalid")
			require.Empty(t, stdout.String())
			require.Empty(t, stderr.String())
		})
	})
}

func makeQueuedUnknownJob(t *testing.T, dsn string) (*jobspkg.Store, string) {
	t.Helper()
	fake := faker.New()
	sqlDB, err := sqlconn.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	store, err := jobspkg.NewStore(sqlDB, dsn, jobspkg.StoreOpts{TablePrefix: "signal_foundry_data_jobs_"})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate())
	now := time.Now().UTC()
	created, err := store.Create(t.Context(), jobspkg.Job{
		ID:      "job-" + fake.UUID().V4(),
		JobType: jobspkg.JobType("unknown-" + fake.UUID().V4()),
		Status:  jobspkg.JobStatusQueued,
		Requester: jobspkg.Requester{
			UserID: "user-" + fake.UUID().V4(),
			Source: jobspkg.RequesterSourceOperator,
		},
		InputHash:     fake.UUID().V4(),
		InputJSON:     []byte(`{"value":true}`),
		CreatedAt:     now,
		UpdatedAt:     now,
		QueuedAt:      now,
		MaxAttempts:   3,
		CorrelationID: "corr-" + fake.UUID().V4(),
	})
	require.NoError(t, err)
	return store, created.ID
}

func findStartCmd(t *testing.T, rootCmd *cobra.Command) *cobra.Command {
	return findRootCommandByName(t, rootCmd, startCommandName)
}

func findRootCommandByName(t *testing.T, rootCmd *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	t.Fatalf("%s command not found", name)
	return nil
}

func TestDevFlow(t *testing.T) {
	chdirModuleRoot(t)
	t.Run("default startup is API-only and requires no UI build artifacts", func(t *testing.T) {
		t.Run("start without ui override completes setup without error", func(t *testing.T) {
			dsn := filepath.Join(t.TempDir(), "signal-foundry.sqlite")
			t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)
			migrateAppDatabaseForTests(t, dsn)
			rootCmd := setupCommands()
			rootCmd.SetArgs([]string{"start", "-e", "test", "--noop", "--logs-file", testLogFile(t)})
			require.NoError(t, rootCmd.Execute())
		})

		t.Run("start command keeps only noop HTTP startup toggle", func(t *testing.T) {
			rootCmd := setupCommands()
			startCmd := findStartCmd(t, rootCmd)
			assert.NotNil(t, startCmd.Flags().Lookup("noop"))
			assert.Nil(t, startCmd.Flags().Lookup("ui-location"))
		})
	})
}

type migrationRunnerStub struct {
	err error
}

func (s migrationRunnerStub) Migrate(context.Context) error {
	return s.err
}

type secretiveMigrationError struct {
	safe   string
	secret string
}

func (e secretiveMigrationError) Error() string {
	return e.safe
}

func (e secretiveMigrationError) Unwrap() error {
	return errors.New(e.secret)
}
