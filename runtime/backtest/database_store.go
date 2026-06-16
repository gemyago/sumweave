package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/gormsignalfoundry"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type datasetInstrumentRecord struct {
	Venue      string `json:"venue"`
	Symbol     string `json:"symbol"`
	AssetClass string `json:"assetClass"`
	Active     bool   `json:"active"`
}

type datasetReferenceModel struct {
	DatasetID        string    `gorm:"column:dataset_id;size:255;not null;primaryKey;uniqueIndex:idx_dataset_references_dataset_id"`
	EntityTypesJSON  string    `gorm:"column:entity_types_json;size:4096;not null"`
	InstrumentsJSON  string    `gorm:"column:instruments_json;size:4096;not null"`
	TimeframesJSON   string    `gorm:"column:timeframes_json;size:1024;not null"`
	TimeRangeStart   time.Time `gorm:"column:time_range_start;not null"`
	TimeRangeEnd     time.Time `gorm:"column:time_range_end;not null"`
	SourceHashesJSON string    `gorm:"column:source_hashes_json;size:4096;not null"`
	ReplayChecksum   string    `gorm:"column:replay_checksum;size:255;not null"`
	MetadataJSON     *string   `gorm:"column:metadata_json;size:4096"`
	CreatedAt        time.Time `gorm:"column:created_at;not null;index:idx_dataset_references_created_at_dataset_id,priority:1"`
}

func (datasetReferenceModel) TableName(namer schema.Namer) string {
	return namer.TableName("dataset_references")
}

type backtestRunModel struct {
	RunID                     string    `gorm:"column:run_id;size:255;not null;primaryKey;uniqueIndex:idx_backtest_runs_run_id"`
	StrategyID                string    `gorm:"column:strategy_id;size:255;not null;index:idx_backtest_runs_strategy_created_id,priority:1"`
	StrategyVersion           string    `gorm:"column:strategy_version;size:255;not null"`
	StrategyArtifactHash      string    `gorm:"column:strategy_artifact_hash;size:255;not null"`
	DatasetID                 string    `gorm:"column:dataset_id;size:255;not null;index:idx_backtest_runs_dataset_created_id,priority:1"`
	GovernorPolicyID          string    `gorm:"column:governor_policy_id;size:255;not null"`
	GovernorPolicyVersion     string    `gorm:"column:governor_policy_version;size:255;not null"`
	GovernorPolicyHash        string    `gorm:"column:governor_policy_hash;size:255;not null"`
	Mode                      string    `gorm:"column:mode;size:32;not null"`
	TestedRangeStart          time.Time `gorm:"column:tested_range_start;not null"`
	TestedRangeEnd            time.Time `gorm:"column:tested_range_end;not null"`
	FeeModelID                string    `gorm:"column:fee_model_id;size:255;not null"`
	FeeAssumptionsJSON        *string   `gorm:"column:fee_assumptions_json;size:4096"`
	SlippageModelID           string    `gorm:"column:slippage_model_id;size:255;not null"`
	SlippageAssumptionsJSON   *string   `gorm:"column:slippage_assumptions_json;size:4096"`
	ExecutionSimulatorVersion string    `gorm:"column:execution_simulator_version;size:255;not null"`
	Status                    string    `gorm:"column:status;size:32;not null;index:idx_backtest_runs_status_created_id,priority:1"`
	MetricsJSON               *string   `gorm:"column:metrics_json;size:4096"`
	FailureReason             string    `gorm:"column:failure_reason;size:255;not null"`
	FailureDetails            string    `gorm:"column:failure_details;size:4096;not null"`
	CreatedAt                 time.Time `gorm:"column:created_at;not null;index:idx_backtest_runs_strategy_created_id,priority:2;index:idx_backtest_runs_dataset_created_id,priority:2;index:idx_backtest_runs_status_created_id,priority:2"`
	UpdatedAt                 time.Time `gorm:"column:updated_at;not null"`
}

