package flows

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"gorm.io/gorm"
)

const historicalRawCandleBackfillSource = "hyperliquid-historical-raw-candle-backfill"

const (
	historicalRawCandleBackfillDefaultMissingIntervalPreviewLimit = 10
	historicalRawCandleBackfill5mDuration                         = 5 * time.Minute
	historicalRawCandleBackfill15mDuration                        = 15 * time.Minute
	historicalRawCandleBackfill4hDuration                         = 4 * time.Hour
	historicalRawCandleBackfill1dDuration                         = 24 * time.Hour
)

// HistoricalRawCandleBackfillRequest defines one manual historical candle backfill run.
type HistoricalRawCandleBackfillRequest struct {
	RunID      string
	Venue      domain.Venue
	Symbol     domain.Symbol
	AssetClass domain.AssetClass
	Timeframe  domain.Timeframe
	TimeRange  domain.TimeRange
	PageSize   int
}

// HistoricalRawCandleBackfillVenueBuildParams configures a run-scoped venue instance.
type HistoricalRawCandleBackfillVenueBuildParams struct {
	RawEvidenceIngestionRun string
	Venue                   domain.Venue
	Symbol                  domain.Symbol
	AssetClass              domain.AssetClass
	Timeframe               domain.Timeframe
	TimeRange               domain.TimeRange
	PageSize                int
}

// HistoricalRawCandleBackfillReport summarizes the currently known persisted result.
type HistoricalRawCandleBackfillReport struct {
	Venue                       domain.Venue
	Symbol                      domain.Symbol
	AssetClass                  domain.AssetClass
	Timeframe                   domain.Timeframe
	TimeRange                   domain.TimeRange
	PersistedCount              int
	ExpectedCount               int
	MissingIntervalCount        int
	DuplicateNaturalKeyCount    int
	FirstPersistedStart         *time.Time
	LastPersistedEnd            *time.Time
	RawPayloadCount             *int
	MissingIntervalPreview      []domain.TimeRange
	MissingIntervalPreviewLimit int
}

// HistoricalRawCandleBackfillResult returns the persisted candles for one run.
type HistoricalRawCandleBackfillResult struct {
	RunID            string
	PersistedCandles []domain.Candle
	Report           HistoricalRawCandleBackfillReport
}

// HistoricalRawCandleBackfillRunnerDeps configures the manual backfill runner.
type HistoricalRawCandleBackfillRunnerDeps struct {
	RecordIngestionRun   func(context.Context, data.IngestionRun) (data.IngestionRun, error)
	BuildVenue           func(context.Context, HistoricalRawCandleBackfillVenueBuildParams) (venueedge.MarketDataVenue, error)
	IngestCandles        func(context.Context, venueedge.MarketDataVenue, venueedge.CandleReadRequest) ([]domain.Candle, error)
	ReadPersistedCandles func(
		context.Context,
		domain.Instrument,
		domain.Timeframe,
		domain.TimeRange,
	) ([]domain.Candle, error)
	ReplayPersistedCandles func(
		context.Context,
		domain.Instrument,
		domain.Timeframe,
		domain.TimeRange,
	) ([]data.ReplayCandle, error)
	CountRunRawPayloads         func(context.Context, string) (int, error)
	MissingIntervalPreviewLimit int
	Clock                       func() time.Time
}

// HistoricalRawCandleBackfillRunner coordinates manual Hyperliquid candle backfills.
type HistoricalRawCandleBackfillRunner struct {
	recordIngestionRun   func(context.Context, data.IngestionRun) (data.IngestionRun, error)
	buildVenue           func(context.Context, HistoricalRawCandleBackfillVenueBuildParams) (venueedge.MarketDataVenue, error)
	ingestCandles        func(context.Context, venueedge.MarketDataVenue, venueedge.CandleReadRequest) ([]domain.Candle, error)
	readPersistedCandles func(
		context.Context,
		domain.Instrument,
		domain.Timeframe,
		domain.TimeRange,
	) ([]domain.Candle, error)
	replayPersistedCandles func(
		context.Context,
		domain.Instrument,
		domain.Timeframe,
		domain.TimeRange,
	) ([]data.ReplayCandle, error)
	countRunRawPayloads         func(context.Context, string) (int, error)
	missingIntervalPreviewLimit int
	clock                       func() time.Time
}

