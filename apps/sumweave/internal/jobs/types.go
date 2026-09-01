package jobs

import (
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
)

const (
	JobStatusQueued              JobStatus       = "queued"
	JobStatusRunning             JobStatus       = "running"
	JobStatusSucceeded           JobStatus       = "succeeded"
	JobStatusFailed              JobStatus       = "failed"
	RequesterSourceOperator      RequesterSource = "operator"
	defaultListLimit                             = 25
	maxListLimit                                 = 100
	defaultWorkerPollInterval                    = 2 * time.Second
	defaultWorkerMaxAttempts                     = 3
	defaultWorkerStaleRunningAge                 = 5 * time.Minute
	maxErrorSummaryLength                        = 240
	maxErrorDetailsLength                        = 1024
	jobConsumerGroup                             = "jobs.workers.v1"
)

type JobType string
type JobStatus string
type RequesterSource string
type Requester struct {
	UserID string
	Source RequesterSource
}

// JobMetadata is the safe visibility projection selected by an observed
// consumer. It must not contain the command payload or provider credentials.
type JobMetadata struct {
	JobType            JobType
	Requester          Requester
	ScheduleID         string
	ScheduledAt        *time.Time
	ScheduledNextRunAt *time.Time
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
	Error              *JobError
	CreatedAt          time.Time
	UpdatedAt          time.Time
	QueuedAt           time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
	WorkerID           string
	AttemptCount       int
	LastAttemptAt      *time.Time
	ScheduleID         string
	ScheduledAt        *time.Time
	ScheduledNextRunAt *time.Time
}

type ListParams struct {
	Statuses []JobStatus
	JobTypes []JobType
	Sources  []RequesterSource
	Limit    int
	Cursor   string
}

type ListResult struct {
	Items      []Job
	NextCursor string
}

type WorkerConfig struct {
	PollInterval    time.Duration
	MaxAttempts     int
	StaleRunningAge time.Duration
}

func normalizeWorkerConfig(cfg WorkerConfig) WorkerConfig {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultWorkerPollInterval
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultWorkerMaxAttempts
	}
	if cfg.StaleRunningAge <= 0 {
		cfg.StaleRunningAge = defaultWorkerStaleRunningAge
	}
	return cfg
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
		UserID: strings.TrimSpace(requester.UserID),
		Source: RequesterSource(strings.TrimSpace(string(requester.Source))),
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
	if strings.Contains(lower, "gorm") || strings.Contains(lower, "sql") ||
		strings.Contains(lower, "select ") || strings.Contains(lower, "insert ") ||
		strings.Contains(lower, "update ") || strings.Contains(lower, "delete ") {
		details = summary
	}
	return &JobError{
		Code: "job_execution_failed", Summary: truncateBounded(summary, maxErrorSummaryLength),
		Details: truncateBounded(details, maxErrorDetailsLength),
	}
}

func jobErrorFromBusinessFailure(failure *appdispatch.BusinessFailureError) *JobError {
	if failure == nil {
		return nil
	}
	jobErr := jobErrorFromExecution(failure)
	if failure.Code != "" {
		jobErr.Code = truncateBounded(failure.Code, 128)
	}
	if failure.Summary != "" {
		jobErr.Summary = truncateBounded(failure.Summary, maxErrorSummaryLength)
	}
	if failure.Details != "" {
		jobErr.Details = truncateBounded(failure.Details, maxErrorDetailsLength)
	}
	return jobErr
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

func validateRequiredTimestamp(field string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("%s must be a non-zero timestamp", field)
	}
	return nil
}
