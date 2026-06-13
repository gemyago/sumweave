package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Venue identifies a trading venue in canonical domain records.
type Venue string

// Symbol identifies an instrument symbol on a venue.
type Symbol string

// AssetClass classifies an instrument into a stable canonical bucket.
type AssetClass string

const (
	AssetClassCrypto AssetClass = "crypto"
	AssetClassEquity AssetClass = "equity"
	AssetClassFX     AssetClass = "fx"
	AssetClassFuture AssetClass = "future"
	AssetClassIndex  AssetClass = "index"
	AssetClassOption AssetClass = "option"
)

// Timeframe identifies a canonical candle aggregation interval.
type Timeframe string

const (
	Timeframe1m  Timeframe = "1m"
	Timeframe5m  Timeframe = "5m"
	Timeframe15m Timeframe = "15m"
	Timeframe1h  Timeframe = "1h"
	Timeframe4h  Timeframe = "4h"
	Timeframe1d  Timeframe = "1d"
)

// DataQuality classifies the confidence state of canonical market data.
type DataQuality string

const (
	DataQualityRaw       DataQuality = "raw"
	DataQualityValidated DataQuality = "validated"
	DataQualitySuspect   DataQuality = "suspect"
)

// StrategyKind identifies a supported deterministic strategy.
type StrategyKind string

const (
	StrategyKindMovingAverageCrossover StrategyKind = "moving-average-crossover"
)

// CandidateActionKind identifies a supported strategy candidate action.
type CandidateActionKind string

const (
	CandidateActionKindLong  CandidateActionKind = "long"
	CandidateActionKindShort CandidateActionKind = "short"
)

// Instrument is the canonical venue-scoped instrument reference record.
type Instrument struct {
	Venue      Venue
	Symbol     Symbol
	AssetClass AssetClass
	Active     bool
}

// SourceProvenance describes where a canonical record came from.
type SourceProvenance struct {
	Source   string
	RecordID string
}

// TimeRange describes a canonical half-open interval [Start, End).
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Candle is the canonical candle record shared across deterministic slices.
type Candle struct {
	Instrument Instrument
	Timeframe  Timeframe
	TimeRange  TimeRange
	Open       float64
	High       float64
	Low        float64
	Close      float64
	Volume     float64
	Quality    DataQuality
	Provenance SourceProvenance
}

// Trade is the canonical trade record shared across deterministic slices.
type Trade struct {
	Instrument Instrument
	EventTime  time.Time
	Price      float64
	Size       float64
	Quality    DataQuality
	Provenance SourceProvenance
}

// IndicatorKind identifies a supported deterministic analytics calculation.
type IndicatorKind string

const (
	IndicatorKindMovingAverage IndicatorKind = "moving-average"
	IndicatorKindPeriodReturn  IndicatorKind = "period-return"
)

// IndicatorParams holds canonical parameters for a supported indicator.
type IndicatorParams struct {
	Window   int
	Lookback int
}

// AnalyticsSeriesIdentity identifies a canonical analytics series.
type AnalyticsSeriesIdentity struct {
	Instrument Instrument
	Timeframe  Timeframe
	Kind       IndicatorKind
	Parameters IndicatorParams
	TimeRange  TimeRange
}

// AnalyticsSeries holds a canonical analytics output series.
type AnalyticsSeries struct {
	Identity AnalyticsSeriesIdentity
	Points   []AnalyticsPoint
}

// StrategyIdentity identifies a canonical strategy evaluation source.
type StrategyIdentity struct {
	Instrument Instrument
	Timeframe  Timeframe
	Kind       StrategyKind
}

// AnalyticsPointTime identifies a canonical analytics point timestamp.
type AnalyticsPointTime time.Time

// CandidateActionTime identifies a canonical strategy decision timestamp.
type CandidateActionTime time.Time

// AnalyticsValueRange describes the half-open candle interval behind a point.
type AnalyticsValueRange struct {
	Start time.Time
	End   time.Time
}

