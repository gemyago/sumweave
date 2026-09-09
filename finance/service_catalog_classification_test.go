package finance

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogServiceClassificationRuleReferences(t *testing.T) {
	makeCategory := func(t *testing.T) (
		*persistence.Store,
		*persistence.ClassificationRuleStore,
		*CatalogService,
		domain.Category,
		string,
	) {
		t.Helper()
		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		ruleStore := persistence.NewClassificationRuleStoreFromStore(store)
		actorUserID := "user-" + fake.UUID().V4()
		tenants := NewTenantService(store)
		tenant, err := tenants.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID: actorUserID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "USD",
			SeedDefaults: false,
		})
		require.NoError(t, err)
		service := NewCatalogService(store, ruleStore)
		category, err := service.CreateCategory(t.Context(), CreateCategoryParams{
			ActorUserID: actorUserID, TenantID: tenant.ID, Name: "category-" + fake.Lorem().Word(),
			Kind: domain.CategoryKindExpense,
		})
		require.NoError(t, err)
		return store, ruleStore, service, category, actorUserID
	}

	t.Run("requires a live rule-reference guard at construction", func(t *testing.T) {
		assert.PanicsWithValue(t, "catalog classification rule reference guard is required", func() {
			NewCatalogService(nil, nil)
		})
	})

	t.Run("hides categories with no referencing rule", func(t *testing.T) {
		store, _, service, category, actorUserID := makeCategory(t)

		err := service.HideCategory(t.Context(), HideCategoryParams{
			ActorUserID: actorUserID, TenantID: category.TenantID, CategoryID: category.ID,
		})

		require.NoError(t, err)
		hidden, getErr := store.GetCategory(t.Context(), category.ID)
		require.NoError(t, getErr)
		assert.NotNil(t, hidden.HiddenAt)
	})

	t.Run("returns every referencing rule ID without hiding the category", func(t *testing.T) {
		store, ruleStore, service, category, actorUserID := makeCategory(t)
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 15, 0, 0, 0, time.FixedZone("test", 2*60*60))
		first, err := ruleStore.AppendClassificationRule(t.Context(), domain.ClassificationRule{
			ID: "rule-" + fake.UUID().V4(), TenantID: category.TenantID,
			MatchType: domain.ClassificationMatchTypeExact, Condition: "condition-" + fake.Lorem().Word(),
			CategoryID: category.ID, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		second, err := ruleStore.AppendClassificationRule(t.Context(), domain.ClassificationRule{
			ID: "rule-" + fake.UUID().V4(), TenantID: category.TenantID,
			MatchType: domain.ClassificationMatchTypeContains, Condition: "condition-" + fake.Lorem().Word(),
			CategoryID: category.ID, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)

		err = service.HideCategory(t.Context(), HideCategoryParams{
			ActorUserID: actorUserID, TenantID: category.TenantID, CategoryID: category.ID,
		})

		var blocked *CategoryReferencedByClassificationRulesError
		require.ErrorAs(t, err, &blocked)
		require.ErrorIs(t, err, ErrCategoryReferencedByClassificationRules)
		assert.Equal(t, []string{first.ID, second.ID}, blocked.RuleIDs)
		stored, getErr := store.GetCategory(t.Context(), category.ID)
		require.NoError(t, getErr)
		assert.Nil(t, stored.HiddenAt)
	})

	t.Run("returns reference lookup failures without hiding the category", func(t *testing.T) {
		fake := faker.New()
		store := persistence.NewStore(openTestDatabase(t))
		references := newMockcategoryRuleReferenceFinder(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenants := NewTenantService(store)
		tenant, err := tenants.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID: actorUserID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "USD",
			SeedDefaults: false,
		})
		require.NoError(t, err)
		service := NewCatalogService(store, references)
		category, err := service.CreateCategory(t.Context(), CreateCategoryParams{
			ActorUserID: actorUserID, TenantID: tenant.ID, Name: "category-" + fake.Lorem().Word(),
			Kind: domain.CategoryKindExpense,
		})
		require.NoError(t, err)
		lookupErr := errors.New("lookup failed")
		references.EXPECT().
			ListClassificationRuleIDsReferencingCategory(t.Context(), category.TenantID, category.ID).
			Return(nil, lookupErr).
			Once()

		err = service.HideCategory(t.Context(), HideCategoryParams{
			ActorUserID: actorUserID, TenantID: category.TenantID, CategoryID: category.ID,
		})

		require.ErrorIs(t, err, lookupErr)
		stored, getErr := store.GetCategory(t.Context(), category.ID)
		require.NoError(t, getErr)
		assert.Nil(t, stored.HiddenAt)
	})

	t.Run(
		"blocks tag hiding until referenced rules are retargeted while retaining transaction tags",
		func(t *testing.T) {
			fake := faker.New()
			database := openTestDatabase(t)
			store := persistence.NewStore(database)
			ruleStore := persistence.NewClassificationRuleStoreFromStore(store)
			transactionStore := persistence.NewTransactionTagStore(database)
			actorUserID := "user-" + fake.UUID().V4()
			tenant, err := NewTenantService(store).CreateTenant(t.Context(), CreateTenantParams{
				ActorUserID: actorUserID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "USD",
				SeedDefaults: false,
			})
			require.NoError(t, err)
			service := NewCatalogService(store, ruleStore)
			tag, err := service.CreateTag(t.Context(), CreateTagParams{
				ActorUserID: actorUserID, TenantID: tenant.ID, Name: "tag-" + fake.Lorem().Word(),
			})
			require.NoError(t, err)
			now := time.Date(2026, time.September, 6, 16, 0, 0, 0, time.FixedZone("test", 2*60*60))
			transaction, err := transactionStore.SaveTransaction(t.Context(), domain.Transaction{
				ID: "transaction-" + fake.UUID().V4(), TenantID: tenant.ID, AccountID: "account-" + fake.UUID().V4(),
				Source: domain.TransactionSourceManual, Status: domain.TransactionStatusBooked,
				Kind: domain.TransactionKindExpense, Currency: "USD", Description: "transaction-" + fake.Lorem().Word(),
				EffectiveAt: now, TagIDs: []string{tag.ID}, CreatedAt: now, UpdatedAt: now,
			})
			require.NoError(t, err)
			rule, err := ruleStore.AppendClassificationRule(t.Context(), domain.ClassificationRule{
				ID: "rule-" + fake.UUID().V4(), TenantID: tenant.ID, CategoryID: "category-" + fake.UUID().V4(),
				MatchType: domain.ClassificationMatchTypeContains, Condition: "condition-" + fake.Lorem().Word(),
				TagIDs: []string{tag.ID}, CreatedAt: now, UpdatedAt: now,
			})
			require.NoError(t, err)

			err = service.HideTag(t.Context(), HideTagParams{
				ActorUserID: actorUserID, TenantID: tenant.ID, TagID: tag.ID,
			})
			var blocked *TagReferencedByClassificationRulesError
			require.ErrorAs(t, err, &blocked)
			require.ErrorIs(t, err, ErrTagReferencedByClassificationRules)
			assert.Equal(t, []string{rule.ID}, blocked.RuleIDs)
			storedTag, err := store.GetTag(t.Context(), tag.ID)
			require.NoError(t, err)
			assert.Nil(t, storedTag.HiddenAt)

			hiddenAt := now.Add(time.Minute)
			tag.HiddenAt = &hiddenAt
			tag.UpdatedAt = hiddenAt
			_, err = store.SaveTag(t.Context(), tag)
			require.ErrorIs(t, err, persistence.ErrTagReferencedByClassificationRules)
			storedTag, err = store.GetTag(t.Context(), tag.ID)
			require.NoError(t, err)
			assert.Nil(t, storedTag.HiddenAt)

			rule.TagIDs = []string{}
			require.NoError(t, ruleStore.ReplaceClassificationRule(t.Context(), rule))
			require.NoError(t, service.HideTag(t.Context(), HideTagParams{
				ActorUserID: actorUserID, TenantID: tenant.ID, TagID: tag.ID,
			}))
			storedTransaction, err := transactionStore.GetTransaction(t.Context(), transaction.ID)
			require.NoError(t, err)
			assert.Equal(t, []string{tag.ID}, storedTransaction.TagIDs)

			secondTag, err := service.CreateTag(t.Context(), CreateTagParams{
				ActorUserID: actorUserID, TenantID: tenant.ID, Name: "tag-" + fake.Lorem().Word(),
			})
			require.NoError(t, err)
			rule.TagIDs = []string{secondTag.ID}
			require.NoError(t, ruleStore.ReplaceClassificationRule(t.Context(), rule))
			err = service.HideTag(t.Context(), HideTagParams{
				ActorUserID: actorUserID, TenantID: tenant.ID, TagID: secondTag.ID,
			})
			require.ErrorIs(t, err, ErrTagReferencedByClassificationRules)
			require.NoError(t, ruleStore.DeleteClassificationRule(t.Context(), tenant.ID, rule.ID))
			require.NoError(t, service.HideTag(t.Context(), HideTagParams{
				ActorUserID: actorUserID, TenantID: tenant.ID, TagID: secondTag.ID,
			}))
		},
	)
}
