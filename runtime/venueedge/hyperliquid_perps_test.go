package venueedge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/stretchr/testify/require"
)

type stubHyperliquidRawEvidenceRecorder struct {
	ids      []string
	err      error
	captures []HyperliquidRawEvidenceCapture
}

func (r *stubHyperliquidRawEvidenceRecorder) RecordHyperliquidRawEvidence(
	_ context.Context,
	capture HyperliquidRawEvidenceCapture,
) (string, error) {
	clonedCapture := capture
	clonedCapture.RequestMetadata = maps.Clone(capture.RequestMetadata)
	clonedCapture.ResponseBody = append([]byte(nil), capture.ResponseBody...)
	if capture.Instrument != nil {
		instrumentCopy := *capture.Instrument
		clonedCapture.Instrument = &instrumentCopy
	}
	if capture.TimeRange != nil {
		timeRangeCopy := *capture.TimeRange
		clonedCapture.TimeRange = &timeRangeCopy
	}
	r.captures = append(r.captures, clonedCapture)
	if r.err != nil {
		return "", r.err
	}
	if len(r.ids) < len(r.captures) {
		return "", nil
	}
	return r.ids[len(r.captures)-1], nil
}

//nolint:cyclop // One top-level adapter test groups behavior with subtests.
func TestHyperliquidPerpsVenue(t *testing.T) {
	t.Parallel()

	type adapterParams struct {
		handler        http.Handler
		recorder       HyperliquidRawEvidenceRecorder
		ingestionRunID string
	}

	makeAdapter := func(t *testing.T, params adapterParams) *HyperliquidPerpsVenue {
		t.Helper()

		server := httptest.NewServer(params.handler)
		t.Cleanup(server.Close)

		adapter, err := NewHyperliquidPerpsVenue(HyperliquidPerpsVenueParams{
			BaseURL:                 server.URL,
			HTTPClient:              server.Client(),
			RawEvidenceRecorder:     params.recorder,
			RawEvidenceIngestionRun: params.ingestionRunID,
		})
		require.NoError(t, err)

		return adapter
	}

	hashJSON := func(t *testing.T, payload any) string {
		t.Helper()

		body, err := json.Marshal(payload)
		require.NoError(t, err)
		sum := sha256.Sum256(body)

		return hex.EncodeToString(sum[:])
	}

	makeInstrument := func() domain.Instrument {
		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      HyperliquidPerpsVenueName,
			Symbol:     domain.Symbol("BTC"),
			AssetClass: domain.AssetClassFuture,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeInstrumentRequest := func() InstrumentReadRequest {
		request, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:   HyperliquidPerpsVenueName,
			Symbols: []domain.Symbol{domain.Symbol("BTC")},
		})
		require.NoError(t, err)

		return request
	}

	makeCandleRequest := func(pageSize int) CandleReadRequest {
		timeRange, err := domain.NewTimeRange(
			time.UnixMilli(1710000000000).UTC(),
			time.UnixMilli(1710000180000).UTC(),
		)
		require.NoError(t, err)
		request, err := NewCandleReadRequest(CandleReadRequestParams{
			Instrument: makeInstrument(),
			Timeframe:  domain.Timeframe1m,
			TimeRange:  timeRange,
			PageSize:   pageSize,
		})
		require.NoError(t, err)

		return request
	}

	makeTradeRequest := func(pageSize int) TradeReadRequest {
		timeRange, err := domain.NewTimeRange(
			time.UnixMilli(1710000005000).UTC(),
			time.UnixMilli(1710000180000).UTC(),
		)
		require.NoError(t, err)
		request, err := NewTradeReadRequest(TradeReadRequestParams{
			Instrument: makeInstrument(),
			TimeRange:  timeRange,
			PageSize:   pageSize,
		})
		require.NoError(t, err)

		return request
	}

	makeTradeRequestWithRange := func(start, end time.Time, pageSize int) TradeReadRequest {
		timeRange, err := domain.NewTimeRange(start, end)
		require.NoError(t, err)
		request, err := NewTradeReadRequest(TradeReadRequestParams{
			Instrument: makeInstrument(),
			TimeRange:  timeRange,
			PageSize:   pageSize,
		})
		require.NoError(t, err)

		return request
	}

	t.Run("maps documented success responses into canonical records", func(t *testing.T) {
		t.Parallel()

		var requestBodies []map[string]any
		adapter := makeAdapter(t, adapterParams{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("unexpected method: %s", r.Method)
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if r.URL.Path != "/info" {
				t.Errorf("unexpected path: %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}

			var payload map[string]any
			if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
				t.Errorf("decode request body: %v", decodeErr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			requestBodies = append(requestBodies, payload)

			switch payload[hyperliquidInfoTypeKey] {
			case hyperliquidInfoTypeMeta:
				fmt.Fprint(w, `{
					"universe": [
						{"name": "BTC", "szDecimals": 5, "maxLeverage": 50},
						{"name": "ETH", "szDecimals": 4, "maxLeverage": 50, "isDelisted": true}
					],
					"marginTables": [
						[50, {"description": "", "marginTiers": [{"lowerBound": "0.0", "maxLeverage": 50}]}]
					]
				}`)
			case hyperliquidInfoTypeCandle:
				fmt.Fprint(w, `[
					{"t":1710000000000,"T":1710000059999,"s":"BTC","i":"1m","o":62000,"c":62010,"h":62020,"l":61990,"v":12.5,"n":42},
					{"t":1710000060000,"T":1710000119999,"s":"BTC","i":"1m","o":62010,"c":62025,"h":62040,"l":62005,"v":15.25,"n":37}
				]`)
			case hyperliquidInfoTypeTrades:
				fmt.Fprint(w, `[
					{"coin":"BTC","side":"B","px":"62001.5","sz":"0.125","hash":"0xabc","time":1710000005000,"tid":101,"users":["0x1","0x2"]},
					{"coin":"BTC","side":"A","px":"62002.0","sz":"0.250","hash":"0xdef","time":1710000015000,"tid":102,"users":["0x3","0x4"]}
				]`)
			default:
				http.NotFound(w, r)
			}
		})})

		instruments, err := adapter.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.NoError(t, err)
		require.Len(t, instruments.Instruments, 1)
		require.Empty(t, instruments.Metadata.RawPayloadIDs)
		require.Equal(t, HyperliquidPerpsVenueName, instruments.Instruments[0].Venue)
		require.Equal(t, domain.AssetClassFuture, instruments.Instruments[0].AssetClass)
		require.Equal(t, domain.Symbol("BTC"), instruments.Instruments[0].Symbol)

		candles, err := adapter.ReadCandles(t.Context(), makeCandleRequest(2))
		require.NoError(t, err)
		require.Len(t, candles.Candles, 2)
		require.Empty(t, candles.Metadata.RawPayloadIDs)
		require.Equal(t, domain.DataQualityRaw, candles.Candles[0].Quality)
		require.Equal(t, time.UnixMilli(1710000000000).UTC(), candles.Candles[0].TimeRange.Start)
		require.Equal(t, time.UnixMilli(1710000060000).UTC(), candles.Candles[0].TimeRange.End)

		trades, err := adapter.ReadTrades(t.Context(), makeTradeRequest(2))
		require.NoError(t, err)
		require.Len(t, trades.Trades, 2)
		require.Empty(t, trades.Metadata.RawPayloadIDs)
		require.Equal(t, domain.DataQualityRaw, trades.Trades[0].Quality)
		require.Equal(t, time.UnixMilli(1710000005000).UTC(), trades.Trades[0].EventTime)
		require.Equal(t, "hyperliquid-perps-rest", trades.Trades[0].Provenance.Source)

		require.Len(t, requestBodies, 3)
		require.Equal(t, hyperliquidInfoTypeMeta, requestBodies[0][hyperliquidInfoTypeKey])
		require.Empty(t, requestBodies[0]["dex"])
		require.Equal(t, hyperliquidInfoTypeCandle, requestBodies[1][hyperliquidInfoTypeKey])
		require.Equal(t, hyperliquidInfoTypeTrades, requestBodies[2][hyperliquidInfoTypeKey])
	})

	t.Run("captures raw payload metadata and preserves canonical results", func(t *testing.T) {
		t.Parallel()

		const (
			metaResponse   = `{"universe":[{"name":"BTC","isDelisted":false}]}`
			candleResponse = `[{"t":1710000000000,"T":1710000059999,"s":"BTC","i":"1m","o":62000,"c":62010,"h":62020,"l":61990,"v":12.5}]`
			tradeResponse  = `[{"coin":"BTC","px":"62001.5","sz":"0.125","time":1710000005000,"tid":101}]`
		)

		recorder := &stubHyperliquidRawEvidenceRecorder{ids: []string{"raw-meta", "raw-candle", "raw-trade"}}
		adapter := makeAdapter(t, adapterParams{
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("decode request body: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				switch payload[hyperliquidInfoTypeKey] {
				case hyperliquidInfoTypeMeta:
					fmt.Fprint(w, metaResponse)
				case hyperliquidInfoTypeCandle:
					fmt.Fprint(w, candleResponse)
				case hyperliquidInfoTypeTrades:
					fmt.Fprint(w, tradeResponse)
				default:
					http.NotFound(w, r)
				}
			}),
			recorder:       recorder,
			ingestionRunID: "ingestion-run-123",
		})

		instrumentResult, err := adapter.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.NoError(t, err)
		require.Equal(t, []string{"raw-meta"}, instrumentResult.Metadata.RawPayloadIDs)
		require.Len(t, instrumentResult.Instruments, 1)
		require.Equal(t, domain.Symbol("BTC"), instrumentResult.Instruments[0].Symbol)

		candleRequest := makeCandleRequest(2)
		candleResult, err := adapter.ReadCandles(t.Context(), candleRequest)
		require.NoError(t, err)
		require.Equal(t, []string{"raw-candle"}, candleResult.Metadata.RawPayloadIDs)
		require.Len(t, candleResult.Candles, 1)
		require.Equal(t, time.UnixMilli(1710000000000).UTC(), candleResult.Candles[0].TimeRange.Start)

		tradeRequest := makeTradeRequest(2)
		tradeResult, err := adapter.ReadTrades(t.Context(), tradeRequest)
		require.NoError(t, err)
		require.Equal(t, []string{"raw-trade"}, tradeResult.Metadata.RawPayloadIDs)
		require.Len(t, tradeResult.Trades, 1)
		require.Equal(t, time.UnixMilli(1710000005000).UTC(), tradeResult.Trades[0].EventTime)

		require.Len(t, recorder.captures, 3)

		require.Equal(t, "ingestion-run-123", recorder.captures[0].IngestionRunID)
		require.Equal(t, HyperliquidPerpsVenueName, recorder.captures[0].Venue)
		require.Equal(t, hyperliquidInfoEndpoint, recorder.captures[0].Endpoint)
		require.Equal(t, hyperliquidInfoTypeMeta, recorder.captures[0].RequestType)
		require.Equal(
			t,
			hashJSON(t, map[string]any{"type": "meta", "dex": ""}),
			recorder.captures[0].RequestPayloadHash,
		)
		require.Equal(
			t,
			map[string]string{"content-type": "application/json", "method": http.MethodPost},
			recorder.captures[0].RequestMetadata,
		)
		require.Equal(t, 200, recorder.captures[0].HTTPStatus)
		require.JSONEq(t, metaResponse, string(recorder.captures[0].ResponseBody))
		require.Equal(t, "instrument", recorder.captures[0].EntityHint)
		require.Nil(t, recorder.captures[0].Instrument)
		require.True(t, recorder.captures[0].RequestAt.Equal(recorder.captures[0].RequestAt.UTC()))
		require.True(t, recorder.captures[0].ResponseAt.Equal(recorder.captures[0].ResponseAt.UTC()))
		require.True(t, recorder.captures[0].ReceivedAt.Equal(recorder.captures[0].ReceivedAt.UTC()))
		require.False(t, recorder.captures[0].ResponseAt.Before(recorder.captures[0].RequestAt))
		require.False(t, recorder.captures[0].ReceivedAt.Before(recorder.captures[0].ResponseAt))

		require.Equal(t, hyperliquidInfoTypeCandle, recorder.captures[1].RequestType)
		require.Equal(
			t,
			hashJSON(t, map[string]any{
				"type": "candleSnapshot",
				"req": map[string]any{
					"coin":      "BTC",
					"interval":  "1m",
					"startTime": int64(1710000000000),
					"endTime":   int64(1710000180000),
				},
			}),
			recorder.captures[1].RequestPayloadHash,
		)
		require.Equal(t, "candle", recorder.captures[1].EntityHint)
		require.NotNil(t, recorder.captures[1].Instrument)
		require.Equal(t, makeInstrument(), *recorder.captures[1].Instrument)
		require.Equal(t, domain.Timeframe1m, recorder.captures[1].Timeframe)
		require.NotNil(t, recorder.captures[1].TimeRange)
		require.Equal(t, candleRequest.TimeRange, *recorder.captures[1].TimeRange)
		require.JSONEq(t, candleResponse, string(recorder.captures[1].ResponseBody))

		require.Equal(t, hyperliquidInfoTypeTrades, recorder.captures[2].RequestType)
		require.Equal(
			t,
			hashJSON(t, map[string]any{"type": "recentTrades", "coin": "BTC"}),
			recorder.captures[2].RequestPayloadHash,
		)
		require.Equal(t, "trade", recorder.captures[2].EntityHint)
		require.NotNil(t, recorder.captures[2].Instrument)
		require.Equal(t, makeInstrument(), *recorder.captures[2].Instrument)
		require.NotNil(t, recorder.captures[2].TimeRange)
		require.Equal(t, tradeRequest.TimeRange, *recorder.captures[2].TimeRange)
		require.Empty(t, recorder.captures[2].Timeframe)
		require.JSONEq(t, tradeResponse, string(recorder.captures[2].ResponseBody))
	})

	t.Run("captures error and malformed responses before returning errors", func(t *testing.T) {
		t.Parallel()

		statusRecorder := &stubHyperliquidRawEvidenceRecorder{ids: []string{"status-raw"}}
		statusAdapter := makeAdapter(t, adapterParams{
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			}),
			recorder: statusRecorder,
		})
		_, err := statusAdapter.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.Error(t, err)
		require.Contains(t, err.Error(), "http status 502")
		require.Len(t, statusRecorder.captures, 1)
		require.Equal(t, 502, statusRecorder.captures[0].HTTPStatus)
		require.Equal(t, hyperliquidInfoTypeMeta, statusRecorder.captures[0].RequestType)
		require.Empty(t, statusRecorder.captures[0].ResponseBody)

		malformedRecorder := &stubHyperliquidRawEvidenceRecorder{ids: []string{"malformed-raw"}}
		malformedAdapter := makeAdapter(t, adapterParams{
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `{`)
			}),
			recorder: malformedRecorder,
		})
		_, err = malformedAdapter.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.Error(t, err)
		require.Contains(t, err.Error(), "decode response body")
		require.Len(t, malformedRecorder.captures, 1)
		require.Equal(t, 200, malformedRecorder.captures[0].HTTPStatus)
		require.Equal(t, []byte(`{`), malformedRecorder.captures[0].ResponseBody)
	})

	t.Run("repeated fetches capture fresh raw payload ids without changing canonical results", func(t *testing.T) {
		t.Parallel()

		recorder := &stubHyperliquidRawEvidenceRecorder{ids: []string{"raw-one", "raw-two"}}
		adapter := makeAdapter(t, adapterParams{
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `{"universe":[{"name":"BTC","isDelisted":false}]}`)
			}),
			recorder: recorder,
		})

		first, err := adapter.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.NoError(t, err)
		second, err := adapter.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.NoError(t, err)

		require.Equal(t, []string{"raw-one"}, first.Metadata.RawPayloadIDs)
		require.Equal(t, []string{"raw-two"}, second.Metadata.RawPayloadIDs)
		require.NotEqual(t, first.Metadata.RawPayloadIDs[0], second.Metadata.RawPayloadIDs[0])
		require.Equal(t, first.Instruments, second.Instruments)
		require.Len(t, recorder.captures, 2)
		require.NotEqual(t, recorder.captures[0].ID, recorder.captures[1].ID)
	})

	t.Run("supports candle paging and rejects unsupported trade paging", func(t *testing.T) {
		t.Parallel()

		requestCount := 0
		adapter := makeAdapter(t, adapterParams{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++

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
			if payload.Type != hyperliquidInfoTypeCandle {
				t.Errorf("unexpected request type: %s", payload.Type)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if payload.Req.Coin != "BTC" {
				t.Errorf("unexpected coin: %s", payload.Req.Coin)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if payload.Req.Interval != "1m" {
				t.Errorf("unexpected interval: %s", payload.Req.Interval)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if payload.Req.StartTime == 1710000000000 {
				fmt.Fprint(w, `[
					{"t":1710000000000,"T":1710000059999,"s":"BTC","i":"1m","o":62000,"c":62010,"h":62020,"l":61990,"v":12.5,"n":42},
					{"t":1710000060000,"T":1710000119999,"s":"BTC","i":"1m","o":62010,"c":62025,"h":62040,"l":62005,"v":15.25,"n":37},
					{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75,"n":21}
				]`)
				return
			}

			if payload.Req.StartTime != 1710000120000 {
				t.Errorf("unexpected start time: %d", payload.Req.StartTime)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `[
				{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75,"n":21}
			]`)
		})})

		candleRequest := makeCandleRequest(2)
		firstPage, err := adapter.ReadCandles(t.Context(), candleRequest)
		require.NoError(t, err)
		require.Len(t, firstPage.Candles, 2)
		require.Equal(t, "startTime:1710000120000", firstPage.NextPageToken)

		secondPageRequest := candleRequest
		secondPageRequest.PageToken = firstPage.NextPageToken
		secondPage, err := adapter.ReadCandles(t.Context(), secondPageRequest)
		require.NoError(t, err)
		require.Len(t, secondPage.Candles, 1)
		require.Empty(t, secondPage.NextPageToken)

		tradeRequest := makeTradeRequest(2)
		tradeRequest.PageToken = "startTime:1710000120000"
		_, err = adapter.ReadTrades(t.Context(), tradeRequest)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
		require.Contains(
			t,
			err.Error(),
			"hyperliquid perps trade paging is unsupported in v0 because recentTrades does not advance deterministically by page token",
		)
		require.Equal(t, 2, requestCount)
	})

	t.Run("rejects unsupported historical trade ranges and truncation-prone page sizes", func(t *testing.T) {
		t.Parallel()

		adapter := makeAdapter(t, adapterParams{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
				t.Errorf("decode request body: %v", decodeErr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if payload[hyperliquidInfoTypeKey] != hyperliquidInfoTypeTrades {
				t.Errorf("unexpected request type: %v", payload[hyperliquidInfoTypeKey])
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			fmt.Fprint(w, `[
				{"coin":"BTC","side":"B","px":"62001.5","sz":"0.125","hash":"0xabc","time":1710000005000,"tid":101,"users":["0x1","0x2"]},
				{"coin":"BTC","side":"A","px":"62002.0","sz":"0.250","hash":"0xdef","time":1710000015000,"tid":102,"users":["0x3","0x4"]},
				{"coin":"BTC","side":"B","px":"62003.5","sz":"0.500","hash":"0xghi","time":1710000025000,"tid":103,"users":["0x5","0x6"]}
			]`)
		})})

		historicalRequest := makeTradeRequestWithRange(
			time.UnixMilli(1709999900000).UTC(),
			time.UnixMilli(1710000180000).UTC(),
			3,
		)
		_, err := adapter.ReadTrades(t.Context(), historicalRequest)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
		require.Contains(
			t,
			err.Error(),
			"hyperliquid perps trade time range is unsupported in v0 because recentTrades only exposes the latest venue window",
		)

		truncatingRequest := makeTradeRequest(2)
		_, err = adapter.ReadTrades(t.Context(), truncatingRequest)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
		require.Contains(
			t,
			err.Error(),
			"hyperliquid perps trade reads that require paging are unsupported in v0 because recentTrades cannot advance deterministically",
		)
	})

	t.Run("surfaces non-success statuses venue errors and malformed payloads", func(t *testing.T) {
		t.Parallel()

		statusAdapter := makeAdapter(t, adapterParams{
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			}),
		})
		_, err := statusAdapter.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.Error(t, err)
		require.Contains(t, err.Error(), "http status 502")

		venueErrorAdapter := makeAdapter(t, adapterParams{
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"Invalid type"}`)
			}),
		})
		_, err = venueErrorAdapter.ReadCandles(t.Context(), makeCandleRequest(2))
		require.Error(t, err)
		require.Contains(t, err.Error(), "hyperliquid perps error: Invalid type")

		malformedAdapter := makeAdapter(t, adapterParams{
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
					t.Errorf("decode request body: %v", decodeErr)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				switch payload[hyperliquidInfoTypeKey] {
				case hyperliquidInfoTypeMeta:
					fmt.Fprint(w, `{`)
				case hyperliquidInfoTypeCandle:
					fmt.Fprint(
						w,
						`[{"t":1710000000000,"T":1710000059999,"s":"ETH","i":"1m","o":62000,"c":62010,"h":62020,"l":61990,"v":12.5,"n":42}]`,
					)
				case hyperliquidInfoTypeTrades:
					fmt.Fprint(
						w,
						`[{"coin":"ETH","px":"62001.5","sz":"0.125","time":1710000005000,"tid":101}]`,
					)
				default:
					http.NotFound(w, r)
				}
			}),
		})

		_, err = malformedAdapter.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.Error(t, err)
		require.Contains(t, err.Error(), "decode response body")

		_, err = malformedAdapter.ReadCandles(t.Context(), makeCandleRequest(2))
		require.Error(t, err)
		require.Contains(t, err.Error(), "decode response body: candle symbol does not match request")

		_, err = malformedAdapter.ReadTrades(t.Context(), makeTradeRequest(2))
		require.Error(t, err)
		require.Contains(t, err.Error(), "decode response body: trade symbol does not match request")
	})

	t.Run("ingests mocked-http records through the data layer deterministically", func(t *testing.T) {
		t.Parallel()

		adapter := makeAdapter(t, adapterParams{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode request body: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			switch payload[hyperliquidInfoTypeKey] {
			case hyperliquidInfoTypeMeta:
				fmt.Fprint(w, `{
					"universe": [
						{"name": "BTC", "szDecimals": 5, "maxLeverage": 50},
						{"name": "ETH", "szDecimals": 4, "maxLeverage": 50, "isDelisted": true}
					]
				}`)
			case hyperliquidInfoTypeCandle:
				reqPayload, ok := payload["req"].(map[string]any)
				if !ok {
					t.Errorf("request payload missing req object: %#v", payload["req"])
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				startTime, ok := reqPayload["startTime"].(float64)
				if !ok {
					t.Errorf("request payload missing startTime: %#v", reqPayload["startTime"])
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				if int64(startTime) == 1710000000000 {
					fmt.Fprint(w, `[
						{"t":1710000000000,"T":1710000059999,"s":"BTC","i":"1m","o":62000,"c":62010,"h":62020,"l":61990,"v":12.5,"n":42},
						{"t":1710000060000,"T":1710000119999,"s":"BTC","i":"1m","o":62010,"c":62025,"h":62040,"l":62005,"v":15.25,"n":37},
						{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75,"n":21}
					]`)
					return
				}

				fmt.Fprint(w, `[
					{"t":1710000120000,"T":1710000179999,"s":"BTC","i":"1m","o":62025,"c":62030,"h":62035,"l":62015,"v":9.75,"n":21}
				]`)
			case hyperliquidInfoTypeTrades:
				fmt.Fprint(w, `[
					{"coin":"BTC","side":"B","px":"62001.5","sz":"0.125","hash":"0xabc","time":1710000005000,"tid":101,"users":["0x1","0x2"]},
					{"coin":"BTC","side":"A","px":"62002.0","sz":"0.250","hash":"0xdef","time":1710000015000,"tid":102,"users":["0x3","0x4"]},
					{"coin":"BTC","side":"B","px":"62003.5","sz":"0.500","hash":"0xghi","time":1710000025000,"tid":103,"users":["0x5","0x6"]}
				]`)
			default:
				http.NotFound(w, r)
			}
		})})

		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()
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
		flow, err := NewIngestionFlow(ingestionService)
		require.NoError(t, err)

		persistedInstruments, err := flow.IngestInstruments(t.Context(), adapter, makeInstrumentRequest())
		require.NoError(t, err)
		require.Len(t, persistedInstruments, 1)

		candleRequest := makeCandleRequest(2)
		persistedCandles, err := flow.IngestCandles(t.Context(), adapter, candleRequest)
		require.NoError(t, err)
		require.Len(t, persistedCandles, 3)

		tradeRequest := makeTradeRequest(3)
		persistedTrades, err := flow.IngestTrades(t.Context(), adapter, tradeRequest)
		require.NoError(t, err)
		require.Len(t, persistedTrades, 3)

		readCandles, err := readService.QueryCandles(
			t.Context(),
			makeInstrument(),
			domain.Timeframe1m,
			candleRequest.TimeRange,
		)
		require.NoError(t, err)
		require.Equal(t, persistedCandles, readCandles)

		readTrades, err := readService.QueryTrades(t.Context(), makeInstrument(), tradeRequest.TimeRange)
		require.NoError(t, err)
		require.Equal(t, persistedTrades, readTrades)
	})
}
