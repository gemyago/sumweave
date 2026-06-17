package jobs

import (
	"context"
	"crypto/sha256"
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
	columnErrorCode       = "error_code"
	columnErrorSummary    = "error_summary"
	columnErrorDetails    = "error_details"
	columnCompletedAt     = "completed_at"
	columnQueuedAt        = "queued_at"
	columnLastAttemptTime = "last_attempt_time"
	columnAttemptCount    = "attempt_count"
	columnResultJSON      = "result_json"
)

type Store struct {
	db        *gorm.DB
	tableName string
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
	CorrelationID      string     `gorm:"column:correlation_id;size:255;not null;default:''"`
}

func (jobModel) TableName() string { return "jobs" }

func NewStore(dsn string, opts StoreOpts) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("database dsn is required")
	}
	dialector := postgres.Open(dsn)
	trimmed := strings.TrimSpace(dsn)
	if trimmed == ":memory:" ||
		strings.HasPrefix(trimmed, "file:") ||
		strings.Contains(trimmed, "sqlite") ||
		strings.HasSuffix(trimmed, ".db") ||
		strings.HasSuffix(trimmed, ".sqlite") {
		dialector = sqlite.Open(dsn)
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

func (s *Store) AutoMigrate() error {
	return s.db.Table(s.tableName).AutoMigrate(&jobModel{})
}

func (s *Store) Create(ctx context.Context, job Job) (Job, error) {
	model, err := newJobModel(job)
	if err != nil {
		return Job{}, err
	}
	createErr := s.db.WithContext(ctx).Table(s.tableName).Create(&model).Error
	if createErr != nil {
		return Job{}, fmt.Errorf("create job: %w", createErr)
	}
	return jobFromModel(model)
}

func (s *Store) CreateIdempotent(ctx context.Context, job Job) (Job, bool, error) {
	model, err := newJobModel(job)
	if err != nil {
		return Job{}, false, err
	}
	if strings.TrimSpace(model.IdempotencyKey) == "" {
		return Job{}, false, ErrNoIdempotency
	}
	createErr := s.db.WithContext(ctx).Table(s.tableName).Create(&model).Error
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
		cursorTime, cursorID, err := decodeCursor(params.Cursor)
		if err != nil {
			return ListResult{}, fmt.Errorf("decode cursor: %w", err)
		}
		statement = statement.Where(
			"(created_at < ?) OR (created_at = ? AND id < ?)",
			cursorTime,
			cursorTime,
			cursorID,
		)
	}
	var models []jobModel
	if err := statement.Order("created_at DESC").Order("id DESC").Limit(params.Limit).Find(&models).Error; err != nil {
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
		result.NextCursor = encodeCursor(models[len(models)-1].CreatedAt.UTC(), models[len(models)-1].ID)
	}
	return result, nil
}

func (s *Store) ClaimQueued(
	ctx context.Context,
	jobID string,
	workerID string,
	claimedAt time.Time,
) (*Job, error) {
	claimedAt = claimedAt.UTC()
	result := s.db.WithContext(ctx).Table(s.tableName).Model(&jobModel{}).
		Where("id = ? AND status = ?", strings.TrimSpace(jobID), JobStatusQueued).
		Updates(map[string]any{
			columnStatus:          string(JobStatusRunning),
			columnWorkerID:        strings.TrimSpace(workerID),
			columnAttemptCount:    gorm.Expr("attempt_count + 1"),
			columnLastAttemptTime: claimedAt,
			columnStartedAt:       claimedAt,
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
	result HistoricalRawCandleBackfillResult,
	completedAt time.Time,
) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal job result: %w", err)
	}
	completedAt = completedAt.UTC()
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

func (s *Store) MarkFailed(
	ctx context.Context,
	jobID string,
	workerID string,
	jobErr *JobError,
	completedAt time.Time,
) error {
	completedAt = completedAt.UTC()
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
	now = now.UTC()
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

func newJobModel(job Job) (jobModel, error) {
	inputJSON, err := marshalHistoricalInput(job.Input)
	if err != nil {
		return jobModel{}, fmt.Errorf("marshal job input: %w", err)
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
		CreatedAt:          job.CreatedAt.UTC(),
		UpdatedAt:          job.UpdatedAt.UTC(),
		QueuedAt:           job.QueuedAt.UTC(),
		StartedAt:          job.StartedAt,
		CompletedAt:        job.CompletedAt,
		WorkerID:           strings.TrimSpace(job.WorkerID),
		AttemptCount:       job.AttemptCount,
		LastAttemptAt:      job.LastAttemptAt,
		CorrelationID:      strings.TrimSpace(job.CorrelationID),
	}
	if job.Result != nil {
		resultJSON, marshalErr := json.Marshal(job.Result)
		if marshalErr != nil {
			return jobModel{}, fmt.Errorf("marshal job result: %w", marshalErr)
		}
		model.ResultJSON = string(resultJSON)
	}
	if job.Error != nil {
		model.ErrorCode = truncateBounded(job.Error.Code, 128)
		model.ErrorSummary = truncateBounded(job.Error.Summary, maxErrorSummaryLength)
		model.ErrorDetails = truncateBounded(job.Error.Details, maxErrorDetailsLength)
	}
	return model, nil
}

func jobFromModel(model jobModel) (Job, error) {
	var input HistoricalRawCandleBackfillInput
	if err := json.Unmarshal([]byte(model.InputJSON), &input); err != nil {
		return Job{}, fmt.Errorf("unmarshal job input: %w", err)
	}
	input = canonicalizeHistoricalInput(input)
	var result *HistoricalRawCandleBackfillResult
	if strings.TrimSpace(model.ResultJSON) != "" {
		decoded := HistoricalRawCandleBackfillResult{}
		if err := json.Unmarshal([]byte(model.ResultJSON), &decoded); err != nil {
			return Job{}, fmt.Errorf("unmarshal job result: %w", err)
		}
		result = &decoded
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
		IdempotencyKey: model.IdempotencyKey,
		InputHash:      model.CanonicalInputHash,
		Input:          input,
		Result:         result,
		Error:          jobErr,
		CreatedAt:      model.CreatedAt.UTC(),
		UpdatedAt:      model.UpdatedAt.UTC(),
		QueuedAt:       model.QueuedAt.UTC(),
		StartedAt:      model.StartedAt,
		CompletedAt:    model.CompletedAt,
		WorkerID:       model.WorkerID,
		AttemptCount:   model.AttemptCount,
		LastAttemptAt:  model.LastAttemptAt,
		CorrelationID:  model.CorrelationID,
	}, nil
}

func HashInput(input HistoricalRawCandleBackfillInput) (string, error) {
	input.IngestionRunID = ""
	payload, err := marshalHistoricalInput(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func encodeCursor(createdAt time.Time, id string) string {
	payload := fmt.Sprintf("%s|%s", createdAt.UTC().Format(time.RFC3339Nano), id)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return parsed.UTC(), parts[1], nil
}
