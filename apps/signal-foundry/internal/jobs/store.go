package jobs

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	ErrJobNotFound   = errors.New("job not found")
	ErrJobNotQueued  = errors.New("job is not queued")
	ErrNoIdempotency = errors.New("idempotency key is empty")
)

const (
	columnStatus          = "status"
	columnWorkerID        = "worker_id"
	columnStartedAt       = "started_at"
	columnUpdatedAt       = "updated_at"
	columnProgressJSON    = "progress_json"
	columnErrorCode       = "error_code"
	columnErrorSummary    = "error_summary"
	columnErrorDetails    = "error_details"
	columnCompletedAt     = "completed_at"
	columnQueuedAt        = "queued_at"
	columnLastAttemptTime = "last_attempt_time"
	columnAttemptCount    = "attempt_count"
	columnMaxAttempts     = "max_attempts"
	columnResultJSON      = "result_json"
)

type Store struct {
	db        *gorm.DB
	tableName string
}

type StoreTx struct {
	db        *gorm.DB
	tableName string
	sqlTx     *sql.Tx
}

type StoreOpts struct {
	TablePrefix string
}

type jobModel struct {
	ID                 string     `gorm:"column:id;size:255;not null;primaryKey"`
	JobType            string     `gorm:"column:job_type;size:64;not null;index:idx_jobs_type_status_created_id,priority:1;index:idx_jobs_idempotency,unique,where:idempotency_key <> '',priority:3"`
	Status             string     `gorm:"column:status;size:32;not null;index:idx_jobs_type_status_created_id,priority:2;index:idx_jobs_status_created_id,priority:1"`
	RequesterUserID    string     `gorm:"column:requester_user_id;size:255;not null;default:'';index:idx_jobs_idempotency,unique,where:idempotency_key <> '',priority:1"`
	RequesterSource    string     `gorm:"column:requester_source;size:32;not null;index:idx_jobs_source_created_id,priority:1;index:idx_jobs_idempotency,unique,where:idempotency_key <> '',priority:2"`
	AgentSessionID     string     `gorm:"column:agent_session_id;size:255;not null;default:''"`
	AgentRunID         string     `gorm:"column:agent_run_id;size:255;not null;default:''"`
	IdempotencyKey     string     `gorm:"column:idempotency_key;size:255;not null;default:'';index:idx_jobs_idempotency,unique,where:idempotency_key <> '',priority:4"`
	CanonicalInputHash string     `gorm:"column:canonical_input_hash;size:64;not null"`
	InputJSON          string     `gorm:"column:input_json;type:text;not null"`
	ResultJSON         string     `gorm:"column:result_json;type:text"`
	ProgressJSON       string     `gorm:"column:progress_json;type:text"`
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
	MaxAttempts        int        `gorm:"column:max_attempts;not null"`
	LastAttemptAt      *time.Time `gorm:"column:last_attempt_time"`
	CorrelationID      string     `gorm:"column:correlation_id;size:255;not null;default:''"`
	ScheduleID         string     `gorm:"column:schedule_id;size:255;not null;default:''"`
	ScheduledAt        *time.Time `gorm:"column:scheduled_at"`
	ScheduledNextRunAt *time.Time `gorm:"column:scheduled_next_run_at"`
}

func (jobModel) TableName() string { return "jobs" }

type scheduleModel struct {
	ID              string     `gorm:"column:id;size:255;not null;primaryKey"`
	JobType         string     `gorm:"column:job_type;size:128;not null"`
	RequesterUserID string     `gorm:"column:requester_user_id;size:255;not null;default:''"`
	RequesterSource string     `gorm:"column:requester_source;size:32;not null"`
	AgentSessionID  string     `gorm:"column:agent_session_id;size:255;not null;default:''"`
	AgentRunID      string     `gorm:"column:agent_run_id;size:255;not null;default:''"`
	InputJSON       string     `gorm:"column:input_json;type:text;not null"`
	IntervalSeconds int64      `gorm:"column:interval_seconds;not null"`
	NextRunAt       *time.Time `gorm:"column:next_run_at;index:idx_job_schedules_next_run_at"`
	LastEnqueuedAt  *time.Time `gorm:"column:last_enqueued_at"`
	CorrelationID   string     `gorm:"column:correlation_id;size:255;not null;default:''"`
	Enabled         bool       `gorm:"column:enabled;not null"`
}

