package strategy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/signal-foundry/runtime/analytics"
	"github.com/gemyago/signal-foundry/runtime/domain"
)

// ErrValidation marks rejected inputs that fail strategy-layer validation.
var ErrValidation = errors.New("strategy validation failed")

// AnalyticsCalculator calculates canonical analytics series for strategy inputs.
type AnalyticsCalculator interface {
	CalculateCandles(
		ctx context.Context,
		request analytics.CalculateCandlesRequest,
	) (domain.AnalyticsSeries, error)
}

// ServiceDeps configures strategy service dependencies.
type ServiceDeps struct {
	AnalyticsCalculator AnalyticsCalculator
}

// MovingAverageCrossoverParams wraps evaluation-only crossover parameters.
type MovingAverageCrossoverParams struct {
	FastWindow int
	SlowWindow int
}

// EvaluateRequest configures deterministic strategy evaluation.
type EvaluateRequest struct {
	Instrument   domain.Instrument
	Timeframe    domain.Timeframe
	TimeRange    domain.TimeRange
	StrategyKind domain.StrategyKind
	Parameters   MovingAverageCrossoverParams
}

// EvaluateResult returns canonical strategy evaluation metadata and actions.
type EvaluateResult struct {
	Strategy   domain.StrategyIdentity
	TimeRange  domain.TimeRange
	Parameters MovingAverageCrossoverParams
	Actions    []domain.CandidateAction
}

// Service evaluates deterministic strategies from canonical analytics inputs.
type Service struct {
	analyticsCalculator AnalyticsCalculator
}

// NewService creates a strategy service with consumer-defined analytics reads.
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.AnalyticsCalculator == nil {
		return nil, errors.New("analytics calculator is required")
	}

	return &Service{
		analyticsCalculator: deps.AnalyticsCalculator,
	}, nil
}

// Evaluate evaluates a deterministic strategy for the requested range.
func (s *Service) Evaluate(
	ctx context.Context,
	request EvaluateRequest,
) (EvaluateResult, error) {
	canonicalRequest, err := canonicalizeEvaluateRequest(request)
	if err != nil {
		return EvaluateResult{}, err
	}

	switch canonicalRequest.Strategy.Kind {
	case domain.StrategyKindMovingAverageCrossover:
		return s.evaluateMovingAverageCrossover(ctx, canonicalRequest)
	default:
		return EvaluateResult{}, validationError(
			fmt.Sprintf("unsupported strategy kind %q", canonicalRequest.Strategy.Kind),
		)
	}
}

type canonicalEvaluateRequest struct {
	Strategy   domain.StrategyIdentity
	TimeRange  domain.TimeRange
	Parameters MovingAverageCrossoverParams
}

func canonicalizeEvaluateRequest(request EvaluateRequest) (canonicalEvaluateRequest, error) {
	strategy, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
		Instrument: request.Instrument,
		Timeframe:  request.Timeframe,
		Kind:       request.StrategyKind,
	})
	if err != nil {
		return canonicalEvaluateRequest{}, validationError(err.Error())
	}

	timeRange, err := domain.NewTimeRange(request.TimeRange.Start, request.TimeRange.End)
	if err != nil {
		return canonicalEvaluateRequest{}, validationError(
			fmt.Sprintf("strategy evaluation time range: %s", err.Error()),
		)
	}

	parameters, err := NewMovingAverageCrossoverParams(request.Parameters)
	if err != nil {
		return canonicalEvaluateRequest{}, validationError(err.Error())
	}

	return canonicalEvaluateRequest{
		Strategy:   strategy,
		TimeRange:  timeRange,
		Parameters: parameters,
	}, nil
}

// NewMovingAverageCrossoverParams validates evaluation-only crossover windows.
func NewMovingAverageCrossoverParams(
	params MovingAverageCrossoverParams,
) (MovingAverageCrossoverParams, error) {
	if params.FastWindow <= 0 {
		return MovingAverageCrossoverParams{}, errors.New(
			"moving average crossover fast window must be positive",
		)
	}
	if params.SlowWindow <= 0 {
		return MovingAverageCrossoverParams{}, errors.New(
			"moving average crossover slow window must be positive",
		)
	}
	if params.FastWindow >= params.SlowWindow {
		return MovingAverageCrossoverParams{}, errors.New(
			"moving average crossover fast window must be less than slow window",
		)
	}

	return params, nil
}

func (s *Service) evaluateMovingAverageCrossover(
	ctx context.Context,
	request canonicalEvaluateRequest,
) (EvaluateResult, error) {
	fastSeries, slowSeries, err := s.loadMovingAverageInputs(ctx, request)
	if err != nil {
		return EvaluateResult{}, err
	}

	actions, err := evaluateMovingAverageCrossoverActions(
		request.Strategy,
		fastSeries.Points,
		slowSeries.Points,
	)
	if err != nil {
		return EvaluateResult{}, err
	}

	return EvaluateResult{
		Strategy:   request.Strategy,
		TimeRange:  request.TimeRange,
		Parameters: request.Parameters,
		Actions:    actions,
	}, nil
}

