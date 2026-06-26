//go:build !release

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1controllers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	appinternal "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/strategyassistant"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

type strategyAssistantSmokeToolContext struct {
	context.Context

	sessionID    string
	invocationID string
	userID       string
}

func (c *strategyAssistantSmokeToolContext) SessionID() string {
	return c.sessionID
}

func (c *strategyAssistantSmokeToolContext) InvocationID() string {
	return c.invocationID
}

func (c *strategyAssistantSmokeToolContext) UserID() string {
	return c.userID
}

type strategyAssistantSmokeCallerIdentity struct{ userID string }

func (c *strategyAssistantSmokeCallerIdentity) UserID() string { return c.userID }

type strategyAssistantSmokeRoundTrip func(*http.Request) (*http.Response, error)

func (f strategyAssistantSmokeRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type strategyAssistantSmokeJobsTools struct {
	start func(*agent.ToolContext, strategyassistant.StartHistoricalDataBackfillRequest) (strategyassistant.StartHistoricalDataBackfillResponse, error)
	list  func(*agent.ToolContext, strategyassistant.ListJobsRequest) (strategyassistant.ListJobsResponse, error)
	get   func(*agent.ToolContext, strategyassistant.GetJobRequest) (strategyassistant.GetJobResponse, error)
}

func lookupStrategyAssistantSmokeJobsTools(
	t *testing.T,
	registry *agent.ToolsRegistry,
) strategyAssistantSmokeJobsTools {
	t.Helper()

	toolsField := reflect.ValueOf(registry).Elem().FieldByName("tools")
	require.True(t, toolsField.IsValid())
	toolsField = reflect.NewAt(toolsField.Type(), unsafe.Pointer(toolsField.UnsafeAddr())).Elem()

	result := strategyAssistantSmokeJobsTools{}
	for index := range toolsField.Len() {
		toolValue := reflect.ValueOf(toolsField.Index(index).Interface())
		name := toolValue.FieldByName("Name").String()
		handler := toolValue.FieldByName("Handler")
		callHandler := func(args ...reflect.Value) ([]reflect.Value, error) {
			if !handler.IsValid() || handler.IsNil() {
				return nil, fmt.Errorf("tool %s handler is unavailable", name)
			}
			return handler.Call(args), nil
		}

		switch name {
		case "sf_jobs_start_historical_data_backfill":
			result.start = func(
				ctx *agent.ToolContext,
				input strategyassistant.StartHistoricalDataBackfillRequest,
			) (strategyassistant.StartHistoricalDataBackfillResponse, error) {
				outputs, err := callHandler(reflect.ValueOf(ctx), reflect.ValueOf(input))
				if err != nil {
					return strategyassistant.StartHistoricalDataBackfillResponse{}, err
				}
				response := outputs[0].Interface().(strategyassistant.StartHistoricalDataBackfillResponse)
				if outputs[1].IsNil() {
					return response, nil
				}
				return response, outputs[1].Interface().(error)
			}
		case "sf_jobs_list":
			result.list = func(
				ctx *agent.ToolContext,
				input strategyassistant.ListJobsRequest,
			) (strategyassistant.ListJobsResponse, error) {
				outputs, err := callHandler(reflect.ValueOf(ctx), reflect.ValueOf(input))
				if err != nil {
					return strategyassistant.ListJobsResponse{}, err
				}
				response := outputs[0].Interface().(strategyassistant.ListJobsResponse)
				if outputs[1].IsNil() {
					return response, nil
				}
				return response, outputs[1].Interface().(error)
			}
		case "sf_jobs_get":
			result.get = func(
				ctx *agent.ToolContext,
				input strategyassistant.GetJobRequest,
			) (strategyassistant.GetJobResponse, error) {
				outputs, err := callHandler(reflect.ValueOf(ctx), reflect.ValueOf(input))
				if err != nil {
					return strategyassistant.GetJobResponse{}, err
				}
				response := outputs[0].Interface().(strategyassistant.GetJobResponse)
				if outputs[1].IsNil() {
					return response, nil
				}
				return response, outputs[1].Interface().(error)
			}
		}
	}

	require.NotNil(t, result.start)
	require.NotNil(t, result.list)
	require.NotNil(t, result.get)
	return result
}

func TestStrategyAssistantSmoke(t *testing.T) {
	fake := faker.New()
	container, bundledPlatformSkillsRoot := makeWiredRuntimeContainer(t)

	type smokeDeps struct {
		dig.In

		DataStore            *data.DatabaseStore
		DataIngestionService *data.IngestionService
		DataReadService      *data.ReadService
		JobsService          *jobspkg.Service
		JobsWorker           *jobspkg.Worker
		Runtime              *Runtime
		ToolsRegistry        *agent.ToolsRegistry
		StrategyWorkspace    *appinternal.StrategyWorkspaceService
		EvaluationWorkspace  *appinternal.EvaluationWorkspaceService
	}

	var deps smokeDeps
	invokeErr := container.Invoke(func(resolved smokeDeps) {
		deps = resolved
	})
	require.NoError(t, invokeErr)
	require.NotNil(t, deps.DataStore)
	require.NotNil(t, deps.DataIngestionService)
	require.NotNil(t, deps.DataReadService)
	require.NotNil(t, deps.JobsService)
	require.NotNil(t, deps.JobsWorker)
	require.NotNil(t, deps.Runtime)
	require.NotNil(t, deps.ToolsRegistry)
	require.NotNil(t, deps.StrategyWorkspace)
	require.NotNil(t, deps.EvaluationWorkspace)
	require.NoError(t, deps.DataStore.AutoMigrate())

	makeInstrument := func(t *testing.T, symbol string) domain.Instrument {
		t.Helper()

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue("hyperliquid-perps"),
			Symbol:     domain.Symbol(symbol),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeDefinition := func(instrument domain.Instrument) appinternal.StrategyDefinitionInput {
		return appinternal.StrategyDefinitionInput{
			Kind: "moving-average-crossover",
			Instrument: appinternal.StrategyInstrumentInput{
				Venue:      instrument.Venue.String(),
				Symbol:     instrument.Symbol.String(),
				AssetClass: instrument.AssetClass.String(),
				Active:     instrument.Active,
			},
			Timeframe:  "1h",
			Parameters: appinternal.StrategyParameterSummary{FastWindow: 2, SlowWindow: 3},
		}
	}

	seedCandles := func(t *testing.T, instrument domain.Instrument, start time.Time, closes []float64) time.Time {
		t.Helper()

		for i, closeValue := range closes {
			openValue := closeValue
			if i > 0 {
				openValue = closes[i-1]
			}
			provenance, err := domain.NewSourceProvenance(
				"smoke-fixture",
				fmt.Sprintf("%s-%02d", instrument.Symbol, i+1),
			)
			require.NoError(t, err)

			candleStart := start.Add(time.Duration(i) * time.Hour)
			candle, err := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1h,
				TimeRange:  domain.TimeRange{Start: candleStart, End: candleStart.Add(time.Hour)},
				Open:       openValue,
				High:       max(openValue, closeValue) + 0.5,
				Low:        min(openValue, closeValue) - 0.5,
				Close:      closeValue,
				Volume:     100 + float64(i),
				Quality:    domain.DataQualityRaw,
				Provenance: provenance,
			})
			require.NoError(t, err)

			_, err = deps.DataIngestionService.IngestCandle(t.Context(), candle)
			require.NoError(t, err)
		}

		return start.Add(time.Duration(len(closes)) * time.Hour)
	}

	t.Run("completed loop stays executable with seeded local data", func(t *testing.T) {
		symbol := "BTC-" + fake.Lorem().Word()
		instrument := makeInstrument(t, symbol)
		rangeStart := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
		rangeEnd := seedCandles(t, instrument, rangeStart, []float64{10, 11, 10, 9, 11, 12, 9, 8})

		availability, err := deps.DataReadService.ListCandleAvailability(
			t.Context(),
			data.CandleAvailabilityListQuery{
				Venue:      instrument.Venue,
				Symbol:     instrument.Symbol,
				AssetClass: instrument.AssetClass,
				Limit:      10,
			},
		)
		require.NoError(t, err)
		require.Len(t, availability.Items, 1)
		assert.Equal(t, instrument.Symbol, availability.Items[0].Symbol)
		assert.Equal(t, domain.Timeframe1h, availability.Items[0].DefaultSlice.Timeframe)
		assert.Equal(t, rangeStart, availability.Items[0].DefaultSlice.StartAt)
		assert.Equal(t, rangeEnd, availability.Items[0].DefaultSlice.EndAt)

		replayed, err := deps.DataReadService.ReplayCandles(
			t.Context(),
			instrument,
			domain.Timeframe1h,
			domain.TimeRange{Start: rangeStart, End: rangeEnd},
		)
		require.NoError(t, err)
		require.Len(t, replayed, 8)
		assert.Equal(t, uint64(1), replayed[0].Identity)
		assert.InDelta(t, 8.0, replayed[len(replayed)-1].Candle.Close, 0.000001)

		definition := makeDefinition(instrument)
		validation, err := deps.StrategyWorkspace.ValidateDefinition(t.Context(), definition)
		require.NoError(t, err)
		require.True(t, validation.Valid)
		require.NotNil(t, validation.Preview)
		assert.Equal(t, "strategy-artifact.v0", validation.Preview.SchemaVersion)

		strategyID := "smoke-strategy-" + fake.Lorem().Word()
		createdVersion, err := deps.StrategyWorkspace.CreateVersion(
			t.Context(),
			appinternal.CreateStrategyVersionParams{
				StrategyID:  strategyID,
				Version:     "v1",
				DisplayName: "Smoke strategy",
				Notes:       "seeded smoke coverage",
				Definition:  definition,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "ready", createdVersion.Status)

		evaluation, err := deps.EvaluationWorkspace.CreateEvaluation(
			t.Context(),
			appinternal.CreateEvaluationParams{
				StrategyID:      strategyID,
				StrategyVersion: createdVersion.Version,
				Start:           rangeStart,
				End:             rangeEnd,
				Quantity:        1,
				Note:            "smoke evaluation",
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "completed", evaluation.Status)
		assert.Empty(t, evaluation.FailureReason)
		assert.NotEmpty(t, evaluation.RunID)

		report, err := deps.EvaluationWorkspace.GetEvaluationReport(t.Context(), evaluation.RunID)
		require.NoError(t, err)
		assert.Equal(t, evaluation.RunID, report.RunID)
		assert.Equal(t, "completed", report.Status)
		assert.NotNil(t, report.Decision)

		evidence, err := deps.EvaluationWorkspace.GetEvaluationEvidence(t.Context(), evaluation.RunID)
		require.NoError(t, err)
		assert.Equal(t, evaluation.RunID, evidence.RunID)
		assert.Equal(t, "completed", evidence.Status)
		assert.NotEmpty(t, evidence.Traces)

		skillBody, err := os.ReadFile(
			filepath.Join(bundledPlatformSkillsRoot, "strategy-research-loop", "SKILL.md"),
		)
		require.NoError(t, err)
		assert.Contains(t, string(skillBody), "sf_data_list_candle_availability")
		assert.Contains(t, string(skillBody), "Safety boundaries")

		historicalJobsSkillBody, err := os.ReadFile(
			filepath.Join(bundledPlatformSkillsRoot, "historical-data-jobs", "SKILL.md"),
		)
		require.NoError(t, err)
		assert.Contains(t, string(historicalJobsSkillBody), "sf_jobs_start_historical_data_backfill")
		assert.Contains(t, string(historicalJobsSkillBody), "Do not invent data")
	})

	t.Run("durable historical jobs product flow stays executable end to end", func(t *testing.T) {
		newRequest := func(method, target, body string) *http.Request {
			req := httptest.NewRequest(method, target, strings.NewReader(body))
			req = req.WithContext(t.Context())
			req.Header.Set("Authorization", "Bearer smoke-token")
			req.Header.Set("Content-Type", "application/json")
			return req
		}

		buildSnapshotJSON := func(symbol string, start time.Time, closes []float64) string {
			rows := make([]string, 0, len(closes))
			for index, closeValue := range closes {
				openValue := closeValue
				if index > 0 {
					openValue = closes[index-1]
				}
				candleStart := start.Add(time.Duration(index) * time.Minute)
				rows = append(rows, fmt.Sprintf(
					`{"t":%d,"T":%d,"s":%q,"i":"1m","o":%.0f,"c":%.0f,"h":%.0f,"l":%.0f,"v":%.0f}`,
					candleStart.UnixMilli(),
					candleStart.Add(time.Minute).Add(-time.Millisecond).UnixMilli(),
					symbol,
					openValue,
					closeValue,
					max(openValue, closeValue)+1,
					min(openValue, closeValue)-1,
					10+float64(index),
				))
			}
			return "[" + strings.Join(rows, ",") + "]"
		}

		rangeStart := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
		rangeEnd := rangeStart.Add(8 * time.Minute)
		toolRangeStart := time.Date(2026, time.June, 15, 1, 0, 0, 0, time.UTC)
		toolRangeEnd := toolRangeStart.Add(8 * time.Minute)

		serverFixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload struct {
				Type string `json:"type"`
				Req  struct {
					Coin      string `json:"coin"`
					StartTime int64  `json:"startTime"`
				} `json:"req"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode request body: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if payload.Type != "candleSnapshot" {
				t.Errorf("unexpected request type: %s", payload.Type)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			switch {
			case payload.Req.Coin == "BTC" && payload.Req.StartTime == rangeStart.UnixMilli():
				_, _ = fmt.Fprint(
					w,
					buildSnapshotJSON("BTC", rangeStart, []float64{101, 103, 102, 104, 106, 105, 107, 108}),
				)
			case payload.Req.Coin == "ETH" && payload.Req.StartTime == toolRangeStart.UnixMilli():
				_, _ = fmt.Fprint(
					w,
					buildSnapshotJSON("ETH", toolRangeStart, []float64{201, 202, 203, 204, 205, 206, 207, 208}),
				)
			default:
				t.Fatalf("unexpected backfill request: coin=%s start=%d", payload.Req.Coin, payload.Req.StartTime)
			}
		}))
		t.Cleanup(serverFixture.Close)

		originalTransport := http.DefaultClient.Transport
		if originalTransport == nil {
			originalTransport = http.DefaultTransport
		}
		parsedServerURL := serverFixture.URL
		http.DefaultClient.Transport = strategyAssistantSmokeRoundTrip(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "api.hyperliquid.xyz" {
				rewritten := req.Clone(req.Context())
				serverReq := httptest.NewRequest(http.MethodGet, parsedServerURL, nil)
				rewritten.URL.Scheme = serverReq.URL.Scheme
				rewritten.URL.Host = serverReq.URL.Host
				return serverFixture.Client().Transport.RoundTrip(rewritten)
			}
			return originalTransport.RoundTrip(req)
		})
		t.Cleanup(func() { http.DefaultClient.Transport = originalTransport })

		authMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				ctx := httpapi.ContextWithCallerIdentity(
					r.Context(),
					&strategyAssistantSmokeCallerIdentity{userID: "operator-smoke"},
				)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}

		handler := server.NewTestRootHandler().
			RegisterJobsRoutes(v1controllers.NewJobsController(v1controllers.JobsControllerDeps{
				JobsService:    deps.JobsService,
				AuthMiddleware: middleware.AuthMiddleware(authMiddleware),
			})).
			RegisterDataRoutes(v1controllers.NewDataController(v1controllers.DataControllerDeps{
				ReadService:    deps.DataReadService,
				LineageService: nil,
				AuthMiddleware: middleware.AuthMiddleware(authMiddleware),
			}))

		createResp := httptest.NewRecorder()
		handler.ServeHTTP(createResp, newRequest(
			http.MethodPost,
			"/api/v1/jobs/historical-data-backfills",
			fmt.Sprintf(
				`{"idempotencyKey":"smoke-operator","venue":"hyperliquid-perps","symbol":"BTC","assetClass":"future","timeframe":"1m","start":"%s","end":"%s","pageSize":10}`,
				rangeStart.Format(time.RFC3339),
				rangeEnd.Format(time.RFC3339),
			),
		))
		require.Equal(t, http.StatusOK, createResp.Code)

		var created models.JobDetailResponse
		require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &created))
		assert.Equal(t, "queued", created.Status)
		assert.Equal(t, "BTC", created.Input.Symbol)

		require.NoError(t, deps.JobsWorker.ProcessJob(t.Context(), created.ID))
		terminalResp := httptest.NewRecorder()
		handler.ServeHTTP(terminalResp, newRequest(http.MethodGet, "/api/v1/jobs/"+created.ID, ""))
		require.Equal(t, http.StatusOK, terminalResp.Code)
		var terminal models.JobDetailResponse
		require.NoError(t, json.Unmarshal(terminalResp.Body.Bytes(), &terminal))
		require.Equal(t, "succeeded", terminal.Status)
		require.NotNil(t, terminal.Result)
		assert.Equal(t, int64(8), terminal.Result.PersistedCount)
		assert.Equal(t, int64(8), terminal.Result.ExpectedCount)
		assert.Equal(t, int64(0), terminal.Result.MissingIntervalCount)

		listResp := httptest.NewRecorder()
		handler.ServeHTTP(listResp, newRequest(
			http.MethodGet,
			"/api/v1/jobs?status=succeeded&jobType=data.historical_raw_candle_backfill&source=operator",
			"",
		))
		require.Equal(t, http.StatusOK, listResp.Code)
		var listed models.JobListResponse
		require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listed))
		require.Len(t, listed.Items, 1)
		assert.Equal(t, created.ID, listed.Items[0].ID)

		dataResp := httptest.NewRecorder()
		handler.ServeHTTP(dataResp, newRequest(
			http.MethodGet,
			"/api/v1/data/candles?venue=hyperliquid-perps&symbol=BTC&assetClass=future&timeframe=1m&start="+
				rangeStart.Format(time.RFC3339)+"&end="+rangeEnd.Format(time.RFC3339),
			"",
		))
		require.Equal(t, http.StatusOK, dataResp.Code)
		var candles models.DataCandleListResponse
		require.NoError(t, json.Unmarshal(dataResp.Body.Bytes(), &candles))
		require.Len(t, candles.Items, 8)

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue("hyperliquid-perps"),
			Symbol:     domain.Symbol("BTC"),
			AssetClass: domain.AssetClass("future"),
			Active:     true,
		})
		require.NoError(t, err)
		definition := appinternal.StrategyDefinitionInput{
			Kind: "moving-average-crossover",
			Instrument: appinternal.StrategyInstrumentInput{
				Venue:      instrument.Venue.String(),
				Symbol:     instrument.Symbol.String(),
				AssetClass: instrument.AssetClass.String(),
				Active:     instrument.Active,
			},
			Timeframe:  "1m",
			Parameters: appinternal.StrategyParameterSummary{FastWindow: 2, SlowWindow: 3},
		}
		strategyID := "smoke-job-flow-" + fake.Lorem().Word()
		createdVersion, err := deps.StrategyWorkspace.CreateVersion(
			t.Context(),
			appinternal.CreateStrategyVersionParams{
				StrategyID:  strategyID,
				Version:     "v1",
				DisplayName: "Historical jobs smoke strategy",
				Notes:       "durable jobs smoke",
				Definition:  definition,
			},
		)
		require.NoError(t, err)

		evaluation, err := deps.EvaluationWorkspace.CreateEvaluation(
			t.Context(),
			appinternal.CreateEvaluationParams{
				StrategyID:      strategyID,
				StrategyVersion: createdVersion.Version,
				Start:           rangeStart,
				End:             rangeEnd,
				Quantity:        1,
				Note:            "post-backfill smoke evaluation",
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "completed", evaluation.Status)
		assert.Empty(t, evaluation.FailureReason)

		jobsTools := lookupStrategyAssistantSmokeJobsTools(t, deps.ToolsRegistry)
		toolCtx := &agent.ToolContext{Context: &strategyAssistantSmokeToolContext{
			Context:      t.Context(),
			sessionID:    "session-" + fake.Lorem().Word(),
			invocationID: "run-" + fake.Lorem().Word(),
			userID:       "assistant-" + fake.Lorem().Word(),
		}}

		toolStart, err := jobsTools.start(toolCtx, strategyassistant.StartHistoricalDataBackfillRequest{
			IdempotencyKey: "smoke-agent",
			Venue:          "hyperliquid-perps",
			Symbol:         "ETH",
			AssetClass:     "future",
			Timeframe:      "1m",
			Start:          toolRangeStart,
			End:            toolRangeEnd,
			PageSize:       10,
		})
		require.NoError(t, err)
		require.NotNil(t, toolStart.Job)
		assert.Equal(t, "queued", toolStart.Job.Status)
		assert.Equal(t, "agent", toolStart.Job.Requester.Source)
		require.NoError(t, deps.JobsWorker.ProcessJob(t.Context(), toolStart.Job.ID))

		toolList, err := jobsTools.list(toolCtx, strategyassistant.ListJobsRequest{
			Statuses: []string{"queued", "running", "succeeded"},
			Sources:  []string{"agent"},
			Limit:    10,
		})
		require.NoError(t, err)
		require.NotNil(t, toolList.Items)
		assert.NotEmpty(t, toolList.Items)

		toolTerminal, err := jobsTools.get(toolCtx, strategyassistant.GetJobRequest{JobID: toolStart.Job.ID})
		require.NoError(t, err)
		require.NotNil(t, toolTerminal.Job)
		require.Equal(t, "succeeded", toolTerminal.Job.Status)
		require.NotNil(t, toolTerminal.Job)
		require.NotNil(t, toolTerminal.Job.Result)
		assert.Equal(t, 8, toolTerminal.Job.Result.PersistedCount)

		skillEntries, err := os.ReadDir(bundledPlatformSkillsRoot)
		require.NoError(t, err)
		skillNames := make([]string, 0, len(skillEntries))
		for _, entry := range skillEntries {
			if entry.IsDir() {
				skillNames = append(skillNames, entry.Name())
			}
		}
		assert.Contains(t, skillNames, "historical-data-jobs")
		assert.Contains(t, skillNames, "strategy-dsl-v0")
		historicalJobsSkillBody, err := os.ReadFile(
			filepath.Join(bundledPlatformSkillsRoot, "historical-data-jobs", "SKILL.md"),
		)
		require.NoError(t, err)
		historicalJobsText := strings.ToLower(string(historicalJobsSkillBody))
		assert.Contains(t, historicalJobsText, "sf_jobs_start_historical_data_backfill")
		assert.Contains(t, historicalJobsText, "poll until terminal")

		strategyDSLSkillBody, err := os.ReadFile(
			filepath.Join(bundledPlatformSkillsRoot, "strategy-dsl-v0", "SKILL.md"),
		)
		require.NoError(t, err)
		assert.Contains(t, string(strategyDSLSkillBody), `"kind": "moving-average-crossover"`)
	})

	t.Run("missing local data persists explicit data-unavailable failure", func(t *testing.T) {
		instrument := makeInstrument(t, "ETH-"+fake.Lorem().Word())
		definition := makeDefinition(instrument)
		strategyID := "smoke-missing-data-" + fake.Lorem().Word()

		createdVersion, err := deps.StrategyWorkspace.CreateVersion(
			t.Context(),
			appinternal.CreateStrategyVersionParams{
				StrategyID:  strategyID,
				Version:     "v1",
				DisplayName: "Smoke missing data",
				Notes:       "assert honest failed-run behavior",
				Definition:  definition,
			},
		)
		require.NoError(t, err)

		rangeStart := time.Date(2026, time.June, 16, 0, 0, 0, 0, time.UTC)
		evaluation, err := deps.EvaluationWorkspace.CreateEvaluation(
			t.Context(),
			appinternal.CreateEvaluationParams{
				StrategyID:      strategyID,
				StrategyVersion: createdVersion.Version,
				Start:           rangeStart,
				End:             rangeStart.Add(8 * time.Hour),
				Quantity:        1,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "failed", evaluation.Status)
		assert.Equal(t, "replay-data-unavailable", evaluation.FailureReason)
		assert.NotEmpty(t, evaluation.FailureDetails)

		report, err := deps.EvaluationWorkspace.GetEvaluationReport(t.Context(), evaluation.RunID)
		require.NoError(t, err)
		assert.Equal(t, "failed", report.Status)
		assert.Equal(t, "replay-data-unavailable", report.FailureReason)

		evidence, err := deps.EvaluationWorkspace.GetEvaluationEvidence(t.Context(), evaluation.RunID)
		require.NoError(t, err)
		assert.Equal(t, "failed", evidence.Status)
		assert.Empty(t, evidence.Traces)
		assert.Empty(t, evidence.OrderIntents)
	})
}
