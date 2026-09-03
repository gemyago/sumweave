//go:build postgres_test

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMain verifies that production commands wire real dependencies correctly.
func TestMain(t *testing.T) {
	t.Run("noop startup commands initialize and close prepared production dependencies", func(t *testing.T) {
		t.Chdir("../..")
		t.Setenv("APP_DATADIR", t.TempDir())

		startCmd := setupCommands()
		startCmd.SetArgs([]string{"--env", "test", startCommandName, "--noop"})
		require.NoError(t, startCmd.ExecuteContext(t.Context()))

		rootCmd := setupCommands()
		rootCmd.SetArgs([]string{"--env", "test", startAllCommandName, "--noop"})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
	})

	t.Run("file-backed runtime initializes and closes prepared production dependencies", func(t *testing.T) {
		t.Chdir("../..")
		t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
		t.Setenv("APP_DATADIR", t.TempDir())

		startCmd := setupCommands()
		startCmd.SetArgs([]string{"--env", "test", startCommandName, "--noop"})
		require.NoError(t, startCmd.ExecuteContext(t.Context()))
	})
}
