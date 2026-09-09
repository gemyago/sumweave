package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

var (
	ErrInvalidClassificationRule               = errors.New("invalid classification rule")
	ErrClassificationRuleNotFound              = errors.New("classification rule not found")
	ErrCategoryReferencedByClassificationRules = errors.New("category referenced by classification rules")
	ErrTagReferencedByClassificationRules      = errors.New("tag referenced by classification rules")
)

type CategoryReferencedByClassificationRulesError struct {
	RuleIDs []string
}

func (e *CategoryReferencedByClassificationRulesError) Error() string {
	return ErrCategoryReferencedByClassificationRules.Error()
}

func (e *CategoryReferencedByClassificationRulesError) Is(target error) bool {
	return target == ErrCategoryReferencedByClassificationRules
}

type TagReferencedByClassificationRulesError struct {
	RuleIDs []string
}

func (e *TagReferencedByClassificationRulesError) Error() string {
	return ErrTagReferencedByClassificationRules.Error()
}

func (e *TagReferencedByClassificationRulesError) Is(target error) bool {
	return target == ErrTagReferencedByClassificationRules
}

type classificationRuleStore interface {
	ListClassificationRules(
		ctx context.Context,
		tenantID string,
		categoryID string,
	) ([]domain.ClassificationRule, error)
	AppendClassificationRule(ctx context.Context, rule domain.ClassificationRule) (domain.ClassificationRule, error)
	ReplaceClassificationRule(ctx context.Context, rule domain.ClassificationRule) error
	DeleteClassificationRule(ctx context.Context, tenantID string, ruleID string) error
	MoveClassificationRule(ctx context.Context, tenantID string, ruleID string, direction int, now time.Time) error
	ListClassificationRuleIDsReferencingCategory(
		ctx context.Context,
		tenantID string,
		categoryID string,
	) ([]string, error)
	ListClassificationRuleIDsReferencingTag(ctx context.Context, tenantID string, tagID string) ([]string, error)
}

type classificationCategoryStore interface {
	GetCategory(ctx context.Context, categoryID string) (*domain.Category, error)
}

type classificationTagStore interface {
	GetTag(ctx context.Context, tagID string) (*domain.Tag, error)
}

type ClassificationRuleServiceArgs struct {
	Access     accessGuardStore
	Categories classificationCategoryStore
	Tags       classificationTagStore
	Rules      classificationRuleStore
	Now        func() time.Time
	NewID      func() string
}

type ClassificationRuleService struct {
	access     *accessGuard
	categories classificationCategoryStore
	tags       classificationTagStore
	rules      classificationRuleStore
	now        func() time.Time
	newID      func() string
}

func NewClassificationRuleService(args ClassificationRuleServiceArgs) (*ClassificationRuleService, error) {
	if args.Access == nil {
		return nil, errors.New("classification rule access store is required")
	}
	if args.Categories == nil {
		return nil, errors.New("classification rule category store is required")
	}
	if args.Tags == nil {
		return nil, errors.New("classification rule tag store is required")
	}
	if args.Rules == nil {
		return nil, errors.New("classification rule store is required")
	}
	if args.Now == nil {
		return nil, errors.New("classification rule clock is required")
	}
	if args.NewID == nil {
		return nil, errors.New("classification rule ID generator is required")
	}
	return &ClassificationRuleService{
		access: newAccessGuard(args.Access), categories: args.Categories, tags: args.Tags, rules: args.Rules,
		now: args.Now, newID: args.NewID,
	}, nil
}

type ListClassificationRulesParams struct {
	ActorUserID string
	TenantID    string
	CategoryID  string
}

type CreateClassificationRuleParams struct {
	ActorUserID string
	TenantID    string
	MatchType   domain.ClassificationMatchType
	Condition   string
	CategoryID  string
	TagIDs      []string
}

type UpdateClassificationRuleParams struct {
	ActorUserID string
	TenantID    string
	RuleID      string
	MatchType   domain.ClassificationMatchType
	Condition   string
	CategoryID  string
	TagIDs      []string
}

type DeleteClassificationRuleParams struct {
	ActorUserID string
	TenantID    string
	RuleID      string
}

type MoveClassificationRuleParams struct {
	ActorUserID string
	TenantID    string
	RuleID      string
	Direction   ClassificationRuleMoveDirection
}

type ClassificationRuleMoveDirection string

const (
	ClassificationRuleMoveDirectionUp   ClassificationRuleMoveDirection = "up"
	ClassificationRuleMoveDirectionDown ClassificationRuleMoveDirection = "down"
)

func (s *ClassificationRuleService) List(
	ctx context.Context,
	params ListClassificationRulesParams,
) ([]domain.ClassificationRule, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	rules, err := s.rules.ListClassificationRules(
		ctx,
		strings.TrimSpace(params.TenantID),
		strings.TrimSpace(params.CategoryID),
	)
	if err != nil {
		return nil, fmt.Errorf("list classification rules: %w", err)
	}
	return rules, nil
}

func (s *ClassificationRuleService) Create(
	ctx context.Context,
	params CreateClassificationRuleParams,
) (domain.ClassificationRule, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.ClassificationRule{}, err
	}
	condition, category, tagIDs, err := s.validateRule(
		ctx,
		params.TenantID,
		params.MatchType,
		params.Condition,
		params.CategoryID,
		params.TagIDs,
	)
	if err != nil {
		return domain.ClassificationRule{}, err
	}
	now := s.now()
	rule, err := s.rules.AppendClassificationRule(ctx, domain.ClassificationRule{
		ID: s.newID(), TenantID: strings.TrimSpace(params.TenantID), MatchType: params.MatchType,
		Condition: condition, CategoryID: category.ID, TagIDs: tagIDs, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return domain.ClassificationRule{}, fmt.Errorf("create classification rule: %w", err)
	}
	return rule, nil
}

