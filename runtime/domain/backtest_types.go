package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// DatasetReferenceID identifies a persisted deterministic dataset reference.
type DatasetReferenceID string

// BacktestRunID identifies a persisted deterministic backtest run.
type BacktestRunID string

// EvaluationReportID identifies a persisted deterministic evaluation report.
type EvaluationReportID string

// BacktestRunStatus identifies a supported backtest lifecycle state.
type BacktestRunStatus string

const (
	BacktestRunStatusPending   BacktestRunStatus = "pending"
	BacktestRunStatusRunning   BacktestRunStatus = "running"
	BacktestRunStatusCompleted BacktestRunStatus = "completed"
	BacktestRunStatusFailed    BacktestRunStatus = "failed"
)

// EvaluationDecision identifies a supported evaluation report decision.
type EvaluationDecision string

const (
	EvaluationDecisionPromoteToPaperCandidate EvaluationDecision = "promote_to_paper_candidate"
	EvaluationDecisionReject                  EvaluationDecision = "reject"
	EvaluationDecisionNeedsReview             EvaluationDecision = "needs_review"
)

// DatasetReferenceTime identifies a canonical dataset reference timestamp.
type DatasetReferenceTime time.Time

// BacktestRunTime identifies a canonical backtest run timestamp.
type BacktestRunTime time.Time

// EvaluationReportTime identifies a canonical evaluation report timestamp.
type EvaluationReportTime time.Time

// VersionedMetrics stores compact versioned summary metrics.
type VersionedMetrics struct {
	SchemaVersion                 string   `json:"schemaVersion"`
	TradeCount                    *int     `json:"tradeCount,omitempty"`
	BlockedGovernorDecisionCount  *int     `json:"blockedGovernorDecisionCount,omitempty"`
	RejectedGovernorDecisionCount *int     `json:"rejectedGovernorDecisionCount,omitempty"`
	MaxDrawdown                   *float64 `json:"maxDrawdown,omitempty"`
}

// VersionedMetricsParams holds inputs for compact summary metrics.
type VersionedMetricsParams struct {
	SchemaVersion                 string
	TradeCount                    *int
	BlockedGovernorDecisionCount  *int
	RejectedGovernorDecisionCount *int
	MaxDrawdown                   *float64
}

// DatasetReference captures reproducible backtest dataset provenance.
type DatasetReference struct {
	DatasetID        DatasetReferenceID
	EntityTypes      []string
	Instruments      []Instrument
	Timeframes       []Timeframe
	TimeRange        TimeRange
	SourceDataHashes []string
	ReplayChecksum   string
	CreatedAt        DatasetReferenceTime
	Metadata         map[string]string
}

// DatasetReferenceParams holds inputs for a canonical dataset reference.
type DatasetReferenceParams struct {
	DatasetID        string
	EntityTypes      []string
	Instruments      []Instrument
	Timeframes       []Timeframe
	TimeRange        TimeRange
	SourceDataHashes []string
	ReplayChecksum   string
	CreatedAt        time.Time
	Metadata         map[string]string
}

// BacktestRun captures durable backtest lifecycle state.
type BacktestRun struct {
	RunID                     BacktestRunID
	StrategyID                string
	StrategyVersion           string
	StrategyArtifactHash      string
	DatasetID                 DatasetReferenceID
	GovernorPolicyID          string
	GovernorPolicyVersion     string
	GovernorPolicyHash        string
	Mode                      DecisionMode
	TestedRange               TimeRange
	FeeModelID                string
	FeeAssumptions            map[string]string
	SlippageModelID           string
	SlippageAssumptions       map[string]string
	ExecutionSimulatorVersion string
	Status                    BacktestRunStatus
	Metrics                   *VersionedMetrics
	FailureReason             string
	FailureDetails            string
	CreatedAt                 BacktestRunTime
	UpdatedAt                 BacktestRunTime
}

