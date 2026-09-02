package persistence

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/providers"
)

var _ providers.WindowSyncPersistence = (*ProviderWindowSyncPersistence)(nil)

type ProviderWindowSyncPersistence struct {
	*Store
}

func NewProviderWindowSyncPersistence(store *Store) *ProviderWindowSyncPersistence {
	return &ProviderWindowSyncPersistence{Store: store}
}

func (s *ProviderWindowSyncPersistence) ListProviderTransactionsInWindow(
	ctx context.Context,
	financeAccountIDs []string,
	window domain.ProviderSyncWindow,
) ([]domain.Transaction, error) {
	accountIDs := nonEmptyTrimmedStrings(financeAccountIDs)
	if len(accountIDs) == 0 {
		return []domain.Transaction{}, nil
	}

	var models []transactionModel
	if err := s.Store.db.WithContext(ctx).
		Table((transactionModel{}).TableName()).
		Where("account_id IN ?", accountIDs).
		Where("source = ?", string(domain.TransactionSourceProvider)).
		Where(instantRangePredicate(columnEffectiveAt), window.Start, window.End).
		Order("effective_at DESC, created_at DESC, id DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list provider transactions in window: %w", err)
	}

	items := make([]domain.Transaction, 0, len(models))
	for _, model := range models {
		items = append(items, transactionFromModel(model))
	}
	return items, nil
}

func (s *ProviderWindowSyncPersistence) ListProviderTransactionMatchesByTransactionIDs(
	ctx context.Context,
	connectionID string,
	transactionIDs []string,
) ([]domain.ProviderTransactionMatch, error) {
	trimmedConnectionID := strings.TrimSpace(connectionID)
	trimmedTransactionIDs := nonEmptyTrimmedStrings(transactionIDs)
	if len(trimmedTransactionIDs) == 0 {
		return []domain.ProviderTransactionMatch{}, nil
	}

	var models []providerTransactionMatchModel
	if err := s.Store.db.WithContext(ctx).
		Table((providerTransactionMatchModel{}).TableName()).
		Where("connection_id = ?", trimmedConnectionID).
		Where("transaction_id IN ?", trimmedTransactionIDs).
		Order("created_at ASC, id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list provider transaction matches by transaction ids: %w", err)
	}

	items := make([]domain.ProviderTransactionMatch, 0, len(models))
	for _, model := range models {
		items = append(items, providerTransactionMatchFromModel(model))
	}
	return items, nil
}

func (s *ProviderWindowSyncPersistence) ListProviderTransactionIdentityMatches(
	ctx context.Context,
	connectionID string,
	identities []providers.ProviderTransactionIdentity,
) ([]providers.ProviderTransactionIdentityMatch, error) {
	identities = nonEmptyProviderTransactionIdentities(identities)
	if len(identities) == 0 {
		return []providers.ProviderTransactionIdentityMatch{}, nil
	}

	identityPredicate := s.Store.db.Where(
		"provider_account_id = ? AND provider_transaction_id = ?",
		identities[0].ProviderAccountID,
		identities[0].ProviderTransactionID,
	)
	for _, identity := range identities[1:] {
		identityPredicate = identityPredicate.Or(
			"provider_account_id = ? AND provider_transaction_id = ?",
			identity.ProviderAccountID,
			identity.ProviderTransactionID,
		)
	}

	var matchModels []providerTransactionMatchModel
	if err := s.Store.db.WithContext(ctx).
		Table((providerTransactionMatchModel{}).TableName()).
		Where("connection_id = ?", connectionID).
		Where(identityPredicate).
		Order("created_at ASC, id ASC").
		Find(&matchModels).Error; err != nil {
		return nil, fmt.Errorf("list provider transaction identity matches: %w", err)
	}
	if len(matchModels) == 0 {
		return []providers.ProviderTransactionIdentityMatch{}, nil
	}

	transactionIDs := make([]string, 0, len(matchModels))
	for _, match := range matchModels {
		transactionIDs = append(transactionIDs, match.TransactionID)
	}
	var transactionModels []transactionModel
	if err := s.Store.db.WithContext(ctx).
		Table((transactionModel{}).TableName()).
		Where("id IN ?", transactionIDs).
		Where("source = ?", string(domain.TransactionSourceProvider)).
		Find(&transactionModels).Error; err != nil {
		return nil, fmt.Errorf("list provider identity match transactions: %w", err)
	}
	transactionsByID := make(map[string]domain.Transaction, len(transactionModels))
	for _, model := range transactionModels {
		transaction := transactionFromModel(model)
		transactionsByID[transaction.ID] = transaction
	}

	items := make([]providers.ProviderTransactionIdentityMatch, 0, len(matchModels))
	for _, model := range matchModels {
		match := providerTransactionMatchFromModel(model)
		transaction, ok := transactionsByID[match.TransactionID]
		if !ok {
			continue
		}
		items = append(items, providers.ProviderTransactionIdentityMatch{
			Transaction: transaction,
			Match:       match,
		})
	}
	return items, nil
}

func (s *ProviderWindowSyncPersistence) WithTransaction(
	ctx context.Context,
	fn func(providers.WindowSyncApplyStore) error,
) error {
	return s.Store.WithTransaction(ctx, func(txStore *Store) error {
		return fn(&providerWindowSyncApplyStore{Store: txStore})
	})
}

var _ providers.WindowSyncApplyStore = (*providerWindowSyncApplyStore)(nil)

type providerWindowSyncApplyStore struct {
	*Store
}

func (s *providerWindowSyncApplyStore) AppendSyncState(
	ctx context.Context,
	state domain.ProviderSyncState,
) error {
	return NewProviderSyncStateJournalStore(s.Store).AppendSyncState(ctx, state)
}

func (s *providerWindowSyncApplyStore) SaveProviderSnapshot(
	ctx context.Context,
	snapshot domain.ProviderSnapshot,
) (domain.ProviderSnapshot, error) {
	return NewProviderSnapshotStoreFromStore(s.Store).SaveProviderSnapshot(ctx, snapshot)
}

func nonEmptyTrimmedStrings(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		trimmed = append(trimmed, normalized)
	}
	return trimmed
}

func nonEmptyProviderTransactionIdentities(
	identities []providers.ProviderTransactionIdentity,
) []providers.ProviderTransactionIdentity {
	result := make([]providers.ProviderTransactionIdentity, 0, len(identities))
	seen := make(map[providers.ProviderTransactionIdentity]struct{}, len(identities))
	for _, identity := range identities {
		if identity.ProviderAccountID == "" || identity.ProviderTransactionID == "" {
			continue
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		result = append(result, identity)
	}
	return result
}
