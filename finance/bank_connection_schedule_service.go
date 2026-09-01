package finance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

// ScheduledSemanticCommandPublisher publishes a semantic command in the
// schedule state transaction. Its implementation belongs to the application.
type ScheduledSemanticCommandPublisher interface {
	PublishScheduledSemanticCommand(context.Context, *sql.Tx, SemanticCommand) (DispatchReference, error)
}

// BankConnectionScheduleService advances due bank-sync occurrences and stores
// the resulting future job reference in the authoritative finance schedule.
type BankConnectionScheduleService struct {
	store     *persistence.BankConnectionScheduleStore
	publisher ScheduledSemanticCommandPublisher
	now       func() time.Time
}

type BankConnectionScheduleServiceOption func(*BankConnectionScheduleService)

func WithBankConnectionScheduleServiceNow(now func() time.Time) BankConnectionScheduleServiceOption {
	return func(service *BankConnectionScheduleService) { service.now = now }
}

func WithBankConnectionScheduleServicePublisher(
	publisher ScheduledSemanticCommandPublisher,
) BankConnectionScheduleServiceOption {
	return func(service *BankConnectionScheduleService) { service.publisher = publisher }
}

func NewBankConnectionScheduleService(
	store *persistence.BankConnectionScheduleStore,
	opts ...BankConnectionScheduleServiceOption,
) *BankConnectionScheduleService {
	if store == nil {
		panic("bank connection schedule store is required")
	}
	service := &BankConnectionScheduleService{store: store, now: time.Now}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

// EnqueueDue publishes one command for each currently due occurrence. The
// occurrence advance and its dispatch reference commit in the same SQL
// transaction, so a failed publication or state write leaves the occurrence due.
func (s *BankConnectionScheduleService) EnqueueDue(ctx context.Context) (int, error) {
	if s.publisher == nil {
		return 0, errors.New("scheduled bank sync command publisher is required")
	}
	now := s.now()
	schedules, err := s.store.ListDue(ctx, now)
	if err != nil { // coverage-ignore // Persistence failures are covered by the focused store tests.
		return 0, fmt.Errorf("list due bank connection schedules: %w", err)
	}
	count := 0
	for _, schedule := range schedules {
		claimed, enqueueErr := s.enqueueOccurrence(ctx, schedule, now)
		if enqueueErr != nil {
			return count, enqueueErr
		}
		if claimed {
			count++
		}
	}
	return count, nil
}

func (s *BankConnectionScheduleService) enqueueOccurrence(
	ctx context.Context,
	schedule domain.BankConnectionSchedule,
	now time.Time,
) (bool, error) {
	if schedule.NextRunAt == nil { // coverage-ignore // ListDue excludes schedules without an occurrence.
		return false, nil
	}
	claimed := false
	err := s.store.WithTransaction(ctx, func(tx *persistence.BankConnectionScheduleTransaction) error {
		current, dueAt, nextRunAt, wasClaimed, claimErr := s.claimOccurrence(ctx, tx, schedule, now)
		if claimErr != nil {
			return claimErr
		}
		if !wasClaimed {
			return nil
		}
		if publishErr := s.publishClaim(ctx, tx, current, dueAt, nextRunAt, now); publishErr != nil {
			return publishErr
		}
		claimed = true
		return nil
	})
	return claimed, err
}

func (s *BankConnectionScheduleService) claimOccurrence(
	ctx context.Context,
	tx *persistence.BankConnectionScheduleTransaction,
	candidate domain.BankConnectionSchedule,
	now time.Time,
) (domain.BankConnectionSchedule, time.Time, time.Time, bool, error) {
	current, err := tx.Get(ctx, candidate.ConnectionID)
	if errors.Is(err, persistence.ErrBankConnectionScheduleNotFound) {
		return domain.BankConnectionSchedule{}, time.Time{}, time.Time{}, false, nil
	}
	if err != nil {
		return domain.BankConnectionSchedule{}, time.Time{}, time.Time{}, false,
			fmt.Errorf("re-read bank connection schedule: %w", err)
	}
	if current.NextRunAt == nil || !current.Enabled || current.NextRunAt.After(now) ||
		candidate.NextRunAt == nil || !current.NextRunAt.Equal(*candidate.NextRunAt) {
		return domain.BankConnectionSchedule{}, time.Time{}, time.Time{}, false, nil
	}
	if current.Interval <= 0 {
		return domain.BankConnectionSchedule{}, time.Time{}, time.Time{}, false,
			errors.New("bank connection schedule interval must be positive")
	}
	dueAt := *current.NextRunAt
	nextRunAt := dueAt
	for !nextRunAt.After(now) {
		nextRunAt = nextRunAt.Add(current.Interval)
	}
	if !nextRunAt.After(dueAt) { // coverage-ignore // A positive duration always advances this occurrence.
		return domain.BankConnectionSchedule{}, time.Time{}, time.Time{}, false,
			errors.New("bank connection schedule interval must advance next run")
	}
	claimed, err := tx.ClaimDue(ctx, current.ConnectionID, dueAt, nextRunAt, now)
	if err != nil {
		return domain.BankConnectionSchedule{}, time.Time{}, time.Time{}, false, err
	}
	return *current, dueAt, nextRunAt, claimed, nil
}

func (s *BankConnectionScheduleService) publishClaim(
	ctx context.Context,
	tx *persistence.BankConnectionScheduleTransaction,
	schedule domain.BankConnectionSchedule,
	dueAt time.Time,
	nextRunAt time.Time,
	now time.Time,
) error {
	command, err := newSemanticCommand(
		BankConnectionSyncCommandTopic,
		BankConnectionSyncCommand{
			ConnectionID: schedule.ConnectionID,
			Reason:       BankConnectionSyncReasonScheduled,
			Requester:    CommandRequester{Source: CommandRequesterSourceSystem},
			ScheduledAt:  &dueAt, ScheduledNextRunAt: &nextRunAt,
		},
		bankConnectionScheduleOccurrenceKey(schedule.ConnectionID, dueAt),
	)
	if err != nil { // coverage-ignore // JSON encoding of this concrete finance command cannot fail.
		return err
	}
	reference, err := s.publisher.PublishScheduledSemanticCommand(ctx, tx.SQLTransaction(), command)
	if err != nil {
		return fmt.Errorf("publish scheduled bank sync command: %w", err)
	}
	if strings.TrimSpace(reference.MessageID) == "" {
		return errors.New("scheduled bank sync command reference is required")
	}
	if finalizeErr := tx.FinalizeClaim(
		ctx, schedule.ConnectionID, dueAt, nextRunAt, reference.MessageID, now,
	); finalizeErr != nil {
		return finalizeErr
	}
	return nil
}

func bankConnectionScheduleOccurrenceKey(connectionID string, dueAt time.Time) string {
	return "finance.bank-connection-sync:" + strings.TrimSpace(connectionID) + ":" + dueAt.Format(time.RFC3339Nano)
}
