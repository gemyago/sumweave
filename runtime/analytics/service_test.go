package analytics

import (
	"context"
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type replayCall struct {
	instrument domain.Instrument
	timeframe  domain.Timeframe
	timeRange  domain.TimeRange
}

type fakeReplayReader struct {
	calls  []replayCall
	result []data.ReplayCandle
	err    error
}

func (f *fakeReplayReader) ReplayCandles(
	_ context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]data.ReplayCandle, error) {
	f.calls = append(f.calls, replayCall{
		instrument: instrument,
		timeframe:  timeframe,
		timeRange:  timeRange,
	})
	if f.err != nil {
		return nil, f.err
	}

	return f.result, nil
}

func TestService(t *testing.T) {
	t.Parallel()

	newFake := func(t *testing.T) faker.Faker {
		t.Helper()

		hasher := fnv.New64a()
		_, _ = hasher.Write([]byte(t.Name()))

		return faker.NewWithSeedInt64(int64(hasher.Sum64()))
	}

	randomWord := func(t *testing.T, fake faker.Faker, prefix string) string {
		t.Helper()

		return prefix + "-" + strings.ToLower(fake.Lorem().Word()) + "-" + strconv.Itoa(fake.IntBetween(1000, 9999))
	}

	randomTime := func(t *testing.T, fake faker.Faker) time.Time {
		t.Helper()

		return time.Date(
			fake.IntBetween(2022, 2031),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone(randomWord(t, fake, "zone"), fake.IntBetween(-11, 12)*3600),
		)
	}

	makeInstrument := func(t *testing.T, fake faker.Faker) domain.Instrument {
		t.Helper()

		venue, err := domain.NewVenue("  " + randomWord(t, fake, "venue") + "  ")
		require.NoError(t, err)

		symbol, err := domain.NewSymbol("  " + strings.ToUpper(randomWord(t, fake, "symbol")) + "  ")
		require.NoError(t, err)

		assetClass, err := domain.NewAssetClass(domain.AssetClassCrypto.String())
		require.NoError(t, err)

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      venue,
			Symbol:     symbol,
			AssetClass: assetClass,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeRequestInstrument := func(instrument domain.Instrument) domain.Instrument {
		return domain.Instrument{
			Venue:      domain.Venue("  " + instrument.Venue.String() + "  "),
			Symbol:     domain.Symbol("  " + instrument.Symbol.String() + "  "),
			AssetClass: instrument.AssetClass,
			Active:     instrument.Active,
		}
	}

	canonicalRequestInstrument := func(t *testing.T, instrument domain.Instrument) domain.Instrument {
		t.Helper()

		venue, err := domain.NewVenue(instrument.Venue.String())
		require.NoError(t, err)

		symbol, err := domain.NewSymbol(instrument.Symbol.String())
		require.NoError(t, err)

		assetClass, err := domain.NewAssetClass(instrument.AssetClass.String())
		require.NoError(t, err)

		canonical, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      venue,
			Symbol:     symbol,
			AssetClass: assetClass,
			Active:     instrument.Active,
		})
		require.NoError(t, err)

		return canonical
	}

	makeRequestRange := func(t *testing.T, start time.Time, candles int, width time.Duration) domain.TimeRange {
		t.Helper()

		timeRange, err := domain.NewTimeRange(
			start.Add(-17*time.Second),
			start.Add(time.Duration(candles)*width).Add(29*time.Second),
		)
		require.NoError(t, err)

		return timeRange
	}

	makeReplayCandles := func(
		t *testing.T,
		fake faker.Faker,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		start time.Time,
		width time.Duration,
		closes []float64,
		qualities []domain.DataQuality,
	) []data.ReplayCandle {
		t.Helper()

		replayed := make([]data.ReplayCandle, len(closes))
		identityBase := uint64(fake.IntBetween(100, 900))
		for idx, closeValue := range closes {
			openValue := closeValue - float64(fake.IntBetween(1, 40))/10
			highValue := closeValue + float64(fake.IntBetween(1, 40))/10
			lowValue := openValue - float64(fake.IntBetween(1, 20))/10

			recordID := ""
			if idx%2 != 0 {
				recordID = randomWord(t, fake, "record")
			}

			provenance, err := domain.NewSourceProvenance(
				randomWord(t, fake, "replay-source"),
				recordID,
			)
			require.NoError(t, err)

			candle, err := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  timeframe,
				TimeRange: domain.TimeRange{
					Start: start.Add(time.Duration(idx) * width),
					End:   start.Add(time.Duration(idx+1) * width),
				},
				Open:       openValue,
				High:       highValue,
				Low:        lowValue,
				Close:      closeValue,
				Volume:     float64(fake.IntBetween(10, 5000)),
				Quality:    qualities[idx],
				Provenance: provenance,
			})
			require.NoError(t, err)

			replayed[idx] = data.ReplayCandle{
				Identity: identityBase + uint64(idx) + 1,
				Candle:   candle,
			}
		}

		return replayed
	}

	buildExpectedMovingAverage := func(
		t *testing.T,
		request CalculateCandlesRequest,
		replayed []data.ReplayCandle,
	) domain.AnalyticsSeries {
		t.Helper()

		points := make([]domain.AnalyticsPoint, 0, len(replayed)-request.IndicatorParams.Window+1)
		window := request.IndicatorParams.Window
		for idx := window - 1; idx < len(replayed); idx++ {
			windowCandles := replayed[idx-window+1 : idx+1]
			sum := 0.0
			quality := domain.DataQualityValidated
			hasRaw := false
			for _, replayedCandle := range windowCandles {
				sum += replayedCandle.Candle.Close
				switch replayedCandle.Candle.Quality {
				case domain.DataQualityValidated:
				case domain.DataQualitySuspect:
					quality = domain.DataQualitySuspect
				case domain.DataQualityRaw:
					hasRaw = true
				}
			}
			if quality != domain.DataQualitySuspect && hasRaw {
				quality = domain.DataQualityRaw
			}

			recordID := windowCandles[len(windowCandles)-1].Candle.Provenance.RecordID
			if recordID == "" {
				recordID = strconv.FormatUint(windowCandles[len(windowCandles)-1].Identity, 10)
			}

			point, err := domain.NewAnalyticsPoint(domain.AnalyticsPointParams{
				Time: windowCandles[len(windowCandles)-1].Candle.TimeRange.End,
				ValueRange: domain.AnalyticsValueRange{
					Start: windowCandles[0].Candle.TimeRange.Start,
					End:   windowCandles[len(windowCandles)-1].Candle.TimeRange.End,
				},
				Value:                sum / float64(window),
				Quality:              quality,
				SourceReplayIdentity: windowCandles[len(windowCandles)-1].Identity,
				SourceProvenance: domain.SourceProvenance{
					Source:   windowCandles[len(windowCandles)-1].Candle.Provenance.Source,
					RecordID: recordID,
				},
			})
			require.NoError(t, err)

			points = append(points, point)
		}

		identity, err := domain.NewAnalyticsSeriesIdentity(domain.AnalyticsSeriesIdentityParams{
			Instrument: canonicalRequestInstrument(t, request.Instrument),
			Timeframe:  request.Timeframe,
			Kind:       request.IndicatorKind,
			Parameters: request.IndicatorParams,
			TimeRange:  request.TimeRange,
		})
		require.NoError(t, err)

		series, err := domain.NewAnalyticsSeries(domain.AnalyticsSeriesParams{
			Identity: identity,
			Points:   points,
		})
		require.NoError(t, err)

		return series
	}

	buildExpectedPeriodReturn := func(
		t *testing.T,
		request CalculateCandlesRequest,
		replayed []data.ReplayCandle,
	) domain.AnalyticsSeries {
		t.Helper()

		points := make([]domain.AnalyticsPoint, 0, len(replayed)-request.IndicatorParams.Lookback)
		lookback := request.IndicatorParams.Lookback
		for idx := lookback; idx < len(replayed); idx++ {
			currentReplay := replayed[idx]
			lookbackReplay := replayed[idx-lookback]

			quality := domain.DataQualityValidated
			hasRaw := false
			for _, replayedCandle := range replayed[idx-lookback : idx+1] {
				switch replayedCandle.Candle.Quality {
				case domain.DataQualityValidated:
				case domain.DataQualitySuspect:
					quality = domain.DataQualitySuspect
				case domain.DataQualityRaw:
					hasRaw = true
				}
			}
			if quality != domain.DataQualitySuspect && hasRaw {
				quality = domain.DataQualityRaw
			}

			recordID := currentReplay.Candle.Provenance.RecordID
			if recordID == "" {
				recordID = strconv.FormatUint(currentReplay.Identity, 10)
			}

			point, err := domain.NewAnalyticsPoint(domain.AnalyticsPointParams{
				Time: currentReplay.Candle.TimeRange.End,
				ValueRange: domain.AnalyticsValueRange{
					Start: lookbackReplay.Candle.TimeRange.Start,
					End:   currentReplay.Candle.TimeRange.End,
				},
				Value:                (currentReplay.Candle.Close - lookbackReplay.Candle.Close) / lookbackReplay.Candle.Close,
				Quality:              quality,
				SourceReplayIdentity: currentReplay.Identity,
				SourceProvenance: domain.SourceProvenance{
					Source:   currentReplay.Candle.Provenance.Source,
					RecordID: recordID,
				},
			})
			require.NoError(t, err)

			points = append(points, point)
		}

		identity, err := domain.NewAnalyticsSeriesIdentity(domain.AnalyticsSeriesIdentityParams{
			Instrument: canonicalRequestInstrument(t, request.Instrument),
			Timeframe:  request.Timeframe,
			Kind:       request.IndicatorKind,
			Parameters: request.IndicatorParams,
			TimeRange:  request.TimeRange,
		})
		require.NoError(t, err)

		series, err := domain.NewAnalyticsSeries(domain.AnalyticsSeriesParams{
			Identity: identity,
			Points:   points,
		})
		require.NoError(t, err)

		return series
	}

	t.Run("new service requires replay reader", func(t *testing.T) {
		t.Parallel()

		svc, err := NewService(ServiceDeps{})
		require.Error(t, err)
		require.Nil(t, svc)
		require.EqualError(t, err, "candle replay reader is required")
	})

	t.Run("calculate candles returns stable moving average series", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		expectedRequestedInstrument := canonicalRequestInstrument(t, requestInstrument)
		timeframe := domain.Timeframe1m
		rangeStart := randomTime(t, fake)
		closes := []float64{
			float64(fake.IntBetween(90, 110)) + 0.25,
			float64(fake.IntBetween(111, 130)) + 0.5,
			float64(fake.IntBetween(131, 150)) + 0.75,
			float64(fake.IntBetween(151, 170)) + 0.125,
			float64(fake.IntBetween(171, 190)) + 0.875,
		}
		qualities := []domain.DataQuality{
			domain.DataQualityValidated,
			domain.DataQualityRaw,
			domain.DataQualityValidated,
			domain.DataQualitySuspect,
			domain.DataQualityValidated,
		}
		replayed := makeReplayCandles(t, fake, instrument, timeframe, rangeStart, time.Minute, closes, qualities)
		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		request := CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe("  " + timeframe.String() + "  "),
			TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
			IndicatorKind: domain.IndicatorKind("  " + domain.IndicatorKindMovingAverage.String() + "  "),
			IndicatorParams: domain.IndicatorParams{
				Window: 3,
			},
		}

		first, err := svc.CalculateCandles(t.Context(), request)
		require.NoError(t, err)

		second, err := svc.CalculateCandles(t.Context(), request)
		require.NoError(t, err)

		require.Equal(t, first, second)
		require.Len(t, reader.calls, 2)

		expectedRange, err := domain.NewTimeRange(request.TimeRange.Start, request.TimeRange.End)
		require.NoError(t, err)
		require.Equal(t, replayCall{
			instrument: expectedRequestedInstrument,
			timeframe:  timeframe,
			timeRange:  expectedRange,
		}, reader.calls[0])
		require.Equal(t, reader.calls[0], reader.calls[1])

		expected := buildExpectedMovingAverage(t, request, replayed)
		require.Equal(t, expected, first)
		require.Equal(t, expectedRequestedInstrument, first.Identity.Instrument)
		require.Len(t, first.Points, 3)
	})

	t.Run("calculate candles uses requested half-open range without hidden lookback reads", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		rangeStart := randomTime(t, fake)
		replayed := makeReplayCandles(
			t,
			fake,
			instrument,
			domain.Timeframe1m,
			rangeStart,
			time.Minute,
			[]float64{
				float64(fake.IntBetween(90, 110)) + 0.1,
				float64(fake.IntBetween(111, 130)) + 0.2,
			},
			[]domain.DataQuality{
				domain.DataQualityValidated,
				domain.DataQualityValidated,
			},
		)
		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		request := CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe1m,
			TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
			IndicatorKind: domain.IndicatorKindPeriodReturn,
			IndicatorParams: domain.IndicatorParams{
				Lookback: 3,
			},
		}

		got, err := svc.CalculateCandles(t.Context(), request)
		require.NoError(t, err)
		require.Empty(t, got.Points)
		require.Len(t, reader.calls, 1)

		expectedRange, err := domain.NewTimeRange(request.TimeRange.Start, request.TimeRange.End)
		require.NoError(t, err)
		require.Equal(t, replayCall{
			instrument: instrument,
			timeframe:  domain.Timeframe1m,
			timeRange:  expectedRange,
		}, reader.calls[0])
	})

	t.Run("calculate candles fails when replay candle asset class disagrees with request", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		replayInstrument := makeInstrument(t, fake)
		requestInstrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      replayInstrument.Venue,
			Symbol:     replayInstrument.Symbol,
			AssetClass: domain.AssetClassEquity,
			Active:     false,
		})
		require.NoError(t, err)

		requestInstrument = makeRequestInstrument(requestInstrument)
		rangeStart := randomTime(t, fake)
		replayed := makeReplayCandles(
			t,
			fake,
			replayInstrument,
			domain.Timeframe1m,
			rangeStart,
			time.Minute,
			[]float64{
				float64(fake.IntBetween(90, 110)) + 0.1,
				float64(fake.IntBetween(111, 130)) + 0.2,
			},
			[]domain.DataQuality{
				domain.DataQualityValidated,
				domain.DataQualityValidated,
			},
		)
		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe1m,
			TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
			IndicatorKind: domain.IndicatorKindMovingAverage,
			IndicatorParams: domain.IndicatorParams{
				Window: 1,
			},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
		require.Contains(t, err.Error(), "replay candle 0 asset class mismatch")
		require.Equal(t, domain.AnalyticsSeries{}, got)
		require.Len(t, reader.calls, 1)
	})

	t.Run("calculate candles fails when replay candle start is outside requested range", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		rangeStart := randomTime(t, fake)
		replayed := makeReplayCandles(
			t,
			fake,
			instrument,
			domain.Timeframe1m,
			rangeStart,
			time.Minute,
			[]float64{
				float64(fake.IntBetween(90, 110)) + 0.1,
				float64(fake.IntBetween(111, 130)) + 0.2,
			},
			[]domain.DataQuality{
				domain.DataQualityValidated,
				domain.DataQualityValidated,
			},
		)
		requestRange, err := domain.NewTimeRange(
			replayed[0].Candle.TimeRange.Start,
			replayed[len(replayed)-1].Candle.TimeRange.End,
		)
		require.NoError(t, err)
		replayed[0].Candle.TimeRange = domain.TimeRange{
			Start: requestRange.Start.Add(-time.Minute),
			End:   requestRange.Start,
		}

		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe1m,
			TimeRange:     requestRange,
			IndicatorKind: domain.IndicatorKindMovingAverage,
			IndicatorParams: domain.IndicatorParams{
				Window: 1,
			},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
		require.Contains(t, err.Error(), "replay candle 0 start time")
		require.Contains(t, err.Error(), "outside requested range")
		require.Equal(t, domain.AnalyticsSeries{}, got)
		require.Len(t, reader.calls, 1)
	})

	t.Run("calculate candles fails when replay candle timeframe disagrees with request", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		rangeStart := randomTime(t, fake)
		replayed := makeReplayCandles(
			t,
			fake,
			instrument,
			domain.Timeframe1m,
			rangeStart,
			time.Minute,
			[]float64{
				float64(fake.IntBetween(90, 110)) + 0.1,
				float64(fake.IntBetween(111, 130)) + 0.2,
			},
			[]domain.DataQuality{
				domain.DataQualityValidated,
				domain.DataQualityValidated,
			},
		)
		replayed[0].Candle.Timeframe = domain.Timeframe5m

		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe1m,
			TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
			IndicatorKind: domain.IndicatorKindMovingAverage,
			IndicatorParams: domain.IndicatorParams{
				Window: 1,
			},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrValidation)
		require.Contains(t, err.Error(), "replay candle 0 timeframe mismatch")
		require.Equal(t, domain.AnalyticsSeries{}, got)
		require.Len(t, reader.calls, 1)
	})

	t.Run("calculate candles uses replay active flag for canonical series identity", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		replayInstrument := makeInstrument(t, fake)
		requestInstrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      replayInstrument.Venue,
			Symbol:     replayInstrument.Symbol,
			AssetClass: replayInstrument.AssetClass,
			Active:     false,
		})
		require.NoError(t, err)

		requestInstrument = makeRequestInstrument(requestInstrument)
		rangeStart := randomTime(t, fake)
		replayed := makeReplayCandles(
			t,
			fake,
			replayInstrument,
			domain.Timeframe1m,
			rangeStart,
			time.Minute,
			[]float64{
				float64(fake.IntBetween(90, 110)) + 0.1,
				float64(fake.IntBetween(111, 130)) + 0.2,
			},
			[]domain.DataQuality{
				domain.DataQualityValidated,
				domain.DataQualityValidated,
			},
		)
		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		request := CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe1m,
			TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
			IndicatorKind: domain.IndicatorKindMovingAverage,
			IndicatorParams: domain.IndicatorParams{
				Window: 1,
			},
		}

		got, err := svc.CalculateCandles(t.Context(), request)
		require.NoError(t, err)

		require.Len(t, got.Points, len(replayed))
		require.Equal(t, replayInstrument, got.Identity.Instrument)
		require.True(t, got.Identity.Instrument.Active)
		require.False(t, canonicalRequestInstrument(t, request.Instrument).Active)
	})

	t.Run("calculate candles returns period return values and ranges", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		rangeStart := randomTime(t, fake)
		replayed := makeReplayCandles(
			t,
			fake,
			instrument,
			domain.Timeframe1m,
			rangeStart,
			time.Minute,
			[]float64{100, 120, 150, 60},
			[]domain.DataQuality{
				domain.DataQualityValidated,
				domain.DataQualityValidated,
				domain.DataQualityValidated,
				domain.DataQualityValidated,
			},
		)
		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		request := CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe1m,
			TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
			IndicatorKind: domain.IndicatorKindPeriodReturn,
			IndicatorParams: domain.IndicatorParams{
				Lookback: 2,
			},
		}

		got, err := svc.CalculateCandles(t.Context(), request)
		require.NoError(t, err)

		expected := buildExpectedPeriodReturn(t, request, replayed)
		require.Equal(t, expected, got)
		require.Len(t, got.Points, 2)
		require.Equal(t, instrument, got.Identity.Instrument)
		require.InDelta(t, 0.5, got.Points[0].Value, 1e-12)
		require.Equal(t, replayed[0].Candle.TimeRange.Start, got.Points[0].ValueRange.Start)
		require.Equal(t, replayed[2].Candle.TimeRange.End, got.Points[0].ValueRange.End)
		require.InDelta(t, -0.5, got.Points[1].Value, 1e-12)
		require.Equal(t, replayed[1].Candle.TimeRange.Start, got.Points[1].ValueRange.Start)
		require.Equal(t, replayed[3].Candle.TimeRange.End, got.Points[1].ValueRange.End)
	})

	t.Run("calculate candles normalizes replay identity order for equal point times", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		rangeStart := randomTime(t, fake)
		pointEnd := rangeStart.Add(3 * time.Minute)

		makeReplayCandle := func(
			t *testing.T,
			identity uint64,
			start time.Time,
			end time.Time,
			closeValue float64,
		) data.ReplayCandle {
			t.Helper()

			provenance, err := domain.NewSourceProvenance(
				randomWord(t, fake, "replay-source"),
				randomWord(t, fake, "record"),
			)
			require.NoError(t, err)

			candle, err := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				TimeRange: domain.TimeRange{
					Start: start,
					End:   end,
				},
				Open:       closeValue - 1,
				High:       closeValue + 1,
				Low:        closeValue - 2,
				Close:      closeValue,
				Volume:     10,
				Quality:    domain.DataQualityValidated,
				Provenance: provenance,
			})
			require.NoError(t, err)

			return data.ReplayCandle{
				Identity: identity,
				Candle:   candle,
			}
		}

		replayed := []data.ReplayCandle{
			makeReplayCandle(t, 102, rangeStart.Add(time.Minute), pointEnd, 120),
			makeReplayCandle(t, 101, rangeStart, pointEnd, 100),
		}
		require.Greater(t, replayed[0].Identity, replayed[1].Identity)
		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe1m,
			TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
			IndicatorKind: domain.IndicatorKindMovingAverage,
			IndicatorParams: domain.IndicatorParams{
				Window: 1,
			},
		})
		require.NoError(t, err)
		require.Len(t, got.Points, 2)
		require.Equal(t, got.Points[0].Time, got.Points[1].Time)
		require.Equal(t, uint64(101), got.Points[0].SourceReplayIdentity)
		require.Equal(t, uint64(102), got.Points[1].SourceReplayIdentity)
	})

	t.Run("calculate candles preserves replay order before sorting emitted points", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		rangeStart := randomTime(t, fake)

		makeReplayCandle := func(
			t *testing.T,
			identity uint64,
			start time.Time,
			end time.Time,
			closeValue float64,
		) data.ReplayCandle {
			t.Helper()

			provenance, err := domain.NewSourceProvenance(
				randomWord(t, fake, "replay-source"),
				randomWord(t, fake, "record"),
			)
			require.NoError(t, err)

			candle, err := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				TimeRange: domain.TimeRange{
					Start: start,
					End:   end,
				},
				Open:       closeValue - 1,
				High:       closeValue + 1,
				Low:        closeValue - 2,
				Close:      closeValue,
				Volume:     10,
				Quality:    domain.DataQualityValidated,
				Provenance: provenance,
			})
			require.NoError(t, err)

			return data.ReplayCandle{
				Identity: identity,
				Candle:   candle,
			}
		}

		firstReplay := makeReplayCandle(
			t,
			101,
			rangeStart,
			rangeStart.Add(3*time.Minute),
			100,
		)
		secondReplay := makeReplayCandle(
			t,
			102,
			rangeStart.Add(time.Minute),
			rangeStart.Add(5*time.Minute),
			120,
		)
		thirdReplay := makeReplayCandle(
			t,
			103,
			rangeStart.Add(2*time.Minute),
			rangeStart.Add(4*time.Minute),
			200,
		)
		replayed := []data.ReplayCandle{firstReplay, secondReplay, thirdReplay}
		reader := &fakeReplayReader{result: replayed}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
			Instrument:    requestInstrument,
			Timeframe:     domain.Timeframe1m,
			TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
			IndicatorKind: domain.IndicatorKindMovingAverage,
			IndicatorParams: domain.IndicatorParams{
				Window: 2,
			},
		})
		require.NoError(t, err)
		require.Len(t, got.Points, 2)

		require.Equal(t, thirdReplay.Candle.TimeRange.End, got.Points[0].Time.Time())
		require.Equal(t, thirdReplay.Identity, got.Points[0].SourceReplayIdentity)
		require.Equal(t, thirdReplay.Candle.Provenance.Source, got.Points[0].SourceProvenance.Source)
		require.Equal(t, thirdReplay.Candle.Provenance.RecordID, got.Points[0].SourceProvenance.RecordID)
		require.InDelta(t, 160.0, got.Points[0].Value, 1e-12)
		require.Equal(t, secondReplay.Candle.TimeRange.Start, got.Points[0].ValueRange.Start)
		require.Equal(t, thirdReplay.Candle.TimeRange.End, got.Points[0].ValueRange.End)

		require.Equal(t, secondReplay.Candle.TimeRange.End, got.Points[1].Time.Time())
		require.Equal(t, secondReplay.Identity, got.Points[1].SourceReplayIdentity)
		require.Equal(t, secondReplay.Candle.Provenance.Source, got.Points[1].SourceProvenance.Source)
		require.Equal(t, secondReplay.Candle.Provenance.RecordID, got.Points[1].SourceProvenance.RecordID)
		require.InDelta(t, 110.0, got.Points[1].Value, 1e-12)
		require.Equal(t, firstReplay.Candle.TimeRange.Start, got.Points[1].ValueRange.Start)
		require.Equal(t, secondReplay.Candle.TimeRange.End, got.Points[1].ValueRange.End)
	})

	t.Run("calculate candles omits warmup points until computations are ready", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		rangeStart := randomTime(t, fake)
		closes := []float64{
			float64(fake.IntBetween(100, 120)) + 0.1,
			float64(fake.IntBetween(121, 140)) + 0.2,
			float64(fake.IntBetween(141, 160)) + 0.3,
			float64(fake.IntBetween(161, 180)) + 0.4,
			float64(fake.IntBetween(181, 200)) + 0.5,
		}
		qualities := []domain.DataQuality{
			domain.DataQualityValidated,
			domain.DataQualityValidated,
			domain.DataQualityRaw,
			domain.DataQualityValidated,
			domain.DataQualitySuspect,
		}
		replayed := makeReplayCandles(
			t,
			fake,
			instrument,
			domain.Timeframe1m,
			rangeStart,
			time.Minute,
			closes,
			qualities,
		)

		t.Run("moving average omits only warmup candles", func(t *testing.T) {
			t.Parallel()

			reader := &fakeReplayReader{result: replayed}
			svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
			require.NoError(t, err)

			got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
				Instrument:    requestInstrument,
				Timeframe:     domain.Timeframe1m,
				TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
				IndicatorKind: domain.IndicatorKindMovingAverage,
				IndicatorParams: domain.IndicatorParams{
					Window: 3,
				},
			})
			require.NoError(t, err)
			require.Len(t, got.Points, 3)
			require.Equal(t, replayed[2].Candle.TimeRange.End, got.Points[0].Time.Time())
			require.Equal(t, replayed[0].Candle.TimeRange.Start, got.Points[0].ValueRange.Start)
			require.Equal(t, replayed[4].Candle.TimeRange.End, got.Points[2].Time.Time())
			require.Equal(t, replayed[2].Candle.TimeRange.Start, got.Points[2].ValueRange.Start)
		})

		t.Run("period return omits only warmup candles", func(t *testing.T) {
			t.Parallel()

			reader := &fakeReplayReader{result: replayed}
			svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
			require.NoError(t, err)

			got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
				Instrument:    requestInstrument,
				Timeframe:     domain.Timeframe1m,
				TimeRange:     makeRequestRange(t, rangeStart, len(replayed), time.Minute),
				IndicatorKind: domain.IndicatorKindPeriodReturn,
				IndicatorParams: domain.IndicatorParams{
					Lookback: 2,
				},
			})
			require.NoError(t, err)
			require.Len(t, got.Points, 3)
			require.Equal(t, replayed[2].Candle.TimeRange.End, got.Points[0].Time.Time())
			require.Equal(t, replayed[0].Candle.TimeRange.Start, got.Points[0].ValueRange.Start)
			require.Equal(t, replayed[4].Candle.TimeRange.End, got.Points[2].Time.Time())
			require.Equal(t, replayed[2].Candle.TimeRange.Start, got.Points[2].ValueRange.Start)
			require.Equal(t, domain.DataQualitySuspect, got.Points[2].Quality)
		})
	})

	t.Run("calculate candles rejects invalid indicator requests without partial output", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestInstrument := makeRequestInstrument(instrument)
		rangeStart := randomTime(t, fake)
		replayed := makeReplayCandles(
			t,
			fake,
			instrument,
			domain.Timeframe1m,
			rangeStart,
			time.Minute,
			[]float64{
				float64(fake.IntBetween(100, 120)) + 0.1,
				float64(fake.IntBetween(121, 140)) + 0.2,
				float64(fake.IntBetween(141, 160)) + 0.3,
			},
			[]domain.DataQuality{
				domain.DataQualityValidated,
				domain.DataQualityValidated,
				domain.DataQualityValidated,
			},
		)

		cases := []struct {
			name    string
			request CalculateCandlesRequest
		}{
			{
				name: "moving average requires positive window",
				request: CalculateCandlesRequest{
					Instrument:    requestInstrument,
					Timeframe:     domain.Timeframe1m,
					TimeRange:     makeRequestRange(t, rangeStart, 3, time.Minute),
					IndicatorKind: domain.IndicatorKindMovingAverage,
					IndicatorParams: domain.IndicatorParams{
						Window: 0,
					},
				},
			},
			{
				name: "moving average rejects lookback",
				request: CalculateCandlesRequest{
					Instrument:    requestInstrument,
					Timeframe:     domain.Timeframe1m,
					TimeRange:     makeRequestRange(t, rangeStart, 3, time.Minute),
					IndicatorKind: domain.IndicatorKindMovingAverage,
					IndicatorParams: domain.IndicatorParams{
						Window:   2,
						Lookback: 1,
					},
				},
			},
			{
				name: "period return requires positive lookback",
				request: CalculateCandlesRequest{
					Instrument:    requestInstrument,
					Timeframe:     domain.Timeframe1m,
					TimeRange:     makeRequestRange(t, rangeStart, 3, time.Minute),
					IndicatorKind: domain.IndicatorKindPeriodReturn,
					IndicatorParams: domain.IndicatorParams{
						Lookback: 0,
					},
				},
			},
			{
				name: "period return rejects window",
				request: CalculateCandlesRequest{
					Instrument:    requestInstrument,
					Timeframe:     domain.Timeframe1m,
					TimeRange:     makeRequestRange(t, rangeStart, 3, time.Minute),
					IndicatorKind: domain.IndicatorKindPeriodReturn,
					IndicatorParams: domain.IndicatorParams{
						Window:   1,
						Lookback: 2,
					},
				},
			},
			{
				name: "unsupported indicator kind is rejected",
				request: CalculateCandlesRequest{
					Instrument:    requestInstrument,
					Timeframe:     domain.Timeframe1m,
					TimeRange:     makeRequestRange(t, rangeStart, 3, time.Minute),
					IndicatorKind: domain.IndicatorKind(randomWord(t, fake, "unsupported-kind")),
					IndicatorParams: domain.IndicatorParams{
						Window: 2,
					},
				},
			},
		}

		for i := range cases {
			tc := cases[i]
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				reader := &fakeReplayReader{result: replayed}
				svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
				require.NoError(t, err)

				got, calcErr := svc.CalculateCandles(t.Context(), tc.request)
				require.Error(t, calcErr)
				require.ErrorIs(t, calcErr, ErrValidation)
				require.Equal(t, domain.AnalyticsSeries{}, got)
				require.Empty(t, reader.calls)
			})
		}
	})

	t.Run(
		"calculate candles fails period return without partial output on non-positive denominator",
		func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			instrument := makeInstrument(t, fake)
			requestInstrument := makeRequestInstrument(instrument)
			rangeStart := randomTime(t, fake)

			cases := []struct {
				name   string
				closes []float64
			}{
				{
					name: "zero denominator",
					closes: []float64{
						float64(fake.IntBetween(100, 120)) + 0.1,
						float64(fake.IntBetween(121, 140)) + 0.2,
						0,
						float64(fake.IntBetween(161, 180)) + 0.4,
					},
				},
				{
					name: "negative denominator",
					closes: []float64{
						float64(fake.IntBetween(100, 120)) + 0.1,
						float64(fake.IntBetween(121, 140)) + 0.2,
						-1 * (float64(fake.IntBetween(1, 20)) + 0.3),
						float64(fake.IntBetween(161, 180)) + 0.4,
					},
				},
			}

			for i := range cases {
				tc := cases[i]
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					caseFake := newFake(t)
					reader := &fakeReplayReader{
						result: makeReplayCandles(
							t,
							caseFake,
							instrument,
							domain.Timeframe1m,
							rangeStart,
							time.Minute,
							tc.closes,
							[]domain.DataQuality{
								domain.DataQualityValidated,
								domain.DataQualityValidated,
								domain.DataQualityValidated,
								domain.DataQualityValidated,
							},
						),
					}
					svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
					require.NoError(t, err)

					got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
						Instrument:    requestInstrument,
						Timeframe:     domain.Timeframe1m,
						TimeRange:     makeRequestRange(t, rangeStart, len(tc.closes), time.Minute),
						IndicatorKind: domain.IndicatorKindPeriodReturn,
						IndicatorParams: domain.IndicatorParams{
							Lookback: 1,
						},
					})
					require.Error(t, err)
					require.ErrorIs(t, err, ErrValidation)
					require.Contains(t, err.Error(), "period return lookback close must be positive")
					require.Equal(t, domain.AnalyticsSeries{}, got)
					require.Len(t, reader.calls, 1)
				})
			}
		},
	)

	t.Run("calculate candles wraps replay reader failures", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		reader := &fakeReplayReader{err: errors.New(randomWord(t, fake, "reader-failed"))}
		svc, err := NewService(ServiceDeps{CandleReplayReader: reader})
		require.NoError(t, err)

		assetClass, assetClassErr := domain.NewAssetClass(domain.AssetClassCrypto.String())
		require.NoError(t, assetClassErr)

		got, err := svc.CalculateCandles(t.Context(), CalculateCandlesRequest{
			Instrument: domain.Instrument{
				Venue:      domain.Venue("  " + randomWord(t, fake, "venue") + "  "),
				Symbol:     domain.Symbol("  " + strings.ToUpper(randomWord(t, fake, "symbol")) + "  "),
				AssetClass: assetClass,
				Active:     true,
			},
			Timeframe:     domain.Timeframe1m,
			TimeRange:     makeRequestRange(t, randomTime(t, fake), 2, time.Minute),
			IndicatorKind: domain.IndicatorKindMovingAverage,
			IndicatorParams: domain.IndicatorParams{
				Window: 1,
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "replay candles:")
		require.Equal(t, domain.AnalyticsSeries{}, got)
	})
}
