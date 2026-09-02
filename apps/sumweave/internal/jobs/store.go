package jobs

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

var (
	ErrJobNotFound  = errors.New("job not found")
	ErrJobNotQueued = errors.New("job is not queued")
	ErrJobClaimLost = errors.New("job claim is no longer active")
)

const (
	columnStatus          = "status"
	columnWorkerID        = "worker_id"
	columnStartedAt       = "started_at"
	columnUpdatedAt       = "updated_at"
	columnErrorCode       = "error_code"
	columnErrorSummary    = "error_summary"
	columnErrorDetails    = "error_details"
	columnCompletedAt     = "completed_at"
	columnQueuedAt        = "queued_at"
	columnLastAttemptTime = "last_attempt_time"
	columnAttemptCount    = "attempt_count"
)

type Store struct {
	db        *gorm.DB
	tableName string
	migration schemaMigrator
}

type StoreOpts struct{ TablePrefix string }

type schemaMigrator interface {
	AutoMigrate(string) error
	DropTableIfExists(string) error
	DropColumnIfExists(string, string) error
}

type gormSchemaMigrator struct{ db *gorm.DB }

type terminalJobState struct {
	status      JobStatus
	workerID    string
	jobError    *JobError
	completedAt time.Time
}

type jobModel struct {
	ID                 string     `gorm:"column:id;size:255;not null;primaryKey"`
	JobType            string     `gorm:"column:job_type;size:64;not null;index:idx_jobs_type_status_created_id,priority:1"`
	Status             string     `gorm:"column:status;size:32;not null;index:idx_jobs_type_status_created_id,priority:2;index:idx_jobs_status_created_id,priority:1"`
	RequesterUserID    string     `gorm:"column:requester_user_id;size:255;not null;default:''"`
	RequesterSource    string     `gorm:"column:requester_source;size:32;not null;index:idx_jobs_source_created_id,priority:1"`
	ErrorCode          string     `gorm:"column:error_code;size:128"`
	ErrorSummary       string     `gorm:"column:error_summary;size:240"`
	ErrorDetails       string     `gorm:"column:error_details;size:1024"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null;index:idx_jobs_status_created_id,priority:2;index:idx_jobs_source_created_id,priority:2;index:idx_jobs_type_status_created_id,priority:3"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null"`
	QueuedAt           time.Time  `gorm:"column:queued_at;not null"`
	StartedAt          *time.Time `gorm:"column:started_at"`
	CompletedAt        *time.Time `gorm:"column:completed_at"`
	WorkerID           string     `gorm:"column:worker_id;size:255;not null;default:''"`
	AttemptCount       int        `gorm:"column:attempt_count;not null"`
	LastAttemptAt      *time.Time `gorm:"column:last_attempt_time"`
	ScheduleID         string     `gorm:"column:schedule_id;size:255;not null;default:''"`
	ScheduledAt        *time.Time `gorm:"column:scheduled_at"`
	ScheduledNextRunAt *time.Time `gorm:"column:scheduled_next_run_at"`
}

func (jobModel) TableName() string { return "jobs" }

func NewStore(sqlDB *sql.DB, dsn string, opts StoreOpts) (*Store, error) {
	if sqlDB == nil {
		return nil, errors.New("sql database is required")
	}
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("database dsn is required")
	}
	dialector := postgres.New(postgres.Config{DSN: dsn, Conn: sqlDB})
	if isSQLiteDSN(dsn) {
		dialector = sqlite.Dialector{DriverName: sqlite.DriverName, DSN: dsn, Conn: sqlDB}
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: opts.TablePrefix}, TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open jobs database: %w", err)
	}
	tableName := "jobs"
	if opts.TablePrefix != "" {
		tableName = opts.TablePrefix + tableName
	}
	return &Store{
		db:        db,
		tableName: tableName,
		migration: gormSchemaMigrator{db: db},
	}, nil
}

