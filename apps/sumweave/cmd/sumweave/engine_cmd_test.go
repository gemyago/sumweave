//go:build !release

package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestEngineOptionsFromRoot(t *testing.T) {
	makeRoot := func(t *testing.T) *cobra.Command {
		t.Helper()
		return newRootCmd()
	}

	t.Run("preserves omitted and explicit false or empty start command overrides", func(t *testing.T) {
		root := makeRoot(t)
		options, err := engineOptionsFromRoot(root)
		require.NoError(t, err)
		require.Empty(t, options.Environment)
		require.Nil(t, options.DefaultLogLevel)
		require.Nil(t, options.JSONLogs)
		require.Nil(t, options.LogsFile)

		require.NoError(t, root.PersistentFlags().Set("env", "test"))
		require.NoError(t, root.PersistentFlags().Set("log-level", ""))
		require.NoError(t, root.PersistentFlags().Set("json-logs", "false"))
		require.NoError(t, root.PersistentFlags().Set("logs-file", ""))
		options, err = engineOptionsFromRoot(root)
		require.NoError(t, err)
		require.Equal(t, "test", options.Environment)
		require.NotNil(t, options.DefaultLogLevel)
		require.Empty(t, *options.DefaultLogLevel)
		require.NotNil(t, options.JSONLogs)
		require.False(t, *options.JSONLogs)
		require.NotNil(t, options.LogsFile)
		require.Empty(t, *options.LogsFile)
	})
}
