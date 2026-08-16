package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrProviderSnapshotNotFound = errors.New("provider snapshot not found")

func latestProviderObservationClause() clause.Where {
	return clause.Where{Exprs: []clause.Expression{
		clause.Gte{
			Column: clause.Column{Table: "excluded", Name: columnCapturedAt},
			Value:  clause.Column{Table: clause.CurrentTable, Name: columnCapturedAt},
		},
	}}
}

// ProviderSnapshotStore owns current provider source snapshots and their reads.
type ProviderSnapshotStore struct {
	db *gorm.DB
}

func NewProviderSnapshotStore(database *Database) *ProviderSnapshotStore {
	return &ProviderSnapshotStore{db: database.db}
}

// NewProviderSnapshotStoreFromStore keeps source snapshots in the caller's transaction.
func NewProviderSnapshotStoreFromStore(store *Store) *ProviderSnapshotStore {
	return &ProviderSnapshotStore{db: store.db}
}

func (s *ProviderSnapshotStore) IsTenantMember(
	ctx context.Context,
	tenantID string,
	userID string,
) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Table((tenantMembershipModel{}).TableName()).
		Where("tenant_id = ? AND user_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(userID)).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check provider snapshot tenant membership: %w", err)
	}
	return count > 0, nil
}

func (s *ProviderSnapshotStore) SaveProviderSnapshot(
	ctx context.Context,
	snapshot domain.ProviderSnapshot,
) (domain.ProviderSnapshot, error) {
	if err := snapshot.Validate(); err != nil {
		return domain.ProviderSnapshot{}, fmt.Errorf("validate provider snapshot: %w", err)
	}
	if err := s.requireSnapshotOwnership(ctx, snapshot); err != nil {
		return domain.ProviderSnapshot{}, fmt.Errorf("validate provider snapshot ownership: %w", err)
	}
	sanitizedDocument, err := domain.SanitizeProviderSnapshotJSON(snapshot.DocumentJSON)
	if err != nil {
		return domain.ProviderSnapshot{}, fmt.Errorf("sanitize provider snapshot document: %w", err)
	}
	snapshot.DocumentJSON = sanitizedDocument
	model := newProviderSnapshotModel(snapshot)
	if createErr := s.db.WithContext(ctx).Table(model.TableName()).Clauses(clause.OnConflict{
		Columns: providerSnapshotIdentityColumns(),
		DoUpdates: clause.Assignments(map[string]any{
			"document_json":  model.DocumentJSON,
			columnCapturedAt: model.CapturedAt,
		}),
		Where: latestProviderObservationClause(),
	}).Create(&model).Error; createErr != nil {
		return domain.ProviderSnapshot{}, fmt.Errorf("save provider snapshot: %w", createErr)
	}
	var persisted providerSnapshotModel
	if getErr := s.db.WithContext(ctx).Table(model.TableName()).
		Where(providerSnapshotIdentityPredicate(), providerSnapshotIdentityValues(model)...).
		First(&persisted).Error; getErr != nil {
		return domain.ProviderSnapshot{}, fmt.Errorf("read saved provider snapshot: %w", getErr)
	}
	return providerSnapshotFromModel(persisted), nil
}

func (s *ProviderSnapshotStore) requireSnapshotOwnership(
	ctx context.Context,
	snapshot domain.ProviderSnapshot,
) error {
	var connectionCount int64
	if err := s.db.WithContext(ctx).Table((bankConnectionModel{}).TableName()).
		Where("id = ? AND tenant_id = ?", snapshot.ConnectionID, snapshot.TenantID).
		Count(&connectionCount).Error; err != nil {
		return fmt.Errorf("check provider snapshot connection: %w", err)
	}
	if connectionCount == 0 {
		return ErrBankConnectionNotFound
	}
	switch snapshot.Subject {
	case domain.ProviderSnapshotSubjectConnection:
		return nil
	case domain.ProviderSnapshotSubjectAccount:
		return s.requireAccount(ctx, snapshot.TenantID, snapshot.FinanceAccountID)
	case domain.ProviderSnapshotSubjectTransaction:
		if err := s.requireAccount(ctx, snapshot.TenantID, snapshot.FinanceAccountID); err != nil {
			return err
		}
		return s.requireTransactionAttachment(ctx, snapshot)
	default:
		return errors.New("provider snapshot subject is unsupported")
	}
}

func (s *ProviderSnapshotStore) requireTransactionAttachment(
	ctx context.Context,
	snapshot domain.ProviderSnapshot,
) error {
	var count int64
	if err := s.db.WithContext(ctx).Table((transactionModel{}).TableName()).
		Where(
			"id = ? AND tenant_id = ? AND account_id = ?",
			snapshot.FinanceTransactionID,
			snapshot.TenantID,
			snapshot.FinanceAccountID,
		).
		Count(&count).Error; err != nil {
		return fmt.Errorf("check provider snapshot transaction attachment: %w", err)
	}
	if count == 0 {
		return ErrTransactionNotFound
	}
	return nil
}

func providerSnapshotIdentityColumns() []clause.Column {
	return []clause.Column{
		{Name: columnTenantID},
		{Name: columnConnectionID},
		{Name: "subject"},
		{Name: columnFinanceAccountID},
		{Name: "finance_transaction_id"},
		{Name: columnKind},
		{Name: columnProviderObjectID},
	}
}

func providerSnapshotIdentityPredicate() string {
	return "tenant_id = ? AND connection_id = ? AND subject = ? AND finance_account_id = ? " +
		"AND finance_transaction_id = ? AND kind = ? AND provider_object_id = ?"
}

