package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnableBankingASPSPsCommand(t *testing.T) {
	fake := faker.New()

	type commandResult struct {
		Provider  string           `json:"provider"`
		Operation string           `json:"operation"`
		FetchedAt string           `json:"fetchedAt"`
		Summary   map[string]any   `json:"summary"`
		Raw       []map[string]any `json:"raw,omitempty"`
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

	writePrivateKeyFile := func(t *testing.T) (*rsa.PrivateKey, string) {
		t.Helper()
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)

		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyDER,
		})
		privateKeyPath := filepath.Join(t.TempDir(), fake.Lorem().Word()+".pem")
		require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))
		return privateKey, privateKeyPath
	}

	t.Run("signs RS256 JWT and calls GET aspsps endpoint", func(t *testing.T) {
		appID := "app-" + fake.Lorem().Word()
		country := "PL"
		fetchedAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
		privateKey, privateKeyPath := writePrivateKeyFile(t)

		var authHeader string
		var requestPath string
		var requestQuery url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			requestPath = r.URL.Path
			requestQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"pko","name":"PKO Bank Polski"}]`))
		}))
		defer server.Close()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{Now: func() time.Time { return fetchedAt }})
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "aspsps",
			"--country", country,
			"--json",
			"--base-url", server.URL,
			"--app-id", appID,
			"--private-key-path", privateKeyPath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		var got commandResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, enableBankingCommandName, got.Provider)
		assert.Equal(t, enableBankingASPSPsOperation, got.Operation)
		assert.Equal(t, fetchedAt.Format(time.RFC3339), got.FetchedAt)
		assert.Equal(t, country, got.Summary["country"])
		require.Len(t, got.Raw, 1)
		assert.Equal(t, "pko", got.Raw[0]["id"])

		assert.Equal(t, "/aspsps", requestPath)
		assert.Equal(t, country, requestQuery.Get("country"))
		require.Greater(t, len(authHeader), len("Bearer "))
		assert.Contains(t, authHeader, "Bearer ")

		tokenString := authHeader[len("Bearer "):]
		registeredClaims := &jwt.RegisteredClaims{}
		parsedToken, err := jwt.ParseWithClaims(tokenString, registeredClaims, func(token *jwt.Token) (any, error) {
			assert.Equal(t, jwt.SigningMethodRS256.Alg(), token.Method.Alg())
			assert.Equal(t, appID, token.Header["kid"])
			return &privateKey.PublicKey, nil
		})
		require.NoError(t, err)
		require.True(t, parsedToken.Valid)
		assert.Equal(t, enableBankingJWTIssuer, registeredClaims.Issuer)
		assert.Equal(t, jwt.ClaimStrings{enableBankingJWTAudience}, registeredClaims.Audience)
		require.NotNil(t, registeredClaims.IssuedAt)
		require.NotNil(t, registeredClaims.ExpiresAt)
		assert.Equal(t, fetchedAt.Unix(), registeredClaims.IssuedAt.Time.Unix())
		assert.Equal(t, fetchedAt.Add(enableBankingJWTLifetime).Unix(), registeredClaims.ExpiresAt.Time.Unix())

		assert.NotContains(t, stdout.String(), tokenString)
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		assert.NotContains(
			t,
			stdout.String(),
			string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})),
		)
		assert.NotContains(t, stderr.String(), tokenString)
		assert.NotContains(t, stderr.String(), "Bearer ")
	})

	t.Run("flags override env for base url, app id, and private key path", func(t *testing.T) {
		envAppID := "env-" + fake.Lorem().Word()
		flagAppID := "flag-" + fake.Lorem().Word()
		envBaseURL := fmt.Sprintf("https://%s.example.test", fake.Lorem().Word())
		flagBaseURL := fmt.Sprintf("https://%s.example.test", fake.Lorem().Word())
		_, envPrivateKeyPath := writePrivateKeyFile(t)
		_, flagPrivateKeyPath := writePrivateKeyFile(t)

		t.Setenv("ENABLE_BANKING_APP_ID", envAppID)
		t.Setenv("ENABLE_BANKING_BASE_URL", envBaseURL)
		t.Setenv("ENABLE_BANKING_PRIVATE_KEY_PATH", envPrivateKeyPath)

		var captured financePOCProviderRequest
		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{
			EnableBankingRunner: func(_ context.Context, request financePOCProviderRequest) (financePOCProviderResult, error) {
				captured = request
				return financePOCProviderResult{
					Summary: map[string]any{
						"baseURL":              request.BaseURL,
						"appIDConfigured":      request.AppID != "",
						"appIDSource":          request.AppIDSource,
						"privateKeySource":     request.KeyPathSource,
						"privateKeyConfigured": request.PrivateKeyPath != "",
					},
				}, nil
			},
		})

		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "aspsps",
			"--country", "PL",
			"--json",
			"--base-url", flagBaseURL,
			"--app-id", flagAppID,
			"--private-key-path", flagPrivateKeyPath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		assert.Equal(t, enableBankingASPSPsOperation, captured.Operation)
		assert.Equal(t, flagBaseURL, captured.BaseURL)
		assert.Equal(t, flagAppID, captured.AppID)
		assert.Equal(t, financePOCAppIDSourceFlag, captured.AppIDSource)
		assert.Equal(t, flagPrivateKeyPath, captured.PrivateKeyPath)
		assert.Equal(t, financePOCKeyPathSourceFlag, captured.KeyPathSource)
		assert.NotContains(t, stdout.String(), envPrivateKeyPath)
		assert.NotContains(t, stdout.String(), flagPrivateKeyPath)
		assert.NotContains(t, stderr.String(), envPrivateKeyPath)
		assert.NotContains(t, stderr.String(), flagPrivateKeyPath)
	})

	t.Run("helpers cover validation and sanitized failures", func(t *testing.T) {
		t.Run("provider timeout helper sanitizes non-timeout errors", func(t *testing.T) {
			secretToken := "secret.token.value"
			result, err := runFinancePOCProviderWithTimeout(
				t.Context(),
				financePOCProviderRequest{Provider: enableBankingCommandName},
				func(_ context.Context, _ financePOCProviderRequest) (financePOCProviderResult, error) {
					return financePOCProviderResult{}, fmt.Errorf("Authorization: Bearer %s", secretToken)
				},
			)
			require.Error(t, err)
			assert.Equal(t, financePOCProviderResult{}, result)
			assert.NotContains(t, err.Error(), secretToken)
			assert.Contains(t, err.Error(), "Bearer [REDACTED]")
		})

		t.Run("default runner returns status summary when operation is not aspsps", func(t *testing.T) {
			result, err := defaultFinancePOCProviderRunner(
				t.Context(),
				financePOCProviderRequest{Provider: enableBankingCommandName, BaseURL: enableBankingDefaultBaseURL},
				nil,
			)
			require.NoError(t, err)
			assert.Equal(t, enableBankingDefaultBaseURL, result.Summary[financePOCSummaryBaseURLKey])
		})

		t.Run("aspsps validates required request fields", func(t *testing.T) {
			_, err := runEnableBankingASPSPs(t.Context(), financePOCProviderRequest{}, time.Now)
			require.Error(t, err)
			require.ErrorContains(t, err, "app ID is required")

			_, err = runEnableBankingASPSPs(
				t.Context(),
				financePOCProviderRequest{AppID: "app"},
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "private key path is required")

			_, err = runEnableBankingASPSPs(
				t.Context(),
				financePOCProviderRequest{AppID: "app", PrivateKeyPath: filepath.Join(t.TempDir(), "key.pem")},
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "country is required")
		})

		t.Run("aspsps surfaces sanitized response and decode errors", func(t *testing.T) {
			_, privateKeyPath := writePrivateKeyFile(t)
			request := financePOCProviderRequest{
				Provider:       enableBankingCommandName,
				Operation:      enableBankingASPSPsOperation,
				Country:        "PL",
				BaseURL:        "://bad",
				AppID:          "app-" + fake.Lorem().Word(),
				PrivateKeyPath: privateKeyPath,
			}

			_, err := runEnableBankingASPSPs(t.Context(), request, time.Now)
			require.Error(t, err)
			require.ErrorContains(t, err, "parse enable-banking aspsps URL")

			serverWithBearerError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"token":"secret","message":"Authorization: Bearer top-secret"}`))
			}))
			defer serverWithBearerError.Close()

			request.BaseURL = serverWithBearerError.URL
			_, err = runEnableBankingASPSPs(t.Context(), request, time.Now)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"token":"[REDACTED]"`)
			assert.NotContains(t, err.Error(), "top-secret")

			serverWithBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("{"))
			}))
			defer serverWithBadJSON.Close()

			request.BaseURL = serverWithBadJSON.URL
			_, err = runEnableBankingASPSPs(t.Context(), request, time.Now)
			require.Error(t, err)
			require.ErrorContains(t, err, "decode enable-banking aspsps response")
		})

		t.Run("private key loading errors are wrapped without leaking contents", func(t *testing.T) {
			_, err := loadEnableBankingPrivateKey(filepath.Join(t.TempDir(), fake.Lorem().Word()+".pem"))
			require.Error(t, err)
			require.ErrorContains(t, err, "read enable-banking private key file")

			invalidKeyPath := filepath.Join(t.TempDir(), fake.Lorem().Word()+".pem")
			require.NoError(t, os.WriteFile(invalidKeyPath, []byte("not-a-key"), 0o600))
			_, err = loadEnableBankingPrivateKey(invalidKeyPath)
			require.Error(t, err)
			require.ErrorContains(t, err, "parse enable-banking private key file")
		})
	})
}
