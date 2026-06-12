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