func isSQLiteDSN(dsn string) bool {
	trimmed := strings.TrimSpace(dsn)
	return trimmed == ":memory:" || strings.HasPrefix(trimmed, "file:") ||
		strings.Contains(trimmed, "sqlite") || strings.HasSuffix(trimmed, ".db") ||
		strings.HasSuffix(trimmed, ".sqlite")
}

// AutoMigrate explicitly removes alpha-only fields because GORM does not do so.
func (s *Store) AutoMigrate() error {
	if err := s.migration.AutoMigrate(s.tableName); err != nil {
		return fmt.Errorf("migrate jobs table: %w", err)
	}
	if err := s.migration.DropTableIfExists(
		s.scheduleTableName(),
	); err != nil {
		return err
	}
	for _, column := range []string{"agent_session_id", "agent_run_id", "idempotency_key", "canonical_input_hash", "input_json", "result_json", "progress_json", "max_attempts", "correlation_id"} {
		if err := s.migration.DropColumnIfExists(s.tableName, column); err != nil {
			return err
		}
	}
	return nil
}

// Concrete schema DDL is exercised by the serialized bootstrap migration smoke.
func (m gormSchemaMigrator) AutoMigrate(
	tableName string,
) error {
	return m.db.Table(tableName).AutoMigrate(&jobModel{})
}

func (m gormSchemaMigrator) DropTableIfExists(
	tableName string,
) error {
	migrator := m.db.Migrator()
	if !migrator.HasTable(tableName) {
		return nil
	}
	if err := migrator.DropTable(tableName); err != nil {
		return fmt.Errorf("drop obsolete table %s: %w", tableName, err)
	}
	return nil
}

func (m gormSchemaMigrator) DropColumnIfExists(
	tableName, column string,
) error {
	migrator := m.db.Table(tableName).Migrator()
	if !migrator.HasColumn(tableName, column) {
		return nil
	}
	if err := m.dropSQLiteIndexesUsingColumn(
		tableName,
		column,
	); err != nil {
		return err
	}
	statement := fmt.Sprintf(
		"ALTER TABLE %s DROP COLUMN %s",
		quoteIdentifier(tableName),
		quoteIdentifier(column),
	)
	if err := m.db.Exec(statement).Error; err != nil {
		return fmt.Errorf("drop obsolete %s column %s: %w", tableName, column, err)
	}
	return nil
}

func (m gormSchemaMigrator) dropSQLiteIndexesUsingColumn(
	tableName, column string,
) error {
	if m.db.Dialector.Name() != "sqlite" {
		return nil
	}
	migrator := m.db.Table(tableName).Migrator()
	indexes, err := migrator.GetIndexes(tableName)
	if err != nil {
		return fmt.Errorf("inspect SQLite indexes for obsolete %s column: %w", column, err)
	}
	for _, index := range indexes {
		for _, indexedColumn := range index.Columns() {
			if indexedColumn != column {
				continue
			}
			if dropErr := migrator.DropIndex(
				tableName,
				index.Name(),
			); dropErr != nil {
				return fmt.Errorf("drop SQLite index %s for obsolete %s column: %w", index.Name(), column, dropErr)
			}
			break
		}
	}
	return nil
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func (s *Store) createWithDB(ctx context.Context, db *gorm.DB, job Job) error {
	model := newJobModel(job)
	if err := db.WithContext(ctx).Table(s.tableName).Create(&model).Error; err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, jobID string) (*Job, error) {
	var model jobModel
	query := s.db.WithContext(ctx).Table(s.tableName).
		Where("id = ?", strings.TrimSpace(jobID)).First(&model)
	if err := query.Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf(
			"get job: %w",
			err,
		)
	}
	job := jobFromModel(model)
	return &job, nil
}

// MaterializeQueued creates the visibility projection for a delivery without
// altering an existing projection for a duplicate delivery.
func (s *Store) MaterializeQueued(ctx context.Context, job Job) (*Job, error) {
	model := newJobModel(job)
	create := s.db.WithContext(ctx).Table(s.tableName).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).
		Create(&model)
	if err := create.Error; err != nil {
		return nil, fmt.Errorf("materialize queued job: %w", err)
	}
	materialized, err := s.Get(ctx, job.ID)
	if err != nil {
		return nil, fmt.Errorf("get materialized job: %w", err)
	}
	return materialized, nil
}

