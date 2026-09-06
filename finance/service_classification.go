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

const ClassificationJobType = "finance.classification"

type ClassificationParams struct {
	TenantID            string
	RangeStart          time.Time
	RangeEndExclusive   time.Time
	MessageID           string
	SourceSyncMessageID string
}

type ClassificationServiceArgs struct {
	Access       accessGuardStore
	Rules        classificationRuleStore
	Transactions classificationTransactionStore
	Categories   classificationCategoryStore
	Logger       *slog.Logger
	Now          func() time.Time
}

type ClassificationService struct {
	access       *accessGuard
	rules        classificationRuleStore
	transactions classificationTransactionStore
	categories   classificationCategoryStore
	logger       *slog.Logger
	now          func() time.Time
	publisher    SemanticCommandPublisher
}

type ClassificationServiceOption func(*ClassificationService)

// WithClassificationServiceCommandPublisher enables explicit classification
// submission for a process root that owns semantic command publication.
func WithClassificationServiceCommandPublisher(publisher SemanticCommandPublisher) ClassificationServiceOption {
	return func(service *ClassificationService) { service.publisher = publisher }
}

func NewClassificationService(
	args ClassificationServiceArgs,
	options ...ClassificationServiceOption,
) (*ClassificationService, error) {
	if args.Access == nil {
		return nil, errors.New("classification access store is required")
	}
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
	service := &ClassificationService{
		access:       newAccessGuard(args.Access),
		rules:        args.Rules,
		transactions: args.Transactions,
		categories:   args.Categories,
		logger:       args.Logger,
		now:          args.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service, nil
}

type SubmitClassificationParams struct {
	ActorUserID       string
	TenantID          string
	RangeStart        time.Time
	RangeEndExclusive time.Time
}

type ClassificationJobRef struct {
	ID string
}

// Submit publishes explicit classification as observed durable work. It never
// executes classification in the caller's request path.
func (s *ClassificationService) Submit(
	ctx context.Context,
	params SubmitClassificationParams,
) (ClassificationJobRef, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return ClassificationJobRef{}, err
	}
	if !params.RangeStart.Before(params.RangeEndExclusive) {
		return ClassificationJobRef{}, fmt.Errorf(
			"%w: start timestamp must be before end timestamp",
			ErrInvalidTimestampRange,
		)
	}
	if s.publisher == nil {
		return ClassificationJobRef{}, errors.New("classification command publisher is required")
	}
	command, err := newSemanticCommand(
		ClassificationExplicitCommandTopic,
		ClassificationExplicitCommand{
			TenantID:          params.TenantID,
			RangeStart:        params.RangeStart,
			RangeEndExclusive: params.RangeEndExclusive,
			Requester: CommandRequester{
				UserID: params.ActorUserID,
				Source: CommandRequesterSourceOperator,
			},
		},
		"finance.classification.explicit:"+params.TenantID+":"+params.ActorUserID+":"+
			params.RangeStart.Format(time.RFC3339Nano)+":"+params.RangeEndExclusive.Format(time.RFC3339Nano),
	)
	if err != nil {
		return ClassificationJobRef{}, err
	}
	reference, err := s.publisher.PublishSemanticCommand(ctx, command)
	if err != nil {
		return ClassificationJobRef{}, fmt.Errorf("publish explicit classification: %w", err)
	}
	return ClassificationJobRef{ID: reference.MessageID}, nil
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
