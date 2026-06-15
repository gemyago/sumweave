package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	columnInstrumentSymbol   = "instrument_symbol"
	columnInstrumentAssetCls = "instrument_asset_class"
	columnUpdatedAt          = "updated_at"
	columnInstrumentID       = "instrument_id"
	columnTimeframe          = "timeframe"
	columnStartAt            = "start_at"
	columnEndAt              = "end_at"
	columnEventTime          = "event_time"
	columnPrice              = "price"
	columnSize               = "size"
	columnQuality            = "quality"
	columnStatus             = "status"
	columnCompletedAt        = "completed_at"
	columnRecordCount        = "record_count"
	columnErrorSummary       = "error_summary"
	columnProvenanceSource   = "provenance_source"
	columnProvenanceRecordID = "provenance_record_id"
	columnProvenanceIdentity = "provenance_identity_key"
	columnDataBatchID        = "data_batch_id"
	columnOpen               = "open"
	columnHigh               = "high"
	columnLow                = "low"
	columnClose              = "close"
	columnVolume             = "volume"
	columnRawPayloadID       = "raw_payload_id"
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
	DataBatchID        string    `gorm:"column:data_batch_id;size:255;index"`
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
	DataBatchID        string    `gorm:"column:data_batch_id;size:255;index"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (tradeModel) TableName(namer schema.Namer) string { return namer.TableName("trades") }

type ingestionRunModel struct {
	ID           string    `gorm:"column:id;size:255;not null;primaryKey;uniqueIndex:idx_ingestion_runs_id"`
	Source       string    `gorm:"column:source;size:255;not null"`
	Venue        string    `gorm:"column:venue;size:255;not null"`
	Status       string    `gorm:"column:status;size:32;not null"`
	StartedAt    time.Time `gorm:"column:started_at;not null"`
	CompletedAt  time.Time `gorm:"column:completed_at"`
	RecordCount  int       `gorm:"column:record_count;not null"`
	ErrorSummary string    `gorm:"column:error_summary"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (ingestionRunModel) TableName(namer schema.Namer) string {
	return namer.TableName("ingestion_runs")
}

