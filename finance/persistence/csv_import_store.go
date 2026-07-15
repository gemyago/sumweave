package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CSVImportStore owns atomic CSV import row writes and their durable audit trail.
type CSVImportStore struct {
	db  *gorm.DB
	now func() time.Time
}

type csvImportRowRejectionError struct{ reason string }

func (e csvImportRowRejectionError) Error() string { return e.reason }

func NewCSVImportStore(database *Database) *CSVImportStore {
	return &CSVImportStore{db: database.db, now: time.Now}
}

func (s *CSVImportStore) ImportTransactionRow(
	ctx context.Context,
	row domain.CSVImportTransactionRow,
) (domain.CSVImportRowOutcome, error) {
	var outcome domain.CSVImportRowOutcome
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, found, lookupErr := getCSVImportRowOutcome(tx, row.ImportID, row.RowNumber)
		if lookupErr != nil {
			return lookupErr
		}
		if found {
			outcome = csvImportRowOutcomeFromModel(existing)
			return nil
		}
		outcomeModel, importErr := s.createTransactionRow(tx, row)
		if importErr != nil {
			return importErr
		}
		if createErr := tx.Create(&outcomeModel).Error; createErr != nil {
			return fmt.Errorf("save import row outcome: %w", createErr)
		}
		outcome = csvImportRowOutcomeFromModel(outcomeModel)
		return nil
	})
	if err != nil {
		var rejected csvImportRowRejectionError
		if errors.As(err, &rejected) {
			return s.saveRejectedOutcome(ctx, row, rejected.reason)
		}
		return domain.CSVImportRowOutcome{}, fmt.Errorf("import csv transaction row: %w", err)
	}
	return outcome, nil
}

func (s *CSVImportStore) saveRejectedOutcome(
	ctx context.Context,
	row domain.CSVImportTransactionRow,
	reason string,
) (domain.CSVImportRowOutcome, error) {
	model := newCSVImportRowOutcome(row, domain.CSVImportRowOutcomeRejected, reason, s.now())
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.CSVImportRowOutcome{}, fmt.Errorf("save rejected import row outcome: %w", err)
	}
	return csvImportRowOutcomeFromModel(model), nil
}

func (s *CSVImportStore) createTransactionRow(
	tx *gorm.DB,
	row domain.CSVImportTransactionRow,
) (csvImportRowOutcomeModel, error) {
	now := s.now()
	account, err := resolveImportAccount(tx, row, now)
	if err != nil {
		return csvImportRowOutcomeModel{}, err
	}
	categoryID, hasCategory, err := resolveImportCategory(tx, row, now)
	if err != nil {
		return csvImportRowOutcomeModel{}, err
	}
	var transactionCategoryID *string
	if hasCategory {
		transactionCategoryID = &categoryID
	}
	tagIDs, err := resolveImportTags(tx, row, now)
	if err != nil {
		return csvImportRowOutcomeModel{}, err
	}
	if duplicateErr := rejectDuplicateImportTransaction(tx, row, account.ID); duplicateErr != nil {
		return csvImportRowOutcomeModel{}, duplicateErr
	}
	transaction, err := createImportTransaction(tx, row, account.ID, transactionCategoryID, now)
	if err != nil {
		return csvImportRowOutcomeModel{}, err
	}
	if assignErr := assignImportTransactionTags(tx, transaction.ID, tagIDs); assignErr != nil {
		return csvImportRowOutcomeModel{}, assignErr
	}
	return newCSVImportRowOutcome(row, domain.CSVImportRowOutcomeImported, transaction.ID, now), nil
}

func (s *CSVImportStore) ListCSVImportRowOutcomes(
	ctx context.Context,
	importID string,
) ([]domain.CSVImportRowOutcome, error) {
	var models []csvImportRowOutcomeModel
	if err := s.db.WithContext(ctx).
		Where("import_id = ?", strings.TrimSpace(importID)).
		Order("row_number ASC").
		Find(&models).
		Error; err != nil {
		return nil, fmt.Errorf("list csv import row outcomes: %w", err)
	}
	result := make([]domain.CSVImportRowOutcome, 0, len(models))
	for _, model := range models {
		result = append(result, csvImportRowOutcomeFromModel(model))
	}
	return result, nil
}

