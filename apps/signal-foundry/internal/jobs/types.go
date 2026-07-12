package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
)

const (
	JobTypeHistoricalRawCandleBackfill JobType = "data.historical_raw_candle_backfill"

	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCanceled  JobStatus = "canceled"

	RequesterSourceOperator RequesterSource = "operator"
	RequesterSourceAgent    RequesterSource = "agent"

	defaultListLimit                 = 25
	maxListLimit                     = 100
	defaultHistoricalMaxIntervals    = 10000
	defaultHistoricalMaxPageSize     = 5000
	defaultWorkerPollInterval        = 2 * time.Second
	defaultWorkerMaxAttempts         = 3
	defaultWorkerMaxConcurrent       = 1
	defaultHistoricalBackfillBaseURL = "https://api.hyperliquid.xyz"
	maxErrorSummaryLength            = 240
	maxErrorDetailsLength            = 1024

	DispatchKindHistoricalRawCandleBackfill appdispatch.ExecutionKind = "jobs.data.historical_raw_candle_backfill.execute.v1"
)

type JobType string

type JobStatus string

type RequesterSource string

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

type HistoricalRawCandleBackfillInput struct {
	IngestionRunID string           `json:"ingestionRunId"`
	Venue          string           `json:"venue"`
	Symbol         string           `json:"symbol"`
	AssetClass     string           `json:"assetClass"`
	Timeframe      string           `json:"timeframe"`
	Start          time.Time        `json:"start"`
	End            time.Time        `json:"end"`
	PageSize       int              `json:"pageSize"`
	TimeRange      domain.TimeRange `json:"-"`
}

type HistoricalRawCandleBackfillResult struct {
	IngestionRunID            string         `json:"ingestionRunId"`
	PersistedCount            int            `json:"persistedCount"`
	ExpectedCount             int            `json:"expectedCount"`
	MissingIntervalCount      int            `json:"missingIntervalCount"`
	DuplicateNaturalKeyCount  int            `json:"duplicateNaturalKeyCount"`
	FirstPersistedStart       *time.Time     `json:"firstPersistedStart,omitempty"`
	LastPersistedEnd          *time.Time     `json:"lastPersistedEnd,omitempty"`
	RawPayloadCount           *int           `json:"rawPayloadCount,omitempty"`
	MissingIntervalPreview    []jobTimeRange `json:"missingIntervalPreview,omitempty"`
	MissingIntervalPreviewCap int            `json:"missingIntervalPreviewLimit"`
}

type jobTimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Job struct {
	ID                 string
	JobType            JobType
	Status             JobStatus
	Requester          Requester
	IdempotencyKey     string
	InputHash          string
	Input              HistoricalRawCandleBackfillInput
	InputJSON          json.RawMessage
	Result             *HistoricalRawCandleBackfillResult
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

type CreateHistoricalRawCandleBackfillParams struct {
	Requester      Requester
	IdempotencyKey string
	CorrelationID  string
	Venue          string
	Symbol         string
	AssetClass     string
	Timeframe      string
	Start          time.Time
	End            time.Time
	PageSize       int
}

type HistoricalBackfillLimits struct {
	MaxIntervals int
	MaxPageSize  int
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
	Enabled                         bool
	PollInterval                    time.Duration
	MaxAttempts                     int
	MaxConcurrentHistoricalBackfill int
}

type DispatchConfig struct {
	DatabaseDSN string
	TablePrefix string
}

type idempotencyConflictError struct {
	key string
}

func (e *idempotencyConflictError) Error() string {
	return fmt.Sprintf("idempotency key conflict: %s", e.key)
}

func (e *idempotencyConflictError) Code() string {
	return "idempotency_key_conflict"
}

func IsIdempotencyConflict(err error) bool {
	var target *idempotencyConflictError
	return errors.As(err, &target)
}

func normalizeHistoricalBackfillLimits(limits HistoricalBackfillLimits) HistoricalBackfillLimits {
	if limits.MaxIntervals <= 0 {
		limits.MaxIntervals = defaultHistoricalMaxIntervals
	}
	if limits.MaxPageSize <= 0 {
		limits.MaxPageSize = defaultHistoricalMaxPageSize
	}
	return limits
}

func normalizeWorkerConfig(cfg WorkerConfig) WorkerConfig {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultWorkerPollInterval
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultWorkerMaxAttempts
	}
	if cfg.MaxConcurrentHistoricalBackfill <= 0 {
		cfg.MaxConcurrentHistoricalBackfill = defaultWorkerMaxConcurrent
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

func DecodeJobInput[T any](job Job) (T, error) { //nolint:ireturn
	return decodeJobPayload[T](job.InputJSON)
}

func DecodeJobResult[T any](job Job) (T, error) { //nolint:ireturn
	return decodeJobPayload[T](job.ResultJSON)
}

func DecodeJobProgress[T any](job Job) (T, error) { //nolint:ireturn
	return decodeJobPayload[T](job.ProgressJSON)
}

func decodeJobPayload[T any](payload json.RawMessage) (T, error) { //nolint:ireturn
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

func canonicalizeHistoricalInput(input HistoricalRawCandleBackfillInput) HistoricalRawCandleBackfillInput {
	input.IngestionRunID = strings.TrimSpace(input.IngestionRunID)
	input.Venue = strings.TrimSpace(input.Venue)
	input.Symbol = strings.ToUpper(strings.TrimSpace(input.Symbol))
	input.AssetClass = strings.TrimSpace(input.AssetClass)
	input.Timeframe = strings.TrimSpace(input.Timeframe)
	input.TimeRange = domain.TimeRange{Start: input.Start, End: input.End}
	return input
}

func marshalHistoricalInput(input HistoricalRawCandleBackfillInput) ([]byte, error) {
	canonical := canonicalizeHistoricalInput(input)
	return json.Marshal(canonical)
}

func jobErrorFromExecution(err error) *JobError {
	if err == nil {
		return nil
	}

	message := strings.TrimSpace(err.Error())
	messageLower := strings.ToLower(message)
	summary := "job execution failed"
	details := message
	if strings.Contains(messageLower, "gorm") ||
		strings.Contains(messageLower, "sql") ||
		strings.Contains(messageLower, "select ") ||
		strings.Contains(messageLower, "insert ") ||
		strings.Contains(messageLower, "update ") ||
		strings.Contains(messageLower, "delete ") {
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

func validateHistoricalBackfillInput(
	input HistoricalRawCandleBackfillInput,
	limits HistoricalBackfillLimits,
	now time.Time,
) (HistoricalRawCandleBackfillInput, error) {
	limits = normalizeHistoricalBackfillLimits(limits)
	input = canonicalizeHistoricalInput(input)

	if input.Venue != string(venueedge.HyperliquidPerpsVenueName) {
		return HistoricalRawCandleBackfillInput{}, errors.New("venue must be hyperliquid-perps")
	}
	if input.Symbol == "" {
		return HistoricalRawCandleBackfillInput{}, errors.New("symbol is required")
	}
	if input.AssetClass != string(domain.AssetClassFuture) {
		return HistoricalRawCandleBackfillInput{}, errors.New("asset class must be future")
	}
	if _, err := domain.NewTimeframe(input.Timeframe); err != nil {
		return HistoricalRawCandleBackfillInput{}, errors.New("timeframe is unsupported")
	}
	if _, err := domain.NewTimeRange(input.Start, input.End); err != nil {
		return HistoricalRawCandleBackfillInput{}, errors.New("time range must be half-open")
	}
	if input.End.After(now) {
		return HistoricalRawCandleBackfillInput{}, errors.New("end must not be in the future")
	}
	if input.PageSize < 0 {
		return HistoricalRawCandleBackfillInput{}, errors.New("page size must be zero or positive")
	}
	if input.PageSize > limits.MaxPageSize {
		return HistoricalRawCandleBackfillInput{}, fmt.Errorf("page size exceeds maximum %d", limits.MaxPageSize)
	}
	intervalDuration, err := historicalBackfillTimeframeDuration(domain.Timeframe(input.Timeframe))
	if err != nil {
		return HistoricalRawCandleBackfillInput{}, errors.New("timeframe is unsupported")
	}
	intervalCount := int(input.End.Sub(input.Start) / intervalDuration)
	if intervalCount > limits.MaxIntervals {
		return HistoricalRawCandleBackfillInput{}, fmt.Errorf(
			"time range exceeds maximum %d intervals",
			limits.MaxIntervals,
		)
	}
	return input, nil
}

func historicalBackfillTimeframeDuration(timeframe domain.Timeframe) (time.Duration, error) {
	switch timeframe {
	case domain.Timeframe1m:
		return time.Minute, nil
	case domain.Timeframe5m:
		return 5 * time.Minute, nil
	case domain.Timeframe15m:
		return 15 * time.Minute, nil
	case domain.Timeframe1h:
		return time.Hour, nil
	case domain.Timeframe4h:
		return 4 * time.Hour, nil
	case domain.Timeframe1d:
		return 24 * time.Hour, nil
	default:
		return 0, errors.New("unsupported timeframe")
	}
}
