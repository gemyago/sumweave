package persistence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDuplicateTransactionTag = errors.New("duplicate transaction tag")

// TransactionTagStore owns transaction persistence together with tag assignments.
type TransactionTagStore struct {
	db *gorm.DB
}

func NewTransactionTagStore(database *Database) *TransactionTagStore {
	return &TransactionTagStore{db: database.db}
}

func (s *TransactionTagStore) SaveTransaction(
	ctx context.Context,
	transaction domain.Transaction,
) (domain.Transaction, error) {
	tagIDs, normalizationErr := normalizedTransactionTagIDs(transaction.TagIDs)
	if normalizationErr != nil {
		return domain.Transaction{}, normalizationErr
	}
	model := newTransactionModel(transaction)
	if transactionErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if validationErr := validateTransactionTags(tx, transaction.TenantID, tagIDs); validationErr != nil {
			return validationErr
		}
		if saveErr := saveTransactionModel(tx, model); saveErr != nil {
			return fmt.Errorf("save transaction record: %w", saveErr)
		}
		if deleteErr := tx.Where("transaction_id = ?", transaction.ID).
			Delete(&transactionTagModel{}).Error; deleteErr != nil {
			return fmt.Errorf("clear transaction tags: %w", deleteErr)
		}
		if len(tagIDs) == 0 {
			return nil
		}
		assignments := make([]transactionTagModel, 0, len(tagIDs))
		for _, tagID := range tagIDs {
			assignments = append(assignments, transactionTagModel{TransactionID: transaction.ID, TagID: tagID})
		}
		if createErr := tx.Create(&assignments).Error; createErr != nil {
			return fmt.Errorf("create transaction tags: %w", createErr)
		}
		return nil
	}); transactionErr != nil {
		return domain.Transaction{}, fmt.Errorf("save transaction with tags: %w", transactionErr)
	}
	saved := transactionFromModel(model)
	saved.TagIDs = tagIDs
	return saved, nil
}

func (s *TransactionTagStore) GetTransaction(
	ctx context.Context,
	transactionID string,
) (*domain.Transaction, error) {
	var model transactionModel
	if err := s.db.WithContext(ctx).
		Where("id = ?", strings.TrimSpace(transactionID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	transactions, err := s.hydrateTransactionTags(ctx, []domain.Transaction{transactionFromModel(model)})
	if err != nil {
		return nil, err
	}
	return &transactions[0], nil
}

func (s *TransactionTagStore) ListTransactions(
	ctx context.Context,
	tenantID string,
	accountID string,
	source domain.TransactionSource,
	status domain.TransactionStatus,
	includeHidden bool,
	page ...ListTransactionsPage,
) ([]domain.Transaction, error) {
	var models []transactionModel
	query := s.db.WithContext(ctx).
		Where("tenant_id = ?", strings.TrimSpace(tenantID))
	if accountID != "" {
		query = query.Where("account_id = ?", accountID)
	}
	if source != "" {
		query = query.Where("source = ?", string(source))
	}
	if status != "" {
		query = query.Where("status = ?", string(status))
	}
	if !includeHidden {
		query = query.Where("hidden_at IS NULL")
	}
	if len(page) > 0 {
		if page[0].Limit > 0 {
			query = query.Limit(dbPageInt(page[0].Limit))
		}
		if page[0].Offset > 0 {
			query = query.Offset(dbPageInt(page[0].Offset))
		}
	}
	if err := query.Order("effective_at DESC, created_at DESC, id DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	transactions := make([]domain.Transaction, 0, len(models))
	for _, model := range models {
		transactions = append(transactions, transactionFromModel(model))
	}
	return s.hydrateTransactionTags(ctx, transactions)
}

func validateTransactionTags(tx *gorm.DB, tenantID string, tagIDs []string) error {
	if len(tagIDs) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&tagModel{}).
		Where("tenant_id = ? AND hidden_at IS NULL AND id IN ?", strings.TrimSpace(tenantID), tagIDs).
		Count(&count).Error; err != nil {
		return fmt.Errorf("validate transaction tags: %w", err)
	}
	if count != int64(len(tagIDs)) {
		return ErrTagNotFound
	}
	return nil
}

func normalizedTransactionTagIDs(tagIDs []string) ([]string, error) {
	result := make([]string, 0, len(tagIDs))
	seen := make(map[string]struct{}, len(tagIDs))
	for _, tagID := range tagIDs {
		trimmedTagID := strings.TrimSpace(tagID)
		if _, ok := seen[trimmedTagID]; ok {
			return nil, ErrDuplicateTransactionTag
		}
		seen[trimmedTagID] = struct{}{}
		result = append(result, trimmedTagID)
	}
	sort.Strings(result)
	return result, nil
}

func saveTransactionModel(db *gorm.DB, model transactionModel) error {
	return db.Table(model.TableName()).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"tenant_id", "account_id", "source", "status", "kind", "amount_minor", columnCurrency,
			"description", columnEffectiveAt, "category_id", "transfer_group_id", "transfer_matched_at",
			"hidden_at", "original_amount_minor", "original_currency", "original_description",
			"original_effective_at", "updated_at",
		}),
	}).Create(&model).Error
}

func (s *TransactionTagStore) hydrateTransactionTags(
	ctx context.Context,
	transactions []domain.Transaction,
) ([]domain.Transaction, error) {
	if len(transactions) == 0 {
		return transactions, nil
	}
	transactionIDs := make([]string, 0, len(transactions))
	for index := range transactions {
		transactions[index].TagIDs = []string{}
		transactionIDs = append(transactionIDs, transactions[index].ID)
	}
	var assignments []transactionTagModel
	if err := s.db.WithContext(ctx).
		Where("transaction_id IN ?", transactionIDs).
		Order("transaction_id ASC, tag_id ASC").
		Find(&assignments).Error; err != nil {
		return nil, fmt.Errorf("list transaction tags: %w", err)
	}
	byTransactionID := make(map[string][]string, len(transactions))
	for _, assignment := range assignments {
		byTransactionID[assignment.TransactionID] = append(byTransactionID[assignment.TransactionID], assignment.TagID)
	}
	for index := range transactions {
		transactions[index].TagIDs = byTransactionID[transactions[index].ID]
		if transactions[index].TagIDs == nil {
			transactions[index].TagIDs = []string{}
		}
	}
	return transactions, nil
}
