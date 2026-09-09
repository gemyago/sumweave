package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
)

var ErrClassificationRuleNotFound = errors.New("classification rule not found")

type ClassificationRuleStore struct {
	db *gorm.DB
}

func NewClassificationRuleStore(database *Database) *ClassificationRuleStore {
	return &ClassificationRuleStore{db: database.db}
}

func NewClassificationRuleStoreFromStore(store *Store) *ClassificationRuleStore {
	return &ClassificationRuleStore{db: store.db}
}

func (s *ClassificationRuleStore) ListClassificationRules(
	ctx context.Context,
	tenantID string,
	categoryID string,
) ([]domain.ClassificationRule, error) {
	var models []classificationRuleModel
	query := s.db.WithContext(ctx).Where("tenant_id = ?", strings.TrimSpace(tenantID))
	if categoryID != "" {
		query = query.Where("category_id = ?", strings.TrimSpace(categoryID))
	}
	if err := query.Order("position ASC, id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list classification rules: %w", err)
	}
	return s.hydrateClassificationRuleTags(ctx, classificationRulesFromModels(models))
}

func (s *ClassificationRuleStore) AppendClassificationRule(
	ctx context.Context,
	rule domain.ClassificationRule,
) (domain.ClassificationRule, error) {
	var saved domain.ClassificationRule
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maximumPosition int
		if err := tx.Model(&classificationRuleModel{}).
			Where("tenant_id = ?", rule.TenantID).
			Select("COALESCE(MAX(position), 0)").Scan(&maximumPosition).Error; err != nil {
			return fmt.Errorf("read classification rule position: %w", err)
		}
		rule.Position = maximumPosition + 1
		model := newClassificationRuleModel(rule)
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("append classification rule: %w", err)
		}
		saved = classificationRuleFromModel(model)
		if err := replaceClassificationRuleTags(tx, saved.ID, rule.TagIDs); err != nil {
			return err
		}
		saved.TagIDs = append([]string{}, rule.TagIDs...)
		return nil
	}); err != nil {
		return domain.ClassificationRule{}, fmt.Errorf("append classification rule: %w", err)
	}
	return saved, nil
}

func (s *ClassificationRuleStore) ReplaceClassificationRule(
	ctx context.Context,
	rule domain.ClassificationRule,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&classificationRuleModel{}).
			Where("tenant_id = ? AND id = ?", strings.TrimSpace(rule.TenantID), strings.TrimSpace(rule.ID)).
			Updates(map[string]any{
				"match_type":     rule.MatchType,
				"condition":      rule.Condition,
				columnCategoryID: rule.CategoryID,
				columnUpdatedAt:  rule.UpdatedAt,
			})
		if result.Error != nil {
			return fmt.Errorf("replace classification rule: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrClassificationRuleNotFound
		}
		return replaceClassificationRuleTags(tx, rule.ID, rule.TagIDs)
	})
}

func (s *ClassificationRuleStore) DeleteClassificationRule(
	ctx context.Context,
	tenantID string,
	ruleID string,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var model classificationRuleModel
		if err := tx.Where("tenant_id = ? AND id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(ruleID)).
			First(&model).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrClassificationRuleNotFound
			}
			return fmt.Errorf("get classification rule for delete: %w", err)
		}
		if err := tx.Where("rule_id = ?", model.ID).Delete(&classificationRuleTagModel{}).Error; err != nil {
			return fmt.Errorf("delete classification rule tags: %w", err)
		}
		if err := tx.Delete(&model).Error; err != nil {
			return fmt.Errorf("delete classification rule: %w", err)
		}
		if err := tx.Model(&classificationRuleModel{}).
			Where("tenant_id = ? AND position > ?", model.TenantID, model.Position).
			Update("position", gorm.Expr("position - 1")).Error; err != nil {
			return fmt.Errorf("close classification rule position gap: %w", err)
		}
		return nil
	})
}

