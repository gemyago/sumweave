package audit

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
	decisionTraceIDColumn = "trace_id"
	orderIntentIDColumn   = "intent_id"
)

type decisionTraceModel struct {
	TraceID              string    `gorm:"column:trace_id;size:255;not null;primaryKey;uniqueIndex:idx_decision_traces_trace_id"`
	Mode                 string    `gorm:"column:mode;size:32;not null;index:idx_decision_traces_mode_time_id,priority:1"`
	DecisionTime         time.Time `gorm:"column:decision_time;not null;index:idx_decision_traces_mode_time_id,priority:2"`
	StrategyID           string    `gorm:"column:strategy_id;size:255;not null;index:idx_decision_traces_strategy_time_id,priority:1"`
	StrategyVersion      string    `gorm:"column:strategy_version;size:255;not null"`
	StrategyArtifactHash string    `gorm:"column:strategy_artifact_hash;size:255;not null"`
	Venue                string    `gorm:"column:venue;size:255;not null;index:idx_decision_traces_instrument_time_id,priority:1"`
	Symbol               string    `gorm:"column:symbol;size:255;not null;index:idx_decision_traces_instrument_time_id,priority:2"`
	AssetClass           string    `gorm:"column:asset_class;size:64;not null;index:idx_decision_traces_instrument_time_id,priority:3"`
	Timeframe            string    `gorm:"column:timeframe;size:32;not null"`
	DatasetReference     string    `gorm:"column:dataset_reference;size:255"`
	RunReference         string    `gorm:"column:run_reference;size:255"`
	InputStartAt         time.Time `gorm:"column:input_start_at;not null"`
	InputEndAt           time.Time `gorm:"column:input_end_at;not null"`
	AnalyticsReference   string    `gorm:"column:analytics_reference;size:255"`
	DataQuality          string    `gorm:"column:data_quality;size:32;not null"`
	EvaluatorName        string    `gorm:"column:evaluator_name;size:255;not null"`
	EvaluatorVersion     string    `gorm:"column:evaluator_version;size:255;not null"`
	Result               string    `gorm:"column:result;size:64;not null"`
	ReasonCodesJSON      string    `gorm:"column:reason_codes_json;size:1024;not null"`
	MetadataJSON         string    `gorm:"column:metadata_json;size:4096"`
	CreatedAt            time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt            time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (decisionTraceModel) TableName(namer schema.Namer) string {
	return namer.TableName("decision_traces")
}

type orderIntentModel struct {
	IntentID                 string    `gorm:"column:intent_id;size:255;not null;primaryKey;uniqueIndex:idx_order_intents_intent_id"`
	TraceID                  string    `gorm:"column:trace_id;size:255;not null;index:idx_order_intents_trace_id"`
	StrategyID               string    `gorm:"column:strategy_id;size:255;not null;index:idx_order_intents_strategy_time_id,priority:1"`
	StrategyVersion          string    `gorm:"column:strategy_version;size:255;not null"`
	StrategyArtifactHash     string    `gorm:"column:strategy_artifact_hash;size:255;not null"`
	Mode                     string    `gorm:"column:mode;size:32;not null;index:idx_order_intents_mode_time_id,priority:1"`
	Venue                    string    `gorm:"column:venue;size:255;not null;index:idx_order_intents_instrument_time_id,priority:1"`
	Symbol                   string    `gorm:"column:symbol;size:255;not null;index:idx_order_intents_instrument_time_id,priority:2"`
	AssetClass               string    `gorm:"column:asset_class;size:64;not null;index:idx_order_intents_instrument_time_id,priority:3"`
	Timeframe                string    `gorm:"column:timeframe;size:32;not null"`
	ActionKind               string    `gorm:"column:action_kind;size:32;not null"`
	OrderType                string    `gorm:"column:order_type;size:32;not null"`
	RequestedQuantity        float64   `gorm:"column:requested_quantity;not null"`
	RequestedNotional        float64   `gorm:"column:requested_notional;not null"`
	RequestedLimitPrice      *float64  `gorm:"column:requested_limit_price"`
	ReduceOnly               bool      `gorm:"column:reduce_only;not null"`
	SourceReasonCode         string    `gorm:"column:source_reason_code;size:64;not null"`
	CandidateActionReference string    `gorm:"column:candidate_action_reference;size:255"`
	CreatedTime              time.Time `gorm:"column:created_time;not null;index:idx_order_intents_mode_time_id,priority:2;index:idx_order_intents_strategy_time_id,priority:2;index:idx_order_intents_instrument_time_id,priority:4"`
	Status                   string    `gorm:"column:status;size:64;not null"`
	MetadataJSON             string    `gorm:"column:metadata_json;size:4096"`
	CreatedAt                time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt                time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (orderIntentModel) TableName(namer schema.Namer) string {
	return namer.TableName("order_intents")
}

// DatabaseStoreOpts configures audit database concerns used by app wiring.
type DatabaseStoreOpts struct {
	TablePrefix string
}

// DatabaseStore persists durable trace and order intent records with GORM.
type DatabaseStore struct {
	db *gorm.DB
}

// NewDatabaseStore opens a database-backed audit store.
func NewDatabaseStore(dsn string, opts DatabaseStoreOpts) (*DatabaseStore, error) {
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
	if err = gormsignalfoundry.ApplySQLiteConnectionDefaults(db, dsn); err != nil {
		return nil, err
	}

	return &DatabaseStore{db: db}, nil
}

// AutoMigrate creates or updates the durable audit relational schema.
func (s *DatabaseStore) AutoMigrate() error {
	return s.db.AutoMigrate(&decisionTraceModel{}, &orderIntentModel{})
}

// CreateTrace idempotently persists a decision trace by stable id.
func (s *DatabaseStore) CreateTrace(
	ctx context.Context,
	trace domain.DecisionTrace,
) (domain.DecisionTrace, error) {
	model, err := decisionTraceToModel(trace)
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: decisionTraceIDColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return domain.DecisionTrace{}, fmt.Errorf("create decision trace: %w", err)
	}

	persisted, err := s.findDecisionTraceModel(ctx, string(trace.TraceID))
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	return decisionTraceModelToDomain(persisted)
}

// GetTrace reads a persisted decision trace by stable id.
func (s *DatabaseStore) GetTrace(
	ctx context.Context,
	traceID string,
) (*domain.DecisionTrace, error) {
	model, err := s.findDecisionTraceModel(ctx, traceID)
	if err != nil {
		if errors.Is(err, ErrTraceNotFound) {
			return nil, err
		}
		return nil, err
	}

	trace, err := decisionTraceModelToDomain(model)
	if err != nil {
		return nil, err
	}

	return &trace, nil
}

// QueryTraces returns deterministic filtered trace rows.
func (s *DatabaseStore) QueryTraces(
	ctx context.Context,
	query TraceQuery,
) ([]domain.DecisionTrace, error) {
	db := applyAuditListQuery(
		s.db.WithContext(ctx).Model(&decisionTraceModel{}),
		query.StrategyID,
		query.Instrument,
		query.Mode,
		query.TimeRange,
		"decision_time",
	)

	var models []decisionTraceModel
	if err := db.Order("decision_time ASC").Order("trace_id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query decision traces: %w", err)
	}

	traces := make([]domain.DecisionTrace, 0, len(models))
	for _, model := range models {
		trace, err := decisionTraceModelToDomain(model)
		if err != nil {
			return nil, err
		}
		traces = append(traces, trace)
	}

	return traces, nil
}