// BacktestRunParams holds inputs for a canonical backtest run.
type BacktestRunParams struct {
	RunID                     string
	StrategyID                string
	StrategyVersion           string
	StrategyArtifactHash      string
	DatasetID                 string
	GovernorPolicyID          string
	GovernorPolicyVersion     string
	GovernorPolicyHash        string
	Mode                      DecisionMode
	TestedRange               TimeRange
	FeeModelID                string
	FeeAssumptions            map[string]string
	SlippageModelID           string
	SlippageAssumptions       map[string]string
	ExecutionSimulatorVersion string
	Status                    BacktestRunStatus
	Metrics                   *VersionedMetrics
	FailureReason             string
	FailureDetails            string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// EvaluationReport captures compact evaluation evidence and outcome.
type EvaluationReport struct {
	EvaluationID         EvaluationReportID
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	BacktestRunID        BacktestRunID
	DatasetID            DatasetReferenceID
	Decision             EvaluationDecision
	Metrics              *VersionedMetrics
	FailureReasons       []string
	Notes                string
	CreatedAt            EvaluationReportTime
}

// EvaluationReportParams holds inputs for a canonical evaluation report.
type EvaluationReportParams struct {
	EvaluationID         string
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	BacktestRunID        string
	DatasetID            string
	Decision             EvaluationDecision
	Metrics              *VersionedMetrics
	FailureReasons       []string
	Notes                string
	CreatedAt            time.Time
}

// NewDatasetReferenceID validates and canonicalizes a dataset reference id.
func NewDatasetReferenceID(value string) (DatasetReferenceID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("dataset reference id is required")
	}

	return DatasetReferenceID(normalized), nil
}

// NewBacktestRunID validates and canonicalizes a backtest run id.
func NewBacktestRunID(value string) (BacktestRunID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("backtest run id is required")
	}

	return BacktestRunID(normalized), nil
}

// NewEvaluationReportID validates and canonicalizes an evaluation report id.
func NewEvaluationReportID(value string) (EvaluationReportID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("evaluation report id is required")
	}

	return EvaluationReportID(normalized), nil
}

// NewBacktestRunStatus validates and canonicalizes a backtest run status.
func NewBacktestRunStatus(value string) (BacktestRunStatus, error) {
	normalized := BacktestRunStatus(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid backtest run status %q", value)
	}

	return normalized, nil
}

// NewEvaluationDecision validates and canonicalizes an evaluation decision.
func NewEvaluationDecision(value string) (EvaluationDecision, error) {
	normalized := EvaluationDecision(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid evaluation decision %q", value)
	}

	return normalized, nil
}

// NewDatasetReferenceTime validates and canonicalizes a dataset reference time.
func NewDatasetReferenceTime(value time.Time) (DatasetReferenceTime, error) {
	if value.IsZero() {
		return DatasetReferenceTime{}, errors.New("dataset reference created at is required")
	}

	return DatasetReferenceTime(canonicalUTC(value)), nil
}

// NewBacktestRunTime validates and canonicalizes a backtest run timestamp.
func NewBacktestRunTime(value time.Time, field string) (BacktestRunTime, error) {
	if value.IsZero() {
		return BacktestRunTime{}, errors.New(field + " is required")
	}

	return BacktestRunTime(canonicalUTC(value)), nil
}

// NewEvaluationReportTime validates and canonicalizes an evaluation report time.
func NewEvaluationReportTime(value time.Time) (EvaluationReportTime, error) {
	if value.IsZero() {
		return EvaluationReportTime{}, errors.New("evaluation report created at is required")
	}

	return EvaluationReportTime(canonicalUTC(value)), nil
}

// NewVersionedMetrics validates and canonicalizes compact versioned metrics.
func NewVersionedMetrics(params VersionedMetricsParams) (*VersionedMetrics, error) {
	schemaVersion := strings.TrimSpace(params.SchemaVersion)
	if schemaVersion == "" {
		return nil, errors.New("metrics schema version is required")
	}

	tradeCount, hasTradeCount, err := canonicalOptionalNonNegativeInt(
		params.TradeCount,
		"metrics trade count",
	)
	if err != nil {
		return nil, err
	}
	blockedCount, hasBlockedCount, err := canonicalOptionalNonNegativeInt(
		params.BlockedGovernorDecisionCount,
		"metrics blocked governor decision count",
	)
	if err != nil {
		return nil, err
	}
	rejectedCount, hasRejectedCount, err := canonicalOptionalNonNegativeInt(
		params.RejectedGovernorDecisionCount,
		"metrics rejected governor decision count",
	)
	if err != nil {
		return nil, err
	}
	maxDrawdown, hasMaxDrawdown, err := canonicalOptionalNonNegativeFloat64(
		params.MaxDrawdown,
		"metrics max drawdown",
	)
	if err != nil {
		return nil, err
	}

	var tradeCountPtr *int
	if hasTradeCount {
		tradeCountPtr = &tradeCount
	}
	var blockedCountPtr *int
	if hasBlockedCount {
		blockedCountPtr = &blockedCount
	}
	var rejectedCountPtr *int
	if hasRejectedCount {
		rejectedCountPtr = &rejectedCount
	}
	var maxDrawdownPtr *float64
	if hasMaxDrawdown {
		maxDrawdownPtr = &maxDrawdown
	}

	return &VersionedMetrics{
		SchemaVersion:                 schemaVersion,
		TradeCount:                    tradeCountPtr,
		BlockedGovernorDecisionCount:  blockedCountPtr,
		RejectedGovernorDecisionCount: rejectedCountPtr,
		MaxDrawdown:                   maxDrawdownPtr,
	}, nil
}

