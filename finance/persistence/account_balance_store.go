package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"gorm.io/gorm"
)

type ListAccountBalancesParams struct {
	TenantID              string
	AccountIDs            []string
	EffectiveAtOnOrBefore *time.Time
}

type accountBalanceRow struct {
	AccountID           string `gorm:"column:account_id"`
	BookedBalanceMinor  int64  `gorm:"column:booked_balance_minor"`
	PendingBalanceMinor int64  `gorm:"column:pending_balance_minor"`
}

type AccountBalanceStore struct {
	db *gorm.DB
}

func NewAccountBalanceStore(database *Database) *AccountBalanceStore {
	return &AccountBalanceStore{db: database.db}
}

func NewAccountBalanceStoreFromStore(store *Store) *AccountBalanceStore {
	return &AccountBalanceStore{db: store.db}
}

func (s *AccountBalanceStore) ListAccountBalances(
	ctx context.Context,
	params ListAccountBalancesParams,
) ([]domain.AccountBalance, error) {
	if len(params.AccountIDs) == 0 {
		if params.AccountIDs != nil {
			return nil, nil
		}
	}
	query := s.db.WithContext(ctx).
		Table((transactionModel{}).TableName()).
		Select(
			"account_id, "+
				"COALESCE(SUM(CASE WHEN status = 'booked' THEN amount_minor ELSE 0 END), 0) AS booked_balance_minor, "+
				"COALESCE(SUM(CASE WHEN status <> 'booked' THEN amount_minor ELSE 0 END), 0) AS pending_balance_minor",
		).
		Where("tenant_id = ?", params.TenantID).
		Where("hidden_at IS NULL")
	if params.EffectiveAtOnOrBefore != nil {
		query = applyInstantAtOrBefore(query, "effective_at", *params.EffectiveAtOnOrBefore)
	}
	if len(params.AccountIDs) > 0 {
		query = query.Where("account_id IN ?", params.AccountIDs)
	}
	var rows []accountBalanceRow
	if err := query.Group("account_id").Order("account_id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list account balances: %w", err)
	}
	items := make([]domain.AccountBalance, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.AccountBalance{
			AccountID:           row.AccountID,
			BookedBalanceMinor:  row.BookedBalanceMinor,
			PendingBalanceMinor: row.PendingBalanceMinor,
		})
	}
	return items, nil
}
