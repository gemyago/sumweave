package strategyassistant

import (
	"errors"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
)

const (
	toolErrorCodeValidation      = "validation"
	toolErrorCodeNotFound        = "not_found"
	toolErrorCodeConflict        = "conflict"
	toolErrorCodeDataUnavailable = "data_unavailable"
	toolErrorCodeNotReady        = "not_ready"
	toolErrorCodeUnsavedVersion  = "unsaved_version"
	toolErrorCodeMissingArtifact = "missing_artifact"
	toolErrorCodeInternal        = "internal"

	toolErrorMessageValidation      = "Request validation failed."
	toolErrorMessageNotFound        = "Requested resource was not found."
	toolErrorMessageConflict        = "Requested change conflicts with existing state."
	toolErrorMessageDataUnavailable = "Requested data is unavailable for the selected scope."
	toolErrorMessageNotReady        = "Requested resource is not ready for this operation."
	toolErrorMessageUnsavedVersion  = "Save the strategy version before running this operation."
	toolErrorMessageMissingArtifact = "Required immutable artifact is unavailable."
	toolErrorMessageInternal        = "Tool request could not be completed."

	defaultPlaceholderNextStepHint = "Retry after the remaining strategy assistant tool chunks are implemented."
)

type ToolFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ToolError struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	FieldErrors []ToolFieldError  `json:"fieldErrors,omitempty"`
	Retryable   bool              `json:"retryable,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
}

type ToolTruncation struct {
	IsTruncated    bool       `json:"isTruncated"`
	Limit          int        `json:"limit"`
	Returned       int        `json:"returned"`
	Total          *int       `json:"total,omitempty"`
	NextCursor     string     `json:"nextCursor,omitempty"`
	NextRangeStart *time.Time `json:"nextRangeStart,omitempty"`
}

type recoverableToolError struct {
	code        string
	message     string
	fieldErrors []ToolFieldError
	retryable   bool
	details     map[string]string
}

func (e *recoverableToolError) Error() string {
	return e.message
}

func NewDataUnavailableError(details map[string]string) error {
	return &recoverableToolError{
		code:    toolErrorCodeDataUnavailable,
		message: toolErrorMessageDataUnavailable,
		details: cloneStringMap(details),
	}
}

func NewNotReadyError(details map[string]string) error {
	return &recoverableToolError{
		code:    toolErrorCodeNotReady,
		message: toolErrorMessageNotReady,
		details: cloneStringMap(details),
	}
}

func NewUnsavedVersionError(details map[string]string) error {
	return &recoverableToolError{
		code:    toolErrorCodeUnsavedVersion,
		message: toolErrorMessageUnsavedVersion,
		details: cloneStringMap(details),
	}
}

func NewMissingArtifactError(details map[string]string) error {
	return &recoverableToolError{
		code:    toolErrorCodeMissingArtifact,
		message: toolErrorMessageMissingArtifact,
		details: cloneStringMap(details),
	}
}

func NewTruncation(limit int, returned int, total *int, nextCursor string, nextRangeStart *time.Time) *ToolTruncation {
	if limit <= 0 {
		return nil
	}

	isTruncated := nextCursor != "" || nextRangeStart != nil
	if total != nil && *total > returned {
		isTruncated = true
	}

	return &ToolTruncation{
		IsTruncated:    isTruncated,
		Limit:          limit,
		Returned:       returned,
		Total:          total,
		NextCursor:     nextCursor,
		NextRangeStart: nextRangeStart,
	}
}

func resultMetaFromError(err error, nextStepHint string) (*ToolError, string) {
	if err == nil {
		return nil, nextStepHint
	}

	var invalidInputErr *app.InvalidInputError
	if errors.As(err, &invalidInputErr) {
		return &ToolError{
			Code:    toolErrorCodeValidation,
			Message: toolErrorMessageValidation,
			FieldErrors: []ToolFieldError{{
				Field:   invalidInputErr.Field,
				Message: invalidInputErr.Reason,
			}},
		}, nextStepHint
	}

	var notFoundErr *app.NotFoundError
	if errors.As(err, &notFoundErr) {
		return &ToolError{Code: toolErrorCodeNotFound, Message: toolErrorMessageNotFound}, nextStepHint
	}

	var conflictErr *app.ConflictError
	if errors.As(err, &conflictErr) {
		return &ToolError{Code: toolErrorCodeConflict, Message: toolErrorMessageConflict}, nextStepHint
	}

	var recoverableErr *recoverableToolError
	if errors.As(err, &recoverableErr) {
		return &ToolError{
			Code:        recoverableErr.code,
			Message:     recoverableErr.message,
			FieldErrors: append([]ToolFieldError(nil), recoverableErr.fieldErrors...),
			Retryable:   recoverableErr.retryable,
			Details:     cloneStringMap(recoverableErr.details),
		}, nextStepHint
	}

	return &ToolError{Code: toolErrorCodeInternal, Message: toolErrorMessageInternal}, nextStepHint
}

func placeholderToolErrorResult() (*ToolError, string) {
	return resultMetaFromError(
		NewNotReadyError(map[string]string{"state": "chunk-pending"}),
		defaultPlaceholderNextStepHint,
	)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}

	return out
}

type ListCandleAvailabilityRequest struct {
	Venue      string `json:"venue,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	AssetClass string `json:"assetClass,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