// NewDatasetReference validates and canonicalizes a deterministic dataset reference.
func NewDatasetReference(params DatasetReferenceParams) (DatasetReference, error) {
	datasetID, err := NewDatasetReferenceID(params.DatasetID)
	if err != nil {
		return DatasetReference{}, err
	}

	entityTypes, err := canonicalDatasetEntityTypes(params.EntityTypes)
	if err != nil {
		return DatasetReference{}, err
	}
	instruments, err := canonicalDatasetInstruments(params.Instruments)
	if err != nil {
		return DatasetReference{}, err
	}
	timeframes, err := canonicalDatasetTimeframes(params.Timeframes)
	if err != nil {
		return DatasetReference{}, err
	}
	timeRange, err := NewTimeRange(params.TimeRange.Start, params.TimeRange.End)
	if err != nil {
		return DatasetReference{}, fmt.Errorf("dataset reference time range: %w", err)
	}
	sourceDataHashes, replayChecksum, err := canonicalDatasetEvidence(
		params.SourceDataHashes,
		params.ReplayChecksum,
	)
	if err != nil {
		return DatasetReference{}, err
	}
	createdAt, err := NewDatasetReferenceTime(params.CreatedAt)
	if err != nil {
		return DatasetReference{}, err
	}
	metadata, err := canonicalAuditMetadata(params.Metadata)
	if err != nil {
		return DatasetReference{}, fmt.Errorf("dataset reference metadata: %w", err)
	}
	if params.Metadata == nil {
		metadata = nil
	}

	return DatasetReference{
		DatasetID:        datasetID,
		EntityTypes:      entityTypes,
		Instruments:      instruments,
		Timeframes:       timeframes,
		TimeRange:        timeRange,
		SourceDataHashes: sourceDataHashes,
		ReplayChecksum:   replayChecksum,
		CreatedAt:        createdAt,
		Metadata:         metadata,
	}, nil
}

// NewBacktestRun validates and canonicalizes a deterministic backtest run.
func NewBacktestRun(params BacktestRunParams) (BacktestRun, error) {
	runID, err := NewBacktestRunID(params.RunID)
	if err != nil {
		return BacktestRun{}, err
	}
	strategyID, strategyVersion, strategyArtifactHash, err := optionalStrategyFields(
		params.StrategyID,
		params.StrategyVersion,
		params.StrategyArtifactHash,
		"backtest run",
	)
	if err != nil || strategyID == "" {
		if err == nil {
			err = errors.New("backtest run strategy id is required")
		}
		return BacktestRun{}, err
	}
	datasetID, err := NewDatasetReferenceID(params.DatasetID)
	if err != nil {
		return BacktestRun{}, fmt.Errorf("backtest run dataset reference: %w", err)
	}
	governorPolicyID, governorPolicyVersion, governorPolicyHash, err := canonicalOptionalIdentityTriple(
		params.GovernorPolicyID,
		params.GovernorPolicyVersion,
		params.GovernorPolicyHash,
		"backtest run governor policy",
	)
	if err != nil {
		return BacktestRun{}, err
	}
	mode, err := NewDecisionMode(params.Mode.String())
	if err != nil {
		return BacktestRun{}, errors.New("backtest run mode is required")
	}
	if mode != DecisionModeBacktest {
		return BacktestRun{}, errors.New("backtest run mode must be backtest")
	}
	testedRange, err := NewTimeRange(params.TestedRange.Start, params.TestedRange.End)
	if err != nil {
		return BacktestRun{}, fmt.Errorf("backtest run tested range: %w", err)
	}
	feeModelID, feeAssumptions, err := canonicalModelOrAssumptions(
		params.FeeModelID,
		params.FeeAssumptions,
		"backtest run fee",
	)
	if err != nil {
		return BacktestRun{}, err
	}
	slippageModelID, slippageAssumptions, err := canonicalModelOrAssumptions(
		params.SlippageModelID,
		params.SlippageAssumptions,
		"backtest run slippage",
	)
	if err != nil {
		return BacktestRun{}, err
	}
	executionSimulatorVersion, err := requiredAuditText(
		"backtest run execution simulator version",
		params.ExecutionSimulatorVersion,
	)
	if err != nil {
		return BacktestRun{}, err
	}
	status, metrics, failureReason, failureDetails, createdAt, updatedAt, err := canonicalBacktestRunLifecycle(
		params,
	)
	if err != nil {
		return BacktestRun{}, err
	}
	if metrics == nil {
		metrics = nil
	}

	return BacktestRun{
		RunID:                     runID,
		StrategyID:                strategyID,
		StrategyVersion:           strategyVersion,
		StrategyArtifactHash:      strategyArtifactHash,
		DatasetID:                 datasetID,
		GovernorPolicyID:          governorPolicyID,
		GovernorPolicyVersion:     governorPolicyVersion,
		GovernorPolicyHash:        governorPolicyHash,
		Mode:                      mode,
		TestedRange:               testedRange,
		FeeModelID:                feeModelID,
		FeeAssumptions:            feeAssumptions,
		SlippageModelID:           slippageModelID,
		SlippageAssumptions:       slippageAssumptions,
		ExecutionSimulatorVersion: executionSimulatorVersion,
		Status:                    status,
		Metrics:                   metrics,
		FailureReason:             failureReason,
		FailureDetails:            failureDetails,
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
	}, nil
}

