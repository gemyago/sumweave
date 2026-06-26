package execution

import (
	"context"
	"encoding/json"
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
	executionCommandIDColumn = "command_id"
	executionOrderIDColumn   = "order_id"
	executionFillIDColumn    = "fill_id"
)

type executionCommandModel struct {
	CommandID                 string    `gorm:"column:command_id;size:255;not null;primaryKey;uniqueIndex:idx_execution_commands_command_id"`
	TraceID                   string    `gorm:"column:trace_id;size:255"`
	IntentID                  string    `gorm:"column:intent_id;size:255"`
	GovernorDecisionReference string    `gorm:"column:governor_decision_reference;size:255;not null"`
	Mode                      string    `gorm:"column:mode;size:32;not null;index:idx_execution_commands_mode_time_id,priority:1"`
	StrategyID                string    `gorm:"column:strategy_id;size:255;not null"`
	StrategyVersion           string    `gorm:"column:strategy_version;size:255;not null"`
	StrategyArtifactHash      string    `gorm:"column:strategy_artifact_hash;size:255;not null"`
	Venue                     string    `gorm:"column:venue;size:255;not null;index:idx_execution_commands_instrument_time_id,priority:1"`
	Symbol                    string    `gorm:"column:symbol;size:255;not null;index:idx_execution_commands_instrument_time_id,priority:2"`
	AssetClass                string    `gorm:"column:asset_class;size:64;not null;index:idx_execution_commands_instrument_time_id,priority:3"`
	ActionKind                string    `gorm:"column:action_kind;size:32;not null"`
	OrderType                 string    `gorm:"column:order_type;size:32;not null"`
	LimitPrice                *float64  `gorm:"column:limit_price"`
	ReduceOnly                bool      `gorm:"column:reduce_only;not null"`
	ApprovedQuantity          float64   `gorm:"column:approved_quantity;not null"`
	ApprovedNotional          float64   `gorm:"column:approved_notional;not null"`
	ApprovedInstrumentActive  bool      `gorm:"column:approved_instrument_active"`
	ApprovedTimeframe         string    `gorm:"column:approved_timeframe;size:32"`
	ApprovedInputStart        time.Time `gorm:"column:approved_input_start"`
	ApprovedInputEnd          time.Time `gorm:"column:approved_input_end"`
	ApprovedQuality           string    `gorm:"column:approved_quality;size:32"`
	DecisionStatus            string    `gorm:"column:decision_status;size:32;not null"`
	DecisionReason            string    `gorm:"column:decision_reason;size:64;not null"`
	DecisionTime              time.Time `gorm:"column:decision_time;not null"`
	Status                    string    `gorm:"column:status;size:32;not null"`
	EventTime                 time.Time `gorm:"column:event_time;not null;index:idx_execution_commands_mode_time_id,priority:2;index:idx_execution_commands_instrument_time_id,priority:4"`
	CreatedAt                 time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt                 time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (executionCommandModel) TableName(namer schema.Namer) string {
	return namer.TableName("execution_commands")
}

type executionOrderModel struct {
	OrderID              string    `gorm:"column:order_id;size:255;not null;primaryKey;uniqueIndex:idx_execution_orders_order_id"`
	CommandID            string    `gorm:"column:command_id;size:255;not null;index:idx_execution_orders_command_id"`
	Mode                 string    `gorm:"column:mode;size:32;not null;index:idx_execution_orders_mode_time_id,priority:1"`
	StrategyID           string    `gorm:"column:strategy_id;size:255;not null"`
	StrategyVersion      string    `gorm:"column:strategy_version;size:255;not null"`
	StrategyArtifactHash string    `gorm:"column:strategy_artifact_hash;size:255;not null"`
	Venue                string    `gorm:"column:venue;size:255;not null;index:idx_execution_orders_instrument_time_id,priority:1"`
	Symbol               string    `gorm:"column:symbol;size:255;not null;index:idx_execution_orders_instrument_time_id,priority:2"`
	AssetClass           string    `gorm:"column:asset_class;size:64;not null;index:idx_execution_orders_instrument_time_id,priority:3"`
	OrderType            string    `gorm:"column:order_type;size:32;not null"`
	TimeInForce          string    `gorm:"column:time_in_force;size:32;not null"`
	ReduceOnly           bool      `gorm:"column:reduce_only;not null"`
	ClientOrderID        string    `gorm:"column:client_order_id;size:255;not null;uniqueIndex:idx_execution_orders_client_order_id"`
	Status               string    `gorm:"column:status;size:32;not null"`
	Quantity             float64   `gorm:"column:quantity;not null"`
	Notional             float64   `gorm:"column:notional;not null"`
	LimitPrice           *float64  `gorm:"column:limit_price"`
	EventTime            time.Time `gorm:"column:event_time;not null;index:idx_execution_orders_mode_time_id,priority:2;index:idx_execution_orders_instrument_time_id,priority:4"`
	CreatedAt            time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt            time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (executionOrderModel) TableName(namer schema.Namer) string {
	return namer.TableName("execution_orders")
}

type executionFillModel struct {
	FillID                    string    `gorm:"column:fill_id;size:255;not null;primaryKey;uniqueIndex:idx_execution_fills_fill_id"`
	CommandID                 string    `gorm:"column:command_id;size:255;not null;index:idx_execution_fills_command_id"`
	OrderID                   string    `gorm:"column:order_id;size:255;not null;index:idx_execution_fills_order_id"`
	Mode                      string    `gorm:"column:mode;size:32;not null;index:idx_execution_fills_mode_time_id,priority:1"`
	StrategyID                string    `gorm:"column:strategy_id;size:255;not null"`
	StrategyVersion           string    `gorm:"column:strategy_version;size:255;not null"`
	StrategyArtifactHash      string    `gorm:"column:strategy_artifact_hash;size:255;not null"`
	Venue                     string    `gorm:"column:venue;size:255;not null;index:idx_execution_fills_instrument_time_id,priority:1"`
	Symbol                    string    `gorm:"column:symbol;size:255;not null;index:idx_execution_fills_instrument_time_id,priority:2"`
	AssetClass                string    `gorm:"column:asset_class;size:64;not null;index:idx_execution_fills_instrument_time_id,priority:3"`
	ActionKind                string    `gorm:"column:action_kind;size:32;not null"`
	Quantity                  float64   `gorm:"column:quantity;not null"`
	Price                     float64   `gorm:"column:price;not null"`
	FeeAmount                 float64   `gorm:"column:fee_amount;not null"`
	SlippageAmount            float64   `gorm:"column:slippage_amount;not null"`
	SourceMarketDataReference string    `gorm:"column:source_market_data_reference;size:255;not null"`
	MetadataJSON              string    `gorm:"column:metadata_json;size:4096;not null"`
	EventTime                 time.Time `gorm:"column:event_time;not null;index:idx_execution_fills_mode_time_id,priority:2;index:idx_execution_fills_instrument_time_id,priority:4"`
	CreatedAt                 time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt                 time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (executionFillModel) TableName(namer schema.Namer) string {
	return namer.TableName("execution_fills")
}

// DatabaseStoreOpts configures execution database concerns used by app wiring.
type DatabaseStoreOpts struct {
	TablePrefix string
}

type executionStore interface {
	CreateCommand(ctx context.Context, command domain.ExecutionCommand) (domain.ExecutionCommand, error)
	CreateOrder(ctx context.Context, order domain.ExecutionOrder) (domain.ExecutionOrder, error)
	UpdateOrderStatus(
		ctx context.Context,
		orderID string,
		status domain.ExecutionOrderStatus,
	) (domain.ExecutionOrder, error)
	CreateFill(ctx context.Context, fill domain.ExecutionFill) (domain.ExecutionFill, error)
	ListFillsByOrder(ctx context.Context, orderID string) ([]domain.ExecutionFill, error)
}

// DatabaseStore persists execution ledger records with GORM.
type DatabaseStore struct {
	db *gorm.DB
}

// NewDatabaseStore opens a database-backed execution store.
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

// AutoMigrate creates or updates the durable execution relational schema.
func (s *DatabaseStore) AutoMigrate() error {
	return s.db.AutoMigrate(
		&executionCommandModel{},
		&executionOrderModel{},
		&executionFillModel{},
		&positionSnapshotModel{},
		&portfolioSnapshotModel{},
	)
}

func (s *DatabaseStore) CreateCommand(
	ctx context.Context,
	command domain.ExecutionCommand,
) (domain.ExecutionCommand, error) {
	model, err := executionCommandToModel(command)
	if err != nil {
		return domain.ExecutionCommand{}, err
	}

	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: executionCommandIDColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return domain.ExecutionCommand{}, fmt.Errorf("create execution command: %w", err)
	}

	return domain.NewExecutionCommand(domain.ExecutionCommandParams{
		CommandID:                 string(command.CommandID),
		TraceID:                   string(command.TraceID),
		IntentID:                  string(command.IntentID),
		GovernorDecisionReference: command.GovernorDecisionReference,
		Mode:                      command.Mode,
		StrategyID:                command.StrategyID,
		StrategyVersion:           command.StrategyVersion,
		StrategyArtifactHash:      command.StrategyArtifactHash,
		Venue:                     command.Venue,
		Instrument:                command.Instrument,
		ActionKind:                command.ActionKind,
		OrderType:                 command.OrderType,
		LimitPrice:                command.LimitPrice,
		ReduceOnly:                command.ReduceOnly,
		ApprovedDecision:          command.ApprovedDecision,
		Status:                    command.Status,
		Quantity:                  command.Quantity,
		Notional:                  command.Notional,
		EventTime:                 command.EventTime.Time(),
	})
}

func (s *DatabaseStore) CreateOrder(
	ctx context.Context,
	order domain.ExecutionOrder,
) (domain.ExecutionOrder, error) {
	model, err := executionOrderToModel(order)
	if err != nil {
		return domain.ExecutionOrder{}, err
	}

	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: executionOrderIDColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return domain.ExecutionOrder{}, fmt.Errorf("create execution order: %w", err)
	}

	return domain.NewExecutionOrder(domain.ExecutionOrderParams{
		OrderID:              string(order.OrderID),
		Command:              order.Command,
		Mode:                 order.Mode,
		StrategyID:           order.StrategyID,
		StrategyVersion:      order.StrategyVersion,
		StrategyArtifactHash: order.StrategyArtifactHash,
		Venue:                order.Venue,
		Instrument:           order.Instrument,
		OrderType:            order.OrderType,
		TimeInForce:          order.TimeInForce,
		ReduceOnly:           order.ReduceOnly,
		ClientOrderID:        order.ClientOrderID,
		Status:               order.Status,
		Quantity:             order.Quantity,
		Notional:             order.Notional,
		LimitPrice:           order.LimitPrice,
		EventTime:            order.EventTime.Time(),
	})
}

func (s *DatabaseStore) UpdateOrderStatus(
	ctx context.Context,
	orderID string,
	status domain.ExecutionOrderStatus,
) (domain.ExecutionOrder, error) {
	canonicalStatus, err := domain.NewExecutionOrderStatus(status.String())
	if err != nil {
		return domain.ExecutionOrder{}, fmt.Errorf("update execution order status: %w", err)
	}

	if err = s.db.WithContext(ctx).
		Model(&executionOrderModel{}).
		Where("order_id = ?", orderID).
		Update("status", canonicalStatus.String()).Error; err != nil {
		return domain.ExecutionOrder{}, fmt.Errorf("update execution order status: %w", err)
	}

	persisted, err := s.findExecutionOrderModel(ctx, orderID)
	if err != nil {
		return domain.ExecutionOrder{}, err
	}

	return s.executionOrderModelToDomain(ctx, persisted)
}

func (s *DatabaseStore) CreateFill(
	ctx context.Context,
	fill domain.ExecutionFill,
) (domain.ExecutionFill, error) {
	model, err := executionFillToModel(fill)
	if err != nil {
		return domain.ExecutionFill{}, err
	}

	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: executionFillIDColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return domain.ExecutionFill{}, fmt.Errorf("create execution fill: %w", err)
	}

	return domain.NewExecutionFill(domain.ExecutionFillParams{
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
}

func (s *DatabaseStore) ListFillsByOrder(
	ctx context.Context,
	orderID string,
) ([]domain.ExecutionFill, error) {
	var models []executionFillModel
	if err := s.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("event_time ASC").
		Order("fill_id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list execution fills by order: %w", err)
	}

	fills := make([]domain.ExecutionFill, 0, len(models))
	for _, model := range models {
		fill, err := s.executionFillModelToDomain(ctx, model)
		if err != nil {
			return nil, err
		}
		fills = append(fills, fill)
	}

	return fills, nil
}

// GetCommand reads one persisted execution command by stable id.
func (s *DatabaseStore) GetCommand(
	ctx context.Context,
	commandID string,
) (*domain.ExecutionCommand, error) {
	model, err := s.findExecutionCommandModel(ctx, commandID)
	if err != nil {
		return nil, err
	}

	command, err := executionCommandModelToDomain(model)
	if err != nil {
		return nil, err
	}

	return &command, nil
}

// GetOrder reads one persisted execution order by stable id.
func (s *DatabaseStore) GetOrder(
	ctx context.Context,
	orderID string,
) (*domain.ExecutionOrder, error) {
	model, err := s.findExecutionOrderModel(ctx, orderID)
	if err != nil {
		return nil, err
	}

	order, err := s.executionOrderModelToDomain(ctx, model)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// GetFill reads one persisted execution fill by stable id.
func (s *DatabaseStore) GetFill(
	ctx context.Context,
	fillID string,
) (*domain.ExecutionFill, error) {
	var model executionFillModel
	if err := s.db.WithContext(ctx).First(&model, "fill_id = ?", fillID).Error; err != nil {
		return nil, fmt.Errorf("find execution fill: %w", err)
	}

	fill, err := s.executionFillModelToDomain(ctx, model)
	if err != nil {
		return nil, err
	}

	return &fill, nil
}

func (s *DatabaseStore) findExecutionCommandModel(
	ctx context.Context,
	commandID string,
) (executionCommandModel, error) {
	var model executionCommandModel
	if err := s.db.WithContext(ctx).First(&model, "command_id = ?", commandID).Error; err != nil {
		return executionCommandModel{}, fmt.Errorf("find execution command: %w", err)
	}

	return model, nil
}

func (s *DatabaseStore) findExecutionOrderModel(
	ctx context.Context,
	orderID string,
) (executionOrderModel, error) {
	var model executionOrderModel
	if err := s.db.WithContext(ctx).First(&model, "order_id = ?", orderID).Error; err != nil {
		return executionOrderModel{}, fmt.Errorf("find execution order: %w", err)
	}

	return model, nil
}

func executionCommandToModel(command domain.ExecutionCommand) (executionCommandModel, error) {
	canonical, err := domain.NewExecutionCommand(domain.ExecutionCommandParams{
		CommandID:                 string(command.CommandID),
		TraceID:                   string(command.TraceID),
		IntentID:                  string(command.IntentID),
		GovernorDecisionReference: command.GovernorDecisionReference,
		Mode:                      command.Mode,
		StrategyID:                command.StrategyID,
		StrategyVersion:           command.StrategyVersion,
		StrategyArtifactHash:      command.StrategyArtifactHash,
		Venue:                     command.Venue,
		Instrument:                command.Instrument,
		ActionKind:                command.ActionKind,
		OrderType:                 command.OrderType,
		LimitPrice:                command.LimitPrice,
		ReduceOnly:                command.ReduceOnly,
		ApprovedDecision:          command.ApprovedDecision,
		Status:                    command.Status,
		Quantity:                  command.Quantity,
		Notional:                  command.Notional,
		EventTime:                 command.EventTime.Time(),
	})
	if err != nil {
		return executionCommandModel{}, err
	}

	return executionCommandModel{
		CommandID:                 string(canonical.CommandID),
		TraceID:                   string(canonical.TraceID),
		IntentID:                  string(canonical.IntentID),
		GovernorDecisionReference: canonical.GovernorDecisionReference,
		Mode:                      canonical.Mode.String(),
		StrategyID:                canonical.StrategyID,
		StrategyVersion:           canonical.StrategyVersion,
		StrategyArtifactHash:      canonical.StrategyArtifactHash,
		Venue:                     canonical.Venue.String(),
		Symbol:                    canonical.Instrument.Symbol.String(),
		AssetClass:                canonical.Instrument.AssetClass.String(),
		ActionKind:                canonical.ActionKind.String(),
		OrderType:                 canonical.OrderType.String(),
		LimitPrice:                canonical.LimitPrice,
		ReduceOnly:                canonical.ReduceOnly,
		ApprovedQuantity:          canonical.Quantity,
		ApprovedNotional:          canonical.Notional,
		ApprovedInstrumentActive:  canonical.ApprovedDecision.CandidateAction.Strategy.Instrument.Active,
		ApprovedTimeframe:         canonical.ApprovedDecision.CandidateAction.Strategy.Timeframe.String(),
		ApprovedInputStart:        canonical.ApprovedDecision.CandidateAction.InputRange.Start,
		ApprovedInputEnd:          canonical.ApprovedDecision.CandidateAction.InputRange.End,
		ApprovedQuality:           canonical.ApprovedDecision.CandidateAction.Quality.String(),
		DecisionStatus:            canonical.ApprovedDecision.Status.String(),
		DecisionReason:            canonical.ApprovedDecision.Reason.String(),
		DecisionTime:              canonical.ApprovedDecision.DecisionTime.Time(),
		Status:                    canonical.Status.String(),
		EventTime:                 canonical.EventTime.Time(),
	}, nil
}

func executionCommandModelToDomain(model executionCommandModel) (domain.ExecutionCommand, error) {
	instrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      domain.Venue(model.Venue),
		Symbol:     domain.Symbol(model.Symbol),
		AssetClass: domain.AssetClass(model.AssetClass),
		Active:     approvedInstrumentActive(model),
	})
	if err != nil {
		return domain.ExecutionCommand{}, fmt.Errorf("execution command instrument: %w", err)
	}

	action, err := domain.NewCandidateAction(domain.CandidateActionParams{
		Strategy: domain.StrategyIdentity{
			Instrument: instrument,
			Timeframe:  approvedTimeframe(model),
			Kind:       domain.StrategyKindMovingAverageCrossover,
		},
		Kind:         domain.CandidateActionKind(model.ActionKind),
		DecisionTime: model.DecisionTime,
		InputRange:   approvedInputRange(model),
		Quality:      approvedQuality(model),
	})
	if err != nil {
		return domain.ExecutionCommand{}, fmt.Errorf("execution command candidate action: %w", err)
	}

	decision, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
		CandidateAction: action,
		Status:          domain.GovernorDecisionStatus(model.DecisionStatus),
		Reason:          domain.GovernorDecisionReason(model.DecisionReason),
		DecisionTime:    model.DecisionTime,
	})
	if err != nil {
		return domain.ExecutionCommand{}, fmt.Errorf("execution command decision: %w", err)
	}

	return domain.NewExecutionCommand(domain.ExecutionCommandParams{
		CommandID:                 model.CommandID,
		TraceID:                   model.TraceID,
		IntentID:                  model.IntentID,
		GovernorDecisionReference: model.GovernorDecisionReference,
		Mode:                      domain.DecisionMode(model.Mode),
		StrategyID:                model.StrategyID,
		StrategyVersion:           model.StrategyVersion,
		StrategyArtifactHash:      model.StrategyArtifactHash,
		Venue:                     domain.Venue(model.Venue),
		Instrument:                instrument,
		ActionKind:                domain.CandidateActionKind(model.ActionKind),
		OrderType:                 domain.OrderType(model.OrderType),
		LimitPrice:                model.LimitPrice,
		ReduceOnly:                model.ReduceOnly,
		ApprovedDecision:          decision,
		Status:                    domain.ExecutionCommandStatus(model.Status),
		Quantity:                  model.ApprovedQuantity,
		Notional:                  model.ApprovedNotional,
		EventTime:                 model.EventTime,
	})
}

