package venueedge

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type fakeVenueReadResult struct {
	instrumentResult InstrumentReadResult
	candleResult     CandleReadResult
	tradeResult      TradeReadResult
	instrumentErr    error
	candleErr        error
	tradeErr         error

	instrumentReads []InstrumentReadRequest
	candleReads     []CandleReadRequest
	tradeReads      []TradeReadRequest
}

func (f *fakeVenueReadResult) ReadInstruments(
	_ context.Context,
	request InstrumentReadRequest,
) (InstrumentReadResult, error) {
	f.instrumentReads = append(f.instrumentReads, request)
	if f.instrumentErr != nil {
		return InstrumentReadResult{}, f.instrumentErr
	}
	return f.instrumentResult, nil
}

func (f *fakeVenueReadResult) ReadCandles(
	_ context.Context,
	request CandleReadRequest,
) (CandleReadResult, error) {
	f.candleReads = append(f.candleReads, request)
	if f.candleErr != nil {
		return CandleReadResult{}, f.candleErr
	}
	return f.candleResult, nil
}

func (f *fakeVenueReadResult) ReadTrades(
	_ context.Context,
	request TradeReadRequest,
) (TradeReadResult, error) {
	f.tradeReads = append(f.tradeReads, request)
	if f.tradeErr != nil {
		return TradeReadResult{}, f.tradeErr
	}
	return f.tradeResult, nil
}

type fakeIngestionSink struct {
	upsertInstrumentErr error
	upsertCandleErr     error
	upsertTradeErr      error

	upsertInstrumentResultHook func(instrument domain.Instrument) domain.Instrument
	upsertCandleResultHook     func(candle domain.Candle) domain.Candle
	upsertTradeResultHook      func(trade domain.Trade) domain.Trade

	upsertedInstruments []domain.Instrument
	upsertedCandles     []domain.Candle
	upsertedTrades      []domain.Trade
}

func (s *fakeIngestionSink) UpsertInstrument(
	_ context.Context,
	instrument domain.Instrument,
) (domain.Instrument, error) {
	s.upsertedInstruments = append(s.upsertedInstruments, instrument)
	if s.upsertInstrumentErr != nil {
		return domain.Instrument{}, s.upsertInstrumentErr
	}
	if s.upsertInstrumentResultHook != nil {
		instrument = s.upsertInstrumentResultHook(instrument)
	}
	return instrument, nil
}

func (s *fakeIngestionSink) IngestCandle(
	_ context.Context,
	candle domain.Candle,
) (domain.Candle, error) {
	s.upsertedCandles = append(s.upsertedCandles, candle)
	if s.upsertCandleErr != nil {
		return domain.Candle{}, s.upsertCandleErr
	}
	if s.upsertCandleResultHook != nil {
		candle = s.upsertCandleResultHook(candle)
	}
	return candle, nil
}

func (s *fakeIngestionSink) IngestTrade(
	_ context.Context,
	trade domain.Trade,
) (domain.Trade, error) {
	s.upsertedTrades = append(s.upsertedTrades, trade)
	if s.upsertTradeErr != nil {
		return domain.Trade{}, s.upsertTradeErr
	}
	if s.upsertTradeResultHook != nil {
		trade = s.upsertTradeResultHook(trade)
	}
	return trade, nil
}

type rawPayloadLineageCall struct {
	rawPayloadID string
	instrument   domain.Instrument
	candle       domain.Candle
	trade        domain.Trade
}

type fakeRawPayloadLineageSink struct {
	linkInstrumentErr error
	linkCandleErr     error
	linkTradeErr      error

	instrumentLinks []rawPayloadLineageCall
	candleLinks     []rawPayloadLineageCall
	tradeLinks      []rawPayloadLineageCall
}

func (s *fakeRawPayloadLineageSink) LinkRawPayloadToInstrument(
	_ context.Context,
	rawPayloadID string,
	instrument domain.Instrument,
) error {
	s.instrumentLinks = append(
		s.instrumentLinks,
		rawPayloadLineageCall{rawPayloadID: rawPayloadID, instrument: instrument},
	)
	return s.linkInstrumentErr
}

func (s *fakeRawPayloadLineageSink) LinkRawPayloadToCandle(
	_ context.Context,
	rawPayloadID string,
	candle domain.Candle,
) error {
	s.candleLinks = append(
		s.candleLinks,
		rawPayloadLineageCall{rawPayloadID: rawPayloadID, candle: candle},
	)
	return s.linkCandleErr
}

func (s *fakeRawPayloadLineageSink) LinkRawPayloadToTrade(
	_ context.Context,
	rawPayloadID string,
	trade domain.Trade,
) error {
	s.tradeLinks = append(
		s.tradeLinks,
		rawPayloadLineageCall{rawPayloadID: rawPayloadID, trade: trade},
	)
	return s.linkTradeErr
}

