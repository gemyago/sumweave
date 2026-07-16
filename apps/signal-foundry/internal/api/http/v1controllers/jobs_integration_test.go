package v1controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/flows"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/stretchr/testify/require"
)

type jobsIntegrationRawRecorder struct {
	lineageService *data.LineageService
	countByRun     map[string]int
}

func (r *jobsIntegrationRawRecorder) RecordHyperliquidRawEvidence(
	ctx context.Context,
	capture venueedge.HyperliquidRawEvidenceCapture,
) (string, error) {
	if r.countByRun == nil {
		r.countByRun = make(map[string]int)
	}
	r.countByRun[capture.IngestionRunID]++

	payload, err := data.NewRawVenuePayload(data.RawVenuePayloadParams{
		ID:                 capture.ID,
		IngestionRunID:     capture.IngestionRunID,
		Source:             string(capture.Venue) + "-rest",
		Venue:              capture.Venue,
		Endpoint:           capture.Endpoint,
		RequestType:        capture.RequestType,
		RequestPayloadHash: capture.RequestPayloadHash,
		RequestMetadata:    capture.RequestMetadata,
		RequestAt:          capture.RequestAt,
		ResponseAt:         capture.ResponseAt,
		HTTPStatus:         capture.HTTPStatus,
		ResponseBody:       capture.ResponseBody,
		ResponseBodyHash:   hashJobsIntegrationBytes(capture.ResponseBody),
		EntityHint:         capture.EntityHint,
		Timeframe:          capture.Timeframe,
		TimeRange:          capture.TimeRange,
		ReceivedAt:         capture.ReceivedAt,
	})
	if err != nil {
		return "", err
	}

	persisted, err := r.lineageService.RecordRawVenuePayload(ctx, payload)
	if err != nil {
		return "", err
	}
	return persisted.ID, nil
}

func hashJobsIntegrationBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestJobsControllerIntegration(t *testing.T) {
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

	rangeStart := time.UnixMilli(1710000000000).UTC()
	rangeEnd := rangeStart.Add(3 * time.Minute)
	testUserID := "operator-integration"
	sharedDSN := filepath.Join(t.TempDir(), "app.db")

	serverFixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Type string `json:"type"`
			Req  struct {
				StartTime int64 `json:"startTime"`
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
		if payload.Req.StartTime != rangeStart.UnixMilli() {
			t.Errorf("unexpected start time: %d", payload.Req.StartTime)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `[
			{"t":1710000000000,"T":1710000059999,"s":"BTC","i":"1m","o":62000,"c":62010,"h":62020,"l":61990,"v":12.5},
			{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75}
		]`)
	}))
	t.Cleanup(serverFixture.Close)
	sharedDB, err := sqlconn.Open(sharedDSN)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sharedDB.Close()) })

	dataStore, err := data.NewDatabaseStore(sharedDB, sharedDSN, data.DatabaseStoreOpts{})
	require.NoError(t, err)
	require.NoError(t, dataStore.AutoMigrate())

	ingestionService, err := data.NewIngestionService(data.IngestionServiceDeps{
		InstrumentStore: dataStore,
		CandleStore:     dataStore,
		TradeStore:      dataStore,
	})
	require.NoError(t, err)

	readService, err := data.NewReadService(data.ReadServiceDeps{
		InstrumentStore: dataStore,
		CandleStore:     dataStore,
		TradeStore:      dataStore,
	})
	require.NoError(t, err)

	lineageService, err := data.NewLineageService(data.LineageServiceDeps{Store: dataStore, BlobStore: dataStore})
	require.NoError(t, err)

	ingestionFlow, err := venueedge.NewIngestionFlow(ingestionService)
	require.NoError(t, err)
	ingestionFlow.WithRawPayloadLineage(lineageService)

	rawRecorder := &jobsIntegrationRawRecorder{lineageService: lineageService}
	runner, err := flows.NewHistoricalRawCandleBackfillRunner(flows.HistoricalRawCandleBackfillRunnerDeps{
		RecordIngestionRun: lineageService.RecordIngestionRun,
		BuildVenue: func(
			_ context.Context,
			params flows.HistoricalRawCandleBackfillVenueBuildParams,
		) (venueedge.MarketDataVenue, error) {
			return venueedge.NewHyperliquidPerpsVenue(venueedge.HyperliquidPerpsVenueParams{
				BaseURL:                 serverFixture.URL,
				HTTPClient:              serverFixture.Client(),
				RawEvidenceRecorder:     rawRecorder,
				RawEvidenceIngestionRun: params.RawEvidenceIngestionRun,
			})
		},
		IngestCandles:          ingestionFlow.IngestCandles,
		ReadPersistedCandles:   readService.QueryCandles,
		ReplayPersistedCandles: readService.ReplayCandles,
		CountRunRawPayloads: func(_ context.Context, runID string) (int, error) {
			return rawRecorder.countByRun[runID], nil
		},
	})
	require.NoError(t, err)

	jobsStore, err := jobspkg.NewStore(sharedDB, sharedDSN, jobspkg.StoreOpts{})
	require.NoError(t, err)
	require.NoError(t, jobsStore.AutoMigrate())
	require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{DatabaseDSN: sharedDSN}, sharedDB))
	publisher, err := appdispatch.NewPublisher(appdispatch.Config{DatabaseDSN: sharedDSN}, sharedDB, slog.Default())
	require.NoError(t, err)

	jobsService, err := jobspkg.NewService(jobspkg.ServiceDeps{
		Store:       jobsStore,
		IDGenerator: ident.NewDefaultGenerator(),
		Publisher:   publisher,
	})
	require.NoError(t, err)

	authMiddleware := makeAuthMiddleware(testUserID)
	handler := server.NewTestRootHandler().
		RegisterJobsRoutes(NewJobsController(JobsControllerDeps{JobsService: jobsService, AuthMiddleware: authMiddleware})).
		RegisterDataRoutes(NewDataController(DataControllerDeps{ReadService: readService, LineageService: lineageService, AuthMiddleware: authMiddleware}))

	newRequest := func(method, target, body string) *http.Request {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req = req.WithContext(t.Context())
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, newRequest(
		http.MethodPost,
		"/api/v1/jobs/historical-data-backfills",
		fmt.Sprintf(
			`{"idempotencyKey":"integration-key","venue":"hyperliquid-perps","symbol":"BTC","assetClass":"future","timeframe":"1m","start":"%s","end":"%s","pageSize":10}`,
			rangeStart.Format(time.RFC3339),
			rangeEnd.Format(time.RFC3339),
		),
	))
	require.Equal(t, http.StatusOK, createResp.Code)

	var created models.JobDetailResponse
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &created))
	require.Equal(t, "queued", created.Status)
	require.NotNil(t, created.Requester)
	require.Equal(t, testUserID, created.Requester.UserID)

	queuedBeforeWorkerResp := httptest.NewRecorder()
	handler.ServeHTTP(queuedBeforeWorkerResp, newRequest(http.MethodGet, "/api/v1/jobs/"+created.ID, ""))
	require.Equal(t, http.StatusOK, queuedBeforeWorkerResp.Code)
	var queuedBeforeWorker models.JobDetailResponse
	require.NoError(t, json.Unmarshal(queuedBeforeWorkerResp.Body.Bytes(), &queuedBeforeWorker))
	require.Equal(t, "queued", queuedBeforeWorker.Status)
	require.Nil(t, queuedBeforeWorker.Result)

	worker, err := jobspkg.NewWorker(jobspkg.WorkerDeps{
		Store:  jobsStore,
		Runner: runner,
		Config: jobspkg.WorkerConfig{Enabled: true, PollInterval: 10 * time.Millisecond},
	})
	require.NoError(t, err)
	require.NoError(t, worker.ProcessJob(t.Context(), created.ID))

	terminalResp := httptest.NewRecorder()
	handler.ServeHTTP(terminalResp, newRequest(http.MethodGet, "/api/v1/jobs/"+created.ID, ""))
	require.Equal(t, http.StatusOK, terminalResp.Code)
	var terminal models.JobDetailResponse
	require.NoError(t, json.Unmarshal(terminalResp.Body.Bytes(), &terminal))
	require.Equal(t, "succeeded", terminal.Status)

	require.NotNil(t, terminal.Result)
	require.Equal(t, created.Input.IngestionRunID, terminal.Result.IngestionRunID)
	require.Equal(t, int64(2), terminal.Result.PersistedCount)
	require.Equal(t, int64(3), terminal.Result.ExpectedCount)
	require.Equal(t, int64(1), terminal.Result.MissingIntervalCount)
	require.NotNil(t, terminal.Result.RawPayloadCount)
	require.Equal(t, int64(1), *terminal.Result.RawPayloadCount)
	require.Len(t, terminal.Result.MissingIntervalPreview, 1)

	listResp := httptest.NewRecorder()
	handler.ServeHTTP(
		listResp,
		newRequest(
			http.MethodGet,
			"/api/v1/jobs?status=succeeded&jobType=data.historical_raw_candle_backfill&source=operator",
			"",
		),
	)
	require.Equal(t, http.StatusOK, listResp.Code)
	var listed models.JobListResponse
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listed))
	require.Len(t, listed.Items, 1)
	require.Equal(t, created.ID, listed.Items[0].ID)

	dataResp := httptest.NewRecorder()
	dataURL := "/api/v1/data/candles?venue=hyperliquid-perps&symbol=BTC&assetClass=future&timeframe=1m&start=" +
		rangeStart.Format(time.RFC3339) +
		"&end=" + rangeEnd.Format(time.RFC3339)
	handler.ServeHTTP(dataResp, newRequest(
		http.MethodGet,
		dataURL,
		"",
	))
	require.Equal(t, http.StatusOK, dataResp.Code)
	var candles models.DataCandleListResponse
	require.NoError(t, json.Unmarshal(dataResp.Body.Bytes(), &candles))
	require.Len(t, candles.Items, 2)

	restartedStore, err := jobspkg.NewStore(sharedDB, sharedDSN, jobspkg.StoreOpts{})
	require.NoError(t, err)
	restartedService, err := jobspkg.NewService(jobspkg.ServiceDeps{
		Store:       restartedStore,
		IDGenerator: ident.NewDefaultGenerator(),
		Publisher:   publisher,
	})
	require.NoError(t, err)
	restartedHandler := server.NewTestRootHandler().RegisterJobsRoutes(
		NewJobsController(JobsControllerDeps{JobsService: restartedService, AuthMiddleware: authMiddleware}),
	)

	restartDetailResp := httptest.NewRecorder()
	restartedHandler.ServeHTTP(restartDetailResp, newRequest(http.MethodGet, "/api/v1/jobs/"+created.ID, ""))
	require.Equal(t, http.StatusOK, restartDetailResp.Code)
	var restartedDetail models.JobDetailResponse
	require.NoError(t, json.Unmarshal(restartDetailResp.Body.Bytes(), &restartedDetail))
	require.Equal(t, "succeeded", restartedDetail.Status)
	require.NotNil(t, restartedDetail.Result)
	require.Equal(t, terminal.Result.IngestionRunID, restartedDetail.Result.IngestionRunID)
}
