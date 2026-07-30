package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMain verifies that production commands wire real dependencies correctly.
func TestMain(t *testing.T) {
	t.Run("db-migrate then noop startup commands initialize and close production dependencies", func(t *testing.T) {
		t.Chdir("../..")
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), "application.sqlite"))

		migrateCmd := setupCommands()
		migrateCmd.SetArgs([]string{"--env", "test", "db-migrate"})
		require.NoError(t, migrateCmd.ExecuteContext(t.Context()))

		startCmd := setupCommands()
		startCmd.SetArgs([]string{"--env", "test", startCommandName, "--noop"})
		require.NoError(t, startCmd.ExecuteContext(t.Context()))

		rootCmd := setupCommands()
		rootCmd.SetArgs([]string{"--env", "test", startAllCommandName, "--noop"})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
	})
}
