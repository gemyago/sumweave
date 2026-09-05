package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

type classificationTransactionStore interface {
	ListEligibleClassificationTransactions(
		ctx context.Context,
		params persistence.ListEligibleClassificationTransactionsParams,
	) ([]domain.Transaction, error)
	AssignClassificationCategory(
		ctx context.Context,
		params persistence.AssignClassificationCategoryParams,
	) (bool, error)
}

type ClassificationAttemptCounts struct {
	Classified int
	Unmatched  int
	Skipped    int
}

type ClassificationParams struct {
	TenantID            string
	RangeStart          time.Time
	RangeEndExclusive   time.Time
	MessageID           string
	SourceSyncMessageID string
}

type ClassificationServiceArgs struct {
	Rules        classificationRuleStore
	Transactions classificationTransactionStore
	Categories   classificationCategoryStore
	Logger       *slog.Logger
	Now          func() time.Time
}

type ClassificationService struct {
	rules        classificationRuleStore
	transactions classificationTransactionStore
	categories   classificationCategoryStore
	logger       *slog.Logger
	now          func() time.Time
}

func NewClassificationService(args ClassificationServiceArgs) (*ClassificationService, error) {
	if args.Rules == nil {
		return nil, errors.New("classification rule store is required")
	}
	if args.Transactions == nil {
		return nil, errors.New("classification transaction store is required")
	}
	if args.Categories == nil {
		return nil, errors.New("classification category store is required")
	}
	if args.Logger == nil {
		return nil, errors.New("classification logger is required")
	}
	if args.Now == nil {
		return nil, errors.New("classification clock is required")
	}
	return &ClassificationService{
		rules: args.Rules, transactions: args.Transactions, categories: args.Categories,
		logger: args.Logger, now: args.Now,
	}, nil
}

func (s *ClassificationService) Classify(
	ctx context.Context,
	params ClassificationParams,
) (ClassificationAttemptCounts, error) {
	startedAt := s.now()
	counts := ClassificationAttemptCounts{}
	rules, err := s.rules.ListClassificationRules(ctx, params.TenantID, "")
	if err != nil {
		return counts, s.logClassificationFailure(
			ctx,
			params,
			startedAt,
			counts,
			fmt.Errorf("load classification rules: %w", err),
		)
	}
	lastID := ""
	for {
		transactions, listErr := s.transactions.ListEligibleClassificationTransactions(
			ctx,
			persistence.ListEligibleClassificationTransactionsParams{
				TenantID:          params.TenantID,
				RangeStart:        params.RangeStart,
				RangeEndExclusive: params.RangeEndExclusive,
				AfterID:           lastID,
			},
		)
		if listErr != nil {
			return counts, s.logClassificationFailure(
				ctx,
				params,
				startedAt,
				counts,
				fmt.Errorf("list eligible classification transactions: %w", listErr),
			)
		}
		if len(transactions) == 0 {
			break
		}
		for _, transaction := range transactions {
			lastID = transaction.ID
			delta, classifyErr := s.classifyTransaction(ctx, params.TenantID, transaction, rules)
			counts = addClassificationAttemptCounts(counts, delta)
			if classifyErr != nil {
				return counts, s.logClassificationFailure(ctx, params, startedAt, counts, classifyErr)
			}
		}
		s.logger.InfoContext(ctx, "classification batch complete", classificationLogArgs(params, startedAt, counts)...)
	}
	s.logger.InfoContext(ctx, "classification complete", classificationLogArgs(params, startedAt, counts)...)
	return counts, nil
}

func (s *ClassificationService) classifyTransaction(
	ctx context.Context,
	tenantID string,
	transaction domain.Transaction,
	rules []domain.ClassificationRule,
) (ClassificationAttemptCounts, error) {
	rule := matchClassificationRule(transaction.Description, rules)
	if rule == nil {
		return ClassificationAttemptCounts{Unmatched: 1}, nil
	}
	category, err := s.categories.GetCategory(ctx, rule.CategoryID)
	if err != nil || category == nil || category.TenantID != tenantID || category.HiddenAt != nil {
		if err == nil {
			err = errors.New("category not found")
		}
		return ClassificationAttemptCounts{}, unavailableClassificationCategoryFailure(rule.CategoryID, err)
	}
	assigned, err := s.transactions.AssignClassificationCategory(
		ctx,
		persistence.AssignClassificationCategoryParams{
			TenantID: tenantID, TransactionID: transaction.ID, CategoryID: category.ID, UpdatedAt: s.now(),
		},
	)
	if err != nil {
		return ClassificationAttemptCounts{}, fmt.Errorf("assign classification category: %w", err)
	}
	if assigned {
		return ClassificationAttemptCounts{Classified: 1}, nil
	}
	return ClassificationAttemptCounts{Skipped: 1}, nil
}

func unavailableClassificationCategoryFailure(categoryID string, cause error) error {
	return NewTerminalFailure(
		fmt.Errorf("classification rule category %q is unavailable: %w", categoryID, cause),
		"classification_category_unavailable",
		"A classification rule references an unavailable category.",
		"Update or delete the classification rule before retrying.",
	)
}

func addClassificationAttemptCounts(
	current ClassificationAttemptCounts,
	delta ClassificationAttemptCounts,
) ClassificationAttemptCounts {
	return ClassificationAttemptCounts{
		Classified: current.Classified + delta.Classified,
		Unmatched:  current.Unmatched + delta.Unmatched,
		Skipped:    current.Skipped + delta.Skipped,
	}
}

func (s *ClassificationService) logClassificationFailure(
	ctx context.Context,
	params ClassificationParams,
	startedAt time.Time,
	counts ClassificationAttemptCounts,
	err error,
) error {
	args := classificationLogArgs(params, startedAt, counts)
	args = append(args, "error", err.Error())
	s.logger.ErrorContext(ctx, "classification failed", args...)
	return err
}

func classificationLogArgs(
	params ClassificationParams,
	startedAt time.Time,
	counts ClassificationAttemptCounts,
) []any {
	return []any{
		"tenantId", params.TenantID,
		"messageId", params.MessageID,
		"sourceSyncMessageId", params.SourceSyncMessageID,
		"classified", counts.Classified,
		"unmatched", counts.Unmatched,
		"skipped", counts.Skipped,
		"elapsed", time.Since(startedAt),
	}
}

func matchClassificationRule(description string, rules []domain.ClassificationRule) *domain.ClassificationRule {
	normalizedDescription := strings.TrimSpace(description)
	if normalizedDescription == "" {
		return nil
	}
	for index := range rules {
		condition := strings.TrimSpace(rules[index].Condition)
		if condition == "" {
			continue
		}
		matches := false
		switch rules[index].MatchType {
		case domain.ClassificationMatchTypeExact:
			matches = strings.EqualFold(normalizedDescription, condition)
		case domain.ClassificationMatchTypeContains:
			matches = containsFold(normalizedDescription, condition)
		}
		if matches {
			return &rules[index]
		}
	}
	return nil
}

func containsFold(value string, substring string) bool {
	if substring == "" {
		return false
	}
	for start := range value {
		for end := range value[start:] {
			if end == 0 {
				continue
			}
			if strings.EqualFold(value[start:start+end], substring) {
				return true
			}
		}
		if strings.EqualFold(value[start:], substring) {
			return true
		}
	}
	return false
}
