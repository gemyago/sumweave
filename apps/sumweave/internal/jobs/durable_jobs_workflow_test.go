package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
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
		dsn := fmt.Sprintf("file:observed-jobs-%s?mode=memory&cache=shared", fake.UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := NewStore(db, dsn, StoreOpts{TablePrefix: "observed_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		config := appdispatch.Config{
			DatabaseDSN:  dsn,
			TablePrefix:  "observed_",
			PollInterval: time.Millisecond,
		}
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), config, db))
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
		require.NoError(
			t,
			publisher.Publish(t.Context(), appdispatch.NewMessage(topic, []byte(fake.UUID().V4()))),
		)
		require.Eventually(
			t,
			func() bool { return calls.Load() == 1 },
			time.Second,
			time.Millisecond,
		)
		items, err := store.List(t.Context(), ListParams{})
		require.NoError(t, err)
		assert.Empty(t, items.Items)
		require.NoError(t, router.Close())
	})

	t.Run(
		"first delivery claims message identity before domain work and skips terminal duplicates",
		func(t *testing.T) {
			store, factory, publisher, _, _ := makeTransport(t)
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
			require.NoError(t, publisher.Publish(t.Context(), message))
			require.NoError(t, worker.RunOnce(t.Context()))
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
		firstWorker := &Worker{
			store: store, registry: registry, logger: slog.New(slog.DiscardHandler),
			clock: func() time.Time { return now }, workerID: firstWorkerID,
		}
		secondWorker := &Worker{
			store: store, registry: registry, logger: slog.New(slog.DiscardHandler),
			clock: func() time.Time { return now }, workerID: secondWorkerID,
		}
		firstResult := make(chan error, 1)
		go func() {
			firstResult <- firstWorker.processObserved(t.Context(), registry.Handlers()[0], message)
		}()
		<-claimed

		require.NoError(
			t,
			store.RecoverStaleRunning(
				t.Context(),
				now,
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
			persistTerminalState(mock.Anything, business.ID, mock.MatchedBy(func(state terminalJobState) bool {
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
		store.EXPECT().RequeueRunning(mock.Anything, transient.ID, now).Return(nil).Once()
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
				persistTerminalState(mock.Anything, message.ID, mock.Anything).
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
			_, err = store.ClaimQueued(t.Context(), job.ID, fake.UUID().V4(), now.Add(time.Second))
			require.NoError(t, err)
			require.ErrorIs(t, func() error {
				_, claimErr := store.ClaimQueued(t.Context(), job.ID, fake.UUID().V4(), now)
				return claimErr
			}(), ErrJobNotQueued)
			require.NoError(t, store.RequeueRunning(t.Context(), job.ID, now.Add(2*time.Second)))
			claimed, err := store.ClaimQueued(
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
					job.ID,
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
			require.Error(t, store.RequeueRunning(t.Context(), job.ID, time.Time{}))

			running := job
			running.ID, running.Status, running.AttemptCount = fake.UUID().
				V4(),
				JobStatusRunning, defaultWorkerMaxAttempts
			staleStartedAt := now.Add(-2 * defaultWorkerStaleRunningAge)
			running.StartedAt = &staleStartedAt
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
		_, err := NewStore(nil, "sqlite", StoreOpts{})
		require.Error(t, err)
		db, err := sqlconn.Open(
			fmt.Sprintf("file:invalid-store-%s?mode=memory&cache=shared", fake.UUID().V4()),
		)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		_, err = NewStore(db, " ", StoreOpts{})
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
