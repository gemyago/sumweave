package wireup

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildHTTP(t *testing.T) {
	fake := faker.New()
	t.Chdir("../..")
	makeRoot := func(t *testing.T) *HTTPRoot {
		t.Helper()
		dsn := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		t.Setenv("APP_APPLICATION_DATABASE_DSN", dsn)
		migration, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		require.NoError(t, migration.Migrate(t.Context()))
		root, err := BuildHTTP(t.Context(), HTTPOptions{Environment: "test"})
		require.NoError(t, err)
		return root
	}

	t.Run("eagerly builds registered production routes without starting a worker", func(t *testing.T) {
		root := makeRoot(t)
		request := httptest.NewRequest(http.MethodGet, "/health", nil)
		response := httptest.NewRecorder()
		root.Handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.NotNil(t, root.Worker)
		require.NotNil(t, root.Scheduler)
		require.NoError(t, root.StartHTTPServer(t.Context(), true))
	})

	t.Run("rejects invalid root inputs before wireup", func(t *testing.T) {
		_, err := BuildHTTP(t.Context(), HTTPOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)
		_, err = parseLogLevel(fake.UUID().V4())
		require.Error(t, err)
		otelConfig, tracesConfig, metricsConfig, logsConfig := makeTelemetryConfigs(config.OpenTelemetry{})
		require.False(t, otelConfig.Enabled)
		require.False(t, tracesConfig.Enabled)
		require.False(t, metricsConfig.Enabled)
		require.False(t, logsConfig.Enabled)
	})
}
