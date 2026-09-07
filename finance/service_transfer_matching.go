package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/gemyago/sumweave/finance/persistence"
)

const (
	transferMatchingWindow        = 72 * time.Hour
	transferMatchingMaxCandidates = 2
	TransferMatchingJobType       = "finance.transfer-matching"
)

type transferMatchingPairStore interface {
	ListEligibleTransferMatchingTransactions(
		ctx context.Context,
		params persistence.ListEligibleTransferMatchingTransactionsParams,
	) ([]persistence.TransferMatchingTransaction, error)
	LinkTransferPair(ctx context.Context, params persistence.TransferPairLinkParams) error
}

type TransferMatchingAttemptCounts struct {
	MatchedPairs int
	Unmatched    int
	Ambiguous    int
	Skipped      int
}

type TransferMatchingParams struct {
	TenantID            string
	RangeStart          time.Time
	RangeEndExclusive   time.Time
	MessageID           string
	SourceSyncMessageID string
}

type TransferMatchingSubmission struct {
	ActorUserID       string
	TenantID          string
	RangeStart        time.Time
	RangeEndExclusive time.Time
}

type TransferMatchingJobRef struct {
	ID string
}

type TransferMatchingServiceArgs struct {
	Access accessGuardStore
	Pairs  transferMatchingPairStore
	Logger *slog.Logger
	Now    func() time.Time
	NewID  func() string
}

type TransferMatchingService struct {
	access    *accessGuard
	pairs     transferMatchingPairStore
	logger    *slog.Logger
	now       func() time.Time
	newID     func() string
	publisher SemanticCommandPublisher
}

type TransferMatchingServiceOption func(*TransferMatchingService)

// WithTransferMatchingServiceSubmissionPublisher enables explicit submission
// for process roots that own the later command publication adapter.
func WithTransferMatchingServiceSubmissionPublisher(
	publisher SemanticCommandPublisher,
) TransferMatchingServiceOption {
	return func(service *TransferMatchingService) { service.publisher = publisher }
}

func NewTransferMatchingService(
	args TransferMatchingServiceArgs,
	options ...TransferMatchingServiceOption,
) (*TransferMatchingService, error) {
	if args.Access == nil {
		return nil, errors.New("transfer matching access store is required")
	}
	if args.Pairs == nil {
		return nil, errors.New("transfer matching pair store is required")
	}
	if args.Logger == nil {
		return nil, errors.New("transfer matching logger is required")
	}
	if args.Now == nil {
		return nil, errors.New("transfer matching clock is required")
	}
	if args.NewID == nil {
		return nil, errors.New("transfer matching ID generator is required")
	}
	service := &TransferMatchingService{
		access: newAccessGuard(args.Access), pairs: args.Pairs, logger: args.Logger, now: args.Now, newID: args.NewID,
	}
	for _, option := range options {
		option(service)
	}
	return service, nil
}

// Submit checks the finance-owned request boundary and delegates durable
// publication to a process-root adapter. It never runs matching inline.
func (s *TransferMatchingService) Submit(
	ctx context.Context,
	params TransferMatchingSubmission,
) (TransferMatchingJobRef, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return TransferMatchingJobRef{}, err
	}
	if err := validateTransferMatchingRange(params.RangeStart, params.RangeEndExclusive); err != nil {
		return TransferMatchingJobRef{}, err
	}
	if s.publisher == nil {
		return TransferMatchingJobRef{}, errors.New(
			"transfer matching submission publisher is required",
		) // coverage-ignore // Process roots always configure publication for submission.
	}
	command, err := newSemanticCommand(
		TransferMatchingExplicitCommandTopic,
		TransferMatchingExplicitCommand{
			TenantID: params.TenantID, RangeStart: params.RangeStart, RangeEndExclusive: params.RangeEndExclusive,
			Requester: CommandRequester{UserID: params.ActorUserID, Source: CommandRequesterSourceOperator},
		},
		"finance.transfer-matching.explicit:"+s.newID(),
	)
	if err != nil {
		return TransferMatchingJobRef{}, err
	}
	reference, err := s.publisher.PublishSemanticCommand(ctx, command)
	if err != nil {
		return TransferMatchingJobRef{}, fmt.Errorf("publish transfer matching: %w", err)
	}
	return TransferMatchingJobRef{ID: reference.MessageID}, nil
}

