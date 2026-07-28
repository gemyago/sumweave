package providers

import (
	"time"

	"github.com/gemyago/sumweave/finance/domain"
)

type ApplyTransactionWrite struct {
	Action            ProviderTransactionAction
	MergedTransaction *domain.Transaction
}

type ApplyPlan struct {
	TransactionWrites []ApplyTransactionWrite
	Stats             domain.ProviderSyncStats
	Issues            []domain.ProviderSyncIssue
}

type ApplyPlanner struct {
	mergePolicy *MergePolicy
}

func NewApplyPlanner(mergePolicy *MergePolicy) *ApplyPlanner {
	if mergePolicy == nil {
		mergePolicy = NewMergePolicy()
	}
	return &ApplyPlanner{mergePolicy: mergePolicy}
}

// Plan turns diff actions into apply intent without touching persistence.
func (p *ApplyPlanner) Plan(diffPlan ProviderDiffPlan) ApplyPlan {
	plan := ApplyPlan{
		Stats:  diffPlan.Stats,
		Issues: append([]domain.ProviderSyncIssue(nil), diffPlan.Issues...),
	}

	for _, action := range diffPlan.TransactionActions {
		write := ApplyTransactionWrite{Action: action}
		if action.Type == ProviderTransactionActionTypeUpdate && action.ExistingTransaction != nil {
			merged := p.mergePolicy.MergeTransaction(*action.ExistingTransaction, action.Observation)
			write.MergedTransaction = &merged
		}
		plan.TransactionWrites = append(plan.TransactionWrites, write)
	}

	return plan
}

type MergePolicy struct{}

func NewMergePolicy() *MergePolicy {
	return &MergePolicy{}
}

// MergeTransaction refreshes provider-original fields while preserving user edits.
func (p *MergePolicy) MergeTransaction(
	existing domain.Transaction,
	observation domain.ProviderTransactionObservation,
) domain.Transaction {
	merged := existing
	previousOriginal := existing.ProviderOriginal

	merged.Status = observation.Status
	if shouldRefreshInt64(existing.AmountMinor, previousOriginal) {
		merged.AmountMinor = observation.AmountMinor
	}
	if shouldRefreshString(existing.Currency, previousOriginal) {
		merged.Currency = observation.Currency
	}
	if shouldRefreshStringValue(existing.Description, previousOriginal) {
		merged.Description = observation.Description
	}
	if shouldRefreshTime(existing.EffectiveAt, previousOriginal) {
		merged.EffectiveAt = observation.EffectiveAt
	}
	merged.ProviderOriginal = buildProviderOriginal(observation)

	return merged
}

func shouldRefreshInt64(current int64, original *domain.ProviderTransactionOriginal) bool {
	if original == nil {
		return false
	}
	return current == original.AmountMinor
}

func shouldRefreshString(current string, original *domain.ProviderTransactionOriginal) bool {
	if original == nil {
		return false
	}
	return current == original.Currency
}

func shouldRefreshStringValue(current string, original *domain.ProviderTransactionOriginal) bool {
	if original == nil {
		return false
	}
	return current == original.Description
}

func shouldRefreshTime(current time.Time, original *domain.ProviderTransactionOriginal) bool {
	if original == nil || original.EffectiveAt == nil {
		return false
	}
	return current.Equal(*original.EffectiveAt)
}

func buildProviderOriginal(
	observation domain.ProviderTransactionObservation,
) *domain.ProviderTransactionOriginal {
	if observation.ProviderOriginal != nil {
		copied := *observation.ProviderOriginal
		if copied.EffectiveAt != nil {
			effectiveAt := *copied.EffectiveAt
			copied.EffectiveAt = &effectiveAt
		}
		return &copied
	}

	effectiveAt := observation.EffectiveAt
	return &domain.ProviderTransactionOriginal{
		AmountMinor: observation.AmountMinor,
		Currency:    observation.Currency,
		Description: observation.Description,
		EffectiveAt: &effectiveAt,
	}
}
