package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFXRefreshScheduleStore(t *testing.T) {
	fake := faker.New()

	makeSchedule := func(now time.Time, nextRunAt *time.Time, enabled bool) domain.FXRefreshSchedule {
		return domain.FXRefreshSchedule{
			ScheduleID: fake.UUID().V4(), Provider: "provider-" + fake.Letter(),
			Interval: time.Hour, NextRunAt: nextRunAt, Enabled: enabled, CreatedAt: now, UpdatedAt: now,
		}
	}

	t.Run("lists only enabled due schedules in occurrence order", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewFXRefreshScheduleStore(database)
		now := time.Now()
		earlier := now.Add(-2 * time.Hour)
		later := now.Add(-time.Hour)
		future := now.Add(time.Hour)

		expected := makeSchedule(now, &earlier, true)
		require.NoError(t, store.Save(t.Context(), expected))
		require.NoError(t, store.Save(t.Context(), makeSchedule(now, &later, true)))
		require.NoError(t, store.Save(t.Context(), makeSchedule(now, &future, true)))
		require.NoError(t, store.Save(t.Context(), makeSchedule(now, &earlier, false)))
		require.NoError(t, store.Save(t.Context(), makeSchedule(now, nil, true)))

		actual, err := store.ListDue(t.Context(), now)

		require.NoError(t, err)
		require.Len(t, actual, 2)
		assert.Equal(t, expected.ScheduleID, actual[0].ScheduleID)
		assert.True(t, later.Equal(*actual[1].NextRunAt))
	})

	t.Run("rolls back occurrence state changes", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewFXRefreshScheduleStore(database)
		now := time.Now()
		dueAt := now.Add(-time.Hour)
		nextRunAt := now.Add(time.Hour)
		schedule := makeSchedule(now, &dueAt, true)
		require.NoError(t, store.Save(t.Context(), schedule))

		err := store.WithTransaction(t.Context(), func(tx *FXRefreshScheduleTransaction) error {
			schedule.NextRunAt = &nextRunAt
			schedule.LastScheduledAt = &dueAt
			schedule.LastJobID = fake.UUID().V4()
			schedule.UpdatedAt = now
			require.NoError(t, tx.Save(t.Context(), schedule))
			return assert.AnError
		})

		require.ErrorIs(t, err, assert.AnError)
		actual, getErr := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, getErr)
		assert.True(t, dueAt.Equal(*actual.NextRunAt))
		assert.Nil(t, actual.LastScheduledAt)
		assert.Empty(t, actual.LastJobID)
	})

	t.Run("keeps committed occurrence state when ensuring the default schedule", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewFXRefreshScheduleStore(database)
		now := time.Now()
		nextRunAt := now.Add(time.Hour)
		lastScheduledAt := now.Add(-time.Hour)
		schedule := makeSchedule(now, &nextRunAt, true)
		schedule.LastScheduledAt = &lastScheduledAt
		schedule.LastJobID = fake.UUID().V4()
		require.NoError(t, store.Save(t.Context(), schedule))

		candidate := makeSchedule(now.Add(time.Hour), &now, true)
		candidate.ScheduleID = schedule.ScheduleID
		require.NoError(t, store.Ensure(t.Context(), candidate))

		actual, err := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, err)
		assert.True(t, actual.NextRunAt.Equal(nextRunAt))
		assert.True(t, actual.LastScheduledAt.Equal(lastScheduledAt))
		assert.Equal(t, schedule.LastJobID, actual.LastJobID)
	})

	t.Run("commits occurrence state changes and reports missing schedules", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewFXRefreshScheduleStore(database)
		now := time.Now()
		dueAt := now.Add(-time.Hour)
		nextRunAt := now.Add(time.Hour)
		schedule := makeSchedule(now, &dueAt, true)
		require.NoError(t, store.Save(t.Context(), schedule))

		require.NoError(t, store.WithTransaction(t.Context(), func(tx *FXRefreshScheduleTransaction) error {
			schedule.NextRunAt = &nextRunAt
			return tx.Save(t.Context(), schedule)
		}))
		actual, err := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, err)
		assert.True(t, actual.NextRunAt.Equal(nextRunAt))
		_, err = store.Get(t.Context(), fake.UUID().V4())
		require.ErrorIs(t, err, ErrFXRefreshScheduleNotFound)
	})

	t.Run("conditionally claims and finalizes only the expected due occurrence", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewFXRefreshScheduleStore(database)
		now := time.Now()
		dueAt := now.Add(-time.Hour)
		nextRunAt := now.Add(time.Hour)
		schedule := makeSchedule(now, &dueAt, true)
		require.NoError(t, store.Save(t.Context(), schedule))
		messageID := fake.UUID().V4()

		require.NoError(t, store.WithTransaction(t.Context(), func(tx *FXRefreshScheduleTransaction) error {
			current, err := tx.Get(t.Context(), schedule.ScheduleID)
			require.NoError(t, err)
			require.Equal(t, schedule.ScheduleID, current.ScheduleID)
			claimed, err := tx.ClaimDue(t.Context(), schedule.ScheduleID, dueAt.Add(-time.Hour), nextRunAt, now)
			require.NoError(t, err)
			assert.False(t, claimed)
			claimed, err = tx.ClaimDue(t.Context(), schedule.ScheduleID, dueAt, nextRunAt, now)
			require.NoError(t, err)
			require.True(t, claimed)
			return tx.FinalizeClaim(t.Context(), schedule.ScheduleID, dueAt, nextRunAt, messageID, now)
		}))

		actual, err := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, err)
		assert.True(t, actual.NextRunAt.Equal(nextRunAt))
		assert.True(t, actual.LastScheduledAt.Equal(dueAt))
		assert.Equal(t, messageID, actual.LastJobID)
	})

	t.Run("returns canceled claim and finalization errors", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewFXRefreshScheduleStore(database)
		now := time.Now()
		dueAt := now.Add(-time.Hour)
		nextRunAt := now.Add(time.Hour)
		schedule := makeSchedule(now, &dueAt, true)
		require.NoError(t, store.Save(t.Context(), schedule))

		err := store.WithTransaction(t.Context(), func(tx *FXRefreshScheduleTransaction) error {
			canceledContext, cancel := context.WithCancel(t.Context())
			cancel()
			_, claimErr := tx.ClaimDue(canceledContext, schedule.ScheduleID, dueAt, nextRunAt, now)
			require.Error(t, claimErr)
			claimed, claimErr := tx.ClaimDue(t.Context(), schedule.ScheduleID, dueAt, nextRunAt, now)
			require.NoError(t, claimErr)
			require.True(t, claimed)
			_, getErr := tx.Get(t.Context(), fake.UUID().V4())
			require.ErrorIs(t, getErr, ErrFXRefreshScheduleNotFound)
			finalizeErr := tx.FinalizeClaim(
				t.Context(), schedule.ScheduleID, dueAt, nextRunAt.Add(time.Hour), fake.UUID().V4(), now,
			)
			require.Error(t, finalizeErr)
			finalizeErr = tx.FinalizeClaim(
				canceledContext, schedule.ScheduleID, dueAt, nextRunAt, fake.UUID().V4(), now,
			)
			require.Error(t, finalizeErr)
			return assert.AnError
		})

		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("reports unavailable schedule persistence", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewFXRefreshScheduleStore(database)
		sqlDB, err := database.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		now := time.Now()
		dueAt := now.Add(-time.Hour)

		_, err = store.ListDue(t.Context(), now)
		require.Error(t, err)
		require.Error(t, store.Save(t.Context(), makeSchedule(now, &dueAt, true)))
		require.Error(t, store.Ensure(t.Context(), makeSchedule(now, &dueAt, true)))
		require.Error(t, store.WithTransaction(t.Context(), func(*FXRefreshScheduleTransaction) error {
			return nil
		}))
	})
}
