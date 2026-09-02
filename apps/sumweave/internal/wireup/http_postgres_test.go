//go:build postgres_test

package wireup

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPostgresBuildHTTP(t *testing.T) {
	t.Chdir("../..")
	makeRoot := func(t *testing.T) *HTTPRoot {
		t.Helper()
		t.Setenv("APP_DATADIR", t.TempDir())
		root, err := BuildHTTP(t.Context(), HTTPOptions{Environment: "test"})
		require.NoError(t, err)
		return root
	}

	t.Run("builds API routes without worker polling resources", func(t *testing.T) {
		root := makeRoot(t)
		t.Cleanup(func() { require.NoError(t, root.Close(t.Context())) })
		request := httptest.NewRequest(http.MethodGet, "/health", nil)
		response := httptest.NewRecorder()
		root.Handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.NotNil(t, root.Server)
	})

	t.Run("closes the root when noop completes", func(t *testing.T) {
		root := makeRoot(t)
		expectedErr := errors.New("noop shutdown")
		root.shutdownHooks.Register("noop-test", func(context.Context) error { return expectedErr })
		require.ErrorIs(t, root.StartHTTPServer(t.Context(), true), expectedErr)
	})

	t.Run("closes the root and joins the server after caller cancellation", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())
		t.Setenv("APP_HTTPSERVER_PORT", strconv.Itoa(port))
		root := makeRoot(t)
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

	t.Run("returns server startup errors", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, listener.Close()) })
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
}
