//go:build postgres_test

package jobs

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStore(t *testing.T) {
	fake := faker.New()

	t.Run("rejects invalid PostgreSQL store inputs", func(t *testing.T) {
		_, err := NewStore(nil, "", StoreOpts{})
		require.ErrorContains(t, err, "sql database is required")
		_, err = NewStore(&sql.DB{}, fake.UUID().V4(), StoreOpts{})
		require.ErrorContains(t, err, "parse jobs database dsn")
	})

	makeStore := func(t *testing.T) *Store {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := NewStore(db, dsn, StoreOpts{TablePrefix: "sumweave_jobs_"})
		require.NoError(t, err)
		return store
	}
	makeQueuedJob := func(now time.Time) Job {
		return Job{
			ID: fake.UUID().V4(), JobType: JobType("finance." + fake.Letter()),
			Status: JobStatusQueued,
			Requester: Requester{
				UserID: fake.UUID().V4(), Source: RequesterSourceOperator,
			},
			CreatedAt: now, UpdatedAt: now, QueuedAt: now,
		}
	}

	t.Run("persists terminal states and retry exhaustion", func(t *testing.T) {
		store := makeStore(t)
		now := time.Now()
		queued := makeQueuedJob(now)
		_, err := store.MaterializeQueued(t.Context(), queued)
		require.NoError(t, err)
		claimed, err := store.ClaimQueued(t.Context(), queued.ID, fake.UUID().V4(), now.Add(time.Second))
		require.NoError(t, err)

		completedAt := now.Add(2 * time.Second)
		expectedError := &JobError{
			Code: fake.Lorem().Word(), Summary: fake.Lorem().Sentence(3), Details: fake.Lorem().Sentence(5),
		}
		require.NoError(t, store.persistTerminalState(
			t.Context(), *claimed, newFailedTerminalJobState(claimed.WorkerID, expectedError, completedAt),
		))
		persisted, err := store.Get(t.Context(), queued.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, persisted.Status)
		assert.Equal(t, claimed.WorkerID, persisted.WorkerID)
		assert.Equal(t, expectedError, persisted.Error)
		require.NotNil(t, persisted.CompletedAt)
		assert.True(t, completedAt.Equal(*persisted.CompletedAt))

		retryQueuedAt := now.Add(3 * time.Second)
		retryQueued := makeQueuedJob(retryQueuedAt)
		_, err = store.MaterializeQueued(t.Context(), retryQueued)
		require.NoError(t, err)
		retryCompletedAt := now.Add(4 * time.Second)
		require.NoError(t, store.FinalizeRetryExhausted(
			t.Context(),
			retryQueued.ID,
			retryQueuedAt,
			newFailedTerminalJobState(fake.UUID().V4(), nil, retryCompletedAt),
		))
		retryPersisted, err := store.Get(t.Context(), retryQueued.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, retryPersisted.Status)
		assert.Nil(t, retryPersisted.Error)
		require.NotNil(t, retryPersisted.CompletedAt)
		assert.True(t, retryCompletedAt.Equal(*retryPersisted.CompletedAt))
		require.ErrorIs(t, store.FinalizeRetryExhausted(
			t.Context(),
			retryQueued.ID,
			retryQueuedAt,
			newFailedTerminalJobState(fake.UUID().V4(), nil, now.Add(5*time.Second)),
		), ErrJobClaimLost)
	})

	t.Run("recovers stale running claims into queued and exhausted terminal states", func(t *testing.T) {
		store := makeStore(t)
		now := time.Now()
		staleAt := now.Add(-2 * time.Minute)
		workerID := fake.UUID().V4()
		makeStale := func(attemptCount int) Job {
			job := makeQueuedJob(staleAt)
			job.AttemptCount = attemptCount - 1
			_, err := store.MaterializeQueued(t.Context(), job)
			require.NoError(t, err)
			claimed, claimErr := store.ClaimQueued(t.Context(), job.ID, workerID, staleAt)
			require.NoError(t, claimErr)
			return *claimed
		}
		requeued := makeStale(defaultWorkerMaxAttempts - 1)
		exhausted := makeStale(defaultWorkerMaxAttempts)

		require.NoError(t, store.RecoverStaleRunning(t.Context(), now, time.Minute, defaultWorkerMaxAttempts))
		recovered, err := store.Get(t.Context(), requeued.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusQueued, recovered.Status)
		assert.Empty(t, recovered.WorkerID)
		assert.Nil(t, recovered.StartedAt)
		assert.Equal(t, "stale_running_requeued", recovered.Error.Code)

		exhaustedRecovered, err := store.Get(t.Context(), exhausted.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, exhaustedRecovered.Status)
		assert.Equal(t, "stale_running_attempts_exhausted", exhaustedRecovered.Error.Code)
		require.NotNil(t, exhaustedRecovered.CompletedAt)
		assert.True(t, now.Equal(*exhaustedRecovered.CompletedAt))
	})

	t.Run("maps persisted models without losing safe job fields", func(t *testing.T) {
		now := time.Now()
		startedAt := now.Add(time.Second)
		completedAt := now.Add(2 * time.Second)
		lastAttemptAt := now.Add(3 * time.Second)
		scheduledAt := now.Add(4 * time.Second)
		nextRunAt := now.Add(5 * time.Second)
		input := Job{
			ID: " " + fake.UUID().V4() + " ", JobType: JobType(fake.Lorem().Word()), Status: JobStatusFailed,
			Requester: Requester{UserID: " " + fake.UUID().V4() + " ", Source: RequesterSource(" operator ")},
			Error:     &JobError{Code: " " + fake.Lorem().Word() + " ", Summary: " " + fake.Lorem().Sentence(3) + " ", Details: " " + fake.Lorem().Sentence(5) + " "},
			CreatedAt: now, UpdatedAt: now, QueuedAt: now, StartedAt: &startedAt, CompletedAt: &completedAt,
			WorkerID: " " + fake.UUID().V4() + " ", AttemptCount: fake.IntBetween(1, 10), LastAttemptAt: &lastAttemptAt,
			ScheduleID: " " + fake.UUID().V4() + " ", ScheduledAt: &scheduledAt, ScheduledNextRunAt: &nextRunAt,
		}

		model := newJobModel(input)
		assert.Equal(t, input.ID[1:len(input.ID)-1], model.ID)
		assert.Equal(t, input.Requester.UserID[1:len(input.Requester.UserID)-1], model.RequesterUserID)
		assert.Equal(t, input.WorkerID[1:len(input.WorkerID)-1], model.WorkerID)
		assert.Equal(t, input.ScheduleID[1:len(input.ScheduleID)-1], model.ScheduleID)
		assert.Equal(t, input.Error.Code[1:len(input.Error.Code)-1], model.ErrorCode)

		expected := input
		expected.ID = model.ID
		expected.Requester.UserID = model.RequesterUserID
		expected.Requester.Source = RequesterSource(model.RequesterSource)
		expected.WorkerID = model.WorkerID
		expected.ScheduleID = model.ScheduleID
		expected.Error = &JobError{Code: model.ErrorCode, Summary: model.ErrorSummary, Details: model.ErrorDetails}
		assert.Equal(t, expected, jobFromModel(model))
	})

	t.Run("retains claim ownership and validates stored cursors", func(t *testing.T) {
		store := makeStore(t)
		now := time.Now()
		queued := makeQueuedJob(now)
		_, err := store.MaterializeQueued(t.Context(), queued)
		require.NoError(t, err)
		claimed, err := store.ClaimQueued(t.Context(), queued.ID, fake.UUID().V4(), now)
		require.NoError(t, err)
		require.NoError(t, store.RequeueRunning(t.Context(), *claimed, now.Add(time.Second)))
		require.ErrorIs(t, store.RequeueRunning(t.Context(), *claimed, now.Add(2*time.Second)), ErrJobClaimLost)
		require.ErrorIs(t, store.RenewRunning(t.Context(), *claimed, now.Add(2*time.Second)), ErrJobClaimLost)
		require.ErrorIs(t, store.persistTerminalState(
			t.Context(), *claimed, newSucceededTerminalJobState(claimed.WorkerID, now.Add(2*time.Second)),
		), ErrJobClaimLost)
		require.Error(t, store.RecoverStaleRunning(t.Context(), now, 0, defaultWorkerMaxAttempts))

		cursor := encodeCursor(now, fake.UUID().V4())
		cursorAt, cursorID, err := decodeCursor(cursor)
		require.NoError(t, err)
		assert.True(t, now.Equal(cursorAt))
		assert.NotEmpty(t, cursorID)
		_, _, err = decodeCursor(base64.RawURLEncoding.EncodeToString([]byte(fake.UUID().V4())))
		require.Error(t, err)
		_, _, err = decodeCursor(base64.RawURLEncoding.EncodeToString([]byte(fake.Lorem().Word() + "|" + fake.UUID().V4())))
		require.Error(t, err)
		_, err = store.List(t.Context(), ListParams{
			Sources: []RequesterSource{RequesterSourceOperator}, Cursor: fake.UUID().V4(),
		})
		require.ErrorContains(t, err, "decode cursor")
	})

	t.Run("returns storage errors without treating them as lifecycle states", func(t *testing.T) {
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		store, err := NewStore(db, dsn, StoreOpts{TablePrefix: "sumweave_jobs_"})
		require.NoError(t, err)
		require.NoError(t, db.Close())
		now := time.Now()
		job := makeQueuedJob(now)
		claim := job
		claim.Status = JobStatusRunning
		claim.StartedAt = &now
		terminal := newSucceededTerminalJobState(fake.UUID().V4(), now)

		require.Error(t, createWithDB(t.Context(), store.db, store.tableName, job))
		_, err = store.Get(t.Context(), job.ID)
		require.Error(t, err)
		_, err = store.MaterializeQueued(t.Context(), job)
		require.Error(t, err)
		_, err = store.List(t.Context(), ListParams{})
		require.Error(t, err)
		_, err = store.ClaimQueued(t.Context(), job.ID, fake.UUID().V4(), now)
		require.Error(t, err)
		require.Error(t, store.persistTerminalState(t.Context(), claim, terminal))
		require.Error(t, store.FinalizeRetryExhausted(t.Context(), job.ID, now, terminal))
		require.Error(t, store.RequeueRunning(t.Context(), claim, now))
		require.Error(t, store.RenewRunning(t.Context(), claim, now))
		require.Error(t, store.RecoverStaleRunning(t.Context(), now, time.Second, defaultWorkerMaxAttempts))
		require.Error(t, store.recoverStaleRunningModel(t.Context(), newJobModel(claim), now, defaultWorkerMaxAttempts))
	})

	t.Run("surfaces a stale recovery write failure", func(t *testing.T) {
		store := makeStore(t)
		now := time.Now()
		staleAt := now.Add(-2 * time.Minute)
		queued := makeQueuedJob(staleAt)
		_, err := store.MaterializeQueued(t.Context(), queued)
		require.NoError(t, err)
		_, err = store.ClaimQueued(t.Context(), queued.ID, fake.UUID().V4(), staleAt)
		require.NoError(t, err)
		callbackName := "jobs_recovery_" + strings.ReplaceAll(fake.UUID().V4(), "-", "")
		updateErr := errors.New(fake.UUID().V4())
		require.NoError(t, store.db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
			tx.AddError(updateErr)
		}))
		t.Cleanup(func() { require.NoError(t, store.db.Callback().Update().Remove(callbackName)) })
		require.ErrorIs(t, store.RecoverStaleRunning(t.Context(), now, time.Minute, defaultWorkerMaxAttempts), updateErr)
	})

	t.Run("keeps schema helpers safe for missing artifacts", func(t *testing.T) {
		store := makeStore(t)
		tableName := "jobs_store_" + strings.ReplaceAll(fake.UUID().V4(), "-", "")
		require.NoError(t, store.dropColumnIfExists(store.tableName, fake.Lorem().Word()))
		require.NoError(t, store.dropColumnIfExists(tableName, fake.Lorem().Word()))
		require.NoError(t, store.dropTableIfExists(tableName))
		assert.Equal(t, `"jobs""store"`, quoteIdentifier(`jobs"store`))
		require.Error(t, store.RenewRunning(t.Context(), Job{}, time.Time{}))
	})
}
