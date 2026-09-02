//go:build postgres_test

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileMain(t *testing.T) {
	t.Chdir("../..")
	t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
	t.Setenv("APP_DATADIR", t.TempDir())

	startCmd := setupCommands()
	startCmd.SetArgs([]string{"--env", "test", startCommandName, "--noop"})
	require.NoError(t, startCmd.ExecuteContext(t.Context()))
}
