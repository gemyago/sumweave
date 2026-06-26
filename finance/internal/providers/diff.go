package providers

import (
	"fmt"

	"github.com/gemyago/signal-foundry/finance/domain"
)

type ProviderTransactionActionType string

const (
	ProviderTransactionActionTypeCreate ProviderTransactionActionType = "create"
	ProviderTransactionActionTypeUpdate ProviderTransactionActionType = "update"
)

type ProviderTransactionMatchStrategy string

const (
	ProviderTransactionMatchStrategyNew           ProviderTransactionMatchStrategy = "new"
	ProviderTransactionMatchStrategyProviderID    ProviderTransactionMatchStrategy = "provider_id"
	ProviderTransactionMatchStrategyFingerprint   ProviderTransactionMatchStrategy = "fingerprint"
	ProviderTransactionMatchStrategyWeakCandidate ProviderTransactionMatchStrategy = "weak_candidate"
	ProviderTransactionMatchStrategyAmbiguous     ProviderTransactionMatchStrategy = "ambiguous"
)

type ProviderTransactionAction struct {
	Type                ProviderTransactionActionType
	MatchStrategy       ProviderTransactionMatchStrategy
	Observation         domain.ProviderTransactionObservation
	ExistingTransaction *domain.Transaction
}

type ProviderDiffPlan struct {
	Connection             domain.ProviderConnectionRef
	RequestedWindow        domain.ProviderSyncWindow
	CandidateWindow        domain.ProviderSyncWindow
	AccountObservations    []domain.ProviderAccountObservation
	BalanceObservations    []domain.ProviderBalanceObservation
	TransactionActions     []ProviderTransactionAction
	RawPayloadObservations []domain.ProviderRawPayloadObservation
	Stats                  domain.ProviderSyncStats
	Issues                 []domain.ProviderSyncIssue
}

type DiffPlanner struct{}

func NewDiffPlanner() *DiffPlanner {
	return &DiffPlanner{}
}

// Plan stays pure: it compares observations with an existing snapshot and only returns intent.
func (p *DiffPlanner) Plan(
	batch domain.ProviderSyncBatch,
	snapshot ExistingWindowSnapshot,
) ProviderDiffPlan {
	plan := ProviderDiffPlan{
		Connection:             batch.Connection,
		RequestedWindow:        batch.RequestedWindow,
		CandidateWindow:        snapshot.CandidateWindow,
		AccountObservations:    batch.Accounts,
		BalanceObservations:    batch.Balances,
		RawPayloadObservations: batch.RawPayloads,
		Stats: domain.ProviderSyncStats{
			ObservedAccounts:     len(batch.Accounts),
			ObservedTransactions: len(batch.Transactions),
		},
	}

	for _, observation := range batch.Transactions {
		action, issue := planTransactionAction(observation, snapshot)
		plan.TransactionActions = append(plan.TransactionActions, action)

		switch action.Type {
		case ProviderTransactionActionTypeUpdate:
			plan.Stats.UpdatedTransactions++
		case ProviderTransactionActionTypeCreate:
			plan.Stats.CreatedTransactions++
			if action.MatchStrategy == ProviderTransactionMatchStrategyWeakCandidate ||
				action.MatchStrategy == ProviderTransactionMatchStrategyAmbiguous {
				plan.Stats.AmbiguousCreatedTransactions++
			}
		}

		if issue != nil {
			plan.Issues = append(plan.Issues, *issue)
		}
	}

	return plan
}