// NewEvaluationReport validates and canonicalizes a deterministic evaluation report.
func NewEvaluationReport(params EvaluationReportParams) (EvaluationReport, error) {
	evaluationID, err := NewEvaluationReportID(params.EvaluationID)
	if err != nil {
		return EvaluationReport{}, err
	}
	strategyID, strategyVersion, strategyArtifactHash, err := optionalStrategyFields(
		params.StrategyID,
		params.StrategyVersion,
		params.StrategyArtifactHash,
		"evaluation report",
	)
	if err != nil || strategyID == "" {
		if err == nil {
			err = errors.New("evaluation report strategy id is required")
		}
		return EvaluationReport{}, err
	}
	backtestRunID, err := NewBacktestRunID(params.BacktestRunID)
	if err != nil {
		return EvaluationReport{}, fmt.Errorf("evaluation report backtest run id: %w", err)
	}
	datasetID, err := NewDatasetReferenceID(params.DatasetID)
	if err != nil {
		return EvaluationReport{}, fmt.Errorf("evaluation report dataset reference: %w", err)
	}
	decision, err := NewEvaluationDecision(params.Decision.String())
	if err != nil {
		return EvaluationReport{}, errors.New("evaluation report decision is required")
	}
	metrics, hasMetrics, err := canonicalOptionalVersionedMetrics(params.Metrics)
	if err != nil {
		return EvaluationReport{}, err
	}
	if !hasMetrics {
		metrics = nil
	}
	failureReasons, err := canonicalAuditReasonCodes(params.FailureReasons)
	if err != nil {
		return EvaluationReport{}, fmt.Errorf("evaluation report failure reasons: %w", err)
	}
	createdAt, err := NewEvaluationReportTime(params.CreatedAt)
	if err != nil {
		return EvaluationReport{}, err
	}

	return EvaluationReport{
		EvaluationID:         evaluationID,
		StrategyID:           strategyID,
		StrategyVersion:      strategyVersion,
		StrategyArtifactHash: strategyArtifactHash,
		BacktestRunID:        backtestRunID,
		DatasetID:            datasetID,
		Decision:             decision,
		Metrics:              metrics,
		FailureReasons:       failureReasons,
		Notes:                strings.TrimSpace(params.Notes),
		CreatedAt:            createdAt,
	}, nil
}

// String returns the canonical string value for a dataset reference id.
func (id DatasetReferenceID) String() string { return string(id) }

// String returns the canonical string value for a backtest run id.
func (id BacktestRunID) String() string { return string(id) }

// String returns the canonical string value for an evaluation report id.
func (id EvaluationReportID) String() string { return string(id) }

// String returns the canonical string value for a backtest run status.
func (s BacktestRunStatus) String() string { return string(s) }

// String returns the canonical string value for an evaluation decision.
func (d EvaluationDecision) String() string { return string(d) }

