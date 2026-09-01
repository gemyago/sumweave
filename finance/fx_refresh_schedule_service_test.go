package finance

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFXRefreshScheduleService(t *testing.T) {
	fake := faker.New()

	makeSchedule := func(now, dueAt time.Time) domain.FXRefreshSchedule {
		return domain.FXRefreshSchedule{
			ScheduleID: fake.UUID().V4(), Provider: "provider-" + fake.Letter(),
			Interval: time.Hour, NextRunAt: &dueAt, Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
	}

	t.Run("publishes and advances one due occurrence only once", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewFXRefreshScheduleStore(database)
		now := time.Now()
		dueAt := now.Add(-time.Hour)
		schedule := makeSchedule(now, dueAt)
		require.NoError(t, store.Save(t.Context(), schedule))
		publisher := NewMockScheduledSemanticCommandPublisher(t)
		messageID := fake.UUID().V4()
		publisher.EXPECT().
			PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.MatchedBy(func(command SemanticCommand) bool {
				var input FXRatesRefreshCommand
				return command.Topic == FXRatesRefreshCommandTopic &&
					command.IdempotencyKey == fxRefreshScheduleOccurrenceKey(schedule.ScheduleID, dueAt) &&
					assert.NoError(t, json.Unmarshal(command.Payload, &input)) &&
					input.Provider == schedule.Provider && input.Requester.Source == CommandRequesterSourceSystem
			})).
			Return(DispatchReference{MessageID: messageID}, nil).
			Once()
		service := NewFXRefreshScheduleService(
			store,
			WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
			WithFXRefreshScheduleServicePublisher(publisher),
		)

		first, err := service.EnqueueDue(t.Context())
		require.NoError(t, err)
		second, err := service.EnqueueDue(t.Context())
		require.NoError(t, err)

		assert.Equal(t, 1, first)
		assert.Zero(t, second)
		actual, err := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, err)
		assert.True(t, actual.LastScheduledAt.Equal(dueAt))
		assert.True(t, actual.NextRunAt.After(now))
		assert.Equal(t, messageID, actual.LastJobID)
	})

	t.Run("rolls back state when publication fails", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewFXRefreshScheduleStore(database)
		now := time.Now()
		schedule := makeSchedule(now, now.Add(-time.Hour))
		require.NoError(t, store.Save(t.Context(), schedule))
		publisher := NewMockScheduledSemanticCommandPublisher(t)
		publisher.EXPECT().PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.Anything).
			Return(DispatchReference{}, assert.AnError).Once()
		service := NewFXRefreshScheduleService(
			store,
			WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
			WithFXRefreshScheduleServicePublisher(publisher),
		)

		_, err := service.EnqueueDue(t.Context())

		require.ErrorIs(t, err, assert.AnError)
		actual, getErr := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, getErr)
		assert.True(t, actual.NextRunAt.Equal(*schedule.NextRunAt))
		assert.Nil(t, actual.LastScheduledAt)
		assert.Empty(t, actual.LastJobID)
	})

	t.Run("initializes the daily schedule once without resetting its future reference", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewFXRefreshScheduleStore(database)
		now := time.Now()
		service := NewFXRefreshScheduleService(
			store,
			WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
		)

		require.NoError(t, service.EnsureDailySchedule(t.Context(), "provider-"+fake.Letter()))
		schedule, err := store.Get(t.Context(), FXDailyRefreshScheduleID)
		require.NoError(t, err)
		nextRunAt := now.Add(time.Hour)
		schedule.NextRunAt = &nextRunAt
		schedule.LastJobID = fake.UUID().V4()
		require.NoError(t, store.Save(t.Context(), *schedule))
		require.NoError(t, service.EnsureDailySchedule(t.Context(), "provider-"+fake.Letter()))

		actual, err := store.Get(t.Context(), FXDailyRefreshScheduleID)
		require.NoError(t, err)
		assert.True(t, actual.NextRunAt.Equal(nextRunAt))
		assert.Equal(t, schedule.LastJobID, actual.LastJobID)
	})

	t.Run("requires a publisher and a positive interval", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewFXRefreshScheduleStore(database)
		now := time.Now()
		schedule := makeSchedule(now, now.Add(-time.Hour))
		require.NoError(t, store.Save(t.Context(), schedule))
		_, err := NewFXRefreshScheduleService(store).EnqueueDue(t.Context())
		require.ErrorContains(t, err, "publisher is required")

		schedule.Interval = 0
		require.NoError(t, store.Save(t.Context(), schedule))
		service := NewFXRefreshScheduleService(
			store,
			WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
			WithFXRefreshScheduleServicePublisher(NewMockScheduledSemanticCommandPublisher(t)),
		)
		_, err = service.EnqueueDue(t.Context())
		require.ErrorContains(t, err, "interval must be positive")
		assert.Panics(t, func() { NewFXRefreshScheduleService(nil) })
	})

	t.Run("rolls back state when publication returns no reference", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewFXRefreshScheduleStore(database)
		now := time.Now()
		schedule := makeSchedule(now, now.Add(-time.Hour))
		require.NoError(t, store.Save(t.Context(), schedule))
		publisher := NewMockScheduledSemanticCommandPublisher(t)
		publisher.EXPECT().PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.Anything).
			Return(DispatchReference{}, nil).Once()
		service := NewFXRefreshScheduleService(
			store,
			WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
			WithFXRefreshScheduleServicePublisher(publisher),
		)

		_, err := service.EnqueueDue(t.Context())

		require.ErrorContains(t, err, "reference is required")
		actual, getErr := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, getErr)
		assert.True(t, actual.NextRunAt.Equal(*schedule.NextRunAt))
		assert.Empty(t, actual.LastJobID)
	})

	t.Run("does not publish a stale occurrence after concurrent pause or reschedule", func(t *testing.T) {
		makeService := func(t *testing.T) (*persistence.FXRefreshScheduleStore, *FXRefreshScheduleService, domain.FXRefreshSchedule, *MockScheduledSemanticCommandPublisher) {
			t.Helper()
			database := openTestDatabase(t)
			store := persistence.NewFXRefreshScheduleStore(database)
			now := time.Now()
			schedule := makeSchedule(now, now.Add(-time.Hour))
			require.NoError(t, store.Save(t.Context(), schedule))
			publisher := NewMockScheduledSemanticCommandPublisher(t)
			publisher.EXPECT().PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.Anything).
				Return(DispatchReference{MessageID: fake.UUID().V4()}, nil).Maybe()
			return store, NewFXRefreshScheduleService(
				store,
				WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
				WithFXRefreshScheduleServicePublisher(publisher),
			), schedule, publisher
		}

		for _, testCase := range []struct {
			name   string
			mutate func(domain.FXRefreshSchedule, time.Time) domain.FXRefreshSchedule
		}{
			{
				name: "pause",
				mutate: func(schedule domain.FXRefreshSchedule, _ time.Time) domain.FXRefreshSchedule {
					schedule.Enabled = false
					return schedule
				},
			},
			{
				name: "reschedule",
				mutate: func(schedule domain.FXRefreshSchedule, now time.Time) domain.FXRefreshSchedule {
					nextRunAt := now.Add(time.Hour)
					schedule.NextRunAt = &nextRunAt
					return schedule
				},
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				store, service, schedule, publisher := makeService(t)
				candidates, err := store.ListDue(t.Context(), schedule.UpdatedAt)
				require.NoError(t, err)
				require.Len(t, candidates, 1)
				releaseScheduler := make(chan struct{})
				done := make(chan error, 1)
				go func() {
					<-releaseScheduler
					_, enqueueErr := service.enqueueOccurrence(t.Context(), candidates[0], schedule.UpdatedAt)
					done <- enqueueErr
				}()

				updated := testCase.mutate(schedule, schedule.UpdatedAt)
				updated.UpdatedAt = schedule.UpdatedAt.Add(time.Minute)
				require.NoError(t, store.Save(t.Context(), updated))
				close(releaseScheduler)
				require.NoError(t, <-done)
				publisher.AssertNotCalled(
					t, "PublishScheduledSemanticCommand", mock.Anything, mock.Anything, mock.Anything,
				)
				actual, err := store.Get(t.Context(), schedule.ScheduleID)
				require.NoError(t, err)
				assert.Equal(t, updated.Enabled, actual.Enabled)
				require.NotNil(t, updated.NextRunAt)
				require.NotNil(t, actual.NextRunAt)
				assert.True(t, actual.NextRunAt.Equal(*updated.NextRunAt))
			})
		}
	})

	t.Run("two schedulers claim one occurrence", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewFXRefreshScheduleStore(database)
		now := time.Now()
		schedule := makeSchedule(now, now.Add(-time.Hour))
		require.NoError(t, store.Save(t.Context(), schedule))
		publisher := NewMockScheduledSemanticCommandPublisher(t)
		var published atomic.Int32
		publisher.EXPECT().PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.Anything).
			Run(func(context.Context, *sql.Tx, SemanticCommand) { published.Add(1) }).
			Return(DispatchReference{MessageID: fake.UUID().V4()}, nil).Maybe()
		first := NewFXRefreshScheduleService(
			store,
			WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
			WithFXRefreshScheduleServicePublisher(publisher),
		)
		second := NewFXRefreshScheduleService(
			store,
			WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
			WithFXRefreshScheduleServicePublisher(publisher),
		)
		candidates, err := store.ListDue(t.Context(), now)
		require.NoError(t, err)
		require.Len(t, candidates, 1)
		start := make(chan struct{})
		var group sync.WaitGroup
		errs := make(chan error, 2)
		for _, service := range []*FXRefreshScheduleService{first, second} {
			group.Go(func() {
				<-start
				_, enqueueErr := service.enqueueOccurrence(t.Context(), candidates[0], now)
				errs <- enqueueErr
			})
		}
		close(start)
		group.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		assert.Equal(t, int32(1), published.Load())
	})
}