func (scheduleModel) TableName() string { return "job_schedules" }

func NewStore(sqlDB *sql.DB, dsn string, opts StoreOpts) (*Store, error) {
	if sqlDB == nil {
		return nil, errors.New("sql database is required")
	}
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("database dsn is required")
	}
	trimmed := strings.TrimSpace(dsn)
	dialector := postgres.New(postgres.Config{DSN: dsn, Conn: sqlDB})
	if isSQLiteDSN(trimmed) {
		dialector = sqlite.Dialector{DriverName: sqlite.DriverName, DSN: dsn, Conn: sqlDB}
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: opts.TablePrefix},
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open jobs database: %w", err)
	}
	tableName := "jobs"
	if opts.TablePrefix != "" {
		tableName = opts.TablePrefix + tableName
	}
	return &Store{db: db, tableName: tableName}, nil
}

func isSQLiteDSN(dsn string) bool {
	trimmed := strings.TrimSpace(dsn)
	return trimmed == ":memory:" ||
		strings.HasPrefix(trimmed, "file:") ||
		strings.Contains(trimmed, "sqlite") ||
		strings.HasSuffix(trimmed, ".db") ||
		strings.HasSuffix(trimmed, ".sqlite")
}

func (s *Store) AutoMigrate() error {
	if err := s.db.Table(s.tableName).AutoMigrate(&jobModel{}); err != nil {
		return err
	}
	return s.db.Table(s.scheduleTableName()).AutoMigrate(&scheduleModel{})
}

func (s *Store) Create(ctx context.Context, job Job) (Job, error) {
	return s.createWithDB(ctx, s.db.WithContext(ctx), job)
}

func (s *Store) createWithDB(ctx context.Context, db *gorm.DB, job Job) (Job, error) {
	model, err := newJobModel(job)
	if err != nil {
		return Job{}, err
	}
	createErr := db.WithContext(ctx).Table(s.tableName).Create(&model).Error
	if createErr != nil {
		return Job{}, fmt.Errorf("create job: %w", createErr)
	}
	return jobFromModel(model)
}

func (s *Store) CreateIdempotent(ctx context.Context, job Job) (Job, bool, error) {
	return s.createIdempotentWithDB(ctx, s.db.WithContext(ctx), job)
}

func (s *Store) createIdempotentWithDB(ctx context.Context, db *gorm.DB, job Job) (Job, bool, error) {
	model, err := newJobModel(job)
	if err != nil {
		return Job{}, false, err
	}
	if strings.TrimSpace(model.IdempotencyKey) == "" {
		return Job{}, false, ErrNoIdempotency
	}
	createErr := db.WithContext(ctx).Table(s.tableName).Create(&model).Error
	if createErr == nil {
		created, jobErr := jobFromModel(model)
		if jobErr != nil {
			return Job{}, false, jobErr
		}
		return created, true, nil
	}
	if !errors.Is(createErr, gorm.ErrDuplicatedKey) {
		return Job{}, false, fmt.Errorf("create idempotent job: %w", createErr)
	}
	existing, findErr := s.FindByIdempotencyKey(
		ctx,
		Requester{
			UserID:         model.RequesterUserID,
			Source:         RequesterSource(model.RequesterSource),
			AgentSessionID: model.AgentSessionID,
			AgentRunID:     model.AgentRunID,
		},
		JobType(model.JobType),
		model.IdempotencyKey,
	)
	if findErr != nil {
		return Job{}, false, fmt.Errorf("load duplicate idempotent job: %w", findErr)
	}
	if existing.InputHash != model.CanonicalInputHash {
		return Job{}, false, &idempotencyConflictError{key: model.IdempotencyKey}
	}
	return *existing, false, nil
}

