package http_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	signalfoundryhttp "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1controllers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/credentials"
	financedomain "github.com/gemyago/signal-foundry/finance/domain"
	financepersistence "github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testReplayReadService struct{}

func (s *testReplayReadService) ListCandleAvailability(
	_ context.Context,
	_ data.CandleAvailabilityListQuery,
) (data.CandleAvailabilityListResult, error) {
	return data.CandleAvailabilityListResult{Items: []data.CandleAvailabilityItem{}}, nil
}

func (s *testReplayReadService) ReplayCandles(
	_ context.Context,
	_ domain.Instrument,
	_ domain.Timeframe,
	_ domain.TimeRange,
) ([]data.ReplayCandle, error) {
	return []data.ReplayCandle{}, nil
}

type testLineageBrowserService struct{}

type testEvaluationWorkspaceService struct{}

type registerTestCallerIdentity struct{ userID string }

func (c *registerTestCallerIdentity) UserID() string { return c.userID }

func (s *testLineageBrowserService) ListRawPayloadMetadata(
	_ context.Context,
	_ data.RawPayloadMetadataListQuery,
) (data.RawPayloadMetadataListResult, error) {
	return data.RawPayloadMetadataListResult{}, nil
}

func (s *testLineageBrowserService) GetRawPayloadDetail(
	_ context.Context,
	_ string,
) (data.RawPayloadDetail, error) {
	return data.RawPayloadDetail{}, nil
}

func (s *testLineageBrowserService) ListCandleLinkedRawPayloadMetadata(
	_ context.Context,
	_ data.CandleLinkedRawPayloadsQuery,
) ([]data.RawPayloadMetadata, error) {
	return []data.RawPayloadMetadata{}, nil
}

func (s *testEvaluationWorkspaceService) CreateEvaluation(
	context.Context,
	app.CreateEvaluationParams,
) (*app.EvaluationDetail, error) {
	return &app.EvaluationDetail{}, nil
}

func (s *testEvaluationWorkspaceService) ListEvaluations(
	context.Context,
	app.ListEvaluationsParams,
) ([]app.EvaluationListItem, error) {
	return []app.EvaluationListItem{}, nil
}

func (s *testEvaluationWorkspaceService) GetEvaluation(
	context.Context,
	string,
) (*app.EvaluationDetail, error) {
	return &app.EvaluationDetail{}, nil
}

func (s *testEvaluationWorkspaceService) GetEvaluationReport(
	context.Context,
	string,
) (*app.EvaluationReportView, error) {
	return &app.EvaluationReportView{}, nil
}

func (s *testEvaluationWorkspaceService) GetEvaluationEvidence(
	context.Context,
	string,
) (*app.EvaluationEvidenceView, error) {
	return &app.EvaluationEvidenceView{}, nil
}