// UpdateTraceMetadata persists stable decision trace metadata updates.
func (s *DatabaseStore) UpdateTraceMetadata(
	ctx context.Context,
	traceID string,
	metadata map[string]string,
) (domain.DecisionTrace, error) {
	metadataJSON, err := marshalAuditMetadata(metadata)
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	var model decisionTraceModel
	if err = updateAuditMetadataModel(
		s.db.WithContext(ctx),
		&model,
		decisionTraceIDColumn,
		traceID,
		metadataJSON,
		ErrTraceNotFound,
		"decision trace",
	); err != nil {
		return domain.DecisionTrace{}, err
	}

	return decisionTraceModelToDomain(model)
}

// CreateOrderIntent idempotently persists an order intent by stable id.
func (s *DatabaseStore) CreateOrderIntent(
	ctx context.Context,
	intent domain.OrderIntent,
) (domain.OrderIntent, error) {
	model, err := orderIntentToModel(intent)
	if err != nil {
		return domain.OrderIntent{}, err
	}

	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: orderIntentIDColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return domain.OrderIntent{}, fmt.Errorf("create order intent: %w", err)
	}

	persisted, err := s.findOrderIntentModel(ctx, string(intent.IntentID))
	if err != nil {
		return domain.OrderIntent{}, err
	}

	return orderIntentModelToDomain(persisted)
}