func (s *TransferMatchingService) Match(
	ctx context.Context,
	params TransferMatchingParams,
) (TransferMatchingAttemptCounts, error) {
	startedAt := s.now()
	counts := TransferMatchingAttemptCounts{}
	logger := s.logger.With(
		"tenantId", params.TenantID,
		"messageId", params.MessageID,
		"sourceSyncMessageId", params.SourceSyncMessageID,
		"rangeStart", params.RangeStart,
		"rangeEndExclusive", params.RangeEndExclusive,
	)
	transactions, loadErr := s.loadTransferMatchingTransactions(ctx, params, startedAt, logger)
	if loadErr != nil {
		return counts, loadErr
	}
	plannedPairs, decisionCounts, decisionErr := decideTransferMatchingPairs(
		ctx,
		transactions,
		params.RangeStart,
		params.RangeEndExclusive,
	)
	counts = decisionCounts
	if decisionErr != nil {
		logger.ErrorContext(
			ctx,
			"transfer matching failed",
			"loadedRows",
			len(transactions),
			"matchedPairs",
			counts.MatchedPairs,
			"unmatched",
			counts.Unmatched,
			"ambiguous",
			counts.Ambiguous,
			"skipped",
			counts.Skipped,
			"elapsed",
			time.Since(startedAt),
			"error",
			decisionErr.Error(),
		)
		return counts, decisionErr
	}
	counts, saveErr := s.saveTransferMatchingPairs(ctx, params, startedAt, len(transactions), plannedPairs, counts)
	if saveErr != nil {
		return counts, saveErr
	}
	logger.InfoContext(ctx, "transfer matching complete",
		"loadedRows", len(transactions), "matchedPairs", counts.MatchedPairs, "unmatched", counts.Unmatched,
		"ambiguous", counts.Ambiguous, "skipped", counts.Skipped, "elapsed", time.Since(startedAt),
	)
	return counts, nil
}

func (s *TransferMatchingService) saveTransferMatchingPairs(
	ctx context.Context,
	params TransferMatchingParams,
	startedAt time.Time,
	loadedRows int,
	plannedPairs []transferMatchingPair,
	counts TransferMatchingAttemptCounts,
) (TransferMatchingAttemptCounts, error) {
	logger := s.logger.With(
		"tenantId", params.TenantID,
		"messageId", params.MessageID,
		"sourceSyncMessageId", params.SourceSyncMessageID,
		"rangeStart", params.RangeStart,
		"rangeEndExclusive", params.RangeEndExclusive,
	)
	for _, pair := range plannedPairs {
		if contextErr := ctx.Err(); contextErr != nil {
			logger.ErrorContext(
				ctx,
				"transfer matching failed",
				"loadedRows",
				loadedRows,
				"matchedPairs",
				counts.MatchedPairs,
				"unmatched",
				counts.Unmatched,
				"ambiguous",
				counts.Ambiguous,
				"skipped",
				counts.Skipped,
				"elapsed",
				time.Since(startedAt),
				"error",
				contextErr.Error(),
			)
			return counts, contextErr
		}
		matchedAt := s.now()
		linkErr := s.pairs.LinkTransferPair(ctx, persistence.TransferPairLinkParams{
			TenantID:            params.TenantID,
			FirstTransactionID:  pair.first.ID,
			SecondTransactionID: pair.second.ID,
			TransferGroupID:     s.newID(),
			TransferMatchedAt:   matchedAt,
			UpdatedAt:           matchedAt,
		})
		if linkErr == nil {
			counts.MatchedPairs++
			continue
		}
		if errors.Is(linkErr, persistence.ErrTransferPairTransactionNotFound) {
			counts.Skipped++
			continue
		}
		wrapped := fmt.Errorf("save transfer matching pair: %w", linkErr)
		logger.ErrorContext(
			ctx,
			"transfer matching failed",
			"loadedRows",
			loadedRows,
			"matchedPairs",
			counts.MatchedPairs,
			"unmatched",
			counts.Unmatched,
			"ambiguous",
			counts.Ambiguous,
			"skipped",
			counts.Skipped,
			"elapsed",
			time.Since(startedAt),
			"error",
			wrapped.Error(),
		)
		return counts, wrapped
	}
	if contextErr := ctx.Err(); contextErr != nil {
		logger.ErrorContext(
			ctx,
			"transfer matching failed",
			"loadedRows",
			loadedRows,
			"matchedPairs",
			counts.MatchedPairs,
			"unmatched",
			counts.Unmatched,
			"ambiguous",
			counts.Ambiguous,
			"skipped",
			counts.Skipped,
			"elapsed",
			time.Since(startedAt),
			"error",
			contextErr.Error(),
		)
		return counts, contextErr
	}
	return counts, nil
}

