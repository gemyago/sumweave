//go:build postgres_test

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

func TestBankConnectionScheduleService(t *testing.T) {
	fake := faker.New()
	saveSchedule := func(t *testing.T, store *persistence.BankConnectionScheduleStore, schedule domain.BankConnectionSchedule) {
		t.Helper()
		require.NoError(t, store.Save(t.Context(), schedule))
		t.Cleanup(func() {
			actual, err := store.Get(context.WithoutCancel(t.Context()), schedule.ConnectionID)
			require.NoError(t, err)
			actual.Enabled = false
			require.NoError(t, store.Save(context.WithoutCancel(t.Context()), *actual))
		})
	}

	makeSchedule := func(now, dueAt time.Time) domain.BankConnectionSchedule {
		return domain.BankConnectionSchedule{
			ConnectionID: fake.UUID().V4(), Interval: time.Hour, NextRunAt: &dueAt,
			Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
	}

	t.Run("publishes and advances one due occurrence only once", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewBankConnectionScheduleStore(database)
		now := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
		dueAt := now.Add(-time.Hour)
		schedule := makeSchedule(now, dueAt)
		saveSchedule(t, store, schedule)
		publisher := NewMockScheduledSemanticCommandPublisher(t)
		messageID := fake.UUID().V4()
		publisher.EXPECT().
			PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.MatchedBy(func(command SemanticCommand) bool {
				var input BankConnectionSyncCommand
				return command.Topic == BankConnectionSyncCommandTopic &&
					assert.NoError(t, json.Unmarshal(command.Payload, &input)) &&
					input.ConnectionID == schedule.ConnectionID && input.Reason == BankConnectionSyncReasonScheduled &&
					input.ScheduledAt != nil && input.ScheduledAt.Equal(dueAt) &&
					command.IdempotencyKey == bankConnectionScheduleOccurrenceKey(schedule.ConnectionID, *input.ScheduledAt) &&
					input.ScheduledNextRunAt != nil && input.ScheduledNextRunAt.After(now)
			})).
			Return(DispatchReference{MessageID: messageID}, nil).
			Once()
		service := NewBankConnectionScheduleService(
			store,
			WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
			WithBankConnectionScheduleServicePublisher(publisher),
		)

		first, err := service.EnqueueDue(t.Context())
		require.NoError(t, err)
		second, err := service.EnqueueDue(t.Context())
		require.NoError(t, err)

		assert.Equal(t, 1, first)
		assert.Zero(t, second)
		actual, err := store.Get(t.Context(), schedule.ConnectionID)
		require.NoError(t, err)
		assert.True(t, actual.LastScheduledAt.Equal(dueAt))
		assert.True(t, actual.NextRunAt.After(now))
		assert.Equal(t, messageID, actual.LastJobID)
	})

	t.Run("rejects missing publisher and invalid intervals", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewBankConnectionScheduleStore(database)
		now := time.Date(2000, time.January, 2, 0, 0, 0, 0, time.UTC)
		dueAt := now.Add(-time.Hour)
		firstSchedule := makeSchedule(now, dueAt)
		saveSchedule(t, store, firstSchedule)
		service := NewBankConnectionScheduleService(store)
		_, err := service.EnqueueDue(t.Context())
		require.ErrorContains(t, err, "publisher is required")
		firstSchedule.Enabled = false
		require.NoError(t, store.Save(t.Context(), firstSchedule))

		invalid := makeSchedule(now, dueAt)
		invalid.Interval = 0
		saveSchedule(t, store, invalid)
		publisher := NewMockScheduledSemanticCommandPublisher(t)
		service = NewBankConnectionScheduleService(
			store,
			WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
			WithBankConnectionScheduleServicePublisher(publisher),
		)
		_, err = service.EnqueueDue(t.Context())
		require.ErrorContains(t, err, "interval must be positive")
		invalid.Enabled = false
		require.NoError(t, store.Save(t.Context(), invalid))
	})

	t.Run("rolls back on publication errors and empty references", func(t *testing.T) {
		makeService := func(t *testing.T, reference DispatchReference, publishErr error) (*persistence.BankConnectionScheduleStore, *BankConnectionScheduleService, domain.BankConnectionSchedule) {
			t.Helper()
			database := openTestDatabase(t)
			store := persistence.NewBankConnectionScheduleStore(database)
			now := time.Date(2000, time.January, 3, 0, 0, 0, 0, time.UTC)
			schedule := makeSchedule(now, now.Add(-time.Hour))
			saveSchedule(t, store, schedule)
			publisher := NewMockScheduledSemanticCommandPublisher(t)
			publisher.EXPECT().PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.Anything).
				Return(reference, publishErr).Once()
			return store, NewBankConnectionScheduleService(
				store,
				WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
				WithBankConnectionScheduleServicePublisher(publisher),
			), schedule
		}

		for _, testCase := range []struct {
			name      string
			reference DispatchReference
			err       error
		}{
			{name: "publication failure", err: assert.AnError},
			{name: "empty reference"},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				store, service, schedule := makeService(t, testCase.reference, testCase.err)
				_, err := service.EnqueueDue(t.Context())
				require.Error(t, err)
				actual, getErr := store.Get(t.Context(), schedule.ConnectionID)
				require.NoError(t, getErr)
				assert.True(t, actual.NextRunAt.Equal(*schedule.NextRunAt))
				assert.Empty(t, actual.LastJobID)
				schedule.Enabled = false
				require.NoError(t, store.Save(t.Context(), schedule))
			})
		}

		assert.Panics(t, func() { NewBankConnectionScheduleService(nil) })
	})

	t.Run("does not publish a stale occurrence after concurrent pause or reschedule", func(t *testing.T) {
		makeService := func(t *testing.T) (*persistence.BankConnectionScheduleStore, *BankConnectionScheduleService, domain.BankConnectionSchedule, *MockScheduledSemanticCommandPublisher) {
			t.Helper()
			database := openTestDatabase(t)
			store := persistence.NewBankConnectionScheduleStore(database)
			now := time.Date(2000, time.January, 4, 0, 0, 0, 0, time.UTC)
			schedule := makeSchedule(now, now.Add(-time.Hour))
			saveSchedule(t, store, schedule)
			publisher := NewMockScheduledSemanticCommandPublisher(t)
			publisher.EXPECT().PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.Anything).
				Return(DispatchReference{MessageID: fake.UUID().V4()}, nil).Maybe()
			return store, NewBankConnectionScheduleService(
				store,
				WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
				WithBankConnectionScheduleServicePublisher(publisher),
			), schedule, publisher
		}

		for _, testCase := range []struct {
			name   string
			mutate func(domain.BankConnectionSchedule, time.Time) domain.BankConnectionSchedule
		}{
			{
				name: "pause",
				mutate: func(schedule domain.BankConnectionSchedule, _ time.Time) domain.BankConnectionSchedule {
					schedule.Enabled = false
					return schedule
				},
			},
			{
				name: "reschedule",
				mutate: func(schedule domain.BankConnectionSchedule, now time.Time) domain.BankConnectionSchedule {
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
				var candidate domain.BankConnectionSchedule
				for _, item := range candidates {
					if item.ConnectionID == schedule.ConnectionID {
						candidate = item
					}
				}
				require.Equal(t, schedule.ConnectionID, candidate.ConnectionID)
				releaseScheduler := make(chan struct{})
				done := make(chan error, 1)
				go func() {
					<-releaseScheduler
					_, enqueueErr := service.enqueueOccurrence(t.Context(), candidate, schedule.UpdatedAt)
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
				actual, err := store.Get(t.Context(), schedule.ConnectionID)
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
		store := persistence.NewBankConnectionScheduleStore(database)
		now := time.Date(2000, time.January, 5, 0, 0, 0, 0, time.UTC)
		schedule := makeSchedule(now, now.Add(-time.Hour))
		saveSchedule(t, store, schedule)
		publisher := NewMockScheduledSemanticCommandPublisher(t)
		var published atomic.Int32
		publisher.EXPECT().PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.Anything).
			Run(func(context.Context, *sql.Tx, SemanticCommand) { published.Add(1) }).
			Return(DispatchReference{MessageID: fake.UUID().V4()}, nil).Maybe()
		first := NewBankConnectionScheduleService(
			store,
			WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
			WithBankConnectionScheduleServicePublisher(publisher),
		)
		second := NewBankConnectionScheduleService(
			store,
			WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
			WithBankConnectionScheduleServicePublisher(publisher),
		)
		candidates, err := store.ListDue(t.Context(), now)
		require.NoError(t, err)
		var candidate domain.BankConnectionSchedule
		for _, item := range candidates {
			if item.ConnectionID == schedule.ConnectionID {
				candidate = item
			}
		}
		require.Equal(t, schedule.ConnectionID, candidate.ConnectionID)
		start := make(chan struct{})
		var group sync.WaitGroup
		errs := make(chan error, 2)
		for _, service := range []*BankConnectionScheduleService{first, second} {
			group.Go(func() {
				<-start
				_, enqueueErr := service.enqueueOccurrence(t.Context(), candidate, now)
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
