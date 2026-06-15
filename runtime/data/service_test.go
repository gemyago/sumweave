package data

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type instrumentLookupCall struct {
	venue  domain.Venue
	symbol domain.Symbol
}

type fakeInstrumentStore struct {
	upserted    []domain.Instrument
	lookupCalls []instrumentLookupCall
	lookupValue *domain.Instrument
	upsertErr   error
	lookupErr   error
}

func (s *fakeInstrumentStore) UpsertInstrument(
	_ context.Context,
	instrument domain.Instrument,
) (domain.Instrument, error) {
	s.upserted = append(s.upserted, instrument)
	if s.upsertErr != nil {
		return domain.Instrument{}, s.upsertErr
	}

	return instrument, nil
}

func (s *fakeInstrumentStore) LookupInstrument(
	_ context.Context,
	venue domain.Venue,
	symbol domain.Symbol,
) (*domain.Instrument, error) {
	s.lookupCalls = append(s.lookupCalls, instrumentLookupCall{
		venue:  venue,
		symbol: symbol,
	})
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}

	return s.lookupValue, nil
}

type fakeCandleStore struct {
	availabilityQueried []CandleAvailabilityListQuery
	availabilityValue   CandleAvailabilityListResult
	upserted            []domain.Candle
	batchUpserts        []batchCandleUpsertCall
	queried             []candleQueryCall
	replayed            []candleQueryCall
	queryValue          []domain.Candle
	replayValue         []ReplayCandle
	availabilityErr     error
	upsertErr           error
	queryErr            error
	replayErr           error
}

func (s *fakeCandleStore) ListCandleAvailability(
	_ context.Context,
	query CandleAvailabilityListQuery,
) (CandleAvailabilityListResult, error) {
	s.availabilityQueried = append(s.availabilityQueried, query)
	if s.availabilityErr != nil {
		return CandleAvailabilityListResult{}, s.availabilityErr
	}

	return s.availabilityValue, nil
}

func (s *fakeCandleStore) UpsertCandle(
	_ context.Context,
	candle domain.Candle,
) (domain.Candle, error) {
	s.upserted = append(s.upserted, candle)
	if s.upsertErr != nil {
		return domain.Candle{}, s.upsertErr
	}

	return candle, nil
}

func (s *fakeCandleStore) UpsertCandleForDataBatch(
	_ context.Context,
	batchID string,
	candle domain.Candle,
) (domain.Candle, error) {
	s.batchUpserts = append(s.batchUpserts, batchCandleUpsertCall{batchID: batchID, candle: candle})
	if s.upsertErr != nil {
		return domain.Candle{}, s.upsertErr
	}

	return candle, nil
}

func (s *fakeCandleStore) QueryCandles(
	_ context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]domain.Candle, error) {
	s.queried = append(s.queried, candleQueryCall{
		instrument: instrument,
		timeframe:  timeframe,
		timeRange:  timeRange,
	})
	if s.queryErr != nil {
		return nil, s.queryErr
	}

	return s.queryValue, nil
}

func (s *fakeCandleStore) ReplayCandles(
	_ context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]ReplayCandle, error) {
	s.replayed = append(s.replayed, candleQueryCall{
		instrument: instrument,
		timeframe:  timeframe,
		timeRange:  timeRange,
	})
	if s.replayErr != nil {
		return nil, s.replayErr
	}

	return s.replayValue, nil
}

type fakeTradeStore struct {
	upserted     []domain.Trade
	batchUpserts []batchTradeUpsertCall
	queried      []tradeQueryCall
	replayed     []tradeQueryCall
	queryValue   []domain.Trade
	replayValue  []ReplayTrade
	upsertErr    error
	queryErr     error
	replayErr    error
}

func (s *fakeTradeStore) UpsertTrade(
	_ context.Context,
	trade domain.Trade,
) (domain.Trade, error) {
	s.upserted = append(s.upserted, trade)
	if s.upsertErr != nil {
		return domain.Trade{}, s.upsertErr
	}

	return trade, nil
}

func (s *fakeTradeStore) UpsertTradeForDataBatch(
	_ context.Context,
	batchID string,
	trade domain.Trade,
) (domain.Trade, error) {
	s.batchUpserts = append(s.batchUpserts, batchTradeUpsertCall{batchID: batchID, trade: trade})
	if s.upsertErr != nil {
		return domain.Trade{}, s.upsertErr
	}

	return trade, nil
}

