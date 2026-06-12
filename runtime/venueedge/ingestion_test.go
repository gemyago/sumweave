package venueedge

import (
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestIngestionFlow(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	makeStore := func(t *testing.T) *data.DatabaseStore {
		t.Helper()

		store, err := data.NewDatabaseStore(":memory:", data.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeSandboxVenue := func(t *testing.T, seed int64) *SandboxVenue {
		t.Helper()

		venue, err := NewSandboxVenue(SandboxVenueParams{
			Seed:  seed,
			Venue: domain.Venue(" sandbox-int "),
			Instruments: []SandboxInstrumentParams{
				{
					Symbol:     domain.Symbol(" btcusd "),
					AssetClass: domain.AssetClass(" crypto "),
					Active:     true,
				},
			},
			SupportedTimeframes: []domain.Timeframe{
				domain.Timeframe1m,
			},
			DefaultPageSize: 2,
		})
		require.NoError(t, err)

		return venue
	}

	makeInstrument := func() domain.Instrument {
		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue("sandbox-int"),
			Symbol:     domain.Symbol("btcusd"),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeCandleRequest := func() CandleReadRequest {
		start := time.Date(
			fake.IntBetween(2022, 2024),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			0,
			0,
			0,
			time.FixedZone("sandbox", 4*3600),
		)
		timeRange, err := domain.NewTimeRange(start, start.Add(5*time.Minute))
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

	t.Run("ingests sandbox records into sqlite and replays them deterministically", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t)
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

		venue := makeSandboxVenue(t, 42)
		instrumentRequest, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:    domain.Venue(" sandbox-int "),
			PageSize: 2,
		})
		require.NoError(t, err)
		candleRequest := makeCandleRequest()
		tradeRequest := makeTradeRequest(candleRequest)

		persistedInstruments, err := flow.IngestInstruments(t.Context(), venue, instrumentRequest)
		require.NoError(t, err)
		require.Len(t, persistedInstruments, 1)

		persistedCandles, err := flow.IngestCandles(t.Context(), venue, candleRequest)
		require.NoError(t, err)
		require.Len(t, persistedCandles, 5)

		persistedTrades, err := flow.IngestTrades(t.Context(), venue, tradeRequest)
		require.NoError(t, err)
		require.NotEmpty(t, persistedTrades)

		lookedUpInstrument, err := readService.LookupInstrument(
			t.Context(),
			candleRequest.Instrument.Venue,
			candleRequest.Instrument.Symbol,
		)
		require.NoError(t, err)
		require.NotNil(t, lookedUpInstrument)

		readCandles, err := readService.QueryCandles(
			t.Context(),
			*lookedUpInstrument,
			candleRequest.Timeframe,
			candleRequest.TimeRange,
		)
		require.NoError(t, err)
		require.Equal(t, persistedCandles, readCandles)

		replayCandles, err := readService.ReplayCandles(
			t.Context(),
			*lookedUpInstrument,
			candleRequest.Timeframe,
			candleRequest.TimeRange,
		)
		require.NoError(t, err)
		require.Len(t, replayCandles, len(readCandles))

		readTrades, err := readService.QueryTrades(
			t.Context(),
			*lookedUpInstrument,
			tradeRequest.TimeRange,
		)
		require.NoError(t, err)
		require.Equal(t, persistedTrades, readTrades)

		replayTrades, err := readService.ReplayTrades(
			t.Context(),
			*lookedUpInstrument,
			tradeRequest.TimeRange,
		)
		require.NoError(t, err)
		require.Len(t, replayTrades, len(readTrades))

		for idx, candle := range readCandles {
			require.False(t, candle.TimeRange.Start.Before(candleRequest.TimeRange.Start))
			require.True(t, candle.TimeRange.Start.Before(candleRequest.TimeRange.End))
			require.Equal(t, time.UTC, candle.TimeRange.Start.Location())
			require.Equal(t, time.UTC, candle.TimeRange.End.Location())
			require.Equal(t, domain.DataQualityValidated, candle.Quality)
			require.True(t, strings.HasPrefix(candle.Provenance.Source, "sandbox-int-sandbox"))
			if idx > 0 {
				require.False(t, candle.TimeRange.Start.Before(readCandles[idx-1].TimeRange.Start))
				require.Greater(t, replayCandles[idx].Identity, replayCandles[idx-1].Identity)
			}
		}

		for idx, trade := range readTrades {
			require.False(t, trade.EventTime.Before(tradeRequest.TimeRange.Start))
			require.True(t, trade.EventTime.Before(tradeRequest.TimeRange.End))
			require.Equal(t, time.UTC, trade.EventTime.Location())
			require.Equal(t, domain.DataQualityValidated, trade.Quality)
			require.True(t, strings.HasPrefix(trade.Provenance.Source, "sandbox-int-sandbox"))
			if idx > 0 {
				require.False(t, trade.EventTime.Before(readTrades[idx-1].EventTime))
				require.Greater(t, replayTrades[idx].Identity, replayTrades[idx-1].Identity)
			}
		}
	})

	t.Run("repeated sandbox ingestion stays idempotent", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t)
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

		venue := makeSandboxVenue(t, 77)
		candleRequest := makeCandleRequest()
		tradeRequest := makeTradeRequest(candleRequest)

		_, err = flow.IngestCandles(t.Context(), venue, candleRequest)
		require.NoError(t, err)
		_, err = flow.IngestTrades(t.Context(), venue, tradeRequest)
		require.NoError(t, err)

		firstCandles, err := readService.QueryCandles(
			t.Context(),
			candleRequest.Instrument,
			candleRequest.Timeframe,
			candleRequest.TimeRange,
		)
		require.NoError(t, err)
		firstTrades, err := readService.QueryTrades(t.Context(), tradeRequest.Instrument, tradeRequest.TimeRange)
		require.NoError(t, err)

		_, err = flow.IngestCandles(t.Context(), venue, candleRequest)
		require.NoError(t, err)
		_, err = flow.IngestTrades(t.Context(), venue, tradeRequest)
		require.NoError(t, err)

		secondCandles, err := readService.QueryCandles(
			t.Context(),
			candleRequest.Instrument,
			candleRequest.Timeframe,
			candleRequest.TimeRange,
		)
		require.NoError(t, err)
		secondTrades, err := readService.QueryTrades(t.Context(), tradeRequest.Instrument, tradeRequest.TimeRange)
		require.NoError(t, err)

		require.Equal(t, firstCandles, secondCandles)
		require.Equal(t, firstTrades, secondTrades)
	})
}
