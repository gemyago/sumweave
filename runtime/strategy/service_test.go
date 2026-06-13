package strategy

import (
	"context"
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/analytics"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type analyticsCall struct {
	request analytics.CalculateCandlesRequest
}

type fakeAnalyticsCalculator struct {
	calls  []analyticsCall
	result []domain.AnalyticsSeries
	err    error
}

func (f *fakeAnalyticsCalculator) CalculateCandles(
	_ context.Context,
	request analytics.CalculateCandlesRequest,
) (domain.AnalyticsSeries, error) {
	f.calls = append(f.calls, analyticsCall{request: request})
	if f.err != nil {
		return domain.AnalyticsSeries{}, f.err
	}

	if len(f.result) == 0 {
		return domain.AnalyticsSeries{}, nil
	}

	result := f.result[0]
	f.result = f.result[1:]

	return result, nil
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
			Active:     fake.Bool(),
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

	makeRequestRange := func(t *testing.T, fake faker.Faker) domain.TimeRange {
		t.Helper()

		start := randomTime(t, fake)
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)

		timeRange, err := domain.NewTimeRange(
			start.Add(-11*time.Second),
			end.Add(17*time.Second),
		)
		require.NoError(t, err)

		return timeRange
	}

	makeSeries := func(
		t *testing.T,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
		window int,
	) domain.AnalyticsSeries {
		t.Helper()

		identity, err := domain.NewAnalyticsSeriesIdentity(domain.AnalyticsSeriesIdentityParams{
			Instrument: instrument,
			Timeframe:  timeframe,
			Kind:       domain.IndicatorKindMovingAverage,
			Parameters: domain.IndicatorParams{Window: window},
			TimeRange:  timeRange,
		})
		require.NoError(t, err)

		series, err := domain.NewAnalyticsSeries(domain.AnalyticsSeriesParams{
			Identity: identity,
			Points:   nil,
		})
		require.NoError(t, err)

		return series
	}

	makePoint := func(
		t *testing.T,
		when time.Time,
		valueRange domain.TimeRange,
		value float64,
		quality domain.DataQuality,
		sourceID uint64,
		recordID string,
	) domain.AnalyticsPoint {
		t.Helper()

		point, err := domain.NewAnalyticsPoint(domain.AnalyticsPointParams{
			Time:                 when,
			ValueRange:           domain.AnalyticsValueRange(valueRange),
			Value:                value,
			Quality:              quality,
			SourceReplayIdentity: sourceID,
			SourceProvenance: domain.SourceProvenance{
				Source:   "test-analytics",
				RecordID: recordID,
			},
		})
		require.NoError(t, err)

		return point
	}

	makeSeriesWithPoints := func(
		t *testing.T,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
		window int,
		points []domain.AnalyticsPoint,
	) domain.AnalyticsSeries {
		t.Helper()

		identity, err := domain.NewAnalyticsSeriesIdentity(domain.AnalyticsSeriesIdentityParams{
			Instrument: instrument,
			Timeframe:  timeframe,
			Kind:       domain.IndicatorKindMovingAverage,
			Parameters: domain.IndicatorParams{Window: window},
			TimeRange:  timeRange,
		})
		require.NoError(t, err)

		series, err := domain.NewAnalyticsSeries(domain.AnalyticsSeriesParams{
			Identity: identity,
			Points:   points,
		})
		require.NoError(t, err)

		return series
	}

	makeAction := func(
		t *testing.T,
		strategy domain.StrategyIdentity,
		kind domain.CandidateActionKind,
		decisionTime time.Time,
		inputRange domain.TimeRange,
		quality domain.DataQuality,
	) domain.CandidateAction {
		t.Helper()

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy:     strategy,
			Kind:         kind,
			DecisionTime: decisionTime,
			InputRange:   inputRange,
			Quality:      quality,
		})
		require.NoError(t, err)

		return action
	}

	makeDeps := func(t *testing.T) (*fakeAnalyticsCalculator, ServiceDeps) {
		t.Helper()

		calculator := &fakeAnalyticsCalculator{}

		return calculator, ServiceDeps{
			AnalyticsCalculator: calculator,
		}
	}

	t.Run("NewService", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects nil analytics calculator", func(t *testing.T) {
			t.Parallel()

			service, err := NewService(ServiceDeps{})
			require.Nil(t, service)
			require.EqualError(t, err, "analytics calculator is required")
		})
	})

	t.Run("Evaluate", func(t *testing.T) {
		t.Parallel()

		t.Run("rejects invalid request fields", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			validInstrument := makeInstrument(t, fake)
			validRange := makeRequestRange(t, fake)
			validParams := MovingAverageCrossoverParams{
				FastWindow: fake.IntBetween(1, 20),
				SlowWindow: fake.IntBetween(21, 60),
			}

			testCases := []struct {
				name        string
				request     EvaluateRequest
				expectedMsg string
			}{
				{
					name: "instrument venue is required",
					request: EvaluateRequest{
						Instrument: domain.Instrument{
							Symbol:     validInstrument.Symbol,
							AssetClass: validInstrument.AssetClass,
							Active:     validInstrument.Active,
						},
						Timeframe:    domain.Timeframe1m,
						TimeRange:    validRange,
						StrategyKind: domain.StrategyKindMovingAverageCrossover,
						Parameters:   validParams,
					},
					expectedMsg: "strategy instrument venue is required",
				},
				{
					name: "timeframe is required",
					request: EvaluateRequest{
						Instrument:   validInstrument,
						TimeRange:    validRange,
						StrategyKind: domain.StrategyKindMovingAverageCrossover,
						Parameters:   validParams,
					},
					expectedMsg: "strategy timeframe is required",
				},
				{
					name: "time range start is required",
					request: EvaluateRequest{
						Instrument:   validInstrument,
						Timeframe:    domain.Timeframe1m,
						TimeRange:    domain.TimeRange{End: validRange.End},
						StrategyKind: domain.StrategyKindMovingAverageCrossover,
						Parameters:   validParams,
					},
					expectedMsg: "strategy evaluation time range: time range start is required",
				},
				{
					name: "strategy kind is required",
					request: EvaluateRequest{
						Instrument: validInstrument,
						Timeframe:  domain.Timeframe1m,
						TimeRange:  validRange,
						Parameters: validParams,
					},
					expectedMsg: "strategy kind is required",
				},
				{
					name: "moving average fast window must be positive",
					request: EvaluateRequest{
						Instrument:   validInstrument,
						Timeframe:    domain.Timeframe1m,
						TimeRange:    validRange,
						StrategyKind: domain.StrategyKindMovingAverageCrossover,
						Parameters: MovingAverageCrossoverParams{
							FastWindow: 0,
							SlowWindow: validParams.SlowWindow,
						},
					},
					expectedMsg: "moving average crossover fast window must be positive",
				},
				{
					name: "moving average windows must stay ordered",
					request: EvaluateRequest{
						Instrument:   validInstrument,
						Timeframe:    domain.Timeframe1m,
						TimeRange:    validRange,
						StrategyKind: domain.StrategyKindMovingAverageCrossover,
						Parameters: MovingAverageCrossoverParams{
							FastWindow: validParams.SlowWindow,
							SlowWindow: validParams.FastWindow,
						},
					},
					expectedMsg: "moving average crossover fast window must be less than slow window",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					result, evalErr := service.Evaluate(t.Context(), testCase.request)
					require.Error(t, evalErr)
					require.ErrorIs(t, evalErr, ErrValidation)
					require.EqualError(t, evalErr, "strategy validation failed: "+testCase.expectedMsg)
					require.Equal(t, EvaluateResult{}, result)
					require.Empty(t, calculator.calls)
				})
			}
		})

		t.Run("requests fast and slow moving averages for the exact evaluation range", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			instrument := makeInstrument(t, fake)
			requestInstrument := makeRequestInstrument(instrument)
			timeRange := makeRequestRange(t, fake)
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fake.IntBetween(fastWindow+1, fastWindow+40)
			timeframe := domain.Timeframe("  " + strings.ToUpper(domain.Timeframe5m.String()) + "  ")

			calculator.result = []domain.AnalyticsSeries{
				makeSeries(t, instrument, domain.Timeframe5m, timeRange, fastWindow),
				makeSeries(t, instrument, domain.Timeframe5m, timeRange, slowWindow),
			}

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				Instrument: requestInstrument,
				Timeframe:  timeframe,
				TimeRange:  timeRange,
				StrategyKind: domain.StrategyKind(
					"  " + strings.ToUpper(domain.StrategyKindMovingAverageCrossover.String()) + "  ",
				),
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fastWindow,
					SlowWindow: slowWindow,
				},
			})
			require.NoError(t, err)

			expectedCalls := []analyticsCall{
				{
					request: analytics.CalculateCandlesRequest{
						Instrument:    instrument,
						Timeframe:     domain.Timeframe5m,
						TimeRange:     timeRange,
						IndicatorKind: domain.IndicatorKindMovingAverage,
						IndicatorParams: domain.IndicatorParams{
							Window: fastWindow,
						},
					},
				},
				{
					request: analytics.CalculateCandlesRequest{
						Instrument:    instrument,
						Timeframe:     domain.Timeframe5m,
						TimeRange:     timeRange,
						IndicatorKind: domain.IndicatorKindMovingAverage,
						IndicatorParams: domain.IndicatorParams{
							Window: slowWindow,
						},
					},
				},
			}
			require.Equal(t, expectedCalls, calculator.calls)

			expectedIdentity, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
				Instrument: instrument,
				Timeframe:  domain.Timeframe5m,
				Kind:       domain.StrategyKindMovingAverageCrossover,
			})
			require.NoError(t, err)

			require.Equal(t, EvaluateResult{
				Strategy:   expectedIdentity,
				TimeRange:  timeRange,
				Parameters: MovingAverageCrossoverParams{FastWindow: fastWindow, SlowWindow: slowWindow},
				Actions:    nil,
			}, result)
		})

		t.Run("wraps analytics failures", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			expectedErr := errors.New(randomWord(t, fake, "analytics"))
			calculator.err = expectedErr

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				Instrument:   makeInstrument(t, fake),
				Timeframe:    domain.Timeframe1m,
				TimeRange:    makeRequestRange(t, fake),
				StrategyKind: domain.StrategyKindMovingAverageCrossover,
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fake.IntBetween(1, 20),
					SlowWindow: fake.IntBetween(21, 60),
				},
			})
			require.Error(t, err)
			require.ErrorIs(t, err, expectedErr)
			require.EqualError(t, err, "calculate fast moving average analytics: "+expectedErr.Error())
			require.Equal(t, EvaluateResult{}, result)
		})

		t.Run("establishes the first aligned point as the in-range baseline only", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			instrument := makeInstrument(t, fake)
			timeRange := makeRequestRange(t, fake)
			timeframe := domain.Timeframe15m
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fake.IntBetween(fastWindow+1, fastWindow+40)
			alignedTime := timeRange.Start.Add(15 * time.Minute)

			calculator.result = []domain.AnalyticsSeries{
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, fastWindow, []domain.AnalyticsPoint{
					makePoint(
						t,
						alignedTime,
						domain.TimeRange{Start: timeRange.Start, End: alignedTime},
						10,
						domain.DataQualityValidated,
						1,
						randomWord(t, fake, "fast"),
					),
				}),
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, slowWindow, []domain.AnalyticsPoint{
					makePoint(
						t,
						alignedTime,
						domain.TimeRange{Start: timeRange.Start.Add(-5 * time.Minute), End: alignedTime},
						12,
						domain.DataQualityValidated,
						2,
						randomWord(t, fake, "slow"),
					),
				}),
			}

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				Instrument:   instrument,
				Timeframe:    timeframe,
				TimeRange:    timeRange,
				StrategyKind: domain.StrategyKindMovingAverageCrossover,
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fastWindow,
					SlowWindow: slowWindow,
				},
			})
			require.NoError(t, err)
			require.Empty(t, result.Actions)
		})

		t.Run("emits bullish and bearish crossovers in stable ascending action order", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			instrument := makeInstrument(t, fake)
			timeRange := makeRequestRange(t, fake)
			timeframe := domain.Timeframe5m
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fake.IntBetween(fastWindow+1, fastWindow+40)

			firstTime := timeRange.Start.Add(5 * time.Minute)
			secondTime := firstTime.Add(5 * time.Minute)
			thirdTime := secondTime.Add(5 * time.Minute)

			fastPoints := []domain.AnalyticsPoint{
				makePoint(
					t,
					firstTime,
					domain.TimeRange{Start: firstTime.Add(-10 * time.Minute), End: firstTime},
					9,
					domain.DataQualityValidated,
					10,
					randomWord(t, fake, "fast"),
				),
				makePoint(
					t,
					secondTime,
					domain.TimeRange{Start: secondTime.Add(-10 * time.Minute), End: secondTime},
					13,
					domain.DataQualityValidated,
					11,
					randomWord(t, fake, "fast"),
				),
				makePoint(
					t,
					thirdTime,
					domain.TimeRange{Start: thirdTime.Add(-10 * time.Minute), End: thirdTime},
					8,
					domain.DataQualityValidated,
					12,
					randomWord(t, fake, "fast"),
				),
			}
			slowPoints := []domain.AnalyticsPoint{
				makePoint(
					t,
					firstTime,
					domain.TimeRange{Start: firstTime.Add(-15 * time.Minute), End: firstTime},
					10,
					domain.DataQualityValidated,
					20,
					randomWord(t, fake, "slow"),
				),
				makePoint(
					t,
					secondTime,
					domain.TimeRange{Start: secondTime.Add(-15 * time.Minute), End: secondTime},
					11,
					domain.DataQualityValidated,
					21,
					randomWord(t, fake, "slow"),
				),
				makePoint(
					t,
					thirdTime,
					domain.TimeRange{Start: thirdTime.Add(-15 * time.Minute), End: thirdTime},
					9,
					domain.DataQualityValidated,
					22,
					randomWord(t, fake, "slow"),
				),
			}

			calculator.result = []domain.AnalyticsSeries{
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, fastWindow, fastPoints),
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, slowWindow, slowPoints),
			}

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				Instrument:   instrument,
				Timeframe:    timeframe,
				TimeRange:    timeRange,
				StrategyKind: domain.StrategyKindMovingAverageCrossover,
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fastWindow,
					SlowWindow: slowWindow,
				},
			})
			require.NoError(t, err)

			expectedStrategy, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
				Instrument: instrument,
				Timeframe:  timeframe,
				Kind:       domain.StrategyKindMovingAverageCrossover,
			})
			require.NoError(t, err)

			require.Equal(t, []domain.CandidateAction{
				makeAction(
					t,
					expectedStrategy,
					domain.CandidateActionKindLong,
					secondTime,
					domain.TimeRange{Start: firstTime.Add(-15 * time.Minute), End: secondTime},
					domain.DataQualityValidated,
				),
				makeAction(
					t,
					expectedStrategy,
					domain.CandidateActionKindShort,
					thirdTime,
					domain.TimeRange{Start: secondTime.Add(-15 * time.Minute), End: thirdTime},
					domain.DataQualityValidated,
				),
			}, result.Actions)
		})

		t.Run("does not emit an action when aligned points do not cross", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			instrument := makeInstrument(t, fake)
			timeRange := makeRequestRange(t, fake)
			timeframe := domain.Timeframe1h
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fake.IntBetween(fastWindow+1, fastWindow+40)

			firstTime := timeRange.Start.Add(1 * time.Hour)
			secondTime := firstTime.Add(1 * time.Hour)

			calculator.result = []domain.AnalyticsSeries{
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, fastWindow, []domain.AnalyticsPoint{
					makePoint(
						t,
						firstTime,
						domain.TimeRange{Start: firstTime.Add(-2 * time.Hour), End: firstTime},
						14,
						domain.DataQualityValidated,
						30,
						randomWord(t, fake, "fast"),
					),
					makePoint(
						t,
						secondTime,
						domain.TimeRange{Start: secondTime.Add(-2 * time.Hour), End: secondTime},
						15,
						domain.DataQualityValidated,
						31,
						randomWord(t, fake, "fast"),
					),
				}),
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, slowWindow, []domain.AnalyticsPoint{
					makePoint(
						t,
						firstTime,
						domain.TimeRange{Start: firstTime.Add(-3 * time.Hour), End: firstTime},
						10,
						domain.DataQualityValidated,
						40,
						randomWord(t, fake, "slow"),
					),
					makePoint(
						t,
						secondTime,
						domain.TimeRange{Start: secondTime.Add(-3 * time.Hour), End: secondTime},
						11,
						domain.DataQualityValidated,
						41,
						randomWord(t, fake, "slow"),
					),
				}),
			}

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				Instrument:   instrument,
				Timeframe:    timeframe,
				TimeRange:    timeRange,
				StrategyKind: domain.StrategyKindMovingAverageCrossover,
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fastWindow,
					SlowWindow: slowWindow,
				},
			})
			require.NoError(t, err)
			require.Empty(t, result.Actions)
		})

		t.Run("uses current aligned point time, combined input range, and propagated quality", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			instrument := makeInstrument(t, fake)
			timeRange := makeRequestRange(t, fake)
			timeframe := domain.Timeframe5m
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fake.IntBetween(fastWindow+1, fastWindow+40)

			firstTime := time.Date(2026, 3, 2, 10, 5, 0, 0, time.FixedZone(randomWord(t, fake, "zone"), 2*3600))
			secondTime := firstTime.Add(5 * time.Minute)
			fastPreviousRange := domain.TimeRange{Start: firstTime.Add(-10 * time.Minute), End: firstTime}
			slowPreviousRange := domain.TimeRange{Start: firstTime.Add(-25 * time.Minute), End: firstTime}
			fastCurrentRange := domain.TimeRange{Start: secondTime.Add(-10 * time.Minute), End: secondTime}
			slowCurrentRange := domain.TimeRange{Start: secondTime.Add(-20 * time.Minute), End: secondTime}

			calculator.result = []domain.AnalyticsSeries{
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, fastWindow, []domain.AnalyticsPoint{
					makePoint(
						t,
						firstTime,
						fastPreviousRange,
						9,
						domain.DataQualityValidated,
						50,
						randomWord(t, fake, "fast"),
					),
					makePoint(
						t,
						secondTime,
						fastCurrentRange,
						13,
						domain.DataQualityRaw,
						51,
						randomWord(t, fake, "fast"),
					),
				}),
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, slowWindow, []domain.AnalyticsPoint{
					makePoint(
						t,
						firstTime,
						slowPreviousRange,
						11,
						domain.DataQualityValidated,
						60,
						randomWord(t, fake, "slow"),
					),
					makePoint(
						t,
						secondTime,
						slowCurrentRange,
						12,
						domain.DataQualityValidated,
						61,
						randomWord(t, fake, "slow"),
					),
				}),
			}

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				Instrument:   instrument,
				Timeframe:    timeframe,
				TimeRange:    timeRange,
				StrategyKind: domain.StrategyKindMovingAverageCrossover,
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fastWindow,
					SlowWindow: slowWindow,
				},
			})
			require.NoError(t, err)
			require.Len(t, result.Actions, 1)
			require.Equal(t, secondTime.UTC(), result.Actions[0].DecisionTime.Time())
			require.Equal(t, domain.TimeRange{
				Start: slowPreviousRange.Start.UTC(),
				End:   slowCurrentRange.End.UTC(),
			}, result.Actions[0].InputRange)
			require.Equal(t, domain.DataQualityRaw, result.Actions[0].Quality)
		})

		t.Run("marks the action suspect when any contributing aligned point is suspect", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			instrument := makeInstrument(t, fake)
			timeRange := makeRequestRange(t, fake)
			timeframe := domain.Timeframe5m
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fake.IntBetween(fastWindow+1, fastWindow+40)

			firstTime := timeRange.Start.Add(5 * time.Minute)
			secondTime := firstTime.Add(5 * time.Minute)

			calculator.result = []domain.AnalyticsSeries{
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, fastWindow, []domain.AnalyticsPoint{
					makePoint(
						t,
						firstTime,
						domain.TimeRange{Start: firstTime.Add(-10 * time.Minute), End: firstTime},
						8,
						domain.DataQualitySuspect,
						70,
						randomWord(t, fake, "fast"),
					),
					makePoint(
						t,
						secondTime,
						domain.TimeRange{Start: secondTime.Add(-10 * time.Minute), End: secondTime},
						12,
						domain.DataQualityValidated,
						71,
						randomWord(t, fake, "fast"),
					),
				}),
				makeSeriesWithPoints(t, instrument, timeframe, timeRange, slowWindow, []domain.AnalyticsPoint{
					makePoint(
						t,
						firstTime,
						domain.TimeRange{Start: firstTime.Add(-15 * time.Minute), End: firstTime},
						10,
						domain.DataQualityValidated,
						80,
						randomWord(t, fake, "slow"),
					),
					makePoint(
						t,
						secondTime,
						domain.TimeRange{Start: secondTime.Add(-15 * time.Minute), End: secondTime},
						11,
						domain.DataQualityValidated,
						81,
						randomWord(t, fake, "slow"),
					),
				}),
			}

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				Instrument:   instrument,
				Timeframe:    timeframe,
				TimeRange:    timeRange,
				StrategyKind: domain.StrategyKindMovingAverageCrossover,
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fastWindow,
					SlowWindow: slowWindow,
				},
			})
			require.NoError(t, err)
			require.Len(t, result.Actions, 1)
			require.Equal(t, domain.DataQualitySuspect, result.Actions[0].Quality)
		})

		t.Run("rejects unsupported contributing analytics quality", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			calculator, deps := makeDeps(t)
			service, err := NewService(deps)
			require.NoError(t, err)

			instrument := makeInstrument(t, fake)
			timeRange := makeRequestRange(t, fake)
			timeframe := domain.Timeframe5m
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fake.IntBetween(fastWindow+1, fastWindow+40)

			firstTime := timeRange.Start.Add(5 * time.Minute)
			secondTime := firstTime.Add(5 * time.Minute)
			invalidQuality := domain.DataQuality(randomWord(t, fake, "invalid-quality"))

			slowInvalidPoint := makePoint(
				t,
				secondTime,
				domain.TimeRange{Start: secondTime.Add(-15 * time.Minute), End: secondTime},
				11,
				domain.DataQualityValidated,
				91,
				randomWord(t, fake, "slow"),
			)
			slowInvalidPoint.Quality = invalidQuality

			fastSeries := makeSeries(t, instrument, timeframe, timeRange, fastWindow)
			fastSeries.Points = []domain.AnalyticsPoint{
				makePoint(
					t,
					firstTime,
					domain.TimeRange{Start: firstTime.Add(-10 * time.Minute), End: firstTime},
					8,
					domain.DataQualityValidated,
					90,
					randomWord(t, fake, "fast"),
				),
				makePoint(
					t,
					secondTime,
					domain.TimeRange{Start: secondTime.Add(-10 * time.Minute), End: secondTime},
					12,
					domain.DataQualityValidated,
					92,
					randomWord(t, fake, "fast"),
				),
			}

			slowSeries := makeSeries(t, instrument, timeframe, timeRange, slowWindow)
			slowSeries.Points = []domain.AnalyticsPoint{
				makePoint(
					t,
					firstTime,
					domain.TimeRange{Start: firstTime.Add(-15 * time.Minute), End: firstTime},
					10,
					domain.DataQualityValidated,
					93,
					randomWord(t, fake, "slow"),
				),
				slowInvalidPoint,
			}

			calculator.result = []domain.AnalyticsSeries{fastSeries, slowSeries}

			result, err := service.Evaluate(t.Context(), EvaluateRequest{
				Instrument:   instrument,
				Timeframe:    timeframe,
				TimeRange:    timeRange,
				StrategyKind: domain.StrategyKindMovingAverageCrossover,
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fastWindow,
					SlowWindow: slowWindow,
				},
			})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(
				t,
				err,
				"strategy validation failed: unsupported analytics quality "+
					strconv.Quote(invalidQuality.String())+
					" for current slow aligned point",
			)
			require.Equal(t, EvaluateResult{}, result)
		})
	})
}
