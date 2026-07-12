package finance

import (
	"context"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountBalanceReadStoreAssignmentAndFallback(t *testing.T) {
	t.Run("reuses explicit balance store option", func(t *testing.T) {
		store := persistence.NewStore(openTestDatabase(t))
		explicit := persistence.NewAccountBalanceStoreFromStore(store)

		service := NewCatalogService(
			store,
			WithCatalogServiceAccountBalanceStore(explicit),
		)

		require.Same(t, explicit, service.balanceStore)
	})

	t.Run("falls back to transaction listing for non-persistence stores", func(t *testing.T) {
		fallbackStore := stubStore{
			listTransactionsFn: func(
				_ context.Context,
				tenantID string,
				accountID string,
				_ domain.TransactionSource,
				_ domain.TransactionStatus,
				includeHidden bool,
			) ([]domain.Transaction, error) {
				require.Equal(t, "tenant-1", tenantID)
				require.Equal(t, "account-1", accountID)
				assert.False(t, includeHidden)
				return []domain.Transaction{
					{
						AccountID:   accountID,
						Status:      domain.TransactionStatusBooked,
						AmountMinor: 120,
						EffectiveAt: time.Date(2026, time.June, 20, 11, 0, 0, 0, time.UTC),
					},
					{
						AccountID:   accountID,
						Status:      domain.TransactionStatusPending,
						AmountMinor: -30,
						EffectiveAt: time.Date(2026, time.June, 20, 11, 30, 0, 0, time.UTC),
					},
					{
						AccountID:   accountID,
						Status:      domain.TransactionStatusBooked,
						AmountMinor: 45,
						EffectiveAt: time.Date(2026, time.June, 21, 13, 0, 0, 0, time.UTC),
					},
				}, nil
			},
		}

		balanceStore := &accountBalanceFromTransactionStore{store: fallbackStore}
		cutoff := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		balances, err := balanceStore.ListAccountBalances(t.Context(), persistence.ListAccountBalancesParams{
			TenantID:              "tenant-1",
			AccountIDs:            []string{"account-1"},
			EffectiveAtOnOrBefore: &cutoff,
		})
		require.NoError(t, err)
		require.Len(t, balances, 1)
		assert.Equal(t, domain.AccountBalance{
			AccountID:           "account-1",
			BookedBalanceMinor:  120,
			PendingBalanceMinor: -30,
		}, balances[0])

		emptyBalances, err := balanceStore.ListAccountBalances(t.Context(), persistence.ListAccountBalancesParams{
			TenantID:   "tenant-1",
			AccountIDs: []string{},
		})
		require.NoError(t, err)
		assert.Empty(t, emptyBalances)
	})

	t.Run("fallback includes the cutoff calendar date across opposite offsets", func(t *testing.T) {
		fallbackStore := stubStore{
			listTransactionsFn: func(
				_ context.Context,
				_ string,
				accountID string,
				_ domain.TransactionSource,
				_ domain.TransactionStatus,
				_ bool,
			) ([]domain.Transaction, error) {
				return []domain.Transaction{{
					AccountID:   accountID,
					Status:      domain.TransactionStatusBooked,
					AmountMinor: 125,
					EffectiveAt: time.Date(
						2026, time.October, 25, 23, 30, 0, 0,
						time.FixedZone("UTC-10", -10*60*60),
					),
				}}, nil
			},
		}
		cutoff := time.Date(
			2026, time.October, 25, 0, 0, 0, 0,
			time.FixedZone("UTC+14", 14*60*60),
		)

		balances, err := (&accountBalanceFromTransactionStore{store: fallbackStore}).ListAccountBalances(
			t.Context(),
			persistence.ListAccountBalancesParams{
				TenantID: "tenant-1", AccountIDs: []string{"account-1"}, EffectiveAtOnOrBefore: &cutoff,
			},
		)

		require.NoError(t, err)
		require.Len(t, balances, 1)
		assert.Zero(t, balances[0].BookedBalanceMinor)
	})
}
