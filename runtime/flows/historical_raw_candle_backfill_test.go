package flows

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type historicalBackfillTestRunRecorder struct {
	callOrder   *[]string
	calls       []data.IngestionRun
	errByStatus map[data.IngestionRunStatus]error
}

func (r *historicalBackfillTestRunRecorder) record(
	_ context.Context,
	run data.IngestionRun,
) (data.IngestionRun, error) {
	if r.callOrder != nil {
		*r.callOrder = append(*r.callOrder, "record-"+run.Status.String())
	}
	r.calls = append(r.calls, run)
	if err := r.errByStatus[run.Status]; err != nil {
		return data.IngestionRun{}, err
	}

	return run, nil
}

type historicalBackfillTestVenueBuilder struct {
	callOrder *[]string
	builds    []HistoricalRawCandleBackfillVenueBuildParams
	venue     venueedge.MarketDataVenue
	err       error
}

func (b *historicalBackfillTestVenueBuilder) build(
	_ context.Context,
	params HistoricalRawCandleBackfillVenueBuildParams,
) (venueedge.MarketDataVenue, error) {
	if b.callOrder != nil {
		*b.callOrder = append(*b.callOrder, "build-venue")
	}
	b.builds = append(b.builds, params)
	if b.err != nil {
		return nil, b.err
	}

	return b.venue, nil
}

type historicalBackfillTestVenue struct {
	callOrder           *[]string
	candleResult        venueedge.CandleReadResult
	candleErr           error
	readInstrumentCalls int
	readCandleCalls     int
	readTradeCalls      int
}

func (v *historicalBackfillTestVenue) ReadInstruments(
	_ context.Context,
	_ venueedge.InstrumentReadRequest,
) (venueedge.InstrumentReadResult, error) {
	v.readInstrumentCalls++
	return venueedge.InstrumentReadResult{}, nil
}

func (v *historicalBackfillTestVenue) ReadCandles(
	_ context.Context,
	_ venueedge.CandleReadRequest,
) (venueedge.CandleReadResult, error) {
	v.readCandleCalls++
	if v.callOrder != nil {
		*v.callOrder = append(*v.callOrder, "read-candles")
	}
	if v.candleErr != nil {
		return venueedge.CandleReadResult{}, v.candleErr
	}

	return v.candleResult, nil
}

func (v *historicalBackfillTestVenue) ReadTrades(
	_ context.Context,
	_ venueedge.TradeReadRequest,
) (venueedge.TradeReadResult, error) {
	v.readTradeCalls++
	return venueedge.TradeReadResult{}, nil
}

type historicalBackfillTestIngestionSink struct {
	callOrder   *[]string
	instruments map[string]domain.Instrument
	candles     map[string]domain.Candle
	candleErr   error
	tradeCalls  int
}

func (s *historicalBackfillTestIngestionSink) UpsertInstrument(
	_ context.Context,
	instrument domain.Instrument,
) (domain.Instrument, error) {
	if s.instruments == nil {
		s.instruments = make(map[string]domain.Instrument)
	}
	key := instrumentKey(instrument)
	s.instruments[key] = instrument
	return s.instruments[key], nil
}

func (s *historicalBackfillTestIngestionSink) IngestCandle(
	ctx context.Context,
	candle domain.Candle,
) (domain.Candle, error) {
	if s.callOrder != nil {
		*s.callOrder = append(*s.callOrder, "persist-candle")
	}
	if s.candleErr != nil {
		return domain.Candle{}, s.candleErr
	}
	persistedInstrument, err := s.UpsertInstrument(ctx, candle.Instrument)
	if err != nil {
		return domain.Candle{}, err
	}
	candle.Instrument = persistedInstrument
	if s.candles == nil {
		s.candles = make(map[string]domain.Candle)
	}
	key := candleKey(candle)
	s.candles[key] = candle
	return s.candles[key], nil
}

func (s *historicalBackfillTestIngestionSink) IngestTrade(
	_ context.Context,
	_ domain.Trade,
) (domain.Trade, error) {
	s.tradeCalls++
	return domain.Trade{}, nil
}

type historicalBackfillTestLineageSink struct {
	callOrder []*[]string
	linkErr   error
	links     []historicalBackfillTestCandleLink
	linkSet   map[string]struct{}
}

type historicalBackfillTestReadback struct {
	queryCalls  []historicalBackfillTestReadbackCall
	replayCalls []historicalBackfillTestReadbackCall
	queryValue  []domain.Candle
	replayValue []data.ReplayCandle
	queryErr    error
	replayErr   error
}

