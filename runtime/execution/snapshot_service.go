package execution

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	snapshotQuantityTolerance      = 1e-9
	projectionFillLedgerV0         = "fill-ledger-v0"
	deferredModelValue             = "deferred"
	notModeledValue                = "not-modeled"
	markPriceV0Model               = "mark-price-v0"
	closedCandleLimitSimulatorName = "closed-candle-limit-v0"
	fundingModelKey                = "funding_model"
	leverageModelKey               = "leverage_model"
	liquidationModelKey            = "liquidation_model"
	marginModelKey                 = "margin_model"
	projectionKey                  = "projection"
	collateralModelKey             = "collateral_model"
	unrealizedPnLModelKey          = "unrealized_pnl_model"
	simulatorMetadataKey           = "simulator"
)

// PositionSnapshotQuery configures deterministic projected position filtering.
type PositionSnapshotQuery struct {
	StrategyID string
	Instrument *domain.Instrument
	Mode       *domain.DecisionMode
	TimeRange  *domain.TimeRange
}

// PortfolioSnapshotQuery configures deterministic projected portfolio filtering.
type PortfolioSnapshotQuery struct {
	Mode      *domain.DecisionMode
	TimeRange *domain.TimeRange
}

// PositionMarkPrice configures a deterministic mark/reference price.
type PositionMarkPrice struct {
	Instrument domain.Instrument
	Price      float64
}

// ProjectPortfolioSnapshotsRequest configures projected portfolio aggregation.
type ProjectPortfolioSnapshotsRequest struct {
	PositionSnapshots []domain.PositionSnapshot
	MarkPrices        []PositionMarkPrice
}

type snapshotStore interface {
	CreatePositionSnapshot(ctx context.Context, snapshot domain.PositionSnapshot) (domain.PositionSnapshot, error)
	QueryPositionSnapshots(ctx context.Context, query PositionSnapshotQuery) ([]domain.PositionSnapshot, error)
	CreatePortfolioSnapshot(ctx context.Context, snapshot domain.PortfolioSnapshot) (domain.PortfolioSnapshot, error)
	QueryPortfolioSnapshots(ctx context.Context, query PortfolioSnapshotQuery) ([]domain.PortfolioSnapshot, error)
}

// SnapshotService projects and persists deterministic paper/backtest snapshots.
type SnapshotService struct {
	store snapshotStore
}

// NewSnapshotService creates a snapshot service with required persistence.
func NewSnapshotService(store snapshotStore) (*SnapshotService, error) {
	if store == nil {
		return nil, errors.New("snapshot store is required")
	}

	return &SnapshotService{store: store}, nil
}

// RecordPositionSnapshots projects snapshots from fills and persists them.
func (s *SnapshotService) RecordPositionSnapshots(
	ctx context.Context,
	fills []domain.ExecutionFill,
) ([]domain.PositionSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	projected, err := projectPositionSnapshots(fills)
	if err != nil {
		return nil, err
	}

	persisted := make([]domain.PositionSnapshot, 0, len(projected))
	for _, snapshot := range projected {
		row, createErr := s.store.CreatePositionSnapshot(ctx, snapshot)
		if createErr != nil {
			return nil, createErr
		}
		persisted = append(persisted, row)
	}

	return persisted, nil
}

// RecordPortfolioSnapshots projects portfolio snapshots and persists them.
func (s *SnapshotService) RecordPortfolioSnapshots(
	ctx context.Context,
	request ProjectPortfolioSnapshotsRequest,
) ([]domain.PortfolioSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	projected, err := projectPortfolioSnapshots(request)
	if err != nil {
		return nil, err
	}

	persisted := make([]domain.PortfolioSnapshot, 0, len(projected))
	for _, snapshot := range projected {
		row, createErr := s.store.CreatePortfolioSnapshot(ctx, snapshot)
		if createErr != nil {
			return nil, createErr
		}
		persisted = append(persisted, row)
	}

	return persisted, nil
}

// QueryPositionSnapshots returns deterministic projected position snapshots.
func (s *SnapshotService) QueryPositionSnapshots(
	ctx context.Context,
	query PositionSnapshotQuery,
) ([]domain.PositionSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.store.QueryPositionSnapshots(ctx, query)
}

