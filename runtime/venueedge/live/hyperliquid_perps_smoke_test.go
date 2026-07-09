//go:build live

package live_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/stretchr/testify/require"
)

const (
	hyperliquidPublicBaseURL      = "https://api.hyperliquid.xyz"
	liveSmokeSymbol               = domain.Symbol("BTC")
	liveSmokeTimeframe            = domain.Timeframe1m
	liveSmokeCandleBuckets        = 5
	liveSmokeTradeProbeTargetRows = 10
	liveSmokeTradeProbeAttempts   = 5
	liveSmokeTradeRangePadding    = 2 * time.Second
	liveSmokeTimeout              = 30 * time.Second
)

type recentTradeProbeRow struct {
	Time int64 `json:"time"`
}

type recentTradeProbeWindow struct {
	TimeRange         domain.TimeRange
	AvailableCount    int
	SelectedCount     int
	EarliestAvailable time.Time
	LatestAvailable   time.Time
}

func selectRecentTradeProbeWindow(rows []recentTradeProbeRow) (recentTradeProbeWindow, error) {
	if len(rows) == 0 {
		return recentTradeProbeWindow{}, errors.New("recentTrades probe returned no rows")
	}

	sortedRows := append([]recentTradeProbeRow(nil), rows...)
	sort.Slice(sortedRows, func(i, j int) bool {
		return sortedRows[i].Time < sortedRows[j].Time
	})

	startIdx := 0
	if len(sortedRows) > liveSmokeTradeProbeTargetRows {
		startIdx = len(sortedRows) - liveSmokeTradeProbeTargetRows
	}

	earliestAvailable := time.UnixMilli(sortedRows[0].Time).UTC()
	latestAvailable := time.UnixMilli(sortedRows[len(sortedRows)-1].Time).UTC()
	timeRange, err := domain.NewTimeRange(
		time.UnixMilli(sortedRows[startIdx].Time).UTC(),
		latestAvailable.Add(liveSmokeTradeRangePadding),
	)
	if err != nil {
		return recentTradeProbeWindow{}, fmt.Errorf("build recent trade probe window: %w", err)
	}

	return recentTradeProbeWindow{
		TimeRange:         timeRange,
		AvailableCount:    len(sortedRows),
		SelectedCount:     len(sortedRows) - startIdx,
		EarliestAvailable: earliestAvailable,
		LatestAvailable:   latestAvailable,
	}, nil
}

func describeRecentTradeProbeWindow(window recentTradeProbeWindow) string {
	return fmt.Sprintf(
		"selected %d of %d recentTrades rows in [%s, %s) from available venue window [%s, %s]",
		window.SelectedCount,
		window.AvailableCount,
		window.TimeRange.Start.Format(time.RFC3339Nano),
		window.TimeRange.End.Format(time.RFC3339Nano),
		window.EarliestAvailable.Format(time.RFC3339Nano),
		window.LatestAvailable.Format(time.RFC3339Nano),
	)
}

