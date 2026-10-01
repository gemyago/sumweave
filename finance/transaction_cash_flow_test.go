package finance

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestLedgerServiceCashFlowInclusion(t *testing.T) {
	fake := faker.New()
	database := openTestDatabase(t)
	store := persistence.NewStore(database)
	transactionStore := persistence.NewTransactionTagStore(database)
	actorID := fake.UUID().V4()
	tenant, err := NewTenantService(store).CreateTenant(t.Context(), CreateTenantParams{
		ActorUserID: actorID, Name: fake.Company().Name(), DisplayCurrency: "USD",
	})
	require.NoError(t, err)
	accountID := fake.UUID().V4()
	start := fake.Time().Time(time.Now()).Truncate(time.Second)
	end := start.Add(time.Hour)
	amount := int64(fake.IntBetween(1, 10000))
	makeTransaction := func(kind domain.TransactionKind, value int64, index int) domain.Transaction {
		return domain.Transaction{
			ID: fake.UUID().V4(), TenantID: tenant.ID, AccountID: accountID,
			Source: domain.TransactionSourceManual, Status: domain.TransactionStatusBooked,
			Kind: kind, AmountMinor: value, Currency: "USD", Description: fake.Lorem().Sentence(3),
			EffectiveAt: start.Add(time.Duration(index) * time.Minute), CreatedAt: start, UpdatedAt: start,
		}
	}
	var incomeIDs, expenseIDs, neutralIDs []string
	var allIDs []string
	for index, fixture := range []struct {
		kind    domain.TransactionKind
		amount  int64
		group   domain.CashFlowInclusion
		pending bool
		matched bool
	}{
		{kind: domain.TransactionKindRegular, amount: amount, group: domain.CashFlowInclusionIncome},
		{kind: domain.TransactionKindRegular, amount: -amount, group: domain.CashFlowInclusionExpense},
		{kind: domain.TransactionKindIncome, amount: -amount, group: domain.CashFlowInclusionExpense},
		{kind: domain.TransactionKindExpense, amount: amount, group: domain.CashFlowInclusionIncome},
		{kind: domain.TransactionKindRefund, amount: amount, group: domain.CashFlowInclusionExpense},
		{kind: domain.TransactionKindRefund, amount: -amount, group: domain.CashFlowInclusionExpense},
		{kind: domain.TransactionKindTransfer, amount: amount, group: domain.CashFlowInclusionIncome},
		{kind: domain.TransactionKindTransfer, amount: -amount, group: domain.CashFlowInclusionExpense},
		{kind: domain.TransactionKindTransfer, amount: amount, group: domain.CashFlowInclusionNone, matched: true},
		{kind: domain.TransactionKindTransfer, amount: -amount, group: domain.CashFlowInclusionNone, matched: true},
		{kind: domain.TransactionKindReconciliation, amount: -amount, group: domain.CashFlowInclusionNone},
		{kind: domain.TransactionKindOpeningBalance, amount: amount, group: domain.CashFlowInclusionNone},
		{kind: domain.TransactionKindRegular, group: domain.CashFlowInclusionNone},
		{kind: domain.TransactionKindTransfer, group: domain.CashFlowInclusionNone},
		{kind: domain.TransactionKindRegular, amount: amount, group: domain.CashFlowInclusionIncome, pending: true},
		{kind: domain.TransactionKindRefund, amount: amount, group: domain.CashFlowInclusionExpense, pending: true},
		{kind: domain.TransactionKindTransfer, amount: -amount, group: domain.CashFlowInclusionExpense, pending: true, matched: true},
	} {
		item := makeTransaction(fixture.kind, fixture.amount, index)
		if fixture.pending {
			item.Status = domain.TransactionStatusPending
		}
		if fixture.matched {
			groupID := fake.UUID().V4()
			item.TransferGroupID, item.TransferMatchedAt = &groupID, &start
		}
		_, saveErr := transactionStore.SaveTransaction(t.Context(), item)
		require.NoError(t, saveErr)
		allIDs = append(allIDs, item.ID)
		income, expense, _ := reportingContribution(item)
		switch fixture.group {
		case domain.CashFlowInclusionIncome:
			incomeIDs = append(incomeIDs, item.ID)
			require.Equal(t, fixture.amount, income)
			require.Zero(t, expense)
		case domain.CashFlowInclusionExpense:
			expenseIDs = append(expenseIDs, item.ID)
			require.Equal(t, -fixture.amount, expense)
			require.Zero(t, income)
		case domain.CashFlowInclusionNone, domain.CashFlowInclusionBoth:
			neutralIDs = append(neutralIDs, item.ID)
			require.Zero(t, income)
			require.Zero(t, expense)
		}
	}
	// Boundary, account, source and tenant distractors must not reach any page.
	for index := range 5 {
		item := makeTransaction(domain.TransactionKindRegular, amount, 0)
		switch index {
		case 0:
			item.EffectiveAt = start.Add(-time.Second)
		case 1:
			item.EffectiveAt = end
		case 2:
			item.AccountID = fake.UUID().V4()
		case 3:
			item.Source = domain.TransactionSourceProvider
		case 4:
			item.TenantID = fake.UUID().V4()
		}
		_, saveErr := transactionStore.SaveTransaction(t.Context(), item)
		require.NoError(t, saveErr)
	}
	ids := func(items []domain.Transaction) []string {
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, item.ID)
		}
		return result
	}
	for _, dedicated := range []bool{false, true} {
		name := "legacy store"
		options := []LedgerServiceOption{}
		if dedicated {
			name = "transaction tag store"
			options = append(options, WithLedgerServiceTransactionStore(transactionStore))
		}
		t.Run(name, func(t *testing.T) {
			service := NewLedgerService(store, options...)
			base := ListTransactionsParams{
				ActorUserID: actorID, TenantID: tenant.ID, AccountID: accountID,
				Source: domain.TransactionSourceManual, StartDate: start, EndDate: end, SortAscending: true,
			}
			for _, inclusion := range []domain.CashFlowInclusion{"", domain.CashFlowInclusionBoth,
				domain.CashFlowInclusionIncome, domain.CashFlowInclusionExpense, domain.CashFlowInclusionNone} {
				params := base
				params.IncludeCashFlow = inclusion
				want := append([]string{}, neutralIDs...)
				switch inclusion {
				case "", domain.CashFlowInclusionBoth:
					want = allIDs
				case domain.CashFlowInclusionIncome:
					want = append(want, incomeIDs...)
				case domain.CashFlowInclusionExpense:
					want = append(want, expenseIDs...)
				case domain.CashFlowInclusionNone:
					want = neutralIDs
				}
				all, listErr := service.ListTransactions(t.Context(), params)
				require.NoError(t, listErr)
				require.ElementsMatch(t, want, ids(all), inclusion)
				params.Limit = 2
				var paged []domain.Transaction
				for params.Offset = 0; ; params.Offset += params.Limit {
					page, pageErr := service.ListTransactions(t.Context(), params)
					require.NoError(t, pageErr)
					paged = append(paged, page...)
					if len(page) < int(params.Limit) {
						break
					}
				}
				require.Equal(t, ids(all), ids(paged), inclusion)
			}
			t.Run("kind and status compose independently", func(t *testing.T) {
				params := base
				params.IncludeCashFlow = domain.CashFlowInclusionExpense
				params.Kind = domain.TransactionKindRefund
				params.Status = domain.TransactionStatusPending
				items, listErr := service.ListTransactions(t.Context(), params)
				require.NoError(t, listErr)
				require.Len(t, items, 1)
				params.IncludeCashFlow = domain.CashFlowInclusionIncome
				items, listErr = service.ListTransactions(t.Context(), params)
				require.NoError(t, listErr)
				require.Empty(t, items)
			})
			t.Run("rejects invalid inclusion", func(t *testing.T) {
				params := base
				params.IncludeCashFlow = domain.CashFlowInclusion("invalid-" + fake.UUID().V4())
				_, listErr := service.ListTransactions(t.Context(), params)
				require.ErrorIs(t, listErr, ErrInvalidCashFlowInclusion)
			})
		})
	}

	t.Run("hidden history retains cash-flow grouping and historic tag IDs", func(t *testing.T) {
		item := makeTransaction(domain.TransactionKindRefund, amount, 30)
		tag, saveErr := store.SaveTag(t.Context(), domain.Tag{
			ID: fake.UUID().V4(), TenantID: tenant.ID, Name: fake.Lorem().Word(), CreatedAt: start, UpdatedAt: start,
		})
		require.NoError(t, saveErr)
		item.HiddenAt, item.TagIDs = &start, []string{tag.ID}
		_, saveErr = transactionStore.SaveTransaction(t.Context(), item)
		require.NoError(t, saveErr)
		tag.HiddenAt = &start
		_, saveErr = store.SaveTag(t.Context(), tag)
		require.NoError(t, saveErr)
		service := NewLedgerService(store, WithLedgerServiceTransactionStore(transactionStore))
		params := ListTransactionsParams{
			ActorUserID: actorID, TenantID: tenant.ID, StartDate: item.EffectiveAt,
			EndDate: item.EffectiveAt.Add(time.Second), IncludeCashFlow: domain.CashFlowInclusionExpense,
		}
		items, listErr := service.ListTransactions(t.Context(), params)
		require.NoError(t, listErr)
		require.Empty(t, items)
		params.IncludeHidden = true
		items, listErr = service.ListTransactions(t.Context(), params)
		require.NoError(t, listErr)
		require.Equal(t, []string{item.ID}, ids(items))
		require.Equal(t, item.TagIDs, items[0].TagIDs)
		params.IncludeCashFlow = domain.CashFlowInclusionNone
		items, listErr = service.ListTransactions(t.Context(), params)
		require.NoError(t, listErr)
		require.Empty(t, items)
	})
}