func (s *ClassificationRuleStore) MoveClassificationRule(
	ctx context.Context,
	tenantID string,
	ruleID string,
	direction int,
	now time.Time,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current classificationRuleModel
		if err := tx.Where("tenant_id = ? AND id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(ruleID)).
			First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrClassificationRuleNotFound
			}
			return fmt.Errorf("get classification rule for move: %w", err)
		}
		query := tx.Where("tenant_id = ?", current.TenantID)
		if direction < 0 {
			query = query.Where("position < ?", current.Position).Order("position DESC, id DESC")
		} else {
			query = query.Where("position > ?", current.Position).Order("position ASC, id ASC")
		}
		var neighbor classificationRuleModel
		if err := query.First(&neighbor).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("get adjacent classification rule: %w", err)
		}
		if err := tx.Model(&classificationRuleModel{}).Where("id = ?", current.ID).
			Updates(map[string]any{"position": neighbor.Position, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("move classification rule: %w", err)
		}
		if err := tx.Model(&classificationRuleModel{}).Where("id = ?", neighbor.ID).
			Updates(map[string]any{"position": current.Position, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("move adjacent classification rule: %w", err)
		}
		return nil
	})
}

func (s *ClassificationRuleStore) ListClassificationRuleIDsReferencingCategory(
	ctx context.Context,
	tenantID string,
	categoryID string,
) ([]string, error) {
	var ids []string
	if err := s.db.WithContext(ctx).Model(&classificationRuleModel{}).
		Where("tenant_id = ? AND category_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(categoryID)).
		Order("position ASC, id ASC").Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list classification rule category references: %w", err)
	}
	return ids, nil
}

func (s *ClassificationRuleStore) ListClassificationRuleIDsReferencingTag(
	ctx context.Context,
	tenantID string,
	tagID string,
) ([]string, error) {
	var ids []string
	if err := s.db.WithContext(ctx).
		Table((classificationRuleTagModel{}).TableName()+" AS rule_tags").
		Joins("JOIN finance_classification_rules AS rules ON rules.id = rule_tags.rule_id").
		Where("rules.tenant_id = ? AND rule_tags.tag_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(tagID)).
		Order("rules.position ASC, rules.id ASC").
		Pluck("rule_tags.rule_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list classification rule tag references: %w", err)
	}
	return ids, nil
}

func classificationRulesFromModels(models []classificationRuleModel) []domain.ClassificationRule {
	rules := make([]domain.ClassificationRule, 0, len(models))
	for _, model := range models {
		rules = append(rules, classificationRuleFromModel(model))
	}
	return rules
}

func replaceClassificationRuleTags(tx *gorm.DB, ruleID string, tagIDs []string) error {
	if err := tx.Where("rule_id = ?", ruleID).Delete(&classificationRuleTagModel{}).Error; err != nil {
		return fmt.Errorf("clear classification rule tags: %w", err)
	}
	if len(tagIDs) == 0 {
		return nil
	}
	assignments := make([]classificationRuleTagModel, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		assignments = append(assignments, classificationRuleTagModel{RuleID: ruleID, TagID: tagID})
	}
	if err := tx.Create(&assignments).Error; err != nil {
		return fmt.Errorf("create classification rule tags: %w", err)
	}
	return nil
}

func (s *ClassificationRuleStore) hydrateClassificationRuleTags(
	ctx context.Context,
	rules []domain.ClassificationRule,
) ([]domain.ClassificationRule, error) {
	if len(rules) == 0 {
		return rules, nil
	}
	ruleIDs := make([]string, 0, len(rules))
	for index := range rules {
		rules[index].TagIDs = []string{}
		ruleIDs = append(ruleIDs, rules[index].ID)
	}
	var assignments []classificationRuleTagModel
	if err := s.db.WithContext(ctx).
		Where("rule_id IN ?", ruleIDs).
		Order("rule_id ASC, tag_id ASC").
		Find(&assignments).Error; err != nil {
		return nil, fmt.Errorf("list classification rule tags: %w", err)
	}
	byRuleID := make(map[string][]string, len(rules))
	for _, assignment := range assignments {
		byRuleID[assignment.RuleID] = append(byRuleID[assignment.RuleID], assignment.TagID)
	}
	for index := range rules {
		if tagIDs := byRuleID[rules[index].ID]; tagIDs != nil {
			rules[index].TagIDs = tagIDs
		}
	}
	return rules, nil
}
