package analytics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
)

// ErrValidation marks rejected inputs that fail analytics-layer validation.
var ErrValidation = errors.New("analytics validation failed")

// CandleReplayReader loads canonical replay candles with stable replay identities.
type CandleReplayReader interface {
	ReplayCandles(
		ctx context.Context,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		timeRange domain.TimeRange,
	) ([]data.ReplayCandle, error)
}

// ServiceDeps configures analytics service dependencies.
type ServiceDeps struct {
	CandleReplayReader CandleReplayReader
}

// CalculateCandlesRequest configures a candle-derived analytics calculation.
type CalculateCandlesRequest struct {
	Instrument      domain.Instrument
	Timeframe       domain.Timeframe
	TimeRange       domain.TimeRange
	IndicatorKind   domain.IndicatorKind
	IndicatorParams domain.IndicatorParams
}

// Service calculates deterministic analytics from canonical replay candles.
type Service struct {
	candleReplayReader CandleReplayReader
}

// NewService creates an analytics service with consumer-defined replay reads.
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.CandleReplayReader == nil {
		return nil, errors.New("candle replay reader is required")
	}

	return &Service{
		candleReplayReader: deps.CandleReplayReader,
	}, nil
}

// CalculateCandles calculates a canonical analytics series from replay candles.
func (s *Service) CalculateCandles(
	ctx context.Context,
	request CalculateCandlesRequest,
) (domain.AnalyticsSeries, error) {
	canonicalRequest, err := canonicalizeCalculateCandlesRequest(request)
	if err != nil {
		return domain.AnalyticsSeries{}, err
	}

	replayCandles, err := s.candleReplayReader.ReplayCandles(
		ctx,
		canonicalRequest.Instrument,
		canonicalRequest.Timeframe,
		canonicalRequest.TimeRange,
	)
	if err != nil {
		return domain.AnalyticsSeries{}, fmt.Errorf("replay candles: %w", err)
	}
	replayCandles, seriesInstrument, err := normalizeReplayCandles(canonicalRequest, replayCandles)
	if err != nil {
		return domain.AnalyticsSeries{}, err
	}

	seriesIdentity, err := domain.NewAnalyticsSeriesIdentity(domain.AnalyticsSeriesIdentityParams{
		Instrument: seriesInstrument,
		Timeframe:  canonicalRequest.Timeframe,
		Kind:       canonicalRequest.IndicatorKind,
		Parameters: canonicalRequest.IndicatorParams,
		TimeRange:  canonicalRequest.TimeRange,
	})
	if err != nil {
		return domain.AnalyticsSeries{}, validationError(err.Error())
	}

	points, err := calculatePoints(canonicalRequest, replayCandles)
	if err != nil {
		return domain.AnalyticsSeries{}, err
	}
	normalizeAnalyticsPoints(points)

	series, err := domain.NewAnalyticsSeries(domain.AnalyticsSeriesParams{
		Identity: seriesIdentity,
		Points:   points,
	})
	if err != nil {
		return domain.AnalyticsSeries{}, fmt.Errorf("build analytics series: %w", err)
	}

	return series, nil
}

func canonicalizeCalculateCandlesRequest(
	request CalculateCandlesRequest,
) (CalculateCandlesRequest, error) {
	venue, err := domain.NewVenue(request.Instrument.Venue.String())
	if err != nil {
		return CalculateCandlesRequest{}, validationError("instrument venue is required")
	}

	symbol, err := domain.NewSymbol(request.Instrument.Symbol.String())
	if err != nil {
		return CalculateCandlesRequest{}, validationError("instrument symbol is required")
	}

	if request.Instrument.AssetClass == "" {
		return CalculateCandlesRequest{}, validationError("instrument asset class is required")
	}

	assetClass, err := domain.NewAssetClass(request.Instrument.AssetClass.String())
	if err != nil {
		return CalculateCandlesRequest{}, validationError(err.Error())
	}

	timeframe, err := domain.NewTimeframe(request.Timeframe.String())
	if err != nil {
		return CalculateCandlesRequest{}, validationError("candle timeframe is required")
	}

	timeRange, err := domain.NewTimeRange(request.TimeRange.Start, request.TimeRange.End)
	if err != nil {
		return CalculateCandlesRequest{}, validationError(err.Error())
	}

	indicatorKind, err := domain.NewIndicatorKind(request.IndicatorKind.String())
	if err != nil {
		return CalculateCandlesRequest{}, validationError(err.Error())
	}

	indicatorParams, err := domain.NewIndicatorParams(indicatorKind, request.IndicatorParams)
	if err != nil {
		return CalculateCandlesRequest{}, validationError(err.Error())
	}

	instrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
		Active:     request.Instrument.Active,
	})
	if err != nil {
		return CalculateCandlesRequest{}, validationError(err.Error())
	}

	return CalculateCandlesRequest{
		Instrument:      instrument,
		Timeframe:       timeframe,
		TimeRange:       timeRange,
		IndicatorKind:   indicatorKind,
		IndicatorParams: indicatorParams,
	}, nil
}