func (s *Store) WithTx(ctx context.Context, run func(*StoreTx) error) error {
	if run == nil {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sqlTx, ok := tx.Statement.ConnPool.(*sql.Tx)
		if !ok || sqlTx == nil {
			return errors.New("resolve sql transaction")
		}
		return run(&StoreTx{db: tx, tableName: s.tableName, sqlTx: sqlTx})
	})
}

func (tx *StoreTx) SQLTx() *sql.Tx {
	if tx == nil {
		return nil
	}
	return tx.sqlTx
}

func (tx *StoreTx) Create(ctx context.Context, job Job) (Job, error) {
	store := Store{tableName: tx.tableName}
	return store.createWithDB(ctx, tx.db, job)
}

func (tx *StoreTx) CreateIdempotent(ctx context.Context, job Job) (Job, bool, error) {
	store := Store{db: tx.db, tableName: tx.tableName}
	return store.createIdempotentWithDB(ctx, tx.db, job)
}

func (tx *StoreTx) UpsertSchedule(ctx context.Context, schedule Schedule) error {
	store := Store{db: tx.db, tableName: tx.tableName}
	return store.upsertScheduleWithDB(ctx, tx.db, schedule)
}

func (s *Store) Get(ctx context.Context, jobID string) (*Job, error) {
	var model jobModel
	if err := s.db.WithContext(ctx).
		Table(s.tableName).
		Where("id = ?", strings.TrimSpace(jobID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	job, err := jobFromModel(model)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) FindByIdempotencyKey(
	ctx context.Context,
	requester Requester,
	jobType JobType,
	idempotencyKey string,
) (*Job, error) {
	requester = canonicalizeRequester(requester)
	trimmedKey := strings.TrimSpace(idempotencyKey)
	if trimmedKey == "" {
		return nil, ErrNoIdempotency
	}
	var model jobModel
	err := s.db.WithContext(ctx).Table(s.tableName).Where(
		"requester_user_id = ? AND requester_source = ? AND job_type = ? AND idempotency_key = ?",
		requester.UserID,
		requester.Source,
		jobType,
		trimmedKey,
	).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("find job by idempotency key: %w", err)
	}
	job, err := jobFromModel(model)
	if err != nil {
		return nil, err
	}
	return &job, nil
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
	if len(params.Sources) > 0 {
		statement = statement.Where("requester_source IN ?", params.Sources)
	}
	if params.Cursor != "" {
		cursorCreatedAt, cursorID, err := decodeCursor(params.Cursor)
		if err != nil {
			return ListResult{}, fmt.Errorf("decode cursor: %w", err)
		}
		statement = statement.Where(
			"(created_at < ?) OR (created_at = ? AND id < ?)",
			cursorCreatedAt,
			cursorCreatedAt,
			cursorID,
		)
	}
	var models []jobModel
	if err := statement.
		Order("created_at DESC").
		Order("id DESC").
		Limit(params.Limit).
		Find(&models).Error; err != nil {
		return ListResult{}, fmt.Errorf("list jobs: %w", err)
	}
	items := make([]Job, 0, len(models))
	for _, model := range models {
		job, err := jobFromModel(model)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, job)
	}
	result := ListResult{Items: items}
	if len(models) == params.Limit {
		result.NextCursor = encodeCursor(models[len(models)-1].CreatedAt, models[len(models)-1].ID)
	}
	return result, nil
}