func approvedInstrumentActive(model executionCommandModel) bool {
	if model.ApprovedTimeframe == "" &&
		model.ApprovedQuality == "" &&
		model.ApprovedInputStart.IsZero() &&
		model.ApprovedInputEnd.IsZero() {
		return true
	}

	return model.ApprovedInstrumentActive
}

func approvedTimeframe(model executionCommandModel) domain.Timeframe {
	if model.ApprovedTimeframe == "" {
		return domain.Timeframe1m
	}

	return domain.Timeframe(model.ApprovedTimeframe)
}

func approvedInputRange(model executionCommandModel) domain.TimeRange {
	if model.ApprovedInputStart.IsZero() || model.ApprovedInputEnd.IsZero() {
		return domain.TimeRange{
			Start: model.DecisionTime,
			End:   model.DecisionTime.Add(time.Minute),
		}
	}

	return domain.TimeRange{
		Start: model.ApprovedInputStart,
		End:   model.ApprovedInputEnd,
	}
}

func approvedQuality(model executionCommandModel) domain.DataQuality {
	if model.ApprovedQuality == "" {
		return domain.DataQualityValidated
	}

	return domain.DataQuality(model.ApprovedQuality)
}

func executionOrderToModel(order domain.ExecutionOrder) (executionOrderModel, error) {
	canonical, err := domain.NewExecutionOrder(domain.ExecutionOrderParams{
		OrderID:              string(order.OrderID),
		Command:              order.Command,
		Mode:                 order.Mode,
		StrategyID:           order.StrategyID,
		StrategyVersion:      order.StrategyVersion,
		StrategyArtifactHash: order.StrategyArtifactHash,
		Venue:                order.Venue,
		Instrument:           order.Instrument,
		OrderType:            order.OrderType,
		TimeInForce:          order.TimeInForce,
		ReduceOnly:           order.ReduceOnly,
		ClientOrderID:        order.ClientOrderID,
		Status:               order.Status,
		Quantity:             order.Quantity,
		Notional:             order.Notional,
		LimitPrice:           order.LimitPrice,
		EventTime:            order.EventTime.Time(),
	})
	if err != nil {
		return executionOrderModel{}, err
	}

	return executionOrderModel{
		OrderID:              string(canonical.OrderID),
		CommandID:            string(canonical.Command.CommandID),
		Mode:                 canonical.Mode.String(),
		StrategyID:           canonical.StrategyID,
		StrategyVersion:      canonical.StrategyVersion,
		StrategyArtifactHash: canonical.StrategyArtifactHash,
		Venue:                canonical.Venue.String(),
		Symbol:               canonical.Instrument.Symbol.String(),
		AssetClass:           canonical.Instrument.AssetClass.String(),
		OrderType:            canonical.OrderType.String(),
		TimeInForce:          canonical.TimeInForce.String(),
		ReduceOnly:           canonical.ReduceOnly,
		ClientOrderID:        canonical.ClientOrderID,
		Status:               canonical.Status.String(),
		Quantity:             canonical.Quantity,
		Notional:             canonical.Notional,
		LimitPrice:           canonical.LimitPrice,
		EventTime:            canonical.EventTime.Time(),
	}, nil
}