type CandleAvailabilityTimeframeSummary struct {
	Timeframe string    `json:"timeframe"`
	Count     int       `json:"count"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
}

type CandleAvailabilityDefaultSelection struct {
	Timeframe string `json:"timeframe"`
	Reason    string `json:"reason"`
}

type CandleAvailabilityRow struct {
	Venue            string                               `json:"venue"`
	Symbol           string                               `json:"symbol"`
	AssetClass       string                               `json:"assetClass"`
	Timeframes       []CandleAvailabilityTimeframeSummary `json:"timeframes"`
	DefaultSelection *CandleAvailabilityDefaultSelection  `json:"defaultSelection,omitempty"`
}

type ListCandleAvailabilityResponse struct {
	Items        []CandleAvailabilityRow `json:"items"`
	Error        *ToolError              `json:"error,omitempty"`
	Truncation   *ToolTruncation         `json:"truncation,omitempty"`
	NextStepHint string                  `json:"nextStepHint,omitempty"`
}

type GetCandlesRequest struct {
	Venue      string    `json:"venue"`
	Symbol     string    `json:"symbol"`
	AssetClass string    `json:"assetClass"`
	Timeframe  string    `json:"timeframe"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
}

type CandleRow struct {
	CandleID         string    `json:"candleId"`
	OpenTime         time.Time `json:"openTime"`
	CloseTime        time.Time `json:"closeTime"`
	Open             float64   `json:"open"`
	High             float64   `json:"high"`
	Low              float64   `json:"low"`
	Close            float64   `json:"close"`
	Volume           float64   `json:"volume"`
	Quality          string    `json:"quality"`
	ProvenanceSource string    `json:"provenanceSource"`
	ProvenanceID     string    `json:"provenanceId"`
}

type GetCandlesResponse struct {
	Candles      []CandleRow     `json:"candles"`
	Error        *ToolError      `json:"error,omitempty"`
	Truncation   *ToolTruncation `json:"truncation,omitempty"`
	NextStepHint string          `json:"nextStepHint,omitempty"`
}

type GetCandleEvidenceRequest struct {
	Venue            string    `json:"venue"`
	Symbol           string    `json:"symbol"`
	AssetClass       string    `json:"assetClass"`
	Timeframe        string    `json:"timeframe"`
	OpenTime         time.Time `json:"openTime"`
	ProvenanceSource string    `json:"provenanceSource"`
	ProvenanceID     string    `json:"provenanceId"`
	Limit            int       `json:"limit,omitempty"`
	Offset           int       `json:"offset,omitempty"`
}

type CandleEvidenceRow struct {
	RawPayloadID string    `json:"rawPayloadId"`
	Venue        string    `json:"venue"`
	CapturedAt   time.Time `json:"capturedAt"`
	SourceType   string    `json:"sourceType"`
	Reference    string    `json:"reference"`
}

type GetCandleEvidenceResponse struct {
	Evidence     []CandleEvidenceRow `json:"evidence"`
	Error        *ToolError          `json:"error,omitempty"`
	Truncation   *ToolTruncation     `json:"truncation,omitempty"`
	NextStepHint string              `json:"nextStepHint,omitempty"`
}

