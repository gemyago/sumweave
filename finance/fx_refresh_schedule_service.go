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

const dailyFXRefreshInterval = 24 * time.Hour

// FXRefreshScheduleService advances finance-owned FX refresh occurrences and
// records their stable future job/message reference.
type FXRefreshScheduleService struct {
	store     *persistence.FXRefreshScheduleStore
	publisher ScheduledSemanticCommandPublisher
	now       func() time.Time
}

type FXRefreshScheduleServiceOption func(*FXRefreshScheduleService)

func WithFXRefreshScheduleServiceNow(now func() time.Time) FXRefreshScheduleServiceOption {
	return func(service *FXRefreshScheduleService) { service.now = now }
}

func WithFXRefreshScheduleServicePublisher(
	publisher ScheduledSemanticCommandPublisher,
) FXRefreshScheduleServiceOption {
	return func(service *FXRefreshScheduleService) { service.publisher = publisher }
}

func NewFXRefreshScheduleService(
	store *persistence.FXRefreshScheduleStore,
	opts ...FXRefreshScheduleServiceOption,
) *FXRefreshScheduleService {
	if store == nil {
		panic("fx refresh schedule store is required")
	}
	service := &FXRefreshScheduleService{store: store, now: time.Now}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

// EnsureDailySchedule creates the default daily schedule if it has not already
// been created. Existing due state is never reset by process composition.
func (s *FXRefreshScheduleService) EnsureDailySchedule(ctx context.Context, provider string) error {
	now := s.now()
	return s.store.Ensure(ctx, domain.FXRefreshSchedule{
		ScheduleID: FXDailyRefreshScheduleID,
		Provider:   strings.TrimSpace(provider),
		Interval:   dailyFXRefreshInterval,
		NextRunAt:  &now,
		Enabled:    true,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
}

// EnqueueDue publishes exactly one FX refresh command for each due occurrence.
// State advance and publication share one application SQL transaction.
func (s *FXRefreshScheduleService) EnqueueDue(ctx context.Context) (int, error) {
	if s.publisher == nil {
		return 0, errors.New("scheduled fx refresh command publisher is required")
	}
	now := s.now()
	schedules, err := s.store.ListDue(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("list due fx refresh schedules: %w", err)
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

func (s *FXRefreshScheduleService) enqueueOccurrence(
	ctx context.Context,
	schedule domain.FXRefreshSchedule,
	now time.Time,
) (bool, error) {
	if schedule.NextRunAt == nil { // coverage-ignore // ListDue excludes schedules without an occurrence.
		return false, nil
	}
	claimed := false
	err := s.store.WithTransaction(ctx, func(tx *persistence.FXRefreshScheduleTransaction) error {
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

func (s *FXRefreshScheduleService) claimOccurrence(
	ctx context.Context,
	tx *persistence.FXRefreshScheduleTransaction,
	candidate domain.FXRefreshSchedule,
	now time.Time,
) (domain.FXRefreshSchedule, time.Time, time.Time, bool, error) {
	current, err := tx.Get(ctx, candidate.ScheduleID)
	if errors.Is(err, persistence.ErrFXRefreshScheduleNotFound) {
		return domain.FXRefreshSchedule{}, time.Time{}, time.Time{}, false, nil
	}
	if err != nil {
		return domain.FXRefreshSchedule{}, time.Time{}, time.Time{}, false,
			fmt.Errorf("re-read fx refresh schedule: %w", err)
	}
	if current.NextRunAt == nil || !current.Enabled || current.NextRunAt.After(now) ||
		candidate.NextRunAt == nil || !current.NextRunAt.Equal(*candidate.NextRunAt) {
		return domain.FXRefreshSchedule{}, time.Time{}, time.Time{}, false, nil
	}
	if current.Interval <= 0 {
		return domain.FXRefreshSchedule{}, time.Time{}, time.Time{}, false,
			errors.New("fx refresh schedule interval must be positive")
	}
	dueAt := *current.NextRunAt
	nextRunAt := dueAt
	for !nextRunAt.After(now) {
		nextRunAt = nextRunAt.Add(current.Interval)
	}
	claimed, err := tx.ClaimDue(ctx, current.ScheduleID, dueAt, nextRunAt, now)
	if err != nil {
		return domain.FXRefreshSchedule{}, time.Time{}, time.Time{}, false, err
	}
	return *current, dueAt, nextRunAt, claimed, nil
}

func (s *FXRefreshScheduleService) publishClaim(
	ctx context.Context,
	tx *persistence.FXRefreshScheduleTransaction,
	schedule domain.FXRefreshSchedule,
	dueAt time.Time,
	nextRunAt time.Time,
	now time.Time,
) error {
	command, err := newSemanticCommand(
		FXRatesRefreshCommandTopic,
		FXRatesRefreshCommand{
			Provider:  schedule.Provider,
			Requester: CommandRequester{Source: CommandRequesterSourceSystem},
		},
		fxRefreshScheduleOccurrenceKey(schedule.ScheduleID, dueAt),
	)
	if err != nil { // coverage-ignore // JSON encoding of this concrete finance command cannot fail.
		return err
	}
	reference, err := s.publisher.PublishScheduledSemanticCommand(ctx, tx.SQLTransaction(), command)
	if err != nil {
		return fmt.Errorf("publish scheduled fx refresh command: %w", err)
	}
	if strings.TrimSpace(reference.MessageID) == "" {
		return errors.New("scheduled fx refresh command reference is required")
	}
	if finalizeErr := tx.FinalizeClaim(
		ctx, schedule.ScheduleID, dueAt, nextRunAt, reference.MessageID, now,
	); finalizeErr != nil {
		return finalizeErr
	}
	return nil
}

func fxRefreshScheduleOccurrenceKey(scheduleID string, dueAt time.Time) string {
	return "finance.fx-rates-refresh:" + strings.TrimSpace(scheduleID) + ":" + dueAt.Format(time.RFC3339Nano)
}
