package flows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type historicalBackfillIntegrationRawRecorder struct {
	lineageService *data.LineageService
	captures       []venueedge.HyperliquidRawEvidenceCapture
	countByRun     map[string]int
}

func (r *historicalBackfillIntegrationRawRecorder) RecordHyperliquidRawEvidence(
	ctx context.Context,
	capture venueedge.HyperliquidRawEvidenceCapture,
) (string, error) {
	r.captures = append(r.captures, capture)
	if r.countByRun == nil {
		r.countByRun = make(map[string]int)
	}
	r.countByRun[capture.IngestionRunID]++

	var instrumentRef *data.BatchInstrumentRef
	if capture.Instrument != nil {
		instrumentRef = &data.BatchInstrumentRef{
			Symbol:     capture.Instrument.Symbol,
			AssetClass: capture.Instrument.AssetClass,
		}
	}

	payload, err := data.NewRawVenuePayload(data.RawVenuePayloadParams{
		ID:                 capture.ID,
		IngestionRunID:     capture.IngestionRunID,
		Source:             historicalRawCandleBackfillSource,
		Venue:              capture.Venue,
		Endpoint:           capture.Endpoint,
		RequestType:        capture.RequestType,
		RequestPayloadHash: capture.RequestPayloadHash,
		RequestMetadata:    capture.RequestMetadata,
		RequestAt:          capture.RequestAt,
		ResponseAt:         capture.ResponseAt,
		HTTPStatus:         capture.HTTPStatus,
		ResponseBody:       capture.ResponseBody,
		ResponseBodyHash:   hashHistoricalBackfillIntegrationBytes(capture.ResponseBody),
		EntityHint:         capture.EntityHint,
		Instrument:         instrumentRef,
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

func hashHistoricalBackfillIntegrationBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestHistoricalRawCandleBackfillRunnerWithHyperliquidAdapter(t *testing.T) {
	t.Parallel()

	fake := faker.New()
	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	makeRequest := func(t *testing.T, runID string) HistoricalRawCandleBackfillRequest {
		t.Helper()
		start := time.UnixMilli(1710000000000).UTC()
		timeRange, err := domain.NewTimeRange(start, start.Add(3*time.Minute))
		require.NoError(t, err)
		return HistoricalRawCandleBackfillRequest{
			RunID:      runID,
			Venue:      venueedge.HyperliquidPerpsVenueName,
			Symbol:     domain.Symbol("BTC"),
			AssetClass: domain.AssetClassFuture,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  timeRange,
			PageSize:   2,
		}
	}

	makeHarness := func(t *testing.T, handler http.Handler) (*HistoricalRawCandleBackfillRunner, *data.LineageService, *historicalBackfillIntegrationRawRecorder) {
		t.Helper()

		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)

		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := data.NewDatabaseStore(sqlDB, ":memory:", data.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		ingestionService, err := data.NewIngestionService(data.IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		readService, err := data.NewReadService(data.ReadServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		blobStore, err := data.NewLocalRawPayloadBlobStore(
			filepath.Join(t.TempDir(), randomWord("raw-payloads")),
		)
		require.NoError(t, err)

		lineageService, err := data.NewLineageService(data.LineageServiceDeps{Store: store, BlobStore: blobStore})
		require.NoError(t, err)

		flow, err := venueedge.NewIngestionFlow(ingestionService)
		require.NoError(t, err)
		flow.WithRawPayloadLineage(lineageService)

		rawRecorder := &historicalBackfillIntegrationRawRecorder{lineageService: lineageService}
		runner, err := NewHistoricalRawCandleBackfillRunner(HistoricalRawCandleBackfillRunnerDeps{
			RecordIngestionRun: lineageService.RecordIngestionRun,
			BuildVenue: func(
				_ context.Context,
				params HistoricalRawCandleBackfillVenueBuildParams,
			) (venueedge.MarketDataVenue, error) {
				return venueedge.NewHyperliquidPerpsVenue(venueedge.HyperliquidPerpsVenueParams{
					BaseURL:                 server.URL,
					HTTPClient:              server.Client(),
					RawEvidenceRecorder:     rawRecorder,
					RawEvidenceIngestionRun: params.RawEvidenceIngestionRun,
				})
			},
			IngestCandles:          flow.IngestCandles,
			ReadPersistedCandles:   readService.QueryCandles,
			ReplayPersistedCandles: readService.ReplayCandles,
			CountRunRawPayloads: func(_ context.Context, runID string) (int, error) {
				return rawRecorder.countByRun[runID], nil
			},
		})
		require.NoError(t, err)

		return runner, lineageService, rawRecorder
	}

	t.Run("captures each HTTP page and links raw payloads to canonical candles", func(t *testing.T) {
		t.Parallel()

		runner, lineageService, rawRecorder := makeHarness(
			t,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Type string `json:"type"`
					Req  struct {
						Coin      string `json:"coin"`
						Interval  string `json:"interval"`
						StartTime int64  `json:"startTime"`
						EndTime   int64  `json:"endTime"`
					} `json:"req"`
				}
				if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
					t.Errorf("decode request body: %v", decodeErr)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if payload.Type != "candleSnapshot" {
					t.Errorf("unexpected request type: %s", payload.Type)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				switch payload.Req.StartTime {
				case 1710000000000:
					fmt.Fprint(w, `[
					{"t":1710000000000,"T":1710000059999,"s":"BTC","i":"1m","o":62000,"c":62010,"h":62020,"l":61990,"v":12.5},
					{"t":1710000060000,"T":1710000119999,"s":"BTC","i":"1m","o":62010,"c":62025,"h":62040,"l":62005,"v":15.25},
					{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75}
				]`)
				case 1710000120000:
					fmt.Fprint(w, `[
					{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75}
				]`)
				default:
					t.Fatalf("unexpected start time: %d", payload.Req.StartTime)
				}
			}),
		)

		request := makeRequest(t, randomWord("run"))
		result, err := runner.Run(t.Context(), request)
		require.NoError(t, err)
		require.Len(t, result.PersistedCandles, 3)
		require.NotNil(t, result.Report.RawPayloadCount)
		require.Equal(t, 2, *result.Report.RawPayloadCount)
		require.Len(t, rawRecorder.captures, 2)
		require.Equal(t, request.RunID, rawRecorder.captures[0].IngestionRunID)
		require.Equal(t, request.RunID, rawRecorder.captures[1].IngestionRunID)

		firstTwoRawIDs := []string{rawRecorder.captures[0].ID}
		thirdRawIDs := []string{rawRecorder.captures[1].ID}
		for index, candle := range result.PersistedCandles {
			rawPayloadIDs, listErr := lineageService.ListCandleRawPayloadIDs(t.Context(), candle)
			require.NoError(t, listErr)
			if index < 2 {
				require.Equal(t, firstTwoRawIDs, rawPayloadIDs)
				continue
			}
			require.Equal(t, thirdRawIDs, rawPayloadIDs)
		}
	})

	t.Run("captures non-2xx and malformed responses before failure", func(t *testing.T) {
		t.Parallel()

		t.Run("non-2xx", func(t *testing.T) {
			t.Parallel()

			runner, _, rawRecorder := makeHarness(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				fmt.Fprint(w, `{"error":"upstream"}`)
			}))

			_, err := runner.Run(t.Context(), makeRequest(t, randomWord("run-status")))
			require.Error(t, err)
			require.Contains(t, err.Error(), "hyperliquid perps error")
			require.Len(t, rawRecorder.captures, 1)
			require.Equal(t, http.StatusBadGateway, rawRecorder.captures[0].HTTPStatus)
		})

		t.Run("malformed", func(t *testing.T) {
			t.Parallel()

			runner, _, rawRecorder := makeHarness(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `{`)
			}))

			_, err := runner.Run(t.Context(), makeRequest(t, randomWord("run-malformed")))
			require.Error(t, err)
			require.Contains(t, err.Error(), "decode response body")
			require.Len(t, rawRecorder.captures, 1)
			require.Equal(t, http.StatusOK, rawRecorder.captures[0].HTTPStatus)
			require.Equal(t, []byte(`{`), rawRecorder.captures[0].ResponseBody)
		})
	})

	t.Run("treats empty first backfills as successful gap reports", func(t *testing.T) {
		t.Parallel()

		runID := randomWord("run-empty")
		runner, _, rawRecorder := makeHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload struct {
				Req struct {
					StartTime int64 `json:"startTime"`
					EndTime   int64 `json:"endTime"`
				} `json:"req"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
				t.Errorf("decode request body: %v", decodeErr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, `[]`)
		}))

		result, err := runner.Run(t.Context(), makeRequest(t, runID))
		require.NoError(t, err)
		require.Empty(t, result.PersistedCandles)
		require.Equal(t, 3, result.Report.ExpectedCount)
		require.Zero(t, result.Report.PersistedCount)
		require.Equal(t, 3, result.Report.MissingIntervalCount)
		require.NotNil(t, result.Report.RawPayloadCount)
		require.Equal(t, 1, *result.Report.RawPayloadCount)
		require.Len(t, rawRecorder.captures, 1)
		require.Equal(t, runID, rawRecorder.captures[0].IngestionRunID)
	})

	t.Run("reruns stay idempotent for candles and append raw evidence by run", func(t *testing.T) {
		t.Parallel()

		runner, lineageService, rawRecorder := makeHarness(
			t,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Req struct {
						StartTime int64 `json:"startTime"`
					} `json:"req"`
				}
				if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
					t.Errorf("decode request body: %v", decodeErr)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if payload.Req.StartTime == 1710000000000 {
					fmt.Fprint(w, `[
					{"t":1710000000000,"T":1710000059999,"s":"BTC","i":"1m","o":62000,"c":62010,"h":62020,"l":61990,"v":12.5},
					{"t":1710000060000,"T":1710000119999,"s":"BTC","i":"1m","o":62010,"c":62025,"h":62040,"l":62005,"v":15.25},
					{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75}
				]`)
					return
				}
				fmt.Fprint(w, `[
				{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75}
			]`)
			}),
		)

		firstRunID := randomWord("run")
		secondRunID := randomWord("run")

		firstResult, err := runner.Run(t.Context(), makeRequest(t, firstRunID))
		require.NoError(t, err)
		secondResult, err := runner.Run(t.Context(), makeRequest(t, firstRunID))
		require.NoError(t, err)
		thirdResult, err := runner.Run(t.Context(), makeRequest(t, secondRunID))
		require.NoError(t, err)

		require.Len(t, firstResult.PersistedCandles, 3)
		require.Equal(t, firstResult.PersistedCandles, secondResult.PersistedCandles)
		require.Equal(t, firstResult.PersistedCandles, thirdResult.PersistedCandles)
		require.Equal(t, 4, rawRecorder.countByRun[firstRunID])
		require.Equal(t, 2, rawRecorder.countByRun[secondRunID])

		firstCandle := firstResult.PersistedCandles[0]
		firstCandleRawIDs, err := lineageService.ListCandleRawPayloadIDs(t.Context(), firstCandle)
		require.NoError(t, err)
		require.Len(t, firstCandleRawIDs, 3)
	})
}