// NewHistoricalRawCandleBackfillRunner creates a manual historical candle backfill runner.
func NewHistoricalRawCandleBackfillRunner(
	deps HistoricalRawCandleBackfillRunnerDeps,
) (*HistoricalRawCandleBackfillRunner, error) {
	if deps.RecordIngestionRun == nil {
		return nil, errors.New("ingestion run recorder is required")
	}
	if deps.BuildVenue == nil {
		return nil, errors.New("venue builder is required")
	}
	if deps.IngestCandles == nil {
		return nil, errors.New("candle ingestion flow is required")
	}
	if deps.ReadPersistedCandles == nil {
		return nil, errors.New("persisted candle reader is required")
	}
	if deps.ReplayPersistedCandles == nil {
		return nil, errors.New("persisted candle replay reader is required")
	}
	if deps.Clock == nil {
		deps.Clock = time.Now
	}
	if deps.MissingIntervalPreviewLimit <= 0 {
		deps.MissingIntervalPreviewLimit =
			historicalRawCandleBackfillDefaultMissingIntervalPreviewLimit
	}

	return &HistoricalRawCandleBackfillRunner{
		recordIngestionRun:          deps.RecordIngestionRun,
		buildVenue:                  deps.BuildVenue,
		ingestCandles:               deps.IngestCandles,
		readPersistedCandles:        deps.ReadPersistedCandles,
		replayPersistedCandles:      deps.ReplayPersistedCandles,
		countRunRawPayloads:         deps.CountRunRawPayloads,
		missingIntervalPreviewLimit: deps.MissingIntervalPreviewLimit,
		clock:                       deps.Clock,
	}, nil
}

// Run validates the request, records lifecycle metadata, and ingests candles.
func (r *HistoricalRawCandleBackfillRunner) Run(
	ctx context.Context,
	request HistoricalRawCandleBackfillRequest,
) (HistoricalRawCandleBackfillResult, error) {
	canonicalRequest, instrument, candleReadRequest, err := canonicalizeHistoricalRawCandleBackfillRequest(request)
	if err != nil {
		return HistoricalRawCandleBackfillResult{}, err
	}

	startedRun, err := data.NewIngestionRun(data.IngestionRunParams{
		ID:          canonicalRequest.RunID,
		Source:      historicalRawCandleBackfillSource,
		Venue:       canonicalRequest.Venue,
		Status:      data.IngestionRunStatusStarted,
		StartedAt:   r.clock(),
		RecordCount: 0,
	})
	if err != nil {
		return HistoricalRawCandleBackfillResult{}, fmt.Errorf("build started ingestion run: %w", err)
	}
	startedRun, err = r.recordIngestionRun(ctx, startedRun)
	if err != nil {
		return HistoricalRawCandleBackfillResult{}, fmt.Errorf("record started ingestion run: %w", err)
	}

	venue, err := r.buildVenue(ctx, HistoricalRawCandleBackfillVenueBuildParams{
		RawEvidenceIngestionRun: canonicalRequest.RunID,
		Venue:                   canonicalRequest.Venue,
		Symbol:                  canonicalRequest.Symbol,
		AssetClass:              canonicalRequest.AssetClass,
		Timeframe:               canonicalRequest.Timeframe,
		TimeRange:               canonicalRequest.TimeRange,
		PageSize:                canonicalRequest.PageSize,
	})
	if err != nil {
		return HistoricalRawCandleBackfillResult{}, r.failRun(ctx, startedRun, 0, err)
	}

	ingestedCandles, err := r.ingestCandles(ctx, venue, candleReadRequest)
	if err != nil {
		return HistoricalRawCandleBackfillResult{}, r.failRun(
			ctx,
			startedRun,
			len(ingestedCandles),
			fmt.Errorf("ingest candles: %w", err),
		)
	}

	persistedCandles, report, err := r.readBackfillResult(ctx, canonicalRequest, instrument)
	if err != nil {
		return HistoricalRawCandleBackfillResult{}, r.failRun(
			ctx,
			startedRun,
			len(ingestedCandles),
			fmt.Errorf("read persisted candles: %w", err),
		)
	}

	completedRun, err := r.recordRunStatus(
		ctx,
		startedRun,
		data.IngestionRunStatusSucceeded,
		len(persistedCandles),
		"",
		r.clock(),
	)
	if err != nil {
		return HistoricalRawCandleBackfillResult{}, fmt.Errorf("record succeeded ingestion run: %w", err)
	}

	return HistoricalRawCandleBackfillResult{
		RunID:            completedRun.ID,
		PersistedCandles: persistedCandles,
		Report:           report,
	}, nil
}

