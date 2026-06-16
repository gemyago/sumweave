package v1controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type strategyWorkspaceServiceStub struct {
	validateDefinitionFunc func(context.Context, app.StrategyDefinitionInput) (app.StrategyValidationResult, error)
	createVersionFunc      func(context.Context, app.CreateStrategyVersionParams) (*app.StrategyVersionRecord, error)
	listVersionsFunc       func(context.Context) ([]app.StrategyVersionRecord, error)
	getVersionFunc         func(context.Context, string, string) (*app.StrategyVersionRecord, error)
	duplicateVersionFunc   func(context.Context, string, string) (*app.StrategyVersionCandidate, error)
}

func (s *strategyWorkspaceServiceStub) ValidateDefinition(
	ctx context.Context,
	definition app.StrategyDefinitionInput,
) (app.StrategyValidationResult, error) {
	if s.validateDefinitionFunc == nil {
		return app.StrategyValidationResult{}, errors.New("unexpected ValidateDefinition call")
	}
	return s.validateDefinitionFunc(ctx, definition)
}

func (s *strategyWorkspaceServiceStub) CreateVersion(
	ctx context.Context,
	params app.CreateStrategyVersionParams,
) (*app.StrategyVersionRecord, error) {
	if s.createVersionFunc == nil {
		return nil, errors.New("unexpected CreateVersion call")
	}
	return s.createVersionFunc(ctx, params)
}

func (s *strategyWorkspaceServiceStub) ListVersions(
	ctx context.Context,
) ([]app.StrategyVersionRecord, error) {
	if s.listVersionsFunc == nil {
		return nil, errors.New("unexpected ListVersions call")
	}
	return s.listVersionsFunc(ctx)
}

func (s *strategyWorkspaceServiceStub) GetVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*app.StrategyVersionRecord, error) {
	if s.getVersionFunc == nil {
		return nil, errors.New("unexpected GetVersion call")
	}
	return s.getVersionFunc(ctx, strategyID, version)
}

func (s *strategyWorkspaceServiceStub) DuplicateVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*app.StrategyVersionCandidate, error) {
	if s.duplicateVersionFunc == nil {
		return nil, errors.New("unexpected DuplicateVersion call")
	}
	return s.duplicateVersionFunc(ctx, strategyID, version)
}

