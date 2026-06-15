package governor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/gemyago/signal-foundry/runtime/domain"
)

const (
	qualityRankSuspect   = 0
	qualityRankRaw       = 1
	qualityRankValidated = 2
)

// ErrValidation marks rejected inputs that fail governor-layer validation.
var ErrValidation = errors.New("governor validation failed")

// Policy defines deterministic governor evaluation rules.
type Policy struct {
	AllowedModes                      []domain.DecisionMode
	AllowedVenues                     []domain.Venue
	AllowedInstruments                []domain.Instrument
	AllowedStrategyIDs                []string
	AllowedActionKinds                []domain.CandidateActionKind
	MinimumQuality                    domain.DataQuality
	BlockNewRisk                      bool
	MaximumOrderNotional              float64
	MaximumStrategyExposureNotional   float64
	MaximumInstrumentExposureNotional float64
	MaximumApprovedCount              int
}

// IntentInput configures intent-based deterministic governor evaluation.
type IntentInput struct {
	CandidateAction                   domain.CandidateAction
	Intent                            domain.OrderIntent
	CurrentStrategyExposureNotional   float64
	CurrentInstrumentExposureNotional float64
	GovernorPolicyID                  string
	GovernorPolicyVersion             string
	GovernorPolicyHash                string
}

// EvaluateRequest configures deterministic governor evaluation.
type EvaluateRequest struct {
	CandidateActions []domain.CandidateAction
	IntentInputs     []IntentInput
	Policy           Policy
}

// EvaluateResult returns ordered canonical governor decisions.
type EvaluateResult struct {
	Decisions []domain.GovernorDecision
}

// Service evaluates strategy candidate actions against deterministic policy.
type Service struct{}

// NewService creates a dependency-free governor service.
func NewService() *Service {
	return &Service{}
}

// Evaluate evaluates canonical candidate actions or intents against policy rules.
func (s *Service) Evaluate(
	_ context.Context,
	request EvaluateRequest,
) (EvaluateResult, error) {
	canonicalPolicy, err := canonicalizePolicy(request.Policy)
	if err != nil {
		return EvaluateResult{}, err
	}

	canonicalActions, err := canonicalizeActions(request.CandidateActions)
	if err != nil && len(request.IntentInputs) == 0 {
		return EvaluateResult{}, err
	}

	canonicalIntentInputs, err := canonicalizeIntentInputs(request.IntentInputs)
	if err != nil {
		return EvaluateResult{}, err
	}

	if len(canonicalActions) > 0 && len(canonicalIntentInputs) > 0 {
		return EvaluateResult{}, invalidIntentError(
			"candidate actions and intent inputs are mutually exclusive",
		)
	}

	if len(canonicalIntentInputs) > 0 {
		return evaluateIntentInputs(canonicalIntentInputs, canonicalPolicy)
	}

	slices.SortStableFunc(canonicalActions, func(left, right domain.CandidateAction) int {
		return left.DecisionTime.Time().Compare(right.DecisionTime.Time())
	})

	decisions := make([]domain.GovernorDecision, 0, len(canonicalActions))
	approvedCount := 0

	for _, action := range canonicalActions {
		status, reason := evaluateAction(action, canonicalPolicy, approvedCount)
		if status == domain.GovernorDecisionStatusApproved {
			approvedCount++
		}

		decision, buildErr := domain.NewGovernorDecision(domain.GovernorDecisionParams{
			CandidateAction: action,
			Status:          status,
			Reason:          reason,
			DecisionTime:    action.DecisionTime.Time(),
		})
		if buildErr != nil {
			return EvaluateResult{}, validationError(
				fmt.Sprintf("build governor decision: %s", buildErr.Error()),
			)
		}

		decisions = append(decisions, decision)
	}

	return EvaluateResult{Decisions: decisions}, nil
}