func (s *DatabaseStore) executionOrderModelToDomain(
	ctx context.Context,
	model executionOrderModel,
) (domain.ExecutionOrder, error) {
	commandModel, err := s.findExecutionCommandModel(ctx, model.CommandID)
	if err != nil {
		return domain.ExecutionOrder{}, err
	}
	command, err := executionCommandModelToDomain(commandModel)
	if err != nil {
		return domain.ExecutionOrder{}, err
	}

	instrument, err := domain.NewInstrument(domain.InstrumentParams{
		Venue:      domain.Venue(model.Venue),
		Symbol:     domain.Symbol(model.Symbol),
		AssetClass: domain.AssetClass(model.AssetClass),
		Active:     true,
	})
	if err != nil {
		return domain.ExecutionOrder{}, fmt.Errorf("execution order instrument: %w", err)
	}

	return domain.NewExecutionOrder(domain.ExecutionOrderParams{
		OrderID:              model.OrderID,
		Command:              command,
		Mode:                 domain.DecisionMode(model.Mode),
		StrategyID:           model.StrategyID,
		StrategyVersion:      model.StrategyVersion,
		StrategyArtifactHash: model.StrategyArtifactHash,
		Venue:                domain.Venue(model.Venue),
		Instrument:           instrument,
		OrderType:            domain.OrderType(model.OrderType),
		TimeInForce:          domain.TimeInForce(model.TimeInForce),
		ReduceOnly:           model.ReduceOnly,
		ClientOrderID:        model.ClientOrderID,
		Status:               domain.ExecutionOrderStatus(model.Status),
		Quantity:             model.Quantity,
		Notional:             model.Notional,
		LimitPrice:           model.LimitPrice,
		EventTime:            model.EventTime,
	})
}