type ListStrategyVersionsRequest struct {
	StrategyID string `json:"strategyId,omitempty"`
	Status     string `json:"status,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

type StrategyInstrument struct {
	Venue      string `json:"venue"`
	Symbol     string `json:"symbol"`
	AssetClass string `json:"assetClass"`
	Active     bool   `json:"active"`
}

type StrategyParameterSummary struct {
	FastWindow int `json:"fastWindow"`
	SlowWindow int `json:"slowWindow"`
}

type StrategyDefinition struct {
	Kind       string                   `json:"kind"`
	Instrument StrategyInstrument       `json:"instrument"`
	Timeframe  string                   `json:"timeframe"`
	Parameters StrategyParameterSummary `json:"parameters"`
}

type StrategyVersionRow struct {
	StrategyID       string                   `json:"strategyId"`
	Version          string                   `json:"version"`
	DisplayName      string                   `json:"displayName"`
	Status           string                   `json:"status"`
	SourceType       string                   `json:"sourceType"`
	SourceLabel      string                   `json:"sourceLabel"`
	ArtifactHash     string                   `json:"artifactHash"`
	SchemaVersion    string                   `json:"schemaVersion"`
	Kind             string                   `json:"kind"`
	Instrument       StrategyInstrument       `json:"instrument"`
	Timeframe        string                   `json:"timeframe"`
	ParameterSummary StrategyParameterSummary `json:"parameterSummary"`
	Notes            string                   `json:"notes,omitempty"`
	ParentStrategyID string                   `json:"parentStrategyId,omitempty"`
	ParentVersion    string                   `json:"parentVersion,omitempty"`
	CreatedAt        time.Time                `json:"createdAt"`
	UpdatedAt        time.Time                `json:"updatedAt"`
}

type ListStrategyVersionsResponse struct {
	Items        []StrategyVersionRow `json:"items"`
	Error        *ToolError           `json:"error,omitempty"`
	Truncation   *ToolTruncation      `json:"truncation,omitempty"`
	NextStepHint string               `json:"nextStepHint,omitempty"`
}

type GetStrategyVersionRequest struct {
	StrategyID string `json:"strategyId"`
	Version    string `json:"version"`
}

type StrategyVersionDetail struct {
	StrategyVersionRow

	Definition StrategyDefinition `json:"definition"`
}

type GetStrategyVersionResponse struct {
	Version      *StrategyVersionDetail `json:"version,omitempty"`
	Error        *ToolError             `json:"error,omitempty"`
	NextStepHint string                 `json:"nextStepHint,omitempty"`
}

type ValidateStrategyDefinitionRequest struct {
	Definition StrategyDefinition `json:"definition"`
}

type StrategyValidationPreview struct {
	SchemaVersion    string                   `json:"schemaVersion"`
	Kind             string                   `json:"kind"`
	Instrument       StrategyInstrument       `json:"instrument"`
	Timeframe        string                   `json:"timeframe"`
	ParameterSummary StrategyParameterSummary `json:"parameterSummary"`
	CanonicalJSON    string                   `json:"canonicalJson"`
	ArtifactHash     string                   `json:"artifactHash"`
	ExistingArtifact bool                     `json:"existingArtifact"`
}

type ValidateStrategyDefinitionResponse struct {
	Valid        bool                       `json:"valid"`
	Preview      *StrategyValidationPreview `json:"preview,omitempty"`
	Error        *ToolError                 `json:"error,omitempty"`
	NextStepHint string                     `json:"nextStepHint,omitempty"`
}

type DuplicateStrategyVersionRequest struct {
	StrategyID string `json:"strategyId"`
	Version    string `json:"version"`
}

type StrategyVersionCandidate struct {
	StrategyID       string             `json:"strategyId"`
	Version          string             `json:"version"`
	DisplayName      string             `json:"displayName"`
	Status           string             `json:"status"`
	SourceType       string             `json:"sourceType"`
	SourceLabel      string             `json:"sourceLabel"`
	Notes            string             `json:"notes,omitempty"`
	ParentStrategyID string             `json:"parentStrategyId,omitempty"`
	ParentVersion    string             `json:"parentVersion,omitempty"`
	Definition       StrategyDefinition `json:"definition"`
}

type DuplicateStrategyVersionResponse struct {
	Candidate    *StrategyVersionCandidate `json:"candidate,omitempty"`
	Error        *ToolError                `json:"error,omitempty"`
	NextStepHint string                    `json:"nextStepHint,omitempty"`
}

type CreateStrategyVersionRequest struct {
	StrategyID       string             `json:"strategyId"`
	Version          string             `json:"version"`
	DisplayName      string             `json:"displayName"`
	Notes            string             `json:"notes,omitempty"`
	ParentStrategyID string             `json:"parentStrategyId,omitempty"`
	ParentVersion    string             `json:"parentVersion,omitempty"`
	Definition       StrategyDefinition `json:"definition"`
}

type CreateStrategyVersionResponse struct {
	Version      *StrategyVersionDetail `json:"version,omitempty"`
	Error        *ToolError             `json:"error,omitempty"`
	NextStepHint string                 `json:"nextStepHint,omitempty"`
}

type RunBacktestRequest struct {
	StrategyID         string    `json:"strategyId"`
	StrategyVersion    string    `json:"strategyVersion"`
	Start              time.Time `json:"start"`
	End                time.Time `json:"end"`
	Quantity           float64   `json:"quantity"`
	GovernorPolicyHash string    `json:"governorPolicyHash,omitempty"`
	Note               string    `json:"note,omitempty"`
}

type EvaluationRunSummary struct {
	RunID          string    `json:"runId"`
	Status         string    `json:"status"`
	FailureReason  string    `json:"failureReason,omitempty"`
	FailureDetails string    `json:"failureDetails,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type EvaluationMetricSummary struct {
	TradeCount                    *int     `json:"tradeCount,omitempty"`
	BlockedGovernorDecisionCount  *int     `json:"blockedGovernorDecisionCount,omitempty"`
	RejectedGovernorDecisionCount *int     `json:"rejectedGovernorDecisionCount,omitempty"`
	MaxDrawdown                   *float64 `json:"maxDrawdown,omitempty"`
}

