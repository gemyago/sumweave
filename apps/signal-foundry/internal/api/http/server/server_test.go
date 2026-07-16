package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPServer(t *testing.T) {
	makeDeps := func() HTTPServerDeps {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())

		return HTTPServerDeps{
			RootLogger:    telemetry.RootTestLogger(),
			Host:          "localhost",
			Port:          port,
			ShutdownHooks: lifecycle.NewTestShutdownHooks(),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			OTELMiddleware:  func(h http.Handler) http.Handler { return h },
			listeningSignal: make(chan struct{}),
		}
	}

	t.Run("Startup/Shutdown", func(t *testing.T) {
		t.Run("should start and stop the server", func(t *testing.T) {
			deps := makeDeps()
			addr := fmt.Sprintf("localhost:%d", deps.Port)

			srv := NewHTTPServer(deps)
			assert.True(t, deps.ShutdownHooks.HasHook("http-server", srv.httpSrv.Shutdown))

			stopCh := make(chan error, 1)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			go func() {
				stopCh <- srv.Start(ctx)
			}()

			select {
			case <-deps.listeningSignal:
			case err := <-stopCh:
				t.Fatalf("server failed to start: %v", err)
			case <-ctx.Done():
				t.Fatalf("server failed to signal readiness in time: %v", ctx.Err())
			}

			res, err := http.Get("http://" + addr)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, res.StatusCode)

			require.NoError(t, srv.httpSrv.Shutdown(ctx), "httpSrv.Shutdown failed")

			select {
			case err = <-stopCh:
				require.NoError(t, err, "srv.Start returned an unexpected error on shutdown")
			case <-ctx.Done():
				t.Fatalf("server failed to shutdown in time: %v", ctx.Err())
			}

			_, err = http.Get("http://" + addr)
			require.Error(t, err, "expected connection error after shutdown")

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				t.Errorf("expected connection refused, but got timeout error: %v", err)
			}

			_, err = http.Get("http://" + srv.httpSrv.Addr)
			require.Error(t, err)
			assert.ErrorIs(t, err, syscall.ECONNREFUSED)
		})

		t.Run("fail if already listening", func(t *testing.T) {
			deps := makeDeps()

			srv1 := NewHTTPServer(deps)
			srv2 := NewHTTPServer(deps)

			stoppedSrv1Ch := make(chan error, 1)
			stoppedSrv2Ch := make(chan error, 1)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			go func() {
				stoppedSrv1Ch <- srv1.Start(ctx)
			}()

			select {
			case <-deps.listeningSignal:
			case err := <-stoppedSrv1Ch:
				t.Fatalf("server failed to start: %v", err)
			case <-ctx.Done():
				t.Fatalf("server failed to signal readiness in time: %v", ctx.Err())
			}

			// We start the second one after first one is up
			go func() {
				stoppedSrv2Ch <- srv2.Start(ctx)
			}()

			select {
			case err := <-stoppedSrv2Ch:
				require.ErrorContains(t, err, "already in use")
			case <-ctx.Done():
				t.Fatalf("server failed to signal readiness in time: %v", ctx.Err())
			}

			require.NoError(t, srv1.httpSrv.Shutdown(ctx), "httpSrv.Shutdown failed")
		})
	})

	t.Run("TLS", func(t *testing.T) {
		t.Run("serves HTTPS with configured certificate files", func(t *testing.T) {
			deps := makeDeps()
			deps.TLSCertFile, deps.TLSKeyFile = writeTestCertificate(t)
			addr := fmt.Sprintf("localhost:%d", deps.Port)
			srv := NewHTTPServer(deps)
			stopCh := make(chan error, 1)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			go func() { stopCh <- srv.Start(ctx) }()
			awaitListening(ctx, t, deps.listeningSignal, stopCh)

			tlsConfig := &tls.Config{
				InsecureSkipVerify: true,
			}
			client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
			res, err := client.Get("https://" + addr)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, res.StatusCode)
			require.NoError(t, res.Body.Close())
			require.NoError(t, srv.httpSrv.Shutdown(ctx))
			require.NoError(t, <-stopCh)
		})

		t.Run("rejects incomplete certificate configuration", func(t *testing.T) {
			deps := makeDeps()
			deps.TLSCertFile = filepath.Join(t.TempDir(), "certificate.pem")
			err := NewHTTPServer(deps).Start(t.Context())
			require.ErrorContains(t, err, "both HTTP TLS certificate and key files are required")
		})
	})
}

func awaitListening(ctx context.Context, t *testing.T, listening <-chan struct{}, stopped <-chan error) {
	t.Helper()
	select {
	case <-listening:
	case err := <-stopped:
		t.Fatalf("server failed to start: %v", err)
	case <-ctx.Done():
		t.Fatalf("server failed to signal readiness in time: %v", ctx.Err())
	}
}

func writeTestCertificate(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	certificateTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Minute),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		certificateTemplate,
		certificateTemplate,
		&privateKey.PublicKey,
		privateKey,
	)
	require.NoError(t, err)
	directory := t.TempDir()
	certificateFile := filepath.Join(directory, "certificate.pem")
	keyFile := filepath.Join(directory, "key.pem")
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	require.NoError(t, os.WriteFile(certificateFile, certificatePEM, 0o600))
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	require.NoError(t, os.WriteFile(keyFile, privateKeyPEM, 0o600))
	return certificateFile, keyFile
}

func TestRouterMiddleware(t *testing.T) {
	fake := faker.New()

	t.Run("should wireup the middleware", func(t *testing.T) {
		otelInvoked := false
		deps := RouterMiddlewareDeps{
			RootLogger:      telemetry.RootTestLogger(),
			AccessLogsLevel: slog.LevelInfo.String(),
			OTELMiddleware: func(h http.Handler) http.Handler {
				otelInvoked = true
				return h
			},
			IDGen: ident.NewDefaultGenerator(),
		}

		middleware := NewRouterMiddleware(deps)

		handlerInvoked := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			handlerInvoked = true
			w.WriteHeader(http.StatusOK)
		})

		wrappedHandler := middleware(handler)

		req, err := http.NewRequest(http.MethodGet, fake.Internet().URL(), nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		assert.True(t, otelInvoked, "OTEL middleware was not invoked")
		assert.True(t, handlerInvoked, "Final handler was not invoked")
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	})
}
