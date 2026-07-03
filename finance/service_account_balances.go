package finance

import (
	"context"
	"fmt"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
)

type accountBalanceReadStore interface {
	ListAccountBalances(
		ctx context.Context,
		params persistence.ListAccountBalancesParams,
	) ([]domain.AccountBalance, error)
}

type accountBalanceFallbackStore interface {
	ListTransactions(
		ctx context.Context,
		tenantID string,
		accountID string,
		source domain.TransactionSource,
		status domain.TransactionStatus,
		includeHidden bool,
	) ([]domain.Transaction, error)
}

type accountBalanceFromTransactionStore struct {
	store accountBalanceFallbackStore
}

func assignAccountBalanceReadStore(store any, target *accountBalanceReadStore) {
	if *target != nil {
		return
	}
	persistenceStore, ok := store.(*persistence.Store)
	if ok {
		*target = persistence.NewAccountBalanceStoreFromStore(persistenceStore)
		return
	}
	txStore, ok := store.(accountBalanceFallbackStore)
	if ok {
		*target = &accountBalanceFromTransactionStore{store: txStore}
		return
	}
	panic("account balance read store is required")
}

func (s *accountBalanceFromTransactionStore) ListAccountBalances(
	ctx context.Context,
	params persistence.ListAccountBalancesParams,
) ([]domain.AccountBalance, error) {
	if len(params.AccountIDs) == 0 {
		return nil, nil
	}
	items := make([]domain.AccountBalance, 0, len(params.AccountIDs))
	for _, accountID := range params.AccountIDs {
		transactions, err := s.store.ListTransactions(ctx, params.TenantID, accountID, "", "", false)
		if err != nil {
			return nil, fmt.Errorf("list transactions for account balance: %w", err)
		}
		balance := domain.AccountBalance{AccountID: accountID}
		for _, item := range transactions {
			if item.HiddenAt != nil || effectiveAtAfterCutoff(item.EffectiveAt, params.EffectiveAtOnOrBefore) {
				continue
			}
			if item.Status == domain.TransactionStatusBooked {
				balance.BookedBalanceMinor += item.AmountMinor
				continue
			}
			balance.PendingBalanceMinor += item.AmountMinor
		}
		items = append(items, balance)
	}
	return items, nil
}

func effectiveAtAfterCutoff(effectiveAt time.Time, cutoff *time.Time) bool {
	if cutoff == nil {
		return false
	}
	effectiveDay := time.Date(
		effectiveAt.UTC().Year(),
		effectiveAt.UTC().Month(),
		effectiveAt.UTC().Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	)
	cutoffDay := time.Date(
		cutoff.UTC().Year(),
		cutoff.UTC().Month(),
		cutoff.UTC().Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	)
	return effectiveDay.After(cutoffDay)
}