func TestStrategiesController(t *testing.T) {
	fake := faker.New()

	makeAuthMiddleware := func() middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			})
		}
	}

	newController := func(service strategyWorkspaceService) *StrategiesController {
		return NewStrategiesController(StrategiesControllerDeps{
			StrategyWorkspaceService: service,
			AuthMiddleware:           makeAuthMiddleware(),
		})
	}

	newHandler := func(ctrl *StrategiesController) http.Handler {
		return server.NewTestRootHandler().RegisterStrategiesRoutes(ctrl)
	}

	newRequest := func(method, target, body string, authenticated bool) *http.Request {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req = req.WithContext(t.Context())
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+fake.Lorem().Word())
		}
		return req
	}

	t.Run("all strategy endpoints require auth", func(t *testing.T) {
		ctrl := newController(&strategyWorkspaceServiceStub{})
		handler := newHandler(ctrl)

		cases := []struct {
			method string
			url    string
			body   string
		}{
			{method: http.MethodGet, url: "/api/v1/strategies"},
			{method: http.MethodGet, url: "/api/v1/strategies/strategy-a/versions/v1"},
			{
				method: http.MethodPost,
				url:    "/api/v1/strategies/validate",
				body:   `{"definition":{"kind":"moving-average-crossover","instrument":{"venue":"binance","symbol":"BTCUSDT","assetClass":"crypto","active":true},"timeframe":"1h","parameters":{"fastWindow":9,"slowWindow":21}}}`,
			},
			{
				method: http.MethodPost,
				url:    "/api/v1/strategies/versions",
				body:   `{"strategyId":"strategy-a","version":"v1","displayName":"Example","definition":{"kind":"moving-average-crossover","instrument":{"venue":"binance","symbol":"BTCUSDT","assetClass":"crypto","active":true},"timeframe":"1h","parameters":{"fastWindow":9,"slowWindow":21}}}`,
			},
			{method: http.MethodPost, url: "/api/v1/strategies/strategy-a/versions/v1/duplicate"},
		}

		for _, tc := range cases {
			t.Run(tc.method+" "+tc.url, func(t *testing.T) {
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, newRequest(tc.method, tc.url, tc.body, false))
				require.Equal(t, http.StatusUnauthorized, resp.Code)
			})
		}
	})

	t.Run("ValidateStrategy returns camelCase validation preview payload", func(t *testing.T) {
		ctrl := newController(&strategyWorkspaceServiceStub{
			validateDefinitionFunc: func(_ context.Context, definition app.StrategyDefinitionInput) (app.StrategyValidationResult, error) {
				require.Equal(t, "moving-average-crossover", definition.Kind)
				return app.StrategyValidationResult{
					Valid: true,
					Preview: &app.StrategyValidationPreview{
						SchemaVersion: rtstrategy.ArtifactSchemaVersion,
						Kind:          "moving-average-crossover",
						Instrument: app.StrategyInstrumentInput{
							Venue:      "binance",
							Symbol:     "BTCUSDT",
							AssetClass: "crypto",
							Active:     true,
						},
						Timeframe: "1h",
						ParameterSummary: app.StrategyParameterSummary{
							FastWindow: 9,
							SlowWindow: 21,
						},
						CanonicalJSON:    `{"schemaVersion":"strategy-artifact.v0"}`,
						ArtifactHash:     fake.UUID().V4(),
						ExistingArtifact: true,
					},
					Errors: []app.StrategyFieldError{},
				}, nil
			},
		})
		handler := newHandler(ctrl)

		resp := httptest.NewRecorder()
		handler.ServeHTTP(
			resp,
			newRequest(
				http.MethodPost,
				"/api/v1/strategies/validate",
				`{"definition":{"kind":"moving-average-crossover","instrument":{"venue":"binance","symbol":"BTCUSDT","assetClass":"crypto","active":true},"timeframe":"1h","parameters":{"fastWindow":9,"slowWindow":21}}}`,
				true,
			),
		)
		require.Equal(t, http.StatusOK, resp.Code)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		require.Equal(t, true, payload["valid"])
		preview := payload["preview"].(map[string]any)
		require.Contains(t, preview, "schemaVersion")
		require.Contains(t, preview, "parameterSummary")
		require.Contains(t, preview, "canonicalJson")
		require.Contains(t, preview, "artifactHash")
		require.Contains(t, preview, "existingArtifact")
		instrument := preview["instrument"].(map[string]any)
		require.Contains(t, instrument, "assetClass")
		parameters := preview["parameterSummary"].(map[string]any)
		require.Contains(t, parameters, "fastWindow")
		require.Contains(t, parameters, "slowWindow")
	})

	t.Run(
		"list get create and duplicate routes return protected strategy payloads",
		func(t *testing.T) {
			createdAt := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
			service := &strategyWorkspaceServiceStub{}
			service.listVersionsFunc = func(context.Context) ([]app.StrategyVersionRecord, error) {
				return []app.StrategyVersionRecord{makeStrategyVersionRecord(createdAt)}, nil
			}
			service.getVersionFunc = func(context.Context, string, string) (*app.StrategyVersionRecord, error) {
				record := makeStrategyVersionRecord(createdAt)
				record.ParentStrategyID = "strategy-parent"
				record.ParentVersion = "v0"
				return &record, nil
			}
			service.createVersionFunc = func(_ context.Context, params app.CreateStrategyVersionParams) (*app.StrategyVersionRecord, error) {
				require.Equal(t, "strategy-a", strings.TrimSpace(params.StrategyID))
				record := makeStrategyVersionRecord(createdAt)
				record.ParentStrategyID = "strategy-parent"
				record.ParentVersion = "v0"
				return &record, nil
			}
			service.duplicateVersionFunc = func(context.Context, string, string) (*app.StrategyVersionCandidate, error) {
				return &app.StrategyVersionCandidate{
					StrategyID:       "strategy-a",
					Version:          "",
					DisplayName:      "Example",
					Status:           "draft",
					SourceType:       "human",
					SourceLabel:      "Human",
					Notes:            "Draft notes",
					ParentStrategyID: "strategy-a",
					ParentVersion:    "v1",
					Definition:       makeStrategyDefinition(),
				}, nil
			}

			ctrl := newController(service)
			handler := newHandler(ctrl)

			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/strategies", "", true))
			require.Equal(t, http.StatusOK, resp.Code)
			require.Contains(t, resp.Body.String(), "sourceLabel")

			resp = httptest.NewRecorder()
			handler.ServeHTTP(
				resp,
				newRequest(http.MethodGet, "/api/v1/strategies/strategy-a/versions/v1", "", true),
			)
			require.Equal(t, http.StatusOK, resp.Code)
			require.Contains(t, resp.Body.String(), "artifactHash")
			require.Contains(t, resp.Body.String(), "parentStrategyId")

			resp = httptest.NewRecorder()
			handler.ServeHTTP(
				resp,
				newRequest(
					http.MethodPost,
					"/api/v1/strategies/versions",
					`{"strategyId":"strategy-a","version":"v1","displayName":"Example","parentStrategyId":"strategy-parent","parentVersion":"v0","definition":{"kind":"moving-average-crossover","instrument":{"venue":"binance","symbol":"BTCUSDT","assetClass":"crypto","active":true},"timeframe":"1h","parameters":{"fastWindow":9,"slowWindow":21}}}`,
					true,
				),
			)
			require.Equal(t, http.StatusOK, resp.Code)
			require.Contains(t, resp.Body.String(), "schemaVersion")

			resp = httptest.NewRecorder()
			handler.ServeHTTP(
				resp,
				newRequest(
					http.MethodPost,
					"/api/v1/strategies/strategy-a/versions/v1/duplicate",
					"",
					true,
				),
			)
			require.Equal(t, http.StatusOK, resp.Code)
			require.Contains(t, resp.Body.String(), "sourceType")
			require.Contains(t, resp.Body.String(), "parentVersion")
		},
	)

	t.Run("helper mapping and error routes stay deterministic", func(t *testing.T) {
		definition := mapStrategyDefinitionInput(nil)
		require.Equal(t, app.StrategyDefinitionInput{}, definition)

		definition = mapStrategyDefinitionInput(
			&models.StrategyDefinition{Kind: "moving-average-crossover"},
		)
		require.Equal(t, "moving-average-crossover", definition.Kind)
		require.Equal(t, app.StrategyParameterSummary{}, definition.Parameters)

		errCases := []struct {
			name    string
			method  string
			url     string
			body    string
			service *strategyWorkspaceServiceStub
			status  int
		}{
			{
				name:   "validate error",
				method: http.MethodPost,
				url:    "/api/v1/strategies/validate",
				body:   `{"definition":{"kind":"moving-average-crossover","instrument":{"venue":"binance","symbol":"BTCUSDT","assetClass":"crypto","active":true},"timeframe":"1h","parameters":{"fastWindow":9,"slowWindow":21}}}`,
				service: &strategyWorkspaceServiceStub{
					validateDefinitionFunc: func(context.Context, app.StrategyDefinitionInput) (app.StrategyValidationResult, error) {
						return app.StrategyValidationResult{}, app.NewErrInvalidInput(
							"definition",
							"bad definition",
						)
					},
				},
				status: http.StatusBadRequest,
			},
			{
				name:   "list error",
				method: http.MethodGet,
				url:    "/api/v1/strategies",
				service: &strategyWorkspaceServiceStub{
					listVersionsFunc: func(context.Context) ([]app.StrategyVersionRecord, error) {
						return nil, app.NewErrInvalidInput("request", "list failed")
					},
				},
				status: http.StatusBadRequest,
			},
			{
				name:   "get error",
				method: http.MethodGet,
				url:    "/api/v1/strategies/strategy-a/versions/v1",
				service: &strategyWorkspaceServiceStub{
					getVersionFunc: func(context.Context, string, string) (*app.StrategyVersionRecord, error) {
						return nil, app.NewErrNotFound("strategy version", "strategy-a/v1")
					},
				},
				status: http.StatusNotFound,
			},
			{
				name:   "create error",
				method: http.MethodPost,
				url:    "/api/v1/strategies/versions",
				body:   `{"strategyId":"strategy-a","version":"v1","displayName":"Example","definition":{"kind":"moving-average-crossover","instrument":{"venue":"binance","symbol":"BTCUSDT","assetClass":"crypto","active":true},"timeframe":"1h","parameters":{"fastWindow":9,"slowWindow":21}}}`,
				service: &strategyWorkspaceServiceStub{
					createVersionFunc: func(context.Context, app.CreateStrategyVersionParams) (*app.StrategyVersionRecord, error) {
						return nil, app.NewErrInvalidInput("request", "create failed")
					},
				},
				status: http.StatusBadRequest,
			},
			{
				name:   "duplicate error",
				method: http.MethodPost,
				url:    "/api/v1/strategies/strategy-a/versions/v1/duplicate",
				service: &strategyWorkspaceServiceStub{
					duplicateVersionFunc: func(context.Context, string, string) (*app.StrategyVersionCandidate, error) {
						return nil, app.NewErrNotFound("strategy version", "strategy-a/v1")
					},
				},
				status: http.StatusNotFound,
			},
		}

		for _, tc := range errCases {
			t.Run(tc.name, func(t *testing.T) {
				handler := newHandler(newController(tc.service))
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, newRequest(tc.method, tc.url, tc.body, true))
				require.Equal(t, tc.status, resp.Code)
			})
		}
	})
}