func (s *Store) List(ctx context.Context, params ListParams) (ListResult, error) {
	params = normalizeListParams(params)
	statement := s.db.WithContext(ctx).Table(s.tableName).Model(&jobModel{})
	if len(params.Statuses) > 0 {
		statement = statement.Where("status IN ?", params.Statuses)
	}
	if len(params.JobTypes) > 0 {
		statement = statement.Where("job_type IN ?", params.JobTypes)
	}
	if len(
		params.Sources,
	) > 0 {
		statement = statement.Where("requester_source IN ?", params.Sources)
	}
	if params.Cursor != "" {
		createdAt, id, err := decodeCursor(params.Cursor)
		if err != nil {
			return ListResult{}, fmt.Errorf(
				"decode cursor: %w",
				err,
			)
		}
		statement = statement.Where(
			"(created_at < ?) OR (created_at = ? AND id < ?)",
			createdAt,
			createdAt,
			id,
		)
	}
	var models []jobModel
	findResult := statement.Order("created_at DESC").Order("id DESC").Limit(params.Limit).Find(&models)
	if err := findResult.Error; err != nil {
		return ListResult{}, fmt.Errorf("list jobs: %w", err)
	}
	result := ListResult{Items: make([]Job, 0, len(models))}
	for _, model := range models {
		result.Items = append(result.Items, jobFromModel(model))
	}
	if len(models) == params.Limit {
		result.NextCursor = encodeCursor(models[len(models)-1].CreatedAt, models[len(models)-1].ID)
	}
	return result, nil
}