func (s *TransferMatchingService) loadTransferMatchingTransactions(
	ctx context.Context,
	params TransferMatchingParams,
	startedAt time.Time,
	logger *slog.Logger,
) ([]persistence.TransferMatchingTransaction, error) {
	counts := TransferMatchingAttemptCounts{}
	if validationErr := validateTransferMatchingRange(
		params.RangeStart,
		params.RangeEndExclusive,
	); validationErr != nil {
		logger.ErrorContext(
			ctx,
			"transfer matching failed",
			"loadedRows",
			0,
			"matchedPairs",
			counts.MatchedPairs,
			"unmatched",
			counts.Unmatched,
			"ambiguous",
			counts.Ambiguous,
			"skipped",
			counts.Skipped,
			"elapsed",
			time.Since(startedAt),
			"error",
			validationErr.Error(),
		)
		return nil, validationErr
	}
	if contextErr := ctx.Err(); contextErr != nil {
		logger.ErrorContext(
			ctx,
			"transfer matching failed",
			"loadedRows",
			0,
			"matchedPairs",
			counts.MatchedPairs,
			"unmatched",
			counts.Unmatched,
			"ambiguous",
			counts.Ambiguous,
			"skipped",
			counts.Skipped,
			"elapsed",
			time.Since(startedAt),
			"error",
			contextErr.Error(),
		)
		return nil, contextErr
	}
	transactions, loadErr := s.pairs.ListEligibleTransferMatchingTransactions(
		ctx,
		persistence.ListEligibleTransferMatchingTransactionsParams{
			TenantID: params.TenantID, RangeStart: params.RangeStart, RangeEndExclusive: params.RangeEndExclusive,
		},
	)
	if loadErr != nil {
		wrapped := fmt.Errorf("load eligible transfer matching transactions: %w", loadErr)
		logger.ErrorContext(
			ctx,
			"transfer matching failed",
			"loadedRows",
			0,
			"matchedPairs",
			counts.MatchedPairs,
			"unmatched",
			counts.Unmatched,
			"ambiguous",
			counts.Ambiguous,
			"skipped",
			counts.Skipped,
			"elapsed",
			time.Since(startedAt),
			"error",
			wrapped.Error(),
		)
		return nil, wrapped
	}
	return transactions, nil
}

func validateTransferMatchingRange(rangeStart time.Time, rangeEndExclusive time.Time) error {
	if rangeStart.IsZero() {
		return fmt.Errorf("%w: start timestamp is required", ErrInvalidTimestampRange)
	}
	if rangeEndExclusive.IsZero() {
		return fmt.Errorf("%w: end timestamp is required", ErrInvalidTimestampRange)
	}
	if !rangeStart.Before(rangeEndExclusive) {
		return fmt.Errorf("%w: start timestamp must be before end timestamp", ErrInvalidTimestampRange)
	}
	return nil
}

type transferMatchingAmountKey struct {
	currency string
	amount   int64
}

type transferMatchingPair struct {
	first  persistence.TransferMatchingTransaction
	second persistence.TransferMatchingTransaction
}

type transferMatchingPairKey struct {
	firstID  string
	secondID string
}

func decideTransferMatchingPairs(
	ctx context.Context,
	transactions []persistence.TransferMatchingTransaction,
	rangeStart time.Time,
	rangeEndExclusive time.Time,
) ([]transferMatchingPair, TransferMatchingAttemptCounts, error) {
	byAmount, starting := transferMatchingIndex(transactions, rangeStart, rangeEndExclusive)
	accepted := make(map[transferMatchingPairKey]transferMatchingPair)
	counts := TransferMatchingAttemptCounts{}
	for _, transaction := range starting {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, counts, contextErr
		}
		pair, delta := evaluateTransferMatchingStart(transaction, byAmount)
		counts = addTransferMatchingAttemptCounts(counts, delta)
		if pair != nil {
			accepted[transferMatchingPairKey{firstID: pair.first.ID, secondID: pair.second.ID}] = *pair
		}
	}
	pairs := make([]transferMatchingPair, 0, len(accepted))
	for _, pair := range accepted {
		pairs = append(pairs, pair)
	}
	sort.Slice(pairs, func(i int, j int) bool {
		if pairs[i].first.ID != pairs[j].first.ID {
			return pairs[i].first.ID < pairs[j].first.ID
		}
		return pairs[i].second.ID < pairs[j].second.ID
	})
	return pairs, counts, nil
}