type EvaluationEvidenceCounts struct {
	Traces             int `json:"traces"`
	OrderIntents       int `json:"orderIntents"`
	GovernorDecisions  int `json:"governorDecisions"`
	ExecutionRecords   int `json:"executionRecords"`
	PositionSnapshots  int `json:"positionSnapshots"`
	PortfolioSnapshots int `json:"portfolioSnapshots"`
}

type EvaluationAIRenderMetadata struct {
	RequestSourceType   string                    `json:"requestSourceType"`
	StrategySourceType  string                    `json:"strategySourceType"`
	StrategySourceLabel string                    `json:"strategySourceLabel"`
	Note                string                    `json:"note,omitempty"`
	EvidenceCounts      *EvaluationEvidenceCounts `json:"evidenceCounts,omitempty"`
}

type EvaluationDatasetReference struct {
	DatasetID      string    `json:"datasetId"`
	ReplayChecksum string    `json:"replayChecksum"`
	CreatedAt      time.Time `json:"createdAt"`
}

type EvaluationPolicyReference struct {
	PolicyID      string `json:"policyId"`
	PolicyVersion string `json:"policyVersion"`
	PolicyHash    string `json:"policyHash"`
}

type RunBacktestResponse struct {
	Run          *EvaluationRunSummary `json:"run,omitempty"`
	Error        *ToolError            `json:"error,omitempty"`
	NextStepHint string                `json:"nextStepHint,omitempty"`
}

