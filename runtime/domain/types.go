package domain

import (
	"errors"
	"fmt"
	"math"
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

// GovernorDecisionStatus identifies a supported governor decision outcome.
type GovernorDecisionStatus string

const (
	GovernorDecisionStatusApproved GovernorDecisionStatus = "approved"
	GovernorDecisionStatusRejected GovernorDecisionStatus = "rejected"
	GovernorDecisionStatusBlocked  GovernorDecisionStatus = "blocked"
)

// GovernorDecisionReason identifies a stable governor decision reason.
type GovernorDecisionReason string

const (
	GovernorDecisionReasonOK                             GovernorDecisionReason = "OK"
	GovernorDecisionReasonModeNotAllowed                 GovernorDecisionReason = "MODE_NOT_ALLOWED"
	GovernorDecisionReasonVenueNotAllowed                GovernorDecisionReason = "VENUE_NOT_ALLOWED"
	GovernorDecisionReasonInstrumentNotAllowed           GovernorDecisionReason = "INSTRUMENT_NOT_ALLOWED"
	GovernorDecisionReasonStrategyNotAllowed             GovernorDecisionReason = "STRATEGY_NOT_ALLOWED"
	GovernorDecisionReasonActionKindNotAllowed           GovernorDecisionReason = "ACTION_KIND_NOT_ALLOWED"
	GovernorDecisionReasonDataQualityTooLow              GovernorDecisionReason = "DATA_QUALITY_TOO_LOW"
	GovernorDecisionReasonKillSwitchActive               GovernorDecisionReason = "KILL_SWITCH_ACTIVE"
	GovernorDecisionReasonOrderNotionalExceedsLimit      GovernorDecisionReason = "ORDER_NOTIONAL_EXCEEDS_LIMIT"
	GovernorDecisionReasonStrategyExposureExceedsLimit   GovernorDecisionReason = "STRATEGY_EXPOSURE_EXCEEDS_LIMIT"
	GovernorDecisionReasonInstrumentExposureExceedsLimit GovernorDecisionReason = "INSTRUMENT_EXPOSURE_EXCEEDS_LIMIT"
	GovernorDecisionReasonApprovalLimitReached           GovernorDecisionReason = "APPROVAL_LIMIT_REACHED"
	GovernorDecisionReasonInvalidIntent                  GovernorDecisionReason = "INVALID_INTENT"

	GovernorDecisionReasonEligible             GovernorDecisionReason = GovernorDecisionReasonOK
	GovernorDecisionReasonDisallowedActionKind GovernorDecisionReason = GovernorDecisionReasonActionKindNotAllowed
	GovernorDecisionReasonBelowMinimumQuality  GovernorDecisionReason = GovernorDecisionReasonDataQualityTooLow
)

// ExecutionCommandStatus identifies a supported execution command state.
type ExecutionCommandStatus string

const (
	ExecutionCommandStatusCreated ExecutionCommandStatus = "created"
)

// ExecutionOrderStatus identifies a supported local execution order state.
type ExecutionOrderStatus string

const (
	ExecutionOrderStatusOpen            ExecutionOrderStatus = "open"
	ExecutionOrderStatusPartiallyFilled ExecutionOrderStatus = "partially-filled"
	ExecutionOrderStatusFilled          ExecutionOrderStatus = "filled"
	ExecutionOrderStatusOverfilled      ExecutionOrderStatus = "overfilled"
)

// TimeInForce identifies a supported execution order time-in-force.
type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "gtc"
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

// GovernorDecisionTime identifies a canonical governor decision timestamp.
type GovernorDecisionTime time.Time

// ExecutionEventTime identifies a canonical execution event timestamp.
type ExecutionEventTime time.Time

// ExecutionCommandID identifies a canonical execution command.
type ExecutionCommandID string

// ExecutionOrderID identifies a canonical local execution order.
type ExecutionOrderID string

// ExecutionFillID identifies a canonical local execution fill.
type ExecutionFillID string

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

// GovernorDecision is a canonical governor output record.
type GovernorDecision struct {
	CandidateAction CandidateAction
	Status          GovernorDecisionStatus
	Reason          GovernorDecisionReason
	DecisionTime    GovernorDecisionTime
}

// ExecutionCommand is a canonical execution admission record.
type ExecutionCommand struct {
	CommandID                 ExecutionCommandID
	TraceID                   DecisionTraceID
	IntentID                  OrderIntentID
	Mode                      DecisionMode
	StrategyID                string
	StrategyVersion           string
	StrategyArtifactHash      string
	Venue                     Venue
	Instrument                Instrument
	ActionKind                CandidateActionKind
	OrderType                 OrderType
	LimitPrice                *float64
	ReduceOnly                bool
	GovernorDecisionReference string
	ApprovedDecision          GovernorDecision
	Status                    ExecutionCommandStatus
	Quantity                  float64
	Notional                  float64
	EventTime                 ExecutionEventTime
}

// ExecutionOrder is a canonical local execution order record.
type ExecutionOrder struct {
	OrderID              ExecutionOrderID
	Command              ExecutionCommand
	Mode                 DecisionMode
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	Venue                Venue
	Instrument           Instrument
	OrderType            OrderType
	TimeInForce          TimeInForce
	ReduceOnly           bool
	ClientOrderID        string
	Status               ExecutionOrderStatus
	Quantity             float64
	Notional             float64
	LimitPrice           *float64
	EventTime            ExecutionEventTime
}

// ExecutionFill is a canonical local execution fill record.
type ExecutionFill struct {
	FillID                    ExecutionFillID
	Order                     ExecutionOrder
	SourceMarketDataReference string
	FeeAmount                 float64
	SlippageAmount            float64
	Metadata                  map[string]string
	Quantity                  float64
	Price                     float64
	EventTime                 ExecutionEventTime
}

// ExecutionReconciliation is a canonical local execution reconciliation record.
type ExecutionReconciliation struct {
	Order          ExecutionOrder
	Fills          []ExecutionFill
	Status         ExecutionOrderStatus
	FilledQuantity float64
	EventTime      ExecutionEventTime
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

// GovernorDecisionParams holds inputs for a canonical governor decision.
type GovernorDecisionParams struct {
	CandidateAction CandidateAction
	Status          GovernorDecisionStatus
	Reason          GovernorDecisionReason
	DecisionTime    time.Time
}

// ExecutionCommandParams holds inputs for a canonical execution command.
type ExecutionCommandParams struct {
	CommandID                 string
	TraceID                   string
	IntentID                  string
	Mode                      DecisionMode
	StrategyID                string
	StrategyVersion           string
	StrategyArtifactHash      string
	Venue                     Venue
	Instrument                Instrument
	ActionKind                CandidateActionKind
	OrderType                 OrderType
	LimitPrice                *float64
	ReduceOnly                bool
	GovernorDecisionReference string
	ApprovedDecision          GovernorDecision
	Status                    ExecutionCommandStatus
	Quantity                  float64
	Notional                  float64
	EventTime                 time.Time
}

// ExecutionOrderParams holds inputs for a canonical execution order.
type ExecutionOrderParams struct {
	OrderID              string
	Command              ExecutionCommand
	Mode                 DecisionMode
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	Venue                Venue
	Instrument           Instrument
	OrderType            OrderType
	TimeInForce          TimeInForce
	ReduceOnly           bool
	ClientOrderID        string
	Status               ExecutionOrderStatus
	Quantity             float64
	Notional             float64
	LimitPrice           *float64
	EventTime            time.Time
}

// ExecutionFillParams holds inputs for a canonical execution fill.
type ExecutionFillParams struct {
	FillID                    string
	Order                     ExecutionOrder
	SourceMarketDataReference string
	FeeAmount                 float64
	SlippageAmount            float64
	Metadata                  map[string]string
	Quantity                  float64
	Price                     float64
	EventTime                 time.Time
}

// ExecutionReconciliationParams holds inputs for a canonical reconciliation record.
type ExecutionReconciliationParams struct {
	Order          ExecutionOrder
	Fills          []ExecutionFill
	Status         ExecutionOrderStatus
	FilledQuantity float64
	EventTime      time.Time
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

// NewGovernorDecisionStatus validates and canonicalizes a governor decision status.
func NewGovernorDecisionStatus(value string) (GovernorDecisionStatus, error) {
	normalized := GovernorDecisionStatus(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid governor decision status %q", value)
	}

	return normalized, nil
}

// NewGovernorDecisionReason validates and canonicalizes a governor decision reason.
func NewGovernorDecisionReason(value string) (GovernorDecisionReason, error) {
	normalized := GovernorDecisionReason(strings.ToUpper(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid governor decision reason %q", value)
	}

	return normalized, nil
}

// NewExecutionCommandStatus validates and canonicalizes an execution command status.
func NewExecutionCommandStatus(value string) (ExecutionCommandStatus, error) {
	normalized := ExecutionCommandStatus(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid execution command status %q", value)
	}

	return normalized, nil
}

// NewExecutionOrderStatus validates and canonicalizes an execution order status.
func NewExecutionOrderStatus(value string) (ExecutionOrderStatus, error) {
	normalized := ExecutionOrderStatus(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid execution order status %q", value)
	}

	return normalized, nil
}

// NewTimeInForce validates and canonicalizes an execution order time-in-force.
func NewTimeInForce(value string) (TimeInForce, error) {
	normalized := TimeInForce(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid time in force %q", value)
	}

	return normalized, nil
}

// NewExecutionCommandID validates and canonicalizes an execution command identifier.
func NewExecutionCommandID(value string) (ExecutionCommandID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("execution command id is required")
	}

	return ExecutionCommandID(normalized), nil
}

// NewExecutionOrderID validates and canonicalizes an execution order identifier.
func NewExecutionOrderID(value string) (ExecutionOrderID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("execution order id is required")
	}

	return ExecutionOrderID(normalized), nil
}

// NewExecutionFillID validates and canonicalizes an execution fill identifier.
func NewExecutionFillID(value string) (ExecutionFillID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("execution fill id is required")
	}

	return ExecutionFillID(normalized), nil
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

	if !end.After(start) {
		return TimeRange{}, errors.New("time range end must be after start")
	}

	return TimeRange{Start: start, End: end}, nil
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

	return AnalyticsPointTime(value), nil
}

// NewCandidateActionTime validates and canonicalizes a candidate action time.
func NewCandidateActionTime(value time.Time) (CandidateActionTime, error) {
	if value.IsZero() {
		return CandidateActionTime{}, errors.New("candidate action decision time is required")
	}

	return CandidateActionTime(value), nil
}

// NewGovernorDecisionTime validates and canonicalizes a governor decision time.
func NewGovernorDecisionTime(value time.Time) (GovernorDecisionTime, error) {
	if value.IsZero() {
		return GovernorDecisionTime{}, errors.New("governor decision time is required")
	}

	return GovernorDecisionTime(value), nil
}

// NewExecutionEventTime validates and canonicalizes an execution event time.
func NewExecutionEventTime(value time.Time) (ExecutionEventTime, error) {
	if value.IsZero() {
		return ExecutionEventTime{}, errors.New("execution event time is required")
	}

	return ExecutionEventTime(value), nil
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

// NewGovernorDecision validates and canonicalizes a canonical governor decision.
func NewGovernorDecision(params GovernorDecisionParams) (GovernorDecision, error) {
	normalizedAction, err := NewCandidateAction(CandidateActionParams{
		Strategy:     params.CandidateAction.Strategy,
		Kind:         params.CandidateAction.Kind,
		DecisionTime: params.CandidateAction.DecisionTime.Time(),
		InputRange:   params.CandidateAction.InputRange,
		Quality:      params.CandidateAction.Quality,
	})
	if err != nil {
		return GovernorDecision{}, fmt.Errorf("governor candidate action: %w", err)
	}

	normalizedStatus, err := NewGovernorDecisionStatus(params.Status.String())
	if err != nil {
		return GovernorDecision{}, errors.New("governor decision status is required")
	}

	normalizedReason, err := NewGovernorDecisionReason(params.Reason.String())
	if err != nil {
		return GovernorDecision{}, errors.New("governor decision reason is required")
	}

	normalizedDecisionTime, err := NewGovernorDecisionTime(params.DecisionTime)
	if err != nil {
		return GovernorDecision{}, err
	}

	return GovernorDecision{
		CandidateAction: normalizedAction,
		Status:          normalizedStatus,
		Reason:          normalizedReason,
		DecisionTime:    normalizedDecisionTime,
	}, nil
}

// NewExecutionCommand validates and canonicalizes a canonical execution command.
func NewExecutionCommand(params ExecutionCommandParams) (ExecutionCommand, error) {
	normalizedCommandID, err := NewExecutionCommandID(params.CommandID)
	if err != nil {
		return ExecutionCommand{}, err
	}

	contextFields, err := canonicalizeExecutionCommandContext(params)
	if err != nil {
		return ExecutionCommand{}, err
	}

	normalizedDecision, err := NewGovernorDecision(GovernorDecisionParams{
		CandidateAction: params.ApprovedDecision.CandidateAction,
		Status:          params.ApprovedDecision.Status,
		Reason:          params.ApprovedDecision.Reason,
		DecisionTime:    params.ApprovedDecision.DecisionTime.Time(),
	})
	if err != nil {
		return ExecutionCommand{}, fmt.Errorf("execution approved decision: %w", err)
	}
	if normalizedDecision.Status != GovernorDecisionStatusApproved {
		return ExecutionCommand{}, errors.New("execution approved decision must be approved")
	}

	normalizedStatus, err := NewExecutionCommandStatus(params.Status.String())
	if err != nil {
		return ExecutionCommand{}, errors.New("execution command status is required")
	}
	if !isFiniteFloat64(params.Quantity) {
		return ExecutionCommand{}, errors.New("execution command quantity must be finite")
	}
	if params.Quantity <= 0 {
		return ExecutionCommand{}, errors.New("execution command quantity must be positive")
	}
	if !isFiniteFloat64(params.Notional) {
		return ExecutionCommand{}, errors.New("execution command notional must be finite")
	}
	if params.Notional < 0 {
		return ExecutionCommand{}, errors.New("execution command notional must be zero or greater")
	}

	normalizedEventTime, err := NewExecutionEventTime(params.EventTime)
	if err != nil {
		return ExecutionCommand{}, err
	}

	return ExecutionCommand{
		CommandID:                 normalizedCommandID,
		TraceID:                   contextFields.traceID,
		IntentID:                  contextFields.intentID,
		Mode:                      contextFields.mode,
		StrategyID:                contextFields.strategyID,
		StrategyVersion:           contextFields.strategyVersion,
		StrategyArtifactHash:      contextFields.strategyArtifactHash,
		Venue:                     contextFields.venue,
		Instrument:                contextFields.instrument,
		ActionKind:                contextFields.actionKind,
		OrderType:                 contextFields.orderType,
		LimitPrice:                contextFields.limitPrice,
		ReduceOnly:                params.ReduceOnly,
		GovernorDecisionReference: contextFields.governorDecisionReference,
		ApprovedDecision:          normalizedDecision,
		Status:                    normalizedStatus,
		Quantity:                  params.Quantity,
		Notional:                  params.Notional,
		EventTime:                 normalizedEventTime,
	}, nil
}

// NewExecutionOrder validates and canonicalizes a canonical execution order.
func NewExecutionOrder(params ExecutionOrderParams) (ExecutionOrder, error) {
	normalizedOrderID, err := NewExecutionOrderID(params.OrderID)
	if err != nil {
		return ExecutionOrder{}, err
	}

	normalizedCommand, err := NewExecutionCommand(ExecutionCommandParams{
		CommandID:                 string(params.Command.CommandID),
		TraceID:                   string(params.Command.TraceID),
		IntentID:                  string(params.Command.IntentID),
		Mode:                      params.Command.Mode,
		StrategyID:                params.Command.StrategyID,
		StrategyVersion:           params.Command.StrategyVersion,
		StrategyArtifactHash:      params.Command.StrategyArtifactHash,
		Venue:                     params.Command.Venue,
		Instrument:                params.Command.Instrument,
		ActionKind:                params.Command.ActionKind,
		OrderType:                 params.Command.OrderType,
		LimitPrice:                params.Command.LimitPrice,
		ReduceOnly:                params.Command.ReduceOnly,
		GovernorDecisionReference: params.Command.GovernorDecisionReference,
		ApprovedDecision:          params.Command.ApprovedDecision,
		Status:                    params.Command.Status,
		Quantity:                  params.Command.Quantity,
		Notional:                  params.Command.Notional,
		EventTime:                 params.Command.EventTime.Time(),
	})
	if err != nil {
		return ExecutionOrder{}, fmt.Errorf("execution order command: %w", err)
	}

	contextFields, err := canonicalizeExecutionOrderContext(params, normalizedCommand)
	if err != nil {
		return ExecutionOrder{}, err
	}

	normalizedClientOrderID := strings.TrimSpace(params.ClientOrderID)
	if normalizedClientOrderID == "" {
		return ExecutionOrder{}, errors.New("execution order client order id is required")
	}

	normalizedStatus, err := NewExecutionOrderStatus(params.Status.String())
	if err != nil {
		return ExecutionOrder{}, errors.New("execution order status is required")
	}
	if !isFiniteFloat64(params.Quantity) {
		return ExecutionOrder{}, errors.New("execution order quantity must be finite")
	}
	if params.Quantity <= 0 {
		return ExecutionOrder{}, errors.New("execution order quantity must be positive")
	}

	normalizedNotional := contextFields.notional
	if !isFiniteFloat64(normalizedNotional) {
		return ExecutionOrder{}, errors.New("execution order notional must be finite")
	}
	if normalizedNotional < 0 {
		return ExecutionOrder{}, errors.New("execution order notional must be zero or greater")
	}

	normalizedEventTime, err := NewExecutionEventTime(params.EventTime)
	if err != nil {
		return ExecutionOrder{}, err
	}

	return ExecutionOrder{
		OrderID:              normalizedOrderID,
		Command:              normalizedCommand,
		Mode:                 contextFields.mode,
		StrategyID:           contextFields.strategyID,
		StrategyVersion:      contextFields.strategyVersion,
		StrategyArtifactHash: contextFields.strategyArtifactHash,
		Venue:                contextFields.venue,
		Instrument:           contextFields.instrument,
		OrderType:            contextFields.orderType,
		TimeInForce:          contextFields.timeInForce,
		ReduceOnly:           contextFields.reduceOnly,
		ClientOrderID:        normalizedClientOrderID,
		Status:               normalizedStatus,
		Quantity:             params.Quantity,
		Notional:             normalizedNotional,
		LimitPrice:           contextFields.limitPrice,
		EventTime:            normalizedEventTime,
	}, nil
}

// NewExecutionFill validates and canonicalizes a canonical execution fill.
func NewExecutionFill(params ExecutionFillParams) (ExecutionFill, error) {
	normalizedFillID, err := NewExecutionFillID(params.FillID)
	if err != nil {
		return ExecutionFill{}, err
	}

	normalizedOrder, err := NewExecutionOrder(ExecutionOrderParams{
		OrderID:              string(params.Order.OrderID),
		Command:              params.Order.Command,
		Mode:                 params.Order.Mode,
		StrategyID:           params.Order.StrategyID,
		StrategyVersion:      params.Order.StrategyVersion,
		StrategyArtifactHash: params.Order.StrategyArtifactHash,
		Venue:                params.Order.Venue,
		Instrument:           params.Order.Instrument,
		OrderType:            params.Order.OrderType,
		TimeInForce:          params.Order.TimeInForce,
		ReduceOnly:           params.Order.ReduceOnly,
		ClientOrderID:        params.Order.ClientOrderID,
		Status:               params.Order.Status,
		Quantity:             params.Order.Quantity,
		Notional:             params.Order.Notional,
		LimitPrice:           params.Order.LimitPrice,
		EventTime:            params.Order.EventTime.Time(),
	})
	if err != nil {
		return ExecutionFill{}, fmt.Errorf("execution fill order: %w", err)
	}
	if !isFiniteFloat64(params.Quantity) {
		return ExecutionFill{}, errors.New("execution fill quantity must be finite")
	}
	if params.Quantity <= 0 {
		return ExecutionFill{}, errors.New("execution fill quantity must be positive")
	}
	if !isFiniteFloat64(params.Price) {
		return ExecutionFill{}, errors.New("execution fill price must be finite")
	}
	if params.Price <= 0 {
		return ExecutionFill{}, errors.New("execution fill price must be positive")
	}
	if !isFiniteFloat64(params.FeeAmount) {
		return ExecutionFill{}, errors.New("execution fill fee amount must be finite")
	}
	if !isFiniteFloat64(params.SlippageAmount) {
		return ExecutionFill{}, errors.New("execution fill slippage amount must be finite")
	}

	normalizedMetadata, err := canonicalAuditMetadata(params.Metadata)
	if err != nil {
		return ExecutionFill{}, fmt.Errorf("execution fill metadata: %w", err)
	}
	if params.Metadata == nil {
		normalizedMetadata = nil
	}

	normalizedEventTime, err := NewExecutionEventTime(params.EventTime)
	if err != nil {
		return ExecutionFill{}, err
	}

	return ExecutionFill{
		FillID:                    normalizedFillID,
		Order:                     normalizedOrder,
		SourceMarketDataReference: strings.TrimSpace(params.SourceMarketDataReference),
		FeeAmount:                 params.FeeAmount,
		SlippageAmount:            params.SlippageAmount,
		Metadata:                  normalizedMetadata,
		Quantity:                  params.Quantity,
		Price:                     params.Price,
		EventTime:                 normalizedEventTime,
	}, nil
}

// NewExecutionReconciliation validates and canonicalizes a reconciliation record.
func NewExecutionReconciliation(params ExecutionReconciliationParams) (ExecutionReconciliation, error) {
	normalizedOrder, err := NewExecutionOrder(ExecutionOrderParams{
		OrderID:       string(params.Order.OrderID),
		Command:       params.Order.Command,
		Venue:         params.Order.Venue,
		ClientOrderID: params.Order.ClientOrderID,
		Status:        params.Order.Status,
		Quantity:      params.Order.Quantity,
		EventTime:     params.Order.EventTime.Time(),
	})
	if err != nil {
		return ExecutionReconciliation{}, fmt.Errorf("execution reconciliation order: %w", err)
	}

	normalizedFills := make([]ExecutionFill, len(params.Fills))
	for idx, fill := range params.Fills {
		normalizedFill, fillErr := NewExecutionFill(ExecutionFillParams{
			FillID:                    string(fill.FillID),
			Order:                     fill.Order,
			SourceMarketDataReference: fill.SourceMarketDataReference,
			FeeAmount:                 fill.FeeAmount,
			SlippageAmount:            fill.SlippageAmount,
			Metadata:                  fill.Metadata,
			Quantity:                  fill.Quantity,
			Price:                     fill.Price,
			EventTime:                 fill.EventTime.Time(),
		})
		if fillErr != nil {
			return ExecutionReconciliation{}, fmt.Errorf("execution reconciliation fill %d: %w", idx, fillErr)
		}

		normalizedFills[idx] = normalizedFill
	}

	normalizedStatus, err := NewExecutionOrderStatus(params.Status.String())
	if err != nil {
		return ExecutionReconciliation{}, errors.New("execution reconciliation status is required")
	}
	if !isFiniteFloat64(params.FilledQuantity) {
		return ExecutionReconciliation{}, errors.New("execution reconciliation filled quantity must be finite")
	}
	if params.FilledQuantity < 0 {
		return ExecutionReconciliation{}, errors.New("execution reconciliation filled quantity must not be negative")
	}

	normalizedEventTime, err := NewExecutionEventTime(params.EventTime)
	if err != nil {
		return ExecutionReconciliation{}, err
	}

	return ExecutionReconciliation{
		Order:          normalizedOrder,
		Fills:          normalizedFills,
		Status:         normalizedStatus,
		FilledQuantity: params.FilledQuantity,
		EventTime:      normalizedEventTime,
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

	return Candle(params), nil
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

	return Trade(params), nil
}

func isFiniteFloat64(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
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

func (s GovernorDecisionStatus) isValid() bool {
	switch s {
	case GovernorDecisionStatusApproved, GovernorDecisionStatusRejected, GovernorDecisionStatusBlocked:
		return true
	default:
		return false
	}
}

func (r GovernorDecisionReason) isValid() bool {
	switch r {
	case GovernorDecisionReasonOK,
		GovernorDecisionReasonModeNotAllowed,
		GovernorDecisionReasonVenueNotAllowed,
		GovernorDecisionReasonInstrumentNotAllowed,
		GovernorDecisionReasonStrategyNotAllowed,
		GovernorDecisionReasonActionKindNotAllowed,
		GovernorDecisionReasonDataQualityTooLow,
		GovernorDecisionReasonKillSwitchActive,
		GovernorDecisionReasonOrderNotionalExceedsLimit,
		GovernorDecisionReasonStrategyExposureExceedsLimit,
		GovernorDecisionReasonInstrumentExposureExceedsLimit,
		GovernorDecisionReasonApprovalLimitReached:
		return true
	case GovernorDecisionReasonInvalidIntent:
		return true
	default:
		return false
	}
}

func (s ExecutionCommandStatus) isValid() bool {
	switch s {
	case ExecutionCommandStatusCreated:
		return true
	default:
		return false
	}
}

func (s ExecutionOrderStatus) isValid() bool {
	switch s {
	case ExecutionOrderStatusOpen,
		ExecutionOrderStatusPartiallyFilled,
		ExecutionOrderStatusFilled,
		ExecutionOrderStatusOverfilled:
		return true
	default:
		return false
	}
}

func (t TimeInForce) isValid() bool {
	switch t {
	case TimeInForceGTC:
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

func (s GovernorDecisionStatus) String() string {
	return string(s)
}

func (r GovernorDecisionReason) String() string {
	return string(r)
}

func (s ExecutionCommandStatus) String() string {
	return string(s)
}

func (s ExecutionOrderStatus) String() string {
	return string(s)
}

func (t TimeInForce) String() string {
	return string(t)
}

// Time returns the time value for a canonical analytics point timestamp.
func (t AnalyticsPointTime) Time() time.Time {
	return time.Time(t)
}

// Time returns the time value for a canonical candidate action timestamp.
func (t CandidateActionTime) Time() time.Time {
	return time.Time(t)
}

// Time returns the time value for a canonical governor decision timestamp.
func (t GovernorDecisionTime) Time() time.Time {
	return time.Time(t)
}

// Time returns the time value for a canonical execution event timestamp.
func (t ExecutionEventTime) Time() time.Time {
	return time.Time(t)
}

type executionCommandContext struct {
	traceID                   DecisionTraceID
	intentID                  OrderIntentID
	mode                      DecisionMode
	strategyID                string
	strategyVersion           string
	strategyArtifactHash      string
	venue                     Venue
	instrument                Instrument
	actionKind                CandidateActionKind
	orderType                 OrderType
	limitPrice                *float64
	governorDecisionReference string
}

type executionOrderContext struct {
	mode                 DecisionMode
	strategyID           string
	strategyVersion      string
	strategyArtifactHash string
	venue                Venue
	instrument           Instrument
	orderType            OrderType
	timeInForce          TimeInForce
	notional             float64
	limitPrice           *float64
	reduceOnly           bool
}

func canonicalizeExecutionCommandContext(
	params ExecutionCommandParams,
) (executionCommandContext, error) {
	traceID, err := optionalDecisionTraceID(params.TraceID)
	if err != nil {
		return executionCommandContext{}, fmt.Errorf("execution command trace id: %w", err)
	}

	intentID, err := optionalOrderIntentID(params.IntentID)
	if err != nil {
		return executionCommandContext{}, fmt.Errorf("execution command intent id: %w", err)
	}

	mode, err := optionalDecisionMode(params.Mode)
	if err != nil {
		return executionCommandContext{}, errors.New("execution command mode is invalid")
	}

	strategyID, strategyVersion, strategyArtifactHash, err := optionalStrategyFields(
		params.StrategyID,
		params.StrategyVersion,
		params.StrategyArtifactHash,
		"execution command",
	)
	if err != nil {
		return executionCommandContext{}, err
	}

	venue, err := optionalVenue(params.Venue, "execution command venue is invalid")
	if err != nil {
		return executionCommandContext{}, err
	}

	instrument, err := optionalInstrument(params.Instrument, "execution command instrument")
	if err != nil {
		return executionCommandContext{}, err
	}

	actionKind, err := optionalActionKind(params.ActionKind, "execution command action kind is invalid")
	if err != nil {
		return executionCommandContext{}, err
	}

	orderType, err := optionalOrderType(params.OrderType, "execution command order type is invalid")
	if err != nil {
		return executionCommandContext{}, err
	}

	limitPrice, hasLimitPrice, err := canonicalExecutionLimitPrice(orderType, params.LimitPrice)
	if err != nil {
		return executionCommandContext{}, err
	}

	return executionCommandContext{
		traceID:                   traceID,
		intentID:                  intentID,
		mode:                      mode,
		strategyID:                strategyID,
		strategyVersion:           strategyVersion,
		strategyArtifactHash:      strategyArtifactHash,
		venue:                     venue,
		instrument:                instrument,
		actionKind:                actionKind,
		orderType:                 orderType,
		limitPrice:                limitPriceOrNil(limitPrice, hasLimitPrice),
		governorDecisionReference: strings.TrimSpace(params.GovernorDecisionReference),
	}, nil
}

func canonicalizeExecutionOrderContext(
	params ExecutionOrderParams,
	command ExecutionCommand,
) (executionOrderContext, error) {
	modeCandidate := params.Mode
	if modeCandidate == "" {
		modeCandidate = command.Mode
	}
	mode, err := optionalDecisionMode(modeCandidate)
	if err != nil {
		return executionOrderContext{}, errors.New("execution order mode is invalid")
	}

	strategyID, strategyVersion, strategyArtifactHash, err := optionalStrategyFields(
		firstNonEmpty(params.StrategyID, command.StrategyID),
		firstNonEmpty(params.StrategyVersion, command.StrategyVersion),
		firstNonEmpty(params.StrategyArtifactHash, command.StrategyArtifactHash),
		"execution order",
	)
	if err != nil {
		return executionOrderContext{}, err
	}

	venue, err := requiredVenue(firstVenue(params.Venue, command.Venue))
	if err != nil {
		return executionOrderContext{}, err
	}

	instrument, err := optionalInstrument(
		firstInstrument(params.Instrument, command.Instrument),
		"execution order instrument",
	)
	if err != nil {
		return executionOrderContext{}, err
	}

	orderType, err := optionalOrderType(
		firstOrderType(params.OrderType, command.OrderType),
		"execution order type is invalid",
	)
	if err != nil {
		return executionOrderContext{}, err
	}

	timeInForce, err := optionalTimeInForce(params.TimeInForce)
	if err != nil {
		return executionOrderContext{}, err
	}

	notional := params.Notional
	if notional == 0 {
		notional = command.Notional
	}

	limitPrice, hasLimitPrice, err := canonicalExecutionLimitPrice(
		orderType,
		firstFloatPointer(params.LimitPrice, command.LimitPrice),
	)
	if err != nil {
		return executionOrderContext{}, err
	}

	return executionOrderContext{
		mode:                 mode,
		strategyID:           strategyID,
		strategyVersion:      strategyVersion,
		strategyArtifactHash: strategyArtifactHash,
		venue:                venue,
		instrument:           instrument,
		orderType:            orderType,
		timeInForce:          timeInForce,
		notional:             notional,
		limitPrice:           limitPriceOrNil(limitPrice, hasLimitPrice),
		reduceOnly:           params.ReduceOnly || command.ReduceOnly,
	}, nil
}

func canonicalExecutionLimitPrice(orderType OrderType, value *float64) (float64, bool, error) {
	if value == nil {
		if orderType == OrderTypeLimit {
			return 0, false, errors.New("execution limit price is required for limit orders")
		}

		return 0, false, nil
	}
	if !isFiniteFloat64(*value) || *value <= 0 {
		return 0, false, errors.New("execution limit price must be finite and positive")
	}
	if orderType != "" && orderType != OrderTypeLimit {
		return 0, false, errors.New("execution order type is unsupported")
	}

	return *value, true, nil
}

func limitPriceOrNil(value float64, ok bool) *float64 {
	if !ok {
		return nil
	}

	normalized := value
	return &normalized
}

func optionalDecisionTraceID(value string) (DecisionTraceID, error) {
	if strings.TrimSpace(value) == "" {
		return DecisionTraceID(""), nil
	}

	return NewDecisionTraceID(value)
}

func optionalOrderIntentID(value string) (OrderIntentID, error) {
	if strings.TrimSpace(value) == "" {
		return OrderIntentID(""), nil
	}

	return NewOrderIntentID(value)
}

func optionalDecisionMode(value DecisionMode) (DecisionMode, error) {
	if value == "" {
		return DecisionMode(""), nil
	}

	return NewDecisionMode(value.String())
}

func optionalStrategyFields(id string, version string, hash string, prefix string) (string, string, string, error) {
	normalizedID := strings.TrimSpace(id)
	normalizedVersion := strings.TrimSpace(version)
	normalizedHash := strings.TrimSpace(hash)
	if normalizedID == "" && normalizedVersion == "" && normalizedHash == "" {
		return "", "", "", nil
	}
	if normalizedID == "" {
		return "", "", "", errors.New(prefix + " strategy id is required")
	}
	if normalizedVersion == "" {
		return "", "", "", errors.New(prefix + " strategy version is required")
	}
	if normalizedHash == "" {
		return "", "", "", errors.New(prefix + " strategy artifact hash is required")
	}

	return normalizedID, normalizedVersion, normalizedHash, nil
}

func optionalVenue(value Venue, message string) (Venue, error) {
	if value == "" {
		return Venue(""), nil
	}

	venue, err := NewVenue(value.String())
	if err != nil {
		return Venue(""), errors.New(message)
	}

	return venue, nil
}

func requiredVenue(value Venue) (Venue, error) {
	venue, err := NewVenue(value.String())
	if err != nil {
		return Venue(""), errors.New("execution order venue is required")
	}

	return venue, nil
}

func optionalInstrument(value Instrument, field string) (Instrument, error) {
	if value == (Instrument{}) {
		return Instrument{}, nil
	}

	instrument, err := NewInstrument(InstrumentParams(value))
	if err != nil {
		return Instrument{}, fmt.Errorf("%s: %w", field, err)
	}

	return instrument, nil
}

func optionalActionKind(value CandidateActionKind, message string) (CandidateActionKind, error) {
	if value == "" {
		return CandidateActionKind(""), nil
	}

	actionKind, err := NewCandidateActionKind(value.String())
	if err != nil {
		return CandidateActionKind(""), errors.New(message)
	}

	return actionKind, nil
}

func optionalOrderType(value OrderType, message string) (OrderType, error) {
	if value == "" {
		return OrderType(""), nil
	}

	orderType, err := NewOrderType(value.String())
	if err != nil {
		return OrderType(""), errors.New(message)
	}

	return orderType, nil
}

func optionalTimeInForce(value TimeInForce) (TimeInForce, error) {
	if value == "" {
		return TimeInForce(""), nil
	}

	timeInForce, err := NewTimeInForce(value.String())
	if err != nil {
		return TimeInForce(""), errors.New("execution order time in force is invalid")
	}

	return timeInForce, nil
}

func firstNonEmpty(primary string, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}

	return fallback
}

func firstVenue(primary Venue, fallback Venue) Venue {
	if primary != "" {
		return primary
	}

	return fallback
}

func firstInstrument(primary Instrument, fallback Instrument) Instrument {
	if primary != (Instrument{}) {
		return primary
	}

	return fallback
}

func firstOrderType(primary OrderType, fallback OrderType) OrderType {
	if primary != "" {
		return primary
	}

	return fallback
}

func firstFloatPointer(primary *float64, fallback *float64) *float64 {
	if primary != nil {
		return primary
	}

	return fallback
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
