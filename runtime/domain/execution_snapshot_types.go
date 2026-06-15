package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// PositionSnapshotID identifies a canonical paper/backtest position snapshot.
type PositionSnapshotID string

// PortfolioSnapshotID identifies a canonical paper/backtest portfolio snapshot.
type PortfolioSnapshotID string

// PositionSnapshot is a canonical projected position state after a fill.
type PositionSnapshot struct {
	SnapshotID           PositionSnapshotID
	SourceFillID         ExecutionFillID
	Mode                 DecisionMode
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	Instrument           Instrument
	Quantity             float64
	AverageEntryPrice    *float64
	RealizedPnL          float64
	ExposureNotional     float64
	EventTime            ExecutionEventTime
	Metadata             map[string]string
}

// PortfolioSnapshot is a canonical projected portfolio state after a fill.
type PortfolioSnapshot struct {
	SnapshotID    PortfolioSnapshotID
	SourceFillID  ExecutionFillID
	Mode          DecisionMode
	GrossExposure float64
	NetExposure   float64
	RealizedPnL   float64
	UnrealizedPnL *float64
	EventTime     ExecutionEventTime
	Metadata      map[string]string
}

// PositionSnapshotParams holds inputs for a canonical position snapshot.
type PositionSnapshotParams struct {
	SnapshotID           string
	SourceFillID         string
	Mode                 DecisionMode
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	Instrument           Instrument
	Quantity             float64
	AverageEntryPrice    *float64
	RealizedPnL          float64
	ExposureNotional     float64
	EventTime            time.Time
	Metadata             map[string]string
}

// PortfolioSnapshotParams holds inputs for a canonical portfolio snapshot.
type PortfolioSnapshotParams struct {
	SnapshotID    string
	SourceFillID  string
	Mode          DecisionMode
	GrossExposure float64
	NetExposure   float64
	RealizedPnL   float64
	UnrealizedPnL *float64
	EventTime     time.Time
	Metadata      map[string]string
}

// NewPositionSnapshotID validates and canonicalizes a position snapshot identifier.
func NewPositionSnapshotID(value string) (PositionSnapshotID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("position snapshot id is required")
	}

	return PositionSnapshotID(normalized), nil
}

// NewPortfolioSnapshotID validates and canonicalizes a portfolio snapshot identifier.
func NewPortfolioSnapshotID(value string) (PortfolioSnapshotID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("portfolio snapshot id is required")
	}

	return PortfolioSnapshotID(normalized), nil
}

// NewPositionSnapshot validates and canonicalizes a projected position snapshot.
func NewPositionSnapshot(params PositionSnapshotParams) (PositionSnapshot, error) {
	normalizedSnapshotID, err := NewPositionSnapshotID(params.SnapshotID)
	if err != nil {
		return PositionSnapshot{}, err
	}

	normalizedSourceFillID, err := NewExecutionFillID(params.SourceFillID)
	if err != nil {
		return PositionSnapshot{}, fmt.Errorf("position snapshot source fill id: %w", err)
	}

	normalizedMode, err := NewDecisionMode(params.Mode.String())
	if err != nil {
		return PositionSnapshot{}, errors.New("position snapshot mode is required")
	}

	strategyID, strategyVersion, strategyArtifactHash, err := optionalStrategyFields(
		params.StrategyID,
		params.StrategyVersion,
		params.StrategyArtifactHash,
		"position snapshot",
	)
	if err != nil {
		return PositionSnapshot{}, err
	}

	instrument, err := NewInstrument(InstrumentParams(params.Instrument))
	if err != nil {
		return PositionSnapshot{}, fmt.Errorf("position snapshot instrument: %w", err)
	}

	if !isFiniteFloat64(params.Quantity) {
		return PositionSnapshot{}, errors.New("position snapshot quantity must be finite")
	}
	if !isFiniteFloat64(params.RealizedPnL) {
		return PositionSnapshot{}, errors.New("position snapshot realized pnl must be finite")
	}
	if !isFiniteFloat64(params.ExposureNotional) {
		return PositionSnapshot{}, errors.New("position snapshot exposure notional must be finite")
	}
	if params.ExposureNotional < 0 {
		return PositionSnapshot{}, errors.New("position snapshot exposure notional must be zero or greater")
	}

	averageEntryPrice, hasAverageEntryPrice, err := canonicalOptionalPositiveFiniteFloat64(
		params.AverageEntryPrice,
		"position snapshot average entry price",
	)
	if err != nil {
		return PositionSnapshot{}, err
	}
	if params.Quantity == 0 {
		if hasAverageEntryPrice {
			return PositionSnapshot{}, errors.New(
				"position snapshot average entry price must be empty when quantity is zero",
			)
		}
		if params.ExposureNotional != 0 {
			return PositionSnapshot{}, errors.New(
				"position snapshot exposure notional must be zero when quantity is zero",
			)
		}
	} else if !hasAverageEntryPrice {
		return PositionSnapshot{}, errors.New(
			"position snapshot average entry price is required when quantity is non-zero",
		)
	}

	normalizedEventTime, err := NewExecutionEventTime(params.EventTime)
	if err != nil {
		return PositionSnapshot{}, err
	}

	normalizedMetadata, err := canonicalAuditMetadata(params.Metadata)
	if err != nil {
		return PositionSnapshot{}, fmt.Errorf("position snapshot metadata: %w", err)
	}
	if params.Metadata == nil {
		normalizedMetadata = nil
	}

	return PositionSnapshot{
		SnapshotID:           normalizedSnapshotID,
		SourceFillID:         normalizedSourceFillID,
		Mode:                 normalizedMode,
		StrategyID:           strategyID,
		StrategyVersion:      strategyVersion,
		StrategyArtifactHash: strategyArtifactHash,
		Instrument:           instrument,
		Quantity:             params.Quantity,
		AverageEntryPrice:    averageEntryPrice,
		RealizedPnL:          params.RealizedPnL,
		ExposureNotional:     params.ExposureNotional,
		EventTime:            normalizedEventTime,
		Metadata:             normalizedMetadata,
	}, nil
}