// GetOrderIntent reads a persisted order intent by stable id.
func (s *DatabaseStore) GetOrderIntent(
	ctx context.Context,
	intentID string,
) (*domain.OrderIntent, error) {
	model, err := s.findOrderIntentModel(ctx, intentID)
	if err != nil {
		if errors.Is(err, ErrOrderIntentNotFound) {
			return nil, err
		}
		return nil, err
	}

	intent, err := orderIntentModelToDomain(model)
	if err != nil {
		return nil, err
	}

	return &intent, nil
}

// QueryOrderIntents returns deterministic filtered order intent rows.
func (s *DatabaseStore) QueryOrderIntents(
	ctx context.Context,
	query OrderIntentQuery,
) ([]domain.OrderIntent, error) {
	db := applyAuditListQuery(
		s.db.WithContext(ctx).Model(&orderIntentModel{}),
		query.StrategyID,
		query.Instrument,
		query.Mode,
		query.TimeRange,
		"created_time",
	)

	var models []orderIntentModel
	if err := db.Order("created_time ASC").Order("intent_id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query order intents: %w", err)
	}

	intents := make([]domain.OrderIntent, 0, len(models))
	for _, model := range models {
		intent, err := orderIntentModelToDomain(model)
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent)
	}

	return intents, nil
}

// UpdateOrderIntentStatus persists a new stable order intent status.
func (s *DatabaseStore) UpdateOrderIntentStatus(
	ctx context.Context,
	intentID string,
	status domain.OrderIntentStatus,
) (domain.OrderIntent, error) {
	var model orderIntentModel
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&model, orderIntentIDColumn+" = ?", strings.TrimSpace(intentID)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", ErrOrderIntentNotFound, intentID)
			}
			return fmt.Errorf("get order intent: %w", err)
		}

		model.Status = status.String()
		if err := tx.Model(&orderIntentModel{}).
			Where(orderIntentIDColumn+" = ?", model.IntentID).
			Update("status", model.Status).Error; err != nil {
			return fmt.Errorf("update order intent status: %w", err)
		}

		if err := tx.First(&model, orderIntentIDColumn+" = ?", model.IntentID).Error; err != nil {
			return fmt.Errorf("refresh order intent: %w", err)
		}

		return nil
	}); err != nil {
		return domain.OrderIntent{}, err
	}

	return orderIntentModelToDomain(model)
}

// UpdateOrderIntentMetadata persists stable order intent metadata updates.
func (s *DatabaseStore) UpdateOrderIntentMetadata(
	ctx context.Context,
	intentID string,
	metadata map[string]string,
) (domain.OrderIntent, error) {
	metadataJSON, err := marshalAuditMetadata(metadata)
	if err != nil {
		return domain.OrderIntent{}, err
	}

	var model orderIntentModel
	if err = updateAuditMetadataModel(
		s.db.WithContext(ctx),
		&model,
		orderIntentIDColumn,
		intentID,
		metadataJSON,
		ErrOrderIntentNotFound,
		"order intent",
	); err != nil {
		return domain.OrderIntent{}, err
	}

	return orderIntentModelToDomain(model)
}

func updateAuditMetadataModel[T any](
	db *gorm.DB,
	model *T,
	column string,
	entityID string,
	metadataJSON string,
	notFoundErr error,
	entityName string,
) error {
	trimmedID := strings.TrimSpace(entityID)

	return db.Transaction(func(tx *gorm.DB) error {
		fetchErr := tx.First(model, column+" = ?", trimmedID).Error
		if fetchErr != nil {
			if errors.Is(fetchErr, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", notFoundErr, entityID)
			}
			return fmt.Errorf("get %s: %w", entityName, fetchErr)
		}

		updateErr := tx.Model(model).
			Where(column+" = ?", trimmedID).
			Update("metadata_json", metadataJSON).Error
		if updateErr != nil {
			return fmt.Errorf("update %s metadata: %w", entityName, updateErr)
		}

		refreshErr := tx.First(model, column+" = ?", trimmedID).Error
		if refreshErr != nil {
			return fmt.Errorf("refresh %s: %w", entityName, refreshErr)
		}

		return nil
	})
}

