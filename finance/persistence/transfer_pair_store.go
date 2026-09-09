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

const transferMatchingLoadExtension = 144 * time.Hour

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

// TransferMatchingTransaction is the compact immutable row used by one
// transfer-matching attempt.
type TransferMatchingTransaction struct {
	ID           string
	AccountID    string
	Currency     string
	AmountMinor  int64
	EffectiveAt  time.Time
	Description  string
	ConnectionID *string
}

// ListEligibleTransferMatchingTransactionsParams bounds one complete matching
// load. The caller owns validation of the original requested range.
type ListEligibleTransferMatchingTransactionsParams struct {
	TenantID          string
	RangeStart        time.Time
	RangeEndExclusive time.Time
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

// ListEligibleTransferMatchingTransactions loads every matching-eligible row
// in the complete extended range, without pagination or a result cap.
func (s *TransferPairStore) ListEligibleTransferMatchingTransactions(
	ctx context.Context,
	params ListEligibleTransferMatchingTransactionsParams,
) ([]TransferMatchingTransaction, error) {
	loadStart := params.RangeStart.Add(-transferMatchingLoadExtension)
	loadEndExclusive := params.RangeEndExclusive.Add(transferMatchingLoadExtension)
	var rows []TransferMatchingTransaction
	err := s.db.WithContext(ctx).
		Table((transactionModel{}).TableName()+" AS transactions").
		Select([]string{
			"transactions.id AS id",
			"transactions.account_id AS account_id",
			"transactions.currency AS currency",
			"transactions.amount_minor AS amount_minor",
			"transactions.effective_at AS effective_at",
			"transactions.description AS description",
			"provenance.connection_id AS connection_id",
		}).
		Joins(
			"JOIN "+(accountModel{}).TableName()+
				" AS accounts ON accounts.id = transactions.account_id AND accounts.tenant_id = transactions.tenant_id",
		).
		Joins(
			"LEFT JOIN ("+
				"SELECT transaction_id, CASE WHEN COUNT(DISTINCT connection_id) = 1 THEN MIN(connection_id) ELSE NULL END AS connection_id "+
				"FROM "+(providerTransactionMatchModel{}).TableName()+" GROUP BY transaction_id"+
				") AS provenance ON provenance.transaction_id = transactions.id",
		).
		Where("transactions.tenant_id = ?", params.TenantID).
		Where("transactions.effective_at >= ? AND transactions.effective_at < ?", loadStart, loadEndExclusive).
		Where("transactions.hidden_at IS NULL AND accounts.hidden_at IS NULL").
		Where("transactions.status = ?", string(domain.TransactionStatusBooked)).
		Where("transactions.kind IN ?", []string{
			string(domain.TransactionKindRegular),
			string(domain.TransactionKindExpense),
			string(domain.TransactionKindIncome),
			string(domain.TransactionKindTransfer),
		}).
		Where("transactions.source <> ?", string(domain.TransactionSourceSystem)).
		Where("transactions.amount_minor <> 0").
		Where("transactions.transfer_matching_excluded = ?", false).
		Where("transactions.transfer_group_id IS NULL AND transactions.transfer_matched_at IS NULL").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list eligible transfer matching transactions: %w", err)
	}
	return rows, nil
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