type canonicalPolicy struct {
	allowedModes                      map[domain.DecisionMode]struct{}
	allowedVenues                     map[domain.Venue]struct{}
	allowedInstruments                map[string]struct{}
	allowedStrategyIDs                map[string]struct{}
	allowedActionKinds                map[domain.CandidateActionKind]struct{}
	minimumQualityRank                int
	blockNewRisk                      bool
	maximumOrderNotional              float64
	maximumStrategyExposureNotional   float64
	maximumInstrumentExposureNotional float64
	maximumApprovedCount              int
}

type canonicalIntentInput struct {
	candidateAction                   domain.CandidateAction
	intent                            domain.OrderIntent
	currentStrategyExposureNotional   float64
	currentInstrumentExposureNotional float64
	orderNotional                     float64
}

func canonicalizePolicy(policy Policy) (canonicalPolicy, error) {
	allowedModes, err := canonicalizeAllowedModes(policy.AllowedModes)
	if err != nil {
		return canonicalPolicy{}, err
	}
	allowedVenues, err := canonicalizeAllowedVenues(policy.AllowedVenues)
	if err != nil {
		return canonicalPolicy{}, err
	}
	allowedInstruments, err := canonicalizeAllowedInstruments(policy.AllowedInstruments)
	if err != nil {
		return canonicalPolicy{}, err
	}
	allowedStrategyIDs, err := canonicalizeAllowedStrategyIDs(policy.AllowedStrategyIDs)
	if err != nil {
		return canonicalPolicy{}, err
	}
	allowedActionKinds, err := canonicalizeAllowedActionKinds(policy.AllowedActionKinds)
	if err != nil {
		return canonicalPolicy{}, err
	}

	minimumQualityRank, err := minimumQualityRank(policy.MinimumQuality)
	if err != nil {
		return canonicalPolicy{}, validationError(err.Error())
	}

	thresholdErr := validatePolicyThresholds(policy)
	if thresholdErr != nil {
		return canonicalPolicy{}, thresholdErr
	}

	return canonicalPolicy{
		allowedModes:                      allowedModes,
		allowedVenues:                     allowedVenues,
		allowedInstruments:                allowedInstruments,
		allowedStrategyIDs:                allowedStrategyIDs,
		allowedActionKinds:                allowedActionKinds,
		minimumQualityRank:                minimumQualityRank,
		blockNewRisk:                      policy.BlockNewRisk,
		maximumOrderNotional:              policy.MaximumOrderNotional,
		maximumStrategyExposureNotional:   policy.MaximumStrategyExposureNotional,
		maximumInstrumentExposureNotional: policy.MaximumInstrumentExposureNotional,
		maximumApprovedCount:              policy.MaximumApprovedCount,
	}, nil
}

func canonicalizeActions(actions []domain.CandidateAction) ([]domain.CandidateAction, error) {
	canonicalActions := make([]domain.CandidateAction, 0, len(actions))

	for _, action := range actions {
		canonicalAction, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy:     action.Strategy,
			Kind:         action.Kind,
			DecisionTime: action.DecisionTime.Time(),
			InputRange:   action.InputRange,
			Quality:      action.Quality,
		})
		if err != nil {
			return nil, validationError(fmt.Sprintf("candidate action: %s", err.Error()))
		}

		canonicalActions = append(canonicalActions, canonicalAction)
	}

	return canonicalActions, nil
}

func canonicalizeIntentInputs(inputs []IntentInput) ([]canonicalIntentInput, error) {
	canonicalInputs := make([]canonicalIntentInput, 0, len(inputs))

	for _, input := range inputs {
		candidateAction, intent, err := canonicalizeIntentInput(input)
		if err != nil {
			return nil, err
		}
		alignmentErr := validateIntentInputAlignment(candidateAction, intent)
		if alignmentErr != nil {
			return nil, alignmentErr
		}
		contextErr := validateIntentInputContext(input)
		if contextErr != nil {
			return nil, contextErr
		}
		orderNotional, err := requestedOrderNotional(intent)
		if err != nil {
			return nil, invalidIntentError(err.Error())
		}

		canonicalInputs = append(canonicalInputs, canonicalIntentInput{
			candidateAction:                   candidateAction,
			intent:                            intent,
			currentStrategyExposureNotional:   input.CurrentStrategyExposureNotional,
			currentInstrumentExposureNotional: input.CurrentInstrumentExposureNotional,
			orderNotional:                     orderNotional,
		})
	}

	return canonicalInputs, nil
}