func (backtestRunModel) TableName(namer schema.Namer) string {
	return namer.TableName("backtest_runs")
}

type evaluationReportModel struct {
	EvaluationID         string    `gorm:"column:evaluation_id;size:255;not null;primaryKey;uniqueIndex:idx_evaluation_reports_evaluation_id"`
	StrategyID           string    `gorm:"column:strategy_id;size:255;not null;index:idx_evaluation_reports_strategy_created_id,priority:1"`
	StrategyVersion      string    `gorm:"column:strategy_version;size:255;not null"`
	StrategyArtifactHash string    `gorm:"column:strategy_artifact_hash;size:255;not null"`
	BacktestRunID        string    `gorm:"column:backtest_run_id;size:255;not null;index:idx_evaluation_reports_backtest_created_id,priority:1"`
	DatasetID            string    `gorm:"column:dataset_id;size:255;not null;index:idx_evaluation_reports_dataset_created_id,priority:1"`
	Decision             string    `gorm:"column:decision;size:64;not null;index:idx_evaluation_reports_decision_created_id,priority:1"`
	MetricsJSON          *string   `gorm:"column:metrics_json;size:4096"`
	FailureReasonsJSON   *string   `gorm:"column:failure_reasons_json;size:4096"`
	Notes                string    `gorm:"column:notes;size:4096;not null"`
	CreatedAt            time.Time `gorm:"column:created_at;not null;index:idx_evaluation_reports_strategy_created_id,priority:2;index:idx_evaluation_reports_backtest_created_id,priority:2;index:idx_evaluation_reports_dataset_created_id,priority:2;index:idx_evaluation_reports_decision_created_id,priority:2"`
}

func (evaluationReportModel) TableName(namer schema.Namer) string {
	return namer.TableName("evaluation_reports")
}

// DatabaseStoreOpts configures backtest database concerns used by app wiring.
type DatabaseStoreOpts struct {
	TablePrefix string
}

// DatabaseStore persists backtest scaffold records with GORM.
type DatabaseStore struct {
	db *gorm.DB
}

// NewDatabaseStore opens a database-backed backtest scaffold store.
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

	return &DatabaseStore{db: db}, nil
}

// AutoMigrate creates or updates the durable backtest relational schema.
func (s *DatabaseStore) AutoMigrate() error {
	return s.db.AutoMigrate(
		&datasetReferenceModel{},
		&backtestRunModel{},
		&evaluationReportModel{},
	)
}

func (s *DatabaseStore) CreateDatasetReference(
	ctx context.Context,
	reference domain.DatasetReference,
) (domain.DatasetReference, error) {
	model, err := datasetReferenceToModel(reference)
	if err != nil {
		return domain.DatasetReference{}, err
	}
	if err = s.db.WithContext(ctx).Create(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			persisted, getErr := s.getDatasetReferenceModel(ctx, reference.DatasetID.String())
			if getErr != nil {
				return domain.DatasetReference{}, fmt.Errorf(
					"create dataset reference: load existing dataset reference: %w",
					getErr,
				)
			}

			return datasetReferenceFromModel(persisted)
		}

		return domain.DatasetReference{}, fmt.Errorf("create dataset reference: %w", err)
	}

	return datasetReferenceFromModel(model)
}

func (s *DatabaseStore) getDatasetReferenceModel(
	ctx context.Context,
	datasetID string,
) (datasetReferenceModel, error) {
	var model datasetReferenceModel
	if err := s.db.WithContext(ctx).Where("dataset_id = ?", datasetID).First(&model).Error; err != nil {
		return datasetReferenceModel{}, err
	}

	return model, nil
}