// AnalyticsPoint is a canonical analytics value derived from replay inputs.
type AnalyticsPoint struct {
	Time                 AnalyticsPointTime
	ValueRange           AnalyticsValueRange
	Value                float64
	Quality              DataQuality
	SourceReplayIdentity uint64
	SourceProvenance     SourceProvenance
}

// CandidateAction is a canonical strategy output record.
type CandidateAction struct {
	Strategy     StrategyIdentity
	Kind         CandidateActionKind
	DecisionTime CandidateActionTime
	InputRange   TimeRange
	Quality      DataQuality
}

// InstrumentParams holds inputs for constructing a canonical instrument.
type InstrumentParams struct {
	Venue      Venue
	Symbol     Symbol
	AssetClass AssetClass
	Active     bool
}

// CandleParams holds inputs for constructing a canonical candle.
type CandleParams struct {
	Instrument Instrument
	Timeframe  Timeframe
	TimeRange  TimeRange
	Open       float64
	High       float64
	Low        float64
	Close      float64
	Volume     float64
	Quality    DataQuality
	Provenance SourceProvenance
}

// TradeParams holds inputs for constructing a canonical trade.
type TradeParams struct {
	Instrument Instrument
	EventTime  time.Time
	Price      float64
	Size       float64
	Quality    DataQuality
	Provenance SourceProvenance
}

// AnalyticsSeriesIdentityParams holds inputs for a series identity.
type AnalyticsSeriesIdentityParams struct {
	Instrument Instrument
	Timeframe  Timeframe
	Kind       IndicatorKind
	Parameters IndicatorParams
	TimeRange  TimeRange
}

// StrategyIdentityParams holds inputs for a strategy identity.
type StrategyIdentityParams struct {
	Instrument Instrument
	Timeframe  Timeframe
	Kind       StrategyKind
}

// AnalyticsPointParams holds inputs for a canonical analytics point.
type AnalyticsPointParams struct {
	Time                 time.Time
	ValueRange           AnalyticsValueRange
	Value                float64
	Quality              DataQuality
	SourceReplayIdentity uint64
	SourceProvenance     SourceProvenance
}

// AnalyticsSeriesParams holds inputs for a canonical analytics series.
type AnalyticsSeriesParams struct {
	Identity AnalyticsSeriesIdentity
	Points   []AnalyticsPoint
}

// CandidateActionParams holds inputs for a canonical candidate action.
type CandidateActionParams struct {
	Strategy     StrategyIdentity
	Kind         CandidateActionKind
	DecisionTime time.Time
	InputRange   TimeRange
	Quality      DataQuality
}

// NewVenue validates and canonicalizes a venue identifier.
func NewVenue(value string) (Venue, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("venue is required")
	}

	return Venue(normalized), nil
}

// NewSymbol validates and canonicalizes a venue symbol.
func NewSymbol(value string) (Symbol, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("symbol is required")
	}

	return Symbol(normalized), nil
}

// NewAssetClass validates and canonicalizes an asset class value.
func NewAssetClass(value string) (AssetClass, error) {
	normalized := AssetClass(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid asset class %q", value)
	}

	return normalized, nil
}

// NewTimeframe validates and canonicalizes a timeframe value.
func NewTimeframe(value string) (Timeframe, error) {
	normalized := Timeframe(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid timeframe %q", value)
	}

	return normalized, nil
}

// NewDataQuality validates and canonicalizes a data quality value.
func NewDataQuality(value string) (DataQuality, error) {
	normalized := DataQuality(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid data quality %q", value)
	}

	return normalized, nil
}

// NewIndicatorKind validates and canonicalizes an indicator kind.
func NewIndicatorKind(value string) (IndicatorKind, error) {
	normalized := IndicatorKind(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid indicator kind %q", value)
	}

	return normalized, nil
}

// NewStrategyKind validates and canonicalizes a strategy kind.
func NewStrategyKind(value string) (StrategyKind, error) {
	normalized := StrategyKind(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid strategy kind %q", value)
	}

	return normalized, nil
}

// NewCandidateActionKind validates and canonicalizes a candidate action kind.
func NewCandidateActionKind(value string) (CandidateActionKind, error) {
	normalized := CandidateActionKind(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid candidate action kind %q", value)
	}

	return normalized, nil
}

