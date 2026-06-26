package jobs

import (
	"context"
	"errors"
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
	now := s.clock().UTC()
	dueSchedules, err := s.store.ListDueSchedules(ctx, now)
	if err != nil {
		return 0, err
	}
	enqueued := 0
	for _, schedule := range dueSchedules {
		dueAt := schedule.NextRunAt.UTC()
		nextRunAt := schedule.NextRunAt.UTC()
		for !nextRunAt.After(now) {
			nextRunAt = nextRunAt.Add(schedule.Interval)
		}
		createdNew := false
		err = s.store.WithTx(ctx, func(tx *StoreTx) error {
			_, txCreatedNew, enqueueErr := s.service.enqueueJSONInTx(ctx, tx, EnqueueJSONParams{
				JobType:        schedule.JobType,
				Requester:      schedule.Requester,
				InputJSON:      schedule.InputJSON,
				IdempotencyKey: scheduleJobIdempotencyKey(schedule.ID, dueAt),
				CorrelationID:  schedule.CorrelationID,
				ScheduleID:     schedule.ID,
			})
			if enqueueErr != nil {
				return enqueueErr
			}
			createdNew = txCreatedNew
			schedule.NextRunAt = nextRunAt
			schedule.LastEnqueuedAt = &dueAt
			return tx.UpsertSchedule(ctx, schedule)
		})
		if err != nil {
			return enqueued, err
		}
		if createdNew {
			enqueued++
		}
	}
	return enqueued, nil
}

func scheduleJobIdempotencyKey(scheduleID string, dueAt time.Time) string {
	return "schedule:" + scheduleID + ":" + dueAt.UTC().Format(time.RFC3339Nano)
}