// ListRecentCSVImports returns the latest confirmed imports for one tenant and type.
// Preview-only records are intentionally excluded because they have no durable job
// or import outcome to reopen.
func (s *CSVImportStore) ListRecentCSVImports(
	ctx context.Context,
	tenantID string,
	importType domain.CSVImportType,
	limit int,
) ([]domain.CSVImportRecord, error) {
	var models []csvImportModel
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND type = ? AND status <> ?", strings.TrimSpace(tenantID), string(importType), string(domain.CSVImportStatusPreviewed)).
		Order("updated_at DESC, id DESC").
		Limit(limit).
		Find(&models).
		Error; err != nil {
		return nil, fmt.Errorf("list recent csv imports: %w", err)
	}
	items := make([]domain.CSVImportRecord, 0, len(models))
	for _, model := range models {
		items = append(items, csvImportFromModel(model))
	}
	return items, nil
}

func getCSVImportRowOutcome(
	tx *gorm.DB,
	importID string,
	rowNumber int,
) (csvImportRowOutcomeModel, bool, error) {
	var model csvImportRowOutcomeModel
	result := tx.Where("import_id = ? AND row_number = ?", importID, rowNumber).First(&model)
	if result.Error == nil {
		return model, true, nil
	}
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return csvImportRowOutcomeModel{}, false, nil
	}
	return csvImportRowOutcomeModel{}, false, fmt.Errorf("get csv import row outcome: %w", result.Error)
}

