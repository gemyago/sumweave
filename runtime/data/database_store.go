package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/gormsignalfoundry"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const (
	columnVenue              = "venue"
	columnSymbol             = "symbol"
	columnAssetClass         = "asset_class"
	columnUpdatedAt          = "updated_at"
	columnInstrumentID       = "instrument_id"
	columnTimeframe          = "timeframe"
	columnStartAt            = "start_at"
	columnEndAt              = "end_at"
	columnEventTime          = "event_time"
	columnPrice              = "price"
	columnSize               = "size"
	columnQuality            = "quality"
	columnProvenanceSource   = "provenance_source"
	columnProvenanceRecordID = "provenance_record_id"
	columnProvenanceIdentity = "provenance_identity_key"
)

type instrumentModel struct {
	ID         uint      `gorm:"column:id;primaryKey;autoIncrement"`
	Venue      string    `gorm:"column:venue;size:255;not null;uniqueIndex:idx_instruments_venue_symbol"`
	Symbol     string    `gorm:"column:symbol;size:255;not null;uniqueIndex:idx_instruments_venue_symbol"`
	AssetClass string    `gorm:"column:asset_class;size:64;not null"`
	Active     bool      `gorm:"column:active;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (instrumentModel) TableName(namer schema.Namer) string { return namer.TableName("instruments") }

type candleModel struct {
	ID                 uint      `gorm:"column:id;primaryKey;autoIncrement"`
	InstrumentID       uint      `gorm:"column:instrument_id;not null;index;uniqueIndex:idx_candles_natural_key"`
	Timeframe          string    `gorm:"column:timeframe;size:32;not null;uniqueIndex:idx_candles_natural_key"`
	StartAt            time.Time `gorm:"column:start_at;not null;uniqueIndex:idx_candles_natural_key"`
	EndAt              time.Time `gorm:"column:end_at;not null;uniqueIndex:idx_candles_natural_key"`
	ProvenanceSource   string    `gorm:"column:provenance_source;size:255;not null;uniqueIndex:idx_candles_natural_key"`
	ProvenanceIdentity string    `gorm:"column:provenance_identity_key;size:255;not null;uniqueIndex:idx_candles_natural_key"`
	Open               float64   `gorm:"column:open;not null"`
	High               float64   `gorm:"column:high;not null"`
	Low                float64   `gorm:"column:low;not null"`
	Close              float64   `gorm:"column:close;not null"`
	Volume             float64   `gorm:"column:volume;not null"`
	Quality            string    `gorm:"column:quality;size:32;not null"`
	ProvenanceRecordID string    `gorm:"column:provenance_record_id;size:255"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (candleModel) TableName(namer schema.Namer) string { return namer.TableName("candles") }

type tradeModel struct {
	ID                 uint      `gorm:"column:id;primaryKey;autoIncrement"`
	InstrumentID       uint      `gorm:"column:instrument_id;not null;index;uniqueIndex:idx_trades_natural_key"`
	EventTime          time.Time `gorm:"column:event_time;not null"`
	Price              float64   `gorm:"column:price;not null"`
	Size               float64   `gorm:"column:size;not null"`
	Quality            string    `gorm:"column:quality;size:32;not null"`
	ProvenanceSource   string    `gorm:"column:provenance_source;size:255;not null;uniqueIndex:idx_trades_natural_key"`
	ProvenanceIdentity string    `gorm:"column:provenance_identity_key;size:255;not null;uniqueIndex:idx_trades_natural_key"`
	ProvenanceRecordID string    `gorm:"column:provenance_record_id;size:255"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (tradeModel) TableName(namer schema.Namer) string { return namer.TableName("trades") }

// DatabaseStore persists canonical data-layer records in SQLite or PostgreSQL via GORM.
type DatabaseStore struct {
	db *gorm.DB
}

// DatabaseStoreOpts configures store-level database concerns used by app wiring.
type DatabaseStoreOpts struct {
	TablePrefix string
}

// NewDatabaseStore opens a database-backed store for canonical instruments and market data.
func NewDatabaseStore(
	dsn string,
	opts DatabaseStoreOpts,
) (*DatabaseStore, error) {
	if dsn == "" {
		return nil, errors.New("dsn is required")
	}

	cfg := gormsignalfoundry.NewGormConfigForSignalFoundryTables(gormsignalfoundry.GormSignalFoundryTablesOpts{
		TablePrefix:    opts.TablePrefix,
		TranslateError: true,
	})
	cfg.NowFunc = func() time.Time {
		return time.Now().UTC()
	}

	db, err := gorm.Open(gormsignalfoundry.NewGormDialector(dsn), cfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return &DatabaseStore{db: db}, nil
}

// AutoMigrate creates or updates the data-layer relational schema.
func (s *DatabaseStore) AutoMigrate() error {
	return s.db.AutoMigrate(&instrumentModel{}, &candleModel{}, &tradeModel{})
}

func (s *DatabaseStore) UpsertInstrument(
	ctx context.Context,
	instrument domain.Instrument,
) (domain.Instrument, error) {
	if err := ctx.Err(); err != nil {
		return domain.Instrument{}, err
	}

	model := instrumentToModel(instrument)
	if createErr := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: columnVenue},
			{Name: columnSymbol},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			columnAssetClass,
			"active",
			columnUpdatedAt,
		}),
	}).Create(&model).Error; createErr != nil {
		return domain.Instrument{}, fmt.Errorf("upsert instrument row: %w", createErr)
	}

	persisted, err := s.findInstrumentModel(ctx, instrument.Venue, instrument.Symbol)
	if err != nil {
		return domain.Instrument{}, err
	}

	mapped, err := instrumentModelToDomain(persisted)
	if err != nil {
		return domain.Instrument{}, err
	}

	return mapped, nil
}

func (s *DatabaseStore) LookupInstrument(
	ctx context.Context,
	venue domain.Venue,
	symbol domain.Symbol,
) (*domain.Instrument, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model, err := s.findInstrumentModel(ctx, venue, symbol)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}

	mapped, err := instrumentModelToDomain(model)
	if err != nil {
		return nil, err
	}

	return &mapped, nil
}

func (s *DatabaseStore) UpsertCandle(ctx context.Context, candle domain.Candle) (domain.Candle, error) {
	if err := ctx.Err(); err != nil {
		return domain.Candle{}, err
	}

	instrumentRow, err := s.findInstrumentModel(ctx, candle.Instrument.Venue, candle.Instrument.Symbol)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("lookup candle instrument: %w", err)
	}

	model := candleToModel(candle, instrumentRow.ID)
	if createErr := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: columnInstrumentID},
			{Name: columnTimeframe},
			{Name: columnStartAt},
			{Name: columnEndAt},
			{Name: columnProvenanceSource},
			{Name: columnProvenanceIdentity},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"open",
			"high",
			"low",
			"close",
			"volume",
			columnQuality,
			columnProvenanceSource,
			columnProvenanceRecordID,
			columnUpdatedAt,
		}),
	}).Create(&model).Error; createErr != nil {
		return domain.Candle{}, fmt.Errorf("upsert candle row: %w", createErr)
	}

	persisted, err := s.findCandleModel(ctx, model)
	if err != nil {
		return domain.Candle{}, err
	}

	mapped, err := candleModelToDomain(persisted, instrumentRow)
	if err != nil {
		return domain.Candle{}, err
	}

	return mapped, nil
}

func (s *DatabaseStore) QueryCandles(
	ctx context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]domain.Candle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	instrumentRow, err := s.findInstrumentModel(ctx, instrument.Venue, instrument.Symbol)
	if err != nil {
		return nil, fmt.Errorf("lookup candle instrument: %w", err)
	}

	var rows []candleModel
	if queryErr := s.db.WithContext(ctx).
		Where("instrument_id = ? AND timeframe = ? AND start_at >= ? AND start_at < ?",
			instrumentRow.ID,
			timeframe.String(),
			timeRange.Start.UTC(),
			timeRange.End.UTC(),
		).
		Order("start_at ASC, id ASC").
		Find(&rows).Error; queryErr != nil {
		return nil, fmt.Errorf("query candles: %w", queryErr)
	}

	candles := make([]domain.Candle, 0, len(rows))
	for _, row := range rows {
		candle, mapErr := candleModelToDomain(row, instrumentRow)
		if mapErr != nil {
			return nil, mapErr
		}
		candles = append(candles, candle)
	}

	return candles, nil
}

func (s *DatabaseStore) ReplayCandles(
	ctx context.Context,
	instrument domain.Instrument,
	timeframe domain.Timeframe,
	timeRange domain.TimeRange,
) ([]ReplayCandle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	instrumentRow, err := s.findInstrumentModel(ctx, instrument.Venue, instrument.Symbol)
	if err != nil {
		return nil, fmt.Errorf("lookup candle instrument: %w", err)
	}

	var rows []candleModel
	if queryErr := s.db.WithContext(ctx).
		Where("instrument_id = ? AND timeframe = ? AND start_at >= ? AND start_at < ?",
			instrumentRow.ID,
			timeframe.String(),
			timeRange.Start.UTC(),
			timeRange.End.UTC(),
		).
		Order("start_at ASC, id ASC").
		Find(&rows).Error; queryErr != nil {
		return nil, fmt.Errorf("replay candles: %w", queryErr)
	}

	candles := make([]ReplayCandle, 0, len(rows))
	for _, row := range rows {
		candle, mapErr := candleModelToDomain(row, instrumentRow)
		if mapErr != nil {
			return nil, mapErr
		}
		candles = append(candles, ReplayCandle{
			Identity: uint64(row.ID),
			Candle:   candle,
		})
	}

	return candles, nil
}

func (s *DatabaseStore) UpsertTrade(ctx context.Context, trade domain.Trade) (domain.Trade, error) {
	if err := ctx.Err(); err != nil {
		return domain.Trade{}, err
	}

	instrumentRow, err := s.findInstrumentModel(ctx, trade.Instrument.Venue, trade.Instrument.Symbol)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("lookup trade instrument: %w", err)
	}

	model := tradeToModel(trade, instrumentRow.ID)
	if createErr := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: columnInstrumentID},
			{Name: columnProvenanceSource},
			{Name: columnProvenanceIdentity},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			columnEventTime,
			columnPrice,
			columnSize,
			columnQuality,
			columnProvenanceRecordID,
			columnUpdatedAt,
		}),
	}).Create(&model).Error; createErr != nil {
		return domain.Trade{}, fmt.Errorf("upsert trade row: %w", createErr)
	}

	persisted, err := s.findTradeModel(ctx, model)
	if err != nil {
		return domain.Trade{}, err
	}

	mapped, err := tradeModelToDomain(persisted, instrumentRow)
	if err != nil {
		return domain.Trade{}, err
	}

	return mapped, nil
}

func (s *DatabaseStore) QueryTrades(
	ctx context.Context,
	instrument domain.Instrument,
	timeRange domain.TimeRange,
) ([]domain.Trade, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	instrumentRow, err := s.findInstrumentModel(ctx, instrument.Venue, instrument.Symbol)
	if err != nil {
		return nil, fmt.Errorf("lookup trade instrument: %w", err)
	}

	var rows []tradeModel
	if queryErr := s.db.WithContext(ctx).
		Where("instrument_id = ? AND event_time >= ? AND event_time < ?",
			instrumentRow.ID,
			timeRange.Start.UTC(),
			timeRange.End.UTC(),
		).
		Order("event_time ASC, id ASC").
		Find(&rows).Error; queryErr != nil {
		return nil, fmt.Errorf("query trades: %w", queryErr)
	}

	trades := make([]domain.Trade, 0, len(rows))
	for _, row := range rows {
		trade, mapErr := tradeModelToDomain(row, instrumentRow)
		if mapErr != nil {
			return nil, mapErr
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

func (s *DatabaseStore) ReplayTrades(
	ctx context.Context,
	instrument domain.Instrument,
	timeRange domain.TimeRange,
) ([]ReplayTrade, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	instrumentRow, err := s.findInstrumentModel(ctx, instrument.Venue, instrument.Symbol)
	if err != nil {
		return nil, fmt.Errorf("lookup trade instrument: %w", err)
	}

	var rows []tradeModel
	if queryErr := s.db.WithContext(ctx).
		Where("instrument_id = ? AND event_time >= ? AND event_time < ?",
			instrumentRow.ID,
			timeRange.Start.UTC(),
			timeRange.End.UTC(),
		).
		Order("event_time ASC, id ASC").
		Find(&rows).Error; queryErr != nil {
		return nil, fmt.Errorf("replay trades: %w", queryErr)
	}

	trades := make([]ReplayTrade, 0, len(rows))
	for _, row := range rows {
		trade, mapErr := tradeModelToDomain(row, instrumentRow)
		if mapErr != nil {
			return nil, mapErr
		}
		trades = append(trades, ReplayTrade{
			Identity: uint64(row.ID),
			Trade:    trade,
		})
	}

	return trades, nil
}

func (s *DatabaseStore) findInstrumentModel(
	ctx context.Context,
	venue domain.Venue,
	symbol domain.Symbol,
) (instrumentModel, error) {
	var model instrumentModel
	if err := s.db.WithContext(ctx).
		Where("venue = ? AND symbol = ?", venue.String(), symbol.String()).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return instrumentModel{}, err
		}
		return instrumentModel{}, fmt.Errorf("lookup instrument row: %w", err)
	}

	return model, nil
}

func (s *DatabaseStore) findCandleModel(ctx context.Context, needle candleModel) (candleModel, error) {
	var model candleModel
	if err := s.db.WithContext(ctx).
		Where(
			"instrument_id = ? AND timeframe = ? AND start_at = ? AND end_at = ? AND provenance_source = ? AND provenance_identity_key = ?",
			needle.InstrumentID,
			needle.Timeframe,
			needle.StartAt.UTC(),
			needle.EndAt.UTC(),
			needle.ProvenanceSource,
			needle.ProvenanceIdentity,
		).
		First(&model).Error; err != nil {
		return candleModel{}, fmt.Errorf("lookup candle row: %w", err)
	}

	return model, nil
}

func (s *DatabaseStore) findTradeModel(ctx context.Context, needle tradeModel) (tradeModel, error) {
	var model tradeModel
	if err := s.db.WithContext(ctx).
		Where(
			"instrument_id = ? AND provenance_source = ? AND provenance_identity_key = ?",
			needle.InstrumentID,
			needle.ProvenanceSource,
			needle.ProvenanceIdentity,
		).
		First(&model).Error; err != nil {
		return tradeModel{}, fmt.Errorf("lookup trade row: %w", err)
	}

	return model, nil
}

func instrumentToModel(instrument domain.Instrument) instrumentModel {
	return instrumentModel{
		Venue:      instrument.Venue.String(),
		Symbol:     instrument.Symbol.String(),
		AssetClass: instrument.AssetClass.String(),
		Active:     instrument.Active,
	}
}

func instrumentModelToDomain(model instrumentModel) (domain.Instrument, error) {
	instrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      domain.Venue(model.Venue),
		Symbol:     domain.Symbol(model.Symbol),
		AssetClass: domain.AssetClass(model.AssetClass),
		Active:     model.Active,
	})
	if err != nil {
		return domain.Instrument{}, fmt.Errorf("map instrument row to domain: %w", err)
	}

	return instrument, nil
}

func candleToModel(candle domain.Candle, instrumentID uint) candleModel {
	return candleModel{
		InstrumentID:       instrumentID,
		Timeframe:          candle.Timeframe.String(),
		StartAt:            candle.TimeRange.Start.UTC(),
		EndAt:              candle.TimeRange.End.UTC(),
		ProvenanceSource:   candle.Provenance.Source,
		ProvenanceIdentity: candleIdentityKey(candle.Provenance),
		Open:               candle.Open,
		High:               candle.High,
		Low:                candle.Low,
		Close:              candle.Close,
		Volume:             candle.Volume,
		Quality:            candle.Quality.String(),
		ProvenanceRecordID: candle.Provenance.RecordID,
	}
}

func candleModelToDomain(model candleModel, instrumentModel instrumentModel) (domain.Candle, error) {
	instrument, err := instrumentModelToDomain(instrumentModel)
	if err != nil {
		return domain.Candle{}, err
	}

	timeframe, err := domain.NewTimeframe(model.Timeframe)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("map candle timeframe: %w", err)
	}

	timeRange, err := domain.NewTimeRange(model.StartAt, model.EndAt)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("map candle time range: %w", err)
	}

	quality, err := domain.NewDataQuality(model.Quality)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("map candle quality: %w", err)
	}

	provenance, err := domain.NewSourceProvenance(model.ProvenanceSource, model.ProvenanceRecordID)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("map candle provenance: %w", err)
	}

	candle, err := domain.NewCandle(domain.CandleParams{
		Instrument: instrument,
		Timeframe:  timeframe,
		TimeRange:  timeRange,
		Open:       model.Open,
		High:       model.High,
		Low:        model.Low,
		Close:      model.Close,
		Volume:     model.Volume,
		Quality:    quality,
		Provenance: provenance,
	})
	if err != nil {
		return domain.Candle{}, fmt.Errorf("map candle row to domain: %w", err)
	}

	return candle, nil
}

func tradeToModel(trade domain.Trade, instrumentID uint) tradeModel {
	return tradeModel{
		InstrumentID:       instrumentID,
		EventTime:          trade.EventTime.UTC(),
		Price:              trade.Price,
		Size:               trade.Size,
		Quality:            trade.Quality.String(),
		ProvenanceSource:   trade.Provenance.Source,
		ProvenanceIdentity: tradeIdentityKey(trade.EventTime, trade.Provenance),
		ProvenanceRecordID: trade.Provenance.RecordID,
	}
}

func candleIdentityKey(provenance domain.SourceProvenance) string {
	return provenance.RecordID
}

func tradeIdentityKey(eventTime time.Time, provenance domain.SourceProvenance) string {
	if provenance.RecordID != "" {
		return provenance.RecordID
	}

	return eventTime.UTC().Format(time.RFC3339Nano)
}

func tradeModelToDomain(model tradeModel, instrumentModel instrumentModel) (domain.Trade, error) {
	instrument, err := instrumentModelToDomain(instrumentModel)
	if err != nil {
		return domain.Trade{}, err
	}

	quality, err := domain.NewDataQuality(model.Quality)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("map trade quality: %w", err)
	}

	provenance, err := domain.NewSourceProvenance(model.ProvenanceSource, model.ProvenanceRecordID)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("map trade provenance: %w", err)
	}

	trade, err := domain.NewTrade(domain.TradeParams{
		Instrument: instrument,
		EventTime:  model.EventTime.UTC(),
		Price:      model.Price,
		Size:       model.Size,
		Quality:    quality,
		Provenance: provenance,
	})
	if err != nil {
		return domain.Trade{}, fmt.Errorf("map trade row to domain: %w", err)
	}

	return trade, nil
}