// NewInstrument validates and canonicalizes a canonical instrument record.
func NewInstrument(params InstrumentParams) (Instrument, error) {
	if params.Venue == "" {
		return Instrument{}, errors.New("instrument venue is required")
	}
	if params.Symbol == "" {
		return Instrument{}, errors.New("instrument symbol is required")
	}
	if !params.AssetClass.isValid() {
		return Instrument{}, errors.New("instrument asset class is required")
	}

	return Instrument(params), nil
}

// NewSourceProvenance validates and canonicalizes source provenance metadata.
func NewSourceProvenance(source, recordID string) (SourceProvenance, error) {
	normalizedSource := strings.TrimSpace(source)
	if normalizedSource == "" {
		return SourceProvenance{}, errors.New("provenance source is required")
	}

	return SourceProvenance{
		Source:   normalizedSource,
		RecordID: strings.TrimSpace(recordID),
	}, nil
}

// NewTimeRange validates and canonicalizes a canonical half-open interval.
func NewTimeRange(start, end time.Time) (TimeRange, error) {
	if start.IsZero() {
		return TimeRange{}, errors.New("time range start is required")
	}
	if end.IsZero() {
		return TimeRange{}, errors.New("time range end is required")
	}

	normalizedStart := canonicalUTC(start)
	normalizedEnd := canonicalUTC(end)
	if !normalizedEnd.After(normalizedStart) {
		return TimeRange{}, errors.New("time range end must be after start")
	}

	return TimeRange{
		Start: normalizedStart,
		End:   normalizedEnd,
	}, nil
}

// NewIndicatorParams validates and canonicalizes supported indicator parameters.
func NewIndicatorParams(kind IndicatorKind, params IndicatorParams) (IndicatorParams, error) {
	switch kind {
	case IndicatorKindMovingAverage:
		if params.Window <= 0 {
			return IndicatorParams{}, errors.New("moving average window must be positive")
		}
		if params.Lookback != 0 {
			return IndicatorParams{}, errors.New("moving average lookback must be zero")
		}
	case IndicatorKindPeriodReturn:
		if params.Lookback <= 0 {
			return IndicatorParams{}, errors.New("period return lookback must be positive")
		}
		if params.Window != 0 {
			return IndicatorParams{}, errors.New("period return window must be zero")
		}
	default:
		return IndicatorParams{}, errors.New("indicator kind is required")
	}

	return params, nil
}

// NewAnalyticsSeriesIdentity validates and canonicalizes a series identity.
func NewAnalyticsSeriesIdentity(params AnalyticsSeriesIdentityParams) (AnalyticsSeriesIdentity, error) {
	normalizedVenue, err := NewVenue(params.Instrument.Venue.String())
	if err != nil {
		return AnalyticsSeriesIdentity{}, errors.New("analytics instrument venue is required")
	}

	normalizedSymbol, err := NewSymbol(params.Instrument.Symbol.String())
	if err != nil {
		return AnalyticsSeriesIdentity{}, errors.New("analytics instrument symbol is required")
	}

	normalizedAssetClass, err := NewAssetClass(params.Instrument.AssetClass.String())
	if err != nil {
		return AnalyticsSeriesIdentity{}, fmt.Errorf("analytics instrument asset class: %w", err)
	}

	normalizedInstrument, err := NewInstrument(InstrumentParams{
		Venue:      normalizedVenue,
		Symbol:     normalizedSymbol,
		AssetClass: normalizedAssetClass,
		Active:     params.Instrument.Active,
	})
	if err != nil {
		return AnalyticsSeriesIdentity{}, fmt.Errorf("analytics instrument: %w", err)
	}

	normalizedTimeframe, err := NewTimeframe(params.Timeframe.String())
	if err != nil {
		return AnalyticsSeriesIdentity{}, errors.New("analytics timeframe is required")
	}

	normalizedKind, err := NewIndicatorKind(params.Kind.String())
	if err != nil {
		return AnalyticsSeriesIdentity{}, errors.New("analytics indicator kind is required")
	}

	normalizedParams, err := NewIndicatorParams(normalizedKind, params.Parameters)
	if err != nil {
		return AnalyticsSeriesIdentity{}, err
	}

	normalizedRange, err := NewTimeRange(params.TimeRange.Start, params.TimeRange.End)
	if err != nil {
		return AnalyticsSeriesIdentity{}, fmt.Errorf("analytics time range: %w", err)
	}

	return AnalyticsSeriesIdentity{
		Instrument: normalizedInstrument,
		Timeframe:  normalizedTimeframe,
		Kind:       normalizedKind,
		Parameters: normalizedParams,
		TimeRange:  normalizedRange,
	}, nil
}