type historicalBackfillTestReadbackCall struct {
	instrument domain.Instrument
	timeframe  domain.Timeframe
	timeRange  domain.TimeRange
}

func (r *historicalBackfillTestReadback) query(
	_ context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]domain.Candle, error) {
	r.queryCalls = append(r.queryCalls, historicalBackfillTestReadbackCall{
		instrument: instrument,
		timeframe:  timeframe,
		timeRange:  timeRange,
	})
	if r.queryErr != nil {
		return nil, r.queryErr
	}

	return r.queryValue, nil
}

func (r *historicalBackfillTestReadback) replay(
	_ context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]data.ReplayCandle, error) {
	r.replayCalls = append(r.replayCalls, historicalBackfillTestReadbackCall{
		instrument: instrument,
		timeframe:  timeframe,
		timeRange:  timeRange,
	})
	if r.replayErr != nil {
		return nil, r.replayErr
	}

	return r.replayValue, nil
}

type historicalBackfillTestCandleLink struct {
	rawPayloadID string
	candle       domain.Candle
}

func (s *historicalBackfillTestLineageSink) LinkRawPayloadToInstrument(
	_ context.Context,
	_ string,
	_ domain.Instrument,
) error {
	return nil
}

func (s *historicalBackfillTestLineageSink) LinkRawPayloadToCandle(
	_ context.Context,
	rawPayloadID string,
	candle domain.Candle,
) error {
	for _, callOrder := range s.callOrder {
		if callOrder != nil {
			*callOrder = append(*callOrder, "link-candle")
		}
	}
	if s.linkErr != nil {
		return s.linkErr
	}
	if s.linkSet == nil {
		s.linkSet = make(map[string]struct{})
	}
	key := rawPayloadID + "|" + candleKey(candle)
	if _, exists := s.linkSet[key]; exists {
		return nil
	}
	s.linkSet[key] = struct{}{}
	s.links = append(s.links, historicalBackfillTestCandleLink{rawPayloadID: rawPayloadID, candle: candle})
	return nil
}

func (s *historicalBackfillTestLineageSink) LinkRawPayloadToTrade(
	_ context.Context,
	_ string,
	_ domain.Trade,
) error {
	return nil
}

