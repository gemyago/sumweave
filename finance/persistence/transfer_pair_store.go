package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
)

var (
	ErrTransferPairTransactionIDsMustDiffer = errors.New("transfer pair transaction IDs must differ")
	ErrTransferPairTransactionNotFound      = errors.New("transfer pair transaction not found")
)

// TransferPairLinkParams identifies two existing tenant transactions to link.
type TransferPairLinkParams struct {
	TenantID            string
	FirstTransactionID  string
	SecondTransactionID string
	TransferGroupID     string
	TransferMatchedAt   time.Time
	UpdatedAt           time.Time
}

// TransferPairUnlinkParams identifies two existing tenant transactions to unlink.
type TransferPairUnlinkParams struct {
	TenantID            string
	FirstTransactionID  string
	SecondTransactionID string
	UpdatedAt           time.Time
}

// TransferPairStore owns narrow, atomic transfer pair state updates.
type TransferPairStore struct {
	db *gorm.DB
}

func NewTransferPairStore(database *Database) *TransferPairStore {
	return &TransferPairStore{db: database.db}
}

func NewTransferPairStoreFromStore(store *Store) *TransferPairStore {
	return &TransferPairStore{db: store.db}
}

func (s *TransferPairStore) LinkTransferPair(ctx context.Context, params TransferPairLinkParams) error {
	updates := map[string]any{
		columnKind:            string(domain.TransactionKindTransfer),
		"transfer_group_id":   params.TransferGroupID,
		"transfer_matched_at": params.TransferMatchedAt,
		columnUpdatedAt:       params.UpdatedAt,
	}
	if err := s.updatePair(
		ctx,
		params.TenantID,
		params.FirstTransactionID,
		params.SecondTransactionID,
		updates,
	); err != nil {
		return fmt.Errorf("link transfer pair: %w", err)
	}
	return nil
}

func (s *TransferPairStore) UnlinkTransferPair(ctx context.Context, params TransferPairUnlinkParams) error {
	updates := map[string]any{
		columnKind:                   string(domain.TransactionKindRegular),
		"transfer_group_id":          nil,
		"transfer_matched_at":        nil,
		"transfer_matching_excluded": true,
		columnUpdatedAt:              params.UpdatedAt,
	}
	if err := s.updatePair(
		ctx,
		params.TenantID,
		params.FirstTransactionID,
		params.SecondTransactionID,
		updates,
	); err != nil {
		return fmt.Errorf("unlink transfer pair: %w", err)
	}
	return nil
}

func (s *TransferPairStore) updatePair(
	ctx context.Context,
	tenantID string,
	firstTransactionID string,
	secondTransactionID string,
	updates map[string]any,
) error {
	if firstTransactionID == secondTransactionID {
		return ErrTransferPairTransactionIDsMustDiffer
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := updateTransferPairLeg(tx, tenantID, firstTransactionID, updates); err != nil {
			return err
		}
		if err := updateTransferPairLeg(tx, tenantID, secondTransactionID, updates); err != nil {
			return err
		}
		return nil
	})
}

func updateTransferPairLeg(tx *gorm.DB, tenantID string, transactionID string, updates map[string]any) error {
	result := tx.Model(&transactionModel{}).
		Where("tenant_id = ? AND id = ?", tenantID, transactionID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update transfer pair leg: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrTransferPairTransactionNotFound
	}
	return nil
}