func (s *Service) loadMovingAverageInputs(
	ctx context.Context,
	request canonicalEvaluateRequest,
) (domain.AnalyticsSeries, domain.AnalyticsSeries, error) {
	fastSeries, err := s.analyticsCalculator.CalculateCandles(ctx, analytics.CalculateCandlesRequest{
		Instrument:    request.Strategy.Instrument,
		Timeframe:     request.Strategy.Timeframe,
		TimeRange:     request.TimeRange,
		IndicatorKind: domain.IndicatorKindMovingAverage,
		IndicatorParams: domain.IndicatorParams{
			Window: request.Parameters.FastWindow,
		},
	})
	if err != nil {
		return domain.AnalyticsSeries{}, domain.AnalyticsSeries{}, fmt.Errorf(
			"calculate fast moving average analytics: %w",
			err,
		)
	}

	slowSeries, err := s.analyticsCalculator.CalculateCandles(ctx, analytics.CalculateCandlesRequest{
		Instrument:    request.Strategy.Instrument,
		Timeframe:     request.Strategy.Timeframe,
		TimeRange:     request.TimeRange,
		IndicatorKind: domain.IndicatorKindMovingAverage,
		IndicatorParams: domain.IndicatorParams{
			Window: request.Parameters.SlowWindow,
		},
	})
	if err != nil {
		return domain.AnalyticsSeries{}, domain.AnalyticsSeries{}, fmt.Errorf(
			"calculate slow moving average analytics: %w",
			err,
		)
	}

	return fastSeries, slowSeries, nil
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

type alignedMovingAveragePoint struct {
	fast domain.AnalyticsPoint
	slow domain.AnalyticsPoint
}

const minimumAlignedPointsForCrossover = 2

func evaluateMovingAverageCrossoverActions(
	strategy domain.StrategyIdentity,
	fastPoints []domain.AnalyticsPoint,
	slowPoints []domain.AnalyticsPoint,
) ([]domain.CandidateAction, error) {
	alignedPoints := alignMovingAveragePoints(fastPoints, slowPoints)
	if len(alignedPoints) < minimumAlignedPointsForCrossover {
		return nil, nil
	}

	actions := make([]domain.CandidateAction, 0, len(alignedPoints)-1)
	previous := alignedPoints[0]

	for _, current := range alignedPoints[1:] {
		actionKind, shouldEmit := crossoverActionKind(previous, current)
		if !shouldEmit {
			previous = current
			continue
		}

		inputRange, err := combinedInputRange(previous, current)
		if err != nil {
			return nil, fmt.Errorf("build candidate action input range: %w", err)
		}

		quality, err := propagatedActionQuality(previous, current)
		if err != nil {
			return nil, err
		}

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy:     strategy,
			Kind:         actionKind,
			DecisionTime: current.fast.Time.Time(),
			InputRange:   inputRange,
			Quality:      quality,
		})
		if err != nil {
			return nil, fmt.Errorf("build candidate action: %w", err)
		}

		actions = append(actions, action)
		previous = current
	}

	return actions, nil
}

func alignMovingAveragePoints(
	fastPoints []domain.AnalyticsPoint,
	slowPoints []domain.AnalyticsPoint,
) []alignedMovingAveragePoint {
	aligned := make([]alignedMovingAveragePoint, 0, min(len(fastPoints), len(slowPoints)))

	fastIdx := 0
	slowIdx := 0
	for fastIdx < len(fastPoints) && slowIdx < len(slowPoints) {
		fastTime := fastPoints[fastIdx].Time.Time()
		slowTime := slowPoints[slowIdx].Time.Time()

		switch {
		case fastTime.Before(slowTime):
			fastIdx++
		case fastTime.After(slowTime):
			slowIdx++
		default:
			aligned = append(aligned, alignedMovingAveragePoint{
				fast: fastPoints[fastIdx],
				slow: slowPoints[slowIdx],
			})
			fastIdx++
			slowIdx++
		}
	}

	return aligned
}

func crossoverActionKind(
	previous alignedMovingAveragePoint,
	current alignedMovingAveragePoint,
) (domain.CandidateActionKind, bool) {
	switch {
	case previous.fast.Value <= previous.slow.Value && current.fast.Value > current.slow.Value:
		return domain.CandidateActionKindLong, true
	case previous.fast.Value >= previous.slow.Value && current.fast.Value < current.slow.Value:
		return domain.CandidateActionKindShort, true
	default:
		return "", false
	}
}

func combinedInputRange(
	previous alignedMovingAveragePoint,
	current alignedMovingAveragePoint,
) (domain.TimeRange, error) {
	start := previous.fast.ValueRange.Start
	end := previous.fast.ValueRange.End

	for _, candidate := range []domain.AnalyticsValueRange{
		previous.slow.ValueRange,
		current.fast.ValueRange,
		current.slow.ValueRange,
	} {
		start = minTime(start, candidate.Start)
		end = maxTime(end, candidate.End)
	}

	return domain.NewTimeRange(start, end)
}

func propagatedActionQuality(
	previous alignedMovingAveragePoint,
	current alignedMovingAveragePoint,
) (domain.DataQuality, error) {
	qualities := []struct {
		label   string
		quality domain.DataQuality
	}{
		{label: "previous fast", quality: previous.fast.Quality},
		{label: "previous slow", quality: previous.slow.Quality},
		{label: "current fast", quality: current.fast.Quality},
		{label: "current slow", quality: current.slow.Quality},
	}

	hasSuspect := false
	hasRaw := false
	for _, quality := range qualities {
		switch quality.quality {
		case domain.DataQualitySuspect:
			hasSuspect = true
		case domain.DataQualityRaw:
			hasRaw = true
		case domain.DataQualityValidated:
			continue
		default:
			return "", validationError(
				fmt.Sprintf(
					"unsupported analytics quality %q for %s aligned point",
					quality.quality,
					quality.label,
				),
			)
		}
	}

	if hasSuspect {
		return domain.DataQualitySuspect, nil
	}

	if hasRaw {
		return domain.DataQualityRaw, nil
	}

	return domain.DataQualityValidated, nil
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}

	return right
}

func maxTime(left, right time.Time) time.Time {
	if left.After(right) {
		return left
	}

	return right
}