func TestIngestionFlow(t *testing.T) {
	t.Parallel()

	fake := faker.New()

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

	fixedCandleRequest := func() CandleReadRequest {
		start := time.Date(2024, time.January, 8, 0, 0, 0, 0, time.UTC)
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

	fixedTradeRequest := func() TradeReadRequest {
		candleRequest := fixedCandleRequest()
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

	t.Run("links raw payload IDs for ingested instruments", func(t *testing.T) {
		t.Parallel()

		venue := &fakeVenueReadResult{
			instrumentResult: InstrumentReadResult{
				Instruments: []domain.Instrument{makeInstrument()},
				Metadata:    ReadResultMetadata{RawPayloadIDs: []string{"rp-instrument-1", "rp-instrument-2"}},
			},
		}

		sink := &fakeIngestionSink{}
		lineageSink := &fakeRawPayloadLineageSink{}

		flow, err := NewIngestionFlow(sink)
		require.NoError(t, err)

		flow.WithRawPayloadLineage(lineageSink)

		request, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:    domain.Venue("sandbox-int"),
			PageSize: 10,
		})
		require.NoError(t, err)

		persisted, err := flow.IngestInstruments(t.Context(), venue, request)
		require.NoError(t, err)
		require.Len(t, persisted, 1)

		require.Len(t, sink.upsertedInstruments, 1)
		require.Len(t, lineageSink.instrumentLinks, 2)
		require.Equal(t, persisted[0], lineageSink.instrumentLinks[0].instrument)
		require.Equal(t, persisted[0], lineageSink.instrumentLinks[1].instrument)
		require.Equal(t, "rp-instrument-1", lineageSink.instrumentLinks[0].rawPayloadID)
		require.Equal(t, "rp-instrument-2", lineageSink.instrumentLinks[1].rawPayloadID)
	})

	t.Run("does not link records when no raw payload IDs are present", func(t *testing.T) {
		t.Parallel()

		venue := &fakeVenueReadResult{
			instrumentResult: InstrumentReadResult{
				Instruments: []domain.Instrument{makeInstrument()},
			},
		}

		sink := &fakeIngestionSink{}
		lineageSink := &fakeRawPayloadLineageSink{}

		flow, err := NewIngestionFlow(sink)
		require.NoError(t, err)

		flow.WithRawPayloadLineage(lineageSink)

		request, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:    domain.Venue("sandbox-int"),
			PageSize: 10,
		})
		require.NoError(t, err)

		_, err = flow.IngestInstruments(t.Context(), venue, request)
		require.NoError(t, err)

		require.Empty(t, lineageSink.instrumentLinks)
		require.Empty(t, lineageSink.candleLinks)
		require.Empty(t, lineageSink.tradeLinks)
	})

	t.Run("links raw payload IDs for ingested candles", func(t *testing.T) {
		t.Parallel()

		instrument := makeInstrument()
		timeRange, err := domain.NewTimeRange(
			time.Date(2024, time.January, 8, 1, 0, 0, 0, time.UTC),
			time.Date(2024, time.January, 8, 1, 5, 0, 0, time.UTC),
		)
		require.NoError(t, err)

		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  timeRange,
			Open:       1,
			High:       1,
			Low:        1,
			Close:      1,
			Volume:     1,
			Quality:    domain.DataQualityRaw,
			Provenance: domain.SourceProvenance{Source: "candle-source", RecordID: ""},
		})
		require.NoError(t, err)

		venue := &fakeVenueReadResult{
			candleResult: CandleReadResult{
				Candles:  []domain.Candle{candle},
				Metadata: ReadResultMetadata{RawPayloadIDs: []string{"rp-candle-1"}},
			},
		}

		sink := &fakeIngestionSink{}
		lineageSink := &fakeRawPayloadLineageSink{}

		flow, err := NewIngestionFlow(sink)
		require.NoError(t, err)

		flow.WithRawPayloadLineage(lineageSink)

		request := fixedCandleRequest()
		persisted, err := flow.IngestCandles(t.Context(), venue, request)
		require.NoError(t, err)

		require.Len(t, persisted, 1)
		require.Len(t, lineageSink.candleLinks, 1)
		require.Equal(t, persisted[0], lineageSink.candleLinks[0].candle)
		require.Equal(t, "rp-candle-1", lineageSink.candleLinks[0].rawPayloadID)
		require.Len(t, sink.upsertedCandles, 1)
	})

	t.Run("links raw payload IDs for ingested trades", func(t *testing.T) {
		t.Parallel()

		instrument := makeInstrument()
		trade, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  time.Date(2024, time.January, 8, 3, 0, 0, 0, time.UTC),
			Price:      2,
			Size:       3,
			Quality:    domain.DataQualityValidated,
			Provenance: domain.SourceProvenance{Source: "trade-source", RecordID: ""},
		})
		require.NoError(t, err)

		venue := &fakeVenueReadResult{
			tradeResult: TradeReadResult{
				Trades:   []domain.Trade{trade},
				Metadata: ReadResultMetadata{RawPayloadIDs: []string{"rp-trade-1", "rp-trade-2"}},
			},
		}

		sink := &fakeIngestionSink{}
		lineageSink := &fakeRawPayloadLineageSink{}

		flow, err := NewIngestionFlow(sink)
		require.NoError(t, err)

		flow.WithRawPayloadLineage(lineageSink)

		request := fixedTradeRequest()
		persisted, err := flow.IngestTrades(t.Context(), venue, request)
		require.NoError(t, err)

		require.Len(t, persisted, 1)
		require.Len(t, lineageSink.tradeLinks, 2)
		require.Equal(t, persisted[0], lineageSink.tradeLinks[0].trade)
		require.Equal(t, persisted[0], lineageSink.tradeLinks[1].trade)
		require.Equal(t, "rp-trade-1", lineageSink.tradeLinks[0].rawPayloadID)
		require.Equal(t, "rp-trade-2", lineageSink.tradeLinks[1].rawPayloadID)
		require.Len(t, sink.upsertedTrades, 1)
	})

	t.Run("returns wrapped error when linking raw payload to instrument fails", func(t *testing.T) {
		t.Parallel()

		venue := &fakeVenueReadResult{
			instrumentResult: InstrumentReadResult{
				Instruments: []domain.Instrument{makeInstrument()},
				Metadata:    ReadResultMetadata{RawPayloadIDs: []string{"rp-fail"}},
			},
		}
		sink := &fakeIngestionSink{}
		lineageSink := &fakeRawPayloadLineageSink{linkInstrumentErr: errors.New("lineage unavailable")}

		flow, err := NewIngestionFlow(sink)
		require.NoError(t, err)

		flow.WithRawPayloadLineage(lineageSink)

		request, err := NewInstrumentReadRequest(InstrumentReadRequestParams{
			Venue:    domain.Venue("sandbox-int"),
			PageSize: 10,
		})
		require.NoError(t, err)

		_, err = flow.IngestInstruments(t.Context(), venue, request)
		require.Error(t, err)
		require.Contains(t, err.Error(), "link raw payload to instrument")
		require.ErrorIs(t, err, lineageSink.linkInstrumentErr)
	})

	t.Run("returns wrapped error when linking raw payload to candle fails", func(t *testing.T) {
		t.Parallel()

		instrument := makeInstrument()
		timeRange, err := domain.NewTimeRange(
			time.Date(2024, time.January, 8, 2, 0, 0, 0, time.UTC),
			time.Date(2024, time.January, 8, 2, 5, 0, 0, time.UTC),
		)
		require.NoError(t, err)
		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  timeRange,
			Open:       1,
			High:       1,
			Low:        1,
			Close:      1,
			Volume:     1,
			Quality:    domain.DataQualityRaw,
			Provenance: domain.SourceProvenance{Source: "candle-source", RecordID: ""},
		})
		require.NoError(t, err)

		venue := &fakeVenueReadResult{
			candleResult: CandleReadResult{
				Candles:  []domain.Candle{candle},
				Metadata: ReadResultMetadata{RawPayloadIDs: []string{"rp-fail"}},
			},
		}

		sink := &fakeIngestionSink{}
		linkErr := errors.New("lineage unavailable")
		lineageSink := &fakeRawPayloadLineageSink{linkCandleErr: linkErr}

		flow, err := NewIngestionFlow(sink)
		require.NoError(t, err)

		flow.WithRawPayloadLineage(lineageSink)

		request := fixedCandleRequest()
		_, err = flow.IngestCandles(t.Context(), venue, request)
		require.Error(t, err)
		require.Contains(t, err.Error(), "link raw payload to candle")
		require.ErrorIs(t, err, linkErr)
	})

	t.Run("returns wrapped error when linking raw payload to trade fails", func(t *testing.T) {
		t.Parallel()

		instrument := makeInstrument()
		trade, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  time.Date(2024, time.January, 8, 4, 0, 0, 0, time.UTC),
			Price:      2,
			Size:       3,
			Quality:    domain.DataQualityValidated,
			Provenance: domain.SourceProvenance{Source: "trade-source", RecordID: ""},
		})
		require.NoError(t, err)

		venue := &fakeVenueReadResult{
			tradeResult: TradeReadResult{
				Trades:   []domain.Trade{trade},
				Metadata: ReadResultMetadata{RawPayloadIDs: []string{"rp-fail"}},
			},
		}

		sink := &fakeIngestionSink{}
		linkErr := errors.New("lineage unavailable")
		lineageSink := &fakeRawPayloadLineageSink{linkTradeErr: linkErr}

		flow, err := NewIngestionFlow(sink)
		require.NoError(t, err)

		flow.WithRawPayloadLineage(lineageSink)

		request := fixedTradeRequest()
		_, err = flow.IngestTrades(t.Context(), venue, request)
		require.Error(t, err)
		require.Contains(t, err.Error(), "link raw payload to trade")
		require.ErrorIs(t, err, linkErr)
	})
}
