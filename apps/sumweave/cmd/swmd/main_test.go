package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/swmdclient"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

//nolint:testifylint // Assertions in httptest handlers report request-contract failures.
func TestAuthCommands(t *testing.T) {
	makeDeps := func(stdin string, configDirectory string, httpClient *http.Client) (commandDeps, *bytes.Buffer) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		return commandDeps{
			stdin:      strings.NewReader(stdin),
			stdout:     stdout,
			stderr:     stderr,
			httpClient: httpClient,
			resolver: &swmdclient.Resolver{
				Getenv:        func(string) string { return "" },
				UserConfigDir: func() (string, error) { return configDirectory, nil },
			},
		}, stdout
	}

	t.Run("configures only with exact stdin opt-in", func(t *testing.T) {
		fake := faker.New()
		directory := t.TempDir()
		configPath := filepath.Join(directory, "selected.json")
		token := "swat_" + fake.UUID().V4()
		deps, stdout := makeDeps(token+"\n", directory, &http.Client{Timeout: time.Second})
		command := newRootCmd(deps)
		command.SetArgs([]string{
			"--config", configPath, "auth", "configure", "--base-url", "https://example.test", "--token-stdin",
		})

		require.NoError(t, command.Execute())
		contents, err := os.ReadFile(configPath)
		require.NoError(t, err)
		require.JSONEq(t, `{"baseUrl":"https://example.test","apiToken":"`+token+`"}`, string(contents))
		require.NotContains(t, stdout.String(), token)
	})

	t.Run("rejects every invalid configure invocation without touching configuration", func(t *testing.T) {
		fake := faker.New()
		cases := []struct {
			name  string
			args  []string
			stdin string
		}{
			{name: "missing base URL", args: []string{"auth", "configure", "--token-stdin"}, stdin: fake.UUID().V4()},
			{
				name:  "missing stdin flag",
				args:  []string{"auth", "configure", "--base-url", "https://example.test"},
				stdin: fake.UUID().V4(),
			},
			{
				name:  "stdin flag value",
				args:  []string{"auth", "configure", "--base-url", "https://example.test", "--token-stdin=secret"},
				stdin: fake.UUID().V4(),
			},
			{
				name:  "empty stdin",
				args:  []string{"auth", "configure", "--base-url", "https://example.test", "--token-stdin"},
				stdin: "\n",
			},
			{
				name:  "secret argument",
				args:  []string{"auth", "configure", "--base-url", "https://example.test", "--token", "secret"},
				stdin: fake.UUID().V4(),
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				directory := t.TempDir()
				configPath := filepath.Join(directory, "swmd.json")
				original := []byte(`{"baseUrl":"https://original.example.test","apiToken":"original"}`)
				require.NoError(t, os.WriteFile(configPath, original, 0o600))
				deps, _ := makeDeps(testCase.stdin, directory, &http.Client{Timeout: time.Second})
				command := newRootCmd(deps)
				command.SetArgs(append([]string{"--config", configPath}, testCase.args...))

				require.Error(t, command.Execute())
				contents, err := os.ReadFile(configPath)
				require.NoError(t, err)
				require.Equal(t, original, contents)
			})
		}
	})

	t.Run("shows redacted offline status and clears configuration", func(t *testing.T) {
		fake := faker.New()
		directory := t.TempDir()
		configPath := filepath.Join(directory, "swmd.json")
		token := fake.UUID().V4()
		require.NoError(t, swmdclient.WriteConfig(
			configPath,
			swmdclient.Config{BaseURL: "https://example.test", APIToken: token},
		))
		deps, stdout := makeDeps("", directory, &http.Client{Timeout: time.Second})
		command := newRootCmd(deps)
		command.SetArgs([]string{"--config", configPath, "auth", "status", "--offline"})
		require.NoError(t, command.Execute())
		require.JSONEq(t, `{"baseUrl":"https://example.test","apiToken":"redacted"}`, stdout.String())
		require.NotContains(t, stdout.String(), token)

		deps, _ = makeDeps("", directory, &http.Client{Timeout: time.Second})
		command = newRootCmd(deps)
		command.SetArgs([]string{"--config", configPath, "auth", "clear"})
		require.NoError(t, command.Execute())
		require.NoFileExists(t, configPath)
	})

	t.Run("shows redacted online status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			require.Equal(t, "/api/v1/auth/me", request.URL.Path)
			_, _ = writer.Write([]byte(`{"id":"user","username":"name"}`))
		}))
		defer server.Close()
		directory := t.TempDir()
		configPath := filepath.Join(directory, "swmd.json")
		token := "token-" + faker.New().UUID().V4()
		require.NoError(t, swmdclient.WriteConfig(configPath, swmdclient.Config{BaseURL: server.URL, APIToken: token}))
		deps, stdout := makeDeps("", directory, &http.Client{Timeout: time.Second})
		command := newRootCmd(deps)
		command.SetArgs([]string{"--config", configPath, "auth", "status"})
		require.NoError(t, command.Execute())
		require.JSONEq(
			t,
			`{"baseUrl":"`+server.URL+`","apiToken":"redacted","user":{"id":"user","username":"name"}}`,
			stdout.String(),
		)
	})
}