// GetDatasetReference reads one persisted dataset reference by stable id.
func (s *DatabaseStore) GetDatasetReference(
	ctx context.Context,
	datasetID string,
) (*domain.DatasetReference, error) {
	model, err := s.getDatasetReferenceModel(ctx, datasetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBacktestRunNotFound
		}

		return nil, fmt.Errorf("get dataset reference: %w", err)
	}

	reference, err := datasetReferenceFromModel(model)
	if err != nil {
		return nil, err
	}

	return &reference, nil
}

func (s *DatabaseStore) CreateBacktestRun(
	ctx context.Context,
	run domain.BacktestRun,
) (domain.BacktestRun, error) {
	model, err := backtestRunToModel(run)
	if err != nil {
		return domain.BacktestRun{}, err
	}
	if err = s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.BacktestRun{}, fmt.Errorf("create backtest run: %w", err)
	}

	return backtestRunFromModel(model)
}

func (s *DatabaseStore) GetBacktestRun(
	ctx context.Context,
	runID string,
) (*domain.BacktestRun, error) {
	var model backtestRunModel
	err := s.db.WithContext(ctx).Where("run_id = ?", runID).First(&model).Error
	if err == nil {
		run, convErr := backtestRunFromModel(model)
		if convErr != nil {
			return nil, convErr
		}
		return &run, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBacktestRunNotFound
	}

	return nil, fmt.Errorf("get backtest run: %w", err)
}

func (s *DatabaseStore) UpdateBacktestRun(
	ctx context.Context,
	run domain.BacktestRun,
) (domain.BacktestRun, error) {
	model, err := backtestRunToModel(run)
	if err != nil {
		return domain.BacktestRun{}, err
	}
	if err = s.db.WithContext(ctx).
		Model(&backtestRunModel{}).
		Where("run_id = ?", run.RunID.String()).
		UpdateColumns(map[string]any{
			"strategy_id":                 model.StrategyID,
			"strategy_version":            model.StrategyVersion,
			"strategy_artifact_hash":      model.StrategyArtifactHash,
			"dataset_id":                  model.DatasetID,
			"governor_policy_id":          model.GovernorPolicyID,
			"governor_policy_version":     model.GovernorPolicyVersion,
			"governor_policy_hash":        model.GovernorPolicyHash,
			"mode":                        model.Mode,
			"tested_range_start":          model.TestedRangeStart,
			"tested_range_end":            model.TestedRangeEnd,
			"fee_model_id":                model.FeeModelID,
			"fee_assumptions_json":        model.FeeAssumptionsJSON,
			"slippage_model_id":           model.SlippageModelID,
			"slippage_assumptions_json":   model.SlippageAssumptionsJSON,
			"execution_simulator_version": model.ExecutionSimulatorVersion,
			"status":                      model.Status,
			"metrics_json":                model.MetricsJSON,
			"failure_reason":              model.FailureReason,
			"failure_details":             model.FailureDetails,
			"created_at":                  model.CreatedAt,
			"updated_at":                  model.UpdatedAt,
		}).Error; err != nil {
		return domain.BacktestRun{}, fmt.Errorf("update backtest run: %w", err)
	}

	return backtestRunFromModel(model)
}

func (s *DatabaseStore) QueryBacktestRuns(
	ctx context.Context,
	query RunQuery,
) ([]domain.BacktestRun, error) {
	statement := s.db.WithContext(ctx).Model(&backtestRunModel{})
	if query.StrategyID != "" {
		statement = statement.Where("strategy_id = ?", query.StrategyID)
	}
	if query.DatasetID != "" {
		statement = statement.Where("dataset_id = ?", query.DatasetID)
	}
	if query.Status != nil {
		statement = statement.Where("status = ?", query.Status.String())
	}
	if query.TimeRange != nil {
		statement = statement.Where("created_at >= ?", query.TimeRange.Start.UTC()).
			Where("created_at < ?", query.TimeRange.End.UTC())
	}

	var models []backtestRunModel
	if err := statement.Order("created_at ASC").Order("run_id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query backtest runs: %w", err)
	}

	runs := make([]domain.BacktestRun, 0, len(models))
	for _, model := range models {
		run, err := backtestRunFromModel(model)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, nil
}

