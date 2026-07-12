package jobs

import (
	"context"
	"errors"
	"strconv"
	"time"
)

type SchedulerDeps struct {
	Store   *Store
	Service *Service
	Clock   func() time.Time
}

type Scheduler struct {
	store   *Store
	service *Service
	clock   func() time.Time
}

func NewScheduler(deps SchedulerDeps) (*Scheduler, error) {
	if deps.Store == nil {
		return nil, errors.New("jobs store is required")
	}
	if deps.Service == nil {
		return nil, errors.New("jobs service is required")
	}
	if deps.Clock == nil {
		deps.Clock = time.Now
	}
	return &Scheduler{store: deps.Store, service: deps.Service, clock: deps.Clock}, nil
}

func (s *Scheduler) EnqueueDue(ctx context.Context) (int, error) {
	now := s.clock()
	dueSchedules, err := s.store.ListDueSchedules(ctx, now)
	if err != nil {
		return 0, err
	}
	enqueued := 0
	for _, schedule := range dueSchedules {
		createdNew, enqueueErr := s.enqueueDueSchedule(ctx, schedule, now)
		if enqueueErr != nil {
			return enqueued, enqueueErr
		}
		if createdNew {
			enqueued++
		}
	}
	return enqueued, nil
}

func (s *Scheduler) enqueueDueSchedule(ctx context.Context, schedule Schedule, now time.Time) (bool, error) {
	if !schedule.Enabled || schedule.NextRunAt == nil {
		return false, nil
	}
	dueAt := *schedule.NextRunAt
	nextRunAt := dueAt
	for !nextRunAt.After(now) {
		nextRunAt = nextRunAt.Add(schedule.Interval)
	}
	var scheduledJob *Job
	createdNew := false
	err := s.store.WithTx(ctx, func(tx *StoreTx) error {
		created, txCreatedNew, enqueueErr := s.service.enqueueJSONInTx(ctx, tx, EnqueueJSONParams{
			JobType: schedule.JobType, Requester: schedule.Requester, InputJSON: schedule.InputJSON,
			IdempotencyKey: scheduleJobIdempotencyKey(schedule.ID, dueAt),
			CorrelationID:  schedule.CorrelationID, ScheduleID: schedule.ID,
			ScheduledAt: &dueAt, ScheduledNextRunAt: &nextRunAt,
		})
		if enqueueErr != nil {
			return enqueueErr
		}
		createdNew = txCreatedNew
		scheduledJob = created
		schedule.NextRunAt = &nextRunAt
		schedule.LastEnqueuedAt = &dueAt
		return tx.UpsertSchedule(ctx, schedule)
	})
	if err != nil || !createdNew {
		return createdNew, err
	}
	handler, err := s.service.registry.Handler(schedule.JobType)
	if err != nil {
		return false, err
	}
	if scheduledErr := handler.onScheduled(ctx, *scheduledJob); scheduledErr != nil {
		return false, scheduledErr
	}
	return true, nil
}

func scheduleJobIdempotencyKey(scheduleID string, dueAt time.Time) string {
	return "schedule:" + scheduleID + ":" + strconv.FormatInt(dueAt.UnixNano(), 10)
}