//nolint:testifylint,golines // Assertions in httptest handlers report request-contract failures.
func TestResourceCommands(t *testing.T) {
	makeCommand := func(t *testing.T, handler http.HandlerFunc) (*cobra.Command, *bytes.Buffer, string) {
		t.Helper()
		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)
		directory := t.TempDir()
		configPath := filepath.Join(directory, "swmd.json")
		require.NoError(t, swmdclient.WriteConfig(
			configPath,
			swmdclient.Config{BaseURL: server.URL, APIToken: "token-" + faker.New().UUID().V4()},
		))
		stdout := &bytes.Buffer{}
		deps := commandDeps{
			stdin:      strings.NewReader(""),
			stdout:     stdout,
			stderr:     &bytes.Buffer{},
			httpClient: &http.Client{Timeout: time.Second},
			resolver: &swmdclient.Resolver{
				Getenv:        func(string) string { return "" },
				UserConfigDir: func() (string, error) { return directory, nil },
			},
		}
		command := newRootCmd(deps)
		return command, stdout, configPath
	}

	cases := []struct {
		name   string
		args   []string
		method string
		path   string
		body   string
	}{
		{"tenant list", []string{"tenant", "list"}, http.MethodGet, "/api/v1/finance/tenants", `{"items":[]}`},
		{"account list", []string{"account", "list", "--tenant", "tenant"}, http.MethodGet, "/api/v1/finance/tenants/tenant/accounts", `{"items":[]}`},
		{"account get", []string{"account", "get", "--tenant", "tenant", "--account", "account"}, http.MethodGet, "/api/v1/finance/tenants/tenant/accounts/account", `{}`},
		{"account provider list", []string{"account", "provider-data-list", "--tenant", "tenant", "--account", "account"}, http.MethodGet, "/api/v1/finance/tenants/tenant/accounts/account/provider-snapshots", `{"items":[]}`},
		{"account provider get", []string{"account", "provider-data-get", "--tenant", "tenant", "--account", "account", "--snapshot", "snapshot"}, http.MethodGet, "/api/v1/finance/tenants/tenant/accounts/account/provider-snapshots/snapshot", `{}`},
		{"transaction list", []string{"transaction", "list", "--tenant", "tenant", "--limit", "1"}, http.MethodGet, "/api/v1/finance/tenants/tenant/transactions", `{"items":[]}`},
		{"transaction get", []string{"transaction", "get", "--tenant", "tenant", "--transaction", "transaction"}, http.MethodGet, "/api/v1/finance/tenants/tenant/transactions/transaction", `{}`},
		{"transaction provider list", []string{"transaction", "provider-data-list", "--tenant", "tenant", "--transaction", "transaction"}, http.MethodGet, "/api/v1/finance/tenants/tenant/transactions/transaction/provider-snapshots", `{"items":[]}`},
		{"transaction provider get", []string{"transaction", "provider-data-get", "--tenant", "tenant", "--transaction", "transaction", "--snapshot", "snapshot"}, http.MethodGet, "/api/v1/finance/tenants/tenant/transactions/transaction/provider-snapshots/snapshot", `{}`},
		{"connection list", []string{"connection", "list", "--tenant", "tenant"}, http.MethodGet, "/api/v1/finance/tenants/tenant/connections", `{"items":[]}`},
		{"connection sync", []string{"connection", "sync", "--tenant", "tenant", "--connection", "connection", "--idempotency-key", "key"}, http.MethodPost, "/api/v1/finance/tenants/tenant/connections/connection/sync", `{"jobId":"job"}`},
		{"classification", []string{"classification", "run", "--tenant", "tenant", "--range-start", "2026-01-01T00:00:00+02:00", "--range-end-exclusive", "2026-01-02T00:00:00+02:00"}, http.MethodPost, "/api/v1/finance/tenants/tenant/transactions/classify", `{"jobId":"job"}`},
		{"transfer", []string{"transfer", "match", "--tenant", "tenant", "--range-start", "2026-01-01T00:00:00+02:00", "--range-end-exclusive", "2026-01-02T00:00:00+02:00"}, http.MethodPost, "/api/v1/finance/tenants/tenant/transactions/match-transfers", `{"jobId":"job"}`},
		{"job list", []string{"job", "list"}, http.MethodGet, "/api/v1/jobs", `{"items":[],"nextCursor":""}`},
		{"job get", []string{"job", "get", "--job", "job"}, http.MethodGet, "/api/v1/jobs/job", `{"id":"job","jobType":"sync","status":"succeeded","requester":null,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","attemptCount":1}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			command, stdout, configPath := makeCommand(t, func(writer http.ResponseWriter, request *http.Request) {
				require.Equal(t, testCase.method, request.Method)
				require.Equal(t, testCase.path, request.URL.Path)
				require.True(t, strings.HasPrefix(request.Header.Get("Authorization"), "Bearer token-"))
				if testCase.method == http.MethodPost {
					require.Equal(t, "application/json", request.Header.Get("Content-Type"))
				}
				_, _ = writer.Write([]byte(testCase.body))
			})
			command.SetArgs(append([]string{"--config", configPath}, testCase.args...))

			require.NoError(t, command.Execute())
			require.True(t, json.Valid(stdout.Bytes()))
			require.NotEmpty(t, stdout.String())
		})
	}

	t.Run("validates timestamp flags and API failures", func(t *testing.T) {
		command, _, configPath := makeCommand(t, func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"code":"unauthorized","message":"Denied","correlationId":"id"}`))
		})
		command.SetArgs([]string{"--config", configPath, "transaction", "list", "--tenant", "tenant", "--start-date", "not-a-time"})
		require.Error(t, command.Execute())
		command, _, configPath = makeCommand(t, func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"code":"unauthorized","message":"Denied","correlationId":"id"}`))
		})
		command.SetArgs([]string{"--config", configPath, "tenant", "list"})
		require.Error(t, command.Execute())
	})

	t.Run("sends optional filters, pages transactions, and validates bounds", func(t *testing.T) {
		requestCount := 0
		command, stdout, configPath := makeCommand(t, func(writer http.ResponseWriter, request *http.Request) {
			requestCount++
			require.Equal(t, "account", request.URL.Query().Get("accountId"))
			require.Equal(t, "true", request.URL.Query().Get("includeHidden"))
			if requestCount == 1 {
				require.Equal(t, "0", request.URL.Query().Get("offset"))
				_, _ = writer.Write([]byte(`{"items":[{"id":"one","tenantId":"tenant","accountId":"account","source":"source","status":"booked","kind":"expense","amountMinor":1,"currency":"USD","description":"one","effectiveAt":"2026-01-01T00:00:00+02:00","createdAt":"2026-01-01T00:00:00+02:00","updatedAt":"2026-01-01T00:00:00+02:00","tagIds":[]}]}`))
				return
			}
			require.Equal(t, "1", request.URL.Query().Get("offset"))
			_, _ = writer.Write([]byte(`{"items":[]}`))
		})
		command.SetArgs([]string{"--config", configPath, "transaction", "list", "--tenant", "tenant", "--account", "account", "--source", "source", "--status", "booked", "--kind", "expense", "--start-date", "2026-01-01T00:00:00+02:00", "--end-date", "2026-01-02T00:00:00+02:00", "--sort", "asc", "--include-hidden", "--limit", "1"})
		require.NoError(t, command.Execute())
		require.Equal(t, 2, requestCount)
		require.JSONEq(t, `{"items":[{"id":"one","tenantId":"tenant","accountId":"account","source":"source","status":"booked","kind":"expense","amountMinor":1,"currency":"USD","description":"one","effectiveAt":"2026-01-01T00:00:00+02:00","createdAt":"2026-01-01T00:00:00+02:00","updatedAt":"2026-01-01T00:00:00+02:00","tagIds":[]}]}`, stdout.String())

		command, _, configPath = makeCommand(t, func(http.ResponseWriter, *http.Request) {})
		command.SetArgs([]string{"--config", configPath, "transaction", "list", "--tenant", "tenant", "--limit", "201"})
		require.Error(t, command.Execute())
		command, _, configPath = makeCommand(t, func(http.ResponseWriter, *http.Request) {})
		command.SetArgs([]string{"--config", configPath, "job", "list", "--limit", "101"})
		require.Error(t, command.Execute())
	})

	t.Run("carries sync windows and job filters", func(t *testing.T) {
		command, _, configPath := makeCommand(t, func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/api/v1/jobs" {
				require.Equal(t, []string{"failed", "queued"}, request.URL.Query()["status"])
				require.Equal(t, []string{"sync"}, request.URL.Query()["jobType"])
				require.Equal(t, []string{"integration"}, request.URL.Query()["source"])
				_, _ = writer.Write([]byte(`{"items":[],"nextCursor":""}`))
				return
			}
			require.Equal(t, "key", request.Header.Get("Idempotency-Key"))
			body := &bytes.Buffer{}
			_, _ = body.ReadFrom(request.Body)
			require.Contains(t, body.String(), "2026-01-01T00:00:00+02:00")
			_, _ = writer.Write([]byte(`{"jobId":"job"}`))
		})
		command.SetArgs([]string{"--config", configPath, "connection", "sync", "--tenant", "tenant", "--connection", "connection", "--window-start", "2026-01-01T00:00:00+02:00", "--window-end", "2026-01-02T00:00:00+02:00", "--idempotency-key", "key"})
		require.NoError(t, command.Execute())

		command, _, configPath = makeCommand(t, func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`{"items":[],"nextCursor":""}`))
		})
		command.SetArgs([]string{"--config", configPath, "job", "list", "--status", "queued,failed", "--job-type", "sync", "--source", "integration", "--cursor", "cursor"})
		require.NoError(t, command.Execute())
	})

	t.Run("waits for a terminal job and validates helpers", func(t *testing.T) {
		command, stdout, configPath := makeCommand(t, func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`{"id":"job","jobType":"sync","status":"succeeded","requester":null,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","attemptCount":1}`))
		})
		command.SetArgs([]string{"--config", configPath, "job", "wait", "--job", "job", "--interval", "1ms", "--timeout", "1s"})
		require.NoError(t, command.Execute())
		require.Contains(t, stdout.String(), `"status":"succeeded"`)
		require.Equal(t, "/api/v1/finance/tenants/a%2Fb/accounts", financeRoute("a/b", "accounts"))
		require.Equal(t, "/api/v1/jobs/a%2Fb", jobRoute("a/b"))
		parsed, err := parseTimestamp("2026-01-01T00:00:00+02:00")
		require.NoError(t, err)
		require.Equal(t, "+02:00", parsed.Format("-07:00"))
		_, err = parseTimestamp("not-a-time")
		require.Error(t, err)
	})
}