func (s *DatabaseStore) findDecisionTraceModel(
	ctx context.Context,
	traceID string,
) (decisionTraceModel, error) {
	var model decisionTraceModel
	if err := s.db.WithContext(ctx).
		First(&model, decisionTraceIDColumn+" = ?", strings.TrimSpace(traceID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return decisionTraceModel{}, fmt.Errorf("%w: %s", ErrTraceNotFound, traceID)
		}
		return decisionTraceModel{}, fmt.Errorf("get decision trace: %w", err)
	}

	return model, nil
}

func (s *DatabaseStore) findOrderIntentModel(
	ctx context.Context,
	intentID string,
) (orderIntentModel, error) {
	var model orderIntentModel
	if err := s.db.WithContext(ctx).
		First(&model, orderIntentIDColumn+" = ?", strings.TrimSpace(intentID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return orderIntentModel{}, fmt.Errorf("%w: %s", ErrOrderIntentNotFound, intentID)
		}
		return orderIntentModel{}, fmt.Errorf("get order intent: %w", err)
	}

	return model, nil
}

func decisionTraceToModel(trace domain.DecisionTrace) (decisionTraceModel, error) {
	reasonCodesJSON, err := json.Marshal(trace.ReasonCodes)
	if err != nil {
		return decisionTraceModel{}, fmt.Errorf("marshal trace reason codes: %w", err)
	}
	metadataJSON, err := marshalAuditMetadata(trace.Metadata)
	if err != nil {
		return decisionTraceModel{}, err
	}

	return decisionTraceModel{
		TraceID:              string(trace.TraceID),
		Mode:                 trace.Mode.String(),
		DecisionTime:         trace.DecisionTime.Time().UTC(),
		StrategyID:           trace.StrategyID,
		StrategyVersion:      trace.StrategyVersion,
		StrategyArtifactHash: trace.StrategyArtifactHash,
		Venue:                trace.Instrument.Venue.String(),
		Symbol:               trace.Instrument.Symbol.String(),
		AssetClass:           trace.Instrument.AssetClass.String(),
		Timeframe:            trace.Timeframe.String(),
		DatasetReference:     trace.DatasetReference,
		RunReference:         trace.RunReference,
		InputStartAt:         trace.InputRange.Start.UTC(),
		InputEndAt:           trace.InputRange.End.UTC(),
		AnalyticsReference:   trace.AnalyticsReference,
		DataQuality:          trace.DataQuality.String(),
		EvaluatorName:        trace.EvaluatorName,
		EvaluatorVersion:     trace.EvaluatorVersion,
		Result:               trace.Result.String(),
		ReasonCodesJSON:      string(reasonCodesJSON),
		MetadataJSON:         metadataJSON,
	}, nil
}

func decisionTraceModelToDomain(model decisionTraceModel) (domain.DecisionTrace, error) {
	var reasonCodes []string
	if err := json.Unmarshal([]byte(model.ReasonCodesJSON), &reasonCodes); err != nil {
		return domain.DecisionTrace{}, fmt.Errorf("unmarshal trace reason codes: %w", err)
	}
	metadata, err := unmarshalAuditMetadata(model.MetadataJSON)
	if err != nil {
		return domain.DecisionTrace{}, err
	}

	return domain.NewDecisionTrace(domain.DecisionTraceParams{
		TraceID:              model.TraceID,
		Mode:                 domain.DecisionMode(model.Mode),
		DecisionTime:         model.DecisionTime.UTC(),
		StrategyID:           model.StrategyID,
		StrategyVersion:      model.StrategyVersion,
		StrategyArtifactHash: model.StrategyArtifactHash,
		Instrument: domain.Instrument{
			Venue:      domain.Venue(model.Venue),
			Symbol:     domain.Symbol(model.Symbol),
			AssetClass: domain.AssetClass(model.AssetClass),
			Active:     true,
		},
		Timeframe:          domain.Timeframe(model.Timeframe),
		DatasetReference:   model.DatasetReference,
		RunReference:       model.RunReference,
		InputRange:         domain.TimeRange{Start: model.InputStartAt.UTC(), End: model.InputEndAt.UTC()},
		AnalyticsReference: model.AnalyticsReference,
		DataQuality:        domain.DataQuality(model.DataQuality),
		EvaluatorName:      model.EvaluatorName,
		EvaluatorVersion:   model.EvaluatorVersion,
		Result:             domain.DecisionTraceResult(model.Result),
		ReasonCodes:        reasonCodes,
		Metadata:           metadata,
	})
}

