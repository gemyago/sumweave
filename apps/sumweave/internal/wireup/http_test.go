package wireup

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildHTTP(t *testing.T) {
	fake := faker.New()
	t.Chdir("../..")
	makeRoot := func(t *testing.T, port int) *HTTPRoot {
		t.Helper()
		hooks := lifecycle.NewTestShutdownHooks()
		logger := telemetry.RootTestLogger()
		return &HTTPRoot{
			Server: server.NewHTTPServer(server.HTTPServerDeps{
				ShutdownHooks: hooks,
				RootLogger:    logger,
				Host:          "localhost",
				Port:          port,
				Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
					response.WriteHeader(http.StatusOK)
				}),
				OTELMiddleware: func(handler http.Handler) http.Handler { return handler },
			}),
			rootLogger: logger, shutdownHooks: hooks,
		}
	}

	t.Run("closes a database-free HTTP root when noop completes", func(t *testing.T) {
		root := makeRoot(t, 0)
		expectedErr := errors.New("noop shutdown")
		root.shutdownHooks.Register("noop-test", func(context.Context) error { return expectedErr })
		require.ErrorIs(t, root.StartHTTPServer(t.Context(), true), expectedErr)
	})

	t.Run("closes file-runtime server after caller cancellation", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())
		root := makeRoot(t, port)
		root.shutdownHooks.Register("cancellation-test", func(ctx context.Context) error { return ctx.Err() })
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		serverErr := make(chan error, 1)
		go func() { serverErr <- root.StartHTTPServer(ctx, false) }()
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

	t.Run("reports server startup errors without constructing persistence", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, listener.Close()) })
		port := listener.Addr().(*net.TCPAddr).Port
		root := makeRoot(t, port)
		require.Error(t, root.StartHTTPServer(t.Context(), false))
	})

	t.Run("returns shutdown errors after caller cancellation", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())
		expectedErr := errors.New(fake.Lorem().Sentence(3))
		root := makeRoot(t, port)
		root.shutdownHooks.Register("cancellation-error-test", func(context.Context) error { return expectedErr })
		ctx, cancel := context.WithCancel(t.Context())
		serverErr := make(chan error, 1)
		go func() { serverErr <- root.StartHTTPServer(ctx, false) }()
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
		require.ErrorIs(t, <-serverErr, expectedErr)
	})

	t.Run("returns typed root validation errors before HTTP construction", func(t *testing.T) {
		_, err := BuildHTTP(t.Context(), HTTPOptions{Environment: "production"})
		require.ErrorContains(t, err, "application database dsn")
	})

	t.Run("exposes the root logger", func(t *testing.T) {
		root := makeRoot(t, 0)
		require.Same(t, root.rootLogger, root.Logger())
	})

	t.Run("rejects invalid root inputs before wireup", func(t *testing.T) {
		_, err := BuildHTTP(t.Context(), HTTPOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)
		_, err = parseLogLevel("info")
		require.NoError(t, err)
		_, err = parseLogLevel(fake.UUID().V4())
		require.Error(t, err)
		otelConfig, tracesConfig, metricsConfig, logsConfig := makeTelemetryConfigs(config.OpenTelemetry{})
		require.False(t, otelConfig.Enabled)
		require.False(t, tracesConfig.Enabled)
		require.False(t, metricsConfig.Enabled)
		require.False(t, logsConfig.Enabled)
	})
}
