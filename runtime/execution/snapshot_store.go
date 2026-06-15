package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const (
	positionSnapshotIDColumn  = "snapshot_id"
	portfolioSnapshotIDColumn = "snapshot_id"
)

type positionSnapshotModel struct {
	SnapshotID           string    `gorm:"column:snapshot_id;size:255;not null;primaryKey;uniqueIndex:idx_position_snapshots_snapshot_id"`
	FillID               string    `gorm:"column:fill_id;size:255;not null;index:idx_position_snapshots_fill_id"`
	Mode                 string    `gorm:"column:mode;size:32;not null;index:idx_position_snapshots_mode_time_id,priority:1"`
	StrategyID           string    `gorm:"column:strategy_id;size:255;not null;index:idx_position_snapshots_strategy_time_id,priority:1"`
	StrategyVersion      string    `gorm:"column:strategy_version;size:255;not null"`
	StrategyArtifactHash string    `gorm:"column:strategy_artifact_hash;size:255;not null"`
	Venue                string    `gorm:"column:venue;size:255;not null;index:idx_position_snapshots_instrument_time_id,priority:1"`
	Symbol               string    `gorm:"column:symbol;size:255;not null;index:idx_position_snapshots_instrument_time_id,priority:2"`
	AssetClass           string    `gorm:"column:asset_class;size:64;not null;index:idx_position_snapshots_instrument_time_id,priority:3"`
	Quantity             float64   `gorm:"column:quantity;not null"`
	AverageEntryPrice    *float64  `gorm:"column:average_entry_price"`
	RealizedPnL          float64   `gorm:"column:realized_pnl;not null"`
	ExposureNotional     float64   `gorm:"column:exposure_notional;not null"`
	MetadataJSON         string    `gorm:"column:metadata_json;size:4096;not null"`
	EventTime            time.Time `gorm:"column:event_time;not null;index:idx_position_snapshots_mode_time_id,priority:2;index:idx_position_snapshots_strategy_time_id,priority:2;index:idx_position_snapshots_instrument_time_id,priority:4"`
	CreatedAt            time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt            time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (positionSnapshotModel) TableName(namer schema.Namer) string {
	return namer.TableName("position_snapshots")
}

type portfolioSnapshotModel struct {
	SnapshotID    string    `gorm:"column:snapshot_id;size:255;not null;primaryKey;uniqueIndex:idx_portfolio_snapshots_snapshot_id"`
	FillID        string    `gorm:"column:fill_id;size:255;not null;index:idx_portfolio_snapshots_fill_id"`
	Mode          string    `gorm:"column:mode;size:32;not null;index:idx_portfolio_snapshots_mode_time_id,priority:1"`
	GrossExposure float64   `gorm:"column:gross_exposure;not null"`
	NetExposure   float64   `gorm:"column:net_exposure;not null"`
	RealizedPnL   float64   `gorm:"column:realized_pnl;not null"`
	UnrealizedPnL *float64  `gorm:"column:unrealized_pnl"`
	MetadataJSON  string    `gorm:"column:metadata_json;size:4096;not null"`
	EventTime     time.Time `gorm:"column:event_time;not null;index:idx_portfolio_snapshots_mode_time_id,priority:2"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (portfolioSnapshotModel) TableName(namer schema.Namer) string {
	return namer.TableName("portfolio_snapshots")
}

// CreatePositionSnapshot idempotently persists a projected position snapshot.
func (s *DatabaseStore) CreatePositionSnapshot(
	ctx context.Context,
	snapshot domain.PositionSnapshot,
) (domain.PositionSnapshot, error) {
	model, err := positionSnapshotToModel(snapshot)
	if err != nil {
		return domain.PositionSnapshot{}, err
	}

	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: positionSnapshotIDColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return domain.PositionSnapshot{}, fmt.Errorf("create position snapshot: %w", err)
	}

	return positionSnapshotModelToDomain(model)
}

// QueryPositionSnapshots returns deterministic filtered position snapshots.
func (s *DatabaseStore) QueryPositionSnapshots(
	ctx context.Context,
	query PositionSnapshotQuery,
) ([]domain.PositionSnapshot, error) {
	db := applyExecutionListQuery(
		s.db.WithContext(ctx).Model(&positionSnapshotModel{}),
		query.StrategyID,
		query.Instrument,
		query.Mode,
		query.TimeRange,
		"event_time",
	)

	var models []positionSnapshotModel
	if err := db.Order("event_time ASC").Order("snapshot_id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query position snapshots: %w", err)
	}

	snapshots := make([]domain.PositionSnapshot, 0, len(models))
	for _, model := range models {
		snapshot, err := positionSnapshotModelToDomain(model)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// CreatePortfolioSnapshot idempotently persists a projected portfolio snapshot.
func (s *DatabaseStore) CreatePortfolioSnapshot(
	ctx context.Context,
	snapshot domain.PortfolioSnapshot,
) (domain.PortfolioSnapshot, error) {
	model, err := portfolioSnapshotToModel(snapshot)
	if err != nil {
		return domain.PortfolioSnapshot{}, err
	}

	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: portfolioSnapshotIDColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return domain.PortfolioSnapshot{}, fmt.Errorf("create portfolio snapshot: %w", err)
	}

	return portfolioSnapshotModelToDomain(model)
}

// QueryPortfolioSnapshots returns deterministic filtered portfolio snapshots.
func (s *DatabaseStore) QueryPortfolioSnapshots(
	ctx context.Context,
	query PortfolioSnapshotQuery,
) ([]domain.PortfolioSnapshot, error) {
	db := s.db.WithContext(ctx).Model(&portfolioSnapshotModel{})
	if query.Mode != nil {
		db = db.Where("mode = ?", query.Mode.String())
	}
	if query.TimeRange != nil {
		db = db.Where(
			"event_time >= ? AND event_time < ?",
			query.TimeRange.Start.UTC(),
			query.TimeRange.End.UTC(),
		)
	}

	var models []portfolioSnapshotModel
	if err := db.Order("event_time ASC").Order("snapshot_id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query portfolio snapshots: %w", err)
	}

	snapshots := make([]domain.PortfolioSnapshot, 0, len(models))
	for _, model := range models {
		snapshot, err := portfolioSnapshotModelToDomain(model)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func positionSnapshotToModel(snapshot domain.PositionSnapshot) (positionSnapshotModel, error) {
	metadataJSON, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return positionSnapshotModel{}, fmt.Errorf("marshal position snapshot metadata: %w", err)
	}

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
		return positionSnapshotModel{}, err
	}

	return positionSnapshotModel{
		SnapshotID:           canonical.SnapshotID.String(),
		FillID:               string(canonical.SourceFillID),
		Mode:                 canonical.Mode.String(),
		StrategyID:           canonical.StrategyID,
		StrategyVersion:      canonical.StrategyVersion,
		StrategyArtifactHash: canonical.StrategyArtifactHash,
		Venue:                canonical.Instrument.Venue.String(),
		Symbol:               canonical.Instrument.Symbol.String(),
		AssetClass:           canonical.Instrument.AssetClass.String(),
		Quantity:             canonical.Quantity,
		AverageEntryPrice:    canonical.AverageEntryPrice,
		RealizedPnL:          canonical.RealizedPnL,
		ExposureNotional:     canonical.ExposureNotional,
		MetadataJSON:         string(metadataJSON),
		EventTime:            canonical.EventTime.Time(),
	}, nil
}

func positionSnapshotModelToDomain(model positionSnapshotModel) (domain.PositionSnapshot, error) {
	metadata := map[string]string{}
	if err := json.Unmarshal([]byte(model.MetadataJSON), &metadata); err != nil {
		return domain.PositionSnapshot{}, fmt.Errorf("unmarshal position snapshot metadata: %w", err)
	}

	instrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      domain.Venue(model.Venue),
		Symbol:     domain.Symbol(model.Symbol),
		AssetClass: domain.AssetClass(model.AssetClass),
		Active:     true,
	})
	if err != nil {
		return domain.PositionSnapshot{}, fmt.Errorf("position snapshot instrument: %w", err)
	}

	return domain.NewPositionSnapshot(domain.PositionSnapshotParams{
		SnapshotID:           model.SnapshotID,
		SourceFillID:         model.FillID,
		Mode:                 domain.DecisionMode(model.Mode),
		StrategyID:           model.StrategyID,
		StrategyVersion:      model.StrategyVersion,
		StrategyArtifactHash: model.StrategyArtifactHash,
		Instrument:           instrument,
		Quantity:             model.Quantity,
		AverageEntryPrice:    model.AverageEntryPrice,
		RealizedPnL:          model.RealizedPnL,
		ExposureNotional:     model.ExposureNotional,
		EventTime:            model.EventTime,
		Metadata:             metadata,
	})
}

func portfolioSnapshotToModel(snapshot domain.PortfolioSnapshot) (portfolioSnapshotModel, error) {
	metadataJSON, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return portfolioSnapshotModel{}, fmt.Errorf("marshal portfolio snapshot metadata: %w", err)
	}

	canonical, err := domain.NewPortfolioSnapshot(domain.PortfolioSnapshotParams{
		SnapshotID:    snapshot.SnapshotID.String(),
		SourceFillID:  string(snapshot.SourceFillID),
		Mode:          snapshot.Mode,
		GrossExposure: snapshot.GrossExposure,
		NetExposure:   snapshot.NetExposure,
		RealizedPnL:   snapshot.RealizedPnL,
		UnrealizedPnL: snapshot.UnrealizedPnL,
		EventTime:     snapshot.EventTime.Time(),
		Metadata:      snapshot.Metadata,
	})
	if err != nil {
		return portfolioSnapshotModel{}, err
	}

	return portfolioSnapshotModel{
		SnapshotID:    canonical.SnapshotID.String(),
		FillID:        string(canonical.SourceFillID),
		Mode:          canonical.Mode.String(),
		GrossExposure: canonical.GrossExposure,
		NetExposure:   canonical.NetExposure,
		RealizedPnL:   canonical.RealizedPnL,
		UnrealizedPnL: canonical.UnrealizedPnL,
		MetadataJSON:  string(metadataJSON),
		EventTime:     canonical.EventTime.Time(),
	}, nil
}

func portfolioSnapshotModelToDomain(model portfolioSnapshotModel) (domain.PortfolioSnapshot, error) {
	metadata := map[string]string{}
	if err := json.Unmarshal([]byte(model.MetadataJSON), &metadata); err != nil {
		return domain.PortfolioSnapshot{}, fmt.Errorf("unmarshal portfolio snapshot metadata: %w", err)
	}

	return domain.NewPortfolioSnapshot(domain.PortfolioSnapshotParams{
		SnapshotID:    model.SnapshotID,
		SourceFillID:  model.FillID,
		Mode:          domain.DecisionMode(model.Mode),
		GrossExposure: model.GrossExposure,
		NetExposure:   model.NetExposure,
		RealizedPnL:   model.RealizedPnL,
		UnrealizedPnL: model.UnrealizedPnL,
		EventTime:     model.EventTime,
		Metadata:      metadata,
	})
}

func applyExecutionListQuery(
	db *gorm.DB,
	strategyID string,
	instrument *domain.Instrument,
	mode *domain.DecisionMode,
	timeRange *domain.TimeRange,
	timeColumn string,
) *gorm.DB {
	if trimmedStrategyID := strings.TrimSpace(strategyID); trimmedStrategyID != "" {
		db = db.Where("strategy_id = ?", trimmedStrategyID)
	}
	if instrument != nil {
		db = db.Where(
			"venue = ? AND symbol = ? AND asset_class = ?",
			instrument.Venue.String(),
			instrument.Symbol.String(),
			instrument.AssetClass.String(),
		)
	}
	if mode != nil {
		db = db.Where("mode = ?", mode.String())
	}
	if timeRange != nil {
		db = db.Where(
			timeColumn+" >= ? AND "+timeColumn+" < ?",
			timeRange.Start.UTC(),
			timeRange.End.UTC(),
		)
	}

	return db
}