func orderIntentToModel(intent domain.OrderIntent) (orderIntentModel, error) {
	metadataJSON, err := marshalAuditMetadata(intent.Metadata)
	if err != nil {
		return orderIntentModel{}, err
	}

	return orderIntentModel{
		IntentID:                 string(intent.IntentID),
		TraceID:                  string(intent.TraceID),
		StrategyID:               intent.StrategyID,
		StrategyVersion:          intent.StrategyVersion,
		StrategyArtifactHash:     intent.StrategyArtifactHash,
		Mode:                     intent.Mode.String(),
		Venue:                    intent.Instrument.Venue.String(),
		Symbol:                   intent.Instrument.Symbol.String(),
		AssetClass:               intent.Instrument.AssetClass.String(),
		Timeframe:                intent.Timeframe.String(),
		ActionKind:               intent.ActionKind.String(),
		OrderType:                intent.OrderType.String(),
		RequestedQuantity:        intent.RequestedQuantity,
		RequestedNotional:        intent.RequestedNotional,
		RequestedLimitPrice:      intent.RequestedLimitPrice,
		ReduceOnly:               intent.ReduceOnly,
		SourceReasonCode:         intent.SourceReasonCode,
		CandidateActionReference: intent.CandidateActionReference,
		CreatedTime:              intent.CreatedTime.Time().UTC(),
		Status:                   intent.Status.String(),
		MetadataJSON:             metadataJSON,
	}, nil
}

func orderIntentModelToDomain(model orderIntentModel) (domain.OrderIntent, error) {
	metadata, err := unmarshalAuditMetadata(model.MetadataJSON)
	if err != nil {
		return domain.OrderIntent{}, err
	}

	return domain.NewOrderIntent(domain.OrderIntentParams{
		IntentID:             model.IntentID,
		TraceID:              model.TraceID,
		StrategyID:           model.StrategyID,
		StrategyVersion:      model.StrategyVersion,
		StrategyArtifactHash: model.StrategyArtifactHash,
		Mode:                 domain.DecisionMode(model.Mode),
		Instrument: domain.Instrument{
			Venue:      domain.Venue(model.Venue),
			Symbol:     domain.Symbol(model.Symbol),
			AssetClass: domain.AssetClass(model.AssetClass),
			Active:     true,
		},
		Timeframe:                domain.Timeframe(model.Timeframe),
		ActionKind:               domain.CandidateActionKind(model.ActionKind),
		OrderType:                domain.OrderType(model.OrderType),
		RequestedQuantity:        model.RequestedQuantity,
		RequestedNotional:        model.RequestedNotional,
		RequestedLimitPrice:      model.RequestedLimitPrice,
		ReduceOnly:               model.ReduceOnly,
		SourceReasonCode:         model.SourceReasonCode,
		CandidateActionReference: model.CandidateActionReference,
		CreatedTime:              model.CreatedTime.UTC(),
		Status:                   domain.OrderIntentStatus(model.Status),
		Metadata:                 metadata,
	})
}

func marshalAuditMetadata(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal audit metadata: %w", err)
	}

	return string(encoded), nil
}

func unmarshalAuditMetadata(value string) (map[string]string, error) {
	if strings.TrimSpace(value) == "" {
		return map[string]string{}, nil
	}

	var metadata map[string]string
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal audit metadata: %w", err)
	}

	return metadata, nil
}

func applyAuditListQuery(
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
