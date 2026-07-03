package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestClient(baseURL string, opts ...func(*Args)) *Client {
	args := Args{
		BaseURL: baseURL,
		Logger:  slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		Now:     func() time.Time { return time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC) },
	}
	for _, opt := range opts {
		opt(&args)
	}
	return NewClient(args)
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func withSignedAuth(t *testing.T) func(*Args) {
	t.Helper()
	fake := faker.New()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPath := filepath.Join(t.TempDir(), "enable-banking-"+fake.UUID().V4()+".pem")
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))

	return func(args *Args) {
		args.AppID = "app-" + fake.UUID().V4()
		args.PrivateKeyPath = privateKeyPath
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type errorReadCloser struct{ err error }

func (r errorReadCloser) Read(_ []byte) (int, error) { return 0, r.err }

func (errorReadCloser) Close() error { return nil }

func TestClient_InternalHelpers(t *testing.T) {
	t.Run("DoRawObject covers encode build transport and read failures", func(t *testing.T) {
		client := makeTestClient("https://provider.example.test")
		_, err := client.DoRawObject(t.Context(), DoRawJSONParams{
			Method: http.MethodPost,
			Path:   "/auth",
			Body:   make(chan int),
		})
		require.ErrorContains(t, err, "enable banking request encode")

		client = makeTestClient(":bad")
		_, err = client.DoRawObject(t.Context(), DoRawJSONParams{Method: http.MethodGet, Path: "/auth"})
		require.ErrorContains(t, err, "enable banking request build")

		client = makeTestClient("https://provider.example.test", func(args *Args) {
			args.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("transport failed")
			})}
		})
		_, err = client.DoRawObject(t.Context(), DoRawJSONParams{Method: http.MethodGet, Path: authPath})
		require.ErrorContains(t, err, "transport failed")

		client = makeTestClient("https://provider.example.test", func(args *Args) {
			args.HTTPClient = &http.Client{Transport: roundTripperFunc(
				func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       errorReadCloser{err: errors.New("read failed")},
					}, nil
				},
			)}
		})
		_, err = client.DoRawObject(t.Context(), DoRawJSONParams{Method: http.MethodGet, Path: authPath})
		require.ErrorContains(t, err, "enable banking response read: read failed")
	})

	t.Run("raw decoders and auth helpers cover fallback branches", func(t *testing.T) {
		responseBody := `{"message":"bad body","error":"BAD_BODY"}`
		client := makeTestClient("https://provider.example.test", func(args *Args) {
			args.HTTPClient = &http.Client{Transport: roundTripperFunc(
				func(request *http.Request) (*http.Response, error) {
					assert.Equal(t, "Bearer token-123", request.Header.Get("Authorization"))
					if request.URL.Path == "/bad-json" {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader("{")),
						}, nil
					}
					if request.URL.Path == "/array" {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(`[{"id":"aspsp-1"}]`)),
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusBadRequest,
						Body:       io.NopCloser(strings.NewReader(responseBody)),
					}, nil
				},
			)}
		})

		ctx := WithBearerToken(t.Context(), "token-123")
		_, err := client.DoRawObject(ctx, DoRawJSONParams{Method: http.MethodGet, Path: "/bad-json"})
		require.ErrorContains(t, err, "enable banking response decode")

		items, err := client.DoRawArray(ctx, DoRawJSONParams{Method: http.MethodGet, Path: "/array"})
		require.NoError(t, err)
		require.Len(t, items, 1)

		_, err = client.DoRawArray(ctx, DoRawJSONParams{Method: http.MethodGet, Path: "/bad-json"})
		require.ErrorContains(t, err, "enable banking response decode")

		_, err = client.DoRawObject(ctx, DoRawJSONParams{Method: http.MethodGet, Path: "/bad-status"})
		require.Error(t, err)
		var responseErr *ResponseError
		require.ErrorAs(t, err, &responseErr)
		assert.Equal(t, "bad-status", responseErr.Operation)
		assert.Equal(t, http.StatusBadRequest, responseErr.StatusCode)
		assert.Equal(t, "BAD_BODY", responseErr.Code)
		assert.Equal(t, "bad body", responseErr.Message)
	})

	t.Run("constructor and parsing helpers cover defaults", func(t *testing.T) {
		client := NewClient(Args{})
		require.NotNil(t, client.httpClient)
		assert.Equal(t, defaultBaseURL, client.baseURL)

		assert.Equal(t, "message", firstNonEmpty("", "message"))
		assert.Equal(t, 7, intValue(map[string]any{"value": float64(7)}, "value"))
		assert.Equal(t, int64(8), int64Value(map[string]any{"value": int64(8)}, "value"))
		assert.Equal(
			t,
			"nested",
			extractSessionIdentifier(
				map[string]any{"session": map[string]any{"id": "nested"}},
				"id",
			),
		)
		assert.Equal(t, int64(-1234), decimalToMinor("-12.34"))
		assert.Equal(
			t,
			int64(1234),
			signedAmountMinor(
				map[string]any{"amount": map[string]any{"amount": "12.34", "currency": "PLN"}},
			),
		)
		assert.Equal(
			t,
			int64(-1234),
			signedAmountMinor(map[string]any{
				"amount":                 map[string]any{"amount": "12.34", "currency": "PLN"},
				"credit_debit_indicator": "DBIT",
			}),
		)
		responseErr := &ResponseError{Message: "provider request failed"}
		assert.Equal(t, "provider request failed", responseErr.Message)
		message, code := parseResponseBody([]byte("{"))
		assert.Empty(t, message)
		assert.Empty(t, code)
		assert.Nil(t, extractSessionAccess(map[string]any{"access": map[string]any{}}))

		client = makeTestClient("https://provider.example.test", func(args *Args) {
			args.PrivateKeyPath = filepath.Join(t.TempDir(), "missing-app-id.pem")
		})
		_, err := client.DoRawObject(t.Context(), DoRawJSONParams{Method: http.MethodGet, Path: authPath})
		require.ErrorContains(t, err, "enable banking app ID is required")

		client = makeTestClient("https://provider.example.test", func(args *Args) {
			args.AppID = "app-1"
		})
		_, err = client.DoRawObject(t.Context(), DoRawJSONParams{Method: http.MethodGet, Path: authPath})
		require.ErrorContains(t, err, "enable banking private key path is required")
	})
}
