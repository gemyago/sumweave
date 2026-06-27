package finance

import (
	"slices"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFocusedCoreServices(t *testing.T) {
	makeService := func(t *testing.T) *Service {
		t.Helper()

		database := openTestDatabase(t)
		store := persistence.NewStore(database)

		return NewService(store)
	}

	t.Run("tenant service seeds default categories for new tenants", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()

		tenant, err := service.tenants.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "usd",
		})
		require.NoError(t, err)

		views, err := service.tenants.ListTenantsForUser(t.Context(), ownerUserID)
		require.NoError(t, err)
		require.Len(t, views, 1)
		assert.Equal(t, tenant.ID, views[0].Tenant.ID)

		categories, err := service.catalog.ListCategories(t.Context(), ListCategoriesParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Len(t, categories, len(service.defaultCategories))

		invite, err := service.CreateTenantInvite(t.Context(), CreateTenantInviteParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Recipient:   "recipient-" + fake.Internet().User() + "@example.com",
		})
		require.NoError(t, err)

		invites, err := service.ListTenantInvites(t.Context(), ListTenantInvitesParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, invites, 1)
		assert.Equal(t, invite.ID, invites[0].ID)
	})

	t.Run("tenant service seeds approved default categories and tags as tenant-local copies", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()

		firstTenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-a-" + fake.Company().Name(),
			DisplayCurrency: "usd",
		})
		require.NoError(t, err)
		secondTenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-b-" + fake.Company().Name(),
			DisplayCurrency: "usd",
		})
		require.NoError(t, err)

		firstCategories, err := service.ListCategories(t.Context(), ListCategoriesParams{
			ActorUserID: ownerUserID,
			TenantID:    firstTenant.ID,
		})
		require.NoError(t, err)
		secondCategories, err := service.ListCategories(t.Context(), ListCategoriesParams{
			ActorUserID: ownerUserID,
			TenantID:    secondTenant.ID,
		})
		require.NoError(t, err)
		firstTags, err := service.ListTags(t.Context(), ListTagsParams{
			ActorUserID: ownerUserID,
			TenantID:    firstTenant.ID,
		})
		require.NoError(t, err)
		secondTags, err := service.ListTags(t.Context(), ListTagsParams{
			ActorUserID: ownerUserID,
			TenantID:    secondTenant.ID,
		})
		require.NoError(t, err)

		categoryKindsByName := func(items []domain.Category) map[string]domain.CategoryKind {
			t.Helper()

			result := make(map[string]domain.CategoryKind, len(items))
			for _, item := range items {
				assert.True(t, item.SeededDefault)
				result[item.Name] = item.Kind
			}
			return result
		}
		tagNames := func(items []domain.Tag) []string {
			t.Helper()

			result := make([]string, 0, len(items))
			for _, item := range items {
				result = append(result, item.Name)
			}
			slices.Sort(result)
			return result
		}

		assert.Equal(t, map[string]domain.CategoryKind{
			"Paycheck":                       domain.CategoryKindIncome,
			"Bonus":                          domain.CategoryKindIncome,
			"Interest & Dividends":           domain.CategoryKindIncome,
			"Business Income":                domain.CategoryKindIncome,
			"Other Income":                   domain.CategoryKindIncome,
			"Housing":                        domain.CategoryKindExpense,
			"Utilities":                      domain.CategoryKindExpense,
			"Groceries":                      domain.CategoryKindExpense,
			"Dining & Coffee":                domain.CategoryKindExpense,
			"Transportation":                 domain.CategoryKindExpense,
			"Health & Medical":               domain.CategoryKindExpense,
			"Insurance":                      domain.CategoryKindExpense,
			"Education & Childcare":          domain.CategoryKindExpense,
			"Pets":                           domain.CategoryKindExpense,
			"Personal Care":                  domain.CategoryKindExpense,
			"Entertainment":                  domain.CategoryKindExpense,
			"Shopping":                       domain.CategoryKindExpense,
			"Home Improvement & Furnishings": domain.CategoryKindExpense,
			"Travel & Vacation":              domain.CategoryKindExpense,
			"Gifts & Donations":              domain.CategoryKindExpense,
			"Taxes & Fees":                   domain.CategoryKindExpense,
			"Debt Payments":                  domain.CategoryKindExpense,
			"Miscellaneous":                  domain.CategoryKindExpense,
		}, categoryKindsByName(firstCategories))
		assert.Equal(t, []string{
			"Business",
			"Reimburse",
			"Split",
			"Subscription",
			"Tax",
			"Travel",
		}, tagNames(firstTags))

		firstCategoryIDsByName := make(map[string]string, len(firstCategories))
		secondCategoryIDsByName := make(map[string]string, len(secondCategories))
		for _, item := range firstCategories {
			firstCategoryIDsByName[item.Name] = item.ID
			assert.NotContains(t, []string{"Transfers", "Reconciliation", "Opening Balance"}, item.Name)
		}
		for _, item := range secondCategories {
			secondCategoryIDsByName[item.Name] = item.ID
		}
		assert.NotEqual(t, firstCategoryIDsByName["Groceries"], secondCategoryIDsByName["Groceries"])

		firstTagIDsByName := make(map[string]string, len(firstTags))
		secondTagIDsByName := make(map[string]string, len(secondTags))
		for _, item := range firstTags {
			firstTagIDsByName[item.Name] = item.ID
		}
		for _, item := range secondTags {
			secondTagIDsByName[item.Name] = item.ID
		}
		assert.NotEqual(t, firstTagIDsByName["Travel"], secondTagIDsByName["Travel"])

		_, err = service.UpdateCategory(t.Context(), UpdateCategoryParams{
			ActorUserID: ownerUserID,
			TenantID:    firstTenant.ID,
			CategoryID:  firstCategoryIDsByName["Groceries"],
			Name:        "Groceries Renamed",
		})
		require.NoError(t, err)
		_, err = service.UpdateTag(t.Context(), UpdateTagParams{
			ActorUserID: ownerUserID,
			TenantID:    firstTenant.ID,
			TagID:       firstTagIDsByName["Travel"],
			Name:        "Trip",
		})
		require.NoError(t, err)

		secondCategoriesAfterUpdate, err := service.ListCategories(t.Context(), ListCategoriesParams{
			ActorUserID: ownerUserID,
			TenantID:    secondTenant.ID,
		})
		require.NoError(t, err)
		secondTagsAfterUpdate, err := service.ListTags(t.Context(), ListTagsParams{
			ActorUserID: ownerUserID,
			TenantID:    secondTenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, categoryKindsByName(secondCategories), categoryKindsByName(secondCategoriesAfterUpdate))
		assert.Equal(t, tagNames(secondTags), tagNames(secondTagsAfterUpdate))
	})

	t.Run("catalog service keeps linked account mutations tenant-scoped", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		outsiderUserID := "outsider-" + fake.UUID().V4()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "EUR",
		})
		require.NoError(t, err)

		account, err := service.catalog.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "cash-" + fake.Lorem().Word(),
			Currency:    "eur",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		linked, err := service.catalog.AttachLinkedAccount(t.Context(), AttachLinkedAccountParams{
			ActorUserID:       ownerUserID,
			TenantID:          tenant.ID,
			AccountID:         account.ID,
			Provider:          "provider-" + fake.Lorem().Word(),
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		require.NotNil(t, linked.LinkedAccount)
		assert.Equal(t, domain.AccountKindLinked, linked.Kind)

		_, err = service.catalog.UpdateAccount(t.Context(), UpdateAccountParams{
			ActorUserID: outsiderUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Name:        "forbidden-" + fake.Lorem().Word(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("ledger service isolates balance and summary logic", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)

		account, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "checking-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		effectiveAt := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		_, err = service.ledger.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: 120_00,
			Currency:    "usd",
			Description: "income-" + fake.Lorem().Word(),
			EffectiveAt: effectiveAt,
		})
		require.NoError(t, err)

		_, err = service.ledger.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusPending,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -45_00,
			Currency:    "usd",
			Description: "pending-" + fake.Lorem().Word(),
			EffectiveAt: effectiveAt.Add(time.Hour),
		})
		require.NoError(t, err)

		balance, err := service.ledger.GetAccountBalance(t.Context(), GetAccountBalanceParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(120_00), balance.BookedBalanceMinor)
		assert.Equal(t, int64(-45_00), balance.PendingBalanceMinor)

		summary, err := service.ledger.SummarizeTransactions(t.Context(), SummarizeTransactionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(120_00), summary.IncomeMinor)
		assert.Equal(t, int64(0), summary.ExpenseMinor)
		assert.Equal(t, int64(120_00), summary.NetMinor)
	})

	t.Run("access guard and helper functions keep tenant lookups explicit", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		outsiderUserID := "outsider-" + fake.UUID().V4()
		fallbackGroupID := "fallback-group"
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)

		category, err := service.CreateCategory(t.Context(), CreateCategoryParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "category-" + fake.Lorem().Word(),
			Kind:        domain.CategoryKindExpense,
		})
		require.NoError(t, err)

		tag, err := service.CreateTag(t.Context(), CreateTagParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "tag-" + fake.Lorem().Word(),
		})
		require.NoError(t, err)

		account, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "account-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		txn, err := service.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindTransfer,
			AmountMinor: -10_00,
			Currency:    "USD",
			Description: "transfer-" + fake.Lorem().Word(),
			EffectiveAt: time.Date(2026, time.June, 21, 12, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)

		_, err = service.access.requireTenantCategory(t.Context(), tenant.ID, outsiderUserID, category.ID)
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		_, err = service.access.requireTenantTag(t.Context(), tenant.ID, outsiderUserID, tag.ID)
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		_, err = service.access.requireTenantTransaction(t.Context(), tenant.ID, outsiderUserID, txn.ID)
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		assert.False(t, bookedMatchedTransfer(domain.Transaction{Kind: domain.TransactionKindRegular}))
		assert.Equal(
			t,
			"fallback-group",
			existingTransferGroupID(
				domain.Transaction{},
				domain.Transaction{TransferGroupID: &fallbackGroupID},
			),
		)
	})
}