func evaluateAction(
	action domain.CandidateAction,
	policy canonicalPolicy,
	approvedCount int,
) (domain.GovernorDecisionStatus, domain.GovernorDecisionReason) {
	if _, ok := policy.allowedActionKinds[action.Kind]; !ok {
		return domain.GovernorDecisionStatusRejected,
			domain.GovernorDecisionReasonActionKindNotAllowed
	}

	actionQualityRank := candidateQualityRank(action.Quality)
	if actionQualityRank < policy.minimumQualityRank {
		return domain.GovernorDecisionStatusRejected,
			domain.GovernorDecisionReasonDataQualityTooLow
	}

	if approvedCount >= policy.maximumApprovedCount {
		return domain.GovernorDecisionStatusBlocked,
			domain.GovernorDecisionReasonApprovalLimitReached
	}

	return domain.GovernorDecisionStatusApproved, domain.GovernorDecisionReasonOK
}

func evaluateIntentInputs(
	inputs []canonicalIntentInput,
	policy canonicalPolicy,
) (EvaluateResult, error) {
	slices.SortStableFunc(inputs, func(left, right canonicalIntentInput) int {
		if compare := left.intent.CreatedTime.Time().Compare(right.intent.CreatedTime.Time()); compare != 0 {
			return compare
		}

		return strings.Compare(string(left.intent.IntentID), string(right.intent.IntentID))
	})

	decisions := make([]domain.GovernorDecision, 0, len(inputs))
	approvedCount := 0

	for _, input := range inputs {
		status, reason := evaluateIntentInput(input, policy, approvedCount)
		if status == domain.GovernorDecisionStatusApproved {
			approvedCount++
		}

		decision, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
			CandidateAction: input.candidateAction,
			Status:          status,
			Reason:          reason,
			DecisionTime:    input.candidateAction.DecisionTime.Time(),
		})
		if err != nil {
			return EvaluateResult{}, validationError(
				fmt.Sprintf("build governor decision: %s", err.Error()),
			)
		}

		decisions = append(decisions, decision)
	}

	return EvaluateResult{Decisions: decisions}, nil
}

func evaluateIntentInput(
	input canonicalIntentInput,
	policy canonicalPolicy,
	approvedCount int,
) (domain.GovernorDecisionStatus, domain.GovernorDecisionReason) {
	if status, reason, matched := evaluateIntentScope(input, policy); matched {
		return status, reason
	}
	if status, reason, matched := evaluateIntentQualityAndRisk(input, policy); matched {
		return status, reason
	}

	projectedStrategyExposure := projectedExposureNotional(
		input.currentStrategyExposureNotional,
		input.candidateAction.Kind,
		input.orderNotional,
	)
	if policy.maximumStrategyExposureNotional > 0 &&
		projectedStrategyExposure > policy.maximumStrategyExposureNotional {
		return domain.GovernorDecisionStatusRejected, domain.GovernorDecisionReasonStrategyExposureExceedsLimit
	}

	projectedInstrumentExposure := projectedExposureNotional(
		input.currentInstrumentExposureNotional,
		input.candidateAction.Kind,
		input.orderNotional,
	)
	if policy.maximumInstrumentExposureNotional > 0 &&
		projectedInstrumentExposure > policy.maximumInstrumentExposureNotional {
		return domain.GovernorDecisionStatusRejected, domain.GovernorDecisionReasonInstrumentExposureExceedsLimit
	}

	if approvedCount >= policy.maximumApprovedCount {
		return domain.GovernorDecisionStatusBlocked, domain.GovernorDecisionReasonApprovalLimitReached
	}

	return domain.GovernorDecisionStatusApproved, domain.GovernorDecisionReasonOK
}