func (r *HistoricalRawCandleBackfillRunner) readBackfillResult(
	ctx context.Context,
	request HistoricalRawCandleBackfillRequest,
	instrument domain.Instrument,
) ([]domain.Candle, HistoricalRawCandleBackfillReport, error) {
	persistedCandles, err := r.readPersistedCandles(
		ctx,
		instrument,
		request.Timeframe,
		request.TimeRange,
	)
	if err != nil {
		if !isHistoricalRawCandleBackfillEmptyReadbackError(err) {
			return nil, HistoricalRawCandleBackfillReport{}, err
		}
		persistedCandles = nil
	}

	replayedCandles, err := r.replayPersistedCandles(
		ctx,
		instrument,
		request.Timeframe,
		request.TimeRange,
	)
	if err != nil {
		if !isHistoricalRawCandleBackfillEmptyReadbackError(err) {
			return nil, HistoricalRawCandleBackfillReport{}, err
		}
		replayedCandles = nil
	}

	var rawPayloadCount *int
	if r.countRunRawPayloads != nil {
		count, countErr := r.countRunRawPayloads(ctx, request.RunID)
		if countErr != nil {
			return nil, HistoricalRawCandleBackfillReport{}, countErr
		}
		rawPayloadCount = &count
	}

	report, err := newHistoricalRawCandleBackfillReport(
		request,
		instrument,
		persistedCandles,
		replayedCandles,
		rawPayloadCount,
		r.missingIntervalPreviewLimit,
	)
	if err != nil {
		return nil, HistoricalRawCandleBackfillReport{}, err
	}

	return persistedCandles, report, nil
}

func (r *HistoricalRawCandleBackfillRunner) failRun(
	ctx context.Context,
	startedRun data.IngestionRun,
	recordCount int,
	runErr error,
) error {
	_, statusErr := r.recordRunStatus(
		ctx,
		startedRun,
		data.IngestionRunStatusFailed,
		recordCount,
		conciseErrorSummary(runErr),
		r.clock(),
	)
	if statusErr != nil {
		return errors.Join(runErr, fmt.Errorf("record failed ingestion run: %w", statusErr))
	}

	return runErr
}

func (r *HistoricalRawCandleBackfillRunner) recordRunStatus(
	ctx context.Context,
	startedRun data.IngestionRun,
	status data.IngestionRunStatus,
	recordCount int,
	errorSummary string,
	completedAt time.Time,
) (data.IngestionRun, error) {
	completedAtValue := completedAt
	run, err := data.NewIngestionRun(data.IngestionRunParams{
		ID:           startedRun.ID,
		Source:       startedRun.Source,
		Venue:        startedRun.Venue,
		Status:       status,
		StartedAt:    startedRun.StartedAt,
		CompletedAt:  &completedAtValue,
		RecordCount:  recordCount,
		ErrorSummary: errorSummary,
	})
	if err != nil {
		return data.IngestionRun{}, fmt.Errorf("build %s ingestion run: %w", status, err)
	}

	persistedRun, err := r.recordIngestionRun(ctx, run)
	if err != nil {
		return data.IngestionRun{}, err
	}

	return persistedRun, nil
}

