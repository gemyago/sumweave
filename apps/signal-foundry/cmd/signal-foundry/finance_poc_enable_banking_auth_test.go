package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type authURLCheckWriter struct {
	buffer   bytes.Buffer
	authFile string
	t        *testing.T
}

func (w *authURLCheckWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "https://bank.example.test/authorize?flow=test") {
		_, err := os.Stat(w.authFile)
		require.NoError(w.t, err)
	}
	return w.buffer.Write(p)
}

func TestEnableBankingAuthCommands(t *testing.T) {
	fake := faker.New()

	type pendingAuthResult struct {
		Provider         string                          `json:"provider"`
		Kind             string                          `json:"kind"`
		CreatedAt        string                          `json:"createdAt"`
		State            string                          `json:"state"`
		Request          enableBankingPendingAuthRequest `json:"request"`
		AuthorizationURL string                          `json:"authorizationUrl"`
		AuthID           string                          `json:"authId,omitempty"`
		Raw              map[string]any                  `json:"raw"`
	}

	type sessionResult struct {
		Provider           string           `json:"provider"`
		CreatedAt          string           `json:"createdAt"`
		Country            string           `json:"country"`
		ASPSPName          string           `json:"aspspName"`
		PSUType            string           `json:"psuType"`
		SessionID          string           `json:"sessionId"`
		AccessValidForDays int              `json:"accessValidForDays,omitempty"`
		Accounts           []map[string]any `json:"accounts,omitempty"`
		Raw                map[string]any   `json:"raw"`
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

		privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: enableBankingPrivateKeyType, Bytes: privateKeyDER})
		privateKeyPath := filepath.Join(t.TempDir(), fake.Lorem().Word()+".pem")
		require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))
		return privateKey, privateKeyPath
	}

	writeLocalhostTLSCertFiles := func(t *testing.T) (string, string) {
		t.Helper()
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		certificateTemplate := &x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			Subject:      pkix.Name{CommonName: enableBankingLocalhost},
			NotBefore:    time.Now().Add(-time.Minute),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth,
			},
			DNSNames:    []string{enableBankingLocalhost},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		}

		certificateDER, err := x509.CreateCertificate(
			rand.Reader,
			certificateTemplate,
			certificateTemplate,
			&privateKey.PublicKey,
			privateKey,
		)
		require.NoError(t, err)

		certificatePath := filepath.Join(t.TempDir(), "localhost-cert.pem")
		keyPath := filepath.Join(t.TempDir(), "localhost-key.pem")
		require.NoError(
			t,
			os.WriteFile(
				certificatePath,
				pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}),
				0o600,
			),
		)

		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		require.NoError(
			t,
			os.WriteFile(
				keyPath,
				pem.EncodeToMemory(&pem.Block{Type: enableBankingPrivateKeyType, Bytes: privateKeyDER}),
				0o600,
			),
		)

		return certificatePath, keyPath
	}

	reserveListenAddr := func(t *testing.T) string {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		require.NoError(t, listener.Close())
		return addr
	}

	reserveListenAddrForHost := func(t *testing.T, host string) string {
		t.Helper()
		listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
		require.NoError(t, err)
		tcpAddr, ok := listener.Addr().(*net.TCPAddr)
		require.True(t, ok)
		require.NoError(t, listener.Close())
		return net.JoinHostPort(host, strconv.Itoa(tcpAddr.Port))
	}

	t.Run("start-auth writes pending auth before printing auth url", func(t *testing.T) {
		appID := "app-" + fake.Lorem().Word()
		privateKey, privateKeyPath := writePrivateKeyFile(t)
		fetchedAt := time.Date(2026, time.June, 18, 13, 0, 0, 0, time.UTC)
		authFile := filepath.Join(t.TempDir(), fake.Lorem().Word()+".pending.json")
		redirectURL := fmt.Sprintf("https://%s.example.test/callback", fake.Lorem().Word())

		var postedBody map[string]any
		var authHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/auth", r.URL.Path)
			authHeader = r.Header.Get("Authorization")
			defer r.Body.Close()
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&postedBody))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"url":"https://bank.example.test/authorize?flow=test","id":"auth-123","session":{"id":"provider-auth-1"}}`,
			))
		}))
		defer server.Close()

		rootCmd, stdout, _ := makeRootCmd(t, financePOCCommandDeps{
			Now: func() time.Time { return fetchedAt },
		})
		stderr := &authURLCheckWriter{authFile: authFile, t: t}
		rootCmd.SetErr(stderr)

		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "start-auth",
			"--country", "PL",
			"--aspsp-name", "PKO Bank Polski",
			"--psu-type", "personal",
			"--valid-days", "90",
			"--redirect-url", redirectURL,
			"--auth-file", authFile,
			"--json",
			"--base-url", server.URL,
			"--app-id", appID,
			"--private-key-path", privateKeyPath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		var got pendingAuthResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, enableBankingCommandName, got.Provider)
		assert.Equal(t, "pending-auth", got.Kind)
		assert.Equal(t, fetchedAt.Format(time.RFC3339), got.CreatedAt)
		assert.NotEmpty(t, got.State)
		assert.Equal(t, "PL", got.Request.Country)
		assert.Equal(t, "PKO Bank Polski", got.Request.ASPSPName)
		assert.Equal(t, "personal", got.Request.PSUType)
		assert.Equal(t, 90, got.Request.ValidDays)
		assert.Equal(t, redirectURL, got.Request.RedirectURL)
		assert.Equal(t, "https://bank.example.test/authorize?flow=test", got.AuthorizationURL)
		assert.Equal(t, "auth-123", got.AuthID)
		assert.Equal(t, "provider-auth-1", got.Raw["session"].(map[string]any)["id"])

		filePayload, err := os.ReadFile(authFile)
		require.NoError(t, err)
		assert.JSONEq(t, stdout.String(), string(filePayload))

		info, err := os.Stat(authFile)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

		aspsp, ok := postedBody["aspsp"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "PL", aspsp["country"])
		assert.Equal(t, "PKO Bank Polski", aspsp["name"])
		access, ok := postedBody["access"].(map[string]any)
		require.True(t, ok)
		validUntil, parseErr := time.Parse(time.RFC3339, access["valid_until"].(string))
		require.NoError(t, parseErr)
		assert.Equal(t, fetchedAt.Add(90*24*time.Hour).UTC(), validUntil.UTC())
		assert.Equal(t, "personal", postedBody["psu_type"])
		assert.Equal(t, redirectURL, postedBody["redirect_url"])
		assert.Equal(t, got.State, postedBody["state"])

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		registeredClaims := &jwt.RegisteredClaims{}
		parsedToken, err := jwt.ParseWithClaims(tokenString, registeredClaims, func(token *jwt.Token) (any, error) {
			assert.Equal(t, jwt.SigningMethodRS256.Alg(), token.Method.Alg())
			assert.Equal(t, appID, token.Header["kid"])
			return &privateKey.PublicKey, nil
		}, jwt.WithTimeFunc(func() time.Time { return fetchedAt }))
		require.NoError(t, err)
		require.True(t, parsedToken.Valid)

		assert.Contains(t, stderr.buffer.String(), "https://bank.example.test/authorize?flow=test")
		assert.NotContains(t, stderr.buffer.String(), tokenString)
		assert.NotContains(t, stdout.String(), tokenString)
	})

	t.Run("start-auth opens browser only when opt-in flag is set", func(t *testing.T) {
		_, privateKeyPath := writePrivateKeyFile(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"https://bank.example.test/authorize","id":"auth-789"}`))
		}))
		defer server.Close()

		openedURLs := make([]string, 0, 1)
		rootCmd, _, _ := makeRootCmd(t, financePOCCommandDeps{
			EnableBankingOpenBrowser: func(targetURL string) error {
				openedURLs = append(openedURLs, targetURL)
				return nil
			},
		})

		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "start-auth",
			"--country", "PL",
			"--aspsp-name", "PKO",
			"--psu-type", "personal",
			"--valid-days", "30",
			"--redirect-url", "https://example.test/callback",
			"--auth-file", filepath.Join(t.TempDir(), "pending.json"),
			"--base-url", server.URL,
			"--app-id", "app-" + fake.Lorem().Word(),
			"--private-key-path", privateKeyPath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		assert.Empty(t, openedURLs)

		rootCmd, _, _ = makeRootCmd(t, financePOCCommandDeps{
			EnableBankingOpenBrowser: func(targetURL string) error {
				openedURLs = append(openedURLs, targetURL)
				return nil
			},
		})
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "start-auth",
			"--country", "PL",
			"--aspsp-name", "PKO",
			"--psu-type", "personal",
			"--valid-days", "30",
			"--redirect-url", "https://example.test/callback",
			"--auth-file", filepath.Join(t.TempDir(), "pending-open.json"),
			"--base-url", server.URL,
			"--app-id", "app-" + fake.Lorem().Word(),
			"--private-key-path", privateKeyPath,
			"--open-browser",
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		require.Len(t, openedURLs, 1)
		assert.Equal(t, "https://bank.example.test/authorize", openedURLs[0])
	})

	t.Run("finish-session verifies state and writes session file", func(t *testing.T) {
		appID := "app-" + fake.Lorem().Word()
		privateKey, privateKeyPath := writePrivateKeyFile(t)
		pendingState := "state-" + fake.Lorem().Word()
		sessionFile := filepath.Join(t.TempDir(), fake.Lorem().Word()+".session.json")
		authFile := filepath.Join(t.TempDir(), fake.Lorem().Word()+".pending.json")
		fetchedAt := time.Date(2026, time.June, 18, 14, 0, 0, 0, time.UTC)

		pendingAuth := enableBankingPendingAuthFile{
			Provider:         enableBankingCommandName,
			Kind:             "pending-auth",
			CreatedAt:        fetchedAt.Add(-time.Minute).Format(time.RFC3339),
			State:            pendingState,
			AuthorizationURL: "https://bank.example.test/authorize",
			Request: enableBankingPendingAuthRequest{
				Country:     "PL",
				ASPSPName:   "PKO Bank Polski",
				PSUType:     "business",
				ValidDays:   180,
				RedirectURL: "https://example.test/callback",
			},
			Raw: map[string]any{"authorization": map[string]any{"id": "auth-456"}},
		}
		pendingBytes, err := json.MarshalIndent(pendingAuth, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(authFile, append(pendingBytes, '\n'), 0o600))

		var postedBody map[string]any
		var authHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/sessions", r.URL.Path)
			authHeader = r.Header.Get("Authorization")
			defer r.Body.Close()
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&postedBody))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"session":{"session_id":"session-123"},"access":{"valid_for_days":180},"accounts":[{"uid":"acc-1"}],"status":"AUTHORIZED"}`,
			))
		}))
		defer server.Close()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{Now: func() time.Time { return fetchedAt }})
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "finish-session",
			"--auth-file", authFile,
			"--code", "manual-code-123",
			"--state", pendingState,
			"--session-file", sessionFile,
			"--json",
			"--base-url", server.URL,
			"--app-id", appID,
			"--private-key-path", privateKeyPath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		var got sessionResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, enableBankingCommandName, got.Provider)
		assert.Equal(t, fetchedAt.Format(time.RFC3339), got.CreatedAt)
		assert.Equal(t, pendingAuth.Request.Country, got.Country)
		assert.Equal(t, pendingAuth.Request.ASPSPName, got.ASPSPName)
		assert.Equal(t, pendingAuth.Request.PSUType, got.PSUType)
		assert.Equal(t, "session-123", got.SessionID)
		assert.Equal(t, 180, got.AccessValidForDays)
		require.Len(t, got.Accounts, 1)
		assert.Equal(t, "acc-1", got.Accounts[0]["uid"])
		assert.Equal(t, "AUTHORIZED", got.Raw["status"])

		assert.Equal(t, "manual-code-123", postedBody["code"])
		assert.NotContains(t, postedBody, "state")
		assert.NotContains(t, postedBody, "auth")

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		registeredClaims := &jwt.RegisteredClaims{}
		parsedToken, err := jwt.ParseWithClaims(tokenString, registeredClaims, func(token *jwt.Token) (any, error) {
			assert.Equal(t, appID, token.Header["kid"])
			return &privateKey.PublicKey, nil
		}, jwt.WithTimeFunc(func() time.Time { return fetchedAt }))
		require.NoError(t, err)
		require.True(t, parsedToken.Valid)

		filePayload, err := os.ReadFile(sessionFile)
		require.NoError(t, err)
		assert.JSONEq(t, stdout.String(), string(filePayload))
		assert.NotContains(t, stderr.String(), tokenString)

		rootCmd, _, _ = makeRootCmd(t, financePOCCommandDeps{})
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "finish-session",
			"--auth-file", authFile,
			"--code", "manual-code-123",
			"--state", "wrong-" + fake.Lorem().Word(),
			"--session-file", filepath.Join(t.TempDir(), "nope.json"),
			"--base-url", server.URL,
			"--app-id", appID,
			"--private-key-path", privateKeyPath,
		})
		err = rootCmd.ExecuteContext(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "state mismatch")
	})

	t.Run(
		"connect preserves localhost host when callback port is auto-selected and rejects mismatched state",
		func(t *testing.T) {
			_, privateKeyPath := writePrivateKeyFile(t)
			callbackAddr := net.JoinHostPort(enableBankingLocalhost, "0")
			sessionFile := filepath.Join(t.TempDir(), fake.Lorem().Word()+".session.json")
			callbackCode := "callback-code-123"

			var postedAuthBody map[string]any
			var postedSessionBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/auth":
					defer r.Body.Close()
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&postedAuthBody))
					_, _ = w.Write([]byte(`{"url":"https://bank.example.test/authorize","id":"auth-connect"}`))
				case "/sessions":
					defer r.Body.Close()
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&postedSessionBody))
					_, _ = w.Write([]byte(
						`{"session":{"id":"session-connect"},"access":{"valid_for_days":30},"accounts":[{"uid":"acc-connect"}]}`,
					))
				default:
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()

			insecureHTTPSClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true},
				},
			}

			go func() {
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					stateValue, _ := postedAuthBody["state"].(string)
					redirectURL, _ := postedAuthBody["redirect_url"].(string)
					if stateValue == "" || redirectURL == "" {
						time.Sleep(10 * time.Millisecond)
						continue
					}
					callbackRequestURL := redirectURL +
						"?code=" + url.QueryEscape(callbackCode) +
						"&state=" + url.QueryEscape(stateValue)
					response, err := insecureHTTPSClient.Get(callbackRequestURL)
					if err == nil {
						_ = response.Body.Close()
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
			}()

			rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{})
			rootCmd.SetArgs([]string{
				"finance-poc", "enable-banking", "connect",
				"--country", "PL",
				"--aspsp-name", "PKO Bank Polski",
				"--psu-type", "personal",
				"--valid-days", "30",
				"--callback-listen-addr", callbackAddr,
				"--session-file", sessionFile,
				"--json",
				"--base-url", server.URL,
				"--app-id", "app-" + fake.Lorem().Word(),
				"--private-key-path", privateKeyPath,
			})
			require.NoError(t, rootCmd.ExecuteContext(t.Context()))

			var got sessionResult
			require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
			redirectURL, ok := postedAuthBody["redirect_url"].(string)
			require.True(t, ok)
			parsedRedirectURL, err := url.Parse(redirectURL)
			require.NoError(t, err)
			assert.Equal(t, enableBankingLocalhost, parsedRedirectURL.Hostname())
			assert.NotEqual(t, "0", parsedRedirectURL.Port())
			assert.Equal(t, "/callback", parsedRedirectURL.Path)
			assert.Equal(t, "PL", got.Country)
			assert.Equal(t, "session-connect", got.SessionID)
			assert.Equal(t, callbackCode, postedSessionBody["code"])
			assert.NotContains(t, postedSessionBody, "state")
			assert.Contains(t, stderr.String(), "waiting for callback on "+redirectURL)
			assert.Contains(t, stderr.String(), "self-signed")
			assert.NotContains(t, stderr.String(), enableBankingPrivateKeyType)

			mismatchAddr := reserveListenAddr(t)
			go func() {
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					_, getErr := insecureHTTPSClient.Get(
						"https://" + mismatchAddr + "/callback?code=bad-code&state=wrong-state",
					)
					if getErr == nil {
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
			}()

			rootCmd, _, _ = makeRootCmd(t, financePOCCommandDeps{})
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.SetArgs([]string{
				"finance-poc", "enable-banking", "connect",
				"--country", "PL",
				"--aspsp-name", "PKO Bank Polski",
				"--psu-type", "personal",
				"--valid-days", "30",
				"--callback-listen-addr", mismatchAddr,
				"--session-file", filepath.Join(t.TempDir(), "mismatch.json"),
				"--base-url", server.URL,
				"--app-id", "app-" + fake.Lorem().Word(),
				"--private-key-path", privateKeyPath,
			})
			execErr := rootCmd.ExecuteContext(t.Context())
			require.Error(t, execErr)
			require.ErrorContains(t, execErr, "state mismatch")
		},
	)

	t.Run("connect uses provided callback cert pair and validates partial input", func(t *testing.T) {
		_, privateKeyPath := writePrivateKeyFile(t)
		callbackAddr := reserveListenAddrForHost(t, enableBankingLocalhost)
		sessionFile := filepath.Join(t.TempDir(), "paired-session.json")
		callbackURL := "https://" + callbackAddr + "/callback"
		callbackCode := "callback-code-paired"
		certFile, keyFile := writeLocalhostTLSCertFiles(t)
		certificatePool := x509.NewCertPool()
		certificatePEM, err := os.ReadFile(certFile)
		require.NoError(t, err)
		require.True(t, certificatePool.AppendCertsFromPEM(certificatePEM))

		var postedAuthBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/auth":
				defer r.Body.Close()
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&postedAuthBody))
				_, _ = w.Write([]byte(`{"url":"https://bank.example.test/authorize","id":"auth-connect"}`))
			case "/sessions":
				_, _ = w.Write([]byte(`{"id":"session-connect-paired"}`))
			default:
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
		}))
		defer server.Close()

		go func() {
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				stateValue, _ := postedAuthBody["state"].(string)
				if stateValue == "" {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				transport := &http.Transport{TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					RootCAs:    certificatePool,
				}}
				response, getErr := (&http.Client{Transport: transport}).Get(
					callbackURL + "?code=" + url.QueryEscape(callbackCode) + "&state=" + url.QueryEscape(stateValue),
				)
				if getErr == nil {
					_ = response.Body.Close()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{})
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "connect",
			"--country", "PL",
			"--aspsp-name", "PKO Bank Polski",
			"--psu-type", "personal",
			"--valid-days", "30",
			"--callback-listen-addr", callbackAddr,
			"--callback-cert-file", certFile,
			"--callback-key-file", keyFile,
			"--session-file", sessionFile,
			"--json",
			"--base-url", server.URL,
			"--app-id", "app-paired-" + fake.Lorem().Word(),
			"--private-key-path", privateKeyPath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		var got sessionResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, "session-connect-paired", got.SessionID)
		assert.Equal(t, callbackURL, postedAuthBody["redirect_url"])
		assert.Contains(t, stderr.String(), "waiting for callback on "+callbackURL)
		assert.NotContains(t, stderr.String(), "self-signed")

		rootCmd, _, _ = makeRootCmd(t, financePOCCommandDeps{})
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "connect",
			"--country", "PL",
			"--aspsp-name", "PKO Bank Polski",
			"--psu-type", "personal",
			"--valid-days", "30",
			"--callback-listen-addr", reserveListenAddr(t),
			"--callback-cert-file", certFile,
			"--session-file", filepath.Join(t.TempDir(), "bad.json"),
			"--base-url", server.URL,
			"--app-id", "app-partial-" + fake.Lorem().Word(),
			"--private-key-path", privateKeyPath,
		})
		err = rootCmd.ExecuteContext(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "callback cert and key files must be provided together")
	})

	t.Run("state generator returns distinct opaque values", func(t *testing.T) {
		stateOne, err := newEnableBankingState()
		require.NoError(t, err)
		stateTwo, err := newEnableBankingState()
		require.NoError(t, err)
		assert.NotEmpty(t, stateOne)
		assert.NotEmpty(t, stateTwo)
		assert.NotEqual(t, stateOne, stateTwo)
	})

	t.Run("helpers cover validation and failure paths", func(t *testing.T) {
		appID := "app-" + fake.Lorem().Word()
		_, privateKeyPath := writePrivateKeyFile(t)
		baseRequest := financePOCProviderRequest{
			Provider:       enableBankingCommandName,
			BaseURL:        "https://example.test",
			AppID:          appID,
			PrivateKeyPath: privateKeyPath,
			Timeout:        time.Second,
		}
		pendingRequest := enableBankingPendingAuthRequest{
			Country:     "PL",
			ASPSPName:   "PKO Bank Polski",
			PSUType:     "personal",
			ValidDays:   30,
			RedirectURL: "https://example.test/callback",
		}

		t.Run("state generation failure is wrapped", func(t *testing.T) {
			_, err := runEnableBankingStartAuth(t.Context(), financePOCCommandDeps{
				EnableBankingState: func() (string, error) {
					return "", assert.AnError
				},
			}, baseRequest, enableBankingStartAuthParams{
				Country:     pendingRequest.Country,
				ASPSPName:   pendingRequest.ASPSPName,
				PSUType:     pendingRequest.PSUType,
				ValidDays:   pendingRequest.ValidDays,
				RedirectURL: pendingRequest.RedirectURL,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "generate enable-banking state")
		})

		t.Run("pending auth file loading surfaces read and decode errors", func(t *testing.T) {
			_, err := loadEnableBankingPendingAuthFile(filepath.Join(t.TempDir(), "missing.json"))
			require.Error(t, err)
			require.ErrorContains(t, err, "read enable-banking auth file")

			invalidPath := filepath.Join(t.TempDir(), "invalid.json")
			require.NoError(t, os.WriteFile(invalidPath, []byte("{"), 0o600))
			_, err = loadEnableBankingPendingAuthFile(invalidPath)
			require.Error(t, err)
			require.ErrorContains(t, err, "decode enable-banking auth file")
		})

		t.Run("session file loading supports success and decode errors", func(t *testing.T) {
			sessionPath := filepath.Join(t.TempDir(), "session.json")
			expected := enableBankingSessionFile{
				Provider:  enableBankingCommandName,
				SessionID: "session-" + fake.Lorem().Word(),
			}
			payload, marshalErr := json.Marshal(expected)
			require.NoError(t, marshalErr)
			require.NoError(t, os.WriteFile(sessionPath, payload, 0o600))

			loaded, loadErr := loadEnableBankingSessionFile(sessionPath)
			require.NoError(t, loadErr)
			assert.Equal(t, expected, loaded)

			invalidPath := filepath.Join(t.TempDir(), "invalid-session.json")
			require.NoError(t, os.WriteFile(invalidPath, []byte("{"), 0o600))
			_, loadErr = loadEnableBankingSessionFile(invalidPath)
			require.Error(t, loadErr)
			require.ErrorContains(t, loadErr, "decode enable-banking session file")
		})

		t.Run("validation helpers reject missing fields", func(t *testing.T) {
			require.ErrorContains(
				t,
				validateEnableBankingCredentials(financePOCProviderRequest{}),
				"app ID is required",
			)
			require.ErrorContains(
				t,
				validateEnableBankingCredentials(financePOCProviderRequest{AppID: appID}),
				"private key path is required",
			)

			require.ErrorContains(
				t,
				validateEnableBankingStartAuthRequest(baseRequest, enableBankingPendingAuthRequest{}),
				"country is required",
			)
			require.ErrorContains(
				t,
				validateEnableBankingStartAuthRequest(baseRequest, enableBankingPendingAuthRequest{Country: "PL"}),
				"aspsp name is required",
			)
			require.ErrorContains(
				t,
				validateEnableBankingStartAuthRequest(
					baseRequest,
					enableBankingPendingAuthRequest{Country: "PL", ASPSPName: "Bank"},
				),
				"psu type is required",
			)
			require.ErrorContains(
				t,
				validateEnableBankingStartAuthRequest(
					baseRequest,
					enableBankingPendingAuthRequest{Country: "PL", ASPSPName: "Bank", PSUType: "personal"},
				),
				"valid days must be greater than zero",
			)
			require.ErrorContains(
				t,
				validateEnableBankingStartAuthRequest(
					baseRequest,
					enableBankingPendingAuthRequest{
						Country:   "PL",
						ASPSPName: "Bank",
						PSUType:   "personal",
						ValidDays: 30,
					},
				),
				"redirect URL is required",
			)
		})

		t.Run("start auth helper validates and sanitizes failures", func(t *testing.T) {
			_, err := startEnableBankingAuthorization(
				t.Context(),
				financePOCProviderRequest{},
				pendingRequest,
				"state-1",
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "app ID is required")

			_, err = callEnableBankingJSONEndpoint(
				t.Context(),
				financePOCProviderRequest{
					Provider:       enableBankingCommandName,
					BaseURL:        "://bad",
					AppID:          appID,
					PrivateKeyPath: privateKeyPath,
				},
				http.MethodPost,
				"/auth",
				map[string]any{"bad": func() {}},
				time.Now,
			)
			require.Error(t, err)

			serverWithErrors := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&map[string]any{}))
				if r.URL.Path == "/auth" {
					if r.Header.Get("X-Case") == "missing-url" {
						_, _ = w.Write([]byte(`{"id":"auth-only"}`))
						return
					}
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(`{"token":"secret","message":"Authorization: Bearer top-secret"}`))
				}
			}))
			defer serverWithErrors.Close()

			request := baseRequest
			request.BaseURL = serverWithErrors.URL
			_, err = startEnableBankingAuthorization(t.Context(), request, pendingRequest, "state-1", time.Now)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"token":"[REDACTED]"`)
			assert.NotContains(t, err.Error(), "top-secret")

			serverMissingURL := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"auth-only"}`))
			}))
			defer serverMissingURL.Close()
			request.BaseURL = serverMissingURL.URL
			_, err = startEnableBankingAuthorization(t.Context(), request, pendingRequest, "state-2", time.Now)
			require.Error(t, err)
			require.ErrorContains(t, err, "missing authorization URL")

			serverNestedAuthID := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(
					`{"url":"https://bank.example.test/authorize","session":{"id":"session-auth-123"}}`,
				))
			}))
			defer serverNestedAuthID.Close()
			request.BaseURL = serverNestedAuthID.URL
			pendingAuth, err := startEnableBankingAuthorization(
				t.Context(),
				request,
				pendingRequest,
				"state-nested-auth",
				time.Now,
			)
			require.NoError(t, err)
			assert.Equal(t, "session-auth-123", pendingAuth.AuthID)

			serverBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("{"))
			}))
			defer serverBadJSON.Close()
			request.BaseURL = serverBadJSON.URL
			_, err = startEnableBankingAuthorization(t.Context(), request, pendingRequest, "state-3", time.Now)
			require.Error(t, err)
			require.ErrorContains(t, err, "decode enable-banking /auth response")
		})

		t.Run("session exchange validates response shape", func(t *testing.T) {
			pendingAuth := enableBankingPendingAuthFile{
				Provider: enableBankingCommandName,
				State:    "state-123",
				Request:  pendingRequest,
				Raw:      map[string]any{"id": "auth-123"},
			}

			_, err := exchangeEnableBankingSession(
				t.Context(),
				baseRequest,
				pendingAuth,
				"",
				filepath.Join(t.TempDir(), "session.json"),
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "code is required")

			_, err = exchangeEnableBankingSession(
				t.Context(),
				baseRequest,
				pendingAuth,
				"code-123",
				"",
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "session file is required")

			serverMissingSessionID := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"accounts":[{"uid":"acc-1"}]}`))
			}))
			defer serverMissingSessionID.Close()
			request := baseRequest
			request.BaseURL = serverMissingSessionID.URL
			_, err = exchangeEnableBankingSession(
				t.Context(),
				request,
				pendingAuth,
				"code-123",
				filepath.Join(t.TempDir(), "session.json"),
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "missing session ID")

			serverBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("{"))
			}))
			defer serverBadJSON.Close()
			request.BaseURL = serverBadJSON.URL
			_, err = exchangeEnableBankingSession(
				t.Context(),
				request,
				pendingAuth,
				"code-123",
				filepath.Join(t.TempDir(), "session.json"),
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "decode enable-banking /sessions response")
		})

		t.Run("callback certificate helpers cover explicit and fallback paths", func(t *testing.T) {
			listenerAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8443}

			fallbackCert, usedFallback, fallbackErr := loadEnableBankingCallbackCertificate(
				listenerAddr,
				enableBankingConnectParams{},
			)
			require.NoError(t, fallbackErr)
			assert.True(t, usedFallback)
			require.NotEmpty(t, fallbackCert.Certificate)

			certFile, keyFile := writeLocalhostTLSCertFiles(t)
			explicitCert, explicitFallback, explicitErr := loadEnableBankingCallbackCertificate(
				listenerAddr,
				enableBankingConnectParams{CallbackCertFile: certFile, CallbackKeyFile: keyFile},
			)
			require.NoError(t, explicitErr)
			assert.False(t, explicitFallback)
			require.NotEmpty(t, explicitCert.Certificate)

			_, _, loadErr := loadEnableBankingCallbackCertificate(
				listenerAddr,
				enableBankingConnectParams{
					CallbackCertFile: certFile,
					CallbackKeyFile:  filepath.Join(t.TempDir(), "missing.pem"),
				},
			)
			require.Error(t, loadErr)
			require.ErrorContains(t, loadErr, "load enable-banking callback TLS certificate")

			generatedCert, generatedErr := newEnableBankingEphemeralCallbackCertificate(listenerAddr)
			require.NoError(t, generatedErr)
			require.NotEmpty(t, generatedCert.Certificate)
			leaf, parseErr := x509.ParseCertificate(generatedCert.Certificate[0])
			require.NoError(t, parseErr)
			assert.Contains(t, leaf.DNSNames, enableBankingLocalhost)
			assert.Equal(t, enableBankingLocalhost, leaf.Subject.CommonName)
		})

		t.Run("query and timeout helpers cover success paths", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "value", r.URL.Query().Get("key"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			request := baseRequest
			request.BaseURL = server.URL
			raw, queryErr := callEnableBankingJSONEndpointWithQuery(
				t.Context(),
				request,
				http.MethodGet,
				"/query",
				url.Values{"key": []string{"value"}},
				nil,
				time.Now,
			)
			require.NoError(t, queryErr)
			assert.Equal(t, true, raw["ok"])

			timeoutCtx, cancel := withFinancePOCTimeout(t.Context(), time.Second)
			defer cancel()
			deadline, ok := timeoutCtx.Deadline()
			require.True(t, ok)
			assert.Positive(t, time.Until(deadline))
		})

		t.Run("callback helper handles missing code and canceled context", func(t *testing.T) {
			listenConfig := net.ListenConfig{}
			listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)

			go func() {
				_, _ = http.Get("http://" + listener.Addr().String() + "/callback?state=expected")
			}()

			_, err = waitForEnableBankingCallback(t.Context(), listener, "expected")
			require.Error(t, err)
			require.ErrorContains(t, err, "callback code is required")

			listener, err = listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			canceledCtx, cancel := context.WithCancel(t.Context())
			cancel()
			_, err = waitForEnableBankingCallback(canceledCtx, listener, "expected")
			require.Error(t, err)
			require.ErrorContains(t, err, "wait for enable-banking callback")
		})

		t.Run("commands surface write open and listen errors", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/auth":
					_, _ = w.Write([]byte(`{"url":"https://bank.example.test/authorize","id":"auth-1"}`))
				case "/sessions":
					_, _ = w.Write([]byte(`{"id":"session-1"}`))
				default:
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()

			blockedParent := filepath.Join(t.TempDir(), "blocked")
			require.NoError(t, os.WriteFile(blockedParent, []byte("x"), 0o600))

			rootCmd, _, _ := makeRootCmd(t, financePOCCommandDeps{})
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.SetArgs([]string{
				"finance-poc", "enable-banking", "start-auth",
				"--country", "PL",
				"--aspsp-name", "PKO",
				"--psu-type", "personal",
				"--valid-days", "30",
				"--redirect-url", "https://example.test/callback",
				"--auth-file", filepath.Join(blockedParent, "pending.json"),
				"--json",
				"--base-url", server.URL,
				"--app-id", appID,
				"--private-key-path", privateKeyPath,
			})
			err := rootCmd.ExecuteContext(t.Context())
			require.Error(t, err)
			require.ErrorContains(t, err, "create finance-poc output directory")

			rootCmd, _, _ = makeRootCmd(t, financePOCCommandDeps{
				EnableBankingOpenBrowser: func(string) error { return assert.AnError },
			})
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.SetArgs([]string{
				"finance-poc", "enable-banking", "start-auth",
				"--country", "PL",
				"--aspsp-name", "PKO",
				"--psu-type", "personal",
				"--valid-days", "30",
				"--redirect-url", "https://example.test/callback",
				"--auth-file", filepath.Join(t.TempDir(), "pending.json"),
				"--base-url", server.URL,
				"--app-id", appID,
				"--private-key-path", privateKeyPath,
				"--open-browser",
			})
			err = rootCmd.ExecuteContext(t.Context())
			require.Error(t, err)
			require.ErrorContains(t, err, "open enable-banking browser")

			pendingAuthFile := filepath.Join(t.TempDir(), "pending-auth.json")
			pendingPayload, marshalErr := json.Marshal(enableBankingPendingAuthFile{
				Provider: enableBankingCommandName,
				Kind:     "pending-auth",
				State:    "state-123",
				Request:  pendingRequest,
				Raw:      map[string]any{"id": "auth-123"},
			})
			require.NoError(t, marshalErr)
			require.NoError(t, os.WriteFile(pendingAuthFile, pendingPayload, 0o600))

			rootCmd, _, _ = makeRootCmd(t, financePOCCommandDeps{})
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.SetArgs([]string{
				"finance-poc", "enable-banking", "finish-session",
				"--auth-file", pendingAuthFile,
				"--code", "code-123",
				"--state", "state-123",
				"--session-file", filepath.Join(blockedParent, "session.json"),
				"--json",
				"--base-url", server.URL,
				"--app-id", appID,
				"--private-key-path", privateKeyPath,
			})
			err = rootCmd.ExecuteContext(t.Context())
			require.Error(t, err)
			require.ErrorContains(t, err, "create finance-poc output directory")

			rootCmd, _, _ = makeRootCmd(t, financePOCCommandDeps{})
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.SetArgs([]string{
				"finance-poc", "enable-banking", "connect",
				"--country", "PL",
				"--aspsp-name", "PKO",
				"--psu-type", "personal",
				"--valid-days", "30",
				"--callback-listen-addr", "bad::addr",
				"--session-file", filepath.Join(t.TempDir(), "session.json"),
				"--base-url", server.URL,
				"--app-id", appID,
				"--private-key-path", privateKeyPath,
			})
			err = rootCmd.ExecuteContext(t.Context())
			require.Error(t, err)
			require.ErrorContains(t, err, "listen for enable-banking callback")

			callbackAddr := reserveListenAddr(t)
			rootCmd, _, _ = makeRootCmd(t, financePOCCommandDeps{
				EnableBankingOpenBrowser: func(string) error { return assert.AnError },
			})
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			rootCmd.SetArgs([]string{
				"finance-poc", "enable-banking", "connect",
				"--country", "PL",
				"--aspsp-name", "PKO",
				"--psu-type", "personal",
				"--valid-days", "30",
				"--callback-listen-addr", callbackAddr,
				"--session-file", filepath.Join(t.TempDir(), "session.json"),
				"--base-url", server.URL,
				"--app-id", appID,
				"--private-key-path", privateKeyPath,
				"--open-browser",
			})
			err = rootCmd.ExecuteContext(t.Context())
			require.Error(t, err)
			require.ErrorContains(t, err, "open enable-banking browser")
		})

		t.Run("auth request body matches Enable Banking schema", func(t *testing.T) {
			now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
			body := buildEnableBankingAuthorizationRequestBody(
				enableBankingPendingAuthRequest{
					Country:     "PL",
					ASPSPName:   "PKO Bank Polski",
					PSUType:     "personal",
					ValidDays:   90,
					RedirectURL: "https://localhost:8085/callback",
				},
				"state-123",
				now,
			)

			aspsp := body["aspsp"].(map[string]any)
			assert.Equal(t, "PL", aspsp["country"])
			assert.Equal(t, "PKO Bank Polski", aspsp["name"])
			access := body["access"].(map[string]any)
			validUntil, err := time.Parse(time.RFC3339, access["valid_until"].(string))
			require.NoError(t, err)
			assert.Equal(t, now.Add(90*24*time.Hour).UTC(), validUntil.UTC())
			assert.Equal(t, "personal", body["psu_type"])
			assert.Equal(t, "https://localhost:8085/callback", body["redirect_url"])
			assert.Equal(t, "state-123", body["state"])
		})

		t.Run("misc helpers cover extraction and browser failure", func(t *testing.T) {
			assert.Equal(t, "value", extractEnableBankingString(map[string]any{"b": "value"}, "a", "b"))
			assert.Equal(
				t,
				"top-level-value",
				extractEnableBankingSessionIdentifier(
					map[string]any{"id": "top-level-value", "session": map[string]any{"id": "nested-value"}},
					"id",
				),
			)
			assert.Equal(
				t,
				"nested-value",
				extractEnableBankingSessionIdentifier(
					map[string]any{"session": map[string]any{"id": "nested-value"}},
					"id",
				),
			)
			assert.Empty(t, extractEnableBankingSessionIdentifier(map[string]any{}, "id"))
			assert.Equal(
				t,
				30,
				extractEnableBankingNestedInt(
					map[string]any{"access": map[string]any{"valid_for_days": float64(30)}},
					"access",
					"valid_for_days",
				),
			)
			assert.Len(
				t,
				extractEnableBankingAccounts(
					map[string]any{"accounts": []any{map[string]any{"uid": "1"}, "skip"}},
				),
				1,
			)
			assert.Equal(t, 7, extractEnableBankingNumber(float64(7)))
			assert.Equal(t, 8, extractEnableBankingNumber(8))
			assert.Equal(t, 0, extractEnableBankingNumber("bad"))

			if runtime.GOOS != "windows" {
				t.Setenv("PATH", "")
				err := openFinancePOCBrowser("https://example.test")
				require.Error(t, err)
				require.ErrorContains(t, err, "start browser command")
			}
		})
	})
}
