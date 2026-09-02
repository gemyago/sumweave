//go:build postgres_test

package jobs

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestObservedSubscriptions(t *testing.T) {
	fake := faker.New()
	type command struct {
		Value     string `json:"value"`
		Requester string `json:"requester"`
	}
	makeTransport := func(t *testing.T) (*Store, *appdispatch.RouterFactory, *appdispatch.Publisher, *sql.DB, appdispatch.Config) {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := NewStore(db, dsn, StoreOpts{TablePrefix: "sumweave_jobs_"})
		require.NoError(t, err)
		config := appdispatch.Config{
			DatabaseDSN:  dsn,
			TablePrefix:  "sumweave_",
			PollInterval: time.Millisecond,
		}
		publisher, err := appdispatch.NewPublisher(config, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		factory, err := appdispatch.NewRouterFactory(
			config,
			db,
			publisher,
			slog.New(slog.DiscardHandler),
		)
		require.NoError(t, err)
		return store, factory, publisher, db, config
	}
	register := func(t *testing.T, registry *Registry, topic string, run func(context.Context, Job, command) error) {
		t.Helper()
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[command]{
			JobType: "finance.test", Topic: topic,
			Metadata: func(value command) (JobMetadata, error) {
				return JobMetadata{
					JobType:   "finance.test",
					Requester: Requester{UserID: value.Requester, Source: RequesterSourceOperator},
				}, nil
			},
			Run: run,
		}))
	}

	t.Run("ordinary subscriptions execute with zero job rows", func(t *testing.T) {
		store, factory, publisher, _, _ := makeTransport(t)
		topic := "ordinary." + fake.UUID().V4()
		var calls atomic.Int32
		router, err := factory.NewRouter("ordinary." + fake.UUID().V4())
		require.NoError(t, err)
		handler, err := appdispatch.NewHandler(
			topic,
			func(_ context.Context, _ appdispatch.Message) error {
				calls.Add(1)
				return nil
			},
		)
		require.NoError(t, err)
		require.NoError(t, router.Handle(handler))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go func() { _ = router.Run(ctx) }()
		message := appdispatch.NewMessage(topic, []byte(fake.UUID().V4()))
		require.NoError(t, publisher.Publish(t.Context(), message))
		require.Eventually(
			t,
			func() bool { return calls.Load() == 1 },
			time.Second,
			time.Millisecond,
		)
		_, getErr := store.Get(t.Context(), message.ID)
		require.ErrorIs(t, getErr, ErrJobNotFound)
		require.NoError(t, router.Close())
	})

	t.Run(
		"first delivery claims message identity before domain work and skips terminal duplicates",
		func(t *testing.T) {
			store, factory, _, _, _ := makeTransport(t)
			registry := NewRegistry()
			topic := "observed." + fake.UUID().V4()
			message := appdispatch.NewMessage(
				topic,
				[]byte(`{"value":"`+fake.UUID().V4()+`","requester":"`+fake.UUID().V4()+`"}`),
			)
			var calls atomic.Int32
			register(t, registry, topic, func(ctx context.Context, job Job, value command) error {
				persisted, err := store.Get(ctx, job.ID)
				require.NoError(t, err)
				assert.Equal(t, JobStatusRunning, persisted.Status)
				assert.Equal(t, message.ID, job.ID)
				assert.Equal(t, value.Requester, job.Requester.UserID)
				calls.Add(1)
				return nil
			})
			worker, err := NewWorker(
				WorkerDeps{
					Store:         store,
					Registry:      registry,
					RouterFactory: factory,
					Logger:        slog.New(slog.DiscardHandler),
					Config:        WorkerConfig{PollInterval: time.Millisecond},
				},
			)
			require.NoError(t, err)
			require.NoError(t, worker.processObserved(t.Context(), registry.Handlers()[0], message))
			persisted, err := store.Get(t.Context(), message.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusSucceeded, persisted.Status)
			assert.Equal(t, int32(1), calls.Load())
			assert.Equal(t, message.ID, persisted.ID)
			require.NoError(t, worker.Stop(t.Context()))
			duplicateWorker := &Worker{
				store:    store,
				registry: registry,
				logger:   slog.New(slog.DiscardHandler),
				clock:    time.Now,
				workerID: fake.UUID().V4(),
			}
			require.NoError(
				t,
				duplicateWorker.processObserved(t.Context(), registry.Handlers()[0], message),
			)
			assert.Equal(t, int32(1), calls.Load())

			stale := Job{
				ID:           fake.UUID().V4(),
				JobType:      "finance.test",
				Status:       JobStatusRunning,
				AttemptCount: 1,
				Requester: Requester{
					Source: RequesterSourceOperator,
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				QueuedAt:  time.Now(),
			}
			staleStartedAt := time.Now().Add(-2 * defaultWorkerStaleRunningAge)
			stale.StartedAt = &staleStartedAt
			stale.UpdatedAt = staleStartedAt
			err = store.createWithDB(t.Context(), store.db, stale)
			require.NoError(t, err)
			require.NoError(
				t,
				store.RecoverStaleRunning(
					t.Context(),
					time.Now(),
					defaultWorkerStaleRunningAge,
					defaultWorkerMaxAttempts,
				),
			)
			recovered, err := store.Get(t.Context(), stale.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusQueued, recovered.Status)
		},
	)

	t.Run("one-shot worker waits for active delivery and persists exhausted retries", func(t *testing.T) {
		store, factory, _, _, _ := makeTransport(t)
		registry := NewRegistry()
		topic := "one-shot." + fake.UUID().V4()
		started := make(chan struct{})
		release := make(chan struct{})
		register(t, registry, topic, func(context.Context, Job, command) error {
			close(started)
			<-release
			return nil
		})
		worker, err := NewWorker(WorkerDeps{
			Store: store, Registry: registry, RouterFactory: factory,
			Logger: slog.New(slog.DiscardHandler), Config: WorkerConfig{PollInterval: time.Millisecond},
		})
		require.NoError(t, err)
		message := appdispatch.NewMessage(
			topic,
			[]byte(`{"value":"`+fake.UUID().V4()+`","requester":"`+fake.UUID().V4()+`"}`),
		)
		runDone := make(chan error, 1)
		go func() { runDone <- worker.processObserved(t.Context(), registry.Handlers()[0], message) }()
		<-started
		time.Sleep(3 * time.Millisecond)
		select {
		case resultErr := <-runDone:
			require.NoError(t, resultErr)
			t.Fatal("one-shot worker stopped while a handler was running")
		default:
		}
		close(release)
		require.NoError(t, <-runDone)
		persisted, err := store.Get(t.Context(), message.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusSucceeded, persisted.Status)

	})

	t.Run("one-shot retry lifecycle keeps final persistence and topics isolated", func(t *testing.T) {
		store := newMockworkerStore(t)
		registry := NewRegistry()
		topic := "retry-tracker." + fake.UUID().V4()
		register(t, registry, topic, func(context.Context, Job, command) error {
			return errors.New("transient " + fake.UUID().V4())
		})
		now := time.Now()
		worker := &Worker{
			store: store, registry: registry, logger: slog.New(slog.DiscardHandler),
			clock: func() time.Time { return now }, workerID: fake.UUID().V4(),
		}
		tracker := &runOnceTracker{}
		worker.runOnce = tracker
		message := appdispatch.Message{
			ID:      fake.UUID().V4(),
			Payload: []byte(`{"value":"` + fake.UUID().V4() + `","requester":"` + fake.UUID().V4() + `"}`),
		}
		queued := Job{ID: message.ID, JobType: "finance.test", Status: JobStatusQueued}
		claimed := queued
		claimed.Status = JobStatusRunning
		store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
		store.EXPECT().ClaimQueued(mock.Anything, message.ID, worker.workerID, now).Return(&claimed, nil).Once()
		store.EXPECT().RequeueRunning(mock.Anything, claimed, now).Return(nil).Once()
		persistStarted := make(chan struct{})
		releasePersist := make(chan struct{})
		store.EXPECT().
			FinalizeRetryExhausted(mock.Anything, message.ID, now, mock.Anything).
			Run(func(context.Context, string, time.Time, terminalJobState) {
				close(persistStarted)
				<-releasePersist
			}).
			Return(nil).
			Once()
		err := worker.processObserved(t.Context(), registry.Handlers()[0], message)
		var exhausted exhaustedRetryError
		require.ErrorAs(t, err, &exhausted)
		worker.startRunOnceRetry(message.ID)
		assert.False(t, tracker.isIdle())
		finalized := make(chan error, 1)
		go func() { finalized <- exhausted.OnRetriesExhausted() }()
		<-persistStarted
		assert.False(t, tracker.isIdle())
		close(releasePersist)
		require.NoError(t, <-finalized)
		worker.finishRunOnceRetry(message.ID)
		assert.True(t, tracker.isIdle())

		pendingJobID := fake.UUID().V4()
		otherJobID := fake.UUID().V4()
		tracker.startRetry(pendingJobID)
		tracker.startDelivery(otherJobID)
		tracker.finishDelivery(otherJobID)
		assert.False(t, tracker.isIdle())
		tracker.finishRetry(pendingJobID)
		assert.True(t, tracker.isIdle())
	})

	t.Run("periodic recovery invokes stale claim recovery", func(t *testing.T) {
		store := newMockworkerStore(t)
		recovered := make(chan struct{}, 1)
		store.EXPECT().
			RecoverStaleRunning(mock.Anything, mock.Anything, time.Millisecond, defaultWorkerMaxAttempts).
			Run(func(context.Context, time.Time, time.Duration, int) {
				select {
				case recovered <- struct{}{}:
				default:
				}
			}).
			Return(nil)
		worker := &Worker{
			store: store, logger: slog.New(slog.DiscardHandler), clock: time.Now,
			config: WorkerConfig{
				PollInterval: time.Millisecond, StaleRunningAge: time.Millisecond,
				MaxAttempts: defaultWorkerMaxAttempts,
			},
		}
		stop := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			worker.recoverStaleRunningPeriodically(t.Context(), stop)
		}()
		<-recovered
		close(stop)
		<-done
	})

	t.Run("second worker startup recovery does not requeue a live claim", func(t *testing.T) {
		store, _, _, _, _ := makeTransport(t)
		registry := NewRegistry()
		topic := "live-claim." + fake.UUID().V4()
		firstWorkerID := fake.UUID().V4()
		secondWorkerID := fake.UUID().V4()
		message := appdispatch.Message{
			ID:      fake.UUID().V4(),
			Payload: []byte(`{"value":"` + fake.UUID().V4() + `","requester":"` + fake.UUID().V4() + `"}`),
		}
		claimed := make(chan struct{})
		release := make(chan struct{})
		var calls atomic.Int32
		register(t, registry, topic, func(_ context.Context, job Job, _ command) error {
			calls.Add(1)
			if job.WorkerID == firstWorkerID {
				close(claimed)
				<-release
			}
			return nil
		})
		now := time.Now()
		var clockMu sync.Mutex
		clockNow := now
		clock := func() time.Time {
			clockMu.Lock()
			defer clockMu.Unlock()
			return clockNow
		}
		advanceClock := func(value time.Time) {
			clockMu.Lock()
			defer clockMu.Unlock()
			clockNow = value
		}
		firstWorker := &Worker{
			store: store, registry: registry, logger: slog.New(slog.DiscardHandler),
			clock: clock, workerID: firstWorkerID,
		}
		secondWorker := &Worker{
			store: store, registry: registry, logger: slog.New(slog.DiscardHandler),
			clock: clock, workerID: secondWorkerID,
		}
		firstResult := make(chan error, 1)
		go func() {
			firstResult <- firstWorker.processObserved(t.Context(), registry.Handlers()[0], message)
		}()
		<-claimed

		advanceClock(now.Add(2 * defaultWorkerStaleRunningAge))
		firstWorker.renewRunningClaims(t.Context())
		require.NoError(
			t,
			store.RecoverStaleRunning(
				t.Context(),
				clock(),
				defaultWorkerStaleRunningAge,
				defaultWorkerMaxAttempts,
			),
		)
		secondErr := secondWorker.processObserved(t.Context(), registry.Handlers()[0], message)
		var runningErr runningJobDeliveryError
		require.ErrorAs(t, secondErr, &runningErr)
		assert.Equal(t, int32(1), calls.Load())

		close(release)
		require.NoError(t, <-firstResult)

		staleStartedAt := now.Add(-2 * defaultWorkerStaleRunningAge)
		makeStale := func(attemptCount int) Job {
			return Job{
				ID:           fake.UUID().V4(),
				JobType:      "finance.test",
				Status:       JobStatusRunning,
				WorkerID:     firstWorkerID,
				AttemptCount: attemptCount,
				Requester:    Requester{Source: RequesterSourceOperator},
				CreatedAt:    staleStartedAt,
				UpdatedAt:    staleStartedAt,
				QueuedAt:     staleStartedAt,
				StartedAt:    &staleStartedAt,
			}
		}
		staleRequeued := makeStale(defaultWorkerMaxAttempts - 1)
		staleExhausted := makeStale(defaultWorkerMaxAttempts)
		for _, stale := range []Job{staleRequeued, staleExhausted} {
			require.NoError(t, store.createWithDB(t.Context(), store.db, stale))
		}
		require.NoError(
			t,
			store.RecoverStaleRunning(
				t.Context(),
				now,
				defaultWorkerStaleRunningAge,
				defaultWorkerMaxAttempts,
			),
		)
		recovered, err := store.Get(t.Context(), staleRequeued.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusQueued, recovered.Status)
		exhausted, err := store.Get(t.Context(), staleExhausted.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, exhausted.Status)
		assert.Equal(t, "stale_running_attempts_exhausted", exhausted.Error.Code)

		scannedLease := makeStale(defaultWorkerMaxAttempts - 1)
		require.NoError(t, store.createWithDB(t.Context(), store.db, scannedLease))
		var scannedModel jobModel
		require.NoError(
			t,
			store.db.WithContext(t.Context()).Table(store.tableName).
				Where("id = ?", scannedLease.ID).
				First(&scannedModel).
				Error,
		)
		require.NoError(
			t,
			store.RenewRunning(t.Context(), jobFromModel(scannedModel), now),
		)
		require.NoError(
			t,
			store.recoverStaleRunningModel(
				t.Context(),
				scannedModel,
				now,
				defaultWorkerMaxAttempts,
			),
		)
		renewed, err := store.Get(t.Context(), scannedLease.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusRunning, renewed.Status)

		conditional := Job{
			ID: fake.UUID().V4(), JobType: "finance.test", Status: JobStatusQueued,
			Requester: Requester{Source: RequesterSourceOperator},
			CreatedAt: now, UpdatedAt: now, QueuedAt: now,
		}
		require.NoError(t, store.createWithDB(t.Context(), store.db, conditional))
		firstClaim, err := store.ClaimQueued(t.Context(), conditional.ID, firstWorkerID, now)
		require.NoError(t, err)
		requeuedAt := now.Add(time.Second)
		require.NoError(t, store.RequeueRunning(t.Context(), *firstClaim, requeuedAt))
		secondClaim, err := store.ClaimQueued(t.Context(), conditional.ID, secondWorkerID, now.Add(2*time.Second))
		require.NoError(t, err)
		require.ErrorIs(
			t,
			store.persistTerminalState(
				t.Context(),
				*firstClaim,
				newSucceededTerminalJobState(firstWorkerID, now.Add(3*time.Second)),
			),
			ErrJobClaimLost,
		)
		persisted, err := store.Get(t.Context(), conditional.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusRunning, persisted.Status)
		assert.Equal(t, secondClaim.WorkerID, persisted.WorkerID)
	})

	t.Run("business failures are sanitized and transport failures requeue", func(t *testing.T) {
		registry := NewRegistry()
		topic := "failure." + fake.UUID().V4()
		register(t, registry, topic, func(_ context.Context, _ Job, value command) error {
			if value.Value == "business" {
				return appdispatch.NewBusinessFailure(
					errors.New("SELECT secret"),
					"business",
					"safe summary",
					"safe details",
				)
			}
			return errors.New("transient")
		})
		store := newMockworkerStore(t)
		now := time.Now()
		worker := &Worker{
			store:    store,
			registry: registry,
			logger:   slog.New(slog.DiscardHandler),
			clock:    func() time.Time { return now },
			workerID: fake.UUID().V4(),
		}
		business := appdispatch.Message{
			ID:      fake.UUID().V4(),
			Payload: []byte(`{"value":"business","requester":"` + fake.UUID().V4() + `"}`),
		}
		queued := Job{ID: business.ID, JobType: "finance.test", Status: JobStatusQueued}
		claimed := queued
		claimed.Status = JobStatusRunning
		store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
		store.EXPECT().
			ClaimQueued(mock.Anything, business.ID, worker.workerID, now).
			Return(&claimed, nil).
			Once()
		store.EXPECT().
			persistTerminalState(mock.Anything, mock.Anything, mock.MatchedBy(func(state terminalJobState) bool {
				return state.status == JobStatusFailed && state.jobError.Code == "business" &&
					state.jobError.Details == "safe details"
			})).
			Return(nil).
			Once()
		require.NoError(t, worker.processObserved(t.Context(), registry.Handlers()[0], business))

		transient := appdispatch.Message{
			ID:      fake.UUID().V4(),
			Payload: []byte(`{"value":"transient","requester":"` + fake.UUID().V4() + `"}`),
		}
		queued.ID = transient.ID
		claimed.ID = transient.ID
		store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
		store.EXPECT().
			ClaimQueued(mock.Anything, transient.ID, worker.workerID, now).
			Return(&claimed, nil).
			Once()
		store.EXPECT().RequeueRunning(mock.Anything, mock.Anything, now).Return(nil).Once()
		require.Error(t, worker.processObserved(t.Context(), registry.Handlers()[0], transient))
	})

	t.Run(
		"materialization, claim, and terminal persistence failures block work",
		func(t *testing.T) {
			registry := NewRegistry()
			topic := "persistence." + fake.UUID().V4()
			called := false
			register(
				t,
				registry,
				topic,
				func(context.Context, Job, command) error { called = true; return nil },
			)
			store := newMockworkerStore(t)
			now := time.Now()
			worker := &Worker{
				store:    store,
				registry: registry,
				logger:   slog.New(slog.DiscardHandler),
				clock:    func() time.Time { return now },
				workerID: fake.UUID().V4(),
			}
			message := appdispatch.Message{
				ID:      fake.UUID().V4(),
				Payload: []byte(`{"value":"ok","requester":"` + fake.UUID().V4() + `"}`),
			}
			store.EXPECT().
				MaterializeQueued(mock.Anything, mock.Anything).
				Return(nil, errors.New(fake.Letter())).
				Once()
			require.Error(t, worker.processObserved(t.Context(), registry.Handlers()[0], message))
			assert.False(t, called)
			queued := Job{ID: message.ID, JobType: "finance.test", Status: JobStatusQueued}
			store.EXPECT().
				MaterializeQueued(mock.Anything, mock.Anything).
				Return(&queued, nil).
				Once()
			store.EXPECT().
				ClaimQueued(mock.Anything, message.ID, worker.workerID, now).
				Return(nil, errors.New(fake.Letter())).
				Once()
			require.Error(t, worker.processObserved(t.Context(), registry.Handlers()[0], message))
			assert.False(t, called)
			claimed := queued
			claimed.Status = JobStatusRunning
			store.EXPECT().
				MaterializeQueued(mock.Anything, mock.Anything).
				Return(&queued, nil).
				Once()
			store.EXPECT().
				ClaimQueued(mock.Anything, message.ID, worker.workerID, now).
				Return(&claimed, nil).
				Once()
			store.EXPECT().
				persistTerminalState(mock.Anything, mock.Anything, mock.Anything).
				Return(errors.New(fake.Letter())).
				Once()
			cancelled, cancel := context.WithCancel(t.Context())
			cancel()
			require.ErrorIs(
				t,
				worker.processObserved(cancelled, registry.Handlers()[0], message),
				context.Canceled,
			)
			assert.True(t, called)
		},
	)

	t.Run("covers worker error and recovery branches without transport timing", func(t *testing.T) {
		jobID := fake.UUID().V4()
		deliveryErr := runningJobDeliveryError{jobID: jobID}
		assert.Contains(t, deliveryErr.Error(), jobID)
		deliveryErr.NonRetryable()

		exhausted := exhaustedRetryError{err: errors.New(fake.UUID().V4())}
		require.Error(t, exhausted)
		require.NoError(t, exhausted.OnRetriesExhausted())

		tracker := &runOnceTracker{}
		tracker.startDelivery(jobID)
		tracker.startDelivery(jobID)
		tracker.finishDelivery(jobID)
		assert.False(t, tracker.isIdle())
		tracker.finishDelivery(jobID)
		assert.True(t, tracker.isIdle())

		now := time.Now()
		store := newMockworkerStore(t)
		worker := &Worker{
			store: store, logger: slog.New(slog.DiscardHandler), clock: func() time.Time { return now },
			claims: map[string]Job{jobID: {ID: jobID}},
		}
		store.EXPECT().RenewRunning(mock.Anything, Job{ID: jobID}, now).Return(errors.New(fake.UUID().V4())).Once()
		worker.renewRunningClaims(t.Context())

		terminalStore := newMockworkerStore(t)
		terminalWorker := &Worker{store: terminalStore, logger: slog.New(slog.DiscardHandler)}
		require.Error(t, terminalWorker.persistTerminal(t.Context(), jobID, terminalJobState{}, nil))
		state := newSucceededTerminalJobState(fake.UUID().V4(), now)
		terminalStore.EXPECT().persistTerminalState(mock.Anything, Job{ID: jobID}, state).Return(ErrJobClaimLost).Once()
		require.ErrorIs(t, terminalWorker.persistTerminalState(t.Context(), Job{ID: jobID}, state), ErrJobClaimLost)

		registry := NewRegistry()
		topic := "requeue-error." + fake.UUID().V4()
		register(t, registry, topic, func(context.Context, Job, command) error {
			return errors.New(fake.UUID().V4())
		})
		queued := Job{ID: jobID, JobType: "finance.test", Status: JobStatusQueued}
		claimed := queued
		claimed.Status = JobStatusRunning
		requeueStore := newMockworkerStore(t)
		requeueWorker := &Worker{
			store: requeueStore, registry: registry, logger: slog.New(slog.DiscardHandler),
			clock: func() time.Time { return now }, workerID: fake.UUID().V4(),
		}
		requeueStore.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
		requeueStore.EXPECT().
			ClaimQueued(mock.Anything, jobID, requeueWorker.workerID, now).
			Return(&claimed, nil).
			Once()
		requeueStore.EXPECT().
			RequeueRunning(mock.Anything, claimed, now).
			Return(errors.New(fake.UUID().V4())).
			Once()
		require.Error(t, requeueWorker.processObserved(
			t.Context(), registry.Handlers()[0], appdispatch.Message{
				ID:      jobID,
				Payload: []byte(`{"value":"` + fake.UUID().V4() + `","requester":"` + fake.UUID().V4() + `"}`),
			},
		))
	})

	t.Run(
		"keeps registry, read service, and lifecycle storage boundaries explicit",
		func(t *testing.T) {
			store, _, _, _, _ := makeTransport(t)
			now := time.Now()
			job := Job{ID: fake.UUID().V4(), JobType: "type-a", Status: JobStatusQueued,
				Requester: Requester{
					UserID: fake.UUID().V4(),
					Source: RequesterSourceOperator,
				}, CreatedAt: now, UpdatedAt: now, QueuedAt: now}
			materialized, err := store.MaterializeQueued(t.Context(), job)
			require.NoError(t, err)
			assert.Equal(t, job.ID, materialized.ID)
			assert.Equal(t, job.JobType, materialized.JobType)
			assert.Equal(t, job.Requester, materialized.Requester)
			duplicate, err := store.MaterializeQueued(t.Context(), job)
			require.NoError(t, err)
			assert.Equal(t, job.ID, duplicate.ID)
			claimed, err := store.ClaimQueued(t.Context(), job.ID, fake.UUID().V4(), now.Add(time.Second))
			require.NoError(t, err)
			require.ErrorIs(t, func() error {
				_, claimErr := store.ClaimQueued(t.Context(), job.ID, fake.UUID().V4(), now)
				return claimErr
			}(), ErrJobNotQueued)
			require.NoError(t, store.RequeueRunning(t.Context(), *claimed, now.Add(2*time.Second)))
			claimed, err = store.ClaimQueued(
				t.Context(),
				job.ID,
				fake.UUID().V4(),
				now.Add(3*time.Second),
			)
			require.NoError(t, err)
			require.NoError(
				t,
				store.persistTerminalState(
					t.Context(),
					*claimed,
					newSucceededTerminalJobState(claimed.WorkerID, now.Add(4*time.Second)),
				),
			)
			service, err := NewService(ServiceDeps{Store: store})
			require.NoError(t, err)
			got, err := service.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusSucceeded, got.Status)
			listed, err := service.List(
				t.Context(),
				ListParams{
					JobTypes: []JobType{"type-a"},
					Statuses: []JobStatus{JobStatusSucceeded},
					Limit:    1,
				},
			)
			require.NoError(t, err)
			require.Len(t, listed.Items, 1)
			_, err = service.Get(t.Context(), fake.UUID().V4())
			require.Error(t, err)
			_, err = store.List(t.Context(), ListParams{Cursor: "%"})
			require.Error(t, err)
			require.Error(t, store.RequeueRunning(t.Context(), *claimed, time.Time{}))

			running := job
			running.ID, running.Status, running.AttemptCount = fake.UUID().
				V4(),
				JobStatusRunning, defaultWorkerMaxAttempts
			staleStartedAt := now.Add(-2 * defaultWorkerStaleRunningAge)
			running.StartedAt = &staleStartedAt
			running.UpdatedAt = staleStartedAt
			err = store.createWithDB(t.Context(), store.db, running)
			require.NoError(t, err)
			require.NoError(
				t,
				store.RecoverStaleRunning(
					t.Context(),
					now.Add(5*time.Second),
					defaultWorkerStaleRunningAge,
					defaultWorkerMaxAttempts,
				),
			)
			exhausted, err := store.Get(t.Context(), running.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusFailed, exhausted.Status)

			registry := NewRegistry()
			topic := "scheduled." + fake.UUID().V4()
			require.Error(t, registry.Register(nil))
			require.Error(t, RegisterTypedHandler[int](nil, TypedHandlerSpec[int]{}))
			require.Error(t, RegisterTypedHandler(registry, TypedHandlerSpec[int]{}))
			require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[int]{
				JobType: "scheduled", Topic: topic,
				Metadata: func(int) (JobMetadata, error) {
					return JobMetadata{
						JobType:   "scheduled",
						Requester: Requester{Source: RequesterSourceOperator},
					}, nil
				},
				Run: func(context.Context, Job, int) error { return nil },
			}))
			require.Error(t, RegisterTypedHandler(registry, TypedHandlerSpec[int]{
				JobType:  "other",
				Topic:    topic,
				Metadata: func(int) (JobMetadata, error) { return JobMetadata{}, nil },
				Run:      func(context.Context, Job, int) error { return nil },
			}))
			_, err = registry.Handler("missing")
			require.ErrorIs(t, err, ErrHandlerNotRegistered)
			handler, err := registry.Handler(topic)
			require.NoError(t, err)
			metadata, err := handler.metadata([]byte("1"))
			require.NoError(t, err)
			assert.Equal(t, JobType("scheduled"), metadata.JobType)
			require.NoError(t, handler.execute(t.Context(), Job{}, []byte("1")))
			_, err = handler.metadata([]byte("{"))
			require.Error(t, err)
			require.Error(t, handler.execute(t.Context(), Job{}, []byte("{")))
			var nilRegistry *Registry
			_, err = nilRegistry.Handler("missing")
			require.ErrorIs(t, err, ErrHandlerNotRegistered)
			require.Empty(t, nilRegistry.Handlers())
			validSpec := TypedHandlerSpec[int]{
				JobType:  "validation",
				Topic:    fake.Lorem().Word(),
				Metadata: func(int) (JobMetadata, error) { return JobMetadata{}, nil },
			}
			for _, invalidSpec := range []TypedHandlerSpec[int]{
				{},
				{Topic: validSpec.Topic},
				{JobType: validSpec.JobType, Topic: validSpec.Topic},
				{JobType: validSpec.JobType, Topic: validSpec.Topic, Metadata: validSpec.Metadata},
			} {
				require.Error(t, RegisterTypedHandler(NewRegistry(), invalidSpec))
			}
		},
	)

	t.Run("validates lifecycle utility inputs", func(t *testing.T) {
		assert.Equal(
			t,
			defaultWorkerPollInterval,
			normalizeWorkerConfig(WorkerConfig{}).PollInterval,
		)
		assert.Equal(
			t,
			maxListLimit,
			normalizeListParams(ListParams{Limit: maxListLimit + 1}).Limit,
		)
		assert.Equal(
			t,
			Requester{UserID: "user", Source: RequesterSourceOperator},
			canonicalizeRequester(Requester{UserID: " user ", Source: " operator "}),
		)
		assert.Nil(t, jobErrorFromExecution(nil))
		assert.Equal(
			t,
			"job execution failed",
			jobErrorFromExecution(errors.New("SELECT secret")).Details,
		)
		assert.Equal(t, "…", truncateBounded("long", len("…")))
		require.Error(t, validateRequiredTimestamp("at", time.Time{}))
		_, err := NewStore(nil, "", StoreOpts{})
		require.Error(t, err)
	})

	t.Run("validates worker construction and pending duplicate delivery", func(t *testing.T) {
		_, factory, _, _, _ := makeTransport(t)
		registry := NewRegistry()
		topic := "worker." + fake.UUID().V4()
		register(t, registry, topic, func(context.Context, Job, command) error { return nil })
		_, err := NewWorker(WorkerDeps{})
		require.Error(t, err)
		store := newMockworkerStore(t)
		_, err = NewWorker(WorkerDeps{Store: store})
		require.Error(t, err)
		_, err = NewWorker(WorkerDeps{Store: store, Registry: registry})
		require.Error(t, err)
		worker, err := NewWorker(
			WorkerDeps{Store: store, Registry: registry, RouterFactory: factory},
		)
		require.NoError(t, err)
		assert.Equal(t, "jobs-worker", worker.workerID)
		require.NoError(t, worker.installHandlers())
		require.NoError(t, worker.installHandlers())
		require.NoError(t, worker.Stop(t.Context()))

		pending := Job{ID: fake.UUID().V4(), JobType: "finance.test", Status: JobStatusRunning}
		store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&pending, nil).Once()
		err = worker.processObserved(
			t.Context(),
			registry.Handlers()[0],
			appdispatch.Message{
				ID:      pending.ID,
				Payload: []byte(`{"value":"ok","requester":"` + fake.UUID().V4() + `"}`),
			},
		)
		var runningErr runningJobDeliveryError
		require.ErrorAs(t, err, &runningErr)
		pending.Status = JobStatusSucceeded
		store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&pending, nil).Once()
		require.NoError(
			t,
			worker.processObserved(
				t.Context(),
				registry.Handlers()[0],
				appdispatch.Message{
					ID:      pending.ID,
					Payload: []byte(`{"value":"ok","requester":"` + fake.UUID().V4() + `"}`),
				},
			),
		)
	})
}
