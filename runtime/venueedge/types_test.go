package venueedge

import (
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestVenueEdgeTypes(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	randomLocationTime := func() time.Time {
		zone := time.FixedZone(randomWord("zone"), fake.IntBetween(-11, 12)*3600)
		return time.Date(
			fake.IntBetween(2020, 2032),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			zone,
		)
	}

	makeInstrument := func() domain.Instrument {
		return domain.Instrument{
			Venue:      domain.Venue("  " + randomWord("venue") + "  "),
			Symbol:     domain.Symbol("  " + strings.ToUpper(randomWord("symbol")) + "  "),
			AssetClass: domain.AssetClass("  CRYPTO  "),
			Active:     fake.Bool(),
		}
	}

	makeTimeRange := func() domain.TimeRange {
		start := randomLocationTime()
		return domain.TimeRange{
			Start: start,
			End:   start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute),
		}
	}

	makeCandle := func() domain.Candle {
		return domain.Candle{
			Instrument: makeInstrument(),
			Timeframe:  domain.Timeframe(" 1M "),
			TimeRange:  makeTimeRange(),
			Open:       fake.Float64(2, 1, 1000),
			High:       fake.Float64(2, 1001, 2000),
			Low:        fake.Float64(2, 0, 999),
			Close:      fake.Float64(2, 1, 1000),
			Volume:     fake.Float64(4, 0, 10000),
			Quality:    domain.DataQuality("  VALIDATED "),
			Provenance: domain.SourceProvenance{
				Source:   "  " + randomWord("source") + "  ",
				RecordID: "  " + randomWord("record") + "  ",
			},
		}
	}

	makeTrade := func() domain.Trade {
		return domain.Trade{
			Instrument: makeInstrument(),
			EventTime:  randomLocationTime(),
			Price:      fake.Float64(4, 1, 100000),
			Size:       fake.Float64(4, 0, 100000),
			Quality:    domain.DataQuality("  RAW "),
			Provenance: domain.SourceProvenance{
				Source:   "  " + randomWord("source") + "  ",
				RecordID: "  " + randomWord("record") + "  ",
			},
		}
	}

	t.Run("request constructors canonicalize inputs and ranges", func(t *testing.T) {
		t.Parallel()

		instrumentRequest, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:     domain.Venue("  " + randomWord("venue") + "  "),
			Symbols:   []domain.Symbol{domain.Symbol("  " + strings.ToUpper(randomWord("symbol")) + "  ")},
			PageSize:  fake.IntBetween(1, 50),
			PageToken: "  " + randomWord("page") + "  ",
		})
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(instrumentRequest.Venue.String()), instrumentRequest.Venue.String())
		require.Equal(
			t,
			strings.TrimSpace(instrumentRequest.Symbols[0].String()),
			instrumentRequest.Symbols[0].String(),
		)
		require.Equal(t, strings.TrimSpace(instrumentRequest.PageToken), instrumentRequest.PageToken)

		candleRange := makeTimeRange()
		candleRequest, err := NewCandleReadRequest(CandleReadRequestParams{
			Instrument: makeInstrument(),
			Timeframe:  domain.Timeframe(" 1M "),
			TimeRange:  candleRange,
			PageSize:   fake.IntBetween(1, 50),
			PageToken:  "  " + randomWord("page") + "  ",
		})
		require.NoError(t, err)
		require.Equal(t, candleRange, candleRequest.TimeRange)
		require.Equal(t, "1m", candleRequest.Timeframe.String())

		tradeRange := makeTimeRange()
		tradeRequest, err := NewTradeReadRequest(TradeReadRequestParams{
			Instrument: makeInstrument(),
			TimeRange:  tradeRange,
			PageSize:   fake.IntBetween(1, 50),
			PageToken:  "  " + randomWord("page") + "  ",
		})
		require.NoError(t, err)
		require.Equal(t, tradeRange, tradeRequest.TimeRange)
	})

	t.Run("request constructors reject invalid canonical inputs", func(t *testing.T) {
		t.Parallel()

		_, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:    "",
			PageSize: fake.IntBetween(1, 50),
		})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)

		_, err = NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:    domain.Venue(randomWord("venue")),
			Symbols:  []domain.Symbol{""},
			PageSize: fake.IntBetween(1, 50),
		})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)

		_, err = NewCandleReadRequest(CandleReadRequestParams{
			Instrument: makeInstrument(),
			Timeframe:  domain.Timeframe(randomWord("timeframe")),
			TimeRange:  makeTimeRange(),
		})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)

		invalidRange := makeTimeRange()
		invalidRange.End = invalidRange.Start

		_, err = NewTradeReadRequest(TradeReadRequestParams{
			Instrument: makeInstrument(),
			TimeRange:  invalidRange,
			PageSize:   -fake.IntBetween(1, 10),
		})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
	})

	t.Run("result constructors return canonical whole values", func(t *testing.T) {
		t.Parallel()

		candle := makeCandle()
		candleResult, err := NewCandleReadResult([]domain.Candle{candle}, "  "+randomWord("page")+"  ")
		require.NoError(t, err)
		expectedCandle, err := domain.NewCandle(domain.CandleParams{
			Instrument: mustInstrument(t, candle.Instrument),
			Timeframe:  mustTimeframe(t, candle.Timeframe),
			TimeRange:  mustTimeRange(t, candle.TimeRange),
			Open:       candle.Open,
			High:       candle.High,
			Low:        candle.Low,
			Close:      candle.Close,
			Volume:     candle.Volume,
			Quality:    mustQuality(t, candle.Quality),
			Provenance: mustProvenance(t, candle.Provenance),
		})
		require.NoError(t, err)
		require.Equal(
			t,
			CandleReadResult{
				Candles:       []domain.Candle{expectedCandle},
				NextPageToken: strings.TrimSpace(candleResult.NextPageToken),
			},
			candleResult,
		)

		trade := makeTrade()
		tradeResult, err := NewTradeReadResult([]domain.Trade{trade}, "  "+randomWord("page")+"  ")
		require.NoError(t, err)
		expectedTrade, err := domain.NewTrade(domain.TradeParams{
			Instrument: mustInstrument(t, trade.Instrument),
			EventTime:  trade.EventTime,
			Price:      trade.Price,
			Size:       trade.Size,
			Quality:    mustQuality(t, trade.Quality),
			Provenance: mustProvenance(t, trade.Provenance),
		})
		require.NoError(t, err)
		require.Equal(
			t,
			TradeReadResult{
				Trades:        []domain.Trade{expectedTrade},
				NextPageToken: strings.TrimSpace(tradeResult.NextPageToken),
			},
			tradeResult,
		)

		instrument := makeInstrument()
		instrumentResult, err := NewInstrumentReadResult([]domain.Instrument{instrument}, "  "+randomWord("page")+"  ")
		require.NoError(t, err)
		require.Equal(
			t,
			InstrumentReadResult{
				Instruments:   []domain.Instrument{mustInstrument(t, instrument)},
				NextPageToken: strings.TrimSpace(instrumentResult.NextPageToken),
			},
			instrumentResult,
		)
	})

	t.Run("result constructors reject invalid canonical records", func(t *testing.T) {
		t.Parallel()

		_, err := NewInstrumentReadResult([]domain.Instrument{{}}, "")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)

		_, err = NewCandleReadResult([]domain.Candle{{}}, "")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)

		_, err = NewTradeReadResult([]domain.Trade{{}}, "")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
	})
}

