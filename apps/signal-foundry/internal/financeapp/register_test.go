package financeapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
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

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/auth"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

type publisherStub struct{}

func (publisherStub) PublishInTx(context.Context, *sql.Tx, appdispatch.Envelope) error { return nil }

func TestNewFinanceStoreFromDI(t *testing.T) {
	database, err := newDatabase(databaseDeps{
		DatabaseDSN: filepath.Join(t.TempDir(), "finance.sqlite"),
	})
	require.NoError(t, err)

	store := persistence.NewStore(database)
	_, err = store.ListTenantsForUser(t.Context(), "user-no-auto-migrate")
	require.Error(t, err)
	require.ErrorContains(t, err, "no such table")
}

//nolint:cyclop,gocyclo // Keeps closely related DI integration scenarios together.
func TestNewFinanceServiceFromDI(t *testing.T) {
	makeJobsService := func(t *testing.T, registry *jobspkg.Registry, dsn string) (*jobspkg.Service, *jobspkg.Store) {
		t.Helper()

		store, err := jobspkg.NewStore(dsn, jobspkg.StoreOpts{TablePrefix: "jobs_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		service, err := jobspkg.NewService(jobspkg.ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   publisherStub{},
			Registry:    registry,
		})
		require.NoError(t, err)

		return service, store
	}

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

	t.Run("registers monobank and pko product choices and keeps sync job-backed", func(t *testing.T) {
		monoToken := "mono-token-test"
		monoClientInfoBody := `{"name":"mono","accounts":[{"id":"mono-acc-1","type":"black","currencyCode":980,"iban":"UA123","balance":101}]}`
		monoTransactionsBody := `[{"id":"mono-txn-1","time":1717203600,"description":"mono txn","currencyCode":980,"amount":-250}]`
		monoServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/personal/client-info":
				if got := request.Header.Get("X-Token"); got != monoToken {
					t.Errorf("monobank link token header = %q, want %q", got, monoToken)
				}
				_, _ = writer.Write([]byte(monoClientInfoBody))
			case "/personal/statement/mono-acc-1/1717200000/1717286400":
				if got := request.Header.Get("X-Token"); got != monoToken {
					t.Errorf("monobank sync token header = %q, want %q", got, monoToken)
				}
				_, _ = writer.Write([]byte(monoTransactionsBody))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer monoServer.Close()

		enablePrivateKeyPath := makePrivateKeyPath(t)
		enableAuthBody := `{"authorizationUrl":"https://bank.example/auth","id":"provider-ref-1"}`
		enableSessionBody := `{"id":"session-1","displayName":"PKO","state":"active"}`
		enableAccountsBody := `{"id":"session-1","accounts":[{"uid":"pko-acc-1","name":"PKO","currency":"PLN","iban":"PL123"}]}`
		enableBalancesBody := `{"balances":[{"balance_amount":{"amount":"12.00","currency":"PLN"},"type":"closingBooked"},{"balance_amount":{"amount":"11.00","currency":"PLN"},"type":"interimAvailable"}]}`
		enableTransactionsBody := `{"transactions":[{"id":"pko-txn-1","status":"booked","amount":{"amount":"5.00","currency":"PLN"},"credit_debit_indicator":"DBIT","remittance_information_unstructured":"pko txn","booking_date":"2026-06-02"}]}`
		enableServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if got := request.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
				t.Errorf("enable banking authorization header = %q", got)
			}
			switch {
			case request.Method == http.MethodPost && request.URL.Path == "/auth":
				_, _ = writer.Write([]byte(enableAuthBody))
			case request.Method == http.MethodPost && request.URL.Path == "/sessions":
				_, _ = writer.Write([]byte(enableSessionBody))
			case request.Method == http.MethodGet && request.URL.Path == "/sessions/session-1":
				_, _ = writer.Write([]byte(enableAccountsBody))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/pko-acc-1/balances":
				_, _ = writer.Write([]byte(enableBalancesBody))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/pko-acc-1/transactions":
				_, _ = writer.Write([]byte(enableTransactionsBody))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer enableServer.Close()

		database, err := persistence.OpenDatabase(filepath.Join(t.TempDir(), "finance.sqlite"))
		require.NoError(t, err)
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))

		financeStore := persistence.NewStore(database)

		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			filepath.Join(t.TempDir(), "jobs.sqlite"),
		)

		financeModule, err := newFinanceModuleFromDI(financeServiceDeps{
			Database:             database,
			Store:                financeStore,
			Jobs:                 jobsService,
			JobsStore:            jobsStore,
			Registry:             registry,
			RootLogger:           nil,
			JWT:                  "jwt-key-for-finance-tests",
			MonoURL:              monoServer.URL,
			EnableURL:            enableServer.URL,
			EnableAppID:          "app-123",
			EnablePrivateKeyPath: enablePrivateKeyPath,
			EnableASPSPName:      "PKO Bank Polski",
			EnableCountry:        "PL",
			EnablePSUType:        "personal",
			EnableValidDays:      90,
		})
		require.NoError(t, err)
		bankConnections := financeModule.BankConnectionService
		bankSyncService := financeModule.BankSyncService
		tenantService := financeModule.TenantService

		tenant, err := tenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-finance",
			DisplayCurrency: "PLN",
		})
		require.NoError(t, err)

		monobankConnection, err := bankConnections.LinkTokenBankConnection(
			t.Context(),
			financepkg.LinkTokenBankConnectionParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "monobank",
				Token:       monoToken,
			},
		)
		require.NoError(t, err)
		require.Equal(t, "monobank", monobankConnection.Provider)

		start, err := bankConnections.StartBankConnectionLink(
			t.Context(),
			financepkg.StartBankConnectionLinkParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "pko",
				RedirectURL: "https://app.example.test/#/finance/connections",
			},
		)
		require.NoError(t, err)

		pkoConnection, err := bankConnections.FinishBankConnectionLink(
			t.Context(),
			financepkg.FinishBankConnectionLinkParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "pko",
				State:       start.State,
				Code:        "code-1",
			},
		)
		require.NoError(t, err)
		require.Equal(t, "pko", pkoConnection.Provider)

		connections := []string{monobankConnection.ID, pkoConnection.ID}
		for _, connectionID := range connections {
			jobRef, triggerErr := bankSyncService.TriggerBankConnectionSync(
				t.Context(),
				financepkg.TriggerBankConnectionSyncParams{
					ActorUserID:  "user-owner",
					TenantID:     tenant.ID,
					ConnectionID: connectionID,
					Reason:       financepkg.BankConnectionSyncReasonManual,
				},
			)
			require.NoError(t, triggerErr)

			job, getErr := jobsStore.Get(t.Context(), jobRef.ID)
			require.NoError(t, getErr)

			var input bankConnectionSyncJobInput
			require.NoError(t, json.Unmarshal(job.InputJSON, &input))
			require.Equal(t, connectionID, input.ConnectionID)
		}

		connectionsView, err := bankSyncService.ListBankConnections(
			t.Context(),
			financepkg.ListBankConnectionsParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
			},
		)
		require.NoError(t, err)
		require.Len(t, connectionsView, 2)
		providers := []string{
			connectionsView[0].Connection.Provider,
			connectionsView[1].Connection.Provider,
		}
		require.ElementsMatch(t, []string{"monobank", "pko"}, providers)

		states := []domain.BankConnectionState{
			connectionsView[0].Connection.State,
			connectionsView[1].Connection.State,
		}
		require.Contains(t, states, domain.BankConnectionStateActive)
	})

	t.Run("enables signed enable banking provider from injected config", func(t *testing.T) {
		privateKeyPath := makePrivateKeyPath(t)
		privateKeyPEM, err := os.ReadFile(privateKeyPath)
		require.NoError(t, err)
		block, _ := pem.Decode(privateKeyPEM)
		require.NotNil(t, block)
		parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		require.NoError(t, err)
		privateKey, ok := parsedKey.(*rsa.PrivateKey)
		require.True(t, ok)

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			authorization := strings.TrimSpace(request.Header.Get("Authorization"))
			if !strings.HasPrefix(authorization, "Bearer ") {
				t.Errorf("authorization header = %q", authorization)
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			tokenString := strings.TrimPrefix(authorization, "Bearer ")
			parser := jwt.NewParser(jwt.WithoutClaimsValidation())
			token, parseErr := parser.Parse(tokenString, func(_ *jwt.Token) (any, error) {
				return &privateKey.PublicKey, nil
			})
			if parseErr != nil {
				t.Errorf("parse jwt: %v", parseErr)
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			assert.True(t, token.Valid)
			assert.Equal(t, "app-123", token.Header["kid"])

			switch {
			case request.Method == http.MethodPost && request.URL.Path == "/auth":
				_, _ = writer.Write([]byte(`{"url":"https://bank.example.test/authorize","id":"auth-123"}`))
			case request.Method == http.MethodPost && request.URL.Path == "/sessions":
				_, _ = writer.Write([]byte(
					`{"id":"session-123","accounts":[{"uid":"acc-1","name":"ROR","currency":"PLN","iban":"PL123"}]}`,
				))
			case request.Method == http.MethodGet && request.URL.Path == "/sessions/session-123":
				_, _ = writer.Write([]byte(
					`{"id":"session-123","state":"active","accounts":[{"uid":"acc-1","name":"ROR","currency":"PLN","iban":"PL123"}]}`,
				))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/acc-1/balances":
				_, _ = writer.Write([]byte(
					`{"balances":[{"type":"closingBooked","balance_amount":{"amount":"100.00","currency":"PLN"}}]}`,
				))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/acc-1/transactions":
				_, _ = writer.Write([]byte(
					`{"transactions":[{"id":"txn-1","status":"booked","booking_date":"2026-06-02","amount":{"amount":"10.00","currency":"PLN"},"credit_debit_indicator":"DBIT","remittance_information_unstructured":"Signed flow txn"}]}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()

		database, err := persistence.OpenDatabase(filepath.Join(t.TempDir(), "finance.sqlite"))
		require.NoError(t, err)
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		financeStore := persistence.NewStore(database)

		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			filepath.Join(t.TempDir(), "jobs.sqlite"),
		)

		financeModule, err := newFinanceModuleFromDI(financeServiceDeps{
			Database:             database,
			Store:                financeStore,
			Jobs:                 jobsService,
			JobsStore:            jobsStore,
			Registry:             registry,
			RootLogger:           nil,
			JWT:                  "jwt-key-for-finance-tests",
			EnableURL:            server.URL,
			EnableAppID:          "app-123",
			EnablePrivateKeyPath: privateKeyPath,
			EnableASPSPName:      "PKO Bank Polski",
			EnableCountry:        "PL",
			EnablePSUType:        "personal",
			EnableValidDays:      90,
		})
		require.NoError(t, err)
		bankConnections := financeModule.BankConnectionService
		bankSyncService := financeModule.BankSyncService
		tenantService := financeModule.TenantService

		tenant, err := tenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-signed-enable-banking",
			DisplayCurrency: "PLN",
		})
		require.NoError(t, err)

		start, err := bankConnections.StartBankConnectionLink(t.Context(), financepkg.StartBankConnectionLinkParams{
			ActorUserID: "user-owner",
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://backend.example.test/enable-banking/callback",
		})
		require.NoError(t, err)

		connection, err := bankConnections.FinishBankConnectionLink(
			t.Context(),
			financepkg.FinishBankConnectionLinkParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "pko",
				State:       start.State,
				Code:        "code-1",
			},
		)
		require.NoError(t, err)
		require.Equal(t, "session-123", connection.ExternalID)

		jobRef, err := bankSyncService.TriggerBankConnectionSync(
			t.Context(),
			financepkg.TriggerBankConnectionSyncParams{
				ActorUserID:  "user-owner",
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
				Reason:       financepkg.BankConnectionSyncReasonManual,
			},
		)
		require.NoError(t, err)

		job, err := jobsStore.Get(t.Context(), jobRef.ID)
		require.NoError(t, err)
		var input bankConnectionSyncJobInput
		require.NoError(t, json.Unmarshal(job.InputJSON, &input))
		require.Equal(t, connection.ID, input.ConnectionID)
	})

	t.Run("uses auth signing key fallback for monobank token linking", func(t *testing.T) {
		monoToken := "mono-token-fallback"
		monoServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/personal/client-info" {
				t.Errorf("monobank link path = %q, want %q", request.URL.Path, "/personal/client-info")
			}
			if got := request.Header.Get("X-Token"); got != monoToken {
				t.Errorf("monobank fallback token header = %q, want %q", got, monoToken)
			}
			_, _ = writer.Write([]byte(
				`{"name":"mono","accounts":[{"id":"mono-acc-fallback","type":"black","currencyCode":980,"iban":"UA456","balance":101}]}`,
			))
		}))
		defer monoServer.Close()
		enablePrivateKeyPath := makePrivateKeyPath(t)

		database, err := persistence.OpenDatabase(filepath.Join(t.TempDir(), "finance.sqlite"))
		require.NoError(t, err)
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		financeStore := persistence.NewStore(database)

		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			filepath.Join(t.TempDir(), "jobs.sqlite"),
		)

		dataDir := t.TempDir()
		container := dig.New()
		require.NoError(
			t,
			di.ProvideAll(
				container,
				di.ProvideValue("", dig.Name("config.auth.jwtSigningKey")),
				di.ProvideValue(24*time.Hour, dig.Name("config.auth.accessTokenTTL")),
				di.ProvideValue(7*24*time.Hour, dig.Name("config.auth.refreshTokenTTL")),
				di.ProvideValue(dataDir, dig.Name("config.dataDir")),
				slog.Default,
				func() *persistence.Database { return database },
				func() *persistence.Store { return financeStore },
				func() *jobspkg.Service { return jobsService },
				func() *jobspkg.Store { return jobsStore },
				func() *jobspkg.Registry { return registry },
				di.ProvideValue(monoServer.URL, dig.Name("config.finance.providers.monobank.baseURL")),
				di.ProvideValue(time.Duration(0), dig.Name("config.finance.providers.monobank.sleepBetweenRequests")),
				di.ProvideValue(
					"https://api.enablebanking.com",
					dig.Name("config.finance.providers.enableBanking.baseURL"),
				),
				di.ProvideValue("app-auth-fallback", dig.Name("config.finance.providers.enableBanking.appID")),
				di.ProvideValue(
					enablePrivateKeyPath,
					dig.Name("config.finance.providers.enableBanking.privateKeyPath"),
				),
				di.ProvideValue("Mock ASPSP", dig.Name("config.finance.providers.enableBanking.aspspName")),
				di.ProvideValue("PL", dig.Name("config.finance.providers.enableBanking.country")),
				di.ProvideValue("personal", dig.Name("config.finance.providers.enableBanking.psuType")),
				di.ProvideValue(90, dig.Name("config.finance.providers.enableBanking.validDays")),
			),
		)
		require.NoError(t, auth.Register(container))
		require.NoError(t, container.Provide(newFinanceModuleFromDI))
		require.NoError(t, container.Provide(newTenantServiceFromDI))
		require.NoError(t, container.Provide(newBankSyncServiceFromDI))
		require.NoError(t, container.Provide(newBankConnectionServiceFromDI))
		require.NoError(t, container.Provide(newSyntheticLinkStateServiceFromDI))

		type resolvedDeps struct {
			dig.In

			JWTKey                    string `name:"auth.jwtKey"`
			TenantService             *financepkg.TenantService
			BankSyncService           *financepkg.BankSyncService
			BankConnectionService     *financepkg.BankConnectionService
			SyntheticLinkStateService *financepkg.SyntheticLinkStateService
		}

		var resolved resolvedDeps
		require.NoError(t, container.Invoke(func(deps resolvedDeps) {
			resolved = deps
		}))
		require.NotEmpty(t, resolved.JWTKey)
		require.NotNil(t, resolved.BankConnectionService)
		require.NotNil(t, resolved.SyntheticLinkStateService)

		persistedKeyPath := filepath.Join(dataDir, "auth", "jwt-signing-key")
		persistedKey, err := os.ReadFile(persistedKeyPath)
		require.NoError(t, err)
		require.Equal(t, resolved.JWTKey, string(persistedKey))

		tenant, err := resolved.TenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-fallback",
			DisplayCurrency: "UAH",
		})
		require.NoError(t, err)

		connection, err := resolved.BankConnectionService.LinkTokenBankConnection(
			t.Context(),
			financepkg.LinkTokenBankConnectionParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "monobank",
				Token:       monoToken,
			},
		)
		require.NoError(t, err)
		require.Equal(t, "monobank", connection.Provider)

		connections, err := resolved.BankSyncService.ListBankConnections(
			t.Context(),
			financepkg.ListBankConnectionsParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
			},
		)
		require.NoError(t, err)
		require.Len(t, connections, 1)
		require.Equal(t, "monobank", connections[0].Connection.Provider)
	})

	t.Run("rejects omitted enable banking credentials", func(t *testing.T) {
		database, err := persistence.OpenDatabase(filepath.Join(t.TempDir(), "finance.sqlite"))
		require.NoError(t, err)
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		financeStore := persistence.NewStore(database)

		_, err = newFinanceModuleFromDI(financeServiceDeps{
			Database:        database,
			Store:           financeStore,
			JWT:             "jwt-key-for-finance-tests",
			EnableURL:       "https://api.enablebanking.com",
			EnableASPSPName: "Mock ASPSP",
			EnableCountry:   "PL",
			EnablePSUType:   "personal",
			EnableValidDays: 90,
		})
		require.ErrorContains(t, err, "validate enable banking config: app ID is required")
	})
}
