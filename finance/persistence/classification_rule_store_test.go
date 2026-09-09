package persistence

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClassificationRuleStore(t *testing.T) {
	makeRule := func(fake faker.Faker, tenantID string, categoryID string, now time.Time) domain.ClassificationRule {
		return domain.ClassificationRule{
			ID: "rule-" + fake.UUID().V4(), TenantID: tenantID, CategoryID: categoryID,
			MatchType: domain.ClassificationMatchTypeContains, Condition: "condition-" + fake.Lorem().Word(),
			CreatedAt: now, UpdatedAt: now,
		}
	}

	t.Run("orders duplicate rules, filters tenant and category, and maintains positions", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 10, 0, 0, 0, time.FixedZone("test", 2*60*60))
		store := NewClassificationRuleStore(openTestDatabase(t))
		tenantID := "tenant-" + fake.UUID().V4()
		categoryID := "category-" + fake.UUID().V4()
		firstRule := makeRule(fake, tenantID, categoryID, now)
		firstRule.TagIDs = []string{"tag-b-" + fake.UUID().V4(), "tag-a-" + fake.UUID().V4()}
		first, err := store.AppendClassificationRule(t.Context(), firstRule)
		require.NoError(t, err)
		secondRule := makeRule(fake, tenantID, categoryID, now)
		secondRule.Condition = first.Condition
		second, err := store.AppendClassificationRule(t.Context(), secondRule)
		require.NoError(t, err)
		_, err = store.AppendClassificationRule(
			t.Context(),
			makeRule(fake, "tenant-other-"+fake.UUID().V4(), categoryID, now),
		)
		require.NoError(t, err)

		items, err := store.ListClassificationRules(t.Context(), tenantID, categoryID)
		require.NoError(t, err)
		assert.Equal(t, []string{first.ID, second.ID}, []string{items[0].ID, items[1].ID})
		assert.Equal(t, []int{first.Position, second.Position}, []int{items[0].Position, items[1].Position})
		assert.Equal(t, []string{firstRule.TagIDs[1], firstRule.TagIDs[0]}, items[0].TagIDs)
		assert.Empty(t, items[1].TagIDs)
		referenceIDs, err := store.ListClassificationRuleIDsReferencingTag(t.Context(), tenantID, firstRule.TagIDs[0])
		require.NoError(t, err)
		assert.Equal(t, []string{first.ID}, referenceIDs)
		require.NoError(t, store.MoveClassificationRule(t.Context(), tenantID, second.ID, -1, now.Add(time.Minute)))
		items, err = store.ListClassificationRules(t.Context(), tenantID, "")
		require.NoError(t, err)
		assert.Equal(t, []string{second.ID, first.ID}, []string{items[0].ID, items[1].ID})
		assert.Equal(t, []string{firstRule.TagIDs[1], firstRule.TagIDs[0]}, items[1].TagIDs)
		require.NoError(t, store.DeleteClassificationRule(t.Context(), tenantID, second.ID))
		items, err = store.ListClassificationRules(t.Context(), tenantID, "")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, 1, items[0].Position)
		assert.Equal(t, []string{firstRule.TagIDs[1], firstRule.TagIDs[0]}, items[0].TagIDs)
	})

	t.Run("replaces complete rule tag sets and clears them with an empty set", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 10, 0, 0, 0, time.FixedZone("test", 2*60*60))
		store := NewClassificationRuleStore(openTestDatabase(t))
		rule := makeRule(fake, "tenant-"+fake.UUID().V4(), "category-"+fake.UUID().V4(), now)
		rule.TagIDs = []string{"tag-old-" + fake.UUID().V4()}
		saved, err := store.AppendClassificationRule(t.Context(), rule)
		require.NoError(t, err)

		saved.TagIDs = []string{"tag-new-" + fake.UUID().V4(), "tag-another-" + fake.UUID().V4()}
		saved.UpdatedAt = now.Add(time.Minute)
		require.NoError(t, store.ReplaceClassificationRule(t.Context(), saved))
		items, err := store.ListClassificationRules(t.Context(), rule.TenantID, "")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, []string{saved.TagIDs[1], saved.TagIDs[0]}, items[0].TagIDs)

		saved.TagIDs = []string{}
		require.NoError(t, store.ReplaceClassificationRule(t.Context(), saved))
		items, err = store.ListClassificationRules(t.Context(), rule.TenantID, "")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, []string{}, items[0].TagIDs)
	})

	t.Run("returns an empty rule list with no tag hydration query", func(t *testing.T) {
		fake := faker.New()
		store := NewClassificationRuleStore(openTestDatabase(t))

		items, err := store.ListClassificationRules(t.Context(), "tenant-"+fake.UUID().V4(), "")

		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("treats edge moves as no-ops and rolls back failed replacement", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 11, 0, 0, 0, time.FixedZone("test", -3*60*60))
		store := NewClassificationRuleStore(openTestDatabase(t))
		tenantID := "tenant-" + fake.UUID().V4()
		rule, err := store.AppendClassificationRule(
			t.Context(),
			makeRule(fake, tenantID, "category-"+fake.UUID().V4(), now),
		)
		require.NoError(t, err)
		require.NoError(t, store.MoveClassificationRule(t.Context(), tenantID, rule.ID, -1, now))
		require.NoError(t, store.MoveClassificationRule(t.Context(), tenantID, rule.ID, 1, now))
		rule.Condition = "replacement-" + fake.Lorem().Word()
		rule.TenantID = "tenant-other-" + fake.UUID().V4()
		require.ErrorIs(t, store.ReplaceClassificationRule(t.Context(), rule), ErrClassificationRuleNotFound)
		items, err := store.ListClassificationRules(t.Context(), tenantID, "")
		require.NoError(t, err)
		assert.Equal(t, 1, items[0].Position)
	})

	t.Run("wraps database failures from every operation", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.FixedZone("test", -3*60*60))
		makeClosedStore := func(t *testing.T) *ClassificationRuleStore {
			t.Helper()
			database := openTestDatabase(t)
			sqlDB, err := database.db.DB()
			require.NoError(t, err)
			require.NoError(t, sqlDB.Close())
			return NewClassificationRuleStore(database)
		}
		rule := makeRule(fake, "tenant-"+fake.UUID().V4(), "category-"+fake.UUID().V4(), now)

		_, err := makeClosedStore(t).ListClassificationRules(t.Context(), rule.TenantID, "")
		require.Error(t, err)
		_, err = makeClosedStore(t).AppendClassificationRule(t.Context(), rule)
		require.Error(t, err)
		require.Error(t, makeClosedStore(t).ReplaceClassificationRule(t.Context(), rule))
		require.Error(t, makeClosedStore(t).DeleteClassificationRule(t.Context(), rule.TenantID, rule.ID))
		require.Error(t, makeClosedStore(t).MoveClassificationRule(t.Context(), rule.TenantID, rule.ID, -1, now))
		_, err = makeClosedStore(t).ListClassificationRuleIDsReferencingCategory(
			t.Context(),
			rule.TenantID,
			rule.CategoryID,
		)
		require.Error(t, err)
		_, err = makeClosedStore(t).ListClassificationRuleIDsReferencingTag(
			t.Context(),
			rule.TenantID,
			"tag-"+fake.UUID().V4(),
		)
		require.Error(t, err)
	})

	t.Run("returns not found errors and rolls back duplicate appends", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 13, 0, 0, 0, time.FixedZone("test", -3*60*60))
		store := NewClassificationRuleStore(openTestDatabase(t))
		rule := makeRule(fake, "tenant-"+fake.UUID().V4(), "category-"+fake.UUID().V4(), now)
		saved, err := store.AppendClassificationRule(t.Context(), rule)
		require.NoError(t, err)
		_, err = store.AppendClassificationRule(t.Context(), rule)
		require.Error(t, err)
		ruleWithDuplicateTags := makeRule(fake, rule.TenantID, rule.CategoryID, now)
		duplicateTagID := "tag-" + fake.UUID().V4()
		ruleWithDuplicateTags.TagIDs = []string{duplicateTagID, duplicateTagID}
		_, err = store.AppendClassificationRule(t.Context(), ruleWithDuplicateTags)
		require.Error(t, err)
		items, err := store.ListClassificationRules(t.Context(), rule.TenantID, "")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, saved.ID, items[0].ID)

		saved.Condition = "condition-" + fake.Lorem().Word()
		saved.UpdatedAt = now.Add(time.Minute)
		require.NoError(t, store.ReplaceClassificationRule(t.Context(), saved))
		items, err = store.ListClassificationRules(t.Context(), rule.TenantID, "")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, saved.Position, items[0].Position)
		require.ErrorIs(
			t,
			store.DeleteClassificationRule(t.Context(), rule.TenantID, "missing-"+fake.UUID().V4()),
			ErrClassificationRuleNotFound,
		)
		require.ErrorIs(
			t,
			store.MoveClassificationRule(t.Context(), rule.TenantID, "missing-"+fake.UUID().V4(), 1, now),
			ErrClassificationRuleNotFound,
		)
	})

	t.Run("rolls back a failed deletion", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 14, 0, 0, 0, time.FixedZone("test", -3*60*60))
		database := openTestDatabase(t)
		store := NewClassificationRuleStore(database)
		rule := makeRule(fake, "tenant-"+fake.UUID().V4(), "category-"+fake.UUID().V4(), now)
		_, err := store.AppendClassificationRule(t.Context(), rule)
		require.NoError(t, err)
		deleteErr := errors.New("delete failed")
		database.db.Callback().
			Delete().
			Before("gorm:delete").
			Register("classification_rule_test_delete_failure", func(tx *gorm.DB) {
				tx.AddError(deleteErr)
			})

		err = store.DeleteClassificationRule(t.Context(), rule.TenantID, rule.ID)

		require.ErrorIs(t, err, deleteErr)
		items, listErr := store.ListClassificationRules(t.Context(), rule.TenantID, "")
		require.NoError(t, listErr)
		assert.Len(t, items, 1)
	})

	t.Run("wraps failed position lookups", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 15, 0, 0, 0, time.FixedZone("test", -3*60*60))
		database := openTestDatabase(t)
		store := NewClassificationRuleStore(database)
		lookupErr := errors.New("position lookup failed")
		database.db.Callback().
			Row().
			Before("gorm:row").
			Register("classification_rule_test_position_failure", func(tx *gorm.DB) {
				tx.AddError(lookupErr)
			})

		_, err := store.AppendClassificationRule(t.Context(), makeRule(
			fake,
			"tenant-"+fake.UUID().V4(),
			"category-"+fake.UUID().V4(),
			now,
		))

		require.ErrorIs(t, err, lookupErr)
	})
}