func mustInstrument(t *testing.T, value domain.Instrument) domain.Instrument {
	t.Helper()

	venue, err := domain.NewVenue(value.Venue.String())
	require.NoError(t, err)
	symbol, err := domain.NewSymbol(value.Symbol.String())
	require.NoError(t, err)
	assetClass, err := domain.NewAssetClass(value.AssetClass.String())
	require.NoError(t, err)
	instrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
		Active:     value.Active,
	})
	require.NoError(t, err)

	return instrument
}

func mustTimeframe(t *testing.T, value domain.Timeframe) domain.Timeframe {
	t.Helper()

	timeframe, err := domain.NewTimeframe(value.String())
	require.NoError(t, err)

	return timeframe
}

func mustQuality(t *testing.T, value domain.DataQuality) domain.DataQuality {
	t.Helper()

	quality, err := domain.NewDataQuality(value.String())
	require.NoError(t, err)

	return quality
}

func mustTimeRange(t *testing.T, value domain.TimeRange) domain.TimeRange {
	t.Helper()

	timeRange, err := domain.NewTimeRange(value.Start, value.End)
	require.NoError(t, err)

	return timeRange
}

func mustProvenance(t *testing.T, value domain.SourceProvenance) domain.SourceProvenance {
	t.Helper()

	provenance, err := domain.NewSourceProvenance(value.Source, value.RecordID)
	require.NoError(t, err)

	return provenance
}