func (s *fakeTradeStore) QueryTrades(
	_ context.Context,
	instrument domain.Instrument,
	timeRange domain.TimeRange,
) ([]domain.Trade, error) {
	s.queried = append(s.queried, tradeQueryCall{
		instrument: instrument,
		timeRange:  timeRange,
	})
	if s.queryErr != nil {
		return nil, s.queryErr
	}

	return s.queryValue, nil
}

func (s *fakeTradeStore) ReplayTrades(
	_ context.Context,
	instrument domain.Instrument,
	timeRange domain.TimeRange,
) ([]ReplayTrade, error) {
	s.replayed = append(s.replayed, tradeQueryCall{
		instrument: instrument,
		timeRange:  timeRange,
	})
	if s.replayErr != nil {
		return nil, s.replayErr
	}

	return s.replayValue, nil
}

type candleQueryCall struct {
	instrument domain.Instrument
	timeframe  domain.Timeframe
	timeRange  domain.TimeRange
}

type batchCandleUpsertCall struct {
	batchID string
	candle  domain.Candle
}

type tradeQueryCall struct {
	instrument domain.Instrument
	timeRange  domain.TimeRange
}

type batchTradeUpsertCall struct {
	batchID string
	trade   domain.Trade
}

type mockDeps struct {
	instrumentStore *fakeInstrumentStore
	candleStore     *fakeCandleStore
	tradeStore      *fakeTradeStore
}