func TestHistoricalRawCandleBackfillRunner(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	randomTimeRange := func(t *testing.T) domain.TimeRange {
		t.Helper()
		start := time.Date(
			fake.IntBetween(2021, 2030),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 20),
			0,
			0,
			0,
			time.UTC,
		)
		timeRange, err := domain.NewTimeRange(start, start.Add(time.Minute))
		require.NoError(t, err)
		return timeRange
	}

	makeRequest := func(t *testing.T) HistoricalRawCandleBackfillRequest {
		t.Helper()
		return HistoricalRawCandleBackfillRequest{
			RunID:      randomWord("run"),
			Venue:      venueedge.HyperliquidPerpsVenueName,
			Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
			AssetClass: domain.AssetClassFuture,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  randomTimeRange(t),
			PageSize:   fake.IntBetween(0, 50),
		}
	}

	makeCandle := func(t *testing.T, request HistoricalRawCandleBackfillRequest) domain.Candle {
		t.Helper()
		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      request.Venue,
			Symbol:     request.Symbol,
			AssetClass: request.AssetClass,
			Active:     true,
		})
		require.NoError(t, err)
		provenance, err := domain.NewSourceProvenance(randomWord("source"), randomWord("record"))
		require.NoError(t, err)
		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  request.Timeframe,
			TimeRange:  request.TimeRange,
			Open:       fake.Float64(2, 1, 1000),
			High:       fake.Float64(2, 1001, 2000),
			Low:        fake.Float64(2, 0, 999),
			Close:      fake.Float64(2, 1, 1000),
			Volume:     fake.Float64(2, 1, 1000),
			Quality:    domain.DataQualityRaw,
			Provenance: provenance,
		})
		require.NoError(t, err)
		return candle
	}

	makeRunner := func(
		t *testing.T,
		recorder *historicalBackfillTestRunRecorder,
		builder *historicalBackfillTestVenueBuilder,
		flow *venueedge.IngestionFlow,
		readback *historicalBackfillTestReadback,
		clockValues []time.Time,
	) *HistoricalRawCandleBackfillRunner {
		t.Helper()
		clockIndex := 0
		runner, err := NewHistoricalRawCandleBackfillRunner(HistoricalRawCandleBackfillRunnerDeps{
			RecordIngestionRun: recorder.record,
			BuildVenue:         builder.build,
			IngestCandles:      flow.IngestCandles,
			ReadPersistedCandles: func(
				ctx context.Context,
				instrument domain.Instrument,
				timeframe domain.Timeframe,
				timeRange domain.TimeRange,
			) ([]domain.Candle, error) {
				return readback.query(ctx, instrument, timeframe, timeRange)
			},
			ReplayPersistedCandles: func(
				ctx context.Context,
				instrument domain.Instrument,
				timeframe domain.Timeframe,
				timeRange domain.TimeRange,
			) ([]data.ReplayCandle, error) {
				return readback.replay(ctx, instrument, timeframe, timeRange)
			},
			Clock: func() time.Time {
				value := clockValues[clockIndex]
				clockIndex++
				return value
			},
		})
		require.NoError(t, err)
		return runner
	}

	t.Run("rejects invalid requests before lineage writes or venue reads", func(t *testing.T) {
		t.Parallel()

		baseRequest := makeRequest(t)

		testCases := []struct {
			name    string
			mutate  func(HistoricalRawCandleBackfillRequest) HistoricalRawCandleBackfillRequest
			message string
		}{
			{
				name: "run ID is required",
				mutate: func(request HistoricalRawCandleBackfillRequest) HistoricalRawCandleBackfillRequest {
					request.RunID = "   "
					return request
				},
				message: "run ID is required",
			},
			{
				name: "venue must be hyperliquid perps",
				mutate: func(request HistoricalRawCandleBackfillRequest) HistoricalRawCandleBackfillRequest {
					request.Venue = domain.Venue(randomWord("venue"))
					return request
				},
				message: "venue must be hyperliquid-perps",
			},
			{
				name: "symbol is required",
				mutate: func(request HistoricalRawCandleBackfillRequest) HistoricalRawCandleBackfillRequest {
					request.Symbol = ""
					return request
				},
				message: "symbol is required",
			},
			{
				name: "asset class must be future",
				mutate: func(request HistoricalRawCandleBackfillRequest) HistoricalRawCandleBackfillRequest {
					request.AssetClass = domain.AssetClassCrypto
					return request
				},
				message: "asset class must be future",
			},
			{
				name: "timeframe must be supported",
				mutate: func(request HistoricalRawCandleBackfillRequest) HistoricalRawCandleBackfillRequest {
					request.Timeframe = domain.Timeframe(randomWord("timeframe"))
					return request
				},
				message: "timeframe is unsupported",
			},
			{
				name: "time range must be half open",
				mutate: func(request HistoricalRawCandleBackfillRequest) HistoricalRawCandleBackfillRequest {
					request.TimeRange.End = request.TimeRange.Start
					return request
				},
				message: "time range must be half-open",
			},
			{
				name: "page size must be zero or positive",
				mutate: func(request HistoricalRawCandleBackfillRequest) HistoricalRawCandleBackfillRequest {
					request.PageSize = -fake.IntBetween(1, 50)
					return request
				},
				message: "page size must be zero or positive",
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				recorder := &historicalBackfillTestRunRecorder{errByStatus: map[data.IngestionRunStatus]error{}}
				venue := &historicalBackfillTestVenue{}
				builder := &historicalBackfillTestVenueBuilder{venue: venue}
				sink := &historicalBackfillTestIngestionSink{}
				lineageSink := &historicalBackfillTestLineageSink{}
				readback := &historicalBackfillTestReadback{}
				flow, err := venueedge.NewIngestionFlow(sink)
				require.NoError(t, err)
				flow.WithRawPayloadLineage(lineageSink)
				runner := makeRunner(
					t,
					recorder,
					builder,
					flow,
					readback,
					[]time.Time{time.Now().UTC(), time.Now().UTC()},
				)

				_, err = runner.Run(t.Context(), testCase.mutate(baseRequest))
				require.Error(t, err)
				require.ErrorIs(t, err, ErrValidation)
				require.Contains(t, err.Error(), testCase.message)
				require.Empty(t, recorder.calls)
				require.Empty(t, builder.builds)
				require.Zero(t, venue.readCandleCalls)
			})
		}
	})

	t.Run("records started before first venue read and succeeded after persistence", func(t *testing.T) {
		t.Parallel()

		request := makeRequest(t)
		startedAt := time.Date(2026, time.January, fake.IntBetween(1, 28), 7, 8, 9, 0, time.FixedZone("start", -3*3600))
		completedAt := startedAt.Add(2 * time.Minute)
		callOrder := make([]string, 0)
		recorder := &historicalBackfillTestRunRecorder{
			callOrder:   &callOrder,
			errByStatus: map[data.IngestionRunStatus]error{},
		}
		venue := &historicalBackfillTestVenue{callOrder: &callOrder}
		candle := makeCandle(t, request)
		venue.candleResult = venueedge.CandleReadResult{
			Candles:  []domain.Candle{candle},
			Metadata: venueedge.ReadResultMetadata{RawPayloadIDs: []string{randomWord("raw")}},
		}
		builder := &historicalBackfillTestVenueBuilder{callOrder: &callOrder, venue: venue}
		sink := &historicalBackfillTestIngestionSink{callOrder: &callOrder}
		lineageSink := &historicalBackfillTestLineageSink{callOrder: []*[]string{&callOrder}}
		readback := &historicalBackfillTestReadback{
			queryValue: []domain.Candle{candle},
			replayValue: []data.ReplayCandle{{
				Identity: 1,
				Candle:   candle,
			}},
		}
		flow, err := venueedge.NewIngestionFlow(sink)
		require.NoError(t, err)
		flow.WithRawPayloadLineage(lineageSink)
		runner := makeRunner(t, recorder, builder, flow, readback, []time.Time{startedAt, completedAt})

		result, err := runner.Run(t.Context(), request)
		require.NoError(t, err)
		require.Len(t, recorder.calls, 2)
		expectedCallOrder := []string{
			"record-started",
			"build-venue",
			"read-candles",
			"persist-candle",
			"link-candle",
			"record-succeeded",
		}
		require.Equal(t, expectedCallOrder, callOrder)

		startedRun := recorder.calls[0]
		require.Equal(t, request.RunID, startedRun.ID)
		require.Equal(t, historicalRawCandleBackfillSource, startedRun.Source)
		require.Equal(t, venueedge.HyperliquidPerpsVenueName, startedRun.Venue)
		require.Equal(t, data.IngestionRunStatusStarted, startedRun.Status)
		require.Equal(t, startedAt, startedRun.StartedAt)
		require.Nil(t, startedRun.CompletedAt)
		require.Zero(t, startedRun.RecordCount)
		require.Empty(t, startedRun.ErrorSummary)

		succeededRun := recorder.calls[1]
		require.Equal(t, request.RunID, succeededRun.ID)
		require.Equal(t, data.IngestionRunStatusSucceeded, succeededRun.Status)
		require.Equal(t, startedAt, succeededRun.StartedAt)
		require.NotNil(t, succeededRun.CompletedAt)
		require.Equal(t, completedAt, *succeededRun.CompletedAt)
		require.Equal(t, 1, succeededRun.RecordCount)
		require.Empty(t, succeededRun.ErrorSummary)

		require.Len(t, builder.builds, 1)
		require.Equal(t, request.RunID, builder.builds[0].RawEvidenceIngestionRun)
		require.Equal(t, request.Venue, builder.builds[0].Venue)
		require.Equal(t, request.Symbol, builder.builds[0].Symbol)
		require.Equal(t, request.AssetClass, builder.builds[0].AssetClass)
		require.Equal(t, request.Timeframe, builder.builds[0].Timeframe)
		require.Equal(t, request.TimeRange, builder.builds[0].TimeRange)

		require.Equal(t, request.RunID, result.RunID)
		require.Equal(t, []domain.Candle{candle}, result.PersistedCandles)
		require.Equal(t, 1, result.Report.PersistedCount)
	})

	t.Run("records failed lifecycle metadata for venue persistence and lineage failures", func(t *testing.T) {
		t.Parallel()

		request := makeRequest(t)
		candle := makeCandle(t, request)

		testCases := []struct {
			name             string
			venueErr         error
			persistErr       error
			linkErr          error
			wantError        string
			wantErrorSummary string
		}{
			{
				name:             "venue read",
				venueErr:         errors.New(randomWord("venue-unavailable")),
				wantError:        "ingest candles: read candles",
				wantErrorSummary: "ingest candles: read candles",
			},
			{
				name:             "persistence",
				persistErr:       errors.New(randomWord("persist-unavailable")),
				wantError:        "ingest candles: persist candle",
				wantErrorSummary: "ingest candles: persist candle",
			},
			{
				name:             "lineage",
				linkErr:          errors.New(randomWord("lineage-unavailable")),
				wantError:        "ingest candles: link raw payload to candle",
				wantErrorSummary: "ingest candles: link raw payload to candle",
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				startedAt := time.Date(
					2026,
					time.February,
					fake.IntBetween(1, 28),
					1,
					2,
					3,
					0,
					time.FixedZone("start", 5*3600),
				)
				failedAt := startedAt.Add(time.Minute)
				recorder := &historicalBackfillTestRunRecorder{errByStatus: map[data.IngestionRunStatus]error{}}
				venue := &historicalBackfillTestVenue{
					candleResult: venueedge.CandleReadResult{
						Candles:  []domain.Candle{candle},
						Metadata: venueedge.ReadResultMetadata{RawPayloadIDs: []string{randomWord("raw")}},
					},
					candleErr: testCase.venueErr,
				}
				builder := &historicalBackfillTestVenueBuilder{venue: venue}
				sink := &historicalBackfillTestIngestionSink{candleErr: testCase.persistErr}
				lineageSink := &historicalBackfillTestLineageSink{linkErr: testCase.linkErr}
				readback := &historicalBackfillTestReadback{}
				flow, err := venueedge.NewIngestionFlow(sink)
				require.NoError(t, err)
				flow.WithRawPayloadLineage(lineageSink)
				runner := makeRunner(t, recorder, builder, flow, readback, []time.Time{startedAt, failedAt})

				_, err = runner.Run(t.Context(), request)
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.wantError)
				require.Len(t, recorder.calls, 2)
				require.Equal(t, data.IngestionRunStatusStarted, recorder.calls[0].Status)
				failedRun := recorder.calls[1]
				require.Equal(t, data.IngestionRunStatusFailed, failedRun.Status)
				require.Equal(t, startedAt, failedRun.StartedAt)
				require.NotNil(t, failedRun.CompletedAt)
				require.Equal(t, failedAt, *failedRun.CompletedAt)
				require.Zero(t, failedRun.RecordCount)
				require.Contains(t, failedRun.ErrorSummary, testCase.wantErrorSummary)
			})
		}
	})

	t.Run("records best-known failed count when ingestion returns partial progress", func(t *testing.T) {
		t.Parallel()

		request := makeRequest(t)
		startedAt := time.Date(2026, time.March, fake.IntBetween(1, 28), 4, 5, 6, 0, time.FixedZone("start", -2*3600))
		failedAt := startedAt.Add(time.Minute)
		recorder := &historicalBackfillTestRunRecorder{errByStatus: map[data.IngestionRunStatus]error{}}
		venue := &historicalBackfillTestVenue{}
		builder := &historicalBackfillTestVenueBuilder{venue: venue}
		persistedCandles := []domain.Candle{makeCandle(t, request)}
		readback := &historicalBackfillTestReadback{}
		runner, err := NewHistoricalRawCandleBackfillRunner(HistoricalRawCandleBackfillRunnerDeps{
			RecordIngestionRun: recorder.record,
			BuildVenue:         builder.build,
			IngestCandles: func(context.Context, venueedge.MarketDataVenue, venueedge.CandleReadRequest) ([]domain.Candle, error) {
				return persistedCandles, errors.New(randomWord("partial-progress"))
			},
			ReadPersistedCandles:   readback.query,
			ReplayPersistedCandles: readback.replay,
			Clock: func() time.Time {
				if len(recorder.calls) == 0 {
					return startedAt
				}
				return failedAt
			},
		})
		require.NoError(t, err)

		_, err = runner.Run(t.Context(), request)
		require.Error(t, err)
		require.Len(t, recorder.calls, 2)
		require.Equal(t, data.IngestionRunStatusStarted, recorder.calls[0].Status)
		failedRun := recorder.calls[1]
		require.Equal(t, data.IngestionRunStatusFailed, failedRun.Status)
		require.Equal(t, len(persistedCandles), failedRun.RecordCount)
	})

	t.Run("uses raw evidence run context and keeps candle persistence idempotent", func(t *testing.T) {
		t.Parallel()

		request := makeRequest(t)
		recorder := &historicalBackfillTestRunRecorder{errByStatus: map[data.IngestionRunStatus]error{}}
		sink := &historicalBackfillTestIngestionSink{}
		lineageSink := &historicalBackfillTestLineageSink{}
		persistedCandle := makeCandle(t, request)
		readback := &historicalBackfillTestReadback{
			queryValue:  []domain.Candle{persistedCandle},
			replayValue: []data.ReplayCandle{{Identity: 1, Candle: persistedCandle}},
		}
		flow, err := venueedge.NewIngestionFlow(sink)
		require.NoError(t, err)
		flow.WithRawPayloadLineage(lineageSink)

		buildRunVenue := func(runID string) *historicalBackfillTestVenue {
			venue := &historicalBackfillTestVenue{}
			venue.candleResult = venueedge.CandleReadResult{
				Candles: []domain.Candle{makeCandle(t, request)},
				Metadata: venueedge.ReadResultMetadata{
					RawPayloadIDs: []string{"raw-" + runID},
				},
			}
			return venue
		}

		builder := &historicalBackfillTestVenueBuilder{}
		builtVenues := make([]*historicalBackfillTestVenue, 0, 3)
		runner, err := NewHistoricalRawCandleBackfillRunner(HistoricalRawCandleBackfillRunnerDeps{
			RecordIngestionRun: recorder.record,
			BuildVenue: func(_ context.Context, params HistoricalRawCandleBackfillVenueBuildParams) (venueedge.MarketDataVenue, error) {
				venue := buildRunVenue(params.RawEvidenceIngestionRun)
				builtVenues = append(builtVenues, venue)
				builder.builds = append(builder.builds, params)
				return venue, nil
			},
			IngestCandles:          flow.IngestCandles,
			ReadPersistedCandles:   readback.query,
			ReplayPersistedCandles: readback.replay,
			Clock: func() time.Time {
				return request.TimeRange.Start
			},
		})
		require.NoError(t, err)

		firstResult, err := runner.Run(t.Context(), request)
		require.NoError(t, err)
		secondResult, err := runner.Run(t.Context(), request)
		require.NoError(t, err)
		secondRunID := randomWord("run-next")
		thirdRequest := request
		thirdRequest.RunID = secondRunID
		thirdResult, err := runner.Run(t.Context(), thirdRequest)
		require.NoError(t, err)

		require.Len(t, builder.builds, 3)
		require.Equal(t, []string{request.RunID, request.RunID, secondRunID}, []string{
			builder.builds[0].RawEvidenceIngestionRun,
			builder.builds[1].RawEvidenceIngestionRun,
			builder.builds[2].RawEvidenceIngestionRun,
		})
		require.Len(t, sink.candles, 1)
		require.Len(t, lineageSink.links, 2)
		require.Equal(t, []string{"raw-" + request.RunID, "raw-" + secondRunID}, []string{
			lineageSink.links[0].rawPayloadID,
			lineageSink.links[1].rawPayloadID,
		})
		for _, venue := range builtVenues {
			require.Zero(t, venue.readTradeCalls)
		}
		require.Equal(t, 1, firstResult.Report.PersistedCount)
		require.Equal(t, 1, secondResult.Report.PersistedCount)
		require.Equal(t, 1, thirdResult.Report.PersistedCount)
	})

	t.Run("reads persisted candles back and computes deterministic completeness metadata", func(t *testing.T) {
		t.Parallel()

		t.Run("reports expected interval counts for each supported timeframe", func(t *testing.T) {
			t.Parallel()

			timeframeDurations := map[domain.Timeframe]time.Duration{
				domain.Timeframe1m:  time.Minute,
				domain.Timeframe5m:  5 * time.Minute,
				domain.Timeframe15m: 15 * time.Minute,
				domain.Timeframe1h:  time.Hour,
				domain.Timeframe4h:  4 * time.Hour,
				domain.Timeframe1d:  24 * time.Hour,
			}

			for timeframe, intervalDuration := range timeframeDurations {
				t.Run(timeframe.String(), func(t *testing.T) {
					t.Parallel()

					request := makeRequest(t)
					request.Timeframe = timeframe
					request.TimeRange.End = request.TimeRange.Start.Add(3 * intervalDuration)
					candle := makeCandle(t, request)

					recorder := &historicalBackfillTestRunRecorder{errByStatus: map[data.IngestionRunStatus]error{}}
					venue := &historicalBackfillTestVenue{
						candleResult: venueedge.CandleReadResult{
							Candles:  []domain.Candle{candle},
							Metadata: venueedge.ReadResultMetadata{RawPayloadIDs: []string{randomWord("raw")}},
						},
					}
					builder := &historicalBackfillTestVenueBuilder{venue: venue}
					sink := &historicalBackfillTestIngestionSink{}
					lineageSink := &historicalBackfillTestLineageSink{}
					readback := &historicalBackfillTestReadback{
						queryValue:  []domain.Candle{candle},
						replayValue: []data.ReplayCandle{{Identity: 1, Candle: candle}},
					}
					flow, err := venueedge.NewIngestionFlow(sink)
					require.NoError(t, err)
					flow.WithRawPayloadLineage(lineageSink)
					runner := makeRunner(
						t,
						recorder,
						builder,
						flow,
						readback,
						[]time.Time{request.TimeRange.Start, request.TimeRange.End},
					)

					result, err := runner.Run(t.Context(), request)
					require.NoError(t, err)
					require.Equal(t, 3, result.Report.ExpectedCount)
				})
			}
		})

		t.Run("reports gaps duplicates and optional raw payload count from readback", func(t *testing.T) {
			t.Parallel()

			request := makeRequest(t)
			request.TimeRange.End = request.TimeRange.Start.Add(4 * time.Minute)

			firstRequest := request
			firstRequest.TimeRange = domain.TimeRange{
				Start: request.TimeRange.Start,
				End:   request.TimeRange.Start.Add(time.Minute),
			}
			firstCandle := makeCandle(t, firstRequest)
			secondRequest := request
			secondRequest.TimeRange = domain.TimeRange{
				Start: request.TimeRange.Start.Add(2 * time.Minute),
				End:   request.TimeRange.Start.Add(3 * time.Minute),
			}
			secondCandle := makeCandle(t, secondRequest)

			recorder := &historicalBackfillTestRunRecorder{errByStatus: map[data.IngestionRunStatus]error{}}
			venue := &historicalBackfillTestVenue{
				candleResult: venueedge.CandleReadResult{
					Candles: []domain.Candle{firstCandle, secondCandle},
					Metadata: venueedge.ReadResultMetadata{RawPayloadIDs: []string{
						randomWord("raw-page-1"),
						randomWord("raw-page-2"),
					}},
				},
			}
			builder := &historicalBackfillTestVenueBuilder{venue: venue}
			sink := &historicalBackfillTestIngestionSink{}
			lineageSink := &historicalBackfillTestLineageSink{}
			readback := &historicalBackfillTestReadback{
				queryValue: []domain.Candle{firstCandle, secondCandle},
				replayValue: []data.ReplayCandle{
					{Identity: 1, Candle: firstCandle},
					{Identity: 2, Candle: firstCandle},
					{Identity: 3, Candle: secondCandle},
				},
			}
			flow, err := venueedge.NewIngestionFlow(sink)
			require.NoError(t, err)
			flow.WithRawPayloadLineage(lineageSink)

			rawPayloadCount := 2 + fake.IntBetween(1, 3)
			clockValues := []time.Time{request.TimeRange.Start, request.TimeRange.End}
			clockIndex := 0
			runner, err := NewHistoricalRawCandleBackfillRunner(HistoricalRawCandleBackfillRunnerDeps{
				RecordIngestionRun:     recorder.record,
				BuildVenue:             builder.build,
				IngestCandles:          flow.IngestCandles,
				ReadPersistedCandles:   readback.query,
				ReplayPersistedCandles: readback.replay,
				CountRunRawPayloads: func(context.Context, string) (int, error) {
					return rawPayloadCount, nil
				},
				MissingIntervalPreviewLimit: 1,
				Clock: func() time.Time {
					value := clockValues[clockIndex]
					clockIndex++
					return value
				},
			})
			require.NoError(t, err)

			result, err := runner.Run(t.Context(), request)
			require.NoError(t, err)
			require.Equal(t, []domain.Candle{firstCandle, secondCandle}, result.PersistedCandles)
			require.Equal(t, 4, result.Report.ExpectedCount)
			require.Equal(t, 2, result.Report.PersistedCount)
			require.Equal(t, 2, result.Report.MissingIntervalCount)
			require.Equal(t, 1, result.Report.DuplicateNaturalKeyCount)
			require.NotNil(t, result.Report.FirstPersistedStart)
			require.Equal(t, firstCandle.TimeRange.Start, *result.Report.FirstPersistedStart)
			require.NotNil(t, result.Report.LastPersistedEnd)
			require.Equal(t, secondCandle.TimeRange.End, *result.Report.LastPersistedEnd)
			require.NotNil(t, result.Report.RawPayloadCount)
			require.Equal(t, rawPayloadCount, *result.Report.RawPayloadCount)
			require.Equal(t, 1, result.Report.MissingIntervalPreviewLimit)
			require.Equal(t, []domain.TimeRange{{
				Start: request.TimeRange.Start.Add(time.Minute),
				End:   request.TimeRange.Start.Add(2 * time.Minute),
			}}, result.Report.MissingIntervalPreview)
		})

		t.Run("aligns expected gaps to candle bucket boundaries", func(t *testing.T) {
			t.Parallel()

			request := makeRequest(t)
			request.Timeframe = domain.Timeframe5m
			request.TimeRange = domain.TimeRange{
				Start: time.Date(2026, time.April, 10, 12, 1, 0, 0, time.UTC),
				End:   time.Date(2026, time.April, 10, 12, 16, 0, 0, time.UTC),
			}

			firstBucketRequest := request
			firstBucketRequest.TimeRange = domain.TimeRange{
				Start: time.Date(2026, time.April, 10, 12, 5, 0, 0, time.UTC),
				End:   time.Date(2026, time.April, 10, 12, 10, 0, 0, time.UTC),
			}
			firstBucketCandle := makeCandle(t, firstBucketRequest)

			thirdBucketRequest := request
			thirdBucketRequest.TimeRange = domain.TimeRange{
				Start: time.Date(2026, time.April, 10, 12, 15, 0, 0, time.UTC),
				End:   time.Date(2026, time.April, 10, 12, 20, 0, 0, time.UTC),
			}
			thirdBucketCandle := makeCandle(t, thirdBucketRequest)

			recorder := &historicalBackfillTestRunRecorder{errByStatus: map[data.IngestionRunStatus]error{}}
			venue := &historicalBackfillTestVenue{
				candleResult: venueedge.CandleReadResult{
					Candles: []domain.Candle{firstBucketCandle, thirdBucketCandle},
					Metadata: venueedge.ReadResultMetadata{RawPayloadIDs: []string{
						randomWord("raw-page-1"),
						randomWord("raw-page-2"),
					}},
				},
			}
			builder := &historicalBackfillTestVenueBuilder{venue: venue}
			sink := &historicalBackfillTestIngestionSink{}
			lineageSink := &historicalBackfillTestLineageSink{}
			readback := &historicalBackfillTestReadback{
				queryValue: []domain.Candle{firstBucketCandle, thirdBucketCandle},
				replayValue: []data.ReplayCandle{
					{Identity: 1, Candle: firstBucketCandle},
					{Identity: 2, Candle: thirdBucketCandle},
				},
			}
			flow, err := venueedge.NewIngestionFlow(sink)
			require.NoError(t, err)
			flow.WithRawPayloadLineage(lineageSink)
			runner := makeRunner(
				t,
				recorder,
				builder,
				flow,
				readback,
				[]time.Time{request.TimeRange.Start, request.TimeRange.End},
			)

			result, err := runner.Run(t.Context(), request)
			require.NoError(t, err)
			require.Equal(t, 3, result.Report.ExpectedCount)
			require.Equal(t, 1, result.Report.MissingIntervalCount)
			require.Equal(t, []domain.TimeRange{{
				Start: time.Date(2026, time.April, 10, 12, 10, 0, 0, time.UTC),
				End:   time.Date(2026, time.April, 10, 12, 15, 0, 0, time.UTC),
			}}, result.Report.MissingIntervalPreview)
		})

		t.Run("omits raw payload count when no cheap run counter is configured", func(t *testing.T) {
			t.Parallel()

			request := makeRequest(t)
			candle := makeCandle(t, request)
			recorder := &historicalBackfillTestRunRecorder{errByStatus: map[data.IngestionRunStatus]error{}}
			venue := &historicalBackfillTestVenue{
				candleResult: venueedge.CandleReadResult{
					Candles:  []domain.Candle{candle},
					Metadata: venueedge.ReadResultMetadata{RawPayloadIDs: []string{randomWord("raw")}},
				},
			}
			builder := &historicalBackfillTestVenueBuilder{venue: venue}
			sink := &historicalBackfillTestIngestionSink{}
			lineageSink := &historicalBackfillTestLineageSink{}
			readback := &historicalBackfillTestReadback{
				queryValue:  []domain.Candle{candle},
				replayValue: []data.ReplayCandle{{Identity: 1, Candle: candle}},
			}
			flow, err := venueedge.NewIngestionFlow(sink)
			require.NoError(t, err)
			flow.WithRawPayloadLineage(lineageSink)
			runner := makeRunner(
				t,
				recorder,
				builder,
				flow,
				readback,
				[]time.Time{request.TimeRange.Start, request.TimeRange.End},
			)

			result, err := runner.Run(t.Context(), request)
			require.NoError(t, err)
			require.Nil(t, result.Report.RawPayloadCount)
		})
	})
}

func instrumentKey(instrument domain.Instrument) string {
	return strings.Join([]string{
		instrument.Venue.String(),
		instrument.Symbol.String(),
		instrument.AssetClass.String(),
	}, "|")
}

func candleKey(candle domain.Candle) string {
	return strings.Join([]string{
		instrumentKey(candle.Instrument),
		candle.Timeframe.String(),
		strconv.FormatInt(candle.TimeRange.Start.UnixNano(), 10),
		strconv.FormatInt(candle.TimeRange.End.UnixNano(), 10),
	}, "|")
}