func makeStrategyDefinition() app.StrategyDefinitionInput {
	return app.StrategyDefinitionInput{
		Kind: "moving-average-crossover",
		Instrument: app.StrategyInstrumentInput{
			Venue:      "binance",
			Symbol:     "BTCUSDT",
			AssetClass: "crypto",
			Active:     true,
		},
		Timeframe:  "1h",
		Parameters: app.StrategyParameterSummary{FastWindow: 9, SlowWindow: 21},
	}
}

func makeStrategyVersionRecord(createdAt time.Time) app.StrategyVersionRecord {
	return app.StrategyVersionRecord{
		StrategyID:    "strategy-a",
		Version:       "v1",
		DisplayName:   "Example",
		Status:        "ready",
		SourceType:    "demo",
		SourceLabel:   "Demo example",
		ArtifactHash:  "hash-a",
		SchemaVersion: rtstrategy.ArtifactSchemaVersion,
		Kind:          "moving-average-crossover",
		Instrument: app.StrategyInstrumentInput{
			Venue:      "binance",
			Symbol:     "BTCUSDT",
			AssetClass: "crypto",
			Active:     true,
		},
		Timeframe:        "1h",
		ParameterSummary: app.StrategyParameterSummary{FastWindow: 9, SlowWindow: 21},
		Notes:            "Demo example only",
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
		Definition:       makeStrategyDefinition(),
	}
}