func executionFillToModel(fill domain.ExecutionFill) (executionFillModel, error) {
	metadataJSON, err := json.Marshal(fill.Metadata)
	if err != nil {
		return executionFillModel{}, fmt.Errorf("marshal execution fill metadata: %w", err)
	}

	canonical, err := domain.NewExecutionFill(domain.ExecutionFillParams{
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
	if err != nil {
		return executionFillModel{}, err
	}

	return executionFillModel{
		FillID:                    string(canonical.FillID),
		CommandID:                 string(canonical.Order.Command.CommandID),
		OrderID:                   string(canonical.Order.OrderID),
		Mode:                      canonical.Order.Mode.String(),
		StrategyID:                canonical.Order.StrategyID,
		StrategyVersion:           canonical.Order.StrategyVersion,
		StrategyArtifactHash:      canonical.Order.StrategyArtifactHash,
		Venue:                     canonical.Order.Venue.String(),
		Symbol:                    canonical.Order.Instrument.Symbol.String(),
		AssetClass:                canonical.Order.Instrument.AssetClass.String(),
		ActionKind:                canonical.Order.Command.ActionKind.String(),
		Quantity:                  canonical.Quantity,
		Price:                     canonical.Price,
		FeeAmount:                 canonical.FeeAmount,
		SlippageAmount:            canonical.SlippageAmount,
		SourceMarketDataReference: canonical.SourceMarketDataReference,
		MetadataJSON:              string(metadataJSON),
		EventTime:                 canonical.EventTime.Time(),
	}, nil
}

func (s *DatabaseStore) executionFillModelToDomain(
	ctx context.Context,
	model executionFillModel,
) (domain.ExecutionFill, error) {
	orderModel, err := s.findExecutionOrderModel(ctx, model.OrderID)
	if err != nil {
		return domain.ExecutionFill{}, err
	}
	order, err := s.executionOrderModelToDomain(ctx, orderModel)
	if err != nil {
		return domain.ExecutionFill{}, err
	}

	metadata := map[string]string{}
	if unmarshalErr := json.Unmarshal([]byte(model.MetadataJSON), &metadata); unmarshalErr != nil {
		return domain.ExecutionFill{}, fmt.Errorf(
			"unmarshal execution fill metadata: %w",
			unmarshalErr,
		)
	}

	return domain.NewExecutionFill(domain.ExecutionFillParams{
		FillID:                    model.FillID,
		Order:                     order,
		SourceMarketDataReference: model.SourceMarketDataReference,
		FeeAmount:                 model.FeeAmount,
		SlippageAmount:            model.SlippageAmount,
		Metadata:                  metadata,
		Quantity:                  model.Quantity,
		Price:                     model.Price,
		EventTime:                 model.EventTime,
	})
}