// NewPortfolioSnapshot validates and canonicalizes a projected portfolio snapshot.
func NewPortfolioSnapshot(params PortfolioSnapshotParams) (PortfolioSnapshot, error) {
	normalizedSnapshotID, err := NewPortfolioSnapshotID(params.SnapshotID)
	if err != nil {
		return PortfolioSnapshot{}, err
	}

	normalizedSourceFillID, err := NewExecutionFillID(params.SourceFillID)
	if err != nil {
		return PortfolioSnapshot{}, fmt.Errorf("portfolio snapshot source fill id: %w", err)
	}

	normalizedMode, err := NewDecisionMode(params.Mode.String())
	if err != nil {
		return PortfolioSnapshot{}, errors.New("portfolio snapshot mode is required")
	}

	if !isFiniteFloat64(params.GrossExposure) {
		return PortfolioSnapshot{}, errors.New("portfolio snapshot gross exposure must be finite")
	}
	if params.GrossExposure < 0 {
		return PortfolioSnapshot{}, errors.New("portfolio snapshot gross exposure must be zero or greater")
	}
	if !isFiniteFloat64(params.NetExposure) {
		return PortfolioSnapshot{}, errors.New("portfolio snapshot net exposure must be finite")
	}
	if !isFiniteFloat64(params.RealizedPnL) {
		return PortfolioSnapshot{}, errors.New("portfolio snapshot realized pnl must be finite")
	}

	unrealizedPnL, _, err := canonicalOptionalFiniteFloat64(
		params.UnrealizedPnL,
		"portfolio snapshot unrealized pnl",
	)
	if err != nil {
		return PortfolioSnapshot{}, err
	}

	normalizedEventTime, err := NewExecutionEventTime(params.EventTime)
	if err != nil {
		return PortfolioSnapshot{}, err
	}

	normalizedMetadata, err := canonicalAuditMetadata(params.Metadata)
	if err != nil {
		return PortfolioSnapshot{}, fmt.Errorf("portfolio snapshot metadata: %w", err)
	}
	if params.Metadata == nil {
		normalizedMetadata = nil
	}

	return PortfolioSnapshot{
		SnapshotID:    normalizedSnapshotID,
		SourceFillID:  normalizedSourceFillID,
		Mode:          normalizedMode,
		GrossExposure: params.GrossExposure,
		NetExposure:   params.NetExposure,
		RealizedPnL:   params.RealizedPnL,
		UnrealizedPnL: unrealizedPnL,
		EventTime:     normalizedEventTime,
		Metadata:      normalizedMetadata,
	}, nil
}

// String returns the string value for a canonical position snapshot id.
func (s PositionSnapshotID) String() string {
	return string(s)
}

// String returns the string value for a canonical portfolio snapshot id.
func (s PortfolioSnapshotID) String() string {
	return string(s)
}

func canonicalOptionalFiniteFloat64(value *float64, field string) (*float64, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	if !isFiniteFloat64(*value) {
		return nil, false, errors.New(field + " must be finite")
	}

	canonical := *value
	return &canonical, true, nil
}

func canonicalOptionalPositiveFiniteFloat64(
	value *float64,
	field string,
) (*float64, bool, error) {
	canonical, ok, err := canonicalOptionalFiniteFloat64(value, field)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	if *canonical <= 0 {
		return nil, false, errors.New(field + " must be positive")
	}

	return canonical, true, nil
}