func calculatePoints(
	request CalculateCandlesRequest,
	replayCandles []data.ReplayCandle,
) ([]domain.AnalyticsPoint, error) {
	switch request.IndicatorKind {
	case domain.IndicatorKindMovingAverage:
		return calculateMovingAveragePoints(replayCandles, request.IndicatorParams.Window)
	case domain.IndicatorKindPeriodReturn:
		return calculatePeriodReturnPoints(replayCandles, request.IndicatorParams.Lookback)
	default:
		return nil, validationError(fmt.Sprintf("unsupported indicator kind %q", request.IndicatorKind))
	}
}

func validateReplayCandlesMatchRequest(
	request CalculateCandlesRequest,
	replayCandles []data.ReplayCandle,
) (domain.Instrument, error) {
	seriesInstrument := request.Instrument

	for idx, replayCandle := range replayCandles {
		if replayCandle.Identity == 0 {
			return domain.Instrument{}, validationError(
				fmt.Sprintf("replay candle %d identity is required", idx),
			)
		}

		replayInstrument, err := canonicalizeReplayInstrument(replayCandle.Candle.Instrument)
		if err != nil {
			return domain.Instrument{}, validationError(
				fmt.Sprintf("replay candle %d instrument %s", idx, err.Error()),
			)
		}

		switch {
		case replayInstrument.Venue != request.Instrument.Venue:
			return domain.Instrument{}, validationError(
				fmt.Sprintf(
					"replay candle %d venue mismatch: expected %q, got %q",
					idx,
					request.Instrument.Venue,
					replayInstrument.Venue,
				),
			)
		case replayInstrument.Symbol != request.Instrument.Symbol:
			return domain.Instrument{}, validationError(
				fmt.Sprintf(
					"replay candle %d symbol mismatch: expected %q, got %q",
					idx,
					request.Instrument.Symbol,
					replayInstrument.Symbol,
				),
			)
		case replayInstrument.AssetClass != request.Instrument.AssetClass:
			return domain.Instrument{}, validationError(
				fmt.Sprintf(
					"replay candle %d asset class mismatch: expected %q, got %q",
					idx,
					request.Instrument.AssetClass,
					replayInstrument.AssetClass,
				),
			)
		}

		replayTimeframe, err := domain.NewTimeframe(replayCandle.Candle.Timeframe.String())
		if err != nil {
			return domain.Instrument{}, validationError(
				fmt.Sprintf("replay candle %d timeframe is required", idx),
			)
		}
		if replayTimeframe != request.Timeframe {
			return domain.Instrument{}, validationError(
				fmt.Sprintf(
					"replay candle %d timeframe mismatch: expected %q, got %q",
					idx,
					request.Timeframe,
					replayTimeframe,
				),
			)
		}

		replayStart := replayCandle.Candle.TimeRange.Start
		if replayStart.Before(request.TimeRange.Start) || !replayStart.Before(request.TimeRange.End) {
			return domain.Instrument{}, validationError(
				fmt.Sprintf(
					"replay candle %d start time %s is outside requested range [%s, %s)",
					idx,
					replayStart.UTC().Format(time.RFC3339Nano),
					request.TimeRange.Start.UTC().Format(time.RFC3339Nano),
					request.TimeRange.End.UTC().Format(time.RFC3339Nano),
				),
			)
		}

		replayCandles[idx].Candle.Instrument = replayInstrument
		replayCandles[idx].Candle.Timeframe = replayTimeframe
		if idx == 0 {
			seriesInstrument = replayInstrument
		}
	}

	return seriesInstrument, nil
}

func normalizeReplayCandles(
	request CalculateCandlesRequest,
	replayCandles []data.ReplayCandle,
) ([]data.ReplayCandle, domain.Instrument, error) {
	if len(replayCandles) == 0 {
		return []data.ReplayCandle{}, request.Instrument, nil
	}

	normalizedReplayCandles := make([]data.ReplayCandle, len(replayCandles))
	copy(normalizedReplayCandles, replayCandles)

	seriesInstrument, err := validateReplayCandlesMatchRequest(request, normalizedReplayCandles)
	if err != nil {
		return nil, domain.Instrument{}, err
	}

	return normalizedReplayCandles, seriesInstrument, nil
}

func canonicalizeReplayInstrument(instrument domain.Instrument) (domain.Instrument, error) {
	venue, err := domain.NewVenue(instrument.Venue.String())
	if err != nil {
		return domain.Instrument{}, errors.New("venue is required")
	}

	symbol, err := domain.NewSymbol(instrument.Symbol.String())
	if err != nil {
		return domain.Instrument{}, errors.New("symbol is required")
	}

	if instrument.AssetClass == "" {
		return domain.Instrument{}, errors.New("asset class is required")
	}

	assetClass, err := domain.NewAssetClass(instrument.AssetClass.String())
	if err != nil {
		return domain.Instrument{}, fmt.Errorf("asset class: %w", err)
	}

	canonicalInstrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      venue,
		Symbol:     symbol,
		AssetClass: assetClass,
		Active:     instrument.Active,
	})
	if err != nil {
		return domain.Instrument{}, err
	}

	return canonicalInstrument, nil
}