func canonicalizeAllowedModes(
	modes []domain.DecisionMode,
) (map[domain.DecisionMode]struct{}, error) {
	allowedModes := make(map[domain.DecisionMode]struct{}, len(modes))
	for _, mode := range modes {
		normalizedMode, err := domain.NewDecisionMode(mode.String())
		if err != nil {
			return nil, validationError(fmt.Sprintf("unsupported allowed mode %q", mode))
		}

		allowedModes[normalizedMode] = struct{}{}
	}

	return allowedModes, nil
}

func canonicalizeAllowedVenues(
	venues []domain.Venue,
) (map[domain.Venue]struct{}, error) {
	allowedVenues := make(map[domain.Venue]struct{}, len(venues))
	for _, venue := range venues {
		normalizedVenue, err := domain.NewVenue(venue.String())
		if err != nil {
			return nil, validationError(fmt.Sprintf("unsupported allowed venue %q", venue))
		}

		allowedVenues[normalizedVenue] = struct{}{}
	}

	return allowedVenues, nil
}

func canonicalizeAllowedInstruments(
	instruments []domain.Instrument,
) (map[string]struct{}, error) {
	allowedInstruments := make(map[string]struct{}, len(instruments))
	for _, instrument := range instruments {
		normalizedInstrument, err := domain.NewInstrument(domain.InstrumentParams(instrument))
		if err != nil {
			return nil, validationError(
				fmt.Sprintf("unsupported allowed instrument %q/%q", instrument.Venue, instrument.Symbol),
			)
		}

		allowedInstruments[instrumentScopeKey(normalizedInstrument)] = struct{}{}
	}

	return allowedInstruments, nil
}

func canonicalizeAllowedStrategyIDs(
	strategyIDs []string,
) (map[string]struct{}, error) {
	allowedStrategyIDs := make(map[string]struct{}, len(strategyIDs))
	for _, strategyID := range strategyIDs {
		normalizedStrategyID := strings.TrimSpace(strategyID)
		if normalizedStrategyID == "" {
			return nil, validationError("allowed strategy ids must not be empty")
		}

		allowedStrategyIDs[normalizedStrategyID] = struct{}{}
	}

	return allowedStrategyIDs, nil
}

func canonicalizeAllowedActionKinds(
	actionKinds []domain.CandidateActionKind,
) (map[domain.CandidateActionKind]struct{}, error) {
	if len(actionKinds) == 0 {
		return nil, validationError("allowed action kinds are required")
	}

	allowedActionKinds := make(map[domain.CandidateActionKind]struct{}, len(actionKinds))
	for _, actionKind := range actionKinds {
		normalizedActionKind, err := domain.NewCandidateActionKind(actionKind.String())
		if err != nil {
			return nil, validationError(
				fmt.Sprintf("unsupported allowed action kind %q", actionKind),
			)
		}

		allowedActionKinds[normalizedActionKind] = struct{}{}
	}

	return allowedActionKinds, nil
}

func validatePolicyThresholds(policy Policy) error {
	if policy.MaximumApprovedCount < 0 {
		return validationError("maximum approved action count must be zero or greater")
	}
	if !isNonNegativeFinite(policy.MaximumOrderNotional) {
		return validationError("maximum order notional must be finite and zero or greater")
	}
	if !isNonNegativeFinite(policy.MaximumStrategyExposureNotional) {
		return validationError(
			"maximum strategy exposure notional must be finite and zero or greater",
		)
	}
	if !isNonNegativeFinite(policy.MaximumInstrumentExposureNotional) {
		return validationError(
			"maximum instrument exposure notional must be finite and zero or greater",
		)
	}

	return nil
}

