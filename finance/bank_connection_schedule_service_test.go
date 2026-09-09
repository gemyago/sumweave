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
	makeConnection := func(now time.Time, state domain.BankConnectionState) domain.BankConnection {
		return domain.BankConnection{
			ID:        fake.UUID().V4(),
			TenantID:  fake.UUID().V4(),
			Provider:  fake.Lorem().Word(),
			SecretID:  fake.UUID().V4(),
			State:     state,
			CreatedAt: now,
			UpdatedAt: now,
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
					command.IdempotencyKey ==
						bankConnectionScheduleOccurrenceKey(schedule.ConnectionID, *input.ScheduledAt) &&
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

	t.Run("repairs active connections missing schedules without changing existing state", func(t *testing.T) {
		assertSchedule := func(expected, actual domain.BankConnectionSchedule) {
			t.Helper()
			assert.Equal(t, expected.ConnectionID, actual.ConnectionID)
			assert.Equal(t, expected.Interval, actual.Interval)
			assert.Equal(t, expected.LastJobID, actual.LastJobID)
			assert.Equal(t, expected.Enabled, actual.Enabled)
			for _, timestamp := range []struct {
				expected *time.Time
				actual   *time.Time
			}{
				{expected: expected.NextRunAt, actual: actual.NextRunAt},
				{expected: expected.LastScheduledAt, actual: actual.LastScheduledAt},
				{expected: expected.LastStartedAt, actual: actual.LastStartedAt},
				{expected: expected.LastCompletedAt, actual: actual.LastCompletedAt},
			} {
				if timestamp.expected == nil {
					assert.Nil(t, timestamp.actual)
					continue
				}
				require.NotNil(t, timestamp.actual)
				assert.True(t, timestamp.actual.Equal(*timestamp.expected))
			}
			assert.True(t, actual.CreatedAt.Equal(expected.CreatedAt))
			assert.True(t, actual.UpdatedAt.Equal(expected.UpdatedAt))
		}

		database := openTestDatabase(t)
		scheduleStore := persistence.NewBankConnectionScheduleStore(database)
		connectionStore := persistence.NewStore(database)
		now := time.Date(2000, time.January, 2, 0, 0, 0, 0, time.UTC)
		activeMissing := makeConnection(now, domain.BankConnectionStateActive)
		inactiveMissing := makeConnection(now, domain.BankConnectionStateDisconnected)
		activeExisting := makeConnection(now, domain.BankConnectionStateActive)
		for _, connection := range []domain.BankConnection{activeMissing, inactiveMissing, activeExisting} {
			_, err := connectionStore.SaveBankConnection(t.Context(), connection)
			require.NoError(t, err)
		}
		existingNextRunAt := now.Add(-time.Hour)
		existingLastScheduledAt := now.Add(-2 * time.Hour)
		existingLastStartedAt := now.Add(-90 * time.Minute)
		existingLastCompletedAt := now.Add(-30 * time.Minute)
		existing := domain.BankConnectionSchedule{
			ConnectionID: activeExisting.ID, Interval: time.Hour, NextRunAt: &existingNextRunAt,
			LastScheduledAt: &existingLastScheduledAt, LastStartedAt: &existingLastStartedAt,
			LastCompletedAt: &existingLastCompletedAt, LastJobID: fake.UUID().V4(), Enabled: false,
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute),
		}
		require.NoError(t, scheduleStore.Save(t.Context(), existing))
		publisher := NewMockScheduledSemanticCommandPublisher(t)
		publisher.EXPECT().PublishScheduledSemanticCommand(mock.Anything, mock.Anything, mock.Anything).
			Return(DispatchReference{MessageID: fake.UUID().V4()}, nil).Maybe()
		service := NewBankConnectionScheduleService(
			scheduleStore,
			WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
			WithBankConnectionScheduleServicePublisher(publisher),
		)

		_, err := service.EnqueueDue(t.Context())

		require.NoError(t, err)
		repaired, err := scheduleStore.Get(t.Context(), activeMissing.ID)
		require.NoError(t, err)
		expectedNextRunAt := now.Add(defaultBankConnectionScheduleInterval)
		assertSchedule(domain.BankConnectionSchedule{
			ConnectionID: activeMissing.ID, Interval: defaultBankConnectionScheduleInterval,
			NextRunAt: &expectedNextRunAt, Enabled: true, CreatedAt: now, UpdatedAt: now,
		}, *repaired)
		_, err = scheduleStore.Get(t.Context(), inactiveMissing.ID)
		require.ErrorIs(t, err, persistence.ErrBankConnectionScheduleNotFound)
		actualExisting, err := scheduleStore.Get(t.Context(), activeExisting.ID)
		require.NoError(t, err)
		assertSchedule(existing, *actualExisting)
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