func (s *DatabaseStore) CreateEvaluationReport(
	ctx context.Context,
	report domain.EvaluationReport,
) (domain.EvaluationReport, error) {
	model, err := evaluationReportToModel(report)
	if err != nil {
		return domain.EvaluationReport{}, err
	}
	if err = s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.EvaluationReport{}, fmt.Errorf("create evaluation report: %w", err)
	}

	return evaluationReportFromModel(model)
}

func (s *DatabaseStore) QueryEvaluationReports(
	ctx context.Context,
	query EvaluationReportQuery,
) ([]domain.EvaluationReport, error) {
	statement := s.db.WithContext(ctx).Model(&evaluationReportModel{})
	if query.StrategyID != "" {
		statement = statement.Where("strategy_id = ?", query.StrategyID)
	}
	if query.BacktestID != "" {
		statement = statement.Where("backtest_run_id = ?", query.BacktestID)
	}
	if query.DatasetID != "" {
		statement = statement.Where("dataset_id = ?", query.DatasetID)
	}
	if query.Decision != nil {
		statement = statement.Where("decision = ?", query.Decision.String())
	}
	if query.TimeRange != nil {
		statement = statement.Where("created_at >= ?", query.TimeRange.Start.UTC()).
			Where("created_at < ?", query.TimeRange.End.UTC())
	}

	var models []evaluationReportModel
	if err := statement.Order("created_at ASC").Order("evaluation_id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query evaluation reports: %w", err)
	}

	reports := make([]domain.EvaluationReport, 0, len(models))
	for _, model := range models {
		report, err := evaluationReportFromModel(model)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// GetEvaluationReport reads one persisted evaluation report by stable id.
func (s *DatabaseStore) GetEvaluationReport(
	ctx context.Context,
	evaluationID string,
) (*domain.EvaluationReport, error) {
	var model evaluationReportModel
	if err := s.db.WithContext(ctx).Where("evaluation_id = ?", evaluationID).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBacktestRunNotFound
		}

		return nil, fmt.Errorf("get evaluation report: %w", err)
	}

	report, err := evaluationReportFromModel(model)
	if err != nil {
		return nil, err
	}

	return &report, nil
}

func datasetReferenceToModel(reference domain.DatasetReference) (datasetReferenceModel, error) {
	entityTypesJSON, err := marshalJSON(reference.EntityTypes)
	if err != nil {
		return datasetReferenceModel{}, fmt.Errorf("marshal dataset entity types: %w", err)
	}
	instrumentsJSON, err := marshalJSON(datasetInstrumentRecords(reference.Instruments))
	if err != nil {
		return datasetReferenceModel{}, fmt.Errorf("marshal dataset instruments: %w", err)
	}
	timeframesJSON, err := marshalJSON(reference.Timeframes)
	if err != nil {
		return datasetReferenceModel{}, fmt.Errorf("marshal dataset timeframes: %w", err)
	}
	sourceHashesJSON, err := marshalJSON(reference.SourceDataHashes)
	if err != nil {
		return datasetReferenceModel{}, fmt.Errorf("marshal dataset source hashes: %w", err)
	}
	metadataJSON, hasMetadataJSON, err := marshalOptionalJSON(reference.Metadata)
	if err != nil {
		return datasetReferenceModel{}, fmt.Errorf("marshal dataset metadata: %w", err)
	}
	if !hasMetadataJSON {
		metadataJSON = nil
	}

	return datasetReferenceModel{
		DatasetID:        reference.DatasetID.String(),
		EntityTypesJSON:  entityTypesJSON,
		InstrumentsJSON:  instrumentsJSON,
		TimeframesJSON:   timeframesJSON,
		TimeRangeStart:   reference.TimeRange.Start.UTC(),
		TimeRangeEnd:     reference.TimeRange.End.UTC(),
		SourceHashesJSON: sourceHashesJSON,
		ReplayChecksum:   reference.ReplayChecksum,
		MetadataJSON:     metadataJSON,
		CreatedAt:        reference.CreatedAt.Time(),
	}, nil
}

