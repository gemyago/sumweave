package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	maxAuditMetadataEntries   = 16
	maxAuditMetadataKeyLength = 64
	maxAuditMetadataValLength = 256
	maxAuditReasonCodeCount   = 16
	maxAuditReasonCodeLength  = 64
)

var auditReasonCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// DecisionMode identifies the deterministic operating mode for audit records.
type DecisionMode string

const (
	DecisionModePaper    DecisionMode = "paper"
	DecisionModeBacktest DecisionMode = "backtest"
	DecisionModeLive     DecisionMode = "live"
)

// DecisionTraceID identifies a canonical decision trace record.
type DecisionTraceID string

// OrderIntentID identifies a canonical order intent record.
type OrderIntentID string

// DecisionTraceResult identifies a supported decision trace outcome.
type DecisionTraceResult string

const (
	DecisionTraceResultNoAction            DecisionTraceResult = "no_action"
	DecisionTraceResultIntentCreated       DecisionTraceResult = "intent_created"
	DecisionTraceResultBlockedBeforeIntent DecisionTraceResult = "blocked_before_intent"
	DecisionTraceResultError               DecisionTraceResult = "error"
)

// OrderType identifies a supported order intent order type.
type OrderType string

const (
	OrderTypeLimit OrderType = "limit"
)

// OrderIntentStatus identifies a supported order intent state.
type OrderIntentStatus string

const (
	OrderIntentStatusCreated          OrderIntentStatus = "created"
	OrderIntentStatusSentToGovernor   OrderIntentStatus = "sent_to_governor"
	OrderIntentStatusApproved         OrderIntentStatus = "approved"
	OrderIntentStatusRejected         OrderIntentStatus = "rejected"
	OrderIntentStatusBlocked          OrderIntentStatus = "blocked"
	OrderIntentStatusExecutionCreated OrderIntentStatus = "execution_created"
)

// DecisionTraceTime identifies a canonical decision trace timestamp.
type DecisionTraceTime time.Time

// OrderIntentTime identifies a canonical order intent timestamp.
type OrderIntentTime time.Time

// DecisionTrace captures compact deterministic strategy decision audit context.
type DecisionTrace struct {
	TraceID              DecisionTraceID
	Mode                 DecisionMode
	DecisionTime         DecisionTraceTime
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	Instrument           Instrument
	Timeframe            Timeframe
	DatasetReference     string
	RunReference         string
	InputRange           TimeRange
	AnalyticsReference   string
	DataQuality          DataQuality
	EvaluatorName        string
	EvaluatorVersion     string
	Result               DecisionTraceResult
	ReasonCodes          []string
	Metadata             map[string]string
}

// OrderIntent captures deterministic execution intent before governor evaluation.
type OrderIntent struct {
	IntentID                 OrderIntentID
	TraceID                  DecisionTraceID
	StrategyID               string
	StrategyVersion          string
	StrategyArtifactHash     string
	Mode                     DecisionMode
	Instrument               Instrument
	Timeframe                Timeframe
	ActionKind               CandidateActionKind
	OrderType                OrderType
	RequestedQuantity        float64
	RequestedNotional        float64
	RequestedLimitPrice      *float64
	ReduceOnly               bool
	SourceReasonCode         string
	CandidateActionReference string
	CreatedTime              OrderIntentTime
	Status                   OrderIntentStatus
	Metadata                 map[string]string
}

// DecisionTraceParams holds inputs for a canonical decision trace.
type DecisionTraceParams struct {
	TraceID              string
	Mode                 DecisionMode
	DecisionTime         time.Time
	StrategyID           string
	StrategyVersion      string
	StrategyArtifactHash string
	Instrument           Instrument
	Timeframe            Timeframe
	DatasetReference     string
	RunReference         string
	InputRange           TimeRange
	AnalyticsReference   string
	DataQuality          DataQuality
	EvaluatorName        string
	EvaluatorVersion     string
	Result               DecisionTraceResult
	ReasonCodes          []string
	Metadata             map[string]string
}

// OrderIntentParams holds inputs for a canonical order intent.
type OrderIntentParams struct {
	IntentID                 string
	TraceID                  string
	StrategyID               string
	StrategyVersion          string
	StrategyArtifactHash     string
	Mode                     DecisionMode
	Instrument               Instrument
	Timeframe                Timeframe
	ActionKind               CandidateActionKind
	OrderType                OrderType
	RequestedQuantity        float64
	RequestedNotional        float64
	RequestedLimitPrice      *float64
	ReduceOnly               bool
	SourceReasonCode         string
	CandidateActionReference string
	CreatedTime              time.Time
	Status                   OrderIntentStatus
	Metadata                 map[string]string
}

