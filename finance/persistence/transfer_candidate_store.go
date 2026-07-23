package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"gorm.io/gorm"
)

// TransferCandidateStore owns transaction reads used by the transfer-detail workflow.
type TransferCandidateStore struct {
	db   *gorm.DB
	tags *TransactionTagStore
}

func NewTransferCandidateStore(database *Database) *TransferCandidateStore {
	return &TransferCandidateStore{db: database.db, tags: NewTransactionTagStore(database)}
}

func (s *TransferCandidateStore) IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Table((tenantMembershipModel{}).TableName()).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check tenant membership: %w", err)
	}
	return count > 0, nil
}

func (s *TransferCandidateStore) GetTransaction(
	ctx context.Context,
	transactionID string,
) (*domain.Transaction, error) {
	var model transactionModel
	if err := s.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(transactionID)).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get transfer transaction: %w", err)
	}
	transactions, err := s.tags.hydrateTransactionTags(ctx, []domain.Transaction{transactionFromModel(model)})
	if err != nil {
		return nil, fmt.Errorf("hydrate transfer transaction tags: %w", err)
	}
	return &transactions[0], nil
}

func (s *TransferCandidateStore) ListCandidates(
	ctx context.Context,
	tenantID string,
	sourceTransactionID string,
	sourceAccountID string,
	effectiveFrom time.Time,
	effectiveBefore time.Time,
	limit int64,
	offset int64,
) ([]domain.Transaction, error) {
	var models []transactionModel
	query := s.db.WithContext(ctx).
		Where("tenant_id = ?", strings.TrimSpace(tenantID)).
		Where("id <> ?", strings.TrimSpace(sourceTransactionID)).
		Where("account_id <> ?", strings.TrimSpace(sourceAccountID)).
		Where("hidden_at IS NULL").
		Where(instantRangePredicate(s.db, "effective_at"), effectiveFrom, effectiveBefore).
		Order("effective_at DESC, created_at DESC, id DESC").
		Limit(dbPageInt(limit))
	if offset > 0 {
		query = query.Offset(dbPageInt(offset))
	}
	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list transfer candidates: %w", err)
	}
	return s.transferTransactionsFromModels(ctx, models)
}

func (s *TransferCandidateStore) ListTransferGroupTransactions(
	ctx context.Context,
	tenantID string,
	transferGroupID string,
) ([]domain.Transaction, error) {
	var models []transactionModel
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", strings.TrimSpace(tenantID)).
		Where("transfer_group_id = ?", strings.TrimSpace(transferGroupID)).
		Order("id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list transfer-group transactions: %w", err)
	}
	return s.transferTransactionsFromModels(ctx, models)
}

func (s *TransferCandidateStore) transferTransactionsFromModels(
	ctx context.Context,
	models []transactionModel,
) ([]domain.Transaction, error) {
	transactions := make([]domain.Transaction, 0, len(models))
	for _, model := range models {
		transactions = append(transactions, transactionFromModel(model))
	}
	transactions, err := s.tags.hydrateTransactionTags(ctx, transactions)
	if err != nil {
		return nil, fmt.Errorf("hydrate transfer transaction tags: %w", err)
	}
	return transactions, nil
}