// NewStrategyIdentity validates and canonicalizes a strategy identity.
func NewStrategyIdentity(params StrategyIdentityParams) (StrategyIdentity, error) {
	normalizedVenue, err := NewVenue(params.Instrument.Venue.String())
	if err != nil {
		return StrategyIdentity{}, errors.New("strategy instrument venue is required")
	}

	normalizedSymbol, err := NewSymbol(params.Instrument.Symbol.String())
	if err != nil {
		return StrategyIdentity{}, errors.New("strategy instrument symbol is required")
	}

	normalizedAssetClass, err := NewAssetClass(params.Instrument.AssetClass.String())
	if err != nil {
		return StrategyIdentity{}, fmt.Errorf("strategy instrument asset class: %w", err)
	}

	normalizedInstrument, err := NewInstrument(InstrumentParams{
		Venue:      normalizedVenue,
		Symbol:     normalizedSymbol,
		AssetClass: normalizedAssetClass,
		Active:     params.Instrument.Active,
	})
	if err != nil {
		return StrategyIdentity{}, fmt.Errorf("strategy instrument: %w", err)
	}

	normalizedTimeframe, err := NewTimeframe(params.Timeframe.String())
	if err != nil {
		return StrategyIdentity{}, errors.New("strategy timeframe is required")
	}

	normalizedKind, err := NewStrategyKind(params.Kind.String())
	if err != nil {
		return StrategyIdentity{}, errors.New("strategy kind is required")
	}

	return StrategyIdentity{
		Instrument: normalizedInstrument,
		Timeframe:  normalizedTimeframe,
		Kind:       normalizedKind,
	}, nil
}

// NewAnalyticsPointTime validates and canonicalizes a point timestamp.
func NewAnalyticsPointTime(value time.Time) (AnalyticsPointTime, error) {
	if value.IsZero() {
		return AnalyticsPointTime{}, errors.New("analytics point time is required")
	}

	return AnalyticsPointTime(canonicalUTC(value)), nil
}

// NewCandidateActionTime validates and canonicalizes a candidate action time.
func NewCandidateActionTime(value time.Time) (CandidateActionTime, error) {
	if value.IsZero() {
		return CandidateActionTime{}, errors.New("candidate action decision time is required")
	}

	return CandidateActionTime(canonicalUTC(value)), nil
}

// NewAnalyticsValueRange validates and canonicalizes a point value range.
func NewAnalyticsValueRange(start, end time.Time) (AnalyticsValueRange, error) {
	normalizedRange, err := NewTimeRange(start, end)
	if err != nil {
		return AnalyticsValueRange{}, fmt.Errorf("analytics value range: %w", err)
	}

	return AnalyticsValueRange(normalizedRange), nil
}

