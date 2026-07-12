//go:build !release

package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/stretchr/testify/require"
)

func TestDatabaseMigrator(t *testing.T) {
	memoryDSNOrdinal := 0
	makeSQLiteMemoryDSN := func() string {
		memoryDSNOrdinal++
		return fmt.Sprintf("file:signal-foundry-migrate-%d?mode=memory&cache=shared", memoryDSNOrdinal)
	}

	makeDeps := func(t *testing.T, storageType string) DatabaseMigrationDeps {
		t.Helper()

		dsn := makeSQLiteMemoryDSN()
		sharedDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sharedDB.Close()) })
		dataStore, err := data.NewDatabaseStore(sharedDB, dsn, data.DatabaseStoreOpts{
			TablePrefix: "signal_foundry_data_",
		})
		require.NoError(t, err)

		return DatabaseMigrationDeps{
			RootLogger:                      telemetry.RootTestLogger(),
			AgentRuntimeStorageType:         storageType,
			AgentRuntimeDatabaseDSN:         dsn,
			AgentRuntimeDatabaseTablePrefix: "runtime_",
			DataLayerDatabaseDSN:            dsn,
			DataLayerDatabaseTablePrefix:    "signal_foundry_data_",
			DataLayerSQLDB:                  sharedDB,
			DataStore:                       dataStore,
		}
	}

	openDB := func(t *testing.T, dsn string) *sql.DB {
		t.Helper()
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return db
	}

	requireTable := func(t *testing.T, db *sql.DB, name string) {
		t.Helper()
		var found string
		err := db.QueryRowContext(
			t.Context(),
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			name,
		).Scan(&found)
		require.NoError(t, err)
		require.Equal(t, name, found)
	}

	requireAnyTable := func(t *testing.T, db *sql.DB, names ...string) {
		t.Helper()
		for _, name := range names {
			var found string
			err := db.QueryRowContext(
				t.Context(),
				`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
				name,
			).Scan(&found)
			if err == nil && found == name {
				return
			}
		}
		t.Fatalf("expected one of tables %v to exist", names)
	}

	requireNoTable := func(t *testing.T, db *sql.DB, name string) {
		t.Helper()
		var found string
		err := db.QueryRowContext(
			t.Context(),
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			name,
		).Scan(&found)
		require.ErrorIs(t, err, sql.ErrNoRows)
	}

	makeReadOnlySQLiteDSN := func(t *testing.T) string {
		t.Helper()

		// File-backed on purpose: read-only SQLite opens need a real database file.
		dbPath := filepath.Join(t.TempDir(), "readonly.sqlite")
		db := openDB(t, dbPath)
		_, err := db.ExecContext(t.Context(), `PRAGMA user_version = 1`)
		require.NoError(t, err)

		return (&url.URL{
			Scheme:   "file",
			Path:     dbPath,
			RawQuery: "mode=ro",
		}).String()
	}

	makeReadOnlyDataStore := func(t *testing.T) *data.DatabaseStore {
		t.Helper()

		dsn := makeReadOnlySQLiteDSN(t)
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := data.NewDatabaseStore(sqlDB, dsn, data.DatabaseStoreOpts{
			TablePrefix: "signal_foundry_data_",
		})
		require.NoError(t, err)
		return store
	}

	t.Run("migrates every configured schema family and stays idempotent", func(t *testing.T) {
		deps := makeDeps(t, storageTypeDatabase)
		migrator := newDatabaseMigrator(deps)

		require.NoError(t, migrator.Migrate(context.Background()))
		require.NoError(t, migrator.Migrate(context.Background()))

		db := openDB(t, deps.DataLayerDatabaseDSN)
		requireAnyTable(t, db, "agent_profiles", "runtime_agent_profiles")
		requireAnyTable(t, db, "provider_configs", "runtime_provider_configs")
		requireAnyTable(t, db, "session_metadata", "runtime_session_metadata")
		for _, tableName := range []string{
			"signal_foundry_data_instruments",
			appdispatch.Config{TablePrefix: "signal_foundry_data_"}.MessagesTable(),
			appdispatch.Config{TablePrefix: "signal_foundry_data_"}.OffsetsTable(),
			"signal_foundry_data_jobs_jobs",
			"signal_foundry_data_jobs_job_schedules",
			"finance_tenants",
			"signal_foundry_data_strategy_strategy_artifacts",
			"signal_foundry_data_strategy_strategy_versions",
			"signal_foundry_data_evaluation_governor_policy_artifacts",
			"signal_foundry_data_evaluation_decision_traces",
			"signal_foundry_data_evaluation_execution_commands",
			"signal_foundry_data_evaluation_backtest_runs",
		} {
			requireTable(t, db, tableName)
		}
	})

	t.Run("skips agent runtime database migration for file-backed storage", func(t *testing.T) {
		deps := makeDeps(t, "file")
		migrator := newDatabaseMigrator(deps)

		require.NoError(t, migrator.Migrate(context.Background()))

		db := openDB(t, deps.DataLayerDatabaseDSN)
		requireNoTable(t, db, "agent_profiles")
		requireNoTable(t, db, "runtime_agent_profiles")
		requireNoTable(t, db, "provider_configs")
		requireNoTable(t, db, "runtime_provider_configs")
		requireNoTable(t, db, "session_metadata")
		requireNoTable(t, db, "runtime_session_metadata")
		requireTable(t, db, "signal_foundry_data_instruments")
		requireTable(t, db, appdispatch.Config{TablePrefix: "signal_foundry_data_"}.MessagesTable())
		requireTable(t, db, appdispatch.Config{TablePrefix: "signal_foundry_data_"}.OffsetsTable())
		requireTable(t, db, "finance_tenants")
	})

	t.Run("wraps step failures with component context", func(t *testing.T) {
		migrator := newDatabaseMigrator(DatabaseMigrationDeps{
			RootLogger: telemetry.RootTestLogger(),
		})
		stepErr := errors.New("boom")

		err := migrator.runStep(t.Context(), "finance", func(context.Context) error {
			return stepErr
		})

		var migrationErr *componentMigrationError
		require.ErrorAs(t, err, &migrationErr)
		require.EqualError(t, err, "migrate finance schema")
		require.ErrorIs(t, err, stepErr)
		require.Equal(t, "finance", migrationErr.component)
	})

	t.Run("stops at the first failing migration component", func(t *testing.T) {
		migrator := newDatabaseMigrator(DatabaseMigrationDeps{
			RootLogger:              telemetry.RootTestLogger(),
			AgentRuntimeStorageType: storageTypeDatabase,
			DataStore:               makeReadOnlyDataStore(t),
		})

		err := migrator.Migrate(t.Context())

		var migrationErr *componentMigrationError
		require.ErrorAs(t, err, &migrationErr)
		require.Equal(t, "agent runtime", migrationErr.component)
		require.ErrorContains(t, err, "migrate agent runtime schema")
	})

	t.Run("surfaces explicit migration failures", func(t *testing.T) {
		makeMigrator := func(overrides func(*DatabaseMigrationDeps)) *DatabaseMigrator {
			deps := DatabaseMigrationDeps{
				RootLogger:                      telemetry.RootTestLogger(),
				AgentRuntimeStorageType:         "file",
				AgentRuntimeDatabaseTablePrefix: "runtime_",
				DataLayerDatabaseTablePrefix:    "signal_foundry_data_",
			}
			if overrides != nil {
				overrides(&deps)
			}
			if deps.DataLayerDatabaseDSN != "" && deps.DataLayerSQLDB == nil {
				sharedDB, err := sqlconn.Open(deps.DataLayerDatabaseDSN)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, sharedDB.Close()) })
				deps.DataLayerSQLDB = sharedDB
			}
			return newDatabaseMigrator(deps)
		}

		t.Run("agent runtime create failure", func(t *testing.T) {
			err := makeMigrator(func(deps *DatabaseMigrationDeps) {
				deps.AgentRuntimeStorageType = storageTypeDatabase
			}).migrateAgentRuntime(t.Context())
			require.ErrorContains(t, err, "create providers config service")
		})

		t.Run("agent runtime write failure", func(t *testing.T) {
			err := makeMigrator(func(deps *DatabaseMigrationDeps) {
				deps.AgentRuntimeStorageType = storageTypeDatabase
				deps.AgentRuntimeDatabaseDSN = makeReadOnlySQLiteDSN(t)
			}).migrateAgentRuntime(t.Context())
			require.ErrorContains(t, err, "auto migrate sessions database")
		})

		t.Run("data layer write failure", func(t *testing.T) {
			err := makeMigrator(func(deps *DatabaseMigrationDeps) {
				deps.DataStore = makeReadOnlyDataStore(t)
			}).migrateDataLayer(t.Context())
			require.ErrorContains(t, err, "auto migrate data-layer database")
		})

		for _, tc := range []struct {
			name string
			run  func(*DatabaseMigrator) error
			want string
		}{
			{
				name: "app dispatch migrator create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateAppDispatch(t.Context())
				},
				want: "create app dispatch migrator",
			},
			{
				name: "jobs store create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateJobs(t.Context())
				},
				want: "create jobs store",
			},
			{
				name: "finance store create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateFinance(t.Context())
				},
				want: "open finance database",
			},
			{
				name: "strategy artifact store create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateStrategyArtifacts(t.Context())
				},
				want: "create strategy artifact store",
			},
			{
				name: "strategy version registry artifact store create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateStrategyVersionRegistry(t.Context())
				},
				want: "create strategy artifact store",
			},
			{
				name: "governor policy artifact store create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateEvaluationGovernorPolicy(t.Context())
				},
				want: "create governor policy artifact store",
			},
			{
				name: "evaluation audit store create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateEvaluationAudit(t.Context())
				},
				want: "create evaluation audit store",
			},
			{
				name: "evaluation execution store create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateEvaluationExecution(t.Context())
				},
				want: "create evaluation execution store",
			},
			{
				name: "evaluation backtest store create failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateEvaluationBacktest(t.Context())
				},
				want: "create evaluation backtest store",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.run(makeMigrator(func(deps *DatabaseMigrationDeps) {
					deps.DataLayerDatabaseDSN = ""
				}))
				require.ErrorContains(t, err, tc.want)
			})
		}

		for _, tc := range []struct {
			name string
			run  func(*DatabaseMigrator) error
			want string
		}{
			{
				name: "app dispatch migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateAppDispatch(t.Context())
				},
				want: "migrate app dispatch transport",
			},
			{
				name: "jobs store auto-migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateJobs(t.Context())
				},
				want: "auto migrate jobs store",
			},
			{
				name: "finance store migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateFinance(t.Context())
				},
				want: "migrate finance schema",
			},
			{
				name: "strategy artifact store auto-migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateStrategyArtifacts(t.Context())
				},
				want: "auto migrate strategy artifact store",
			},
			{
				name: "strategy version registry auto-migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateStrategyVersionRegistry(t.Context())
				},
				want: "auto migrate strategy version registry service",
			},
			{
				name: "governor policy artifact store auto-migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateEvaluationGovernorPolicy(t.Context())
				},
				want: "auto migrate governor policy artifact store",
			},
			{
				name: "evaluation audit store auto-migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateEvaluationAudit(t.Context())
				},
				want: "auto migrate evaluation audit store",
			},
			{
				name: "evaluation execution store auto-migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateEvaluationExecution(t.Context())
				},
				want: "auto migrate evaluation execution store",
			},
			{
				name: "evaluation backtest store auto-migrate failure",
				run: func(m *DatabaseMigrator) error {
					return m.migrateEvaluationBacktest(t.Context())
				},
				want: "auto migrate evaluation backtest store",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.run(makeMigrator(func(deps *DatabaseMigrationDeps) {
					deps.DataLayerDatabaseDSN = makeReadOnlySQLiteDSN(t)
				}))
				require.ErrorContains(t, err, tc.want)
			})
		}
	})
}