// Time returns the UTC time value for a dataset reference timestamp.
func (t DatasetReferenceTime) Time() time.Time { return canonicalUTC(time.Time(t)) }

// Time returns the UTC time value for a backtest run timestamp.
func (t BacktestRunTime) Time() time.Time { return canonicalUTC(time.Time(t)) }

// Time returns the UTC time value for an evaluation report timestamp.
func (t EvaluationReportTime) Time() time.Time { return canonicalUTC(time.Time(t)) }

// MarshalJSON renders compact deterministic metrics JSON.
func (m VersionedMetrics) MarshalJSON() ([]byte, error) {
	type alias VersionedMetrics
	return json.Marshal(alias(m))
}

func versionedMetricsParamsFromPointer(metrics *VersionedMetrics) VersionedMetricsParams {
	if metrics == nil {
		return VersionedMetricsParams{}
	}

	return VersionedMetricsParams{
		SchemaVersion:                 metrics.SchemaVersion,
		TradeCount:                    metrics.TradeCount,
		BlockedGovernorDecisionCount:  metrics.BlockedGovernorDecisionCount,
		RejectedGovernorDecisionCount: metrics.RejectedGovernorDecisionCount,
		MaxDrawdown:                   metrics.MaxDrawdown,
	}
}

func canonicalOptionalVersionedMetrics(metrics *VersionedMetrics) (*VersionedMetrics, bool, error) {
	params := versionedMetricsParamsFromPointer(metrics)
	if params.SchemaVersion == "" &&
		params.TradeCount == nil &&
		params.BlockedGovernorDecisionCount == nil &&
		params.RejectedGovernorDecisionCount == nil &&
		params.MaxDrawdown == nil {
		return nil, false, nil
	}

	canonical, err := NewVersionedMetrics(params)
	if err != nil {
		return nil, false, err
	}

	return canonical, true, nil
}

func canonicalDatasetEntityTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, errors.New("dataset reference entity types are required")
	}

	canonical := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			return nil, errors.New("dataset reference entity type is required")
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		canonical = append(canonical, normalized)
	}
	slices.Sort(canonical)

	return canonical, nil
}

func canonicalDatasetInstruments(values []Instrument) ([]Instrument, error) {
	if len(values) == 0 {
		return nil, errors.New("dataset reference instruments are required")
	}

	canonical := make([]Instrument, 0, len(values))
	seen := map[string]struct{}{}
	for idx, value := range values {
		instrument, err := NewInstrument(InstrumentParams(value))
		if err != nil {
			return nil, fmt.Errorf("dataset reference instrument %d: %w", idx, err)
		}
		key := stableDatasetInstrumentKey(instrument)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		canonical = append(canonical, instrument)
	}
	slices.SortStableFunc(canonical, func(left, right Instrument) int {
		return strings.Compare(stableDatasetInstrumentKey(left), stableDatasetInstrumentKey(right))
	})

	return canonical, nil
}

func canonicalDatasetTimeframes(values []Timeframe) ([]Timeframe, error) {
	if len(values) == 0 {
		return nil, errors.New("dataset reference timeframes are required")
	}

	canonical := make([]Timeframe, 0, len(values))
	seen := map[string]struct{}{}
	for idx, value := range values {
		timeframe, err := NewTimeframe(value.String())
		if err != nil {
			return nil, fmt.Errorf("dataset reference timeframe %d: %w", idx, err)
		}
		if _, ok := seen[timeframe.String()]; ok {
			continue
		}
		seen[timeframe.String()] = struct{}{}
		canonical = append(canonical, timeframe)
	}
	slices.SortStableFunc(canonical, func(left, right Timeframe) int {
		return strings.Compare(left.String(), right.String())
	})

	return canonical, nil
}