func datasetReferenceFromModel(model datasetReferenceModel) (domain.DatasetReference, error) {
	var entityTypes []string
	if err := json.Unmarshal([]byte(model.EntityTypesJSON), &entityTypes); err != nil {
		return domain.DatasetReference{}, fmt.Errorf("unmarshal dataset entity types: %w", err)
	}
	var instrumentRecords []datasetInstrumentRecord
	if err := json.Unmarshal([]byte(model.InstrumentsJSON), &instrumentRecords); err != nil {
		return domain.DatasetReference{}, fmt.Errorf("unmarshal dataset instruments: %w", err)
	}
	instruments, err := datasetInstrumentsFromRecords(instrumentRecords)
	if err != nil {
		return domain.DatasetReference{}, fmt.Errorf("unmarshal dataset instruments: %w", err)
	}
	var timeframes []domain.Timeframe
	if unmarshalErr := json.Unmarshal([]byte(model.TimeframesJSON), &timeframes); unmarshalErr != nil {
		return domain.DatasetReference{}, fmt.Errorf("unmarshal dataset timeframes: %w", unmarshalErr)
	}
	var sourceDataHashes []string
	if unmarshalErr := json.Unmarshal([]byte(model.SourceHashesJSON), &sourceDataHashes); unmarshalErr != nil {
		return domain.DatasetReference{}, fmt.Errorf("unmarshal dataset source hashes: %w", unmarshalErr)
	}
	metadata, hasMetadata, err := unmarshalOptionalMap(model.MetadataJSON)
	if err != nil {
		return domain.DatasetReference{}, fmt.Errorf("unmarshal dataset metadata: %w", err)
	}
	if !hasMetadata {
		metadata = nil
	}

	return domain.NewDatasetReference(domain.DatasetReferenceParams{
		DatasetID:        model.DatasetID,
		EntityTypes:      entityTypes,
		Instruments:      instruments,
		Timeframes:       timeframes,
		TimeRange:        domain.TimeRange{Start: model.TimeRangeStart.UTC(), End: model.TimeRangeEnd.UTC()},
		SourceDataHashes: sourceDataHashes,
		ReplayChecksum:   model.ReplayChecksum,
		CreatedAt:        model.CreatedAt.UTC(),
		Metadata:         metadata,
	})
}

func backtestRunToModel(run domain.BacktestRun) (backtestRunModel, error) {
	feeAssumptionsJSON, _, err := marshalOptionalJSON(run.FeeAssumptions)
	if err != nil {
		return backtestRunModel{}, fmt.Errorf("marshal fee assumptions: %w", err)
	}
	slippageAssumptionsJSON, _, err := marshalOptionalJSON(run.SlippageAssumptions)
	if err != nil {
		return backtestRunModel{}, fmt.Errorf("marshal slippage assumptions: %w", err)
	}
	metricsJSON, _, err := marshalOptionalJSON(run.Metrics)
	if err != nil {
		return backtestRunModel{}, fmt.Errorf("marshal metrics: %w", err)
	}

	return backtestRunModel{
		RunID:                     run.RunID.String(),
		StrategyID:                run.StrategyID,
		StrategyVersion:           run.StrategyVersion,
		StrategyArtifactHash:      run.StrategyArtifactHash,
		DatasetID:                 run.DatasetID.String(),
		GovernorPolicyID:          run.GovernorPolicyID,
		GovernorPolicyVersion:     run.GovernorPolicyVersion,
		GovernorPolicyHash:        run.GovernorPolicyHash,
		Mode:                      run.Mode.String(),
		TestedRangeStart:          run.TestedRange.Start.UTC(),
		TestedRangeEnd:            run.TestedRange.End.UTC(),
		FeeModelID:                run.FeeModelID,
		FeeAssumptionsJSON:        feeAssumptionsJSON,
		SlippageModelID:           run.SlippageModelID,
		SlippageAssumptionsJSON:   slippageAssumptionsJSON,
		ExecutionSimulatorVersion: run.ExecutionSimulatorVersion,
		Status:                    run.Status.String(),
		MetricsJSON:               metricsJSON,
		FailureReason:             run.FailureReason,
		FailureDetails:            run.FailureDetails,
		CreatedAt:                 run.CreatedAt.Time(),
		UpdatedAt:                 run.UpdatedAt.Time(),
	}, nil
}