// QueryPortfolioSnapshots returns deterministic projected portfolio snapshots.
func (s *SnapshotService) QueryPortfolioSnapshots(
	ctx context.Context,
	query PortfolioSnapshotQuery,
) ([]domain.PortfolioSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.store.QueryPortfolioSnapshots(ctx, query)
}

func projectPositionSnapshots(fills []domain.ExecutionFill) ([]domain.PositionSnapshot, error) {
	canonicalFills := make([]domain.ExecutionFill, 0, len(fills))
	for idx, fill := range fills {
		canonical, err := canonicalFill(fill)
		if err != nil {
			return nil, validationError(fmt.Sprintf("position projection fill %d: %s", idx, err.Error()))
		}
		canonicalFills = append(canonicalFills, canonical)
	}

	slices.SortStableFunc(canonicalFills, func(left, right domain.ExecutionFill) int {
		if comparison := left.EventTime.Time().Compare(right.EventTime.Time()); comparison != 0 {
			return comparison
		}

		return strings.Compare(string(left.FillID), string(right.FillID))
	})

	states := map[string]projectedPositionState{}
	snapshots := make([]domain.PositionSnapshot, 0, len(canonicalFills))
	for _, fill := range canonicalFills {
		key := positionProjectionKey(fill)
		nextState, err := applyFillToPositionState(states[key], fill)
		if err != nil {
			return nil, err
		}
		states[key] = nextState

		snapshot, err := domain.NewPositionSnapshot(domain.PositionSnapshotParams{
			SnapshotID:           stableID("position-snapshot", string(fill.FillID)),
			SourceFillID:         string(fill.FillID),
			Mode:                 fill.Order.Mode,
			StrategyID:           fill.Order.StrategyID,
			StrategyVersion:      fill.Order.StrategyVersion,
			StrategyArtifactHash: fill.Order.StrategyArtifactHash,
			Instrument:           fill.Order.Instrument,
			Quantity:             nextState.Quantity,
			AverageEntryPrice:    cloneFloat64Pointer(nextState.AverageEntryPrice),
			RealizedPnL:          nextState.RealizedPnL,
			ExposureNotional:     nextState.ExposureNotional,
			EventTime:            fill.EventTime.Time(),
			Metadata:             positionSnapshotMetadata(),
		})
		if err != nil {
			return nil, validationError(err.Error())
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func projectPortfolioSnapshots(
	request ProjectPortfolioSnapshotsRequest,
) ([]domain.PortfolioSnapshot, error) {
	canonicalSnapshots, err := canonicalPositionSnapshots(request.PositionSnapshots)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(canonicalSnapshots, func(left, right domain.PositionSnapshot) int {
		if comparison := left.EventTime.Time().Compare(right.EventTime.Time()); comparison != 0 {
			return comparison
		}

		return strings.Compare(left.SnapshotID.String(), right.SnapshotID.String())
	})

	markPrices, err := canonicalMarkPrices(request.MarkPrices)
	if err != nil {
		return nil, err
	}

	statesByMode := map[string]map[string]domain.PositionSnapshot{}
	portfolioSnapshots := make([]domain.PortfolioSnapshot, 0, len(canonicalSnapshots))
	for _, snapshot := range canonicalSnapshots {
		modeKey := snapshot.Mode.String()
		if statesByMode[modeKey] == nil {
			statesByMode[modeKey] = map[string]domain.PositionSnapshot{}
		}
		statesByMode[modeKey][positionSnapshotStateKey(snapshot)] = snapshot

		grossExposure, netExposure, realizedPnL, unrealizedPnLPtr, hasUnrealizedPnL, aggregateErr := aggregatePortfolioState(
			statesByMode[modeKey],
			markPrices,
		)
		if aggregateErr != nil {
			return nil, aggregateErr
		}

		portfolioSnapshot, createErr := domain.NewPortfolioSnapshot(domain.PortfolioSnapshotParams{
			SnapshotID:    stableID("portfolio-snapshot", string(snapshot.SourceFillID)),
			SourceFillID:  string(snapshot.SourceFillID),
			Mode:          snapshot.Mode,
			GrossExposure: grossExposure,
			NetExposure:   netExposure,
			RealizedPnL:   realizedPnL,
			UnrealizedPnL: unrealizedPnLPtr,
			EventTime:     snapshot.EventTime.Time(),
			Metadata:      portfolioSnapshotMetadata(hasUnrealizedPnL),
		})
		if createErr != nil {
			return nil, validationError(createErr.Error())
		}

		portfolioSnapshots = append(portfolioSnapshots, portfolioSnapshot)
	}

	return portfolioSnapshots, nil
}

func canonicalPositionSnapshots(
	positionSnapshots []domain.PositionSnapshot,
) ([]domain.PositionSnapshot, error) {
	canonicalSnapshots := make([]domain.PositionSnapshot, 0, len(positionSnapshots))
	for idx, snapshot := range positionSnapshots {
		canonical, err := domain.NewPositionSnapshot(domain.PositionSnapshotParams{
			SnapshotID:           snapshot.SnapshotID.String(),
			SourceFillID:         string(snapshot.SourceFillID),
			Mode:                 snapshot.Mode,
			StrategyID:           snapshot.StrategyID,
			StrategyVersion:      snapshot.StrategyVersion,
			StrategyArtifactHash: snapshot.StrategyArtifactHash,
			Instrument:           snapshot.Instrument,
			Quantity:             snapshot.Quantity,
			AverageEntryPrice:    snapshot.AverageEntryPrice,
			RealizedPnL:          snapshot.RealizedPnL,
			ExposureNotional:     snapshot.ExposureNotional,
			EventTime:            snapshot.EventTime.Time(),
			Metadata:             snapshot.Metadata,
		})
		if err != nil {
			return nil, validationError(
				fmt.Sprintf("portfolio projection position snapshot %d: %s", idx, err.Error()),
			)
		}
		canonicalSnapshots = append(canonicalSnapshots, canonical)
	}

	return canonicalSnapshots, nil
}

func aggregatePortfolioState(
	positions map[string]domain.PositionSnapshot,
	markPrices map[string]float64,
) (float64, float64, float64, *float64, bool, error) {
	grossExposure := 0.0
	netExposure := 0.0
	realizedPnL := 0.0
	openPositions := 0
	unrealizedPnL := 0.0
	hasAllMarks := true
	positionKeys := make([]string, 0, len(positions))

	for key := range positions {
		positionKeys = append(positionKeys, key)
	}
	slices.Sort(positionKeys)

	for _, key := range positionKeys {
		position := positions[key]
		grossExposure += position.ExposureNotional
		netExposure += math.Copysign(position.ExposureNotional, position.Quantity)
		realizedPnL += position.RealizedPnL
		if math.Abs(position.Quantity) <= snapshotQuantityTolerance {
			continue
		}
		openPositions++

		markPrice, ok := markPrices[instrumentKey(position.Instrument)]
		if !ok {
			hasAllMarks = false
			continue
		}
		if position.AverageEntryPrice == nil {
			return 0, 0, 0, nil, false, validationError(
				"portfolio projection requires average entry price for open positions",
			)
		}
		unrealizedPnL += (markPrice - *position.AverageEntryPrice) * position.Quantity
	}

	if openPositions > 0 && hasAllMarks {
		return grossExposure, netExposure, realizedPnL, &unrealizedPnL, true, nil
	}

	return grossExposure, netExposure, realizedPnL, nil, false, nil
}

type projectedPositionState struct {
	Quantity          float64
	AverageEntryPrice *float64
	RealizedPnL       float64
	ExposureNotional  float64
}

func applyFillToPositionState(
	current projectedPositionState,
	fill domain.ExecutionFill,
) (projectedPositionState, error) {
	deltaQuantity, err := signedFillQuantity(fill)
	if err != nil {
		return projectedPositionState{}, err
	}

	if math.Abs(current.Quantity) <= snapshotQuantityTolerance {
		averageEntryPrice := fill.Price
		return projectedPositionState{
			Quantity:          deltaQuantity,
			AverageEntryPrice: &averageEntryPrice,
			RealizedPnL:       current.RealizedPnL,
			ExposureNotional:  math.Abs(deltaQuantity) * averageEntryPrice,
		}, nil
	}
	if current.AverageEntryPrice == nil {
		return projectedPositionState{}, validationError(
			"position projection average entry price is required for open positions",
		)
	}

	if sameSign(current.Quantity, deltaQuantity) {
		combinedQuantity := current.Quantity + deltaQuantity
		averageEntryPrice :=
			((math.Abs(current.Quantity) * *current.AverageEntryPrice) +
				(math.Abs(deltaQuantity) * fill.Price)) / math.Abs(combinedQuantity)
		return projectedPositionState{
			Quantity:          combinedQuantity,
			AverageEntryPrice: &averageEntryPrice,
			RealizedPnL:       current.RealizedPnL,
			ExposureNotional:  math.Abs(combinedQuantity) * averageEntryPrice,
		}, nil
	}

	if math.Abs(deltaQuantity)-math.Abs(current.Quantity) > snapshotQuantityTolerance {
		return projectedPositionState{}, validationError("position projection reversal is unsupported")
	}

	realizedPnL := current.RealizedPnL +
		(math.Abs(deltaQuantity) *
			(fill.Price - *current.AverageEntryPrice) *
			math.Copysign(1, current.Quantity))
	remainingQuantity := current.Quantity + deltaQuantity
	if math.Abs(remainingQuantity) <= snapshotQuantityTolerance {
		return projectedPositionState{
			Quantity:         0,
			RealizedPnL:      realizedPnL,
			ExposureNotional: 0,
		}, nil
	}

	averageEntryPrice := *current.AverageEntryPrice
	return projectedPositionState{
		Quantity:          remainingQuantity,
		AverageEntryPrice: &averageEntryPrice,
		RealizedPnL:       realizedPnL,
		ExposureNotional:  math.Abs(remainingQuantity) * averageEntryPrice,
	}, nil
}

func signedFillQuantity(fill domain.ExecutionFill) (float64, error) {
	switch fill.Order.Command.ActionKind {
	case domain.CandidateActionKindLong:
		return fill.Quantity, nil
	case domain.CandidateActionKindShort:
		return -fill.Quantity, nil
	default:
		return 0, validationError("position projection action kind is unsupported")
	}
}

func sameSign(left, right float64) bool {
	return (left > 0 && right > 0) || (left < 0 && right < 0)
}

func positionProjectionKey(fill domain.ExecutionFill) string {
	return strings.Join([]string{
		fill.Order.Mode.String(),
		fill.Order.StrategyID,
		fill.Order.StrategyVersion,
		fill.Order.StrategyArtifactHash,
		instrumentKey(fill.Order.Instrument),
	}, "|")
}

func positionSnapshotStateKey(snapshot domain.PositionSnapshot) string {
	return strings.Join([]string{
		snapshot.Mode.String(),
		snapshot.StrategyID,
		snapshot.StrategyVersion,
		snapshot.StrategyArtifactHash,
		instrumentKey(snapshot.Instrument),
	}, "|")
}

func instrumentKey(instrument domain.Instrument) string {
	return strings.Join([]string{
		instrument.Venue.String(),
		instrument.Symbol.String(),
		instrument.AssetClass.String(),
	}, "|")
}

func canonicalMarkPrices(markPrices []PositionMarkPrice) (map[string]float64, error) {
	canonical := make(map[string]float64, len(markPrices))
	for idx, markPrice := range markPrices {
		instrument, err := domain.NewInstrument(domain.InstrumentParams(markPrice.Instrument))
		if err != nil {
			return nil, validationError(fmt.Sprintf("portfolio mark price %d instrument: %s", idx, err.Error()))
		}
		if !isFinite(markPrice.Price) {
			return nil, validationError(fmt.Sprintf("portfolio mark price %d must be finite", idx))
		}
		if markPrice.Price <= 0 {
			return nil, validationError(fmt.Sprintf("portfolio mark price %d must be positive", idx))
		}

		canonical[instrumentKey(instrument)] = markPrice.Price
	}

	return canonical, nil
}

func cloneFloat64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func positionSnapshotMetadata() map[string]string {
	return map[string]string{
		fundingModelKey:     deferredModelValue,
		leverageModelKey:    notModeledValue,
		liquidationModelKey: deferredModelValue,
		marginModelKey:      deferredModelValue,
		projectionKey:       projectionFillLedgerV0,
	}
}

func portfolioSnapshotMetadata(hasUnrealizedPnL bool) map[string]string {
	metadata := positionSnapshotMetadata()
	metadata[collateralModelKey] = notModeledValue
	if hasUnrealizedPnL {
		metadata[unrealizedPnLModelKey] = markPriceV0Model
	} else {
		metadata[unrealizedPnLModelKey] = deferredModelValue
	}

	return metadata
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