func providerSnapshotIdentityValues(model providerSnapshotModel) []any {
	return []any{
		model.TenantID,
		model.ConnectionID,
		model.Subject,
		model.FinanceAccountID,
		model.FinanceTransactionID,
		model.Kind,
		model.ProviderObjectID,
	}
}

func (s *ProviderSnapshotStore) DeleteProviderSnapshotsByConnection(
	ctx context.Context,
	connectionID string,
) error {
	if err := s.db.WithContext(ctx).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Delete(&providerSnapshotModel{}).Error; err != nil {
		return fmt.Errorf("delete provider snapshots: %w", err)
	}
	return nil
}

func (s *ProviderSnapshotStore) ListProviderSnapshotsByConnection(
	ctx context.Context,
	connectionID string,
) ([]domain.ProviderSnapshot, error) {
	var models []providerSnapshotModel
	if err := s.db.WithContext(ctx).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Order("captured_at DESC, id DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list provider snapshots by connection: %w", err)
	}
	return providerSnapshotsFromModels(models), nil
}

func (s *ProviderSnapshotStore) ListAccountProviderSnapshots(
	ctx context.Context,
	tenantID string,
	accountID string,
) ([]domain.ProviderSnapshot, error) {
	if err := s.requireAccount(ctx, tenantID, accountID); err != nil {
		return nil, err
	}
	return s.listBySubject(
		ctx,
		tenantID,
		accountID,
		domain.ProviderSnapshotSubjectAccount,
		columnFinanceAccountID,
	)
}

func (s *ProviderSnapshotStore) GetAccountProviderSnapshot(
	ctx context.Context,
	tenantID string,
	accountID string,
	snapshotID string,
) (domain.ProviderSnapshot, error) {
	if err := s.requireAccount(ctx, tenantID, accountID); err != nil {
		return domain.ProviderSnapshot{}, err
	}
	return s.getBySubject(
		ctx,
		tenantID,
		accountID,
		snapshotID,
		domain.ProviderSnapshotSubjectAccount,
		columnFinanceAccountID,
	)
}

func (s *ProviderSnapshotStore) ListTransactionProviderSnapshots(
	ctx context.Context,
	tenantID string,
	transactionID string,
) ([]domain.ProviderSnapshot, error) {
	if err := s.requireTransaction(ctx, tenantID, transactionID); err != nil {
		return nil, err
	}
	return s.listBySubject(
		ctx,
		tenantID,
		transactionID,
		domain.ProviderSnapshotSubjectTransaction,
		"finance_transaction_id",
	)
}

func (s *ProviderSnapshotStore) GetTransactionProviderSnapshot(
	ctx context.Context,
	tenantID string,
	transactionID string,
	snapshotID string,
) (domain.ProviderSnapshot, error) {
	if err := s.requireTransaction(ctx, tenantID, transactionID); err != nil {
		return domain.ProviderSnapshot{}, err
	}
	return s.getBySubject(
		ctx,
		tenantID,
		transactionID,
		snapshotID,
		domain.ProviderSnapshotSubjectTransaction,
		"finance_transaction_id",
	)
}

func (s *ProviderSnapshotStore) requireAccount(ctx context.Context, tenantID string, accountID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Table((accountModel{}).TableName()).
		Where("id = ? AND tenant_id = ?", strings.TrimSpace(accountID), strings.TrimSpace(tenantID)).
		Count(&count).Error; err != nil {
		return fmt.Errorf("check provider snapshot account: %w", err)
	}
	if count == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func (s *ProviderSnapshotStore) requireTransaction(ctx context.Context, tenantID string, transactionID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Table((transactionModel{}).TableName()).
		Where("id = ? AND tenant_id = ?", strings.TrimSpace(transactionID), strings.TrimSpace(tenantID)).
		Count(&count).Error; err != nil {
		return fmt.Errorf("check provider snapshot transaction: %w", err)
	}
	if count == 0 {
		return ErrTransactionNotFound
	}
	return nil
}

func (s *ProviderSnapshotStore) listBySubject(
	ctx context.Context,
	tenantID string,
	financeID string,
	subject domain.ProviderSnapshotSubject,
	financeColumn string,
) ([]domain.ProviderSnapshot, error) {
	var models []providerSnapshotModel
	if err := s.db.WithContext(ctx).Where(
		"tenant_id = ? AND "+financeColumn+" = ? AND subject = ?",
		strings.TrimSpace(tenantID), strings.TrimSpace(financeID), string(subject),
	).Order("captured_at DESC, id DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list provider snapshots: %w", err)
	}
	return providerSnapshotsFromModels(models), nil
}

func (s *ProviderSnapshotStore) getBySubject(
	ctx context.Context,
	tenantID string,
	financeID string,
	snapshotID string,
	subject domain.ProviderSnapshotSubject,
	financeColumn string,
) (domain.ProviderSnapshot, error) {
	var model providerSnapshotModel
	if err := s.db.WithContext(ctx).Where(
		"tenant_id = ? AND "+financeColumn+" = ? AND id = ? AND subject = ?",
		strings.TrimSpace(tenantID), strings.TrimSpace(financeID), strings.TrimSpace(snapshotID), string(subject),
	).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProviderSnapshot{}, ErrProviderSnapshotNotFound
		}
		return domain.ProviderSnapshot{}, fmt.Errorf("get provider snapshot: %w", err)
	}
	return providerSnapshotFromModel(model), nil
}

func providerSnapshotsFromModels(models []providerSnapshotModel) []domain.ProviderSnapshot {
	items := make([]domain.ProviderSnapshot, 0, len(models))
	for _, model := range models {
		items = append(items, providerSnapshotFromModel(model))
	}
	return items
}