func backtestRunFromModel(model backtestRunModel) (domain.BacktestRun, error) {
	feeAssumptions, hasFeeAssumptions, err := unmarshalOptionalMap(model.FeeAssumptionsJSON)
	if err != nil {
		return domain.BacktestRun{}, fmt.Errorf("unmarshal fee assumptions: %w", err)
	}
	slippageAssumptions, hasSlippageAssumptions, err := unmarshalOptionalMap(
		model.SlippageAssumptionsJSON,
	)
	if err != nil {
		return domain.BacktestRun{}, fmt.Errorf("unmarshal slippage assumptions: %w", err)
	}
	metrics, hasMetrics, err := unmarshalOptionalMetrics(model.MetricsJSON)
	if err != nil {
		return domain.BacktestRun{}, fmt.Errorf("unmarshal metrics: %w", err)
	}
	if !hasFeeAssumptions {
		feeAssumptions = nil
	}
	if !hasSlippageAssumptions {
		slippageAssumptions = nil
	}
	if !hasMetrics {
		metrics = nil
	}

	return domain.NewBacktestRun(domain.BacktestRunParams{
		RunID:                 model.RunID,
		StrategyID:            model.StrategyID,
		StrategyVersion:       model.StrategyVersion,
		StrategyArtifactHash:  model.StrategyArtifactHash,
		DatasetID:             model.DatasetID,
		GovernorPolicyID:      model.GovernorPolicyID,
		GovernorPolicyVersion: model.GovernorPolicyVersion,
		GovernorPolicyHash:    model.GovernorPolicyHash,
		Mode:                  domain.DecisionMode(model.Mode),
		TestedRange: domain.TimeRange{
			Start: model.TestedRangeStart.UTC(),
			End:   model.TestedRangeEnd.UTC(),
		},
		FeeModelID:                model.FeeModelID,
		FeeAssumptions:            feeAssumptions,
		SlippageModelID:           model.SlippageModelID,
		SlippageAssumptions:       slippageAssumptions,
		ExecutionSimulatorVersion: model.ExecutionSimulatorVersion,
		Status:                    domain.BacktestRunStatus(model.Status),
		Metrics:                   metrics,
		FailureReason:             model.FailureReason,
		FailureDetails:            model.FailureDetails,
		CreatedAt:                 model.CreatedAt.UTC(),
		UpdatedAt:                 model.UpdatedAt.UTC(),
	})
}

func evaluationReportToModel(report domain.EvaluationReport) (evaluationReportModel, error) {
	metricsJSON, _, err := marshalOptionalJSON(report.Metrics)
	if err != nil {
		return evaluationReportModel{}, fmt.Errorf("marshal metrics: %w", err)
	}
	failureReasonsJSON, _, err := marshalOptionalJSON(report.FailureReasons)
	if err != nil {
		return evaluationReportModel{}, fmt.Errorf("marshal failure reasons: %w", err)
	}

	return evaluationReportModel{
		EvaluationID:         report.EvaluationID.String(),
		StrategyID:           report.StrategyID,
		StrategyVersion:      report.StrategyVersion,
		StrategyArtifactHash: report.StrategyArtifactHash,
		BacktestRunID:        report.BacktestRunID.String(),
		DatasetID:            report.DatasetID.String(),
		Decision:             report.Decision.String(),
		MetricsJSON:          metricsJSON,
		FailureReasonsJSON:   failureReasonsJSON,
		Notes:                report.Notes,
		CreatedAt:            report.CreatedAt.Time(),
	}, nil
}

