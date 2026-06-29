package persistence

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
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
		Where("effective_at >= ?", window.Start.UTC()).
		Where("effective_at < ?", window.End.UTC()).
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