type ListBacktestsRequest struct {
	StrategyID string `json:"strategyId,omitempty"`
	Status     string `json:"status,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

type EvaluationListRow struct {
	RunID                string                   `json:"runId"`
	StrategyID           string                   `json:"strategyId"`
	StrategyVersion      string                   `json:"strategyVersion"`
	StrategyArtifactHash string                   `json:"strategyArtifactHash"`
	SourceType           string                   `json:"sourceType"`
	SourceLabel          string                   `json:"sourceLabel"`
	Instrument           StrategyInstrument       `json:"instrument"`
	Timeframe            string                   `json:"timeframe"`
	TestedRangeStart     time.Time                `json:"testedRangeStart"`
	TestedRangeEnd       time.Time                `json:"testedRangeEnd"`
	Status               string                   `json:"status"`
	Decision             *string                  `json:"decision,omitempty"`
	Metrics              *EvaluationMetricSummary `json:"metrics,omitempty"`
	FailureReason        string                   `json:"failureReason,omitempty"`
	FailureDetails       string                   `json:"failureDetails,omitempty"`
	CreatedAt            time.Time                `json:"createdAt"`
	UpdatedAt            time.Time                `json:"updatedAt"`
}

type ListBacktestsResponse struct {
	Items        []EvaluationListRow `json:"items"`
	Error        *ToolError          `json:"error,omitempty"`
	Truncation   *ToolTruncation     `json:"truncation,omitempty"`
	NextStepHint string              `json:"nextStepHint,omitempty"`
}

type GetBacktestDetailRequest struct {
	RunID string `json:"runId"`
}

type EvaluationDetail struct {
	RunID                string                      `json:"runId"`
	StrategyID           string                      `json:"strategyId"`
	StrategyVersion      string                      `json:"strategyVersion"`
	StrategyArtifactHash string                      `json:"strategyArtifactHash"`
	SourceType           string                      `json:"sourceType"`
	SourceLabel          string                      `json:"sourceLabel"`
	Instrument           StrategyInstrument          `json:"instrument"`
	Timeframe            string                      `json:"timeframe"`
	TestedRangeStart     time.Time                   `json:"testedRangeStart"`
	TestedRangeEnd       time.Time                   `json:"testedRangeEnd"`
	Status               string                      `json:"status"`
	Decision             *string                     `json:"decision,omitempty"`
	FailureReason        string                      `json:"failureReason,omitempty"`
	FailureDetails       string                      `json:"failureDetails,omitempty"`
	Metrics              *EvaluationMetricSummary    `json:"metrics,omitempty"`
	DatasetReference     *EvaluationDatasetReference `json:"datasetReference,omitempty"`
	PolicyReference      EvaluationPolicyReference   `json:"policyReference"`
	CreatedAt            time.Time                   `json:"createdAt"`
	UpdatedAt            time.Time                   `json:"updatedAt"`
	AIRenderMetadata     *EvaluationAIRenderMetadata `json:"aiRenderMetadata,omitempty"`
}

type GetBacktestDetailResponse struct {
	Detail       *EvaluationDetail `json:"detail,omitempty"`
	Error        *ToolError        `json:"error,omitempty"`
	NextStepHint string            `json:"nextStepHint,omitempty"`
}

type GetBacktestReportRequest struct {
	RunID string `json:"runId"`
}

type EvaluationReport struct {
	RunID            string                      `json:"runId"`
	Status           string                      `json:"status"`
	Summary          string                      `json:"summary,omitempty"`
	Decision         string                      `json:"decision,omitempty"`
	FailureReason    string                      `json:"failureReason,omitempty"`
	FailureDetails   string                      `json:"failureDetails,omitempty"`
	Metrics          *EvaluationMetricSummary    `json:"metrics,omitempty"`
	DatasetReference *EvaluationDatasetReference `json:"datasetReference,omitempty"`
	PolicyReference  EvaluationPolicyReference   `json:"policyReference"`
	AIRenderMetadata *EvaluationAIRenderMetadata `json:"aiRenderMetadata,omitempty"`
}

type GetBacktestReportResponse struct {
	Report       *EvaluationReport `json:"report,omitempty"`
	Error        *ToolError        `json:"error,omitempty"`
	Truncation   *ToolTruncation   `json:"truncation,omitempty"`
	NextStepHint string            `json:"nextStepHint,omitempty"`
}

type GetBacktestEvidenceRequest struct {
	RunID  string `json:"runId"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

type EvaluationTraceEvidenceRow struct {
	TraceID      string    `json:"traceId"`
	DecisionTime time.Time `json:"decisionTime"`
	Result       string    `json:"result"`
	ReasonCodes  []string  `json:"reasonCodes,omitempty"`
	DataQuality  string    `json:"dataQuality"`
	RunReference string    `json:"runReference,omitempty"`
}

type EvaluationOrderIntentEvidenceRow struct {
	IntentID          string    `json:"intentId"`
	TraceID           string    `json:"traceId,omitempty"`
	Status            string    `json:"status"`
	ActionKind        string    `json:"actionKind"`
	RequestedQuantity float64   `json:"requestedQuantity"`
	RequestedNotional float64   `json:"requestedNotional"`
	CreatedTime       time.Time `json:"createdTime"`
}

type EvaluationGovernorDecisionEvidenceRow struct {
	DecisionID string `json:"decisionId,omitempty"`
	IntentID   string `json:"intentId,omitempty"`
	Status     string `json:"status,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Reference  string `json:"reference,omitempty"`
}

type EvaluationExecutionEvidenceRow struct {
	CommandID string     `json:"commandId,omitempty"`
	OrderID   string     `json:"orderId,omitempty"`
	FillID    string     `json:"fillId,omitempty"`
	Status    string     `json:"status,omitempty"`
	EventTime *time.Time `json:"eventTime,omitempty"`
}

type EvaluationPositionSnapshotEvidenceRow struct {
	SnapshotID  string    `json:"snapshotId"`
	FillID      string    `json:"fillId,omitempty"`
	Quantity    float64   `json:"quantity"`
	RealizedPnL float64   `json:"realizedPnl"`
	EventTime   time.Time `json:"eventTime"`
}

type EvaluationPortfolioSnapshotEvidenceRow struct {
	SnapshotID    string    `json:"snapshotId"`
	FillID        string    `json:"fillId,omitempty"`
	GrossExposure float64   `json:"grossExposure"`
	NetExposure   float64   `json:"netExposure"`
	RealizedPnL   float64   `json:"realizedPnl"`
	EventTime     time.Time `json:"eventTime"`
}

type EvaluationTraceEvidenceSection struct {
	Rows       []EvaluationTraceEvidenceRow `json:"rows"`
	Truncation *ToolTruncation              `json:"truncation,omitempty"`
}

type EvaluationOrderIntentEvidenceSection struct {
	Rows       []EvaluationOrderIntentEvidenceRow `json:"rows"`
	Truncation *ToolTruncation                    `json:"truncation,omitempty"`
}

type EvaluationGovernorDecisionEvidenceSection struct {
	Rows       []EvaluationGovernorDecisionEvidenceRow `json:"rows"`
	Truncation *ToolTruncation                         `json:"truncation,omitempty"`
}

type EvaluationExecutionEvidenceSection struct {
	Rows       []EvaluationExecutionEvidenceRow `json:"rows"`
	Truncation *ToolTruncation                  `json:"truncation,omitempty"`
}

type EvaluationPositionSnapshotEvidenceSection struct {
	Rows       []EvaluationPositionSnapshotEvidenceRow `json:"rows"`
	Truncation *ToolTruncation                         `json:"truncation,omitempty"`
}

type EvaluationPortfolioSnapshotEvidenceSection struct {
	Rows       []EvaluationPortfolioSnapshotEvidenceRow `json:"rows"`
	Truncation *ToolTruncation                          `json:"truncation,omitempty"`
}

type EvaluationEvidence struct {
	RunID              string                                     `json:"runId"`
	Status             string                                     `json:"status"`
	AIRenderMetadata   *EvaluationAIRenderMetadata                `json:"aiRenderMetadata,omitempty"`
	Traces             EvaluationTraceEvidenceSection             `json:"traces"`
	OrderIntents       EvaluationOrderIntentEvidenceSection       `json:"orderIntents"`
	GovernorDecisions  EvaluationGovernorDecisionEvidenceSection  `json:"governorDecisions"`
	ExecutionRecords   EvaluationExecutionEvidenceSection         `json:"executionRecords"`
	PositionSnapshots  EvaluationPositionSnapshotEvidenceSection  `json:"positionSnapshots"`
	PortfolioSnapshots EvaluationPortfolioSnapshotEvidenceSection `json:"portfolioSnapshots"`
}

type GetBacktestEvidenceResponse struct {
	Evidence     *EvaluationEvidence `json:"evidence,omitempty"`
	Error        *ToolError          `json:"error,omitempty"`
	Truncation   *ToolTruncation     `json:"truncation,omitempty"`
	NextStepHint string              `json:"nextStepHint,omitempty"`
}