func normalizeAnalyticsPoints(points []domain.AnalyticsPoint) {
	sort.SliceStable(points, func(leftIdx, rightIdx int) bool {
		leftTime := points[leftIdx].Time.Time()
		rightTime := points[rightIdx].Time.Time()
		if leftTime.Before(rightTime) {
			return true
		}
		if leftTime.After(rightTime) {
			return false
		}

		return points[leftIdx].SourceReplayIdentity < points[rightIdx].SourceReplayIdentity
	})
}

func calculateMovingAveragePoints(
	replayCandles []data.ReplayCandle,
	window int,
) ([]domain.AnalyticsPoint, error) {
	if len(replayCandles) < window {
		return []domain.AnalyticsPoint{}, nil
	}

	points := make([]domain.AnalyticsPoint, 0, len(replayCandles)-window+1)
	var runningCloseSum float64

	for idx, replayCandle := range replayCandles {
		if replayCandle.Identity == 0 {
			return nil, errors.New("replay candle identity is required")
		}

		runningCloseSum += replayCandle.Candle.Close
		if idx >= window {
			runningCloseSum -= replayCandles[idx-window].Candle.Close
		}
		if idx < window-1 {
			continue
		}

		windowCandles := replayCandles[idx-window+1 : idx+1]
		quality, err := propagateQuality(windowCandles)
		if err != nil {
			return nil, err
		}

		point, err := buildPoint(
			replayCandle,
			windowCandles[0].Candle.TimeRange.Start,
			runningCloseSum/float64(window),
			quality,
		)
		if err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, nil
}

func calculatePeriodReturnPoints(
	replayCandles []data.ReplayCandle,
	lookback int,
) ([]domain.AnalyticsPoint, error) {
	if len(replayCandles) <= lookback {
		return []domain.AnalyticsPoint{}, nil
	}

	points := make([]domain.AnalyticsPoint, 0, len(replayCandles)-lookback)

	for idx := lookback; idx < len(replayCandles); idx++ {
		currentReplay := replayCandles[idx]
		if currentReplay.Identity == 0 {
			return nil, errors.New("replay candle identity is required")
		}

		lookbackReplay := replayCandles[idx-lookback]
		lookbackClose := lookbackReplay.Candle.Close
		if lookbackClose <= 0 {
			return nil, validationError("period return lookback close must be positive")
		}

		contributingCandles := replayCandles[idx-lookback : idx+1]
		quality, err := propagateQuality(contributingCandles)
		if err != nil {
			return nil, err
		}

		point, err := buildPoint(
			currentReplay,
			lookbackReplay.Candle.TimeRange.Start,
			(currentReplay.Candle.Close-lookbackClose)/lookbackClose,
			quality,
		)
		if err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, nil
}

func buildPoint(
	currentReplay data.ReplayCandle,
	valueRangeStartTime time.Time,
	value float64,
	quality domain.DataQuality,
) (domain.AnalyticsPoint, error) {
	provenanceRecordID := currentReplay.Candle.Provenance.RecordID
	if provenanceRecordID == "" {
		provenanceRecordID = strconv.FormatUint(currentReplay.Identity, 10)
	}

	point, err := domain.NewAnalyticsPoint(domain.AnalyticsPointParams{
		Time: currentReplay.Candle.TimeRange.End,
		ValueRange: domain.AnalyticsValueRange{
			Start: valueRangeStartTime,
			End:   currentReplay.Candle.TimeRange.End,
		},
		Value:                value,
		Quality:              quality,
		SourceReplayIdentity: currentReplay.Identity,
		SourceProvenance: domain.SourceProvenance{
			Source:   currentReplay.Candle.Provenance.Source,
			RecordID: provenanceRecordID,
		},
	})
	if err != nil {
		return domain.AnalyticsPoint{}, fmt.Errorf("build analytics point: %w", err)
	}

	return point, nil
}

func propagateQuality(replayCandles []data.ReplayCandle) (domain.DataQuality, error) {
	hasRaw := false

	for _, replayCandle := range replayCandles {
		switch replayCandle.Candle.Quality {
		case domain.DataQualitySuspect:
			return domain.DataQualitySuspect, nil
		case domain.DataQualityRaw:
			hasRaw = true
		case domain.DataQualityValidated:
		default:
			return "", fmt.Errorf("unsupported candle quality %q", replayCandle.Candle.Quality)
		}
	}

	if hasRaw {
		return domain.DataQualityRaw, nil
	}

	return domain.DataQualityValidated, nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