func TestSetupV1Routes(t *testing.T) {
	fake := faker.New()
	makePassthroughMiddleware := func(userID string) middleware.AuthMiddleware {
		return middleware.AuthMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resolvedUserID := userID
				if resolvedUserID == "" {
					resolvedUserID = fake.UUID().V4()
				}
				ctx := httpapi.ContextWithCallerIdentity(
					r.Context(),
					&registerTestCallerIdentity{userID: resolvedUserID},
				)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
	}

	makeSetup := func(
		t *testing.T,
		uiLocation string,
		authUserID string,
	) (*server.HTTPRouter, *v1controllers.HealthController, http.Handler, *financepkg.Finance, *financepkg.BankConnectionService, *financepersistence.Store) {
		t.Helper()
		passthroughMiddleware := makePassthroughMiddleware(authUserID)
		strategyDSN := filepath.Join(t.TempDir(), "strategy-workspace.db")
		strategySQLDB, err := sqlconn.Open(strategyDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, strategySQLDB.Close()) })
		artifactStore, err := rtstrategy.NewArtifactDatabaseStore(
			strategySQLDB,
			strategyDSN,
			rtstrategy.ArtifactDatabaseStoreOpts{TablePrefix: "http_test_"},
		)
		require.NoError(t, err)
		require.NoError(t, artifactStore.AutoMigrate())
		registry, err := rtstrategy.NewVersionRegistryService(
			strategySQLDB,
			strategyDSN,
			rtstrategy.VersionRegistryServiceDeps{
				ArtifactStore: artifactStore,
				TablePrefix:   "http_test_",
			},
		)
		require.NoError(t, err)
		require.NoError(t, registry.AutoMigrate())
		strategyService, err := app.NewStrategyWorkspaceService(
			app.StrategyWorkspaceServiceDeps{
				ArtifactStore:   artifactStore,
				VersionRegistry: registry,
			},
		)
		require.NoError(t, err)
		jobsDSN := filepath.Join(t.TempDir(), "jobs.db")
		jobsSQLDB, err := sqlconn.Open(jobsDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, jobsSQLDB.Close()) })
		jobsStore, err := jobspkg.NewStore(jobsSQLDB, jobsDSN, jobspkg.StoreOpts{})
		require.NoError(t, err)
		require.NoError(t, jobsStore.AutoMigrate())
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{DatabaseDSN: jobsDSN}, jobsSQLDB))
		jobsPublisher, err := appdispatch.NewPublisher(
			appdispatch.Config{DatabaseDSN: jobsDSN},
			jobsSQLDB,
			telemetry.RootTestLogger(),
		)
		require.NoError(t, err)
		jobsService, err := jobspkg.NewService(jobspkg.ServiceDeps{
			Store:       jobsStore,
			Publisher:   jobsPublisher,
			IDGenerator: ident.NewDefaultGenerator(),
		})
		require.NoError(t, err)
		financeDSN := filepath.Join(t.TempDir(), "finance.db")
		financeSQLDB, err := sqlconn.Open(financeDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, financeSQLDB.Close()) })
		financeDatabase, err := financepersistence.NewDatabase(financeSQLDB, financeDSN)
		require.NoError(t, err)
		require.NoError(t, financepersistence.NewMigrator(financeDatabase).Migrate(t.Context()))
		financeStore := financepersistence.NewStore(financeDatabase)
		cipherKey := sha256.Sum256([]byte("http-register-test-cipher"))
		connectionCipher, err := credentials.NewAESGCMCipher(cipherKey[:], "signal-foundry-finance")
		require.NoError(t, err)
		financeModule, err := financepkg.New(&financepkg.Config{
			Database:               financeDatabase,
			Logger:                 telemetry.RootTestLogger(),
			Now:                    time.Now,
			NewID:                  uuid.NewString,
			HTTPClient:             http.DefaultClient,
			ConnectionSecretCipher: connectionCipher,
			Monobank: financepkg.MonobankConfig{
				BaseURL: "https://api.monobank.ua",
			},
			EnableBanking: financepkg.EnableBankingConfig{
				BaseURL:        "https://api.enablebanking.com",
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: "enable-banking-private-key-" + fake.UUID().V4() + ".pem",
				ASPSPs: []financepkg.EnableBankingASPSP{{
					ProviderID: financedomain.ProviderIDPKO,
					Name:       "Mock ASPSP",
					Country:    "PL",
					PSUType:    "personal",
					ValidDays:  90,
				}},
			},
		})
		require.NoError(t, err)
		bankConnectionService := financeModule.BankConnectionService

		router := server.NewHTTPRouter(server.HTTPRouterDeps{
			Middleware: func(h http.Handler) http.Handler { return h },
		})
		rootHandler := server.NewRootHandler(server.RootHandlerDeps{
			RootLogger: telemetry.RootTestLogger(),
			Router:     router,
		})
		rt := &internal.Runtime{
			HTTPHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("runtime"))
			}),
		}
		authCtrl := v1controllers.NewAuthController(v1controllers.AuthControllerDeps{
			AuthService:    nil,
			AuthMiddleware: passthroughMiddleware,
		})
		dataCtrl := v1controllers.NewDataController(v1controllers.DataControllerDeps{
			ReadService:    &testReplayReadService{},
			LineageService: &testLineageBrowserService{},
			AuthMiddleware: passthroughMiddleware,
		})
		jobsCtrl := v1controllers.NewJobsController(v1controllers.JobsControllerDeps{
			JobsService:    jobsService,
			AuthMiddleware: passthroughMiddleware,
		})
		financeCtrl := v1controllers.NewFinanceController(v1controllers.FinanceControllerDeps{
			TenantService:             financeModule.TenantService,
			CatalogService:            financeModule.CatalogService,
			LedgerService:             financeModule.LedgerService,
			BankSyncService:           financeModule.BankSyncService,
			ReportingService:          financeModule.ReportingService,
			FXService:                 financeModule.FXService,
			CSVImportService:          financeModule.CSVImportService,
			BankConnectionService:     bankConnectionService,
			SyntheticLinkStateService: financeModule.SyntheticLinkStateService,
			AuthMiddleware:            passthroughMiddleware,
		})
		strategiesCtrl := v1controllers.NewStrategiesController(
			v1controllers.StrategiesControllerDeps{
				StrategyWorkspaceService: strategyService,
				AuthMiddleware:           passthroughMiddleware,
			},
		)
		evaluationsCtrl := v1controllers.NewEvaluationsController(
			v1controllers.EvaluationsControllerDeps{
				EvaluationWorkspaceService: &testEvaluationWorkspaceService{},
				AuthMiddleware:             passthroughMiddleware,
			},
		)
		healthCtrl := &v1controllers.HealthController{}
		signalfoundryhttp.SetupV1Routes(signalfoundryhttp.V1RoutesDeps{
			HealthController:      healthCtrl,
			AuthController:        authCtrl,
			DataController:        dataCtrl,
			JobsController:        jobsCtrl,
			FinanceController:     financeCtrl,
			StrategiesController:  strategiesCtrl,
			EvaluationsController: evaluationsCtrl,
			AuthMiddleware:        passthroughMiddleware,
			RootHandler:           rootHandler,
			HTTPRouter:            router,
			Runtime:               rt,
			RootLogger:            telemetry.RootTestLogger(),
			BankConnectionService: bankConnectionService,
			UILocation:            uiLocation,
		})
		return router, healthCtrl, rootHandler, financeModule, bankConnectionService, financeStore
	}

	t.Run("should mount agent API routes", func(t *testing.T) {
		calls := []string{}
		passthroughMiddleware := makePassthroughMiddleware("")
		router := server.NewHTTPRouter(server.HTTPRouterDeps{
			Middleware: func(h http.Handler) http.Handler {
				calls = append(calls, "middleware")
				return h
			},
		})
		rootHandler := server.NewRootHandler(server.RootHandlerDeps{
			RootLogger: telemetry.RootTestLogger(),
			Router:     router,
		})
		wantResult := fake.Lorem().Word()
		rt := &internal.Runtime{
			HTTPHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls = append(calls, "handler")
				_, _ = w.Write([]byte(wantResult))
			}),
		}

		authCtrl := v1controllers.NewAuthController(v1controllers.AuthControllerDeps{
			AuthService:    nil,
			AuthMiddleware: passthroughMiddleware,
		})
		dataCtrl := v1controllers.NewDataController(v1controllers.DataControllerDeps{
			ReadService:    &testReplayReadService{},
			LineageService: &testLineageBrowserService{},
			AuthMiddleware: passthroughMiddleware,
		})
		strategyDSN := filepath.Join(t.TempDir(), "strategy-workspace.db")
		strategySQLDB, err := sqlconn.Open(strategyDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, strategySQLDB.Close()) })
		artifactStore, err := rtstrategy.NewArtifactDatabaseStore(
			strategySQLDB,
			strategyDSN,
			rtstrategy.ArtifactDatabaseStoreOpts{TablePrefix: "http_test_"},
		)
		require.NoError(t, err)
		require.NoError(t, artifactStore.AutoMigrate())
		registry, err := rtstrategy.NewVersionRegistryService(
			strategySQLDB,
			strategyDSN,
			rtstrategy.VersionRegistryServiceDeps{
				ArtifactStore: artifactStore,
				TablePrefix:   "http_test_",
			},
		)
		require.NoError(t, err)
		require.NoError(t, registry.AutoMigrate())
		strategyService, err := app.NewStrategyWorkspaceService(
			app.StrategyWorkspaceServiceDeps{
				ArtifactStore:   artifactStore,
				VersionRegistry: registry,
			},
		)
		require.NoError(t, err)
		jobsDSN := filepath.Join(t.TempDir(), "jobs.db")
		jobsSQLDB, err := sqlconn.Open(jobsDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, jobsSQLDB.Close()) })
		jobsStore, err := jobspkg.NewStore(jobsSQLDB, jobsDSN, jobspkg.StoreOpts{})
		require.NoError(t, err)
		require.NoError(t, jobsStore.AutoMigrate())
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{DatabaseDSN: jobsDSN}, jobsSQLDB))
		jobsPublisher, err := appdispatch.NewPublisher(
			appdispatch.Config{DatabaseDSN: jobsDSN},
			jobsSQLDB,
			telemetry.RootTestLogger(),
		)
		require.NoError(t, err)
		jobsService, err := jobspkg.NewService(jobspkg.ServiceDeps{
			Store:       jobsStore,
			Publisher:   jobsPublisher,
			IDGenerator: ident.NewDefaultGenerator(),
		})
		require.NoError(t, err)
		financeDSN := filepath.Join(t.TempDir(), "finance.db")
		financeSQLDB, err := sqlconn.Open(financeDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, financeSQLDB.Close()) })
		financeDatabase, err := financepersistence.NewDatabase(financeSQLDB, financeDSN)
		require.NoError(t, err)
		require.NoError(t, financepersistence.NewMigrator(financeDatabase).Migrate(t.Context()))
		_ = financepersistence.NewStore(financeDatabase)
		cipherKey := sha256.Sum256([]byte("http-register-test-cipher-routes"))
		connectionCipher, err := credentials.NewAESGCMCipher(cipherKey[:], "signal-foundry-finance")
		require.NoError(t, err)
		financeModule, err := financepkg.New(&financepkg.Config{
			Database:               financeDatabase,
			Logger:                 telemetry.RootTestLogger(),
			Now:                    time.Now,
			NewID:                  uuid.NewString,
			HTTPClient:             http.DefaultClient,
			ConnectionSecretCipher: connectionCipher,
			Monobank: financepkg.MonobankConfig{
				BaseURL: "https://api.monobank.ua",
			},
			EnableBanking: financepkg.EnableBankingConfig{
				BaseURL:        "https://api.enablebanking.com",
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: "enable-banking-private-key-" + fake.UUID().V4() + ".pem",
				ASPSPs: []financepkg.EnableBankingASPSP{{
					ProviderID: financedomain.ProviderIDPKO,
					Name:       "Mock ASPSP",
					Country:    "PL",
					PSUType:    "personal",
					ValidDays:  90,
				}},
			},
		})
		require.NoError(t, err)
		bankConnectionService := financeModule.BankConnectionService
		strategiesCtrl := v1controllers.NewStrategiesController(
			v1controllers.StrategiesControllerDeps{
				StrategyWorkspaceService: strategyService,
				AuthMiddleware:           passthroughMiddleware,
			},
		)
		evaluationsCtrl := v1controllers.NewEvaluationsController(
			v1controllers.EvaluationsControllerDeps{
				EvaluationWorkspaceService: &testEvaluationWorkspaceService{},
				AuthMiddleware:             passthroughMiddleware,
			},
		)
		jobsCtrl := v1controllers.NewJobsController(v1controllers.JobsControllerDeps{
			JobsService:    jobsService,
			AuthMiddleware: passthroughMiddleware,
		})
		financeCtrl := v1controllers.NewFinanceController(v1controllers.FinanceControllerDeps{
			TenantService:             financeModule.TenantService,
			CatalogService:            financeModule.CatalogService,
			LedgerService:             financeModule.LedgerService,
			BankSyncService:           financeModule.BankSyncService,
			ReportingService:          financeModule.ReportingService,
			FXService:                 financeModule.FXService,
			CSVImportService:          financeModule.CSVImportService,
			BankConnectionService:     bankConnectionService,
			SyntheticLinkStateService: financeModule.SyntheticLinkStateService,
			AuthMiddleware:            passthroughMiddleware,
		})

		signalfoundryhttp.SetupV1Routes(signalfoundryhttp.V1RoutesDeps{
			HealthController:      &v1controllers.HealthController{},
			AuthController:        authCtrl,
			DataController:        dataCtrl,
			JobsController:        jobsCtrl,
			FinanceController:     financeCtrl,
			StrategiesController:  strategiesCtrl,
			EvaluationsController: evaluationsCtrl,
			AuthMiddleware:        passthroughMiddleware,
			RootHandler:           rootHandler,
			HTTPRouter:            router,
			Runtime:               rt,
			RootLogger:            telemetry.RootTestLogger(),
			BankConnectionService: bankConnectionService,
		})

		t.Run(
			"routes under /api/v1/runtime/ are handled by runtime HTTP handler",
			func(t *testing.T) {
				subpath := fmt.Sprintf("%s/subpath", fake.Lorem().Word())
				req := httptest.NewRequest(
					http.MethodPost,
					fmt.Sprintf("/api/v1/runtime/%s", subpath),
					http.NoBody,
				)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, wantResult, w.Body.String())
			},
		)

		t.Run("data routes are registered on the app router", func(t *testing.T) {
			for _, target := range []string{
				"/api/v1/data/candle-availability",
				"/api/v1/data/candles?venue=hyperliquid-perps&symbol=BTCUSD&assetClass=crypto&timeframe=1m&start=2026-06-15T12:00:00Z&end=2026-06-15T13:00:00Z",
				"/api/v1/jobs",
				"/api/v1/finance/tenants",
				"/api/v1/finance/fx/diagnostics",
			} {
				req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				require.Equal(t, http.StatusOK, w.Code)
			}
		})

		t.Run("generated finance POST routes are registered on the app router", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/finance/invites/accept", http.NoBody)
			w := httptest.NewRecorder()
			rootHandler.ServeHTTP(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("generated finance DELETE routes are registered on the app router", func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodDelete,
				"/api/v1/finance/tenants/tenant-a/connections/connection-a",
				http.NoBody,
			)
			w := httptest.NewRecorder()
			rootHandler.ServeHTTP(w, req)
			require.NotEqual(t, http.StatusNotFound, w.Code)
		})
	})

	t.Run("synthetic link-state routes are registered on the app router", func(t *testing.T) {
		userID := fake.UUID().V4()
		_, _, rootHandler, financeModule, _, _ := makeSetup(t, "", userID)
		tenant, err := financeModule.TenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     userID,
			Name:            "tenant-" + fake.UUID().V4(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)

		startReq := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/finance/tenants/"+tenant.ID+"/connections/link-redirect/start",
			strings.NewReader(
				`{"provider":"synthetic","callbackUrl":"https://app.example.test/#/finance/connections"}`,
			),
		)
		startReq.Header.Set("Content-Type", "application/json")
		startResp := httptest.NewRecorder()
		rootHandler.ServeHTTP(startResp, startReq)
		require.Equal(t, http.StatusOK, startResp.Code)

		var startPayload struct {
			State string `json:"state"`
		}
		require.NoError(t, json.Unmarshal(startResp.Body.Bytes(), &startPayload))
		require.NotEmpty(t, startPayload.State)

		for _, req := range []*http.Request{
			httptest.NewRequest(
				http.MethodGet,
				"/api/v1/finance/tenants/"+tenant.ID+"/connections/synthetic-link-states/"+startPayload.State,
				http.NoBody,
			),
			httptest.NewRequest(
				http.MethodPut,
				"/api/v1/finance/tenants/"+tenant.ID+"/connections/synthetic-link-states/"+startPayload.State,
				strings.NewReader(`{"configuredAccounts":[{"name":"Checking","currency":"USD"}]}`),
			),
		} {
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			rootHandler.ServeHTTP(resp, req)
			require.NotEqual(t, http.StatusNotFound, resp.Code)
		}
	})

	t.Run("enable banking callback route redirects back to finance connections", func(t *testing.T) {
		_, _, rootHandler, _, bankConnectionService, financeStore := makeSetup(t, "", "")

		t.Run("redirects provider return params back to the browser route", func(t *testing.T) {
			pendingStart, err := financeStore.SavePendingBankConnectionLinkStart(
				t.Context(),
				financedomain.PendingBankConnectionLinkStart{
					ID:          "pending-1",
					TenantID:    "tenant-1",
					ActorUserID: "user-owner",
					Provider:    "pko",
					State:       "state-1",
					CallbackURL: "http://localhost:5173/#/finance/connections",
				},
			)
			require.NoError(t, err)
			resolvedStart, err := bankConnectionService.GetPendingBankConnectionLinkStartByState(
				t.Context(),
				financepkg.GetPendingBankConnectionLinkStartByStateParams{
					Provider: "pko",
					State:    pendingStart.State,
				},
			)
			require.NoError(t, err)
			require.Equal(t, pendingStart.CallbackURL, resolvedStart.CallbackURL)
			req := httptest.NewRequest(
				http.MethodGet,
				"/enable-banking/callback?code=code-1&state="+url.QueryEscape(pendingStart.State),
				http.NoBody,
			)
			w := httptest.NewRecorder()
			rootHandler.ServeHTTP(w, req)
			require.Equal(t, http.StatusFound, w.Code)
			assert.Equal(
				t,
				"http://localhost:5173/?code=code-1&state="+url.QueryEscape(pendingStart.State)+"#/finance/connections",
				w.Header().Get("Location"),
			)
		})

		t.Run("rejects invalid callback targets", func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"/enable-banking/callback?code=code-1&state=missing-state",
				http.NoBody,
			)
			w := httptest.NewRecorder()
			rootHandler.ServeHTTP(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code)
		})
	})

	t.Run("UI serving", func(t *testing.T) {
		t.Run("when ui location is empty, server operates in API-only mode", func(t *testing.T) {
			_, _, rootHandler, _, _, _ := makeSetup(t, "", "")

			t.Run("GET / returns 404", func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusNotFound, w.Code)
			})

			t.Run("GET /some-asset.js returns 404", func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/some-asset.js", http.NoBody)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusNotFound, w.Code)
			})
		})

		t.Run("when ui location is a valid directory, server serves UI", func(t *testing.T) {
			uiDir := t.TempDir()
			wantIndexContent := fake.Lorem().Sentence(5)
			wantAssetContent := fake.Lorem().Sentence(3)
			assetName := fake.Lorem().Word() + ".js"
			require.NoError(
				t,
				os.WriteFile(filepath.Join(uiDir, "index.html"), []byte(wantIndexContent), 0o600),
			)
			require.NoError(
				t,
				os.WriteFile(filepath.Join(uiDir, assetName), []byte(wantAssetContent), 0o600),
			)

			_, _, rootHandler, _, _, _ := makeSetup(t, uiDir, "")

			t.Run("GET / serves index.html", func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, wantIndexContent, w.Body.String())
			})

			t.Run("GET /asset serves static file", func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/"+assetName, http.NoBody)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, wantAssetContent, w.Body.String())
			})

			t.Run("API routes remain functional", func(t *testing.T) {
				subpath := fmt.Sprintf("%s/subpath", fake.Lorem().Word())
				req := httptest.NewRequest(
					http.MethodPost,
					fmt.Sprintf("/api/v1/runtime/%s", subpath),
					http.NoBody,
				)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "runtime", w.Body.String())
			})
		})

		t.Run(
			"when ui location is invalid directory, server operates in API-only mode",
			func(t *testing.T) {
				nonExistentDir := filepath.Join(t.TempDir(), fake.Lorem().Word())
				_, _, rootHandler, _, _, _ := makeSetup(t, nonExistentDir, "")

				t.Run("GET / returns 404", func(t *testing.T) {
					req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
					w := httptest.NewRecorder()
					rootHandler.ServeHTTP(w, req)
					assert.Equal(t, http.StatusNotFound, w.Code)
				})
			},
		)
	})
}
