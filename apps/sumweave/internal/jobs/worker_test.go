package jobs

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWorker(t *testing.T) {
	fake := faker.New()
	_, workerErr := NewWorker(WorkerDeps{})
	require.Error(t, workerErr)

	makeRouterFactory := func(t *testing.T) *appdispatch.RouterFactory {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, openErr := sql.Open("pgx", dsn)
		require.NoError(t, openErr)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		config := appdispatch.Config{
			DatabaseDSN:  dsn,
			TablePrefix:  "sumweave_",
			PollInterval: time.Millisecond,
		}
		publisher, publisherErr := appdispatch.NewPublisher(config, db, slog.New(slog.DiscardHandler))
		require.NoError(t, publisherErr)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		factory, factoryErr := appdispatch.NewRouterFactory(config, db, publisher, slog.New(slog.DiscardHandler))
		require.NoError(t, factoryErr)
		return factory
	}
	makeWorker := func(t *testing.T, store *mockworkerStore, config WorkerConfig) *Worker {
		t.Helper()
		worker, newWorkerErr := NewWorker(WorkerDeps{
			Store: store, Registry: NewRegistry(), RouterFactory: makeRouterFactory(t),
			Logger: slog.New(slog.DiscardHandler), Clock: func() time.Time { return time.Time{} },
			Config: config, WorkerID: fake.UUID().V4(),
		})
		require.NoError(t, newWorkerErr)
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

	t.Run("returns startup recovery errors", func(t *testing.T) {
		store := newMockworkerStore(t)
		recoveryErr := errors.New(fake.UUID().V4())
		store.EXPECT().
			RecoverStaleRunning(mock.Anything, time.Time{}, time.Second, defaultWorkerMaxAttempts).
			Return(recoveryErr).
			Once()
		worker := makeWorker(t, store, WorkerConfig{StaleRunningAge: time.Second})
		require.ErrorIs(t, worker.Run(t.Context()), recoveryErr)
		require.NoError(t, worker.Stop(t.Context()))
	})

	t.Run("accepts cancellation before one-shot routing starts", func(t *testing.T) {
		store := newMockworkerStore(t)
		store.EXPECT().
			RecoverStaleRunning(mock.Anything, time.Time{}, time.Second, defaultWorkerMaxAttempts).
			Return(nil).
			Once()
		worker := makeWorker(t, store, WorkerConfig{
			PollInterval: time.Hour, StaleRunningAge: time.Second,
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.NoError(t, worker.RunOnce(ctx))
		require.NoError(t, worker.Stop(t.Context()))
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
		require.ErrorContains(t, exhausted, finalizeErr.Error())
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
			persistStateErr := worker.persistTerminalState(ctx, Job{ID: jobID}, state)
			require.ErrorIs(t, persistStateErr, context.Canceled)
			require.ErrorIs(t, persistStateErr, persistErr)
		})
	})

	t.Run("handles claim renewal and invalid terminal state", func(t *testing.T) {
		store := newMockworkerStore(t)
		job := Job{ID: fake.UUID().V4()}
		store.EXPECT().RenewRunning(mock.Anything, job, mock.Anything).Return(errors.New(fake.UUID().V4())).Once()
		worker := &Worker{
			store: store, logger: slog.New(slog.DiscardHandler), clock: time.Now,
			claims: map[string]Job{job.ID: job},
		}
		worker.renewRunningClaims(t.Context())
		require.Error(t, worker.persistTerminalState(t.Context(), job, terminalJobState{}))
		unexpected := errors.New(fake.UUID().V4())
		require.ErrorIs(t, worker.runOnceResult(unexpected), unexpected)
	})

	t.Run("returns transient requeue failures", func(t *testing.T) {
		store := newMockworkerStore(t)
		registry := NewRegistry()
		jobType := JobType("finance." + fake.Letter())
		topic := "worker." + fake.UUID().V4()
		requester := Requester{UserID: fake.UUID().V4(), Source: RequesterSourceOperator}
		retryErr := errors.New(fake.UUID().V4())
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[struct{}]{
			JobType: jobType,
			Topic:   topic,
			Metadata: func(struct{}) (JobMetadata, error) {
				return JobMetadata{JobType: jobType, Requester: requester}, nil
			},
			Run: func(context.Context, Job, struct{}) error { return retryErr },
		}))
		now := time.Now()
		workerID := fake.UUID().V4()
		worker := &Worker{
			store: store, registry: registry, logger: slog.New(slog.DiscardHandler),
			clock: func() time.Time { return now }, workerID: workerID,
		}
		message := appdispatch.Message{ID: fake.UUID().V4(), Payload: []byte(`{}`)}
		queued := Job{ID: message.ID, JobType: jobType, Status: JobStatusQueued}
		claimed := queued
		claimed.Status = JobStatusRunning
		store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
		store.EXPECT().ClaimQueued(mock.Anything, message.ID, workerID, now).Return(&claimed, nil).Once()
		store.EXPECT().RequeueRunning(mock.Anything, claimed, now).Return(retryErr).Once()

		require.ErrorIs(t, worker.processObserved(t.Context(), registry.Handlers()[0], message), retryErr)
	})
}