func canonicalizeIntentInput(
	input IntentInput,
) (domain.CandidateAction, domain.OrderIntent, error) {
	candidateAction, err := domain.NewCandidateAction(domain.CandidateActionParams{
		Strategy:     input.CandidateAction.Strategy,
		Kind:         input.CandidateAction.Kind,
		DecisionTime: input.CandidateAction.DecisionTime.Time(),
		InputRange:   input.CandidateAction.InputRange,
		Quality:      input.CandidateAction.Quality,
	})
	if err != nil {
		return domain.CandidateAction{}, domain.OrderIntent{}, invalidIntentError(
			fmt.Sprintf("candidate action: %s", err.Error()),
		)
	}

	intent, err := domain.NewOrderIntent(domain.OrderIntentParams{
		IntentID:                 string(input.Intent.IntentID),
		TraceID:                  string(input.Intent.TraceID),
		StrategyID:               input.Intent.StrategyID,
		StrategyVersion:          input.Intent.StrategyVersion,
		StrategyArtifactHash:     input.Intent.StrategyArtifactHash,
		Mode:                     input.Intent.Mode,
		Instrument:               input.Intent.Instrument,
		Timeframe:                input.Intent.Timeframe,
		ActionKind:               input.Intent.ActionKind,
		OrderType:                input.Intent.OrderType,
		RequestedQuantity:        input.Intent.RequestedQuantity,
		RequestedNotional:        input.Intent.RequestedNotional,
		RequestedLimitPrice:      input.Intent.RequestedLimitPrice,
		ReduceOnly:               input.Intent.ReduceOnly,
		SourceReasonCode:         input.Intent.SourceReasonCode,
		CandidateActionReference: input.Intent.CandidateActionReference,
		CreatedTime:              input.Intent.CreatedTime.Time(),
		Status:                   input.Intent.Status,
		Metadata:                 input.Intent.Metadata,
	})
	if err != nil {
		return domain.CandidateAction{}, domain.OrderIntent{}, invalidIntentError(err.Error())
	}

	return candidateAction, intent, nil
}

func validateIntentInputAlignment(
	candidateAction domain.CandidateAction,
	intent domain.OrderIntent,
) error {
	if candidateAction.Kind != intent.ActionKind {
		return invalidIntentError("candidate action kind must match intent action kind")
	}
	if candidateAction.Strategy.Instrument != intent.Instrument {
		return invalidIntentError("candidate action instrument must match intent instrument")
	}
	if candidateAction.Strategy.Timeframe != intent.Timeframe {
		return invalidIntentError("candidate action timeframe must match intent timeframe")
	}

	return nil
}

func validateIntentInputContext(input IntentInput) error {
	if !isFinite(input.CurrentStrategyExposureNotional) {
		return invalidIntentError("current strategy exposure notional must be finite")
	}
	if !isFinite(input.CurrentInstrumentExposureNotional) {
		return invalidIntentError("current instrument exposure notional must be finite")
	}
	if strings.TrimSpace(input.GovernorPolicyID) == "" {
		return invalidIntentError("governor policy id is required")
	}
	if strings.TrimSpace(input.GovernorPolicyVersion) == "" {
		return invalidIntentError("governor policy version is required")
	}
	if strings.TrimSpace(input.GovernorPolicyHash) == "" {
		return invalidIntentError("governor policy hash is required")
	}

	return nil
}