func planTransactionAction(
	observation domain.ProviderTransactionObservation,
	snapshot ExistingWindowSnapshot,
) (ProviderTransactionAction, *domain.ProviderSyncIssue) {
	strongMatches := matchingProviderIDs(snapshot.Matches, observation)
	if len(strongMatches) == 1 {
		transaction := snapshotTransaction(
			snapshot.Transactions,
			strongMatches[0].TransactionID,
		)
		if transaction != nil {
			return ProviderTransactionAction{
				Type:                ProviderTransactionActionTypeUpdate,
				MatchStrategy:       ProviderTransactionMatchStrategyProviderID,
				Observation:         observation,
				ExistingTransaction: transaction,
			}, nil
		}
	}
	if len(strongMatches) > 1 {
		return ambiguousCreateAction(observation, "multiple provider-id matches")
	}

	fingerprintMatches := matchingFingerprints(snapshot.Matches, observation)
	if len(fingerprintMatches) == 1 {
		transaction := snapshotTransaction(
			snapshot.Transactions,
			fingerprintMatches[0].TransactionID,
		)
		if transaction != nil {
			return ProviderTransactionAction{
				Type:                ProviderTransactionActionTypeUpdate,
				MatchStrategy:       ProviderTransactionMatchStrategyFingerprint,
				Observation:         observation,
				ExistingTransaction: transaction,
			}, nil
		}
	}
	if len(fingerprintMatches) > 1 {
		return ambiguousCreateAction(observation, "multiple fingerprint matches")
	}

	if hasWeakCandidate(snapshot, observation) {
		issue := domain.ProviderSyncIssue{
			Code:                  "weak-transaction-match",
			Severity:              domain.ProviderSyncIssueSeverityWarning,
			Summary:               "created a new transaction because only weak candidates were available",
			ProviderAccountID:     observation.ProviderAccountID,
			ProviderTransactionID: observation.ProviderTransactionID,
		}
		return ProviderTransactionAction{
			Type:          ProviderTransactionActionTypeCreate,
			MatchStrategy: ProviderTransactionMatchStrategyWeakCandidate,
			Observation:   observation,
		}, &issue
	}

	return ProviderTransactionAction{
		Type:          ProviderTransactionActionTypeCreate,
		MatchStrategy: ProviderTransactionMatchStrategyNew,
		Observation:   observation,
	}, nil
}

func matchingProviderIDs(
	matches []domain.ProviderTransactionMatch,
	observation domain.ProviderTransactionObservation,
) []domain.ProviderTransactionMatch {
	if observation.ProviderTransactionID == "" {
		return nil
	}

	result := make([]domain.ProviderTransactionMatch, 0, len(matches))
	for _, match := range matches {
		if match.ConnectionID != observation.Connection.ConnectionID {
			continue
		}
		if match.ProviderAccountID != observation.ProviderAccountID {
			continue
		}
		if match.ProviderTransactionID == observation.ProviderTransactionID {
			result = append(result, match)
		}
	}
	return result
}

func matchingFingerprints(
	matches []domain.ProviderTransactionMatch,
	observation domain.ProviderTransactionObservation,
) []domain.ProviderTransactionMatch {
	if observation.Fingerprint == "" {
		return nil
	}

	result := make([]domain.ProviderTransactionMatch, 0, len(matches))
	for _, match := range matches {
		if match.ConnectionID != observation.Connection.ConnectionID {
			continue
		}
		if match.ProviderAccountID != observation.ProviderAccountID {
			continue
		}
		if match.Fingerprint == observation.Fingerprint {
			result = append(result, match)
		}
	}
	return result
}

func snapshotTransaction(
	transactions []domain.Transaction,
	transactionID string,
) *domain.Transaction {
	for _, transaction := range transactions {
		if transaction.ID != transactionID {
			continue
		}
		copied := transaction
		return &copied
	}
	return nil
}

func ambiguousCreateAction(
	observation domain.ProviderTransactionObservation,
	reason string,
) (ProviderTransactionAction, *domain.ProviderSyncIssue) {
	issue := domain.ProviderSyncIssue{
		Code:                  "ambiguous-transaction-match",
		Severity:              domain.ProviderSyncIssueSeverityWarning,
		Summary:               fmt.Sprintf("created a new transaction because %s", reason),
		ProviderAccountID:     observation.ProviderAccountID,
		ProviderTransactionID: observation.ProviderTransactionID,
	}
	return ProviderTransactionAction{
		Type:          ProviderTransactionActionTypeCreate,
		MatchStrategy: ProviderTransactionMatchStrategyAmbiguous,
		Observation:   observation,
	}, &issue
}

func hasWeakCandidate(
	snapshot ExistingWindowSnapshot,
	observation domain.ProviderTransactionObservation,
) bool {
	financeAccountID := ""
	for _, account := range snapshot.Accounts {
		if account.ConnectionID != observation.Connection.ConnectionID {
			continue
		}
		if account.ProviderAccountID == observation.ProviderAccountID {
			financeAccountID = account.FinanceAccountID
			break
		}
	}
	if financeAccountID == "" {
		return false
	}

	for _, transaction := range snapshot.Transactions {
		if transaction.Source != domain.TransactionSourceProvider {
			continue
		}
		if transaction.AccountID != financeAccountID {
			continue
		}
		if transaction.AmountMinor != observation.AmountMinor {
			continue
		}
		if transaction.Currency != observation.Currency {
			continue
		}
		if transaction.EffectiveAt.Equal(observation.EffectiveAt) {
			return true
		}
	}

	return false
}
