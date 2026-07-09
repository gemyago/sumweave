package venueedge

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/stretchr/testify/require"
)

func TestBinanceSpotVenue(t *testing.T) {
	t.Parallel()

	makeAdapter := func(t *testing.T, handler http.Handler) *BinanceSpotVenue {
		t.Helper()

		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)

		adapter, err := NewBinanceSpotVenue(BinanceSpotVenueParams{
			BaseURL:    server.URL,
			HTTPClient: server.Client(),
		})
		require.NoError(t, err)

		return adapter
	}

	makeInstrument := func() domain.Instrument {
		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      BinanceSpotVenueName,
			Symbol:     domain.Symbol("BTCUSDT"),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeCandleRequest := func() CandleReadRequest {
		timeRange, err := domain.NewTimeRange(
			time.UnixMilli(1710000000000).UTC(),
			time.UnixMilli(1710000180000).UTC(),
		)
		require.NoError(t, err)
		request, err := NewCandleReadRequest(CandleReadRequestParams{
			Instrument: makeInstrument(),
			Timeframe:  domain.Timeframe1m,
			TimeRange:  timeRange,
			PageSize:   2,
		})
		require.NoError(t, err)

		return request
	}

	makeTradeRequest := func(candleRequest CandleReadRequest) TradeReadRequest {
		request, err := NewTradeReadRequest(TradeReadRequestParams{
			Instrument: candleRequest.Instrument,
			TimeRange:  candleRequest.TimeRange,
			PageSize:   2,
		})
		require.NoError(t, err)

		return request
	}

	t.Run("maps documented success responses into canonical records", func(t *testing.T) {
		t.Parallel()

		requests := make([]string, 0)
		adapter := makeAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.String())

			switch r.URL.Path {
			case "/api/v3/exchangeInfo":
				fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","status":"TRADING"}]}`)
			case "/api/v3/klines":
				fmt.Fprint(w, `[
					[1710000000000,"100.0","101.0","99.5","100.5","12.0",1710000059999,"0",0,"0","0","0"],
					[1710000060000,"100.5","102.0","100.0","101.5","15.0",1710000119999,"0",0,"0","0","0"]
				]`)
			case "/api/v3/aggTrades":
				fmt.Fprint(w, `[
					{"a":101,"p":"100.1","q":"0.5","T":1710000005000},
					{"a":102,"p":"100.2","q":"0.7","T":1710000015000}
				]`)
			default:
				http.NotFound(w, r)
			}
		}))

		instrumentRequest, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:   BinanceSpotVenueName,
			Symbols: []domain.Symbol{domain.Symbol("BTCUSDT")},
		})
		require.NoError(t, err)
		instruments, err := adapter.ReadInstruments(t.Context(), instrumentRequest)
		require.NoError(t, err)
		require.Len(t, instruments.Instruments, 1)
		require.Equal(t, BinanceSpotVenueName, instruments.Instruments[0].Venue)

		candleRequest := makeCandleRequest()
		candles, err := adapter.ReadCandles(t.Context(), candleRequest)
		require.NoError(t, err)
		require.Len(t, candles.Candles, 2)
		require.Equal(t, domain.DataQualityRaw, candles.Candles[0].Quality)
		require.Equal(t, time.UnixMilli(1710000000000).UTC(), candles.Candles[0].TimeRange.Start)

		trades, err := adapter.ReadTrades(t.Context(), makeTradeRequest(candleRequest))
		require.NoError(t, err)
		require.Len(t, trades.Trades, 2)
		require.Equal(t, domain.DataQualityRaw, trades.Trades[0].Quality)
		require.Equal(t, time.UnixMilli(1710000005000).UTC(), trades.Trades[0].EventTime)

		require.Contains(t, requests[0], "/api/v3/exchangeInfo")
		require.Contains(t, requests[1], "interval=1m")
		require.Contains(t, requests[2], "/api/v3/aggTrades")
	})

	t.Run("supports adapter paging tokens for candles and trades", func(t *testing.T) {
		t.Parallel()

		adapter := makeAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v3/klines":
				if r.URL.Query().Get("startTime") == "1710000000000" {
					fmt.Fprint(w, `[
						[1710000000000,"100.0","101.0","99.5","100.5","12.0",1710000059999,"0",0,"0","0","0"],
						[1710000060000,"100.5","102.0","100.0","101.5","15.0",1710000119999,"0",0,"0","0","0"]
					]`)
					return
				}
				fmt.Fprint(w, `[
					[1710000120000,"101.5","103.0","101.0","102.0","20.0",1710000179999,"0",0,"0","0","0"]
				]`)
			case "/api/v3/aggTrades":
				if r.URL.Query().Get("fromId") == "" {
					fmt.Fprint(w, `[
						{"a":101,"p":"100.1","q":"0.5","T":1710000005000},
						{"a":102,"p":"100.2","q":"0.7","T":1710000015000}
					]`)
					return
				}
				fmt.Fprint(w, `[
					{"a":103,"p":"100.3","q":"0.2","T":1710000025000}
				]`)
			default:
				http.NotFound(w, r)
			}
		}))

		candleRequest := makeCandleRequest()
		firstCandlePage, err := adapter.ReadCandles(t.Context(), candleRequest)
		require.NoError(t, err)
		require.Equal(t, "start:1710000120000", firstCandlePage.NextPageToken)

		secondCandleRequest := candleRequest
		secondCandleRequest.PageToken = firstCandlePage.NextPageToken
		secondCandlePage, err := adapter.ReadCandles(t.Context(), secondCandleRequest)
		require.NoError(t, err)
		require.Len(t, secondCandlePage.Candles, 1)
		require.Empty(t, secondCandlePage.NextPageToken)

		tradeRequest := makeTradeRequest(candleRequest)
		firstTradePage, err := adapter.ReadTrades(t.Context(), tradeRequest)
		require.NoError(t, err)
		require.Equal(t, "fromId:103", firstTradePage.NextPageToken)

		secondTradeRequest := tradeRequest
		secondTradeRequest.PageToken = firstTradePage.NextPageToken
		secondTradePage, err := adapter.ReadTrades(t.Context(), secondTradeRequest)
		require.NoError(t, err)
		require.Len(t, secondTradePage.Trades, 1)
	})

	t.Run("surfaces non-success statuses venue errors and malformed payloads", func(t *testing.T) {
		t.Parallel()

		candleRequest := makeCandleRequest()

		statusAdapter := makeAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		_, err := statusAdapter.ReadCandles(t.Context(), candleRequest)
		require.Error(t, err)
		require.Contains(t, err.Error(), "http status 502")

		venueErrorAdapter := makeAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"code":-1121,"msg":"Invalid symbol."}`)
		}))
		_, err = venueErrorAdapter.ReadCandles(t.Context(), candleRequest)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Invalid symbol.")

		malformedAdapter := makeAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{`)
		}))
		_, err = malformedAdapter.ReadCandles(t.Context(), candleRequest)
		require.Error(t, err)
		require.Contains(t, err.Error(), "decode response body")
	})

	t.Run("ingests mocked-http records through the data layer deterministically", func(t *testing.T) {
		t.Parallel()

		adapter := makeAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v3/exchangeInfo":
				fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","status":"TRADING"}]}`)
			case "/api/v3/klines":
				if r.URL.Query().Get("startTime") == "1710000000000" {
					fmt.Fprint(w, `[
						[1710000000000,"100.0","101.0","99.5","100.5","12.0",1710000059999,"0",0,"0","0","0"],
						[1710000060000,"100.5","102.0","100.0","101.5","15.0",1710000119999,"0",0,"0","0","0"]
					]`)
					return
				}
				fmt.Fprint(w, `[
					[1710000120000,"101.5","103.0","101.0","102.0","20.0",1710000179999,"0",0,"0","0","0"]
				]`)
			case "/api/v3/aggTrades":
				if r.URL.Query().Get("fromId") == "" {
					fmt.Fprint(w, `[
						{"a":101,"p":"100.1","q":"0.5","T":1710000005000},
						{"a":102,"p":"100.2","q":"0.7","T":1710000015000}
					]`)
					return
				}
				fmt.Fprint(w, `[
					{"a":103,"p":"100.3","q":"0.2","T":1710000025000}
				]`)
			default:
				http.NotFound(w, r)
			}
		}))

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

		instrumentRequest, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:   BinanceSpotVenueName,
			Symbols: []domain.Symbol{domain.Symbol("BTCUSDT")},
		})
		require.NoError(t, err)
		_, err = flow.IngestInstruments(t.Context(), adapter, instrumentRequest)
		require.NoError(t, err)

		candleRequest := makeCandleRequest()
		persistedCandles, err := flow.IngestCandles(t.Context(), adapter, candleRequest)
		require.NoError(t, err)
		require.Len(t, persistedCandles, 3)

		tradeRequest := makeTradeRequest(candleRequest)
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
