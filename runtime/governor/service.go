package governor

import (
	"context"
	"errors"
	"fmt"
	"slices"

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
	AllowedActionKinds   []domain.CandidateActionKind
	MinimumQuality       domain.DataQuality
	MaximumApprovedCount int
}

// EvaluateRequest configures deterministic governor evaluation.
type EvaluateRequest struct {
	CandidateActions []domain.CandidateAction
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

// Evaluate evaluates canonical candidate actions against policy rules.
func (s *Service) Evaluate(
	_ context.Context,
	request EvaluateRequest,
) (EvaluateResult, error) {
	canonicalPolicy, err := canonicalizePolicy(request.Policy)
	if err != nil {
		return EvaluateResult{}, err
	}

	actions, err := canonicalizeActions(request.CandidateActions)
	if err != nil {
		return EvaluateResult{}, err
	}

	slices.SortStableFunc(actions, func(left, right domain.CandidateAction) int {
		return left.DecisionTime.Time().Compare(right.DecisionTime.Time())
	})

	decisions := make([]domain.GovernorDecision, 0, len(actions))
	approvedCount := 0

	for _, action := range actions {
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
	allowedActionKinds   map[domain.CandidateActionKind]struct{}
	minimumQualityRank   int
	maximumApprovedCount int
}

func canonicalizePolicy(policy Policy) (canonicalPolicy, error) {
	if len(policy.AllowedActionKinds) == 0 {
		return canonicalPolicy{}, validationError("allowed action kinds are required")
	}

	allowedActionKinds := make(map[domain.CandidateActionKind]struct{}, len(policy.AllowedActionKinds))
	for _, actionKind := range policy.AllowedActionKinds {
		normalizedActionKind, err := domain.NewCandidateActionKind(actionKind.String())
		if err != nil {
			return canonicalPolicy{}, validationError(
				fmt.Sprintf("unsupported allowed action kind %q", actionKind),
			)
		}

		allowedActionKinds[normalizedActionKind] = struct{}{}
	}

	minimumQualityRank, err := minimumQualityRank(policy.MinimumQuality)
	if err != nil {
		return canonicalPolicy{}, validationError(err.Error())
	}

	if policy.MaximumApprovedCount < 0 {
		return canonicalPolicy{}, validationError(
			"maximum approved action count must be zero or greater",
		)
	}

	return canonicalPolicy{
		allowedActionKinds:   allowedActionKinds,
		minimumQualityRank:   minimumQualityRank,
		maximumApprovedCount: policy.MaximumApprovedCount,
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

func evaluateAction(
	action domain.CandidateAction,
	policy canonicalPolicy,
	approvedCount int,
) (domain.GovernorDecisionStatus, domain.GovernorDecisionReason) {
	if _, ok := policy.allowedActionKinds[action.Kind]; !ok {
		return domain.GovernorDecisionStatusRejected,
			domain.GovernorDecisionReasonDisallowedActionKind
	}

	actionQualityRank := candidateQualityRank(action.Quality)
	if actionQualityRank < policy.minimumQualityRank {
		return domain.GovernorDecisionStatusRejected,
			domain.GovernorDecisionReasonBelowMinimumQuality
	}

	if approvedCount >= policy.maximumApprovedCount {
		return domain.GovernorDecisionStatusBlocked,
			domain.GovernorDecisionReasonApprovalLimitReached
	}

	return domain.GovernorDecisionStatusApproved, domain.GovernorDecisionReasonEligible
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

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}