// NewAnalyticsPoint validates and canonicalizes a canonical analytics point.
func NewAnalyticsPoint(params AnalyticsPointParams) (AnalyticsPoint, error) {
	normalizedTime, err := NewAnalyticsPointTime(params.Time)
	if err != nil {
		return AnalyticsPoint{}, err
	}

	normalizedRange, err := NewAnalyticsValueRange(params.ValueRange.Start, params.ValueRange.End)
	if err != nil {
		return AnalyticsPoint{}, err
	}

	if !params.Quality.isValid() {
		return AnalyticsPoint{}, errors.New("analytics point quality is required")
	}
	if params.SourceReplayIdentity == 0 {
		return AnalyticsPoint{}, errors.New("analytics point source replay identity is required")
	}

	normalizedSource := strings.TrimSpace(params.SourceProvenance.Source)
	if normalizedSource == "" {
		return AnalyticsPoint{}, errors.New("analytics point provenance is required")
	}
	normalizedRecordID := strings.TrimSpace(params.SourceProvenance.RecordID)
	if normalizedRecordID == "" {
		return AnalyticsPoint{}, errors.New("analytics point provenance record id is required")
	}

	return AnalyticsPoint{
		Time:                 normalizedTime,
		ValueRange:           normalizedRange,
		Value:                params.Value,
		Quality:              params.Quality,
		SourceReplayIdentity: params.SourceReplayIdentity,
		SourceProvenance: SourceProvenance{
			Source:   normalizedSource,
			RecordID: normalizedRecordID,
		},
	}, nil
}

// NewAnalyticsSeries validates a canonical analytics series and point ordering.
func NewAnalyticsSeries(params AnalyticsSeriesParams) (AnalyticsSeries, error) {
	identity, err := NewAnalyticsSeriesIdentity(AnalyticsSeriesIdentityParams(params.Identity))
	if err != nil {
		return AnalyticsSeries{}, err
	}

	points := make([]AnalyticsPoint, len(params.Points))
	for idx, point := range params.Points {
		normalizedPoint, pointErr := NewAnalyticsPoint(AnalyticsPointParams{
			Time:                 point.Time.Time(),
			ValueRange:           point.ValueRange,
			Value:                point.Value,
			Quality:              point.Quality,
			SourceReplayIdentity: point.SourceReplayIdentity,
			SourceProvenance:     point.SourceProvenance,
		})
		if pointErr != nil {
			return AnalyticsSeries{}, fmt.Errorf("analytics point %d: %w", idx, pointErr)
		}

		if idx > 0 && compareAnalyticsPointOrder(points[idx-1], normalizedPoint) > 0 {
			return AnalyticsSeries{}, errors.New("analytics points must be ordered by point time and source identity")
		}

		points[idx] = normalizedPoint
	}

	return AnalyticsSeries{
		Identity: identity,
		Points:   points,
	}, nil
}

// NewCandidateAction validates and canonicalizes a canonical candidate action.
func NewCandidateAction(params CandidateActionParams) (CandidateAction, error) {
	normalizedStrategy, err := NewStrategyIdentity(StrategyIdentityParams(params.Strategy))
	if err != nil {
		return CandidateAction{}, err
	}

	normalizedKind, err := NewCandidateActionKind(params.Kind.String())
	if err != nil {
		return CandidateAction{}, errors.New("candidate action kind is required")
	}

	normalizedDecisionTime, err := NewCandidateActionTime(params.DecisionTime)
	if err != nil {
		return CandidateAction{}, err
	}

	normalizedInputRange, err := NewTimeRange(params.InputRange.Start, params.InputRange.End)
	if err != nil {
		return CandidateAction{}, fmt.Errorf("candidate action input range: %w", err)
	}

	if !params.Quality.isValid() {
		return CandidateAction{}, errors.New("candidate action quality is required")
	}

	return CandidateAction{
		Strategy:     normalizedStrategy,
		Kind:         normalizedKind,
		DecisionTime: normalizedDecisionTime,
		InputRange:   normalizedInputRange,
		Quality:      params.Quality,
	}, nil
}

