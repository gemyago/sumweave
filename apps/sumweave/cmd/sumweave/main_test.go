package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMain verifies that the production command setup wires real dependencies
// correctly. It runs the actual startup path with --noop to avoid listening.
func TestMain(t *testing.T) {
	t.Run("start initializes production dependencies", func(t *testing.T) {
		t.Chdir("../..")
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), "application.sqlite"))

		migrateCmd := setupCommands()
		migrateCmd.SetArgs([]string{"--env", "test", "db-migrate"})
		require.NoError(t, migrateCmd.ExecuteContext(t.Context()))

		rootCmd := setupCommands()
		rootCmd.SetArgs([]string{"--env", "test", startCommandName, "--noop"})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
	})
}