func evaluationReportFromModel(model evaluationReportModel) (domain.EvaluationReport, error) {
	metrics, hasMetrics, err := unmarshalOptionalMetrics(model.MetricsJSON)
	if err != nil {
		return domain.EvaluationReport{}, fmt.Errorf("unmarshal metrics: %w", err)
	}
	failureReasons, hasFailureReasons, err := unmarshalOptionalReasons(model.FailureReasonsJSON)
	if err != nil {
		return domain.EvaluationReport{}, fmt.Errorf("unmarshal failure reasons: %w", err)
	}
	if !hasMetrics {
		metrics = nil
	}
	if !hasFailureReasons {
		failureReasons = nil
	}

	return domain.NewEvaluationReport(domain.EvaluationReportParams{
		EvaluationID:         model.EvaluationID,
		StrategyID:           model.StrategyID,
		StrategyVersion:      model.StrategyVersion,
		StrategyArtifactHash: model.StrategyArtifactHash,
		BacktestRunID:        model.BacktestRunID,
		DatasetID:            model.DatasetID,
		Decision:             domain.EvaluationDecision(model.Decision),
		Metrics:              metrics,
		FailureReasons:       failureReasons,
		Notes:                model.Notes,
		CreatedAt:            model.CreatedAt.UTC(),
	})
}

func marshalJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func marshalOptionalJSON(value any) (*string, bool, error) {
	if value == nil || isNilJSONValue(value) {
		return nil, false, nil
	}
	payload, err := marshalJSON(value)
	if err != nil {
		return nil, false, err
	}

	return &payload, true, nil
}

func isNilJSONValue(value any) bool {
	reflectValue := reflect.ValueOf(value)
	kind := reflectValue.Kind()
	if kind == reflect.Pointer || kind == reflect.Map || kind == reflect.Slice {
		return reflectValue.IsNil()
	}

	return false
}

func unmarshalOptionalMap(value *string) (map[string]string, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(*value), &result); err != nil {
		return nil, false, err
	}

	return result, true, nil
}

func unmarshalOptionalMetrics(value *string) (*domain.VersionedMetrics, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	var result domain.VersionedMetrics
	if err := json.Unmarshal([]byte(*value), &result); err != nil {
		return nil, false, err
	}

	metrics, err := domain.NewVersionedMetrics(domain.VersionedMetricsParams(result))
	if err != nil {
		return nil, false, err
	}

	return metrics, true, nil
}

func unmarshalOptionalReasons(value *string) ([]string, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	var result []string
	if err := json.Unmarshal([]byte(*value), &result); err != nil {
		return nil, false, err
	}

	return result, true, nil
}

func datasetInstrumentRecords(instruments []domain.Instrument) []datasetInstrumentRecord {
	records := make([]datasetInstrumentRecord, 0, len(instruments))
	for _, instrument := range instruments {
		records = append(records, datasetInstrumentRecord{
			Venue:      instrument.Venue.String(),
			Symbol:     instrument.Symbol.String(),
			AssetClass: instrument.AssetClass.String(),
			Active:     instrument.Active,
		})
	}

	return records
}

func datasetInstrumentsFromRecords(records []datasetInstrumentRecord) ([]domain.Instrument, error) {
	instruments := make([]domain.Instrument, 0, len(records))
	for idx, record := range records {
		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue(record.Venue),
			Symbol:     domain.Symbol(record.Symbol),
			AssetClass: domain.AssetClass(record.AssetClass),
			Active:     record.Active,
		})
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", idx, err)
		}
		instruments = append(instruments, instrument)
	}

	return instruments, nil
}