// NewCandle validates and canonicalizes a canonical candle record.
func NewCandle(params CandleParams) (Candle, error) {
	if params.Instrument.Venue == "" {
		return Candle{}, errors.New("candle instrument venue is required")
	}
	if params.Instrument.Symbol == "" {
		return Candle{}, errors.New("candle instrument symbol is required")
	}
	if !params.Timeframe.isValid() {
		return Candle{}, errors.New("candle timeframe is required")
	}
	if params.TimeRange.Start.IsZero() || params.TimeRange.End.IsZero() {
		return Candle{}, errors.New("candle time range is required")
	}
	if !params.Quality.isValid() {
		return Candle{}, errors.New("candle quality is required")
	}
	if params.Provenance.Source == "" {
		return Candle{}, errors.New("candle provenance is required")
	}

	return Candle{
		Instrument: params.Instrument,
		Timeframe:  params.Timeframe,
		TimeRange: TimeRange{
			Start: canonicalUTC(params.TimeRange.Start),
			End:   canonicalUTC(params.TimeRange.End),
		},
		Open:       params.Open,
		High:       params.High,
		Low:        params.Low,
		Close:      params.Close,
		Volume:     params.Volume,
		Quality:    params.Quality,
		Provenance: params.Provenance,
	}, nil
}

// NewTrade validates and canonicalizes a canonical trade record.
func NewTrade(params TradeParams) (Trade, error) {
	if params.Instrument.Venue == "" {
		return Trade{}, errors.New("trade instrument venue is required")
	}
	if params.Instrument.Symbol == "" {
		return Trade{}, errors.New("trade instrument symbol is required")
	}
	if params.EventTime.IsZero() {
		return Trade{}, errors.New("trade event time is required")
	}
	if !params.Quality.isValid() {
		return Trade{}, errors.New("trade quality is required")
	}
	if params.Provenance.Source == "" {
		return Trade{}, errors.New("trade provenance is required")
	}

	return Trade{
		Instrument: params.Instrument,
		EventTime:  canonicalUTC(params.EventTime),
		Price:      params.Price,
		Size:       params.Size,
		Quality:    params.Quality,
		Provenance: params.Provenance,
	}, nil
}

func canonicalUTC(value time.Time) time.Time {
	return value.UTC()
}

func (a AssetClass) isValid() bool {
	switch a {
	case AssetClassCrypto, AssetClassEquity, AssetClassFX, AssetClassFuture, AssetClassIndex, AssetClassOption:
		return true
	default:
		return false
	}
}

func (k IndicatorKind) isValid() bool {
	switch k {
	case IndicatorKindMovingAverage, IndicatorKindPeriodReturn:
		return true
	default:
		return false
	}
}

func (k StrategyKind) isValid() bool {
	switch k {
	case StrategyKindMovingAverageCrossover:
		return true
	default:
		return false
	}
}

func (k CandidateActionKind) isValid() bool {
	switch k {
	case CandidateActionKindLong, CandidateActionKindShort:
		return true
	default:
		return false
	}
}

func compareAnalyticsPointOrder(left, right AnalyticsPoint) int {
	leftTime := left.Time.Time()
	rightTime := right.Time.Time()
	if leftTime.Before(rightTime) {
		return -1
	}
	if leftTime.After(rightTime) {
		return 1
	}

	if left.SourceReplayIdentity < right.SourceReplayIdentity {
		return -1
	}
	if left.SourceReplayIdentity > right.SourceReplayIdentity {
		return 1
	}

	return 0
}

func (v Venue) String() string {
	return string(v)
}

func (s Symbol) String() string {
	return string(s)
}

func (a AssetClass) String() string {
	return string(a)
}

func (t Timeframe) String() string {
	return string(t)
}

func (q DataQuality) String() string {
	return string(q)
}

func (k IndicatorKind) String() string {
	return string(k)
}

func (k StrategyKind) String() string {
	return string(k)
}

func (k CandidateActionKind) String() string {
	return string(k)
}

// Time returns the time value for a canonical analytics point timestamp.
func (t AnalyticsPointTime) Time() time.Time {
	return time.Time(t)
}

// Time returns the time value for a canonical candidate action timestamp.
func (t CandidateActionTime) Time() time.Time {
	return time.Time(t)
}

func (t Timeframe) isValid() bool {
	switch t {
	case Timeframe1m, Timeframe5m, Timeframe15m, Timeframe1h, Timeframe4h, Timeframe1d:
		return true
	default:
		return false
	}
}

func (q DataQuality) isValid() bool {
	switch q {
	case DataQualityRaw, DataQualityValidated, DataQualitySuspect:
		return true
	default:
		return false
	}
}