// NewDecisionMode validates and canonicalizes a decision mode.
func NewDecisionMode(value string) (DecisionMode, error) {
	normalized := DecisionMode(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid decision mode %q", value)
	}

	return normalized, nil
}

// NewDecisionTraceID validates and canonicalizes a decision trace identifier.
func NewDecisionTraceID(value string) (DecisionTraceID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("decision trace id is required")
	}

	return DecisionTraceID(normalized), nil
}

// NewOrderIntentID validates and canonicalizes an order intent identifier.
func NewOrderIntentID(value string) (OrderIntentID, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("order intent id is required")
	}

	return OrderIntentID(normalized), nil
}

// NewDecisionTraceResult validates and canonicalizes a decision trace result.
func NewDecisionTraceResult(value string) (DecisionTraceResult, error) {
	normalized := DecisionTraceResult(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid decision trace result %q", value)
	}

	return normalized, nil
}

// NewOrderType validates and canonicalizes an order type.
func NewOrderType(value string) (OrderType, error) {
	normalized := OrderType(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid order type %q", value)
	}

	return normalized, nil
}

// NewOrderIntentStatus validates and canonicalizes an order intent status.
func NewOrderIntentStatus(value string) (OrderIntentStatus, error) {
	normalized := OrderIntentStatus(strings.ToLower(strings.TrimSpace(value)))
	if !normalized.isValid() {
		return "", fmt.Errorf("invalid order intent status %q", value)
	}

	return normalized, nil
}

// NewDecisionTraceTime validates and canonicalizes a decision trace time.
func NewDecisionTraceTime(value time.Time) (DecisionTraceTime, error) {
	if value.IsZero() {
		return DecisionTraceTime{}, errors.New("decision trace time is required")
	}

	return DecisionTraceTime(canonicalUTC(value)), nil
}

// NewOrderIntentTime validates and canonicalizes an order intent time.
func NewOrderIntentTime(value time.Time) (OrderIntentTime, error) {
	if value.IsZero() {
		return OrderIntentTime{}, errors.New("order intent time is required")
	}

	return OrderIntentTime(canonicalUTC(value)), nil
}

// NewDecisionTrace validates and canonicalizes a canonical decision trace.
func NewDecisionTrace(params DecisionTraceParams) (DecisionTrace, error) {
	traceID, err := NewDecisionTraceID(params.TraceID)
	if err != nil {
		return DecisionTrace{}, err
	}

	mode, err := NewDecisionMode(params.Mode.String())
	if err != nil {
		return DecisionTrace{}, errors.New("decision trace mode is required")
	}

	decisionTime, err := NewDecisionTraceTime(params.DecisionTime)
	if err != nil {
		return DecisionTrace{}, err
	}

	instrument, err := canonicalAuditInstrument(params.Instrument)
	if err != nil {
		return DecisionTrace{}, fmt.Errorf("decision trace instrument: %w", err)
	}

	timeframe, err := NewTimeframe(params.Timeframe.String())
	if err != nil {
		return DecisionTrace{}, errors.New("decision trace timeframe is required")
	}

	inputRange, err := NewTimeRange(params.InputRange.Start, params.InputRange.End)
	if err != nil {
		return DecisionTrace{}, fmt.Errorf("decision trace input range: %w", err)
	}

	if !params.DataQuality.isValid() {
		return DecisionTrace{}, errors.New("decision trace data quality is required")
	}

	result, err := NewDecisionTraceResult(params.Result.String())
	if err != nil {
		return DecisionTrace{}, errors.New("decision trace result is required")
	}

	reasonCodes, err := canonicalAuditReasonCodes(params.ReasonCodes)
	if err != nil {
		return DecisionTrace{}, fmt.Errorf("decision trace reason codes: %w", err)
	}

	metadata, err := canonicalAuditMetadata(params.Metadata)
	if err != nil {
		return DecisionTrace{}, fmt.Errorf("decision trace metadata: %w", err)
	}

	strategyID, err := requiredAuditText("decision trace strategy id", params.StrategyID)
	if err != nil {
		return DecisionTrace{}, err
	}
	strategyVersion, err := requiredAuditText(
		"decision trace strategy version",
		params.StrategyVersion,
	)
	if err != nil {
		return DecisionTrace{}, err
	}
	strategyArtifactHash, err := requiredAuditText(
		"decision trace strategy artifact hash",
		params.StrategyArtifactHash,
	)
	if err != nil {
		return DecisionTrace{}, err
	}
	evaluatorName, err := requiredAuditText("decision trace evaluator name", params.EvaluatorName)
	if err != nil {
		return DecisionTrace{}, err
	}
	evaluatorVersion, err := requiredAuditText(
		"decision trace evaluator version",
		params.EvaluatorVersion,
	)
	if err != nil {
		return DecisionTrace{}, err
	}

	return DecisionTrace{
		TraceID:              traceID,
		Mode:                 mode,
		DecisionTime:         decisionTime,
		StrategyID:           strategyID,
		StrategyVersion:      strategyVersion,
		StrategyArtifactHash: strategyArtifactHash,
		Instrument:           instrument,
		Timeframe:            timeframe,
		DatasetReference:     strings.TrimSpace(params.DatasetReference),
		RunReference:         strings.TrimSpace(params.RunReference),
		InputRange:           inputRange,
		AnalyticsReference:   strings.TrimSpace(params.AnalyticsReference),
		DataQuality:          params.DataQuality,
		EvaluatorName:        evaluatorName,
		EvaluatorVersion:     evaluatorVersion,
		Result:               result,
		ReasonCodes:          reasonCodes,
		Metadata:             metadata,
	}, nil
}

