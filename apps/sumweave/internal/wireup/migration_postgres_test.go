//go:build postgres_test

package wireup

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPreparedMigrationRoot(t *testing.T) {
	t.Chdir("../..")
	values, err := config.LoadValues(config.ValuesLoadInput{Environment: "test"})
	require.NoError(t, err)
	rootConfig, err := values.MigrationRoot("test")
	require.NoError(t, err)
	jsonLogs := true
	logsFile := filepath.Join(t.TempDir(), "migration.log")
	rootConfig.JSONLogs = &jsonLogs
	rootConfig.LogsFile = &logsFile
	collector := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(collector.Close)
	rootConfig.OpenTelemetry.Enabled = true
	rootConfig.OpenTelemetry.Logs.Enabled = true
	rootConfig.OpenTelemetry.Traces.Enabled = true
	rootConfig.OpenTelemetry.Metrics.Enabled = true
	rootConfig.OpenTelemetry.Logs.Endpoint = collector.URL
	rootConfig.OpenTelemetry.Logs.URLPath = "/"
	rootConfig.OpenTelemetry.Traces.Endpoint = collector.URL
	rootConfig.OpenTelemetry.Traces.URLPath = "/"
	rootConfig.OpenTelemetry.Metrics.Endpoint = collector.URL
	rootConfig.OpenTelemetry.Metrics.URLPath = "/"

	root, err := buildMigration(t.Context(), rootConfig)
	require.NoError(t, err)
	require.NotNil(t, root.migrator)
	require.NoError(t, root.shutdownHooks.PerformShutdown(t.Context()))

	components, err := newDatabaseAgentRuntimeMigrationPreparer(
		rootConfig.AgentRuntime.Database.DSN,
		rootConfig.AgentRuntime.Database.TablePrefix,
		slog.Default(),
	).Prepare()
	require.NoError(t, err)
	require.NotNil(t, components.sessions)
	require.NotNil(t, components.profiles)
	require.NotNil(t, components.providers)

	failedConfig := rootConfig
	failedConfig.OpenTelemetry.Traces.Protocol = "unsupported"
	_, err = buildMigration(t.Context(), failedConfig)
	require.ErrorContains(t, err, "create migration tracer provider")
}
