package swmdclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

//nolint:testifylint // Assertions in httptest handlers report request-contract failures.
func TestClient(t *testing.T) {
	makeClient := func(t *testing.T, serverURL string) *Client {
		t.Helper()
		client, err := NewClient(
			Config{BaseURL: serverURL, APIToken: "token-" + faker.New().UUID().V4()},
			&http.Client{Timeout: time.Second},
		)
		require.NoError(t, err)
		return client
	}

	t.Run("sends authenticated JSON requests and decodes responses", func(t *testing.T) {
		fake := faker.New()
		token := "token-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			require.Equal(t, "/api/v1/test", request.URL.Path)
			require.Equal(t, "value", request.URL.Query().Get("key"))
			require.Equal(t, "Bearer "+token, request.Header.Get("Authorization"))
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"value":"ok"}`))
		}))
		defer server.Close()
		client := makeClient(t, server.URL)
		client.token = token
		output := struct {
			Value string `json:"value"`
		}{}

		err := client.Get(t.Context(), "/api/v1/test", url.Values{"key": {"value"}}, &output)

		require.NoError(t, err)
		require.Equal(t, "ok", output.Value)
	})

	t.Run("preserves configured base URL path prefixes", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			require.Equal(t, "/sumweave/api/v1/test", request.URL.Path)
			_, _ = writer.Write([]byte(`{"value":"ok"}`))
		}))
		defer server.Close()
		client := makeClient(t, server.URL+"/sumweave")
		output := struct {
			Value string `json:"value"`
		}{}

		err := client.Get(t.Context(), "/api/v1/test", nil, &output)

		require.NoError(t, err)
		require.Equal(t, "ok", output.Value)
	})

	t.Run("sends post bodies and idempotency headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			require.Equal(t, http.MethodPost, request.Method)
			require.Equal(t, "application/json", request.Header.Get("Content-Type"))
			require.Equal(t, "retry-key", request.Header.Get("Idempotency-Key"))
			writer.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		err := makeClient(t, server.URL).Post(
			t.Context(), "/api/v1/test", map[string]string{"value": "ok"}, nil, "retry-key",
		)

		require.NoError(t, err)
	})

	t.Run("returns shared API errors with correlation diagnostics", func(t *testing.T) {
		fake := faker.New()
		correlationID := fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("X-Correlation-ID", correlationID)
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(
				`{"code":"insufficient_permission","message":"Denied","correlationId":"` + correlationID + `"}`,
			))
		}))
		defer server.Close()
		var output struct{}

		err := makeClient(t, server.URL).Get(t.Context(), "/api/v1/test", nil, &output)

		var apiError *APIError
		require.ErrorAs(t, err, &apiError)
		require.Equal(t, http.StatusForbidden, apiError.Status)
		require.Equal(t, correlationID, apiError.CorrelationID)
		require.Contains(t, apiError.Error(), correlationID)
	})

	t.Run("rejects unsafe URLs and infinite clients", func(t *testing.T) {
		for _, baseURL := range []string{"http://example.test", "ftp://example.test", "https://example.test?query=value", "https://user@example.test"} {
			_, err := NewClient(Config{BaseURL: baseURL, APIToken: "token"}, &http.Client{Timeout: time.Second})
			require.Error(t, err)
		}
		for _, baseURL := range []string{"http://localhost", "http://127.0.0.1", "http://[::1]", "https://example.test"} {
			_, err := NewClient(Config{BaseURL: baseURL, APIToken: "token"}, &http.Client{Timeout: time.Second})
			require.NoError(t, err)
		}
		_, err := NewClient(Config{BaseURL: "https://example.test", APIToken: "token"}, &http.Client{})
		require.Error(t, err)
		client, err := NewClient(Config{BaseURL: "https://example.test", APIToken: "token"}, nil)
		require.NoError(t, err)
		require.Equal(t, defaultRequestTimeout, client.httpClient.Timeout)
		_, err = NewClient(Config{}, nil)
		require.Error(t, err)
		apiError := &APIError{Status: 500, Code: "internal_error", Message: "failed"}
		require.Contains(t, apiError.Error(), "internal_error")
	})

	t.Run("reports decode and transport failures", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte("not json"))
		}))
		defer server.Close()
		var output struct{}
		require.Error(t, makeClient(t, server.URL).Get(t.Context(), "/api/v1/test", nil, &output))
		client, err := NewClient(
			Config{BaseURL: "http://127.0.0.1:1", APIToken: "token"},
			&http.Client{Timeout: time.Millisecond},
		)
		require.NoError(t, err)
		require.Error(t, client.Get(t.Context(), "/api/v1/test", nil, &output))
	})

	t.Run("handles malformed error payloads, current user, and escaped jobs", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requestCount++
			if requestCount == 1 {
				require.Equal(t, "/api/v1/auth/me", request.URL.Path)
				_, _ = writer.Write([]byte(`{"id":"user","username":"user"}`))
				return
			}
			require.Equal(t, "/api/v1/jobs/a/b", request.URL.Path)
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte("not-json"))
		}))
		defer server.Close()
		client := makeClient(t, server.URL)
		user, err := client.CurrentUser(t.Context())
		require.NoError(t, err)
		require.Equal(t, "user", user.ID)
		_, err = client.GetJob(t.Context(), "a/b")
		var apiError *APIError
		require.ErrorAs(t, err, &apiError)
		require.Equal(t, "http_error", apiError.Code)
	})

	t.Run("reports encoding and cancelled sleep failures", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer server.Close()
		client := makeClient(t, server.URL)
		require.Error(t, client.Post(t.Context(), "/api/v1/test", make(chan int), nil, ""))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, sleepContext(ctx, time.Second), context.Canceled)
		require.Equal(t, "/api/v1/jobs/a%2Fb", route("api", "v1", "jobs", "a/b"))
	})
}

func TestWaitForJob(t *testing.T) {
	makeClient := func(t *testing.T, handler http.Handler) *Client {
		t.Helper()
		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)
		client, err := NewClient(
			Config{BaseURL: server.URL, APIToken: faker.New().UUID().V4()},
			&http.Client{Timeout: time.Second},
		)
		require.NoError(t, err)
		return client
	}
	makeOptions := func(start time.Time) WaitOptions {
		now := start
		return WaitOptions{
			Interval: time.Second,
			Timeout:  time.Minute,
			Now:      func() time.Time { return now },
			Sleep: func(_ context.Context, duration time.Duration) error {
				now = now.Add(duration)
				return nil
			},
		}
	}

	t.Run("waits through initial 404 and succeeds", func(t *testing.T) {
		attempt := 0
		client := makeClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			attempt++
			if attempt == 1 {
				writer.WriteHeader(http.StatusNotFound)
				_, _ = writer.Write([]byte(`{"code":"not_found","message":"Missing","correlationId":"id"}`))
				return
			}
			_, _ = writer.Write([]byte(
				`{"id":"job","jobType":"sync","status":"succeeded","requester":null,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","attemptCount":1}`,
			))
		}))
		options := makeOptions(time.Now())

		job, err := client.WaitForJob(t.Context(), "job", options)

		require.NoError(t, err)
		require.Equal(t, "succeeded", job.Status)
	})

	t.Run("returns terminal failed job and late missing jobs", func(t *testing.T) {
		failed := makeClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(
				`{"id":"job","jobType":"sync","status":"failed","requester":null,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","attemptCount":1}`,
			))
		}))
		options := makeOptions(time.Now())
		job, err := failed.WaitForJob(t.Context(), "job", options)
		require.Error(t, err)
		require.Equal(t, "failed", job.Status)

		late := makeClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"code":"not_found","message":"Missing","correlationId":"id"}`))
		}))
		start := time.Now().Add(-31 * time.Second)
		options = makeOptions(start)
		_, err = late.WaitForJob(t.Context(), "job", options)
		var apiError *APIError
		require.ErrorAs(t, err, &apiError)
	})

	t.Run("bounds timeout and validates options", func(t *testing.T) {
		client := makeClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(
				`{"id":"job","jobType":"sync","status":"running","requester":null,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","attemptCount":1}`,
			))
		}))
		now := time.Now()
		options := WaitOptions{
			Interval: time.Second,
			Timeout:  time.Second,
			Now:      func() time.Time { return now },
			Sleep: func(_ context.Context, duration time.Duration) error {
				now = now.Add(duration)
				return nil
			},
		}
		_, err := client.WaitForJob(t.Context(), "job", options)
		require.Error(t, err)
		_, err = client.WaitForJob(t.Context(), "", options)
		require.Error(t, err)
		_, err = client.WaitForJob(t.Context(), "job", WaitOptions{})
		require.Error(t, err)
		require.NotErrorIs(t, err, context.Canceled)
	})

	t.Run("uses default clock and polling dependencies", func(t *testing.T) {
		client := makeClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(
				`{"id":"job","jobType":"sync","status":"succeeded","requester":null,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","attemptCount":1}`,
			))
		}))
		job, err := client.WaitForJob(t.Context(), "job", WaitOptions{Interval: time.Millisecond, Timeout: time.Second})
		require.NoError(t, err)
		require.Equal(t, "succeeded", job.Status)
	})
}