// NewOrderIntent validates and canonicalizes a canonical order intent.
func NewOrderIntent(params OrderIntentParams) (OrderIntent, error) {
	strategyID, strategyVersion, strategyArtifactHash, err := canonicalOrderIntentStrategyFields(params)
	if err != nil {
		return OrderIntent{}, err
	}

	intentID, err := NewOrderIntentID(params.IntentID)
	if err != nil {
		return OrderIntent{}, err
	}

	traceID, err := NewDecisionTraceID(params.TraceID)
	if err != nil {
		return OrderIntent{}, errors.New("order intent trace id is required")
	}

	mode, err := NewDecisionMode(params.Mode.String())
	if err != nil {
		return OrderIntent{}, errors.New("order intent mode is required")
	}

	instrument, err := canonicalAuditInstrument(params.Instrument)
	if err != nil {
		return OrderIntent{}, fmt.Errorf("order intent instrument: %w", err)
	}

	timeframe, err := NewTimeframe(params.Timeframe.String())
	if err != nil {
		return OrderIntent{}, errors.New("order intent timeframe is required")
	}

	actionKind, err := NewCandidateActionKind(params.ActionKind.String())
	if err != nil {
		return OrderIntent{}, errors.New("order intent action kind is required")
	}

	orderType, err := NewOrderType(params.OrderType.String())
	if err != nil {
		return OrderIntent{}, errors.New("order intent order type is required")
	}

	if amountErr := validateOrderIntentAmounts(
		params.RequestedQuantity,
		params.RequestedNotional,
	); amountErr != nil {
		return OrderIntent{}, amountErr
	}

	limitPrice, err := canonicalLimitPrice(orderType, params.RequestedLimitPrice)
	if err != nil {
		return OrderIntent{}, err
	}

	createdAt, err := NewOrderIntentTime(params.CreatedTime)
	if err != nil {
		return OrderIntent{}, err
	}

	status, err := NewOrderIntentStatus(params.Status.String())
	if err != nil {
		return OrderIntent{}, errors.New("order intent status is required")
	}

	metadata, err := canonicalAuditMetadata(params.Metadata)
	if err != nil {
		return OrderIntent{}, fmt.Errorf("order intent metadata: %w", err)
	}

	sourceReasonCode, err := canonicalAuditReasonCode(params.SourceReasonCode)
	if err != nil && strings.TrimSpace(params.SourceReasonCode) != "" {
		return OrderIntent{}, fmt.Errorf("order intent source reason code: %w", err)
	}

	return OrderIntent{
		IntentID:                 intentID,
		TraceID:                  traceID,
		StrategyID:               strategyID,
		StrategyVersion:          strategyVersion,
		StrategyArtifactHash:     strategyArtifactHash,
		Mode:                     mode,
		Instrument:               instrument,
		Timeframe:                timeframe,
		ActionKind:               actionKind,
		OrderType:                orderType,
		RequestedQuantity:        params.RequestedQuantity,
		RequestedNotional:        params.RequestedNotional,
		RequestedLimitPrice:      limitPrice,
		ReduceOnly:               params.ReduceOnly,
		SourceReasonCode:         sourceReasonCode,
		CandidateActionReference: strings.TrimSpace(params.CandidateActionReference),
		CreatedTime:              createdAt,
		Status:                   status,
		Metadata:                 metadata,
	}, nil
}

func canonicalAuditInstrument(instrument Instrument) (Instrument, error) {
	return NewInstrument(InstrumentParams(instrument))
}

func canonicalLimitPrice(orderType OrderType, value *float64) (*float64, error) {
	if orderType != OrderTypeLimit {
		return nil, errors.New("order intent order type is unsupported")
	}
	if value == nil {
		return nil, errors.New("order intent requested limit price is required for limit orders")
	}
	if !isFiniteFloat64(*value) || *value <= 0 {
		return nil, errors.New("order intent requested limit price must be finite and positive")
	}

	normalized := *value
	return &normalized, nil
}