func TestHyperliquidPerpsLiveSmoke(t *testing.T) {
	t.Parallel()

	makeHTTPClient := func() *http.Client {
		return &http.Client{Timeout: 10 * time.Second}
	}

	makeStore := func(t *testing.T) *data.DatabaseStore {
		t.Helper()

		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := data.NewDatabaseStore(sqlDB, ":memory:", data.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeVenue := func(t *testing.T, client *http.Client) *venueedge.HyperliquidPerpsVenue {
		t.Helper()

		venue, err := venueedge.NewHyperliquidPerpsVenue(venueedge.HyperliquidPerpsVenueParams{
			BaseURL:    hyperliquidPublicBaseURL,
			HTTPClient: client,
		})
		require.NoError(t, err)

		return venue
	}

	makeServices := func(t *testing.T, store *data.DatabaseStore) (*venueedge.IngestionFlow, *data.ReadService) {
		t.Helper()

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

		flow, err := venueedge.NewIngestionFlow(ingestionService)
		require.NoError(t, err)

		return flow, readService
	}

	makeClosedCandleRange := func(now time.Time) domain.TimeRange {
		t.Helper()

		end := now.UTC().Truncate(time.Minute)
		start := end.Add(-liveSmokeCandleBuckets * time.Minute)
		timeRange, err := domain.NewTimeRange(start, end)
		require.NoError(t, err)

		return timeRange
	}

	probeRecentTradeWindow := func(client *http.Client) recentTradeProbeWindow {
		t.Helper()

		requestBody, err := json.Marshal(map[string]any{
			"type": "recentTrades",
			"coin": liveSmokeSymbol.String(),
		})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), liveSmokeTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			hyperliquidPublicBaseURL+"/info",
			bytes.NewReader(requestBody),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(
			t,
			err,
			"live smoke could not probe Hyperliquid recentTrades; rerun if the public venue is transiently unavailable",
		)
		defer resp.Body.Close()

		require.Equal(
			t,
			http.StatusOK,
			resp.StatusCode,
			"live smoke probe expected Hyperliquid public recentTrades to stay read-only and available",
		)

		var rows []recentTradeProbeRow
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&rows))
		require.NotEmpty(
			t,
			rows,
			"live smoke probe expected recentTrades to return a non-empty public BTC window",
		)

		window, err := selectRecentTradeProbeWindow(rows)
		require.NoError(t, err)

		return window
	}

	client := makeHTTPClient()
	venue := makeVenue(t, client)
	store := makeStore(t)
	flow, readService := makeServices(t, store)

	instrumentRequest, err := venueedge.NewInstrumentReadRequest(venueedge.InstrumentReadRequestParams{
		Venue:    venueedge.HyperliquidPerpsVenueName,
		Symbols:  []domain.Symbol{liveSmokeSymbol},
		PageSize: 10,
	})
	require.NoError(t, err)

	persistedInstruments, err := flow.IngestInstruments(t.Context(), venue, instrumentRequest)
	require.NoError(
		t,
		err,
		"live smoke instrument ingestion failed; rerun if Hyperliquid public metadata is transiently unstable, otherwise inspect the adapter or SQLite path",
	)
	require.Len(t, persistedInstruments, 1)
	require.Equal(t, venueedge.HyperliquidPerpsVenueName, persistedInstruments[0].Venue)
	require.Equal(t, liveSmokeSymbol, persistedInstruments[0].Symbol)
	require.Equal(t, domain.AssetClassFuture, persistedInstruments[0].AssetClass)
	require.True(t, persistedInstruments[0].Active)

	lookedUpInstrument, err := readService.LookupInstrument(
		t.Context(),
		venueedge.HyperliquidPerpsVenueName,
		liveSmokeSymbol,
	)
	require.NoError(
		t,
		err,
		"live smoke instrument readback failed after SQLite persistence; this points to runtime persistence rather than venue volatility",
	)
	require.NotNil(t, lookedUpInstrument)
	require.Equal(t, persistedInstruments[0], *lookedUpInstrument)

	candleRange := makeClosedCandleRange(time.Now().UTC())
	candleRequest, err := venueedge.NewCandleReadRequest(venueedge.CandleReadRequestParams{
		Instrument: *lookedUpInstrument,
		Timeframe:  liveSmokeTimeframe,
		TimeRange:  candleRange,
		PageSize:   liveSmokeCandleBuckets + 2,
	})
	require.NoError(t, err)

	persistedCandles, err := flow.IngestCandles(t.Context(), venue, candleRequest)
	require.NoError(
		t,
		err,
		"live smoke candle ingestion failed; rerun if Hyperliquid public candle reads are transiently unstable, otherwise inspect canonical mapping or SQLite persistence",
	)
	require.NotEmpty(t, persistedCandles)

	readCandles, err := readService.QueryCandles(
		t.Context(),
		*lookedUpInstrument,
		liveSmokeTimeframe,
		candleRange,
	)
	require.NoError(
		t,
		err,
		"live smoke candle readback failed after SQLite persistence; this points to runtime query or canonicalization regressions",
	)
	require.Equal(t, persistedCandles, readCandles)
	require.Len(
		t,
		readCandles,
		liveSmokeCandleBuckets,
		"live smoke expected one fully closed 1m candle per requested bucket; fewer rows point to venue incompleteness or runtime persistence drift",
	)

	replayCandles, err := readService.ReplayCandles(
		t.Context(),
		*lookedUpInstrument,
		liveSmokeTimeframe,
		candleRange,
	)
	require.NoError(
		t,
		err,
		"live smoke candle replay failed after SQLite persistence; this points to runtime replay regressions rather than venue volatility",
	)
	require.Len(t, replayCandles, len(readCandles))

	for idx, candle := range readCandles {
		expectedStart := candleRange.Start.Add(time.Duration(idx) * time.Minute)
		expectedEnd := expectedStart.Add(time.Minute)

		require.Equal(t, *lookedUpInstrument, candle.Instrument)
		require.Equal(t, liveSmokeTimeframe, candle.Timeframe)
		require.Equal(t, expectedStart, candle.TimeRange.Start)
		require.Equal(t, expectedEnd, candle.TimeRange.End)
		require.Equal(t, time.UTC, candle.TimeRange.Start.Location())
		require.Equal(t, time.UTC, candle.TimeRange.End.Location())
		require.Equal(t, domain.DataQualityRaw, candle.Quality)
		require.Equal(t, "hyperliquid-perps-rest", candle.Provenance.Source)
		require.NotEmpty(t, candle.Provenance.RecordID)
		require.GreaterOrEqual(t, candle.Open, 0.0)
		require.GreaterOrEqual(t, candle.High, 0.0)
		require.GreaterOrEqual(t, candle.Low, 0.0)
		require.GreaterOrEqual(t, candle.Close, 0.0)
		require.GreaterOrEqual(t, candle.Volume, 0.0)
		require.Equal(t, candle, replayCandles[idx].Candle)
		if idx > 0 {
			require.Greater(t, replayCandles[idx].Identity, replayCandles[idx-1].Identity)
		}
	}

	var (
		persistedTrades []domain.Trade
		tradeWindow     recentTradeProbeWindow
		tradeAttempts   int
	)
	for attempt := 0; attempt < liveSmokeTradeProbeAttempts; attempt++ {
		tradeAttempts = attempt + 1
		tradeWindow = probeRecentTradeWindow(client)
		tradeRequest, requestErr := venueedge.NewTradeReadRequest(venueedge.TradeReadRequestParams{
			Instrument: *lookedUpInstrument,
			TimeRange:  tradeWindow.TimeRange,
			PageSize:   50,
		})
		require.NoError(t, requestErr)

		persistedTrades, err = flow.IngestTrades(t.Context(), venue, tradeRequest)
		if err == nil {
			break
		}
		if errors.Is(err, venueedge.ErrValidation) &&
			strings.Contains(err.Error(), "recentTrades only exposes the latest venue window") {
			continue
		}
		break
	}
	require.NoError(
		t,
		err,
		"live smoke trade ingestion failed after %d attempts using %s; rerun if Hyperliquid recentTrades shifted faster than the probe/ingest loop, otherwise inspect canonical mapping or SQLite persistence",
		tradeAttempts,
		describeRecentTradeProbeWindow(tradeWindow),
	)
	require.NotEmpty(t, persistedTrades)

	readTrades, err := readService.QueryTrades(t.Context(), *lookedUpInstrument, tradeWindow.TimeRange)
	require.NoError(
		t,
		err,
		"live smoke trade readback failed after SQLite persistence; this points to runtime query or canonicalization regressions",
	)
	require.Equal(t, persistedTrades, readTrades)

	replayTrades, err := readService.ReplayTrades(t.Context(), *lookedUpInstrument, tradeWindow.TimeRange)
	require.NoError(
		t,
		err,
		"live smoke trade replay failed after SQLite persistence; this points to runtime replay regressions rather than venue volatility",
	)
	require.Len(t, replayTrades, len(readTrades))

	for idx, trade := range readTrades {
		require.Equal(t, *lookedUpInstrument, trade.Instrument)
		require.False(t, trade.EventTime.Before(tradeWindow.TimeRange.Start))
		require.True(t, trade.EventTime.Before(tradeWindow.TimeRange.End))
		require.Equal(t, time.UTC, trade.EventTime.Location())
		require.Equal(t, domain.DataQualityRaw, trade.Quality)
		require.Equal(t, "hyperliquid-perps-rest", trade.Provenance.Source)
		require.NotEmpty(t, trade.Provenance.RecordID)
		require.GreaterOrEqual(t, trade.Price, 0.0)
		require.GreaterOrEqual(t, trade.Size, 0.0)
		require.Equal(t, trade, replayTrades[idx].Trade)
		if idx > 0 {
			require.False(t, trade.EventTime.Before(readTrades[idx-1].EventTime))
			require.Greater(t, replayTrades[idx].Identity, replayTrades[idx-1].Identity)
		}
	}

	t.Logf(
		"live smoke ingested %d instrument, %d candles, and %d trades for %s via Hyperliquid -> ingestion flow -> SQLite -> canonical read services (%s after %d attempt(s))",
		len(persistedInstruments),
		len(persistedCandles),
		len(persistedTrades),
		liveSmokeSymbol,
		describeRecentTradeProbeWindow(tradeWindow),
		tradeAttempts,
	)
}
