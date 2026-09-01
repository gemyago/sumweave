package wireup

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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

	t.Run("builds API routes without worker polling resources", func(t *testing.T) {
		root := makeRoot(t)
		defer func() { require.NoError(t, root.Close(t.Context())) }()
		request := httptest.NewRequest(http.MethodGet, "/health", nil)
		response := httptest.NewRecorder()
		root.Handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.NotNil(t, root.Server)
	})

	t.Run("closes the root when noop completes", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(tempDir, fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", filepath.Join(tempDir, fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_DATADIR", tempDir)
		root, err := BuildHTTP(t.Context(), HTTPOptions{})
		require.NoError(t, err)
		expectedErr := errors.New("noop shutdown")
		root.shutdownHooks.Register("noop-test", func(context.Context) error {
			return expectedErr
		})

		require.ErrorIs(t, root.StartHTTPServer(t.Context(), true), expectedErr)
	})

	t.Run("closes the root and joins the server after caller cancellation", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())
		t.Setenv("APP_HTTPSERVER_PORT", strconv.Itoa(port))
		root := makeRoot(t)
		root.shutdownHooks.Register("cancellation-test", func(ctx context.Context) error {
			return ctx.Err()
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		serverErr := make(chan error, 1)
		go func() {
			serverErr <- root.StartHTTPServer(ctx, false)
		}()
		client := &http.Client{Timeout: time.Second}
		require.Eventually(t, func() bool {
			response, requestErr := client.Get("http://localhost:" + strconv.Itoa(port) + "/health")
			if requestErr != nil {
				return false
			}
			defer response.Body.Close()
			return response.StatusCode == http.StatusOK
		}, time.Second, 10*time.Millisecond)

		cancel()
		select {
		case startErr := <-serverErr:
			require.NoError(t, startErr)
		case <-time.After(time.Second):
			t.Fatal("HTTP server did not stop after caller cancellation")
		}
	})

	t.Run("returns server startup errors", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		defer func() { require.NoError(t, listener.Close()) }()
		port := listener.Addr().(*net.TCPAddr).Port
		t.Setenv("APP_HTTPSERVER_PORT", strconv.Itoa(port))
		root := makeRoot(t)
		expectedCleanupErr := errors.New("startup cleanup")
		root.shutdownHooks.Register("startup-error-test", func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return expectedCleanupErr
		})

		startErr := root.StartHTTPServer(t.Context(), false)
		require.Error(t, startErr)
		require.ErrorIs(t, startErr, expectedCleanupErr)
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