func newCSVImportRowOutcome(
	row domain.CSVImportTransactionRow,
	status domain.CSVImportRowOutcomeStatus,
	value string,
	now time.Time,
) csvImportRowOutcomeModel {
	model := csvImportRowOutcomeModel{
		ImportID:  row.ImportID,
		RowNumber: row.RowNumber,
		Status:    string(status),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if status == domain.CSVImportRowOutcomeImported {
		model.TransactionID = value
	} else {
		model.Reason = value
	}
	return model
}

func resolveImportAccount(
	tx *gorm.DB,
	row domain.CSVImportTransactionRow,
	now time.Time,
) (accountModel, error) {
	account, found, err := importAccountByName(tx, row.TenantID, row.AccountName)
	if err != nil {
		return accountModel{}, err
	}
	if !found {
		account = accountModel{
			ID:        uuid.NewString(),
			TenantID:  row.TenantID,
			Name:      row.AccountName,
			Currency:  row.Currency,
			Kind:      string(domain.AccountKindImported),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if createErr := tx.Create(&account).Error; createErr != nil {
			return accountModel{}, fmt.Errorf("create import account: %w", createErr)
		}
		return account, nil
	}
	if account.Currency != row.Currency {
		return accountModel{}, csvImportRowRejectionError{reason: fmt.Sprintf(
			"account %q currency is %s, not %s", row.AccountName, account.Currency, row.Currency,
		)}
	}
	return account, nil
}

func resolveImportCategory(
	tx *gorm.DB,
	row domain.CSVImportTransactionRow,
	now time.Time,
) (string, bool, error) {
	if row.CategoryName == "" {
		return "", false, nil
	}
	category, found, err := importCategoryByName(tx, row.TenantID, row.CategoryName)
	if err != nil {
		return "", false, err
	}
	expectedKind := importCategoryKind(row.AmountMinor)
	if !found {
		category = categoryModel{
			ID:        uuid.NewString(),
			TenantID:  row.TenantID,
			Name:      row.CategoryName,
			Kind:      string(expectedKind),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if createErr := tx.Create(&category).Error; createErr != nil {
			return "", false, fmt.Errorf("create import category: %w", createErr)
		}
	} else if category.Kind != string(expectedKind) {
		return "", false, csvImportRowRejectionError{reason: fmt.Sprintf(
			"category %q is %s, incompatible with transaction direction", row.CategoryName, category.Kind,
		)}
	}
	return category.ID, true, nil
}

func importCategoryKind(amountMinor int64) domain.CategoryKind {
	if amountMinor < 0 {
		return domain.CategoryKindExpense
	}
	return domain.CategoryKindIncome
}

func resolveImportTags(
	tx *gorm.DB,
	row domain.CSVImportTransactionRow,
	now time.Time,
) ([]string, error) {
	tagIDs := make([]string, 0, len(row.TagNames))
	for _, name := range row.TagNames {
		tag, found, err := importTagByName(tx, row.TenantID, name)
		if err != nil {
			return nil, err
		}
		if !found {
			tag = tagModel{
				ID: uuid.NewString(), TenantID: row.TenantID, Name: name, CreatedAt: now, UpdatedAt: now,
			}
			if createErr := tx.Create(&tag).Error; createErr != nil {
				return nil, fmt.Errorf("create import tag: %w", createErr)
			}
		}
		tagIDs = append(tagIDs, tag.ID)
	}
	return tagIDs, nil
}

func rejectDuplicateImportTransaction(
	tx *gorm.DB,
	row domain.CSVImportTransactionRow,
	accountID string,
) error {
	var duplicateCount int64
	queryErr := tx.Model(&transactionModel{}).
		Where("tenant_id = ? AND account_id = ? AND currency = ? AND amount_minor = ? AND description = ? AND effective_at = ?", row.TenantID, accountID, row.Currency, row.AmountMinor, row.Description, row.EffectiveAt).
		Count(&duplicateCount).Error
	if queryErr != nil {
		return fmt.Errorf("check import duplicate: %w", queryErr)
	}
	if duplicateCount > 0 {
		return csvImportRowRejectionError{reason: "duplicate transaction"}
	}
	return nil
}

func createImportTransaction(
	tx *gorm.DB,
	row domain.CSVImportTransactionRow,
	accountID string,
	categoryID *string,
	now time.Time,
) (transactionModel, error) {
	transaction := transactionModel{
		ID:          uuid.NewString(),
		TenantID:    row.TenantID,
		AccountID:   accountID,
		Source:      string(domain.TransactionSourceCSV),
		Status:      string(domain.TransactionStatusBooked),
		Kind:        string(domain.TransactionKindRegular),
		AmountMinor: row.AmountMinor,
		Currency:    row.Currency,
		Description: row.Description,
		EffectiveAt: row.EffectiveAt,
		CategoryID:  categoryID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := tx.Create(&transaction).Error; err != nil {
		return transactionModel{}, fmt.Errorf("create import transaction: %w", err)
	}
	return transaction, nil
}

func assignImportTransactionTags(tx *gorm.DB, transactionID string, tagIDs []string) error {
	if len(tagIDs) == 0 {
		return nil
	}
	assignments := make([]transactionTagModel, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		assignments = append(assignments, transactionTagModel{TransactionID: transactionID, TagID: tagID})
	}
	if err := tx.Create(&assignments).Error; err != nil {
		return fmt.Errorf("assign import transaction tags: %w", err)
	}
	return nil
}

func importAccountByName(tx *gorm.DB, tenantID, name string) (accountModel, bool, error) {
	var items []accountModel
	if err := tx.Where("tenant_id = ? AND name = ?", tenantID, name).Find(&items).Error; err != nil {
		return accountModel{}, false, fmt.Errorf("lookup account: %w", err)
	}
	if len(items) > 1 {
		return accountModel{}, false, csvImportRowRejectionError{reason: fmt.Sprintf("account %q is ambiguous", name)}
	}
	if len(items) == 0 {
		return accountModel{}, false, nil
	}
	return items[0], true, nil
}
func importCategoryByName(tx *gorm.DB, tenantID, name string) (categoryModel, bool, error) {
	var items []categoryModel
	if err := tx.Where("tenant_id = ? AND name = ?", tenantID, name).Find(&items).Error; err != nil {
		return categoryModel{}, false, fmt.Errorf("lookup category: %w", err)
	}
	if len(items) > 1 {
		return categoryModel{}, false, csvImportRowRejectionError{reason: fmt.Sprintf("category %q is ambiguous", name)}
	}
	if len(items) == 0 {
		return categoryModel{}, false, nil
	}
	return items[0], true, nil
}
func importTagByName(tx *gorm.DB, tenantID, name string) (tagModel, bool, error) {
	var items []tagModel
	if err := tx.Where("tenant_id = ? AND name = ?", tenantID, name).Find(&items).Error; err != nil {
		return tagModel{}, false, fmt.Errorf("lookup tag: %w", err)
	}
	if len(items) > 1 {
		return tagModel{}, false, csvImportRowRejectionError{reason: fmt.Sprintf("tag %q is ambiguous", name)}
	}
	if len(items) == 0 {
		return tagModel{}, false, nil
	}
	return items[0], true, nil
}