func canonicalizeHistoricalRawCandleBackfillRequest(
	request HistoricalRawCandleBackfillRequest,
) (HistoricalRawCandleBackfillRequest, domain.Instrument, venueedge.CandleReadRequest, error) {
	runID := strings.TrimSpace(request.RunID)
	if runID == "" {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			validationError("historical raw candle backfill run ID is required")
	}

	if request.Venue != venueedge.HyperliquidPerpsVenueName {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			validationError("historical raw candle backfill venue must be hyperliquid-perps")
	}

	symbol, err := domain.NewSymbol(request.Symbol.String())
	if err != nil {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			validationError("historical raw candle backfill symbol is required")
	}

	assetClass, err := domain.NewAssetClass(request.AssetClass.String())
	if err != nil || assetClass != domain.AssetClassFuture {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			validationError("historical raw candle backfill asset class must be future")
	}

	timeframe, err := domain.NewTimeframe(request.Timeframe.String())
	if err != nil || !isSupportedHistoricalRawCandleBackfillTimeframe(timeframe) {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			validationError("historical raw candle backfill timeframe is unsupported")
	}

	timeRange, err := domain.NewTimeRange(request.TimeRange.Start, request.TimeRange.End)
	if err != nil {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			validationError("historical raw candle backfill time range must be half-open")
	}

	if request.PageSize < 0 {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			validationError("historical raw candle backfill page size must be zero or positive")
	}

	instrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      venueedge.HyperliquidPerpsVenueName,
		Symbol:     symbol,
		AssetClass: assetClass,
		Active:     true,
	})
	if err != nil {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			validationError("historical raw candle backfill instrument is invalid")
	}

	candleReadRequest, err := venueedge.NewCandleReadRequest(venueedge.CandleReadRequestParams{
		Instrument: instrument,
		Timeframe:  timeframe,
		TimeRange:  timeRange,
		PageSize:   request.PageSize,
	})
	if err != nil {
		return HistoricalRawCandleBackfillRequest{},
			domain.Instrument{},
			venueedge.CandleReadRequest{},
			fmt.Errorf("build candle read request: %w", err)
	}

	return HistoricalRawCandleBackfillRequest{
		RunID:      runID,
		Venue:      venueedge.HyperliquidPerpsVenueName,
		Symbol:     symbol,
		AssetClass: assetClass,
		Timeframe:  timeframe,
		TimeRange:  timeRange,
		PageSize:   request.PageSize,
	}, instrument, candleReadRequest, nil
}

func isSupportedHistoricalRawCandleBackfillTimeframe(timeframe domain.Timeframe) bool {
	switch timeframe {
	case domain.Timeframe1m,
		domain.Timeframe5m,
		domain.Timeframe15m,
		domain.Timeframe1h,
		domain.Timeframe4h,
		domain.Timeframe1d:
		return true
	default:
		return false
	}
}

func conciseErrorSummary(err error) string {
	const maxSummaryLength = 240

	summary := strings.TrimSpace(err.Error())
	if len(summary) <= maxSummaryLength {
		return summary
	}

	return strings.TrimSpace(summary[:maxSummaryLength-1]) + "…"
}

