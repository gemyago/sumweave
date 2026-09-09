package finance

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestClassificationRuleService(t *testing.T) {
	t.Run("checks membership and validates a visible same-tenant category before appending", func(t *testing.T) {
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		now := time.Date(2026, time.September, 6, 13, 0, 0, 0, time.FixedZone("test", 60*60))
		params := CreateClassificationRuleParams{
			ActorUserID: "user-a", TenantID: "tenant-a", MatchType: domain.ClassificationMatchTypeContains,
			Condition: "  condition  ", CategoryID: "category-a",
		}
		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		categories.EXPECT().
			GetCategory(t.Context(), params.CategoryID).
			Return(&domain.Category{ID: params.CategoryID, TenantID: params.TenantID}, nil).
			Once()
		rules.EXPECT().AppendClassificationRule(t.Context(), domain.ClassificationRule{
			ID: "rule-a", TenantID: params.TenantID, MatchType: params.MatchType, Condition: "condition",
			CategoryID: params.CategoryID, TagIDs: []string{}, CreatedAt: now, UpdatedAt: now,
		}).Return(domain.ClassificationRule{ID: "rule-a", Position: 1}, nil).Once()
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access:     access,
			Categories: categories,
			Tags:       tags,
			Rules:      rules,
			Now:        func() time.Time { return now },
			NewID:      func() string { return "rule-a" },
		})
		require.NoError(t, err)
		created, err := service.Create(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, "rule-a", created.ID)
	})

	t.Run("validates visible same-tenant tags before persisting a complete rule tag set", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 14, 0, 0, 0, time.FixedZone("test", 60*60))
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		params := CreateClassificationRuleParams{
			ActorUserID: "user-" + fake.UUID().V4(), TenantID: "tenant-" + fake.UUID().V4(),
			MatchType: domain.ClassificationMatchTypeContains, Condition: "condition-" + fake.Lorem().Word(),
			CategoryID: "category-" + fake.UUID().V4(),
			TagIDs:     []string{" tag-a-" + fake.UUID().V4() + " ", "tag-b-" + fake.UUID().V4()},
		}
		expectedTagIDs := []string{params.TagIDs[0][1 : len(params.TagIDs[0])-1], params.TagIDs[1]}
		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		categories.EXPECT().GetCategory(t.Context(), params.CategoryID).
			Return(&domain.Category{ID: params.CategoryID, TenantID: params.TenantID}, nil).Once()
		for _, tagID := range expectedTagIDs {
			tags.EXPECT().GetTag(t.Context(), tagID).
				Return(&domain.Tag{ID: tagID, TenantID: params.TenantID}, nil).Once()
		}
		rules.EXPECT().AppendClassificationRule(t.Context(), domain.ClassificationRule{
			ID: "rule-" + params.TenantID, TenantID: params.TenantID, MatchType: params.MatchType,
			Condition: params.Condition, CategoryID: params.CategoryID, TagIDs: expectedTagIDs,
			CreatedAt: now, UpdatedAt: now,
		}).Return(domain.ClassificationRule{}, nil).Once()
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules,
			Now: func() time.Time { return now }, NewID: func() string { return "rule-" + params.TenantID },
		})
		require.NoError(t, err)
		_, err = service.Create(t.Context(), params)
		require.NoError(t, err)
	})

	t.Run("rejects invalid or unavailable tags without replacing the stored rule", func(t *testing.T) {
		fake := faker.New()
		makeService := func(t *testing.T) (
			*ClassificationRuleService,
			*mockaccessGuardStore,
			*mockclassificationCategoryStore,
			*mockclassificationTagStore,
		) {
			t.Helper()
			access := newMockaccessGuardStore(t)
			categories := newMockclassificationCategoryStore(t)
			tags := newMockclassificationTagStore(t)
			service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
				Access: access, Categories: categories, Tags: tags, Rules: newMockclassificationRuleStore(t),
				Now: time.Now, NewID: fake.UUID().V4,
			})
			require.NoError(t, err)
			return service, access, categories, tags
		}
		params := UpdateClassificationRuleParams{
			ActorUserID: "user-" + fake.UUID().V4(), TenantID: "tenant-" + fake.UUID().V4(),
			RuleID: "rule-" + fake.UUID().V4(), MatchType: domain.ClassificationMatchTypeExact,
			Condition: "condition-" + fake.Lorem().Word(), CategoryID: "category-" + fake.UUID().V4(),
		}

		t.Run("blank or duplicate IDs", func(t *testing.T) {
			service, access, _, _ := makeService(t)
			params.TagIDs = []string{"  ", "tag-" + fake.UUID().V4()}
			access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
			err := service.Update(t.Context(), params)
			require.ErrorIs(t, err, ErrInvalidClassificationRule)

			service, access, _, _ = makeService(t)
			tagID := "tag-" + fake.UUID().V4()
			params.TagIDs = []string{tagID, " " + tagID + " "}
			access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
			err = service.Update(t.Context(), params)
			require.ErrorIs(t, err, ErrInvalidClassificationRule)
		})

		for _, unavailable := range []string{"missing", "hidden", "other tenant"} {
			t.Run(unavailable, func(t *testing.T) {
				service, access, categories, tags := makeService(t)
				tagID := "tag-" + fake.UUID().V4()
				params.TagIDs = []string{tagID}
				access.EXPECT().
					IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).
					Return(true, nil).
					Once()
				categories.EXPECT().GetCategory(t.Context(), params.CategoryID).
					Return(&domain.Category{ID: params.CategoryID, TenantID: params.TenantID}, nil).Once()
				switch unavailable {
				case "missing":
					tags.EXPECT().GetTag(t.Context(), tagID).Return(nil, persistence.ErrTagNotFound).Once()
				case "hidden":
					hiddenAt := time.Now()
					tags.EXPECT().GetTag(t.Context(), tagID).
						Return(&domain.Tag{ID: tagID, TenantID: params.TenantID, HiddenAt: &hiddenAt}, nil).Once()
				default:
					tags.EXPECT().GetTag(t.Context(), tagID).
						Return(&domain.Tag{ID: tagID, TenantID: "tenant-" + fake.UUID().V4()}, nil).Once()
				}

				err := service.Update(t.Context(), params)
				require.ErrorIs(t, err, ErrTagNotFound)
			})
		}
	})

	t.Run("rejects missing required dependencies", func(t *testing.T) {
		_, err := NewClassificationRuleService(ClassificationRuleServiceArgs{})
		require.ErrorContains(t, err, "access store is required")
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		_, err = NewClassificationRuleService(ClassificationRuleServiceArgs{Access: access})
		require.ErrorContains(t, err, "category store is required")
		_, err = NewClassificationRuleService(ClassificationRuleServiceArgs{Access: access, Categories: categories})
		require.ErrorContains(t, err, "tag store is required")
		_, err = NewClassificationRuleService(
			ClassificationRuleServiceArgs{Access: access, Categories: categories, Tags: tags},
		)
		require.ErrorContains(t, err, "rule store is required")
		_, err = NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules,
		})
		require.ErrorContains(t, err, "clock is required")
		_, err = NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules, Now: time.Now,
		})
		require.ErrorContains(t, err, "ID generator is required")
	})

	t.Run("returns ordered list results, create failures, and reference IDs", func(t *testing.T) {
		fake := faker.New()
		tenantID, actorUserID := "tenant-"+fake.UUID().V4(), "user-"+fake.UUID().V4()
		categoryID := "category-" + fake.UUID().V4()
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules, Now: time.Now, NewID: fake.UUID().V4,
		})
		require.NoError(t, err)
		listed := []domain.ClassificationRule{{ID: "rule-" + fake.UUID().V4(), TenantID: tenantID, Position: 1}}
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		rules.EXPECT().ListClassificationRules(t.Context(), tenantID, categoryID).Return(listed, nil).Once()
		actual, err := service.List(t.Context(), ListClassificationRulesParams{
			TenantID: tenantID, ActorUserID: actorUserID, CategoryID: categoryID,
		})
		require.NoError(t, err)
		assert.Equal(t, listed, actual)

		create := CreateClassificationRuleParams{
			TenantID: tenantID, ActorUserID: actorUserID, MatchType: domain.ClassificationMatchTypeExact,
			Condition: "condition-" + fake.Lorem().Word(), CategoryID: categoryID,
		}
		appendErr := errors.New("append failed")
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		categories.EXPECT().
			GetCategory(t.Context(), categoryID).
			Return(&domain.Category{ID: categoryID, TenantID: tenantID}, nil).
			Once()
		rules.EXPECT().
			AppendClassificationRule(t.Context(), mock.Anything).
			Return(domain.ClassificationRule{}, appendErr).
			Once()
		_, err = service.Create(t.Context(), create)
		require.ErrorIs(t, err, appendErr)

		ids := []string{"rule-" + fake.UUID().V4()}
		rules.EXPECT().
			ListClassificationRuleIDsReferencingCategory(t.Context(), tenantID, categoryID).
			Return(ids, nil).
			Once()
		actualIDs, err := service.ReferencingRuleIDs(t.Context(), tenantID, categoryID)
		require.NoError(t, err)
		assert.Equal(t, ids, actualIDs)
	})

	t.Run("exposes the typed category-reference error", func(t *testing.T) {
		fake := faker.New()
		err := &CategoryReferencedByClassificationRulesError{RuleIDs: []string{"rule-" + fake.UUID().V4()}}
		assert.Equal(t, ErrCategoryReferencedByClassificationRules.Error(), err.Error())
		assert.ErrorIs(t, err, ErrCategoryReferencedByClassificationRules)
	})

	t.Run("lists rules and wraps membership and persistence failures", func(t *testing.T) {
		fake := faker.New()
		tenantID, actorUserID := "tenant-"+fake.UUID().V4(), "user-"+fake.UUID().V4()
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules, Now: time.Now, NewID: fake.UUID().V4,
		})
		require.NoError(t, err)
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(false, nil).Once()
		_, err = service.List(t.Context(), ListClassificationRulesParams{TenantID: tenantID, ActorUserID: actorUserID})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		listErr := errors.New("list failed")
		rules.EXPECT().ListClassificationRules(t.Context(), tenantID, "").Return(nil, listErr).Once()
		_, err = service.List(t.Context(), ListClassificationRulesParams{TenantID: tenantID, ActorUserID: actorUserID})
		require.ErrorIs(t, err, listErr)
	})

	t.Run("rejects invalid rule values and unavailable categories", func(t *testing.T) {
		fake := faker.New()
		tenantID, actorUserID := "tenant-"+fake.UUID().V4(), "user-"+fake.UUID().V4()
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules, Now: time.Now, NewID: fake.UUID().V4,
		})
		require.NoError(t, err)
		invalid := CreateClassificationRuleParams{TenantID: tenantID, ActorUserID: actorUserID, MatchType: "invalid"}
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		_, err = service.Create(t.Context(), invalid)
		require.ErrorIs(t, err, ErrInvalidClassificationRule)

		blank := invalid
		blank.MatchType, blank.Condition = domain.ClassificationMatchTypeExact, " \t "
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		_, err = service.Create(t.Context(), blank)
		require.ErrorIs(t, err, ErrInvalidClassificationRule)

		missing := blank
		missing.Condition, missing.CategoryID = "condition-"+fake.Lorem().Word(), "category-"+fake.UUID().V4()
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		categories.EXPECT().
			GetCategory(t.Context(), missing.CategoryID).
			Return(nil, persistence.ErrCategoryNotFound).
			Once()
		_, err = service.Create(t.Context(), missing)
		require.ErrorIs(t, err, ErrCategoryNotFound)

		otherTenant := missing
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		categories.EXPECT().GetCategory(t.Context(), otherTenant.CategoryID).Return(&domain.Category{
			ID: otherTenant.CategoryID, TenantID: "tenant-other-" + fake.UUID().V4(),
		}, nil).Once()
		_, err = service.Create(t.Context(), otherTenant)
		require.ErrorIs(t, err, ErrCategoryNotFound)

		hiddenAt := time.Now()
		hidden := missing
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		categories.EXPECT().GetCategory(t.Context(), hidden.CategoryID).Return(&domain.Category{
			ID: hidden.CategoryID, TenantID: tenantID, HiddenAt: &hiddenAt,
		}, nil).Once()
		_, err = service.Create(t.Context(), hidden)
		require.ErrorIs(t, err, ErrCategoryNotFound)
	})

	t.Run("requires membership before creating a rule", func(t *testing.T) {
		fake := faker.New()
		tenantID, actorUserID := "tenant-"+fake.UUID().V4(), "user-"+fake.UUID().V4()
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules, Now: time.Now, NewID: fake.UUID().V4,
		})
		require.NoError(t, err)
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(false, nil).Once()
		_, err = service.Create(t.Context(), CreateClassificationRuleParams{
			TenantID: tenantID, ActorUserID: actorUserID, MatchType: domain.ClassificationMatchTypeExact,
			Condition: "condition-" + fake.Lorem().Word(), CategoryID: "category-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("requires membership before updating a rule", func(t *testing.T) {
		fake := faker.New()
		tenantID, actorUserID := "tenant-"+fake.UUID().V4(), "user-"+fake.UUID().V4()
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules, Now: time.Now, NewID: fake.UUID().V4,
		})
		require.NoError(t, err)
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(false, nil).Once()
		err = service.Update(t.Context(), UpdateClassificationRuleParams{TenantID: tenantID, ActorUserID: actorUserID})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("updates deletes and moves only member tenant rules", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 6, 17, 0, 0, 0, time.FixedZone("test", 2*60*60))
		tenantID, actorUserID := "tenant-"+fake.UUID().V4(), "user-"+fake.UUID().V4()
		categoryID, ruleID := "category-"+fake.UUID().V4(), "rule-"+fake.UUID().V4()
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access:     access,
			Categories: categories,
			Tags:       tags,
			Rules:      rules,
			Now:        func() time.Time { return now },
			NewID:      fake.UUID().V4,
		})
		require.NoError(t, err)

		update := UpdateClassificationRuleParams{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			RuleID:      ruleID,
			MatchType:   domain.ClassificationMatchTypeContains,
			Condition:   " condition-" + fake.Lorem().Word(),
			CategoryID:  categoryID,
		}
		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		categories.EXPECT().
			GetCategory(t.Context(), categoryID).
			Return(&domain.Category{ID: categoryID, TenantID: tenantID}, nil).
			Once()
		rules.EXPECT().ReplaceClassificationRule(t.Context(), domain.ClassificationRule{
			ID:         ruleID,
			TenantID:   tenantID,
			MatchType:  update.MatchType,
			Condition:  update.Condition[1:],
			CategoryID: categoryID,
			TagIDs:     []string{},
			UpdatedAt:  now,
		}).Return(nil).Once()
		require.NoError(t, service.Update(t.Context(), update))

		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		rules.EXPECT().
			DeleteClassificationRule(t.Context(), tenantID, ruleID).
			Return(persistence.ErrClassificationRuleNotFound).
			Once()
		require.ErrorIs(
			t,
			service.Delete(
				t.Context(),
				DeleteClassificationRuleParams{TenantID: tenantID, ActorUserID: actorUserID, RuleID: ruleID},
			),
			ErrClassificationRuleNotFound,
		)

		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		rules.EXPECT().MoveClassificationRule(t.Context(), tenantID, ruleID, -1, now).Return(nil).Once()
		require.NoError(t, service.Move(t.Context(), MoveClassificationRuleParams{
			TenantID: tenantID, ActorUserID: actorUserID, RuleID: ruleID, Direction: ClassificationRuleMoveDirectionUp,
		}))

		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		require.ErrorIs(t, service.Move(t.Context(), MoveClassificationRuleParams{
			TenantID: tenantID, ActorUserID: actorUserID, RuleID: ruleID, Direction: "sideways",
		}), ErrInvalidClassificationRule)
	})

	t.Run("wraps rule mutation and reference failures", func(t *testing.T) {
		fake := faker.New()
		tenantID, actorUserID := "tenant-"+fake.UUID().V4(), "user-"+fake.UUID().V4()
		ruleID, categoryID := "rule-"+fake.UUID().V4(), "category-"+fake.UUID().V4()
		access := newMockaccessGuardStore(t)
		categories := newMockclassificationCategoryStore(t)
		tags := newMockclassificationTagStore(t)
		rules := newMockclassificationRuleStore(t)
		service, err := NewClassificationRuleService(ClassificationRuleServiceArgs{
			Access: access, Categories: categories, Tags: tags, Rules: rules, Now: time.Now, NewID: fake.UUID().V4,
		})
		require.NoError(t, err)
		operationErr := errors.New("operation failed")

		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		categories.EXPECT().
			GetCategory(t.Context(), categoryID).
			Return(&domain.Category{ID: categoryID, TenantID: tenantID}, nil).
			Once()
		rules.EXPECT().ReplaceClassificationRule(t.Context(), mock.Anything).Return(operationErr).Once()
		require.ErrorIs(t, service.Update(t.Context(), UpdateClassificationRuleParams{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			RuleID:      ruleID,
			MatchType:   domain.ClassificationMatchTypeExact,
			Condition:   "condition-" + fake.Lorem().Word(),
			CategoryID:  categoryID,
		}), operationErr)

		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		rules.EXPECT().DeleteClassificationRule(t.Context(), tenantID, ruleID).Return(operationErr).Once()
		require.ErrorIs(
			t,
			service.Delete(
				t.Context(),
				DeleteClassificationRuleParams{TenantID: tenantID, ActorUserID: actorUserID, RuleID: ruleID},
			),
			operationErr,
		)

		access.EXPECT().IsTenantMember(t.Context(), tenantID, actorUserID).Return(true, nil).Once()
		rules.EXPECT().
			MoveClassificationRule(t.Context(), tenantID, ruleID, 1, mock.Anything).
			Return(operationErr).
			Once()
		require.ErrorIs(t, service.Move(t.Context(), MoveClassificationRuleParams{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			RuleID:      ruleID,
			Direction:   ClassificationRuleMoveDirectionDown,
		}), operationErr)

		rules.EXPECT().
			ListClassificationRuleIDsReferencingCategory(t.Context(), tenantID, categoryID).
			Return(nil, operationErr).
			Once()
		_, err = service.ReferencingRuleIDs(t.Context(), tenantID, categoryID)
		require.ErrorIs(t, err, operationErr)
	})
}