func transferMatchingIndex(
	transactions []persistence.TransferMatchingTransaction,
	rangeStart time.Time,
	rangeEndExclusive time.Time,
) (map[transferMatchingAmountKey][]persistence.TransferMatchingTransaction, []persistence.TransferMatchingTransaction) {
	byAmount := make(map[transferMatchingAmountKey][]persistence.TransferMatchingTransaction)
	starting := make([]persistence.TransferMatchingTransaction, 0, len(transactions))
	for _, transaction := range transactions {
		key := transferMatchingAmountKey{currency: transaction.Currency, amount: transaction.AmountMinor}
		byAmount[key] = append(byAmount[key], transaction)
		if !transaction.EffectiveAt.Before(rangeStart) && transaction.EffectiveAt.Before(rangeEndExclusive) {
			starting = append(starting, transaction)
		}
	}
	for key := range byAmount {
		sort.Slice(byAmount[key], func(i int, j int) bool {
			if byAmount[key][i].EffectiveAt.Equal(byAmount[key][j].EffectiveAt) {
				return byAmount[key][i].ID < byAmount[key][j].ID
			}
			return byAmount[key][i].EffectiveAt.Before(byAmount[key][j].EffectiveAt)
		})
	}
	sort.Slice(starting, func(i int, j int) bool {
		if starting[i].EffectiveAt.Equal(starting[j].EffectiveAt) {
			return starting[i].ID < starting[j].ID
		}
		return starting[i].EffectiveAt.Before(starting[j].EffectiveAt)
	})
	return byAmount, starting
}

func evaluateTransferMatchingStart(
	transaction persistence.TransferMatchingTransaction,
	byAmount map[transferMatchingAmountKey][]persistence.TransferMatchingTransaction,
) (*transferMatchingPair, TransferMatchingAttemptCounts) {
	counterparts := transferMatchingCandidates(transaction, byAmount)
	if len(counterparts) == 0 {
		return nil, TransferMatchingAttemptCounts{Unmatched: 1}
	}
	if len(counterparts) > 1 {
		return nil, TransferMatchingAttemptCounts{Ambiguous: 1}
	}
	counterpart := counterparts[0]
	reverseCandidates := transferMatchingCandidates(counterpart, byAmount)
	if len(reverseCandidates) != 1 || reverseCandidates[0].ID != transaction.ID {
		return nil, TransferMatchingAttemptCounts{Ambiguous: 1}
	}
	first, second := transaction, counterpart
	if second.ID < first.ID {
		first, second = second, first
	}
	return &transferMatchingPair{first: first, second: second}, TransferMatchingAttemptCounts{}
}

func addTransferMatchingAttemptCounts(
	current TransferMatchingAttemptCounts,
	delta TransferMatchingAttemptCounts,
) TransferMatchingAttemptCounts {
	return TransferMatchingAttemptCounts{
		MatchedPairs: current.MatchedPairs + delta.MatchedPairs,
		Unmatched:    current.Unmatched + delta.Unmatched,
		Ambiguous:    current.Ambiguous + delta.Ambiguous,
		Skipped:      current.Skipped + delta.Skipped,
	}
}

func transferMatchingCandidates(
	transaction persistence.TransferMatchingTransaction,
	byAmount map[transferMatchingAmountKey][]persistence.TransferMatchingTransaction,
) []persistence.TransferMatchingTransaction {
	if transaction.AmountMinor == math.MinInt64 {
		return nil
	}
	bucket := byAmount[transferMatchingAmountKey{currency: transaction.Currency, amount: -transaction.AmountMinor}]
	if len(bucket) == 0 {
		return nil
	}
	windowStart := transaction.EffectiveAt.Add(-transferMatchingWindow)
	windowEnd := transaction.EffectiveAt.Add(transferMatchingWindow)
	start := sort.Search(len(bucket), func(index int) bool {
		return !bucket[index].EffectiveAt.Before(windowStart)
	})
	candidates := make([]persistence.TransferMatchingTransaction, 0, transferMatchingMaxCandidates)
	for index := start; index < len(bucket); index++ {
		candidate := bucket[index]
		if candidate.EffectiveAt.After(windowEnd) {
			break
		}
		if candidate.AccountID == transaction.AccountID {
			continue
		}
		candidates = append(candidates, candidate)
		if len(candidates) == transferMatchingMaxCandidates {
			break
		}
	}
	return candidates
}
