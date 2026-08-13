package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	JobStatusQueued           JobStatus       = "queued"
	JobStatusRunning          JobStatus       = "running"
	JobStatusSucceeded        JobStatus       = "succeeded"
	JobStatusFailed           JobStatus       = "failed"
	JobStatusCanceled         JobStatus       = "canceled"
	RequesterSourceOperator   RequesterSource = "operator"
	RequesterSourceAgent      RequesterSource = "agent"
	defaultListLimit                          = 25
	maxListLimit                              = 100
	defaultWorkerPollInterval                 = 2 * time.Second
	defaultWorkerMaxAttempts                  = 3
	maxErrorSummaryLength                     = 240
	maxErrorDetailsLength                     = 1024
	jobExecutionTopic                         = "jobs.execution.v1"
	jobConsumerGroup                          = "jobs.workers.v1"
	jobEnvelopeVersion                        = "v1"
)

type JobType string
type JobStatus string
type RequesterSource string
type executionKind string
type executionEnvelope struct {
	Version         string          `json:"version"`
	Kind            executionKind   `json:"kind"`
	Payload         json.RawMessage `json:"payload"`
	ObservableJobID string          `json:"observableJobId,omitempty"`
	CorrelationID   string          `json:"correlationId,omitempty"`
	RequesterID     string          `json:"requesterId,omitempty"`
	RequesterSource string          `json:"requesterSource,omitempty"`
}
type Requester struct {
	UserID         string
	Source         RequesterSource
	AgentSessionID string
	AgentRunID     string
}
type JobError struct {
	Code    string
	Summary string
	Details string
}
type Job struct {
	ID                 string
	JobType            JobType
	Status             JobStatus
	Requester          Requester
	IdempotencyKey     string
	InputHash          string
	InputJSON          json.RawMessage
	ResultJSON         json.RawMessage
	ProgressJSON       json.RawMessage
	Error              *JobError
	CreatedAt          time.Time
	UpdatedAt          time.Time
	QueuedAt           time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
	WorkerID           string
	AttemptCount       int
	MaxAttempts        int
	LastAttemptAt      *time.Time
	CorrelationID      string
	ScheduleID         string
	ScheduledAt        *time.Time
	ScheduledNextRunAt *time.Time
}
type ListParams struct {
	Statuses   []JobStatus
	JobTypes   []JobType
	Sources    []RequesterSource
	Limit      int
	Cursor     string
	JobIDsOnly bool
}
type ListResult struct {
	Items      []Job
	NextCursor string
}
type EnqueueParams struct {
	JobType        JobType
	Requester      Requester
	IdempotencyKey string
	CorrelationID  string
	ScheduleID     string
	Input          any
}
type EnqueueJSONParams struct {
	JobType            JobType
	Requester          Requester
	IdempotencyKey     string
	CorrelationID      string
	ScheduleID         string
	ScheduledAt        *time.Time
	ScheduledNextRunAt *time.Time
	InputHash          string
	InputJSON          json.RawMessage
}
type Schedule struct {
	ID             string
	JobType        JobType
	Requester      Requester
	InputJSON      json.RawMessage
	Interval       time.Duration
	NextRunAt      *time.Time
	LastEnqueuedAt *time.Time
	CorrelationID  string
	Enabled        bool
}
type WorkerConfig struct {
	Enabled      bool
	PollInterval time.Duration
	MaxAttempts  int
}
type idempotencyConflictError struct{ key string }

func (e *idempotencyConflictError) Error() string {
	return fmt.Sprintf("idempotency key conflict: %s", e.key)
}
func (e *idempotencyConflictError) Code() string { return "idempotency_key_conflict" }
func IsIdempotencyConflict(err error) bool {
	var target *idempotencyConflictError
	return errors.As(err, &target)
}
func normalizeWorkerConfig(cfg WorkerConfig) WorkerConfig {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultWorkerPollInterval
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultWorkerMaxAttempts
	}
	return cfg
}
func EncodeJobPayload[T any](payload T) (json.RawMessage, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

//nolint:ireturn // Typed job payload decoding returns the requested concrete value.
func DecodeJobInput[T any](job Job) (T, error) {
	return decodeJobPayload[T](job.InputJSON)
}

//nolint:ireturn // Typed job payload decoding returns the requested concrete value.
func DecodeJobResult[T any](job Job) (T, error) {
	return decodeJobPayload[T](job.ResultJSON)
}

//nolint:ireturn // Typed job payload decoding returns the requested concrete value.
func DecodeJobProgress[T any](job Job) (T, error) {
	return decodeJobPayload[T](job.ProgressJSON)
}

//nolint:ireturn // Typed job payload decoding returns the requested concrete value.
func decodeJobPayload[T any](payload json.RawMessage) (T, error) {
	var decoded T
	if len(payload) == 0 {
		return decoded, nil
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return decoded, err
	}
	return decoded, nil
}
func normalizeListParams(params ListParams) ListParams {
	if params.Limit <= 0 {
		params.Limit = defaultListLimit
	}
	if params.Limit > maxListLimit {
		params.Limit = maxListLimit
	}
	return params
}
func canonicalizeRequester(requester Requester) Requester {
	return Requester{
		UserID:         strings.TrimSpace(requester.UserID),
		Source:         RequesterSource(strings.TrimSpace(string(requester.Source))),
		AgentSessionID: strings.TrimSpace(requester.AgentSessionID),
		AgentRunID:     strings.TrimSpace(requester.AgentRunID),
	}
}
func jobErrorFromExecution(err error) *JobError {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(err.Error())
	summary := "job execution failed"
	details := message
	lower := strings.ToLower(message)
	if strings.Contains(lower, "gorm") ||
		strings.Contains(lower, "sql") ||
		strings.Contains(lower, "select ") ||
		strings.Contains(lower, "insert ") ||
		strings.Contains(lower, "update ") ||
		strings.Contains(lower, "delete ") {
		details = summary
	}
	return &JobError{
		Code:    "job_execution_failed",
		Summary: truncateBounded(summary, maxErrorSummaryLength),
		Details: truncateBounded(details, maxErrorDetailsLength),
	}
}
func truncateBounded(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= maxLen {
		return trimmed
	}
	if maxLen <= 1 {
		return trimmed[:maxLen]
	}
	const ellipsis = "…"
	if maxLen <= len(ellipsis) {
		return ellipsis[:maxLen]
	}
	return trimmed[:maxLen-len(ellipsis)] + ellipsis
}
func dispatchKindForJobType(jobType JobType) executionKind {
	return executionKind(jobType)
}