func newHistoricalRawCandleBackfillReport(
	request HistoricalRawCandleBackfillRequest,
	instrument domain.Instrument,
	persistedCandles []domain.Candle,
	replayedCandles []data.ReplayCandle,
	rawPayloadCount *int,
	missingIntervalPreviewLimit int,
) (HistoricalRawCandleBackfillReport, error) {
	intervalDuration, err := historicalRawCandleBackfillTimeframeDuration(request.Timeframe)
	if err != nil {
		return HistoricalRawCandleBackfillReport{}, err
	}

	expectedIntervals := expectedHistoricalRawCandleIntervals(request.TimeRange, intervalDuration)
	persistedKeys := make(map[string]int, len(replayedCandles))
	for _, replayed := range replayedCandles {
		persistedKeys[historicalRawCandleNaturalKey(replayed.Candle)]++
	}
	if len(persistedKeys) == 0 {
		for _, candle := range persistedCandles {
			persistedKeys[historicalRawCandleNaturalKey(candle)]++
		}
	}

	missingIntervals := make([]domain.TimeRange, 0)
	for _, interval := range expectedIntervals {
		intervalKey := historicalRawCandleNaturalKey(domain.Candle{
			Instrument: instrument,
			Timeframe:  request.Timeframe,
			TimeRange:  interval,
		})
		if persistedKeys[intervalKey] == 0 {
			missingIntervals = append(missingIntervals, interval)
		}
	}

	duplicateNaturalKeyCount := 0
	for _, count := range persistedKeys {
		if count > 1 {
			duplicateNaturalKeyCount += count - 1
		}
	}

	var firstPersistedStart *time.Time
	var lastPersistedEnd *time.Time
	for _, candle := range persistedCandles {
		start := candle.TimeRange.Start
		end := candle.TimeRange.End
		if firstPersistedStart == nil || start.Before(*firstPersistedStart) {
			value := start
			firstPersistedStart = &value
		}
		if lastPersistedEnd == nil || end.After(*lastPersistedEnd) {
			value := end
			lastPersistedEnd = &value
		}
	}

	previewCount := missingIntervalPreviewLimit
	previewCount = min(previewCount, len(missingIntervals))
	missingIntervalPreview := append([]domain.TimeRange(nil), missingIntervals[:previewCount]...)

	return HistoricalRawCandleBackfillReport{
		Venue:                       request.Venue,
		Symbol:                      instrument.Symbol,
		AssetClass:                  instrument.AssetClass,
		Timeframe:                   request.Timeframe,
		TimeRange:                   request.TimeRange,
		PersistedCount:              len(persistedCandles),
		ExpectedCount:               len(expectedIntervals),
		MissingIntervalCount:        len(missingIntervals),
		DuplicateNaturalKeyCount:    duplicateNaturalKeyCount,
		FirstPersistedStart:         firstPersistedStart,
		LastPersistedEnd:            lastPersistedEnd,
		RawPayloadCount:             rawPayloadCount,
		MissingIntervalPreview:      missingIntervalPreview,
		MissingIntervalPreviewLimit: missingIntervalPreviewLimit,
	}, nil
}

func expectedHistoricalRawCandleIntervals(
	timeRange domain.TimeRange,
	intervalDuration time.Duration,
) []domain.TimeRange {
	intervals := make([]domain.TimeRange, 0)
	for current := alignHistoricalRawCandleIntervalStart(
		timeRange.Start,
		intervalDuration,
	); current.Before(timeRange.End); current = current.Add(intervalDuration) {
		intervals = append(intervals, domain.TimeRange{Start: current, End: current.Add(intervalDuration)})
	}
	return intervals
}

func alignHistoricalRawCandleIntervalStart(start time.Time, intervalDuration time.Duration) time.Time {
	aligned := start.Truncate(intervalDuration)
	if aligned.Before(start) {
		return aligned.Add(intervalDuration)
	}

	return aligned
}

func historicalRawCandleBackfillTimeframeDuration(timeframe domain.Timeframe) (time.Duration, error) {
	switch timeframe {
	case domain.Timeframe1m:
		return time.Minute, nil
	case domain.Timeframe5m:
		return historicalRawCandleBackfill5mDuration, nil
	case domain.Timeframe15m:
		return historicalRawCandleBackfill15mDuration, nil
	case domain.Timeframe1h:
		return time.Hour, nil
	case domain.Timeframe4h:
		return historicalRawCandleBackfill4hDuration, nil
	case domain.Timeframe1d:
		return historicalRawCandleBackfill1dDuration, nil
	default:
		return 0, validationError("historical raw candle backfill timeframe is unsupported")
	}
}

func historicalRawCandleNaturalKey(candle domain.Candle) string {
	return strings.Join([]string{
		candle.Instrument.Venue.String(),
		candle.Instrument.Symbol.String(),
		candle.Instrument.AssetClass.String(),
		candle.Timeframe.String(),
		strconv.FormatInt(candle.TimeRange.Start.UnixNano(), 10),
		strconv.FormatInt(candle.TimeRange.End.UnixNano(), 10),
	}, "|")
}

func isHistoricalRawCandleBackfillEmptyReadbackError(err error) bool {
	return errors.Is(err, data.ErrInstrumentNotFound) || errors.Is(err, gorm.ErrRecordNotFound)
}