func (s *Store) ClaimQueued(
	ctx context.Context,
	jobID string,
	workerID string,
	claimedAt time.Time,
) (*Job, error) {
	if err := validateRequiredTimestamp("claimedAt", claimedAt); err != nil {
		return nil, err
	}
	result := s.db.WithContext(ctx).Table(s.tableName).Model(&jobModel{}).
		Where("id = ? AND status = ?", strings.TrimSpace(jobID), JobStatusQueued).
		Updates(map[string]any{
			columnStatus:          string(JobStatusRunning),
			columnWorkerID:        strings.TrimSpace(workerID),
			columnAttemptCount:    gorm.Expr("attempt_count + 1"),
			columnLastAttemptTime: claimedAt,
			columnStartedAt:       claimedAt,
			columnProgressJSON:    "",
			columnUpdatedAt:       claimedAt,
		})
	if result.Error != nil {
		return nil, fmt.Errorf("claim queued job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrJobNotQueued
	}
	return s.Get(ctx, jobID)
}

func (s *Store) MarkSucceeded(
	ctx context.Context,
	jobID string,
	workerID string,
	result any,
	completedAt time.Time,
) error {
	if err := validateRequiredTimestamp("completedAt", completedAt); err != nil {
		return err
	}
	resultJSON, err := resultJSONFromValue(result)
	if err != nil {
		return err
	}
	updates := map[string]any{
		columnStatus:       string(JobStatusSucceeded),
		columnWorkerID:     strings.TrimSpace(workerID),
		columnResultJSON:   string(resultJSON),
		columnErrorCode:    "",
		columnErrorSummary: "",
		columnErrorDetails: "",
		columnCompletedAt:  completedAt,
		columnUpdatedAt:    completedAt,
	}
	updateErr := s.db.WithContext(ctx).Table(s.tableName).
		Model(&jobModel{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(updates).Error
	if updateErr != nil {
		return fmt.Errorf("mark job succeeded: %w", updateErr)
	}
	return nil
}

func resultJSONFromValue(result any) (json.RawMessage, error) {
	switch typed := result.(type) {
	case nil:
		return nil, nil
	case json.RawMessage:
		return typed, nil
	case []byte:
		return json.RawMessage(typed), nil
	default:
		encoded, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal job result: %w", err)
		}
		return json.RawMessage(encoded), nil
	}
}

func (s *Store) MarkCanceled(ctx context.Context, jobID string, completedAt time.Time) error {
	if err := validateRequiredTimestamp("completedAt", completedAt); err != nil {
		return err
	}
	updateErr := s.db.WithContext(ctx).Table(s.tableName).
		Model(&jobModel{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(map[string]any{
			columnStatus:      string(JobStatusCanceled),
			columnCompletedAt: completedAt,
			columnUpdatedAt:   completedAt,
		}).Error
	if updateErr != nil {
		return fmt.Errorf("mark job canceled: %w", updateErr)
	}
	return nil
}

func (s *Store) UpdateProgress(
	ctx context.Context,
	jobID string,
	progressJSON json.RawMessage,
	updatedAt time.Time,
) error {
	if err := validateRequiredTimestamp("updatedAt", updatedAt); err != nil {
		return err
	}
	updateErr := s.db.WithContext(ctx).Table(s.tableName).
		Model(&jobModel{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(map[string]any{columnProgressJSON: string(progressJSON), columnUpdatedAt: updatedAt}).Error
	if updateErr != nil {
		return fmt.Errorf("update job progress: %w", updateErr)
	}
	return nil
}

func (s *Store) MarkFailed(
	ctx context.Context,
	jobID string,
	workerID string,
	jobErr *JobError,
	completedAt time.Time,
) error {
	if err := validateRequiredTimestamp("completedAt", completedAt); err != nil {
		return err
	}
	updates := map[string]any{
		columnStatus:       string(JobStatusFailed),
		columnWorkerID:     strings.TrimSpace(workerID),
		columnErrorCode:    "",
		columnErrorSummary: "",
		columnErrorDetails: "",
		columnCompletedAt:  completedAt,
		columnUpdatedAt:    completedAt,
	}
	if jobErr != nil {
		updates[columnErrorCode] = truncateBounded(jobErr.Code, 128)
		updates[columnErrorSummary] = truncateBounded(jobErr.Summary, maxErrorSummaryLength)
		updates[columnErrorDetails] = truncateBounded(jobErr.Details, maxErrorDetailsLength)
	}
	updateErr := s.db.WithContext(ctx).Table(s.tableName).
		Model(&jobModel{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(updates).Error
	if updateErr != nil {
		return fmt.Errorf("mark job failed: %w", updateErr)
	}
	return nil
}

func (s *Store) RecoverStaleRunning(ctx context.Context, now time.Time, maxAttempts int) error {
	if err := validateRequiredTimestamp("now", now); err != nil {
		return err
	}
	var models []jobModel
	if err := s.db.WithContext(ctx).
		Table(s.tableName).
		Where("status = ?", JobStatusRunning).
		Find(&models).Error; err != nil {
		return fmt.Errorf("list stale running jobs: %w", err)
	}
	for _, model := range models {
		var updates map[string]any
		if model.AttemptCount >= maxAttempts {
			updates = map[string]any{
				columnStatus:       string(JobStatusFailed),
				columnUpdatedAt:    now,
				columnCompletedAt:  now,
				columnErrorCode:    "stale_running_attempts_exhausted",
				columnErrorSummary: "stale running job attempts exhausted",
				columnErrorDetails: "startup recovery marked job failed after attempts were exhausted",
			}
		} else {
			updates = map[string]any{
				columnStatus:       string(JobStatusQueued),
				columnUpdatedAt:    now,
				columnQueuedAt:     now,
				columnStartedAt:    nil,
				columnWorkerID:     "",
				columnProgressJSON: "",
				columnErrorCode:    "stale_running_requeued",
				columnErrorSummary: "stale running job requeued",
				columnErrorDetails: "startup recovery requeued a stale running job",
			}
		}
		updateErr := s.db.WithContext(ctx).Table(s.tableName).
			Model(&jobModel{}).
			Where("id = ?", model.ID).
			Updates(updates).Error
		if updateErr != nil {
			return fmt.Errorf("recover stale running job %s: %w", model.ID, updateErr)
		}
	}
	return nil
}

func (s *Store) UpsertSchedule(ctx context.Context, schedule Schedule) error {
	return s.upsertScheduleWithDB(ctx, s.db, schedule)
}

func (s *Store) upsertScheduleWithDB(ctx context.Context, db *gorm.DB, schedule Schedule) error {
	if !schedule.Enabled {
		schedule.NextRunAt = nil
	}
	if err := validateScheduleTimestamps(schedule); err != nil {
		return err
	}
	model := scheduleModel{
		ID:              strings.TrimSpace(schedule.ID),
		JobType:         string(schedule.JobType),
		RequesterUserID: strings.TrimSpace(schedule.Requester.UserID),
		RequesterSource: strings.TrimSpace(string(schedule.Requester.Source)),
		AgentSessionID:  strings.TrimSpace(schedule.Requester.AgentSessionID),
		AgentRunID:      strings.TrimSpace(schedule.Requester.AgentRunID),
		InputJSON:       string(schedule.InputJSON),
		IntervalSeconds: int64(schedule.Interval / time.Second),
		NextRunAt:       schedule.NextRunAt,
		LastEnqueuedAt:  schedule.LastEnqueuedAt,
		CorrelationID:   strings.TrimSpace(schedule.CorrelationID),
		Enabled:         schedule.Enabled,
	}
	if err := db.WithContext(ctx).Table(s.scheduleTableName()).Save(&model).Error; err != nil {
		return fmt.Errorf("upsert schedule: %w", err)
	}
	return nil
}

func (s *Store) ListDueSchedules(ctx context.Context, now time.Time) ([]Schedule, error) {
	var models []scheduleModel
	duePredicate := "next_run_at IS NULL OR next_run_at <= ?"
	if s.db.Dialector.Name() == "sqlite" {
		duePredicate = "next_run_at IS NULL OR julianday(next_run_at) <= julianday(?)"
	}
	if err := s.db.WithContext(ctx).
		Table(s.scheduleTableName()).
		Where("enabled = ?", true).
		Where(duePredicate, now).
		Order("next_run_at ASC").
		Order("id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list due schedules: %w", err)
	}
	items := make([]Schedule, 0, len(models))
	for _, model := range models {
		schedule, err := scheduleFromModel(model)
		if err != nil {
			return nil, err
		}
		if schedule.NextRunAt.After(now) {
			continue
		}
		items = append(items, schedule)
	}
	return items, nil
}

func (s *Store) scheduleTableName() string {
	return strings.TrimSuffix(s.tableName, "jobs") + "job_schedules"
}

func newJobModel(job Job) (jobModel, error) {
	inputJSON := job.InputJSON
	if len(inputJSON) == 0 && job.JobType == JobTypeHistoricalRawCandleBackfill {
		marshaled, err := marshalHistoricalInput(job.Input)
		if err != nil {
			return jobModel{}, fmt.Errorf("marshal job input: %w", err)
		}
		inputJSON = marshaled
	}
	resultJSON := job.ResultJSON
	if len(resultJSON) == 0 && job.Result != nil {
		marshaled, err := json.Marshal(job.Result)
		if err != nil {
			return jobModel{}, fmt.Errorf("marshal job result: %w", err)
		}
		resultJSON = marshaled
	}
	model := jobModel{
		ID:                 strings.TrimSpace(job.ID),
		JobType:            string(job.JobType),
		Status:             string(job.Status),
		RequesterUserID:    strings.TrimSpace(job.Requester.UserID),
		RequesterSource:    strings.TrimSpace(string(job.Requester.Source)),
		AgentSessionID:     strings.TrimSpace(job.Requester.AgentSessionID),
		AgentRunID:         strings.TrimSpace(job.Requester.AgentRunID),
		IdempotencyKey:     strings.TrimSpace(job.IdempotencyKey),
		CanonicalInputHash: strings.TrimSpace(job.InputHash),
		InputJSON:          string(inputJSON),
		ResultJSON:         string(resultJSON),
		ProgressJSON:       string(job.ProgressJSON),
		CreatedAt:          job.CreatedAt,
		UpdatedAt:          job.UpdatedAt,
		QueuedAt:           job.QueuedAt,
		StartedAt:          job.StartedAt,
		CompletedAt:        job.CompletedAt,
		WorkerID:           strings.TrimSpace(job.WorkerID),
		AttemptCount:       job.AttemptCount,
		MaxAttempts:        job.MaxAttempts,
		LastAttemptAt:      job.LastAttemptAt,
		CorrelationID:      strings.TrimSpace(job.CorrelationID),
		ScheduleID:         strings.TrimSpace(job.ScheduleID),
		ScheduledAt:        job.ScheduledAt,
		ScheduledNextRunAt: job.ScheduledNextRunAt,
	}
	if job.Error != nil {
		model.ErrorCode = truncateBounded(job.Error.Code, 128)
		model.ErrorSummary = truncateBounded(job.Error.Summary, maxErrorSummaryLength)
		model.ErrorDetails = truncateBounded(job.Error.Details, maxErrorDetailsLength)
	}
	return model, nil
}

func jobFromModel(model jobModel) (Job, error) {
	var historicalInput HistoricalRawCandleBackfillInput
	var historicalResult *HistoricalRawCandleBackfillResult
	if JobType(model.JobType) == JobTypeHistoricalRawCandleBackfill && strings.TrimSpace(model.InputJSON) != "" {
		if err := json.Unmarshal([]byte(model.InputJSON), &historicalInput); err != nil {
			return Job{}, fmt.Errorf("unmarshal job input: %w", err)
		}
		historicalInput = canonicalizeHistoricalInput(historicalInput)
	}
	if JobType(model.JobType) == JobTypeHistoricalRawCandleBackfill && strings.TrimSpace(model.ResultJSON) != "" {
		decoded := HistoricalRawCandleBackfillResult{}
		if err := json.Unmarshal([]byte(model.ResultJSON), &decoded); err != nil {
			return Job{}, fmt.Errorf("unmarshal job result: %w", err)
		}
		historicalResult = &decoded
	}
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
			UserID:         model.RequesterUserID,
			Source:         RequesterSource(model.RequesterSource),
			AgentSessionID: model.AgentSessionID,
			AgentRunID:     model.AgentRunID,
		},
		IdempotencyKey:     model.IdempotencyKey,
		InputHash:          model.CanonicalInputHash,
		Input:              historicalInput,
		InputJSON:          json.RawMessage(model.InputJSON),
		Result:             historicalResult,
		ResultJSON:         json.RawMessage(model.ResultJSON),
		ProgressJSON:       json.RawMessage(model.ProgressJSON),
		Error:              jobErr,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		QueuedAt:           model.QueuedAt,
		StartedAt:          model.StartedAt,
		CompletedAt:        model.CompletedAt,
		WorkerID:           model.WorkerID,
		AttemptCount:       model.AttemptCount,
		MaxAttempts:        model.MaxAttempts,
		LastAttemptAt:      model.LastAttemptAt,
		CorrelationID:      model.CorrelationID,
		ScheduleID:         model.ScheduleID,
		ScheduledAt:        model.ScheduledAt,
		ScheduledNextRunAt: model.ScheduledNextRunAt,
	}, nil
}

func HashInput(input HistoricalRawCandleBackfillInput) (string, error) {
	input.IngestionRunID = ""
	payload, err := marshalHistoricalInput(input)
	if err != nil {
		return "", err
	}
	return hashBytes(payload), nil
}

func hashBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func encodeCursor(createdAt time.Time, id string) string {
	payload := createdAt.Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", errors.New("invalid cursor timestamp")
	}
	return parsed, parts[1], nil
}

func scheduleFromModel(model scheduleModel) (Schedule, error) {
	schedule := Schedule{
		ID:      model.ID,
		JobType: JobType(model.JobType),
		Requester: Requester{
			UserID:         model.RequesterUserID,
			Source:         RequesterSource(model.RequesterSource),
			AgentSessionID: model.AgentSessionID,
			AgentRunID:     model.AgentRunID,
		},
		InputJSON:      json.RawMessage(model.InputJSON),
		Interval:       time.Duration(model.IntervalSeconds) * time.Second,
		NextRunAt:      model.NextRunAt,
		LastEnqueuedAt: model.LastEnqueuedAt,
		CorrelationID:  model.CorrelationID,
		Enabled:        model.Enabled,
	}
	if err := validateScheduleTimestamps(schedule); err != nil {
		return Schedule{}, fmt.Errorf("map schedule row: %w", err)
	}
	return schedule, nil
}

func validateScheduleTimestamps(schedule Schedule) error {
	if schedule.Enabled && schedule.NextRunAt == nil {
		return errors.New("enabled schedule nextRunAt is required")
	}
	if !schedule.Enabled && schedule.NextRunAt != nil {
		return errors.New("disabled schedule nextRunAt must be empty")
	}
	if schedule.NextRunAt != nil && schedule.NextRunAt.IsZero() {
		return errors.New("nextRunAt must be a non-zero timestamp")
	}
	if schedule.LastEnqueuedAt != nil && schedule.LastEnqueuedAt.IsZero() {
		return errors.New("lastEnqueuedAt must be a non-zero timestamp")
	}
	return nil
}

func validateRequiredTimestamp(field string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("%s must be a non-zero timestamp", field)
	}
	return nil
}
