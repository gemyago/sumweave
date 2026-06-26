package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestDatabaseMigrateCommand(t *testing.T) {
	chdirModuleRoot(t)

	t.Run("runs migrations when resolver succeeds", func(t *testing.T) {
		cmd := newDatabaseMigrateCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (databaseMigrationRunner, error) {
				return migrationRunnerStub{}, nil
			},
		)

		require.NoError(t, cmd.ExecuteContext(t.Context()))
	})

	t.Run("returns resolver errors directly", func(t *testing.T) {
		wantErr := errors.New("resolve migrator")
		cmd := newDatabaseMigrateCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (databaseMigrationRunner, error) {
				return nil, wantErr
			},
		)

		err := cmd.ExecuteContext(t.Context())
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("resolveDatabaseMigrator resolves the configured migrator", func(t *testing.T) {
		dsn := filepath.Join(t.TempDir(), "signal-foundry.sqlite")
		t.Setenv("APP_DATADIR", filepath.Join(t.TempDir(), "data"))
		t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)

		rootCmd := setupCommands()
		require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))
		require.NoError(t, rootCmd.PersistentFlags().Set("logs-file", testLogFile(t)))

		migrator, err := resolveDatabaseMigrator(
			findRootCommandByName(t, rootCmd, dbMigrateCommandName),
			dig.New(),
		)
		require.NoError(t, err)
		require.NotNil(t, migrator)
	})

	t.Run("resolveDatabaseMigrator returns engine setup errors", func(t *testing.T) {
		rootCmd := setupCommands()
		require.NoError(t, rootCmd.PersistentFlags().Set("env", "missing-env"))
		require.NoError(t, rootCmd.PersistentFlags().Set("logs-file", testLogFile(t)))

		_, err := resolveDatabaseMigrator(
			findRootCommandByName(t, rootCmd, dbMigrateCommandName),
			dig.New(),
		)
		require.Error(t, err)
	})
}
