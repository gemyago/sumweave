package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/finance/domain"
	"gorm.io/gorm"
)

var ErrProviderEvidenceNotFound = errors.New("provider evidence not found")

// ProviderEvidenceStore owns sanitized provider-evidence persistence and reads.
type ProviderEvidenceStore struct {
	db *gorm.DB
}

func NewProviderEvidenceStore(database *Database) *ProviderEvidenceStore {
	return &ProviderEvidenceStore{db: database.db}
}

// NewProviderEvidenceStoreFromStore keeps provider evidence in the caller's transaction.
func NewProviderEvidenceStoreFromStore(store *Store) *ProviderEvidenceStore {
	return &ProviderEvidenceStore{db: store.db}
}

func (s *ProviderEvidenceStore) IsTenantMember(
	ctx context.Context,
	tenantID string,
	userID string,
) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Table((tenantMembershipModel{}).TableName()).
		Where("tenant_id = ? AND user_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(userID)).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check provider evidence tenant membership: %w", err)
	}
	return count > 0, nil
}

func (s *ProviderEvidenceStore) SaveProviderEvidence(
	ctx context.Context,
	evidence domain.ProviderEvidence,
) (domain.ProviderEvidence, error) {
	sanitizedPayload, err := domain.SanitizeProviderEvidenceJSON(evidence.PayloadJSON)
	if err != nil {
		return domain.ProviderEvidence{}, fmt.Errorf("sanitize provider evidence payload: %w", err)
	}
	evidence.PayloadJSON = sanitizedPayload
	model := newProviderEvidenceModel(evidence)
	if createErr := s.db.WithContext(ctx).Table(model.TableName()).Create(&model).Error; createErr != nil {
		return domain.ProviderEvidence{}, fmt.Errorf("save provider evidence: %w", createErr)
	}
	return providerEvidenceFromModel(model), nil
}

func (s *ProviderEvidenceStore) DeleteProviderEvidenceByConnection(
	ctx context.Context,
	connectionID string,
) error {
	if err := s.db.WithContext(ctx).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Delete(&providerEvidenceModel{}).Error; err != nil {
		return fmt.Errorf("delete provider evidence: %w", err)
	}
	return nil
}

func (s *ProviderEvidenceStore) ListAccountProviderEvidence(
	ctx context.Context,
	tenantID string,
	accountID string,
) ([]domain.ProviderEvidence, error) {
	if err := s.requireAccount(ctx, tenantID, accountID); err != nil {
		return nil, err
	}
	return s.list(
		ctx,
		"finance_account_id = ? AND subject = ?",
		strings.TrimSpace(accountID),
		tenantID,
		domain.ProviderEvidenceSubjectAccount,
	)
}

func (s *ProviderEvidenceStore) GetAccountProviderEvidence(
	ctx context.Context,
	tenantID string,
	accountID string,
	evidenceID string,
) (domain.ProviderEvidence, error) {
	if err := s.requireAccount(ctx, tenantID, accountID); err != nil {
		return domain.ProviderEvidence{}, err
	}
	return s.get(
		ctx,
		tenantID,
		"finance_account_id = ? AND id = ? AND subject = ?",
		accountID,
		evidenceID,
		domain.ProviderEvidenceSubjectAccount,
	)
}

func (s *ProviderEvidenceStore) ListTransactionProviderEvidence(
	ctx context.Context,
	tenantID string,
	transactionID string,
) ([]domain.ProviderEvidence, error) {
	if err := s.requireTransaction(ctx, tenantID, transactionID); err != nil {
		return nil, err
	}
	return s.list(
		ctx,
		"finance_transaction_id = ? AND subject = ?",
		strings.TrimSpace(transactionID),
		tenantID,
		domain.ProviderEvidenceSubjectTransaction,
	)
}

func (s *ProviderEvidenceStore) GetTransactionProviderEvidence(
	ctx context.Context,
	tenantID string,
	transactionID string,
	evidenceID string,
) (domain.ProviderEvidence, error) {
	if err := s.requireTransaction(ctx, tenantID, transactionID); err != nil {
		return domain.ProviderEvidence{}, err
	}
	return s.get(
		ctx,
		tenantID,
		"finance_transaction_id = ? AND id = ? AND subject = ?",
		transactionID,
		evidenceID,
		domain.ProviderEvidenceSubjectTransaction,
	)
}

func (s *ProviderEvidenceStore) requireAccount(ctx context.Context, tenantID string, accountID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Table((accountModel{}).TableName()).
		Where("id = ? AND tenant_id = ?", strings.TrimSpace(accountID), strings.TrimSpace(tenantID)).
		Count(&count).Error; err != nil {
		return fmt.Errorf("check provider evidence account: %w", err)
	}
	if count == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func (s *ProviderEvidenceStore) requireTransaction(ctx context.Context, tenantID string, transactionID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Table((transactionModel{}).TableName()).
		Where("id = ? AND tenant_id = ?", strings.TrimSpace(transactionID), strings.TrimSpace(tenantID)).
		Count(&count).Error; err != nil {
		return fmt.Errorf("check provider evidence transaction: %w", err)
	}
	if count == 0 {
		return ErrTransactionNotFound
	}
	return nil
}

func (s *ProviderEvidenceStore) list(
	ctx context.Context,
	predicate string,
	entityID string,
	tenantID string,
	subject domain.ProviderEvidenceSubject,
) ([]domain.ProviderEvidence, error) {
	var models []providerEvidenceModel
	if err := s.db.WithContext(ctx).Table((providerEvidenceModel{}).TableName()).
		Where(
			"tenant_id = ? AND "+predicate,
			strings.TrimSpace(tenantID),
			strings.TrimSpace(entityID),
			string(subject),
		).
		Order("captured_at DESC, id DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list provider evidence: %w", err)
	}
	items := make([]domain.ProviderEvidence, 0, len(models))
	for _, model := range models {
		items = append(items, providerEvidenceFromModel(model))
	}
	return items, nil
}

func (s *ProviderEvidenceStore) get(
	ctx context.Context,
	tenantID string,
	predicate string,
	entityID string,
	evidenceID string,
	subject domain.ProviderEvidenceSubject,
) (domain.ProviderEvidence, error) {
	var model providerEvidenceModel
	if err := s.db.WithContext(ctx).Table(model.TableName()).
		Where("tenant_id = ? AND "+predicate,
			strings.TrimSpace(tenantID),
			strings.TrimSpace(entityID),
			strings.TrimSpace(evidenceID),
			string(subject),
		).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ProviderEvidence{}, ErrProviderEvidenceNotFound
		}
		return domain.ProviderEvidence{}, fmt.Errorf("get provider evidence: %w", err)
	}
	return providerEvidenceFromModel(model), nil
}