func evaluateIntentScope(
	input canonicalIntentInput,
	policy canonicalPolicy,
) (domain.GovernorDecisionStatus, domain.GovernorDecisionReason, bool) {
	if input.intent.Mode == domain.DecisionModeLive {
		return domain.GovernorDecisionStatusRejected,
			domain.GovernorDecisionReasonModeNotAllowed,
			true
	}
	if len(policy.allowedModes) > 0 {
		if _, ok := policy.allowedModes[input.intent.Mode]; !ok {
			return domain.GovernorDecisionStatusRejected,
				domain.GovernorDecisionReasonModeNotAllowed,
				true
		}
	}
	if len(policy.allowedVenues) > 0 {
		if _, ok := policy.allowedVenues[input.intent.Instrument.Venue]; !ok {
			return domain.GovernorDecisionStatusRejected,
				domain.GovernorDecisionReasonVenueNotAllowed,
				true
		}
	}
	if len(policy.allowedInstruments) > 0 {
		if _, ok := policy.allowedInstruments[instrumentScopeKey(input.intent.Instrument)]; !ok {
			return domain.GovernorDecisionStatusRejected,
				domain.GovernorDecisionReasonInstrumentNotAllowed,
				true
		}
	}
	if len(policy.allowedStrategyIDs) > 0 {
		if _, ok := policy.allowedStrategyIDs[input.intent.StrategyID]; !ok {
			return domain.GovernorDecisionStatusRejected,
				domain.GovernorDecisionReasonStrategyNotAllowed,
				true
		}
	}
	if _, ok := policy.allowedActionKinds[input.candidateAction.Kind]; !ok {
		return domain.GovernorDecisionStatusRejected,
			domain.GovernorDecisionReasonActionKindNotAllowed,
			true
	}

	return "", "", false
}

func evaluateIntentQualityAndRisk(
	input canonicalIntentInput,
	policy canonicalPolicy,
) (domain.GovernorDecisionStatus, domain.GovernorDecisionReason, bool) {
	if candidateQualityRank(input.candidateAction.Quality) < policy.minimumQualityRank {
		return domain.GovernorDecisionStatusRejected,
			domain.GovernorDecisionReasonDataQualityTooLow,
			true
	}
	if policy.blockNewRisk && !input.intent.ReduceOnly {
		return domain.GovernorDecisionStatusBlocked,
			domain.GovernorDecisionReasonKillSwitchActive,
			true
	}
	if policy.maximumOrderNotional > 0 && input.orderNotional > policy.maximumOrderNotional {
		return domain.GovernorDecisionStatusRejected,
			domain.GovernorDecisionReasonOrderNotionalExceedsLimit,
			true
	}

	return "", "", false
}

func minimumQualityRank(quality domain.DataQuality) (int, error) {
	switch quality {
	case domain.DataQualityRaw:
		return qualityRankRaw, nil
	case domain.DataQualityValidated:
		return qualityRankValidated, nil
	case domain.DataQualitySuspect:
		return qualityRankSuspect, errors.New("unsupported minimum quality \"suspect\"")
	default:
		return 0, fmt.Errorf("unsupported minimum quality %q", quality)
	}
}

func candidateQualityRank(quality domain.DataQuality) int {
	switch quality {
	case domain.DataQualitySuspect:
		return qualityRankSuspect
	case domain.DataQualityRaw:
		return qualityRankRaw
	case domain.DataQualityValidated:
		return qualityRankValidated
	default:
		return -1
	}
}

func requestedOrderNotional(intent domain.OrderIntent) (float64, error) {
	if !isFinite(intent.RequestedNotional) || intent.RequestedNotional < 0 {
		return 0, errors.New("requested notional must be finite and zero or greater")
	}
	if intent.RequestedNotional > 0 {
		return intent.RequestedNotional, nil
	}
	if intent.RequestedLimitPrice == nil {
		return 0, errors.New("requested limit price is required to derive requested notional")
	}

	return intent.RequestedQuantity * *intent.RequestedLimitPrice, nil
}

func projectedExposureNotional(
	currentExposureNotional float64,
	actionKind domain.CandidateActionKind,
	orderNotional float64,
) float64 {
	signedOrderNotional := orderNotional
	if actionKind == domain.CandidateActionKindShort {
		signedOrderNotional = -orderNotional
	}

	return math.Abs(currentExposureNotional + signedOrderNotional)
}

func instrumentScopeKey(instrument domain.Instrument) string {
	return strings.Join([]string{
		instrument.Venue.String(),
		instrument.Symbol.String(),
		instrument.AssetClass.String(),
	}, "|")
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func isNonNegativeFinite(value float64) bool {
	return isFinite(value) && value >= 0
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

func invalidIntentError(message string) error {
	return validationError(string(domain.GovernorDecisionReasonInvalidIntent) + ": " + message)
}