func (s *ClassificationRuleService) Update(ctx context.Context, params UpdateClassificationRuleParams) error {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return err
	}
	condition, category, tagIDs, err := s.validateRule(
		ctx,
		params.TenantID,
		params.MatchType,
		params.Condition,
		params.CategoryID,
		params.TagIDs,
	)
	if err != nil {
		return err
	}
	err = s.rules.ReplaceClassificationRule(ctx, domain.ClassificationRule{
		ID: strings.TrimSpace(params.RuleID), TenantID: strings.TrimSpace(params.TenantID),
		MatchType: params.MatchType, Condition: condition, CategoryID: category.ID, TagIDs: tagIDs, UpdatedAt: s.now(),
	})
	if errors.Is(err, persistence.ErrClassificationRuleNotFound) {
		return ErrClassificationRuleNotFound
	}
	if err != nil {
		return fmt.Errorf("update classification rule: %w", err)
	}
	return nil
}

func (s *ClassificationRuleService) Delete(ctx context.Context, params DeleteClassificationRuleParams) error {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return err
	}
	if err := s.rules.DeleteClassificationRule(
		ctx,
		strings.TrimSpace(params.TenantID),
		strings.TrimSpace(params.RuleID),
	); err != nil {
		if errors.Is(err, persistence.ErrClassificationRuleNotFound) {
			return ErrClassificationRuleNotFound
		}
		return fmt.Errorf("delete classification rule: %w", err)
	}
	return nil
}

func (s *ClassificationRuleService) Move(ctx context.Context, params MoveClassificationRuleParams) error {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return err
	}
	direction, err := classificationRuleMoveDirection(params.Direction)
	if err != nil {
		return err
	}
	moveErr := s.rules.MoveClassificationRule(
		ctx,
		strings.TrimSpace(params.TenantID),
		strings.TrimSpace(params.RuleID),
		direction,
		s.now(),
	)
	if moveErr != nil {
		if errors.Is(moveErr, persistence.ErrClassificationRuleNotFound) {
			return ErrClassificationRuleNotFound
		}
		return fmt.Errorf("move classification rule: %w", moveErr)
	}
	return nil
}

func (s *ClassificationRuleService) ReferencingRuleIDs(
	ctx context.Context,
	tenantID string,
	categoryID string,
) ([]string, error) {
	ids, err := s.rules.ListClassificationRuleIDsReferencingCategory(ctx, tenantID, categoryID)
	if err != nil {
		return nil, fmt.Errorf("list classification rule references: %w", err)
	}
	return ids, nil
}

func (s *ClassificationRuleService) validateRule(
	ctx context.Context,
	tenantID string,
	matchType domain.ClassificationMatchType,
	condition string,
	categoryID string,
	tagIDs []string,
) (string, domain.Category, []string, error) {
	normalizedTagIDs, err := normalizedClassificationRuleTagIDs(tagIDs)
	if err != nil {
		return "", domain.Category{}, nil, err
	}
	if matchType != domain.ClassificationMatchTypeExact && matchType != domain.ClassificationMatchTypeContains {
		return "", domain.Category{}, nil, fmt.Errorf("%w: match type", ErrInvalidClassificationRule)
	}
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return "", domain.Category{}, nil, fmt.Errorf("%w: condition", ErrInvalidClassificationRule)
	}
	category, err := s.categories.GetCategory(ctx, strings.TrimSpace(categoryID))
	if err != nil {
		if errors.Is(err, persistence.ErrCategoryNotFound) {
			return "", domain.Category{}, nil, ErrCategoryNotFound
		}
		return "", domain.Category{}, nil, fmt.Errorf("get classification rule category: %w", err)
	}
	if category.TenantID != strings.TrimSpace(tenantID) || category.HiddenAt != nil {
		return "", domain.Category{}, nil, ErrCategoryNotFound
	}
	for _, tagID := range normalizedTagIDs {
		tag, tagErr := s.tags.GetTag(ctx, tagID)
		if tagErr != nil {
			if errors.Is(tagErr, persistence.ErrTagNotFound) {
				return "", domain.Category{}, nil, ErrTagNotFound
			}
			return "", domain.Category{}, nil, fmt.Errorf("get classification rule tag: %w", tagErr)
		}
		if tag.TenantID != strings.TrimSpace(tenantID) || tag.HiddenAt != nil {
			return "", domain.Category{}, nil, ErrTagNotFound
		}
	}
	return condition, *category, normalizedTagIDs, nil
}

func normalizedClassificationRuleTagIDs(tagIDs []string) ([]string, error) {
	result := make([]string, 0, len(tagIDs))
	seen := make(map[string]struct{}, len(tagIDs))
	for _, tagID := range tagIDs {
		trimmedTagID := strings.TrimSpace(tagID)
		if trimmedTagID == "" {
			return nil, fmt.Errorf("%w: tag ID", ErrInvalidClassificationRule)
		}
		if _, ok := seen[trimmedTagID]; ok {
			return nil, fmt.Errorf("%w: duplicate tag ID", ErrInvalidClassificationRule)
		}
		seen[trimmedTagID] = struct{}{}
		result = append(result, trimmedTagID)
	}
	return result, nil
}

func classificationRuleMoveDirection(direction ClassificationRuleMoveDirection) (int, error) {
	switch direction {
	case ClassificationRuleMoveDirectionUp:
		return -1, nil
	case ClassificationRuleMoveDirectionDown:
		return 1, nil
	default:
		return 0, fmt.Errorf("%w: move direction", ErrInvalidClassificationRule)
	}
}