func (s *Store) ClaimQueued(
	ctx context.Context,
	jobID, workerID string,
	claimedAt time.Time,
) (*Job, error) {
	if err := validateRequiredTimestamp(
		"claimedAt", claimedAt,
	); err != nil {
		return nil, err
	}
	result := s.db.WithContext(ctx).Table(s.tableName).Model(&jobModel{}).
		Where("id = ? AND status = ?", strings.TrimSpace(jobID), JobStatusQueued).
		Updates(map[string]any{
			columnStatus: string(JobStatusRunning), columnWorkerID: strings.TrimSpace(workerID),
			columnAttemptCount: gorm.Expr("attempt_count + 1"), columnLastAttemptTime: claimedAt,
			columnStartedAt: claimedAt, columnUpdatedAt: claimedAt,
		})
	if result.Error != nil {
		return nil, fmt.Errorf("claim queued job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrJobNotQueued
	}
	return s.Get(ctx, jobID)
}

func (s *Store) persistTerminalState(
	ctx context.Context,
	claim Job,
	state terminalJobState,
) error {
	updates := terminalStateUpdates(state)
	result := s.db.WithContext(ctx).
		Table(s.tableName).
		Model(&jobModel{}).
		Where(
			"id = ? AND status = ? AND worker_id = ? AND started_at = ?",
			strings.TrimSpace(claim.ID),
			JobStatusRunning,
			strings.TrimSpace(claim.WorkerID),
			claim.StartedAt,
		).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("persist terminal job state: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrJobClaimLost
	}
	return nil
}

func (s *Store) FinalizeRetryExhausted(
	ctx context.Context,
	jobID string,
	queuedAt time.Time,
	state terminalJobState,
) error {
	updates := terminalStateUpdates(state)
	result := s.db.WithContext(ctx).
		Table(s.tableName).
		Model(&jobModel{}).
		Where(
			"id = ? AND status = ? AND queued_at = ?",
			strings.TrimSpace(jobID),
			JobStatusQueued,
			queuedAt,
		).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("finalize exhausted job retries: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrJobClaimLost
	}
	return nil
}

func terminalStateUpdates(state terminalJobState) map[string]any {
	updates := map[string]any{
		columnStatus: string(state.status), columnWorkerID: strings.TrimSpace(state.workerID),
		columnErrorCode: "", columnErrorSummary: "", columnErrorDetails: "",
		columnCompletedAt: state.completedAt, columnUpdatedAt: state.completedAt,
	}
	if state.jobError != nil {
		updates[columnErrorCode] = truncateBounded(state.jobError.Code, 128)
		updates[columnErrorSummary] = truncateBounded(state.jobError.Summary, maxErrorSummaryLength)
		updates[columnErrorDetails] = truncateBounded(state.jobError.Details, maxErrorDetailsLength)
	}
	return updates
}

func (s *Store) RequeueRunning(ctx context.Context, claim Job, queuedAt time.Time) error {
	// Input validation is unit-tested at the worker boundary.
	if err := validateRequiredTimestamp("queuedAt", queuedAt); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Table(s.tableName).Model(&jobModel{}).
		Where(
			"id = ? AND status = ? AND worker_id = ? AND started_at = ?",
			strings.TrimSpace(claim.ID),
			JobStatusRunning,
			strings.TrimSpace(claim.WorkerID),
			claim.StartedAt,
		).
		Updates(map[string]any{
			columnStatus: string(JobStatusQueued), columnWorkerID: "", columnStartedAt: nil,
			columnQueuedAt: queuedAt, columnUpdatedAt: queuedAt,
		})
	// Driver requeue failure propagation.
	if result.Error != nil {
		return fmt.Errorf("requeue running job: %w", result.Error)
	}
	// A claim that has already been recovered cannot be requeued by its former owner.
	if result.RowsAffected == 0 {
		return ErrJobClaimLost
	}
	return nil
}

func (s *Store) RenewRunning(ctx context.Context, claim Job, renewedAt time.Time) error {
	if err := validateRequiredTimestamp("renewedAt", renewedAt); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Table(s.tableName).Model(&jobModel{}).
		Where(
			"id = ? AND status = ? AND worker_id = ? AND started_at = ?",
			strings.TrimSpace(claim.ID),
			JobStatusRunning,
			strings.TrimSpace(claim.WorkerID),
			claim.StartedAt,
		).
		Update(columnUpdatedAt, renewedAt)
	if result.Error != nil {
		return fmt.Errorf("renew running job claim: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrJobClaimLost
	}
	return nil
}

func newSucceededTerminalJobState(workerID string, completedAt time.Time) terminalJobState {
	return terminalJobState{
		status:      JobStatusSucceeded,
		workerID:    workerID,
		completedAt: completedAt,
	}
}

func newFailedTerminalJobState(
	workerID string,
	jobErr *JobError,
	completedAt time.Time,
) terminalJobState {
	return terminalJobState{
		status:      JobStatusFailed,
		workerID:    workerID,
		jobError:    jobErr,
		completedAt: completedAt,
	}
}

func (s *Store) RecoverStaleRunning(
	ctx context.Context,
	now time.Time,
	staleRunningAge time.Duration,
	maxAttempts int,
) error {
	// Worker supplies its validated clock value.
	if err := validateRequiredTimestamp("now", now); err != nil {
		return err
	}
	if staleRunningAge <= 0 {
		return errors.New("stale running age must be positive")
	}
	staleBefore := now.Add(-staleRunningAge)
	var models []jobModel
	// Driver recovery scan failure propagation.
	if err := s.db.WithContext(ctx).
		Table(s.tableName).
		Where(
			"status = ? AND updated_at <= ?",
			JobStatusRunning,
			staleBefore,
		).
		Find(&models).
		Error; err != nil {
		return fmt.Errorf("list stale running jobs: %w", err)
	}
	for _, model := range models {
		if err := s.recoverStaleRunningModel(ctx, model, now, maxAttempts); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) recoverStaleRunningModel(
	ctx context.Context,
	model jobModel,
	now time.Time,
	maxAttempts int,
) error {
	updates := map[string]any{
		columnStatus: string(JobStatusQueued), columnUpdatedAt: now, columnQueuedAt: now,
		columnStartedAt: nil, columnWorkerID: "", columnErrorCode: "stale_running_requeued",
		columnErrorSummary: "stale running job requeued",
		columnErrorDetails: "startup recovery requeued a stale running job",
	}
	if model.AttemptCount >= maxAttempts {
		updates = map[string]any{
			columnStatus: string(JobStatusFailed), columnUpdatedAt: now, columnCompletedAt: now,
			columnErrorCode:    "stale_running_attempts_exhausted",
			columnErrorSummary: "stale running job attempts exhausted",
			columnErrorDetails: "startup recovery marked job failed after attempts were exhausted",
		}
	}
	// Driver recovery write failure propagation.
	result := s.db.WithContext(ctx).
		Table(s.tableName).
		Model(&jobModel{}).
		Where(
			"id = ? AND status = ? AND worker_id = ? AND started_at = ? AND updated_at = ?",
			model.ID,
			JobStatusRunning,
			model.WorkerID,
			model.StartedAt,
			model.UpdatedAt,
		).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("recover stale running job %s: %w", model.ID, result.Error)
	}
	return nil
}

func (s *Store) scheduleTableName() string {
	return strings.TrimSuffix(s.tableName, "jobs") + "job_schedules"
}

func newJobModel(job Job) jobModel {
	model := jobModel{
		ID:                 strings.TrimSpace(job.ID),
		JobType:            string(job.JobType),
		Status:             string(job.Status),
		RequesterUserID:    strings.TrimSpace(job.Requester.UserID),
		RequesterSource:    strings.TrimSpace(string(job.Requester.Source)),
		CreatedAt:          job.CreatedAt,
		UpdatedAt:          job.UpdatedAt,
		QueuedAt:           job.QueuedAt,
		StartedAt:          job.StartedAt,
		CompletedAt:        job.CompletedAt,
		WorkerID:           strings.TrimSpace(job.WorkerID),
		AttemptCount:       job.AttemptCount,
		LastAttemptAt:      job.LastAttemptAt,
		ScheduleID:         strings.TrimSpace(job.ScheduleID),
		ScheduledAt:        job.ScheduledAt,
		ScheduledNextRunAt: job.ScheduledNextRunAt,
	}
	// Sanitized errors are covered by lifecycle mapping.
	if job.Error != nil {
		model.ErrorCode = truncateBounded(job.Error.Code, 128)
		model.ErrorSummary = truncateBounded(job.Error.Summary, maxErrorSummaryLength)
		model.ErrorDetails = truncateBounded(job.Error.Details, maxErrorDetailsLength)
	}
	return model
}
func jobFromModel(model jobModel) Job {
	var jobErr *JobError
	if model.ErrorCode != "" || model.ErrorSummary != "" || model.ErrorDetails != "" {
		jobErr = &JobError{
			Code:    model.ErrorCode,
			Summary: model.ErrorSummary,
			Details: model.ErrorDetails,
		}
	}
	return Job{
		ID:      model.ID,
		JobType: JobType(model.JobType),
		Status:  JobStatus(model.Status),
		Requester: Requester{
			UserID: model.RequesterUserID,
			Source: RequesterSource(model.RequesterSource),
		},
		Error:              jobErr,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		QueuedAt:           model.QueuedAt,
		StartedAt:          model.StartedAt,
		CompletedAt:        model.CompletedAt,
		WorkerID:           model.WorkerID,
		AttemptCount:       model.AttemptCount,
		LastAttemptAt:      model.LastAttemptAt,
		ScheduleID:         model.ScheduleID,
		ScheduledAt:        model.ScheduledAt,
		ScheduledNextRunAt: model.ScheduledNextRunAt,
	}
}
func encodeCursor(createdAt time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(createdAt.Format(time.RFC3339Nano) + "|" + id),
	)
}
func decodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 ||
		strings.TrimSpace(
			parts[1],
		) == "" {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", errors.New("invalid cursor timestamp")
	}
	return parsed, parts[1], nil
}
