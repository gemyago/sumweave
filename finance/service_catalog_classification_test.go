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
}
