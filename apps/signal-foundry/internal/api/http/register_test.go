package http_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	signalfoundryhttp "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1controllers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testReplayReadService struct{}

func (s *testReplayReadService) ReplayCandles(
	_ context.Context,
	_ domain.Instrument,
	_ domain.Timeframe,
	_ domain.TimeRange,
) ([]data.ReplayCandle, error) {
	return []data.ReplayCandle{}, nil
}

type testLineageBrowserService struct{}

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

func TestSetupV1Routes(t *testing.T) {
	fake := faker.New()

	// passthroughMiddleware is an AuthMiddleware that passes all requests through.
	passthroughMiddleware := middleware.AuthMiddleware(func(next http.Handler) http.Handler {
		return next
	})

	makeSetup := func(t *testing.T, uiLocation string) (*server.HTTPRouter, *v1controllers.HealthController, http.Handler) {
		t.Helper()
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
		healthCtrl := &v1controllers.HealthController{}
		signalfoundryhttp.SetupV1Routes(signalfoundryhttp.V1RoutesDeps{
			HealthController: healthCtrl,
			AuthController:   authCtrl,
			DataController:   dataCtrl,
			AuthMiddleware:   passthroughMiddleware,
			RootHandler:      rootHandler,
			HTTPRouter:       router,
			Runtime:          rt,
			RootLogger:       telemetry.RootTestLogger(),
			UILocation:       uiLocation,
		})
		return router, healthCtrl, rootHandler
	}

	t.Run("should mount agent API routes", func(t *testing.T) {
		calls := []string{}
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

		signalfoundryhttp.SetupV1Routes(signalfoundryhttp.V1RoutesDeps{
			HealthController: &v1controllers.HealthController{},
			AuthController:   authCtrl,
			DataController:   dataCtrl,
			AuthMiddleware:   passthroughMiddleware,
			RootHandler:      rootHandler,
			HTTPRouter:       router,
			Runtime:          rt,
			RootLogger:       telemetry.RootTestLogger(),
		})

		t.Run("routes under /api/v1/runtime/ are handled by runtime HTTP handler", func(t *testing.T) {
			subpath := fmt.Sprintf("%s/subpath", fake.Lorem().Word())
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/runtime/%s", subpath), http.NoBody)
			w := httptest.NewRecorder()
			rootHandler.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, wantResult, w.Body.String())
		})

		t.Run("data routes are registered on the app router", func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/data/candles?venue=hyperliquid-perps&symbol=BTCUSD&assetClass=crypto&timeframe=1m&start=2026-06-15T12:00:00Z&end=2026-06-15T13:00:00Z",
				http.NoBody,
			)
			w := httptest.NewRecorder()
			rootHandler.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
		})
	})

	t.Run("UI serving", func(t *testing.T) {
		t.Run("when ui location is empty, server operates in API-only mode", func(t *testing.T) {
			_, _, rootHandler := makeSetup(t, "")

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
			require.NoError(t, os.WriteFile(filepath.Join(uiDir, "index.html"), []byte(wantIndexContent), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(uiDir, assetName), []byte(wantAssetContent), 0o600))

			_, _, rootHandler := makeSetup(t, uiDir)

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
				req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/runtime/%s", subpath), http.NoBody)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "runtime", w.Body.String())
			})
		})

		t.Run("when ui location is invalid directory, server operates in API-only mode", func(t *testing.T) {
			nonExistentDir := filepath.Join(t.TempDir(), fake.Lorem().Word())
			_, _, rootHandler := makeSetup(t, nonExistentDir)

			t.Run("GET / returns 404", func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
				w := httptest.NewRecorder()
				rootHandler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusNotFound, w.Code)
			})
		})
	})
}