func canonicalDatasetEvidence(hashes []string, replayChecksum string) ([]string, string, error) {
	canonicalReplayChecksum := strings.TrimSpace(replayChecksum)
	canonicalHashes := make([]string, 0, len(hashes))
	seen := map[string]struct{}{}
	for _, hash := range hashes {
		normalized := strings.TrimSpace(hash)
		if normalized == "" {
			return nil, "", errors.New("dataset reference source data hash is required")
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		canonicalHashes = append(canonicalHashes, normalized)
	}
	slices.Sort(canonicalHashes)

	if len(canonicalHashes) == 0 && canonicalReplayChecksum == "" {
		return nil, "", errors.New("dataset reference source data hashes or replay checksum is required")
	}

	return canonicalHashes, canonicalReplayChecksum, nil
}

func canonicalOptionalIdentityTriple(
	id string,
	version string,
	hash string,
	prefix string,
) (string, string, string, error) {
	normalizedID := strings.TrimSpace(id)
	normalizedVersion := strings.TrimSpace(version)
	normalizedHash := strings.TrimSpace(hash)
	if normalizedID == "" && normalizedVersion == "" && normalizedHash == "" {
		return "", "", "", nil
	}
	if normalizedID == "" {
		return "", "", "", errors.New(prefix + " id is required")
	}
	if normalizedVersion == "" {
		return "", "", "", errors.New(prefix + " version is required")
	}
	if normalizedHash == "" {
		return "", "", "", errors.New(prefix + " hash is required")
	}

	return normalizedID, normalizedVersion, normalizedHash, nil
}

func canonicalModelOrAssumptions(
	modelID string,
	assumptions map[string]string,
	prefix string,
) (string, map[string]string, error) {
	normalizedModelID := strings.TrimSpace(modelID)
	canonicalAssumptions, err := canonicalAuditMetadata(assumptions)
	if err != nil {
		return "", nil, fmt.Errorf("%s assumptions: %w", prefix, err)
	}
	if assumptions == nil {
		canonicalAssumptions = nil
	}
	if normalizedModelID == "" && len(canonicalAssumptions) == 0 {
		return "", nil, errors.New(prefix + " model id or assumptions are required")
	}

	return normalizedModelID, canonicalAssumptions, nil
}

func canonicalBacktestRunLifecycle(
	params BacktestRunParams,
) (BacktestRunStatus, *VersionedMetrics, string, string, BacktestRunTime, BacktestRunTime, error) {
	status, err := NewBacktestRunStatus(params.Status.String())
	if err != nil {
		return "", nil, "", "", BacktestRunTime{}, BacktestRunTime{}, errors.New(
			"backtest run status is required",
		)
	}
	metrics, hasMetrics, err := canonicalOptionalVersionedMetrics(params.Metrics)
	if err != nil {
		return "", nil, "", "", BacktestRunTime{}, BacktestRunTime{}, err
	}
	if !hasMetrics {
		metrics = nil
	}
	failureReason := strings.TrimSpace(params.FailureReason)
	failureDetails := strings.TrimSpace(params.FailureDetails)
	if status == BacktestRunStatusFailed && failureReason == "" {
		return "", nil, "", "", BacktestRunTime{}, BacktestRunTime{}, errors.New(
			"backtest run failure reason is required when status is failed",
		)
	}
	if status != BacktestRunStatusFailed {
		failureReason = ""
		failureDetails = ""
	}
	createdAt, err := NewBacktestRunTime(params.CreatedAt, "backtest run created at")
	if err != nil {
		return "", nil, "", "", BacktestRunTime{}, BacktestRunTime{}, err
	}
	updatedAt, err := NewBacktestRunTime(params.UpdatedAt, "backtest run updated at")
	if err != nil {
		return "", nil, "", "", BacktestRunTime{}, BacktestRunTime{}, err
	}
	if updatedAt.Time().Before(createdAt.Time()) {
		return "", nil, "", "", BacktestRunTime{}, BacktestRunTime{}, errors.New(
			"backtest run updated at must be equal to or after created at",
		)
	}

	return status, metrics, failureReason, failureDetails, createdAt, updatedAt, nil
}

func canonicalOptionalNonNegativeInt(value *int, field string) (int, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	if *value < 0 {
		return 0, false, errors.New(field + " must be zero or greater")
	}

	return *value, true, nil
}

func canonicalOptionalNonNegativeFloat64(value *float64, field string) (float64, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return 0, false, errors.New(field + " must be finite and zero or greater")
	}

	return *value, true, nil
}

func stableDatasetInstrumentKey(instrument Instrument) string {
	return instrument.Venue.String() + "|" + instrument.Symbol.String() + "|" + instrument.AssetClass.String()
}

func (s BacktestRunStatus) isValid() bool {
	switch s {
	case BacktestRunStatusPending,
		BacktestRunStatusRunning,
		BacktestRunStatusCompleted,
		BacktestRunStatusFailed:
		return true
	default:
		return false
	}
}

func (d EvaluationDecision) isValid() bool {
	switch d {
	case EvaluationDecisionPromoteToPaperCandidate,
		EvaluationDecisionReject,
		EvaluationDecisionNeedsReview:
		return true
	default:
		return false
	}
}
