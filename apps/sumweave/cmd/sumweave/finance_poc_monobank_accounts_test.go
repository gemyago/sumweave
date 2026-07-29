package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonobankAccountsCommand(t *testing.T) {
	fake := faker.New()

	type monobankAccountResult struct {
		ID           string         `json:"id"`
		Type         string         `json:"type,omitempty"`
		CurrencyCode int            `json:"currencyCode,omitempty"`
		Balance      int64          `json:"balance,omitempty"`
		CreditLimit  int64          `json:"creditLimit,omitempty"`
		MaskedPAN    []string       `json:"maskedPan,omitempty"`
		IBAN         string         `json:"iban,omitempty"`
		Raw          map[string]any `json:"raw"`
	}

	type monobankJarResult struct {
		ID           string         `json:"id"`
		CurrencyCode int            `json:"currencyCode,omitempty"`
		Balance      int64          `json:"balance,omitempty"`
		IBAN         string         `json:"iban,omitempty"`
		Raw          map[string]any `json:"raw"`
	}

	type monobankManagedClientResult struct {
		ClientID string                  `json:"clientId,omitempty"`
		Name     string                  `json:"name,omitempty"`
		Accounts []monobankAccountResult `json:"accounts"`
		Raw      map[string]any          `json:"raw"`
	}

	type monobankAccountsCommandResult struct {
		Provider       string                        `json:"provider"`
		Operation      string                        `json:"operation"`
		FetchedAt      string                        `json:"fetchedAt"`
		Name           string                        `json:"name,omitempty"`
		Accounts       []monobankAccountResult       `json:"accounts"`
		Jars           []monobankJarResult           `json:"jars"`
		ManagedClients []monobankManagedClientResult `json:"managedClients"`
		Raw            map[string]any                `json:"raw"`
	}

	makeRootCmd := func(t *testing.T, deps financePOCCommandDeps) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
		t.Helper()
		rootCmd := newRootCmd()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.AddCommand(newFinancePOCCmd(deps))
		return rootCmd, stdout, stderr
	}

	t.Run("loads token from env and lists accounts jars and managed clients", func(t *testing.T) {
		fetchedAt := time.Date(2026, time.June, 18, 18, 0, 0, 0, time.UTC)
		envToken := fake.Internet().Password() + "-env"
		t.Setenv("MONOBANK_TOKEN", envToken)

		requestedHeaders := http.Header{}
		requestedPath := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			requestedHeaders = r.Header.Clone()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"name":%q,
				"accounts":[{"id":"acc-primary","type":"black","currencyCode":980,"balance":12345,"creditLimit":5000,"maskedPan":["537541******1234"],"iban":"UA1234567890"}],
				"jars":[{"id":"jar-savings","currencyCode":978,"balance":777,"iban":"UA0987654321"}],
				"clients":[{"clientId":"managed-1","name":"Kid","accounts":[{"id":"acc-managed","type":"white","currencyCode":840,"balance":456,"creditLimit":0,"maskedPan":["4444********1111"]}]}]
			}`, "Owner "+fake.Person().FirstName())
		}))
		defer server.Close()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{Now: func() time.Time { return fetchedAt }})
		rootCmd.SetArgs([]string{"finance-poc", "monobank", "accounts", "--json", "--base-url", server.URL})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		var got monobankAccountsCommandResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, monobankCommandName, got.Provider)
		assert.Equal(t, "accounts", got.Operation)
		assert.Equal(t, fetchedAt.Format(time.RFC3339), got.FetchedAt)
		assert.Equal(t, "/personal/client-info", requestedPath)
		assert.Equal(t, envToken, requestedHeaders.Get("X-Token"))
		assert.Equal(t, got.Name, got.Raw["name"])
		require.Len(t, got.Accounts, 1)
		assert.Equal(t, "acc-primary", got.Accounts[0].ID)
		assert.Equal(t, 980, got.Accounts[0].CurrencyCode)
		assert.Equal(t, int64(12345), got.Accounts[0].Balance)
		assert.Equal(t, int64(5000), got.Accounts[0].CreditLimit)
		assert.Equal(t, []string{"537541******1234"}, got.Accounts[0].MaskedPAN)
		assert.Equal(t, "UA1234567890", got.Accounts[0].IBAN)
		require.Len(t, got.Jars, 1)
		assert.Equal(t, "jar-savings", got.Jars[0].ID)
		require.Len(t, got.ManagedClients, 1)
		assert.Equal(t, "managed-1", got.ManagedClients[0].ClientID)
		require.Len(t, got.ManagedClients[0].Accounts, 1)
		assert.Equal(t, "acc-managed", got.ManagedClients[0].Accounts[0].ID)
		assert.Contains(t, stderr.String(), "retrieved 1 monobank accounts, 1 jars, 1 managed clients")
		assert.NotContains(t, stdout.String(), envToken)
		assert.NotContains(t, stderr.String(), envToken)
	})

	t.Run("flag token overrides env and provider errors redact secrets", func(t *testing.T) {
		envToken := fake.Internet().Password() + "-env"
		flagToken := fake.Internet().Password() + "-flag"
		t.Setenv("MONOBANK_TOKEN", envToken)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/personal/client-info", r.URL.Path)
			assert.Equal(t, flagToken, r.Header.Get("X-Token"))
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprintf(w, `{"token":%q,"secret":%q}`, flagToken, "secret-"+fake.Internet().Password())
		}))
		defer server.Close()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{})
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"finance-poc", "monobank", "accounts", "--json",
			"--base-url", server.URL,
			"--token", flagToken,
		})

		err := rootCmd.ExecuteContext(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "status 401")
		assert.NotContains(t, err.Error(), envToken)
		assert.NotContains(t, err.Error(), flagToken)
		assert.NotContains(t, stdout.String(), envToken)
		assert.NotContains(t, stdout.String(), flagToken)
		assert.NotContains(t, stderr.String(), envToken)
		assert.NotContains(t, stderr.String(), flagToken)
		assert.Contains(t, err.Error(), `"token":"[REDACTED]"`)
	})

	t.Run("helper coverage includes missing token transport decode and normalization fallbacks", func(t *testing.T) {
		_, err := runMonobankAccounts(t.Context(), nil, financePOCCommandDeps{}, financePOCProviderRequest{})
		require.Error(t, err)
		require.ErrorContains(t, err, "monobank token is required")

		_, err = callMonobankClientInfo(t.Context(), financePOCProviderRequest{
			BaseURL: "http://[::1",
			Token:   "token-" + fake.Lorem().Word(),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "parse monobank client-info URL")

		_, err = callMonobankClientInfo(t.Context(), financePOCProviderRequest{
			BaseURL: "http://127.0.0.1:1",
			Token:   "token-" + fake.Lorem().Word(),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "request monobank client-info")

		badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"accounts":`)
		}))
		defer badJSONServer.Close()

		_, err = callMonobankClientInfo(t.Context(), financePOCProviderRequest{
			BaseURL: badJSONServer.URL,
			Token:   "token-" + fake.Lorem().Word(),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "decode monobank client-info response")

		managedClients := normalizeMonobankManagedClients(map[string]any{
			"managedClients": []any{
				map[string]any{
					"id":   "client-" + fake.Lorem().Word(),
					"name": "Name " + fake.Person().FirstName(),
					"accounts": []any{
						map[string]any{
							"id":           "acc-" + fake.Lorem().Word(),
							"type":         "type-" + fake.Lorem().Word(),
							"currencyCode": json.Number("980"),
							"balance":      int32(11),
							"creditLimit":  int64(22),
							"maskedPan":    []any{123, " 4444********1111 "},
						},
					},
				},
			},
		})
		require.Len(t, managedClients, 1)
		assert.NotEmpty(t, managedClients[0].ClientID)
		require.Len(t, managedClients[0].Accounts, 1)
		assert.Equal(t, 980, managedClients[0].Accounts[0].CurrencyCode)
		assert.Equal(t, int64(11), managedClients[0].Accounts[0].Balance)
		assert.Equal(t, int64(22), managedClients[0].Accounts[0].CreditLimit)
		assert.Equal(t, []string{"4444********1111"}, managedClients[0].Accounts[0].MaskedPAN)

		assert.Nil(t, extractMonobankObjects(map[string]any{}, "accounts"))
		assert.Nil(t, extractMonobankObjects(map[string]any{"accounts": "bad"}, "accounts"))
		assert.Empty(t, extractMonobankObjects(map[string]any{"accounts": []any{"bad"}}, "accounts"))
		assert.Empty(t, extractMonobankString(map[string]any{"id": 123}, "id"))
		assert.Nil(t, extractMonobankMaskedPANs(map[string]any{}))
		assert.Nil(t, extractMonobankMaskedPANs(map[string]any{"maskedPan": "bad"}))
		assert.Nil(t, extractMonobankMaskedPANs(map[string]any{"maskedPan": []any{123}}))
		assert.Equal(t, 0, extractMonobankInt(map[string]any{}, "currencyCode"))
		assert.Equal(t, int64(0), extractMonobankInt64(map[string]any{"balance": "bad"}, "balance"))
		assert.Equal(t, int64(7), extractMonobankInt64(map[string]any{"balance": float32(7)}, "balance"))
		assert.Equal(t, int64(8), extractMonobankInt64(map[string]any{"balance": int(8)}, "balance"))
	})
}