type rawVenuePayloadModel struct {
	ID                  string    `gorm:"column:id;size:255;not null;primaryKey;uniqueIndex:idx_raw_venue_payloads_id"`
	IngestionRunID      string    `gorm:"column:ingestion_run_id;size:255;index"`
	Source              string    `gorm:"column:source;size:255;not null"`
	Venue               string    `gorm:"column:venue;size:255;not null"`
	Endpoint            string    `gorm:"column:endpoint;size:255;not null"`
	RequestType         string    `gorm:"column:request_type;size:255;not null"`
	RequestPayloadHash  string    `gorm:"column:request_payload_hash;size:255;not null;index"`
	RequestMetadataJSON string    `gorm:"column:request_metadata_json;type:text"`
	RequestAt           time.Time `gorm:"column:request_at;not null"`
	ResponseAt          time.Time `gorm:"column:response_at;not null"`
	HTTPStatus          int       `gorm:"column:http_status;not null"`
	ResponseBodyHash    string    `gorm:"column:response_body_hash;size:255;not null"`
	PayloadBodyRef      string    `gorm:"column:payload_body_ref;size:1024;not null"`
	EntityHint          string    `gorm:"column:entity_hint;size:255"`
	InstrumentSymbol    string    `gorm:"column:instrument_symbol;size:255"`
	InstrumentAssetCls  string    `gorm:"column:instrument_asset_class;size:64"`
	Timeframe           string    `gorm:"column:timeframe;size:32"`
	StartAt             time.Time `gorm:"column:start_at"`
	EndAt               time.Time `gorm:"column:end_at"`
	ReceivedAt          time.Time `gorm:"column:received_at;not null;index:idx_raw_venue_payloads_received_at_id,priority:1"`
	CreatedAt           time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt           time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (rawVenuePayloadModel) TableName(namer schema.Namer) string {
	return namer.TableName("raw_venue_payloads")
}

type normalizationRunModel struct {
	ID                   string    `gorm:"column:id;size:255;not null;primaryKey;uniqueIndex:idx_normalization_runs_id"`
	Status               string    `gorm:"column:status;size:32;not null"`
	StartedAt            time.Time `gorm:"column:started_at;not null"`
	CompletedAt          time.Time `gorm:"column:completed_at"`
	RecordKind           string    `gorm:"column:record_kind;size:32;not null"`
	SourceRecordCount    int       `gorm:"column:source_record_count;not null"`
	CanonicalRecordCount int       `gorm:"column:canonical_record_count;not null"`
	ErrorSummary         string    `gorm:"column:error_summary"`
	CreatedAt            time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt            time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (normalizationRunModel) TableName(namer schema.Namer) string {
	return namer.TableName("normalization_runs")
}

type normalizationRunRawPayloadLinkModel struct {
	NormalizationRunID string    `gorm:"column:normalization_run_id;size:255;not null;primaryKey;uniqueIndex:idx_norm_run_raw_payload_links,priority:1;index"`
	RawPayloadID       string    `gorm:"column:raw_payload_id;size:255;not null;primaryKey;uniqueIndex:idx_norm_run_raw_payload_links,priority:2;index"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (normalizationRunRawPayloadLinkModel) TableName(namer schema.Namer) string {
	return namer.TableName("normalization_run_raw_payload_links")
}

type dataBatchModel struct {
	ID                 string    `gorm:"column:id;size:255;not null;primaryKey;uniqueIndex:idx_data_batches_id"`
	NormalizationRunID string    `gorm:"column:normalization_run_id;size:255;not null;index"`
	Venue              string    `gorm:"column:venue;size:255;not null"`
	InstrumentSymbol   string    `gorm:"column:instrument_symbol;size:255"`
	InstrumentAssetCls string    `gorm:"column:instrument_asset_class;size:64"`
	RecordKind         string    `gorm:"column:record_kind;size:32;not null"`
	StartAt            time.Time `gorm:"column:start_at;not null"`
	EndAt              time.Time `gorm:"column:end_at;not null"`
	Quality            string    `gorm:"column:quality;size:32;not null"`
	RecordCount        int       `gorm:"column:record_count;not null"`
	Summary            string    `gorm:"column:summary"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (dataBatchModel) TableName(namer schema.Namer) string { return namer.TableName("data_batches") }

type rawPayloadInstrumentLinkModel struct {
	RawPayloadID string    `gorm:"column:raw_payload_id;size:255;not null;primaryKey;uniqueIndex:idx_raw_payload_instrument_links,priority:1;index"`
	InstrumentID uint      `gorm:"column:instrument_id;not null;primaryKey;uniqueIndex:idx_raw_payload_instrument_links,priority:2;index"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (rawPayloadInstrumentLinkModel) TableName(namer schema.Namer) string {
	return namer.TableName("raw_payload_instrument_links")
}

type rawPayloadCandleLinkModel struct {
	RawPayloadID string    `gorm:"column:raw_payload_id;size:255;not null;primaryKey;uniqueIndex:idx_raw_payload_candle_links,priority:1;index"`
	CandleID     uint      `gorm:"column:candle_id;not null;primaryKey;uniqueIndex:idx_raw_payload_candle_links,priority:2;index"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (rawPayloadCandleLinkModel) TableName(namer schema.Namer) string {
	return namer.TableName("raw_payload_candle_links")
}

type rawPayloadTradeLinkModel struct {
	RawPayloadID string    `gorm:"column:raw_payload_id;size:255;not null;primaryKey;uniqueIndex:idx_raw_payload_trade_links,priority:1;index"`
	TradeID      uint      `gorm:"column:trade_id;not null;primaryKey;uniqueIndex:idx_raw_payload_trade_links,priority:2;index"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (rawPayloadTradeLinkModel) TableName(namer schema.Namer) string {
	return namer.TableName("raw_payload_trade_links")
}

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
	return s.db.AutoMigrate(
		&instrumentModel{},
		&candleModel{},
		&tradeModel{},
		&ingestionRunModel{},
		&rawVenuePayloadModel{},
		&normalizationRunModel{},
		&normalizationRunRawPayloadLinkModel{},
		&dataBatchModel{},
		&rawPayloadInstrumentLinkModel{},
		&rawPayloadCandleLinkModel{},
		&rawPayloadTradeLinkModel{},
	)
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
	return s.upsertCandle(ctx, candle, "")
}

func (s *DatabaseStore) UpsertCandleForDataBatch(
	ctx context.Context,
	batchID string,
	candle domain.Candle,
) (domain.Candle, error) {
	return s.upsertCandle(ctx, candle, batchID)
}

func (s *DatabaseStore) upsertCandle(
	ctx context.Context,
	candle domain.Candle,
	batchID string,
) (domain.Candle, error) {
	if err := ctx.Err(); err != nil {
		return domain.Candle{}, err
	}

	instrumentRow, err := s.findInstrumentModel(ctx, candle.Instrument.Venue, candle.Instrument.Symbol)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("lookup candle instrument: %w", err)
	}

	model := candleToModel(candle, instrumentRow.ID, batchID)
	assignmentColumns := []string{
		columnOpen,
		columnHigh,
		columnLow,
		columnClose,
		columnVolume,
		columnQuality,
		columnProvenanceSource,
		columnProvenanceRecordID,
		columnUpdatedAt,
	}
	if batchID != "" {
		if ensureErr := ensureDataBatchSupportsRecordKind(
			s.db.WithContext(ctx),
			batchID,
			LineageRecordKindCandle,
		); ensureErr != nil {
			return domain.Candle{}, ensureErr
		}
		assignmentColumns = append(assignmentColumns, columnDataBatchID)
	}

	if createErr := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: columnInstrumentID},
			{Name: columnTimeframe},
			{Name: columnStartAt},
			{Name: columnEndAt},
			{Name: columnProvenanceSource},
			{Name: columnProvenanceIdentity},
		},
		DoUpdates: clause.AssignmentColumns(assignmentColumns),
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
	return s.upsertTrade(ctx, trade, "")
}

func (s *DatabaseStore) UpsertTradeForDataBatch(
	ctx context.Context,
	batchID string,
	trade domain.Trade,
) (domain.Trade, error) {
	return s.upsertTrade(ctx, trade, batchID)
}

func (s *DatabaseStore) upsertTrade(
	ctx context.Context,
	trade domain.Trade,
	batchID string,
) (domain.Trade, error) {
	if err := ctx.Err(); err != nil {
		return domain.Trade{}, err
	}

	instrumentRow, err := s.findInstrumentModel(ctx, trade.Instrument.Venue, trade.Instrument.Symbol)
	if err != nil {
		return domain.Trade{}, fmt.Errorf("lookup trade instrument: %w", err)
	}

	model := tradeToModel(trade, instrumentRow.ID, batchID)
	assignmentColumns := []string{
		columnEventTime,
		columnPrice,
		columnSize,
		columnQuality,
		columnProvenanceRecordID,
		columnUpdatedAt,
	}
	if batchID != "" {
		if ensureErr := ensureDataBatchSupportsRecordKind(
			s.db.WithContext(ctx),
			batchID,
			LineageRecordKindTrade,
		); ensureErr != nil {
			return domain.Trade{}, ensureErr
		}
		assignmentColumns = append(assignmentColumns, columnDataBatchID)
	}

	if createErr := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: columnInstrumentID},
			{Name: columnProvenanceSource},
			{Name: columnProvenanceIdentity},
		},
		DoUpdates: clause.AssignmentColumns(assignmentColumns),
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

func (s *DatabaseStore) ReplayCandlesByDataBatch(
	ctx context.Context,
	batchID string,
) ([]ReplayCandle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var rows []candleModel
	if err := s.db.WithContext(ctx).
		Where("data_batch_id = ?", batchID).
		Order("start_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("replay candles by data batch: %w", err)
	}

	return mapReplayCandles(s.db.WithContext(ctx), rows)
}

func (s *DatabaseStore) ReplayTradesByDataBatch(
	ctx context.Context,
	batchID string,
) ([]ReplayTrade, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var rows []tradeModel
	if err := s.db.WithContext(ctx).
		Where("data_batch_id = ?", batchID).
		Order("event_time ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("replay trades by data batch: %w", err)
	}

	return mapReplayTrades(s.db.WithContext(ctx), rows)
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

func (s *DatabaseStore) UpsertIngestionRun(
	ctx context.Context,
	run IngestionRun,
) (IngestionRun, error) {
	if err := ctx.Err(); err != nil {
		return IngestionRun{}, err
	}

	model := ingestionRunToModel(run)
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			columnStatus,
			columnCompletedAt,
			columnRecordCount,
			columnErrorSummary,
			columnUpdatedAt,
		}),
	}).Create(&model).Error; err != nil {
		return IngestionRun{}, fmt.Errorf("upsert ingestion run row: %w", err)
	}

	persisted, err := s.findIngestionRunModel(ctx, run.ID)
	if err != nil {
		return IngestionRun{}, err
	}

	return ingestionRunModelToDomain(persisted)
}

func (s *DatabaseStore) UpsertRawVenuePayload(
	ctx context.Context,
	payload RawVenuePayload,
) (RawVenuePayload, error) {
	if err := ctx.Err(); err != nil {
		return RawVenuePayload{}, err
	}

	var persisted rawVenuePayloadModel
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureIngestionRunExists(tx, payload.IngestionRunID); err != nil {
			return err
		}

		model, err := rawVenuePayloadToModel(payload)
		if err != nil {
			return err
		}

		if createErr := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"ingestion_run_id",
				"endpoint",
				"request_type",
				"request_payload_hash",
				"request_metadata_json",
				"request_at",
				"response_at",
				"http_status",
				"received_at",
				"entity_hint",
				columnInstrumentSymbol,
				columnInstrumentAssetCls,
				columnTimeframe,
				columnStartAt,
				columnEndAt,
				columnUpdatedAt,
			}),
		}).Create(&model).Error; createErr != nil {
			return fmt.Errorf("upsert raw venue payload row: %w", createErr)
		}

		if lookupErr := tx.Where("id = ?", payload.ID).First(&persisted).Error; lookupErr != nil {
			return fmt.Errorf("lookup raw venue payload row: %w", lookupErr)
		}

		return nil
	}); err != nil {
		return RawVenuePayload{}, err
	}

	return rawVenuePayloadModelToDomain(persisted)
}

func (s *DatabaseStore) UpsertNormalizationRun(
	ctx context.Context,
	run NormalizationRun,
) (NormalizationRun, error) {
	if err := ctx.Err(); err != nil {
		return NormalizationRun{}, err
	}

	var persisted normalizationRunModel
	var rawPayloadIDs []string
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureRawPayloadsExist(tx, run.RawPayloadIDs); err != nil {
			return err
		}

		model := normalizationRunToModel(run)
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnStatus,
				columnCompletedAt,
				"source_record_count",
				"canonical_record_count",
				columnErrorSummary,
				columnUpdatedAt,
			}),
		}).Create(&model).Error; err != nil {
			return fmt.Errorf("upsert normalization run row: %w", err)
		}

		if err := replaceNormalizationRunRawPayloadLinks(tx, run.ID, run.RawPayloadIDs); err != nil {
			return err
		}

		if err := tx.Where("id = ?", run.ID).First(&persisted).Error; err != nil {
			return fmt.Errorf("lookup normalization run row: %w", err)
		}

		var err error
		rawPayloadIDs, err = listNormalizationRunRawPayloadIDs(tx, run.ID)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return NormalizationRun{}, err
	}

	return normalizationRunModelToDomain(persisted, rawPayloadIDs)
}

func (s *DatabaseStore) UpsertDataBatch(
	ctx context.Context,
	batch DataBatch,
) (DataBatch, error) {
	if err := ctx.Err(); err != nil {
		return DataBatch{}, err
	}

	var persisted dataBatchModel
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureNormalizationRunExists(tx, batch.NormalizationRunID); err != nil {
			return err
		}

		model := dataBatchToModel(batch)
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnQuality,
				columnRecordCount,
				"summary",
				columnUpdatedAt,
			}),
		}).Create(&model).Error; err != nil {
			return fmt.Errorf("upsert data batch row: %w", err)
		}

		if err := tx.Where("id = ?", batch.ID).First(&persisted).Error; err != nil {
			return fmt.Errorf("lookup data batch row: %w", err)
		}

		return nil
	}); err != nil {
		return DataBatch{}, err
	}

	return dataBatchModelToDomain(persisted)
}

// ListRawPayloadMetadata returns one deterministic page of raw payload metadata rows.
func (s *DatabaseStore) ListRawPayloadMetadata(
	ctx context.Context,
	query RawPayloadMetadataListQuery,
) (RawPayloadMetadataListResult, error) {
	if err := ctx.Err(); err != nil {
		return RawPayloadMetadataListResult{}, err
	}

	canonicalQuery, err := canonicalizeRawPayloadMetadataListQuery(query)
	if err != nil {
		return RawPayloadMetadataListResult{}, err
	}

	rows, err := queryRawPayloadMetadataRows(s.db.WithContext(ctx), canonicalQuery, canonicalQuery.Limit+1)
	if err != nil {
		return RawPayloadMetadataListResult{}, err
	}

	result := RawPayloadMetadataListResult{Items: make([]RawPayloadMetadata, 0, min(len(rows), canonicalQuery.Limit))}
	for idx, row := range rows {
		if idx == canonicalQuery.Limit {
			lastReturned := rows[canonicalQuery.Limit-1]
			result.NextCursor = encodeRawPayloadListCursor(lastReturned.ReceivedAt, lastReturned.ID)
			break
		}

		metadata, metadataErr := rawPayloadMetadataFromModel(row)
		if metadataErr != nil {
			return RawPayloadMetadataListResult{}, metadataErr
		}
		result.Items = append(result.Items, metadata)
	}

	return result, nil
}

// GetRawPayloadMetadata returns one raw payload metadata row by ID.
func (s *DatabaseStore) GetRawPayloadMetadata(
	ctx context.Context,
	rawPayloadID string,
) (RawPayloadMetadata, error) {
	if err := ctx.Err(); err != nil {
		return RawPayloadMetadata{}, err
	}

	canonicalRawPayloadID, err := canonicalizeRawPayloadID(rawPayloadID)
	if err != nil {
		return RawPayloadMetadata{}, err
	}

	var row rawVenuePayloadModel
	lookupErr := s.db.WithContext(ctx).Where("id = ?", canonicalRawPayloadID).First(&row).Error
	if lookupErr != nil {
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return RawPayloadMetadata{}, ErrRawPayloadNotFound
		}
		return RawPayloadMetadata{}, fmt.Errorf("lookup raw payload row: %w", lookupErr)
	}

	return rawPayloadMetadataFromModel(row)
}

// ListCandleLinkedRawPayloadMetadata returns raw payload metadata linked to one exact candle key.
func (s *DatabaseStore) ListCandleLinkedRawPayloadMetadata(
	ctx context.Context,
	query CandleLinkedRawPayloadsQuery,
) ([]RawPayloadMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	canonicalQuery, err := canonicalizeCandleLinkedRawPayloadsQuery(query)
	if err != nil {
		return nil, err
	}

	rows, err := s.queryCandleLinkedRawPayloadMetadataRows(ctx, canonicalQuery)
	if err != nil {
		return nil, err
	}

	items := make([]RawPayloadMetadata, 0, len(rows))
	for _, row := range rows {
		metadata, metadataErr := rawPayloadMetadataFromModel(row)
		if metadataErr != nil {
			return nil, metadataErr
		}
		items = append(items, metadata)
	}

	return items, nil
}

// LinkRawPayloadToInstrument persists a raw-payload link to one canonical instrument.
func (s *DatabaseStore) LinkRawPayloadToInstrument(
	ctx context.Context,
	rawPayloadID string,
	instrument domain.Instrument,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureRawPayloadsExist(tx, []string{rawPayloadID}); err != nil {
			return err
		}
		instrumentRow, err := findInstrumentModelByIdentity(tx, instrument.Venue, instrument.Symbol)
		if err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rawPayloadInstrumentLinkModel{
			RawPayloadID: rawPayloadID,
			InstrumentID: instrumentRow.ID,
		}).Error
	})
}

// LinkRawPayloadToCandle persists a raw-payload link to one canonical candle.
func (s *DatabaseStore) LinkRawPayloadToCandle(
	ctx context.Context,
	rawPayloadID string,
	candle domain.Candle,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureRawPayloadsExist(tx, []string{rawPayloadID}); err != nil {
			return err
		}
		instrumentRow, err := findInstrumentModelByIdentity(tx, candle.Instrument.Venue, candle.Instrument.Symbol)
		if err != nil {
			return err
		}
		candleRow, err := findCandleModelByNaturalKey(tx, candle, instrumentRow.ID)
		if err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rawPayloadCandleLinkModel{
			RawPayloadID: rawPayloadID,
			CandleID:     candleRow.ID,
		}).Error
	})
}

// LinkRawPayloadToTrade persists a raw-payload link to one canonical trade.
func (s *DatabaseStore) LinkRawPayloadToTrade(
	ctx context.Context,
	rawPayloadID string,
	trade domain.Trade,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureRawPayloadsExist(tx, []string{rawPayloadID}); err != nil {
			return err
		}
		instrumentRow, err := findInstrumentModelByIdentity(tx, trade.Instrument.Venue, trade.Instrument.Symbol)
		if err != nil {
			return err
		}
		tradeRow, err := findTradeModelByNaturalKey(tx, trade, instrumentRow.ID)
		if err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rawPayloadTradeLinkModel{
			RawPayloadID: rawPayloadID,
			TradeID:      tradeRow.ID,
		}).Error
	})
}

// ListInstrumentRawPayloadIDs returns stable raw payload IDs linked to one instrument.
func (s *DatabaseStore) ListInstrumentRawPayloadIDs(
	ctx context.Context,
	instrument domain.Instrument,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	instrumentRow, err := findInstrumentModelByIdentity(s.db.WithContext(ctx), instrument.Venue, instrument.Symbol)
	if err != nil {
		return nil, err
	}
	return listRawPayloadIDsByLinkedColumn(
		s.db.WithContext(ctx),
		rawPayloadInstrumentLinkModel{},
		"instrument_id",
		instrumentRow.ID,
	)
}

// ListCandleRawPayloadIDs returns stable raw payload IDs linked to one candle.
func (s *DatabaseStore) ListCandleRawPayloadIDs(ctx context.Context, candle domain.Candle) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	instrumentRow, err := findInstrumentModelByIdentity(
		s.db.WithContext(ctx),
		candle.Instrument.Venue,
		candle.Instrument.Symbol,
	)
	if err != nil {
		return nil, err
	}
	candleRow, err := findCandleModelByNaturalKey(s.db.WithContext(ctx), candle, instrumentRow.ID)
	if err != nil {
		return nil, err
	}
	return listRawPayloadIDsByLinkedColumn(
		s.db.WithContext(ctx),
		rawPayloadCandleLinkModel{},
		"candle_id",
		candleRow.ID,
	)
}

// ListTradeRawPayloadIDs returns stable raw payload IDs linked to one trade.
func (s *DatabaseStore) ListTradeRawPayloadIDs(ctx context.Context, trade domain.Trade) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	instrumentRow, err := findInstrumentModelByIdentity(
		s.db.WithContext(ctx),
		trade.Instrument.Venue,
		trade.Instrument.Symbol,
	)
	if err != nil {
		return nil, err
	}
	tradeRow, err := findTradeModelByNaturalKey(s.db.WithContext(ctx), trade, instrumentRow.ID)
	if err != nil {
		return nil, err
	}
	return listRawPayloadIDsByLinkedColumn(
		s.db.WithContext(ctx),
		rawPayloadTradeLinkModel{},
		"trade_id",
		tradeRow.ID,
	)
}

func (s *DatabaseStore) GetDataBatchAudit(ctx context.Context, batchID string) (DataBatchAudit, error) {
	if err := ctx.Err(); err != nil {
		return DataBatchAudit{}, err
	}

	audit, err := loadDataBatchAudit(s.db.WithContext(ctx), batchID)
	if err != nil {
		return DataBatchAudit{}, err
	}

	return audit, nil
}

func (s *DatabaseStore) queryCandleLinkedRawPayloadMetadataRows(
	ctx context.Context,
	query CandleLinkedRawPayloadsQuery,
) ([]rawVenuePayloadModel, error) {
	rawPayloadTable := s.db.Config.NamingStrategy.TableName("raw_venue_payloads")
	rawPayloadCandleLinkTable := s.db.Config.NamingStrategy.TableName("raw_payload_candle_links")
	candleTable := s.db.Config.NamingStrategy.TableName("candles")
	instrumentTable := s.db.Config.NamingStrategy.TableName("instruments")

	var rows []rawVenuePayloadModel
	err := s.db.WithContext(ctx).
		Model(&rawVenuePayloadModel{}).
		Joins(
			fmt.Sprintf(
				"JOIN %s ON %s.raw_payload_id = %s.id",
				rawPayloadCandleLinkTable,
				rawPayloadCandleLinkTable,
				rawPayloadTable,
			),
		).
		Joins(
			fmt.Sprintf(
				"JOIN %s ON %s.id = %s.candle_id",
				candleTable,
				candleTable,
				rawPayloadCandleLinkTable,
			),
		).
		Joins(
			fmt.Sprintf(
				"JOIN %s ON %s.id = %s.instrument_id",
				instrumentTable,
				instrumentTable,
				candleTable,
			),
		).
		Where(fmt.Sprintf("%s.venue = ?", instrumentTable), query.Venue.String()).
		Where(fmt.Sprintf("%s.symbol = ?", instrumentTable), query.Symbol.String()).
		Where(fmt.Sprintf("%s.asset_class = ?", instrumentTable), query.AssetClass.String()).
		Where(fmt.Sprintf("%s.timeframe = ?", candleTable), query.Timeframe.String()).
		Where(fmt.Sprintf("%s.start_at = ?", candleTable), query.TimeRange.Start.UTC()).
		Where(fmt.Sprintf("%s.end_at = ?", candleTable), query.TimeRange.End.UTC()).
		Where(fmt.Sprintf("%s.provenance_source = ?", candleTable), query.ProvenanceSource).
		Where(fmt.Sprintf("%s.provenance_identity_key = ?", candleTable), query.ProvenanceIdentity).
		Order(fmt.Sprintf("%s.received_at ASC", rawPayloadTable)).
		Order(fmt.Sprintf("%s.id ASC", rawPayloadTable)).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query candle linked raw payload rows: %w", err)
	}

	return rows, nil
}

func (s *DatabaseStore) findInstrumentModel(
	ctx context.Context,
	venue domain.Venue,
	symbol domain.Symbol,
) (instrumentModel, error) {
	return findInstrumentModelByIdentity(s.db.WithContext(ctx), venue, symbol)
}

func findInstrumentModelByIdentity(
	db *gorm.DB,
	venue domain.Venue,
	symbol domain.Symbol,
) (instrumentModel, error) {
	var model instrumentModel
	if err := db.Where("venue = ? AND symbol = ?", venue.String(), symbol.String()).First(&model).Error; err != nil {
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

func findCandleModelByNaturalKey(
	db *gorm.DB,
	candle domain.Candle,
	instrumentID uint,
) (candleModel, error) {
	target := candleToModel(candle, instrumentID, "")
	var model candleModel
	if err := db.Where(
		"instrument_id = ? AND timeframe = ? AND start_at = ? AND end_at = ? AND provenance_source = ? AND provenance_identity_key = ?",
		target.InstrumentID,
		target.Timeframe,
		target.StartAt.UTC(),
		target.EndAt.UTC(),
		target.ProvenanceSource,
		target.ProvenanceIdentity,
	).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return candleModel{}, fmt.Errorf("lookup candle row: %w", ErrLineageParentNotFound)
		}
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

func findTradeModelByNaturalKey(
	db *gorm.DB,
	trade domain.Trade,
	instrumentID uint,
) (tradeModel, error) {
	target := tradeToModel(trade, instrumentID, "")
	var model tradeModel
	if err := db.Where(
		"instrument_id = ? AND provenance_source = ? AND provenance_identity_key = ?",
		target.InstrumentID,
		target.ProvenanceSource,
		target.ProvenanceIdentity,
	).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tradeModel{}, fmt.Errorf("lookup trade row: %w", ErrLineageParentNotFound)
		}
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

func candleToModel(candle domain.Candle, instrumentID uint, batchID string) candleModel {
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
		DataBatchID:        batchID,
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

func tradeToModel(trade domain.Trade, instrumentID uint, batchID string) tradeModel {
	return tradeModel{
		InstrumentID:       instrumentID,
		EventTime:          trade.EventTime.UTC(),
		Price:              trade.Price,
		Size:               trade.Size,
		Quality:            trade.Quality.String(),
		ProvenanceSource:   trade.Provenance.Source,
		ProvenanceIdentity: tradeIdentityKey(trade.EventTime, trade.Provenance),
		ProvenanceRecordID: trade.Provenance.RecordID,
		DataBatchID:        batchID,
	}
}

func ingestionRunToModel(run IngestionRun) ingestionRunModel {
	return ingestionRunModel{
		ID:           run.ID,
		Source:       run.Source,
		Venue:        run.Venue.String(),
		Status:       run.Status.String(),
		StartedAt:    run.StartedAt.UTC(),
		CompletedAt:  run.CompletedAt.UTC(),
		RecordCount:  run.RecordCount,
		ErrorSummary: run.ErrorSummary,
	}
}

func ingestionRunModelToDomain(model ingestionRunModel) (IngestionRun, error) {
	run, err := NewIngestionRun(IngestionRunParams{
		ID:           model.ID,
		Source:       model.Source,
		Venue:        domain.Venue(model.Venue),
		Status:       IngestionRunStatus(model.Status),
		StartedAt:    model.StartedAt.UTC(),
		CompletedAt:  model.CompletedAt.UTC(),
		RecordCount:  model.RecordCount,
		ErrorSummary: model.ErrorSummary,
	})
	if err != nil {
		return IngestionRun{}, fmt.Errorf("map ingestion run row to domain: %w", err)
	}

	return run, nil
}

func rawVenuePayloadToModel(payload RawVenuePayload) (rawVenuePayloadModel, error) {
	requestMetadataJSON, err := marshalLineageMetadata(payload.RequestMetadata)
	if err != nil {
		return rawVenuePayloadModel{}, err
	}

	var instrumentSymbol string
	var instrumentAssetClass string
	if payload.Instrument != nil {
		instrumentSymbol = payload.Instrument.Symbol.String()
		instrumentAssetClass = payload.Instrument.AssetClass.String()
	}

	var timeframe string
	if payload.Timeframe != "" {
		timeframe = payload.Timeframe.String()
	}

	var startAt time.Time
	var endAt time.Time
	if payload.TimeRange != nil {
		startAt = payload.TimeRange.Start.UTC()
		endAt = payload.TimeRange.End.UTC()
	}
	if payload.PayloadBodyRef == "" {
		return rawVenuePayloadModel{}, validationError("raw payload body ref is required")
	}
	if payload.ResponseBodyHash == "" {
		return rawVenuePayloadModel{}, validationError("raw payload response body hash is required")
	}

	return rawVenuePayloadModel{
		ID:                  payload.ID,
		IngestionRunID:      payload.IngestionRunID,
		Source:              payload.Source,
		Venue:               payload.Venue.String(),
		Endpoint:            payload.Endpoint,
		RequestType:         payload.RequestType,
		RequestPayloadHash:  payload.RequestPayloadHash,
		RequestMetadataJSON: requestMetadataJSON,
		RequestAt:           payload.RequestAt.UTC(),
		ResponseAt:          payload.ResponseAt.UTC(),
		HTTPStatus:          payload.HTTPStatus,
		ResponseBodyHash:    payload.ResponseBodyHash,
		PayloadBodyRef:      payload.PayloadBodyRef,
		EntityHint:          payload.EntityHint,
		InstrumentSymbol:    instrumentSymbol,
		InstrumentAssetCls:  instrumentAssetClass,
		Timeframe:           timeframe,
		StartAt:             startAt,
		EndAt:               endAt,
		ReceivedAt:          payload.ReceivedAt.UTC(),
	}, nil
}

func rawVenuePayloadModelToDomain(model rawVenuePayloadModel) (RawVenuePayload, error) {
	requestMetadata, err := unmarshalLineageMetadata(model.RequestMetadataJSON)
	if err != nil {
		return RawVenuePayload{}, err
	}

	var instrument *BatchInstrumentRef
	if strings.TrimSpace(model.InstrumentSymbol) != "" || strings.TrimSpace(model.InstrumentAssetCls) != "" {
		instrument = &BatchInstrumentRef{
			Symbol:     domain.Symbol(model.InstrumentSymbol),
			AssetClass: domain.AssetClass(model.InstrumentAssetCls),
		}
	}

	var timeRange *domain.TimeRange
	if !model.StartAt.IsZero() || !model.EndAt.IsZero() {
		candidate := domain.TimeRange{Start: model.StartAt.UTC(), End: model.EndAt.UTC()}
		timeRange = &candidate
	}

	payload, err := NewRawVenuePayload(RawVenuePayloadParams{
		ID:                 model.ID,
		IngestionRunID:     model.IngestionRunID,
		Source:             model.Source,
		Venue:              domain.Venue(model.Venue),
		Endpoint:           model.Endpoint,
		RequestType:        model.RequestType,
		RequestPayloadHash: model.RequestPayloadHash,
		RequestMetadata:    requestMetadata,
		RequestAt:          model.RequestAt.UTC(),
		ResponseAt:         model.ResponseAt.UTC(),
		HTTPStatus:         model.HTTPStatus,
		ResponseBodyHash:   model.ResponseBodyHash,
		PayloadBodyRef:     model.PayloadBodyRef,
		EntityHint:         model.EntityHint,
		Instrument:         instrument,
		Timeframe:          domain.Timeframe(model.Timeframe),
		TimeRange:          timeRange,
		ReceivedAt:         model.ReceivedAt.UTC(),
	})
	if err != nil {
		return RawVenuePayload{}, fmt.Errorf("map raw venue payload row to domain: %w", err)
	}

	return payload, nil
}

func rawPayloadMetadataFromModel(model rawVenuePayloadModel) (RawPayloadMetadata, error) {
	payload, err := rawVenuePayloadModelToDomain(model)
	if err != nil {
		return RawPayloadMetadata{}, err
	}

	return rawPayloadMetadataFromDomain(payload), nil
}

func normalizationRunToModel(run NormalizationRun) normalizationRunModel {
	return normalizationRunModel{
		ID:                   run.ID,
		Status:               run.Status.String(),
		StartedAt:            run.StartedAt.UTC(),
		CompletedAt:          run.CompletedAt.UTC(),
		RecordKind:           run.RecordKind.String(),
		SourceRecordCount:    run.SourceRecordCount,
		CanonicalRecordCount: run.CanonicalRecordCount,
		ErrorSummary:         run.ErrorSummary,
	}
}

func normalizationRunModelToDomain(
	model normalizationRunModel,
	rawPayloadIDs []string,
) (NormalizationRun, error) {
	run, err := NewNormalizationRun(NormalizationRunParams{
		ID:                   model.ID,
		RawPayloadIDs:        rawPayloadIDs,
		Status:               NormalizationRunStatus(model.Status),
		StartedAt:            model.StartedAt.UTC(),
		CompletedAt:          model.CompletedAt.UTC(),
		RecordKind:           LineageRecordKind(model.RecordKind),
		SourceRecordCount:    model.SourceRecordCount,
		CanonicalRecordCount: model.CanonicalRecordCount,
		ErrorSummary:         model.ErrorSummary,
	})
	if err != nil {
		return NormalizationRun{}, fmt.Errorf("map normalization run row to domain: %w", err)
	}

	return run, nil
}

func dataBatchToModel(batch DataBatch) dataBatchModel {
	model := dataBatchModel{
		ID:                 batch.ID,
		NormalizationRunID: batch.NormalizationRunID,
		Venue:              batch.Venue.String(),
		RecordKind:         batch.RecordKind.String(),
		StartAt:            batch.TimeRange.Start.UTC(),
		EndAt:              batch.TimeRange.End.UTC(),
		Quality:            batch.Quality.String(),
		RecordCount:        batch.RecordCount,
		Summary:            batch.Summary,
	}
	if batch.Instrument != nil {
		model.InstrumentSymbol = batch.Instrument.Symbol.String()
		model.InstrumentAssetCls = batch.Instrument.AssetClass.String()
	}

	return model
}

func dataBatchModelToDomain(model dataBatchModel) (DataBatch, error) {
	timeRange, err := domain.NewTimeRange(model.StartAt.UTC(), model.EndAt.UTC())
	if err != nil {
		return DataBatch{}, fmt.Errorf("map data batch time range: %w", err)
	}

	var instrument *BatchInstrumentRef
	if model.InstrumentSymbol != "" || model.InstrumentAssetCls != "" {
		instrument = &BatchInstrumentRef{
			Symbol:     domain.Symbol(model.InstrumentSymbol),
			AssetClass: domain.AssetClass(model.InstrumentAssetCls),
		}
	}

	batch, err := NewDataBatch(DataBatchParams{
		ID:                 model.ID,
		NormalizationRunID: model.NormalizationRunID,
		Venue:              domain.Venue(model.Venue),
		Instrument:         instrument,
		RecordKind:         LineageRecordKind(model.RecordKind),
		TimeRange:          timeRange,
		Quality:            domain.DataQuality(model.Quality),
		RecordCount:        model.RecordCount,
		Summary:            model.Summary,
	})
	if err != nil {
		return DataBatch{}, fmt.Errorf("map data batch row to domain: %w", err)
	}

	return batch, nil
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

func (s *DatabaseStore) findIngestionRunModel(ctx context.Context, id string) (ingestionRunModel, error) {
	return findIngestionRunModel(s.db.WithContext(ctx), id)
}

func findIngestionRunModel(db *gorm.DB, id string) (ingestionRunModel, error) {
	var model ingestionRunModel
	if err := db.Where("id = ?", id).First(&model).Error; err != nil {
		return ingestionRunModel{}, fmt.Errorf("lookup ingestion run row: %w", err)
	}

	return model, nil
}

func findNormalizationRunModel(db *gorm.DB, id string) (normalizationRunModel, error) {
	var model normalizationRunModel
	if err := db.Where("id = ?", id).First(&model).Error; err != nil {
		return normalizationRunModel{}, fmt.Errorf("lookup normalization run row: %w", err)
	}

	return model, nil
}

func findDataBatchModel(db *gorm.DB, id string) (dataBatchModel, error) {
	var model dataBatchModel
	if err := db.Where("id = ?", id).First(&model).Error; err != nil {
		return dataBatchModel{}, fmt.Errorf("lookup data batch row: %w", err)
	}

	return model, nil
}

func ensureIngestionRunExists(tx *gorm.DB, ingestionRunID string) error {
	if strings.TrimSpace(ingestionRunID) == "" {
		return nil
	}

	var count int64
	if err := tx.Model(&ingestionRunModel{}).Where("id = ?", ingestionRunID).Count(&count).Error; err != nil {
		return fmt.Errorf("lookup ingestion run row: %w", err)
	}
	if count == 0 {
		return ErrLineageParentNotFound
	}

	return nil
}

func ensureRawPayloadsExist(tx *gorm.DB, rawPayloadIDs []string) error {
	uniqueIDs := uniqueStrings(rawPayloadIDs)
	var count int64
	if err := tx.Model(&rawVenuePayloadModel{}).Where("id IN ?", uniqueIDs).Count(&count).Error; err != nil {
		return fmt.Errorf("lookup raw venue payload rows: %w", err)
	}
	if count != int64(len(uniqueIDs)) {
		return ErrLineageParentNotFound
	}

	return nil
}

func ensureNormalizationRunExists(tx *gorm.DB, normalizationRunID string) error {
	var count int64
	if err := tx.Model(&normalizationRunModel{}).Where("id = ?", normalizationRunID).Count(&count).Error; err != nil {
		return fmt.Errorf("lookup normalization run row: %w", err)
	}
	if count == 0 {
		return ErrLineageParentNotFound
	}

	return nil
}

func ensureDataBatchSupportsRecordKind(
	tx *gorm.DB,
	batchID string,
	expectedKind LineageRecordKind,
) error {
	batchRow, err := findDataBatchModel(tx, batchID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLineageParentNotFound
		}
		return err
	}

	if batchRow.RecordKind != expectedKind.String() {
		return validationError("data batch record kind must match canonical record kind")
	}

	return nil
}

func listInstrumentModelsByID(db *gorm.DB, instrumentIDs []uint) (map[uint]instrumentModel, error) {
	uniqueIDs := uniqueUintValues(instrumentIDs)
	if len(uniqueIDs) == 0 {
		return map[uint]instrumentModel{}, nil
	}

	var rows []instrumentModel
	if err := db.Where("id IN ?", uniqueIDs).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query instrument rows: %w", err)
	}

	byID := make(map[uint]instrumentModel, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	return byID, nil
}

func candleInstrumentIDs(rows []candleModel) []uint {
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.InstrumentID)
	}

	return ids
}

func tradeInstrumentIDs(rows []tradeModel) []uint {
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.InstrumentID)
	}

	return ids
}

func mapReplayCandles(db *gorm.DB, rows []candleModel) ([]ReplayCandle, error) {
	instrumentsByID, err := listInstrumentModelsByID(db, candleInstrumentIDs(rows))
	if err != nil {
		return nil, err
	}

	candles := make([]ReplayCandle, 0, len(rows))
	for _, row := range rows {
		instrumentRow, ok := instrumentsByID[row.InstrumentID]
		if !ok {
			return nil, fmt.Errorf("lookup instrument row: %w", ErrInstrumentNotFound)
		}
		candle, mapErr := candleModelToDomain(row, instrumentRow)
		if mapErr != nil {
			return nil, mapErr
		}
		candles = append(candles, ReplayCandle{Identity: uint64(row.ID), Candle: candle})
	}

	return candles, nil
}

func mapReplayTrades(db *gorm.DB, rows []tradeModel) ([]ReplayTrade, error) {
	instrumentsByID, err := listInstrumentModelsByID(db, tradeInstrumentIDs(rows))
	if err != nil {
		return nil, err
	}

	trades := make([]ReplayTrade, 0, len(rows))
	for _, row := range rows {
		instrumentRow, ok := instrumentsByID[row.InstrumentID]
		if !ok {
			return nil, fmt.Errorf("lookup instrument row: %w", ErrInstrumentNotFound)
		}
		trade, mapErr := tradeModelToDomain(row, instrumentRow)
		if mapErr != nil {
			return nil, mapErr
		}
		trades = append(trades, ReplayTrade{Identity: uint64(row.ID), Trade: trade})
	}

	return trades, nil
}

func listRawPayloadIDsByLinkedColumn(
	db *gorm.DB,
	model any,
	column string,
	value any,
) ([]string, error) {
	type rawPayloadIDRow struct {
		RawPayloadID string `gorm:"column:raw_payload_id"`
	}

	var rows []rawPayloadIDRow
	if err := db.Model(model).
		Where(column+" = ?", value).
		Order("raw_payload_id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query raw payload links: %w", err)
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.RawPayloadID)
	}

	return ids, nil
}

func replaceNormalizationRunRawPayloadLinks(
	tx *gorm.DB,
	normalizationRunID string,
	rawPayloadIDs []string,
) error {
	if err := tx.Where(
		"normalization_run_id = ?",
		normalizationRunID,
	).Delete(&normalizationRunRawPayloadLinkModel{}).Error; err != nil {
		return fmt.Errorf("delete normalization raw payload links: %w", err)
	}

	links := make([]normalizationRunRawPayloadLinkModel, 0, len(rawPayloadIDs))
	for _, rawPayloadID := range uniqueStrings(rawPayloadIDs) {
		links = append(links, normalizationRunRawPayloadLinkModel{
			NormalizationRunID: normalizationRunID,
			RawPayloadID:       rawPayloadID,
		})
	}
	if len(links) == 0 {
		return nil
	}

	if err := tx.Create(&links).Error; err != nil {
		return fmt.Errorf("insert normalization raw payload links: %w", err)
	}

	return nil
}

func queryRawPayloadMetadataRows(
	db *gorm.DB,
	query RawPayloadMetadataListQuery,
	limit int,
) ([]rawVenuePayloadModel, error) {
	statement := db.Model(&rawVenuePayloadModel{}).Where("venue = ?", query.Venue.String())
	if query.Instrument != nil {
		statement = statement.
			Where("instrument_symbol = ?", query.Instrument.Symbol.String()).
			Where("instrument_asset_class = ?", query.Instrument.AssetClass.String())
	}
	if query.Timeframe != "" {
		statement = statement.Where("timeframe = ?", query.Timeframe.String())
	}
	if query.TimeRange != nil {
		statement = statement.
			Where("start_at >= ?", query.TimeRange.Start.UTC()).
			Where("end_at <= ?", query.TimeRange.End.UTC())
	}
	if query.IngestionRunID != "" {
		statement = statement.Where("ingestion_run_id = ?", query.IngestionRunID)
	}
	if query.EntityHint != "" {
		statement = statement.Where("entity_hint = ?", query.EntityHint)
	}
	if query.Endpoint != "" {
		statement = statement.Where("endpoint = ?", query.Endpoint)
	}
	if query.RequestType != "" {
		statement = statement.Where("request_type = ?", query.RequestType)
	}
	if !query.cursor.ReceivedAt.IsZero() {
		statement = statement.Where(
			"(received_at > ?) OR (received_at = ? AND id > ?)",
			query.cursor.ReceivedAt.UTC(),
			query.cursor.ReceivedAt.UTC(),
			query.cursor.ID,
		)
	}

	var rows []rawVenuePayloadModel
	if err := statement.Order("received_at ASC").Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query raw payload rows: %w", err)
	}

	return rows, nil
}

func listNormalizationRunRawPayloadIDs(tx *gorm.DB, normalizationRunID string) ([]string, error) {
	var links []normalizationRunRawPayloadLinkModel
	if err := tx.Where("normalization_run_id = ?", normalizationRunID).
		Order("raw_payload_id ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("query normalization raw payload links: %w", err)
	}

	ids := make([]string, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.RawPayloadID)
	}

	return ids, nil
}

func listRawVenuePayloadModelsByNormalizationRun(
	tx *gorm.DB,
	normalizationRunID string,
) ([]rawVenuePayloadModel, error) {
	linkTable := tx.NamingStrategy.TableName("normalization_run_raw_payload_links")
	rawPayloadTable := tx.NamingStrategy.TableName("raw_venue_payloads")

	var rows []rawVenuePayloadModel
	if err := tx.Table(rawPayloadTable+" AS raw").
		Joins("JOIN "+linkTable+" AS links ON links.raw_payload_id = raw.id").
		Where("links.normalization_run_id = ?", normalizationRunID).
		Order("raw.received_at ASC, raw.id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query raw venue payload rows: %w", err)
	}

	return rows, nil
}

func listIngestionRunsByID(
	tx *gorm.DB,
	rawPayloadRows []rawVenuePayloadModel,
) (map[string]ingestionRunModel, error) {
	ingestionRunIDs := make([]string, 0, len(rawPayloadRows))
	seen := make(map[string]struct{}, len(rawPayloadRows))
	for _, row := range rawPayloadRows {
		if strings.TrimSpace(row.IngestionRunID) == "" {
			continue
		}
		if _, ok := seen[row.IngestionRunID]; ok {
			continue
		}
		seen[row.IngestionRunID] = struct{}{}
		ingestionRunIDs = append(ingestionRunIDs, row.IngestionRunID)
	}
	if len(ingestionRunIDs) == 0 {
		return map[string]ingestionRunModel{}, nil
	}

	var rows []ingestionRunModel
	if err := tx.Where("id IN ?", ingestionRunIDs).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query ingestion run rows: %w", err)
	}

	byID := make(map[string]ingestionRunModel, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	return byID, nil
}

func loadDataBatchAudit(db *gorm.DB, batchID string) (DataBatchAudit, error) {
	batchRow, err := findDataBatchModel(db, batchID)
	if err != nil {
		return DataBatchAudit{}, err
	}

	normalizationRow, err := findNormalizationRunModel(db, batchRow.NormalizationRunID)
	if err != nil {
		return DataBatchAudit{}, err
	}

	rawPayloadRows, err := listRawVenuePayloadModelsByNormalizationRun(db, normalizationRow.ID)
	if err != nil {
		return DataBatchAudit{}, err
	}

	rawPayloads, rawPayloadIDs, err := buildRawVenuePayloadAudits(db, rawPayloadRows)
	if err != nil {
		return DataBatchAudit{}, err
	}

	batch, err := dataBatchModelToDomain(batchRow)
	if err != nil {
		return DataBatchAudit{}, err
	}
	normalizationRun, err := normalizationRunModelToDomain(normalizationRow, rawPayloadIDs)
	if err != nil {
		return DataBatchAudit{}, err
	}

	return DataBatchAudit{
		Batch:            batch,
		NormalizationRun: normalizationRun,
		RawPayloads:      rawPayloads,
	}, nil
}

func buildRawVenuePayloadAudits(
	db *gorm.DB,
	rawPayloadRows []rawVenuePayloadModel,
) ([]RawVenuePayloadAudit, []string, error) {
	ingestionRunsByID, err := listIngestionRunsByID(db, rawPayloadRows)
	if err != nil {
		return nil, nil, err
	}

	rawPayloads := make([]RawVenuePayloadAudit, 0, len(rawPayloadRows))
	rawPayloadIDs := make([]string, 0, len(rawPayloadRows))
	for _, payloadRow := range rawPayloadRows {
		payload, payloadErr := rawVenuePayloadModelToDomain(payloadRow)
		if payloadErr != nil {
			return nil, nil, payloadErr
		}
		var ingestionRun *IngestionRun
		if strings.TrimSpace(payloadRow.IngestionRunID) != "" {
			runRow, ok := ingestionRunsByID[payloadRow.IngestionRunID]
			if !ok {
				return nil, nil, fmt.Errorf("lookup ingestion run row: %w", ErrLineageParentNotFound)
			}
			mappedRun, runErr := ingestionRunModelToDomain(runRow)
			if runErr != nil {
				return nil, nil, runErr
			}
			ingestionRun = &mappedRun
		}
		rawPayloadIDs = append(rawPayloadIDs, payload.ID)
		rawPayloads = append(rawPayloads, RawVenuePayloadAudit{
			Payload:      payload,
			IngestionRun: ingestionRun,
		})
	}

	return rawPayloads, rawPayloadIDs, nil
}

func marshalLineageMetadata(metadata map[string]string) (string, error) {
	sanitized := sanitizeLineageMetadata(metadata)
	if len(sanitized) == 0 {
		return "", nil
	}

	payload, err := json.Marshal(sanitized)
	if err != nil {
		return "", fmt.Errorf("marshal lineage metadata: %w", err)
	}

	return string(payload), nil
}

func unmarshalLineageMetadata(value string) (map[string]string, error) {
	if strings.TrimSpace(value) == "" {
		return map[string]string{}, nil
	}

	var metadata map[string]string
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal lineage metadata: %w", err)
	}

	return canonicalizeMetadataMap(metadata), nil
}

func sanitizeLineageMetadata(metadata map[string]string) map[string]string {
	canonical := canonicalizeMetadataMap(metadata)
	if len(canonical) == 0 {
		return nil
	}

	sanitized := make(map[string]string, len(canonical))
	for key, value := range canonical {
		if isSecretBearingMetadataKey(key) {
			continue
		}
		sanitized[key] = value
	}
	if len(sanitized) == 0 {
		return nil
	}

	return sanitized
}

func isSecretBearingMetadataKey(key string) bool {
	canonicalKey := strings.ToLower(strings.TrimSpace(key))
	for _, needle := range []string{
		"authorization",
		"cookie",
		"set-cookie",
		"api-key",
		"api_key",
		"apikey",
		"signature",
		"secret",
		"credential",
		"password",
		"token",
	} {
		if strings.Contains(canonicalKey, needle) {
			return true
		}
	}

	return false
}

func uniqueStrings(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}

	return unique
}

func uniqueUintValues(values []uint) []uint {
	unique := make([]uint, 0, len(values))
	seen := make(map[uint]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}

	return unique
}
