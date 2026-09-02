package main

import (
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestMigrationOptionsFromRoot(t *testing.T) {
	fake := faker.New()
	makeRoot := func(t *testing.T) *cobra.Command {
		t.Helper()
		return newRootCmd()
	}

	t.Run("preserves omitted and explicit CLI values", func(t *testing.T) {
		root := makeRoot(t)
		options, err := migrationOptionsFromRoot(root)
		require.NoError(t, err)
		require.Empty(t, options.Environment)
		require.Nil(t, options.DefaultLogLevel)
		require.Nil(t, options.JSONLogs)
		require.Nil(t, options.LogsFile)

		logLevel, logsFile := "WARN", fake.UUID().V4()
		require.NoError(t, root.PersistentFlags().Set("env", "test"))
		require.NoError(t, root.PersistentFlags().Set("log-level", logLevel))
		require.NoError(t, root.PersistentFlags().Set("json-logs", "false"))
		require.NoError(t, root.PersistentFlags().Set("logs-file", logsFile))
		options, err = migrationOptionsFromRoot(root)
		require.NoError(t, err)
		require.Equal(t, "test", options.Environment)
		require.Equal(t, logLevel, *options.DefaultLogLevel)
		require.False(t, *options.JSONLogs)
		require.Equal(t, logsFile, *options.LogsFile)
	})

	t.Run("reports unavailable or wrongly typed flags", func(t *testing.T) {
		_, err := migrationOptionsFromRoot(&cobra.Command{})
		require.Error(t, err)

		for _, testCase := range []struct {
			name      string
			configure func(*cobra.Command)
		}{
			{
				name: "log level",
				configure: func(root *cobra.Command) {
					root.PersistentFlags().Bool("log-level", false, "")
					require.NoError(t, root.PersistentFlags().Set("log-level", "true"))
				},
			},
			{
				name: "json logs",
				configure: func(root *cobra.Command) {
					root.PersistentFlags().String("json-logs", "", "")
					require.NoError(t, root.PersistentFlags().Set("json-logs", fake.UUID().V4()))
				},
			},
			{
				name: "logs file",
				configure: func(root *cobra.Command) {
					root.PersistentFlags().Bool("logs-file", false, "")
					require.NoError(t, root.PersistentFlags().Set("logs-file", "true"))
				},
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				root := &cobra.Command{}
				root.PersistentFlags().String("env", "test", "")
				testCase.configure(root)
				_, optionsErr := migrationOptionsFromRoot(root)
				require.Error(t, optionsErr)
			})
		}
	})

	t.Run("surfaces typed-root construction errors", func(t *testing.T) {
		_, err := resolveDatabaseMigrator(&cobra.Command{})
		require.Error(t, err)

		t.Setenv("APP_APPLICATION_DATABASE_DSN", "")
		root := makeRoot(t)
		require.NoError(t, root.PersistentFlags().Set("env", "production"))
		_, err = resolveDatabaseMigrator(root)
		require.ErrorContains(t, err, "application database dsn")
	})
}
