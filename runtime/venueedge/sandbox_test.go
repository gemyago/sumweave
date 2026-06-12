package venueedge

import (
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestSandboxVenue(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	makeVenue := func(seed int64) *SandboxVenue {
		t.Helper()

		venue, err := NewSandboxVenue(SandboxVenueParams{
			Seed:  seed,
			Venue: domain.Venue(" sandbox-test "),
			Instruments: []SandboxInstrumentParams{
				{
					Symbol:     domain.Symbol(" btcusd "),
					AssetClass: domain.AssetClass(" crypto "),
					Active:     true,
				},
				{
					Symbol:     domain.Symbol(" ethusd "),
					AssetClass: domain.AssetClass(" crypto "),
					Active:     true,
				},
			},
			SupportedTimeframes: []domain.Timeframe{
				domain.Timeframe1m,
				domain.Timeframe5m,
			},
			DefaultPageSize: 2,
		})
		require.NoError(t, err)

		return venue
	}

	makeInstrumentRequest := func() InstrumentReadRequest {
		request, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:    domain.Venue(" sandbox-test "),
			PageSize: 2,
		})
		require.NoError(t, err)

		return request
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
			time.FixedZone("sandbox", 2*3600),
		)
		timeRange, err := domain.NewTimeRange(start, start.Add(5*time.Minute))
		require.NoError(t, err)
		request, err := NewCandleReadRequest(CandleReadRequestParams{
			Instrument: domain.Instrument{
				Venue:      domain.Venue(" sandbox-test "),
				Symbol:     domain.Symbol(" btcusd "),
				AssetClass: domain.AssetClass(" crypto "),
				Active:     true,
			},
			Timeframe: domain.Timeframe(" 1m "),
			TimeRange: timeRange,
			PageSize:  2,
		})
		require.NoError(t, err)

		return request
	}

	makeTradeRequest := func() TradeReadRequest {
		start := time.Date(
			fake.IntBetween(2022, 2024),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			0,
			0,
			0,
			time.FixedZone("sandbox", -3*3600),
		)
		timeRange, err := domain.NewTimeRange(start, start.Add(2*time.Minute))
		require.NoError(t, err)
		request, err := NewTradeReadRequest(TradeReadRequestParams{
			Instrument: domain.Instrument{
				Venue:      domain.Venue(" sandbox-test "),
				Symbol:     domain.Symbol(" btcusd "),
				AssetClass: domain.AssetClass(" crypto "),
				Active:     true,
			},
			TimeRange: timeRange,
			PageSize:  2,
		})
		require.NoError(t, err)

		return request
	}

	t.Run("returns reproducible outputs for the same seed", func(t *testing.T) {
		t.Parallel()

		request := makeCandleRequest()
		firstVenue := makeVenue(42)
		secondVenue := makeVenue(42)

		firstCandles, err := firstVenue.ReadCandles(t.Context(), request)
		require.NoError(t, err)
		secondCandles, err := secondVenue.ReadCandles(t.Context(), request)
		require.NoError(t, err)

		require.Equal(t, firstCandles, secondCandles)

		tradeRequest := makeTradeRequest()
		firstTrades, err := firstVenue.ReadTrades(t.Context(), tradeRequest)
		require.NoError(t, err)
		secondTrades, err := secondVenue.ReadTrades(t.Context(), tradeRequest)
		require.NoError(t, err)

		require.Equal(t, firstTrades, secondTrades)
	})

	t.Run("changes outputs when the seed changes", func(t *testing.T) {
		t.Parallel()

		request := makeCandleRequest()
		firstCandles, err := makeVenue(1).ReadCandles(t.Context(), request)
		require.NoError(t, err)
		secondCandles, err := makeVenue(2).ReadCandles(t.Context(), request)
		require.NoError(t, err)

		require.NotEqual(t, firstCandles.Candles, secondCandles.Candles)
	})

	t.Run("respects half-open range boundaries and canonical timestamps", func(t *testing.T) {
		t.Parallel()

		request := makeTradeRequest()
		result, err := makeVenue(42).ReadTrades(t.Context(), request)
		require.NoError(t, err)
		require.NotEmpty(t, result.Trades)

		for _, trade := range result.Trades {
			require.False(t, trade.EventTime.Before(request.TimeRange.Start))
			require.True(t, trade.EventTime.Before(request.TimeRange.End))
			require.Equal(t, time.UTC, trade.EventTime.Location())
			require.GreaterOrEqual(t, trade.Price, 0.0)
			require.GreaterOrEqual(t, trade.Size, 0.0)
			require.NotEmpty(t, trade.Provenance.Source)
			require.NotEmpty(t, trade.Provenance.RecordID)
		}

		candleRequest := makeCandleRequest()
		candleResult, err := makeVenue(42).ReadCandles(t.Context(), candleRequest)
		require.NoError(t, err)
		require.NotEmpty(t, candleResult.Candles)
		for _, candle := range candleResult.Candles {
			require.False(t, candle.TimeRange.Start.Before(candleRequest.TimeRange.Start))
			require.True(t, candle.TimeRange.Start.Before(candleRequest.TimeRange.End))
			require.Equal(t, time.UTC, candle.TimeRange.Start.Location())
			require.Equal(t, time.UTC, candle.TimeRange.End.Location())
			require.GreaterOrEqual(t, candle.Low, 0.0)
			require.GreaterOrEqual(t, candle.Volume, 0.0)
			require.NotEmpty(t, candle.Provenance.RecordID)
		}
	})

	t.Run("supports deterministic paging boundaries", func(t *testing.T) {
		t.Parallel()

		venue := makeVenue(42)
		instrumentPage, err := venue.ReadInstruments(t.Context(), makeInstrumentRequest())
		require.NoError(t, err)
		require.Len(t, instrumentPage.Instruments, 2)
		require.Empty(t, instrumentPage.NextPageToken)

		candleRequest := makeCandleRequest()
		firstPage, err := venue.ReadCandles(t.Context(), candleRequest)
		require.NoError(t, err)
		require.Len(t, firstPage.Candles, 2)
		require.Equal(t, "2", firstPage.NextPageToken)

		secondPageRequest := candleRequest
		secondPageRequest.PageToken = firstPage.NextPageToken
		secondPage, err := venue.ReadCandles(t.Context(), secondPageRequest)
		require.NoError(t, err)
		require.Len(t, secondPage.Candles, 2)
		require.Equal(t, "4", secondPage.NextPageToken)

		thirdPageRequest := candleRequest
		thirdPageRequest.PageToken = secondPage.NextPageToken
		thirdPage, err := venue.ReadCandles(t.Context(), thirdPageRequest)
		require.NoError(t, err)
		require.Len(t, thirdPage.Candles, 1)
		require.Empty(t, thirdPage.NextPageToken)
	})

	t.Run("rejects invalid requests and unsupported config", func(t *testing.T) {
		t.Parallel()

		_, err := NewSandboxVenue(SandboxVenueParams{
			Seed:                1,
			Venue:               domain.Venue("sandbox-test"),
			Instruments:         nil,
			SupportedTimeframes: []domain.Timeframe{domain.Timeframe1m},
		})
		require.Error(t, err)

		venue := makeVenue(42)

		invalidInstrumentRequest, err := NewCandleReadRequest(CandleReadRequestParams{
			Instrument: domain.Instrument{
				Venue:      domain.Venue(" sandbox-test "),
				Symbol:     domain.Symbol(" missing "),
				AssetClass: domain.AssetClass(" crypto "),
				Active:     true,
			},
			Timeframe: domain.Timeframe(" 1m "),
			TimeRange: makeCandleRequest().TimeRange,
		})
		require.NoError(t, err)
		_, err = venue.ReadCandles(t.Context(), invalidInstrumentRequest)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)

		invalidPageRequest := makeTradeRequest()
		invalidPageRequest.PageToken = "bad-token"
		_, err = venue.ReadTrades(t.Context(), invalidPageRequest)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
	})
}
