//go:build postgres_test

package jobs

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWorkerUnit(t *testing.T) {
	fake := faker.New()

	makeRouterFactory := func(t *testing.T) *appdispatch.RouterFactory {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		config := appdispatch.Config{
			DatabaseDSN:  dsn,
			TablePrefix:  "sumweave_",
			PollInterval: time.Millisecond,
		}
		publisher, err := appdispatch.NewPublisher(config, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		factory, err := appdispatch.NewRouterFactory(config, db, publisher, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		return factory
	}
	makeWorker := func(t *testing.T, store *mockworkerStore, config WorkerConfig) *Worker {
		t.Helper()
		worker, err := NewWorker(WorkerDeps{
			Store: store, Registry: NewRegistry(), RouterFactory: makeRouterFactory(t),
			Logger: slog.New(slog.DiscardHandler), Clock: func() time.Time { return time.Time{} },
			Config: config, WorkerID: fake.UUID().V4(),
		})
		require.NoError(t, err)
		return worker
	}

	t.Run("runs once until two idle polls after startup recovery", func(t *testing.T) {
		store := newMockworkerStore(t)
		var recoveries int
		store.EXPECT().
			RecoverStaleRunning(mock.Anything, mock.Anything, 10*time.Second, defaultWorkerMaxAttempts).
			Run(func(context.Context, time.Time, time.Duration, int) { recoveries++ }).
			Return(nil).
			Maybe()
		worker := makeWorker(t, store, WorkerConfig{PollInterval: time.Millisecond, StaleRunningAge: 10 * time.Second})
		require.NoError(t, worker.RunOnce(t.Context()))
		assert.True(t, worker.installed)
		assert.GreaterOrEqual(t, recoveries, 1)
	})

	t.Run("installs observed handlers once", func(t *testing.T) {
		worker := makeWorker(t, newMockworkerStore(t), WorkerConfig{})
		require.NoError(t, worker.installHandlers())
		require.NoError(t, worker.installHandlers())
		assert.True(t, worker.installed)
		require.NoError(t, worker.Stop(t.Context()))
	})

	t.Run("periodically renews claims before recovering stale jobs", func(t *testing.T) {
		store := newMockworkerStore(t)
		now := time.Now()
		claim := Job{ID: fake.UUID().V4()}
		recovered := make(chan struct{}, 1)
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		store.EXPECT().RenewRunning(mock.Anything, claim, now).Return(nil).Once()
		store.EXPECT().
			RecoverStaleRunning(mock.Anything, now, time.Millisecond, defaultWorkerMaxAttempts).
			Run(func(context.Context, time.Time, time.Duration, int) {
				recovered <- struct{}{}
				cancel()
			}).
			Return(nil).
			Once()
		worker := &Worker{
			store:  store,
			logger: slog.New(slog.DiscardHandler),
			clock:  func() time.Time { return now },
			config: WorkerConfig{
				PollInterval:    time.Millisecond,
				StaleRunningAge: time.Millisecond,
				MaxAttempts:     defaultWorkerMaxAttempts,
			},
			claims: map[string]Job{claim.ID: claim},
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			worker.recoverStaleRunningPeriodically(ctx, nil)
		}()
		<-recovered
		<-done
	})

	t.Run("skips recovery when renewal callback cancels context", func(t *testing.T) {
		store := newMockworkerStore(t)
		now := time.Now()
		claim := Job{ID: fake.UUID().V4()}
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		store.EXPECT().RenewRunning(mock.Anything, claim, now).
			Run(func(context.Context, Job, time.Time) { cancel() }).
			Return(nil).
			Once()
		worker := &Worker{
			store:  store,
			logger: slog.New(slog.DiscardHandler),
			clock:  func() time.Time { return now },
			config: WorkerConfig{MaxAttempts: defaultWorkerMaxAttempts},
			claims: map[string]Job{claim.ID: claim},
		}

		assert.False(t, worker.runPeriodicRecoveryCycle(ctx))
	})

	t.Run("does not start a periodic recovery cycle after cancellation", func(t *testing.T) {
		store := newMockworkerStore(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		claim := Job{ID: fake.UUID().V4()}
		worker := &Worker{
			store:  store,
			logger: slog.New(slog.DiscardHandler),
			config: WorkerConfig{
				MaxAttempts: defaultWorkerMaxAttempts,
			},
			claims: map[string]Job{claim.ID: claim},
		}

		assert.False(t, worker.runPeriodicRecoveryCycle(ctx))
	})

	t.Run("finalizes exhausted retries through the store", func(t *testing.T) {
		store := newMockworkerStore(t)
		jobID := fake.UUID().V4()
		now := time.Now()
		state := newFailedTerminalJobState(fake.UUID().V4(), &JobError{Code: fake.Lorem().Word()}, now)
		store.EXPECT().FinalizeRetryExhausted(mock.Anything, jobID, now, state).Return(nil).Once()
		worker := &Worker{store: store, logger: slog.New(slog.DiscardHandler)}
		require.NoError(t, worker.finalizeRetryExhausted(t.Context(), jobID, now, state))

		finalizeErr := errors.New(fake.UUID().V4())
		exhausted := exhaustedRetryError{err: finalizeErr, finalize: func() error { return finalizeErr }}
		require.ErrorIs(t, exhausted, finalizeErr)
		require.ErrorIs(t, exhausted.OnRetriesExhausted(), finalizeErr)
	})

	t.Run("retries terminal persistence until success and stops for terminal outcomes", func(t *testing.T) {
		jobID := fake.UUID().V4()
		now := time.Now()
		state := newSucceededTerminalJobState(fake.UUID().V4(), now)

		t.Run("retries transient persistence", func(t *testing.T) {
			store := newMockworkerStore(t)
			transientErr := errors.New(fake.UUID().V4())
			store.EXPECT().persistTerminalState(mock.Anything, Job{ID: jobID}, state).Return(transientErr).Once()
			store.EXPECT().persistTerminalState(mock.Anything, Job{ID: jobID}, state).Return(nil).Once()
			worker := &Worker{store: store, logger: slog.New(slog.DiscardHandler)}
			require.NoError(t, worker.persistTerminalState(t.Context(), Job{ID: jobID}, state))
		})

		t.Run("does not retry a lost claim", func(t *testing.T) {
			store := newMockworkerStore(t)
			store.EXPECT().persistTerminalState(mock.Anything, Job{ID: jobID}, state).Return(ErrJobClaimLost).Once()
			worker := &Worker{store: store, logger: slog.New(slog.DiscardHandler)}
			require.ErrorIs(t, worker.persistTerminalState(t.Context(), Job{ID: jobID}, state), ErrJobClaimLost)
		})

		t.Run("returns cancellation with the final persistence error", func(t *testing.T) {
			store := newMockworkerStore(t)
			persistErr := errors.New(fake.UUID().V4())
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			store.EXPECT().
				persistTerminalState(mock.Anything, Job{ID: jobID}, state).
				Run(func(context.Context, Job, terminalJobState) { cancel() }).
				Return(persistErr).
				Once()
			worker := &Worker{store: store, logger: slog.New(slog.DiscardHandler)}
			err := worker.persistTerminalState(ctx, Job{ID: jobID}, state)
			require.ErrorIs(t, err, context.Canceled)
			require.ErrorIs(t, err, persistErr)
		})
	})
}