func TestServices(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	makeMockDeps := func() mockDeps {
		return mockDeps{
			instrumentStore: &fakeInstrumentStore{},
			candleStore:     &fakeCandleStore{},
			tradeStore:      &fakeTradeStore{},
		}
	}

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	randomTime := func() time.Time {
		return time.Date(
			fake.IntBetween(2021, 2032),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone(randomWord("zone"), fake.IntBetween(-11, 12)*3600),
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

	makeCandle := func() domain.Candle {
		start := randomTime()
		return domain.Candle{
			Instrument: makeInstrument(),
			Timeframe:  domain.Timeframe(" 1M "),
			TimeRange: domain.TimeRange{
				Start: start,
				End:   start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute),
			},
			Open:    fake.Float64(2, 1, 1000),
			High:    fake.Float64(2, 1, 1000),
			Low:     fake.Float64(2, 0, 1000),
			Close:   fake.Float64(2, 1, 1000),
			Volume:  fake.Float64(4, 0, 10000),
			Quality: domain.DataQuality("  VALIDATED "),
			Provenance: domain.SourceProvenance{
				Source:   "  " + randomWord("source") + "  ",
				RecordID: "  " + randomWord("record") + "  ",
			},
		}
	}

	makeTrade := func() domain.Trade {
		return domain.Trade{
			Instrument: makeInstrument(),
			EventTime:  randomTime(),
			Price:      fake.Float64(4, 1, 100000),
			Size:       fake.Float64(4, 0, 100000),
			Quality:    domain.DataQuality("  RAW "),
			Provenance: domain.SourceProvenance{
				Source:   "  " + randomWord("source") + "  ",
				RecordID: "  " + randomWord("record") + "  ",
			},
		}
	}

	t.Run("NewIngestionService", func(t *testing.T) {
		t.Parallel()

		t.Run("requires all stores", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()

			_, err := NewIngestionService(IngestionServiceDeps{
				CandleStore: deps.candleStore,
				TradeStore:  deps.tradeStore,
			})
			require.Error(t, err)

			_, err = NewIngestionService(IngestionServiceDeps{
				InstrumentStore: deps.instrumentStore,
				TradeStore:      deps.tradeStore,
			})
			require.Error(t, err)

			_, err = NewIngestionService(IngestionServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
			})
			require.Error(t, err)
		})
	})

	t.Run("IngestionService", func(t *testing.T) {
		t.Parallel()

		t.Run("upserts a valid instrument", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			svc, err := NewIngestionService(IngestionServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			instrument, err := svc.UpsertInstrument(t.Context(), makeInstrument())
			require.NoError(t, err)
			require.Len(t, deps.instrumentStore.upserted, 1)
			require.Equal(t, deps.instrumentStore.upserted[0], instrument)
			require.Equal(t, strings.TrimSpace(instrument.Venue.String()), instrument.Venue.String())
			require.Equal(t, strings.TrimSpace(instrument.Symbol.String()), instrument.Symbol.String())
			require.Equal(
				t,
				strings.ToLower(strings.TrimSpace(instrument.AssetClass.String())),
				instrument.AssetClass.String(),
			)
		})

		t.Run("persists a valid candle after instrument upsert", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			svc, err := NewIngestionService(IngestionServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			candle, err := svc.IngestCandle(t.Context(), makeCandle())
			require.NoError(t, err)

			require.Len(t, deps.instrumentStore.upserted, 1)
			require.Len(t, deps.candleStore.upserted, 1)
			require.Empty(t, deps.tradeStore.upserted)
			require.Equal(t, deps.candleStore.upserted[0], candle)
			require.Equal(t, time.UTC, candle.TimeRange.Start.Location())
			require.Equal(t, time.UTC, candle.TimeRange.End.Location())
			require.Equal(t, deps.instrumentStore.upserted[0], candle.Instrument)
			require.Equal(t, strings.TrimSpace(candle.Provenance.Source), candle.Provenance.Source)
			require.Equal(t, strings.TrimSpace(candle.Provenance.RecordID), candle.Provenance.RecordID)
		})

		t.Run("persists a valid candle linked to a data batch", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			svc, err := NewIngestionService(IngestionServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			batchID := "  " + randomWord("batch") + "  "
			candle, err := svc.IngestCandleForDataBatch(t.Context(), batchID, makeCandle())
			require.NoError(t, err)

			require.Len(t, deps.instrumentStore.upserted, 1)
			require.Empty(t, deps.candleStore.upserted)
			require.Len(t, deps.candleStore.batchUpserts, 1)
			require.Equal(t, strings.TrimSpace(batchID), deps.candleStore.batchUpserts[0].batchID)
			require.Equal(t, deps.candleStore.batchUpserts[0].candle, candle)
		})

		t.Run("persists a valid trade with UTC normalization", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			svc, err := NewIngestionService(IngestionServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			trade, err := svc.IngestTrade(t.Context(), makeTrade())
			require.NoError(t, err)

			require.Len(t, deps.instrumentStore.upserted, 1)
			require.Len(t, deps.tradeStore.upserted, 1)
			require.Empty(t, deps.candleStore.upserted)
			require.Equal(t, deps.tradeStore.upserted[0], trade)
			require.Equal(t, time.UTC, trade.EventTime.Location())
			require.Equal(t, deps.instrumentStore.upserted[0], trade.Instrument)
		})

		t.Run("persists a valid trade linked to a data batch", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			svc, err := NewIngestionService(IngestionServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			batchID := "  " + randomWord("batch") + "  "
			trade, err := svc.IngestTradeForDataBatch(t.Context(), batchID, makeTrade())
			require.NoError(t, err)

			require.Len(t, deps.instrumentStore.upserted, 1)
			require.Empty(t, deps.tradeStore.upserted)
			require.Len(t, deps.tradeStore.batchUpserts, 1)
			require.Equal(t, strings.TrimSpace(batchID), deps.tradeStore.batchUpserts[0].batchID)
			require.Equal(t, deps.tradeStore.batchUpserts[0].trade, trade)
		})

		t.Run("rejects invalid inputs without persistence", func(t *testing.T) {
			t.Parallel()

			testCases := []struct {
				name string
				run  func(*IngestionService) error
			}{
				{
					name: "instrument missing symbol",
					run: func(svc *IngestionService) error {
						instrument := makeInstrument()
						instrument.Symbol = ""
						_, err := svc.UpsertInstrument(t.Context(), instrument)
						return err
					},
				},
				{
					name: "candle negative close",
					run: func(svc *IngestionService) error {
						candle := makeCandle()
						candle.Close = -fake.Float64(2, 1, 1000)
						_, err := svc.IngestCandle(t.Context(), candle)
						return err
					},
				},
				{
					name: "candle invalid time range",
					run: func(svc *IngestionService) error {
						candle := makeCandle()
						candle.TimeRange.End = candle.TimeRange.Start
						_, err := svc.IngestCandle(t.Context(), candle)
						return err
					},
				},
				{
					name: "trade missing provenance source",
					run: func(svc *IngestionService) error {
						trade := makeTrade()
						trade.Provenance.Source = "   "
						_, err := svc.IngestTrade(t.Context(), trade)
						return err
					},
				},
				{
					name: "trade negative size",
					run: func(svc *IngestionService) error {
						trade := makeTrade()
						trade.Size = -fake.Float64(2, 1, 1000)
						_, err := svc.IngestTrade(t.Context(), trade)
						return err
					},
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					deps := makeMockDeps()
					svc, err := NewIngestionService(IngestionServiceDeps{
						InstrumentStore: deps.instrumentStore,
						CandleStore:     deps.candleStore,
						TradeStore:      deps.tradeStore,
					})
					require.NoError(t, err)

					err = testCase.run(svc)
					require.Error(t, err)
					require.ErrorIs(t, err, ErrValidation)
					require.Empty(t, deps.instrumentStore.upserted)
					require.Empty(t, deps.candleStore.upserted)
					require.Empty(t, deps.tradeStore.upserted)
				})
			}
		})

		t.Run("wraps store failures", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			deps.instrumentStore.upsertErr = errors.New(randomWord("instrument-store"))
			svc, err := NewIngestionService(IngestionServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			_, err = svc.IngestTrade(t.Context(), makeTrade())
			require.Error(t, err)
			require.Contains(t, err.Error(), "upsert trade instrument")
			require.Empty(t, deps.tradeStore.upserted)
		})
	})

	t.Run("ReadService", func(t *testing.T) {
		t.Parallel()

		t.Run("requires an instrument store", func(t *testing.T) {
			t.Parallel()

			_, err := NewReadService(ReadServiceDeps{})
			require.Error(t, err)
		})

		t.Run("requires candle and trade stores", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()

			_, err := NewReadService(ReadServiceDeps{
				InstrumentStore: deps.instrumentStore,
				TradeStore:      deps.tradeStore,
			})
			require.Error(t, err)

			_, err = NewReadService(ReadServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
			})
			require.Error(t, err)
		})

		t.Run("validates and normalizes lookup input", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			instrument := makeInstrument()
			expectedInstrument, err := domain.NewInstrument(domain.InstrumentParams{
				Venue:      domain.Venue(strings.TrimSpace(instrument.Venue.String())),
				Symbol:     domain.Symbol(strings.TrimSpace(instrument.Symbol.String())),
				AssetClass: domain.AssetClassCrypto,
				Active:     true,
			})
			require.NoError(t, err)
			deps.instrumentStore.lookupValue = &expectedInstrument

			readSvc, err := NewReadService(ReadServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			actualInstrument, err := readSvc.LookupInstrument(
				t.Context(),
				domain.Venue("  "+expectedInstrument.Venue.String()+"  "),
				domain.Symbol("  "+expectedInstrument.Symbol.String()+"  "),
			)
			require.NoError(t, err)
			require.Equal(t, &expectedInstrument, actualInstrument)
			require.Len(t, deps.instrumentStore.lookupCalls, 1)
			require.Equal(t, expectedInstrument.Venue, deps.instrumentStore.lookupCalls[0].venue)
			require.Equal(t, expectedInstrument.Symbol, deps.instrumentStore.lookupCalls[0].symbol)
		})

		t.Run("queries candles with normalized half-open inputs", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			expectedInstrument, err := domain.NewInstrument(domain.InstrumentParams{
				Venue:      domain.Venue(randomWord("venue")),
				Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
				AssetClass: domain.AssetClassCrypto,
				Active:     fake.Bool(),
			})
			require.NoError(t, err)

			start := randomTime()
			expectedRange, err := domain.NewTimeRange(start, start.Add(5*time.Minute))
			require.NoError(t, err)

			expectedCandle, err := domain.NewCandle(domain.CandleParams{
				Instrument: expectedInstrument,
				Timeframe:  domain.Timeframe1m,
				TimeRange:  expectedRange,
				Open:       fake.Float64(2, 1, 1000),
				High:       fake.Float64(2, 1, 1000),
				Low:        fake.Float64(2, 0, 1000),
				Close:      fake.Float64(2, 1, 1000),
				Volume:     fake.Float64(2, 0, 1000),
				Quality:    domain.DataQualityValidated,
				Provenance: domain.SourceProvenance{Source: randomWord("source"), RecordID: randomWord("record")},
			})
			require.NoError(t, err)
			deps.candleStore.queryValue = []domain.Candle{expectedCandle}

			readSvc, err := NewReadService(ReadServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			got, err := readSvc.QueryCandles(
				t.Context(),
				domain.Instrument{
					Venue:  domain.Venue("  " + expectedInstrument.Venue.String() + "  "),
					Symbol: domain.Symbol("  " + expectedInstrument.Symbol.String() + "  "),
				},
				domain.Timeframe(" 1M "),
				domain.TimeRange{
					Start: start.In(time.FixedZone(randomWord("zone"), 2*3600)),
					End:   start.Add(5 * time.Minute).In(time.FixedZone(randomWord("zone"), -3*3600)),
				},
			)
			require.NoError(t, err)
			require.Equal(t, []domain.Candle{expectedCandle}, got)
			require.Len(t, deps.candleStore.queried, 1)
			require.Equal(t, expectedInstrument.Venue, deps.candleStore.queried[0].instrument.Venue)
			require.Equal(t, expectedInstrument.Symbol, deps.candleStore.queried[0].instrument.Symbol)
			require.Zero(t, deps.candleStore.queried[0].instrument.AssetClass)
			require.Equal(t, domain.Timeframe1m, deps.candleStore.queried[0].timeframe)
			require.Equal(t, expectedRange, deps.candleStore.queried[0].timeRange)
		})

		t.Run("delegates candle availability reads with canonical filters", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			start := randomTime().UTC().Truncate(time.Minute)
			deps.candleStore.availabilityValue = CandleAvailabilityListResult{
				Items: []CandleAvailabilityItem{{
					Venue:      domain.Venue(randomWord("venue")),
					Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
					AssetClass: domain.AssetClassCrypto,
					Timeframes: []CandleAvailabilityTimeframeSummary{{
						Timeframe: domain.Timeframe1m,
						StartAt:   start,
						EndAt:     start.Add(time.Minute),
						Count:     1,
					}},
					DefaultSlice: CandleAvailabilityDefaultSlice{
						Timeframe: domain.Timeframe1m,
						StartAt:   start,
						EndAt:     start.Add(time.Minute),
					},
				}},
			}

			readSvc, err := NewReadService(ReadServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			result, err := readSvc.ListCandleAvailability(t.Context(), CandleAvailabilityListQuery{
				Venue:      domain.Venue("  " + randomWord("venue") + "  "),
				Symbol:     domain.Symbol("  " + strings.ToUpper(randomWord("symbol")) + "  "),
				AssetClass: domain.AssetClass("  CRYPTO  "),
				Limit:      1,
				Cursor: encodeCandleAvailabilityListCursor(
					start.Add(2*time.Minute),
					domain.Venue(randomWord("cursor-venue")),
					domain.Symbol(strings.ToUpper(randomWord("cursor-symbol"))),
					domain.AssetClassCrypto,
				),
			})
			require.NoError(t, err)
			require.Equal(t, deps.candleStore.availabilityValue, result)
			require.Len(t, deps.candleStore.availabilityQueried, 1)
			require.Equal(t, 1, deps.candleStore.availabilityQueried[0].Limit)
			require.Equal(
				t,
				strings.TrimSpace(deps.candleStore.availabilityQueried[0].Venue.String()),
				deps.candleStore.availabilityQueried[0].Venue.String(),
			)
			require.Equal(
				t,
				strings.TrimSpace(deps.candleStore.availabilityQueried[0].Symbol.String()),
				deps.candleStore.availabilityQueried[0].Symbol.String(),
			)
			require.Equal(t, domain.AssetClassCrypto, deps.candleStore.availabilityQueried[0].AssetClass)
		})

		t.Run("queries trades with normalized half-open inputs", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			expectedInstrument, err := domain.NewInstrument(domain.InstrumentParams{
				Venue:      domain.Venue(randomWord("venue")),
				Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
				AssetClass: domain.AssetClassCrypto,
				Active:     fake.Bool(),
			})
			require.NoError(t, err)

			start := randomTime()
			expectedRange, err := domain.NewTimeRange(start, start.Add(3*time.Minute))
			require.NoError(t, err)

			expectedTrade, err := domain.NewTrade(domain.TradeParams{
				Instrument: expectedInstrument,
				EventTime:  start.Add(time.Minute),
				Price:      fake.Float64(4, 1, 100000),
				Size:       fake.Float64(4, 0, 100000),
				Quality:    domain.DataQualitySuspect,
				Provenance: domain.SourceProvenance{Source: randomWord("source"), RecordID: randomWord("record")},
			})
			require.NoError(t, err)
			deps.tradeStore.queryValue = []domain.Trade{expectedTrade}

			readSvc, err := NewReadService(ReadServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			got, err := readSvc.QueryTrades(
				t.Context(),
				domain.Instrument{
					Venue:  domain.Venue("  " + expectedInstrument.Venue.String() + "  "),
					Symbol: domain.Symbol("  " + expectedInstrument.Symbol.String() + "  "),
				},
				domain.TimeRange{
					Start: start.In(time.FixedZone(randomWord("zone"), 4*3600)),
					End:   start.Add(3 * time.Minute).In(time.FixedZone(randomWord("zone"), -7*3600)),
				},
			)
			require.NoError(t, err)
			require.Equal(t, []domain.Trade{expectedTrade}, got)
			require.Len(t, deps.tradeStore.queried, 1)
			require.Equal(t, expectedInstrument.Venue, deps.tradeStore.queried[0].instrument.Venue)
			require.Equal(t, expectedInstrument.Symbol, deps.tradeStore.queried[0].instrument.Symbol)
			require.Equal(t, expectedRange, deps.tradeStore.queried[0].timeRange)
		})

		t.Run("replays candles with stable identities", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			expectedInstrument, err := domain.NewInstrument(domain.InstrumentParams{
				Venue:      domain.Venue(randomWord("venue")),
				Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
				AssetClass: domain.AssetClassCrypto,
				Active:     fake.Bool(),
			})
			require.NoError(t, err)

			start := randomTime()
			expectedRange, err := domain.NewTimeRange(start, start.Add(time.Minute))
			require.NoError(t, err)

			expectedCandle, err := domain.NewCandle(domain.CandleParams{
				Instrument: expectedInstrument,
				Timeframe:  domain.Timeframe1m,
				TimeRange:  expectedRange,
				Open:       fake.Float64(2, 1, 1000),
				High:       fake.Float64(2, 1, 1000),
				Low:        fake.Float64(2, 0, 1000),
				Close:      fake.Float64(2, 1, 1000),
				Volume:     fake.Float64(2, 0, 1000),
				Quality:    domain.DataQualityRaw,
				Provenance: domain.SourceProvenance{Source: randomWord("source"), RecordID: randomWord("record")},
			})
			require.NoError(t, err)
			deps.candleStore.replayValue = []ReplayCandle{{
				Identity: uint64(fake.IntBetween(1, 1_000_000)),
				Candle:   expectedCandle,
			}}

			readSvc, err := NewReadService(ReadServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			got, err := readSvc.ReplayCandles(
				t.Context(),
				domain.Instrument{
					Venue:  domain.Venue("  " + expectedInstrument.Venue.String() + "  "),
					Symbol: domain.Symbol("  " + expectedInstrument.Symbol.String() + "  "),
				},
				domain.Timeframe(" 1M "),
				domain.TimeRange{
					Start: expectedRange.Start.In(time.FixedZone(randomWord("zone"), -5*3600)),
					End:   expectedRange.End.In(time.FixedZone(randomWord("zone"), 6*3600)),
				},
			)
			require.NoError(t, err)
			require.Equal(t, deps.candleStore.replayValue, got)
			require.Len(t, deps.candleStore.replayed, 1)
			require.Equal(t, expectedRange, deps.candleStore.replayed[0].timeRange)
		})

		t.Run("replays trades with stable identities", func(t *testing.T) {
			t.Parallel()

			deps := makeMockDeps()
			expectedInstrument, err := domain.NewInstrument(domain.InstrumentParams{
				Venue:      domain.Venue(randomWord("venue")),
				Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
				AssetClass: domain.AssetClassCrypto,
				Active:     fake.Bool(),
			})
			require.NoError(t, err)

			start := randomTime()
			expectedRange, err := domain.NewTimeRange(start, start.Add(2*time.Minute))
			require.NoError(t, err)

			expectedTrade, err := domain.NewTrade(domain.TradeParams{
				Instrument: expectedInstrument,
				EventTime:  start.Add(30 * time.Second),
				Price:      fake.Float64(4, 1, 100000),
				Size:       fake.Float64(4, 0, 100000),
				Quality:    domain.DataQualityValidated,
				Provenance: domain.SourceProvenance{Source: randomWord("source"), RecordID: randomWord("record")},
			})
			require.NoError(t, err)
			deps.tradeStore.replayValue = []ReplayTrade{{
				Identity: uint64(fake.IntBetween(1, 1_000_000)),
				Trade:    expectedTrade,
			}}

			readSvc, err := NewReadService(ReadServiceDeps{
				InstrumentStore: deps.instrumentStore,
				CandleStore:     deps.candleStore,
				TradeStore:      deps.tradeStore,
			})
			require.NoError(t, err)

			got, err := readSvc.ReplayTrades(
				t.Context(),
				domain.Instrument{
					Venue:  domain.Venue("  " + expectedInstrument.Venue.String() + "  "),
					Symbol: domain.Symbol("  " + expectedInstrument.Symbol.String() + "  "),
				},
				domain.TimeRange{
					Start: expectedRange.Start.In(time.FixedZone(randomWord("zone"), 8*3600)),
					End:   expectedRange.End.In(time.FixedZone(randomWord("zone"), -2*3600)),
				},
			)
			require.NoError(t, err)
			require.Equal(t, deps.tradeStore.replayValue, got)
			require.Len(t, deps.tradeStore.replayed, 1)
			require.Equal(t, expectedRange, deps.tradeStore.replayed[0].timeRange)
		})

		t.Run("rejects invalid read inputs without store calls", func(t *testing.T) {
			t.Parallel()

			testCases := []struct {
				name string
				run  func(*ReadService) error
			}{
				{
					name: "list candle availability invalid limit",
					run: func(svc *ReadService) error {
						_, err := svc.ListCandleAvailability(t.Context(), CandleAvailabilityListQuery{Limit: -1})
						return err
					},
				},
				{
					name: "query candles missing instrument venue",
					run: func(svc *ReadService) error {
						_, err := svc.QueryCandles(
							t.Context(),
							domain.Instrument{Symbol: domain.Symbol(randomWord("symbol"))},
							domain.Timeframe1m,
							domain.TimeRange{Start: randomTime(), End: randomTime().Add(time.Minute)},
						)
						return err
					},
				},
				{
					name: "query candles missing timeframe",
					run: func(svc *ReadService) error {
						start := randomTime()
						_, err := svc.QueryCandles(
							t.Context(),
							domain.Instrument{
								Venue:  domain.Venue(randomWord("venue")),
								Symbol: domain.Symbol(randomWord("symbol")),
							},
							"",
							domain.TimeRange{Start: start, End: start.Add(time.Minute)},
						)
						return err
					},
				},
				{
					name: "query trades invalid range",
					run: func(svc *ReadService) error {
						start := randomTime()
						_, err := svc.QueryTrades(
							t.Context(),
							domain.Instrument{
								Venue:  domain.Venue(randomWord("venue")),
								Symbol: domain.Symbol(randomWord("symbol")),
							},
							domain.TimeRange{Start: start, End: start},
						)
						return err
					},
				},
				{
					name: "replay trades missing symbol",
					run: func(svc *ReadService) error {
						start := randomTime()
						_, err := svc.ReplayTrades(
							t.Context(),
							domain.Instrument{Venue: domain.Venue(randomWord("venue"))},
							domain.TimeRange{Start: start, End: start.Add(time.Minute)},
						)
						return err
					},
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					deps := makeMockDeps()
					readSvc, err := NewReadService(ReadServiceDeps{
						InstrumentStore: deps.instrumentStore,
						CandleStore:     deps.candleStore,
						TradeStore:      deps.tradeStore,
					})
					require.NoError(t, err)

					err = testCase.run(readSvc)
					require.Error(t, err)
					require.ErrorIs(t, err, ErrValidation)
					require.Empty(t, deps.candleStore.queried)
					require.Empty(t, deps.candleStore.availabilityQueried)
					require.Empty(t, deps.candleStore.replayed)
					require.Empty(t, deps.tradeStore.queried)
					require.Empty(t, deps.tradeStore.replayed)
				})
			}
		})
	})
}
