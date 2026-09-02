//go:build postgres_test

package v1controllers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	financepersistence "github.com/gemyago/sumweave/finance/persistence"
	"github.com/gemyago/sumweave/runtime/httpapi"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinanceSyntheticLinkStateControllerPostgres(t *testing.T) {
	fake := faker.New()
	makePrivateKeyPath := func(t *testing.T) string {
		t.Helper()
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		privateKeyPath := filepath.Join(t.TempDir(), "enable-banking-private-key.pem")
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
		require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))
		return privateKeyPath
	}
	makeAuthMiddleware := func(userID string) middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				ctx := httpapi.ContextWithCallerIdentity(r.Context(), &testCallerIdentity{userID: userID})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	}
	newRequest := func(method, target, body string, authenticated bool) *http.Request {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req = req.WithContext(t.Context())
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+fake.UUID().V4())
		}
		return req
	}
	decode := func(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var payload map[string]any
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
		return payload
	}
	newActualHandler := func(
		t *testing.T,
		module *financepkg.Finance,
		auth middleware.AuthMiddleware,
	) http.Handler {
		t.Helper()
		controller := NewFinanceController(FinanceControllerDeps{
			BankConnectionService:     module.BankConnectionService,
			SyntheticLinkStateService: module.SyntheticLinkStateService,
			AuthMiddleware:            auth,
		})
		return server.NewTestRootHandler().RegisterFinanceRoutes(controller)
	}

	dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
	require.NotEmpty(t, dsn)
	sqlDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	database, err := financepersistence.NewDatabase(sqlDB, dsn)
	require.NoError(t, err)
	cipherKey := sha256.Sum256([]byte(fake.UUID().V4()))
	connectionCipher, err := credentials.NewAESGCMCipher(cipherKey[:], "test-"+fake.UUID().V4())
	require.NoError(t, err)
	module, err := financepkg.New(&financepkg.Config{
		Database:               database,
		Logger:                 slog.New(slog.DiscardHandler),
		Now:                    time.Now,
		NewID:                  func() string { return fake.UUID().V4() },
		HTTPClient:             http.DefaultClient,
		ConnectionSecretCipher: connectionCipher,
		Monobank:               financepkg.MonobankConfig{BaseURL: "https://api.monobank.ua"},
		EnableBanking: financepkg.EnableBankingConfig{
			BaseURL:        "https://enable-banking.example.test",
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: makePrivateKeyPath(t),
			ASPSPs: []financepkg.EnableBankingASPSP{{
				ProviderID: domain.ProviderIDPKO,
				Name:       "Mock ASPSP",
				Country:    "PL",
				PSUType:    "personal",
				ValidDays:  90,
			}},
		},
	})
	require.NoError(t, err)

	userID := fake.UUID().V4()
	otherUserID := fake.UUID().V4()
	tenant, err := module.TenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
		ActorUserID:     userID,
		Name:            "tenant-" + fake.UUID().V4(),
		DisplayCurrency: "USD",
		SeedDefaults:    true,
	})
	require.NoError(t, err)
	handler := newActualHandler(t, module, makeAuthMiddleware(userID))
	otherHandler := newActualHandler(t, module, makeAuthMiddleware(otherUserID))
	configuredAccountsJSON := func(items []map[string]string) string {
		payload, marshalErr := json.Marshal(map[string]any{"configuredAccounts": items})
		require.NoError(t, marshalErr)
		return string(payload)
	}

	startResponse := httptest.NewRecorder()
	handler.ServeHTTP(startResponse, newRequest(
		http.MethodPost,
		"/api/v1/finance/tenants/"+tenant.ID+"/connections/link-redirect/start",
		`{"provider":"synthetic","callbackUrl":"https://app.example.test/#/finance/connections"}`,
		true,
	))
	require.Equal(t, http.StatusOK, startResponse.Code)
	startPayload := decode(t, startResponse)
	state := startPayload["state"].(string)
	assert.Equal(t, "synthetic", startPayload["provider"])
	assert.Contains(t, startPayload["authorizationUrl"], "#/finance/connections/synthetic?state=")
	statePath := "/api/v1/finance/tenants/" + tenant.ID + "/connections/synthetic-link-states/state/" + state

	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, newRequest(http.MethodGet, statePath, "", false))
	require.Equal(t, http.StatusUnauthorized, unauthorizedResponse.Code)
	initialResponse := httptest.NewRecorder()
	handler.ServeHTTP(initialResponse, newRequest(http.MethodGet, statePath, "", true))
	require.Equal(t, http.StatusOK, initialResponse.Code)
	initialPayload := decode(t, initialResponse)
	assert.Equal(t, "synthetic", initialPayload["provider"])
	assert.Equal(t, state, initialPayload["state"])
	assert.Empty(t, initialPayload["configuredAccounts"])
	assert.False(t, initialPayload["canFinish"].(bool))

	isolationResponse := httptest.NewRecorder()
	otherHandler.ServeHTTP(isolationResponse, newRequest(http.MethodGet, statePath, "", true))
	require.Equal(t, http.StatusUnauthorized, isolationResponse.Code)

	putResponse := httptest.NewRecorder()
	handler.ServeHTTP(putResponse, newRequest(
		http.MethodPut,
		statePath,
		configuredAccountsJSON([]map[string]string{
			{"name": "checking-" + fake.UUID().V4(), "currency": "USD"},
			{"name": "checking-" + fake.UUID().V4(), "currency": "USD"},
		}),
		true,
	))
	require.Equal(t, http.StatusOK, putResponse.Code)
	configuredAccounts := decode(t, putResponse)["configuredAccounts"].([]any)
	require.Len(t, configuredAccounts, 2)
	firstKey := configuredAccounts[0].(map[string]any)["key"].(string)
	secondKey := configuredAccounts[1].(map[string]any)["key"].(string)
	assert.NotEmpty(t, firstKey)
	assert.NotEmpty(t, secondKey)
	assert.NotEqual(t, firstKey, secondKey)

	getAgainResponse := httptest.NewRecorder()
	handler.ServeHTTP(getAgainResponse, newRequest(http.MethodGet, statePath, "", true))
	require.Equal(t, http.StatusOK, getAgainResponse.Code)
	refreshedAccounts := decode(t, getAgainResponse)["configuredAccounts"].([]any)
	require.Len(t, refreshedAccounts, 2)
	assert.Equal(t, firstKey, refreshedAccounts[0].(map[string]any)["key"])
	assert.Equal(t, secondKey, refreshedAccounts[1].(map[string]any)["key"])

	reSaveResponse := httptest.NewRecorder()
	handler.ServeHTTP(reSaveResponse, newRequest(
		http.MethodPut,
		statePath,
		configuredAccountsJSON([]map[string]string{
			{"key": firstKey, "name": "checking-" + fake.UUID().V4(), "currency": "USD"},
			{"key": secondKey, "name": "checking-" + fake.UUID().V4(), "currency": "USD"},
		}),
		true,
	))
	require.Equal(t, http.StatusOK, reSaveResponse.Code)
	reSavedAccounts := decode(t, reSaveResponse)["configuredAccounts"].([]any)
	assert.Equal(t, firstKey, reSavedAccounts[0].(map[string]any)["key"])
	assert.Equal(t, secondKey, reSavedAccounts[1].(map[string]any)["key"])

	finishResponse := httptest.NewRecorder()
	handler.ServeHTTP(finishResponse, newRequest(
		http.MethodPost,
		"/api/v1/finance/tenants/"+tenant.ID+"/connections/link-redirect/finish",
		`{"provider":"synthetic","state":"`+state+`"}`,
		true,
	))
	require.Equal(t, http.StatusOK, finishResponse.Code)
	finishPayload := decode(t, finishResponse)
	assert.Equal(t, "synthetic", finishPayload["provider"])
	assert.Equal(t, state, finishPayload["providerReference"])
}