func requiredAuditText(field string, value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New(field + " is required")
	}

	return normalized, nil
}

func canonicalAuditReasonCodes(values []string) ([]string, error) {
	if len(values) > maxAuditReasonCodeCount {
		return nil, fmt.Errorf("too many reason codes: max %d", maxAuditReasonCodeCount)
	}

	canonical := make([]string, 0, len(values))
	for idx, value := range values {
		normalized, err := canonicalAuditReasonCode(value)
		if err != nil {
			return nil, fmt.Errorf("reason code %d: %w", idx, err)
		}
		canonical = append(canonical, normalized)
	}

	return canonical, nil
}

func canonicalAuditReasonCode(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("reason code is required")
	}
	if len(normalized) > maxAuditReasonCodeLength {
		return "", fmt.Errorf("reason code exceeds %d characters", maxAuditReasonCodeLength)
	}
	if !auditReasonCodePattern.MatchString(normalized) {
		return "", fmt.Errorf("reason code %q must use uppercase snake-case", value)
	}

	return normalized, nil
}

func canonicalAuditMetadata(values map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return map[string]string{}, nil
	}
	if len(values) > maxAuditMetadataEntries {
		return nil, fmt.Errorf("too many metadata entries: max %d", maxAuditMetadataEntries)
	}

	canonical := make(map[string]string, len(values))
	for key, value := range values {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			return nil, errors.New("metadata key is required")
		}
		if len(normalizedKey) > maxAuditMetadataKeyLength {
			return nil, fmt.Errorf("metadata key %q exceeds %d characters", normalizedKey, maxAuditMetadataKeyLength)
		}

		normalizedValue := strings.TrimSpace(value)
		if normalizedValue == "" {
			return nil, fmt.Errorf("metadata value for key %q is required", normalizedKey)
		}
		if len(normalizedValue) > maxAuditMetadataValLength {
			return nil, fmt.Errorf(
				"metadata value for key %q exceeds %d characters",
				normalizedKey,
				maxAuditMetadataValLength,
			)
		}

		canonical[normalizedKey] = normalizedValue
	}

	return canonical, nil
}

func canonicalOrderIntentStrategyFields(
	params OrderIntentParams,
) (string, string, string, error) {
	strategyID, err := requiredAuditText("order intent strategy id", params.StrategyID)
	if err != nil {
		return "", "", "", err
	}
	strategyVersion, err := requiredAuditText(
		"order intent strategy version",
		params.StrategyVersion,
	)
	if err != nil {
		return "", "", "", err
	}
	strategyArtifactHash, err := requiredAuditText(
		"order intent strategy artifact hash",
		params.StrategyArtifactHash,
	)
	if err != nil {
		return "", "", "", err
	}

	return strategyID, strategyVersion, strategyArtifactHash, nil
}

func validateOrderIntentAmounts(quantity float64, notional float64) error {
	if !isFiniteFloat64(quantity) || quantity < 0 {
		return errors.New("order intent requested quantity must be finite and zero or greater")
	}
	if !isFiniteFloat64(notional) || notional < 0 {
		return errors.New("order intent requested notional must be finite and zero or greater")
	}
	if quantity == 0 && notional == 0 {
		return errors.New("order intent requested quantity or requested notional is required")
	}

	return nil
}

func (m DecisionMode) isValid() bool {
	switch m {
	case DecisionModePaper, DecisionModeBacktest, DecisionModeLive:
		return true
	default:
		return false
	}
}

func (r DecisionTraceResult) isValid() bool {
	switch r {
	case DecisionTraceResultNoAction,
		DecisionTraceResultIntentCreated,
		DecisionTraceResultBlockedBeforeIntent,
		DecisionTraceResultError:
		return true
	default:
		return false
	}
}

func (t OrderType) isValid() bool {
	switch t {
	case OrderTypeLimit:
		return true
	default:
		return false
	}
}

func (s OrderIntentStatus) isValid() bool {
	switch s {
	case OrderIntentStatusCreated,
		OrderIntentStatusSentToGovernor,
		OrderIntentStatusApproved,
		OrderIntentStatusRejected,
		OrderIntentStatusBlocked,
		OrderIntentStatusExecutionCreated:
		return true
	default:
		return false
	}
}

func (m DecisionMode) String() string {
	return string(m)
}

func (r DecisionTraceResult) String() string {
	return string(r)
}

func (t OrderType) String() string {
	return string(t)
}

func (s OrderIntentStatus) String() string {
	return string(s)
}

// Time returns the time value for a canonical decision trace timestamp.
func (t DecisionTraceTime) Time() time.Time {
	return time.Time(t)
}

// Time returns the time value for a canonical order intent timestamp.
func (t OrderIntentTime) Time() time.Time {
	return time.Time(t)
}
