package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
)

const classificationBatchSize = 200

type ClassificationTransactionStore struct {
	db *gorm.DB
}

type ListEligibleClassificationTransactionsParams struct {
	TenantID          string
	RangeStart        time.Time
	RangeEndExclusive time.Time
	AfterID           string
}

type AssignClassificationCategoryParams struct {
	TenantID      string
	TransactionID string
	CategoryID    string
	UpdatedAt     time.Time
}

func NewClassificationTransactionStore(database *Database) *ClassificationTransactionStore {
	return &ClassificationTransactionStore{db: database.db}
}

func NewClassificationTransactionStoreFromStore(store *Store) *ClassificationTransactionStore {
	return &ClassificationTransactionStore{db: store.db}
}

func (s *ClassificationTransactionStore) ListEligibleClassificationTransactions(
	ctx context.Context,
	params ListEligibleClassificationTransactionsParams,
) ([]domain.Transaction, error) {
	var models []transactionModel
	query := eligibleClassificationTransactionQuery(
		s.db.WithContext(ctx),
		params.TenantID,
		params.RangeStart,
		params.RangeEndExclusive,
	)
	if afterID := strings.TrimSpace(params.AfterID); afterID != "" {
		query = query.Where("id > ?", afterID)
	}
	if err := query.Order("id ASC").Limit(classificationBatchSize).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list eligible classification transactions: %w", err)
	}
	transactions := make([]domain.Transaction, 0, len(models))
	for _, model := range models {
		transactions = append(transactions, transactionFromModel(model))
	}
	return transactions, nil
}

func (s *ClassificationTransactionStore) AssignClassificationCategory(
	ctx context.Context,
	params AssignClassificationCategoryParams,
) (bool, error) {
	query := eligibleClassificationTransactionQuery(s.db.WithContext(ctx), params.TenantID, time.Time{}, time.Time{}).
		Where("id = ?", strings.TrimSpace(params.TransactionID))
	result := query.Updates(map[string]any{columnCategoryID: params.CategoryID, columnUpdatedAt: params.UpdatedAt})
	if result.Error != nil {
		return false, fmt.Errorf("assign classification category: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func eligibleClassificationTransactionQuery(
	query *gorm.DB,
	tenantID string,
	rangeStart time.Time,
	rangeEndExclusive time.Time,
) *gorm.DB {
	query = query.Model(&transactionModel{}).
		Where("tenant_id = ?", strings.TrimSpace(tenantID)).
		Where("category_id IS NULL").
		Where("hidden_at IS NULL").
		Where("status = ?", string(domain.TransactionStatusBooked)).
		Where("kind IN ?", []string{
			string(domain.TransactionKindRegular), string(domain.TransactionKindExpense),
			string(domain.TransactionKindIncome), string(domain.TransactionKindRefund),
		})
	if !rangeStart.IsZero() {
		query = query.Where("effective_at >= ?", rangeStart)
	}
	if !rangeEndExclusive.IsZero() {
		query = query.Where("effective_at < ?", rangeEndExclusive)
	}
	return query
}
