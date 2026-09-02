package jobs

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestObservedSubscriptionsUnit(t *testing.T) {
	fake := faker.New()
	type command struct {
		Value     string `json:"value"`
		Requester string `json:"requester"`
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

	t.Run("keeps worker orchestration database-free", func(t *testing.T) {
		store := newMockworkerStore(t)
		registry := NewRegistry()
		topic := "unit." + fake.UUID().V4()
		register(t, registry, topic, func(context.Context, Job, command) error { return errors.New(fake.UUID().V4()) })
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
			Payload: []byte(`{"value":"` + fake.UUID().V4() + `","requester":"` + fake.UUID().V4() + `"}`),
		}
		queued := Job{ID: message.ID, JobType: "finance.test", Status: JobStatusQueued}
		claimed := queued
		claimed.Status = JobStatusRunning
		store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
		store.EXPECT().ClaimQueued(mock.Anything, message.ID, worker.workerID, now).Return(&claimed, nil).Once()
		store.EXPECT().RequeueRunning(mock.Anything, claimed, now).Return(nil).Once()
		require.Error(t, worker.processObserved(t.Context(), registry.Handlers()[0], message))
	})

	t.Run("validates registry and lifecycle utilities without persistence", func(t *testing.T) {
		registry := NewRegistry()
		require.Error(t, registry.Register(nil))
		require.Error(t, RegisterTypedHandler[int](nil, TypedHandlerSpec[int]{}))
		require.Error(t, RegisterTypedHandler(registry, TypedHandlerSpec[int]{}))
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[int]{
			JobType: "unit", Topic: "unit." + fake.UUID().V4(),
			Metadata: func(int) (JobMetadata, error) { return JobMetadata{JobType: "unit"}, nil },
			Run:      func(context.Context, Job, int) error { return nil },
		}))
		assert.Equal(t, defaultWorkerPollInterval, normalizeWorkerConfig(WorkerConfig{}).PollInterval)
		assert.Equal(t, maxListLimit, normalizeListParams(ListParams{Limit: maxListLimit + 1}).Limit)
		require.Error(t, validateRequiredTimestamp("at", time.Time{}))
		_, err := NewStore(nil, "", StoreOpts{})
		require.ErrorContains(t, err, "sql database is required")
		assert.NotNil(t, (*Store).createWithDB)
	})

	t.Run("keeps registry, job values, and read service database-free", func(t *testing.T) {
		registry := NewRegistry()
		topic := "registry." + fake.UUID().V4()
		metadataErr := errors.New(fake.UUID().V4())
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[string]{
			JobType: "unit", Topic: topic,
			Metadata: func(string) (JobMetadata, error) { return JobMetadata{}, metadataErr },
			Run:      func(context.Context, Job, string) error { return nil },
		}))
		handler, err := registry.Handler(topic)
		require.NoError(t, err)
		_, err = handler.metadata([]byte(`"` + fake.UUID().V4() + `"`))
		require.ErrorIs(t, err, metadataErr)
		_, err = handler.metadata([]byte(`{`))
		require.Error(t, err)
		require.Error(t, handler.execute(t.Context(), Job{}, []byte(`{`)))
		require.Error(t, registry.Register(handler))
		_, err = (*Registry)(nil).Handler(topic)
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
		assert.Nil(t, (*Registry)(nil).Handlers())

		assert.Equal(t, Requester{UserID: "user", Source: RequesterSourceOperator}, canonicalizeRequester(Requester{
			UserID: " user ", Source: " operator ",
		}))
		assert.Equal(t, "…", truncateBounded("long", len("…")))
		assert.Equal(t, "l", truncateBounded("long", 1))
		assert.Equal(t, "job execution failed", jobErrorFromExecution(errors.New("SELECT secret")).Details)
		failure, ok := appdispatch.BusinessFailureFrom(
			appdispatch.NewBusinessFailure(errors.New(fake.UUID().V4()), "code", "summary", "details"),
		)
		require.True(t, ok)
		assert.Equal(t, "code", jobErrorFromBusinessFailure(failure).Code)
		require.NoError(t, validateRequiredTimestamp("at", time.Now()))

		store := newMockjobReader(t)
		service, err := NewService(ServiceDeps{Store: store})
		require.NoError(t, err)
		jobID := fake.UUID().V4()
		store.EXPECT().Get(t.Context(), jobID).Return(nil, ErrJobNotFound).Once()
		_, err = service.Get(t.Context(), jobID)
		require.ErrorContains(t, err, "job not found")
		result := ListResult{Items: []Job{{ID: fake.UUID().V4()}}}
		store.EXPECT().List(t.Context(), ListParams{}).Return(result, nil).Once()
		actual, err := service.List(t.Context(), ListParams{})
		require.NoError(t, err)
		assert.Equal(t, result, actual)
		storeErr := errors.New(fake.UUID().V4())
		store.EXPECT().Get(t.Context(), jobID).Return(nil, storeErr).Once()
		_, err = service.Get(t.Context(), jobID)
		require.ErrorIs(t, err, storeErr)
		store.EXPECT().List(t.Context(), ListParams{}).Return(ListResult{}, storeErr).Once()
		_, err = service.List(t.Context(), ListParams{})
		require.ErrorIs(t, err, storeErr)
		_, err = NewService(ServiceDeps{})
		require.Error(t, err)
	})

	t.Run("handles observed delivery branches with deterministic store outcomes", func(t *testing.T) {
		makeObserved := func(t *testing.T, run func(context.Context, Job, command) error) (*Worker, observedHandler, appdispatch.Message, Job, Job, time.Time) {
			t.Helper()
			registry := NewRegistry()
			topic := "unit." + fake.UUID().V4()
			register(t, registry, topic, run)
			now := time.Now()
			worker := &Worker{
				store: newMockworkerStore(t), registry: registry, logger: slog.New(slog.DiscardHandler),
				clock: func() time.Time { return now }, workerID: fake.UUID().V4(),
			}
			message := appdispatch.Message{
				ID:      fake.UUID().V4(),
				Payload: []byte(`{"value":"` + fake.UUID().V4() + `","requester":"` + fake.UUID().V4() + `"}`),
			}
			queued := Job{ID: message.ID, JobType: "finance.test", Status: JobStatusQueued}
			claimed := queued
			claimed.Status = JobStatusRunning
			return worker, registry.Handlers()[0], message, queued, claimed, now
		}

		t.Run("leaves duplicate running and terminal jobs alone", func(t *testing.T) {
			worker, handler, message, queued, _, _ := makeObserved(
				t,
				func(context.Context, Job, command) error { return nil },
			)
			running := queued
			running.Status = JobStatusRunning
			store := worker.store.(*mockworkerStore)
			store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&running, nil).Once()
			var runningErr runningJobDeliveryError
			require.ErrorAs(t, worker.processObserved(t.Context(), handler, message), &runningErr)

			terminal := queued
			terminal.Status = JobStatusSucceeded
			store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&terminal, nil).Once()
			require.NoError(t, worker.processObserved(t.Context(), handler, message))
		})

		t.Run("surfaces materialization and claim outcomes before work", func(t *testing.T) {
			called := false
			worker, handler, message, queued, _, now := makeObserved(t, func(context.Context, Job, command) error {
				called = true
				return nil
			})
			store := worker.store.(*mockworkerStore)
			materializeErr := errors.New(fake.UUID().V4())
			store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(nil, materializeErr).Once()
			require.ErrorIs(t, worker.processObserved(t.Context(), handler, message), materializeErr)
			assert.False(t, called)

			store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
			store.EXPECT().
				ClaimQueued(mock.Anything, message.ID, worker.workerID, now).
				Return(nil, ErrJobNotQueued).
				Once()
			require.NoError(t, worker.processObserved(t.Context(), handler, message))
			assert.False(t, called)
		})

		t.Run("persists success and business failures without a database", func(t *testing.T) {
			worker, handler, message, queued, claimed, now := makeObserved(
				t,
				func(context.Context, Job, command) error { return nil },
			)
			store := worker.store.(*mockworkerStore)
			store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
			store.EXPECT().ClaimQueued(mock.Anything, message.ID, worker.workerID, now).Return(&claimed, nil).Once()
			store.EXPECT().
				persistTerminalState(
					mock.Anything,
					claimed,
					newSucceededTerminalJobState(worker.workerID, now),
				).
				Return(nil).
				Once()
			require.NoError(t, worker.processObserved(t.Context(), handler, message))

			businessWorker, businessHandler, businessMessage, businessQueued, businessClaimed, businessNow := makeObserved(
				t,
				func(context.Context, Job, command) error {
					return appdispatch.NewBusinessFailure(
						errors.New(fake.UUID().V4()),
						"business",
						"summary",
						"details",
					)
				},
			)
			businessStore := businessWorker.store.(*mockworkerStore)
			businessStore.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&businessQueued, nil).Once()
			businessStore.EXPECT().
				ClaimQueued(mock.Anything, businessMessage.ID, businessWorker.workerID, businessNow).
				Return(&businessClaimed, nil).
				Once()
			businessStore.EXPECT().
				persistTerminalState(
					mock.Anything,
					businessClaimed,
					mock.MatchedBy(func(state terminalJobState) bool {
						return state.status == JobStatusFailed && state.jobError.Code == "business"
					}),
				).
				Return(nil).
				Once()
			require.NoError(t, businessWorker.processObserved(t.Context(), businessHandler, businessMessage))
		})
	})

	t.Run("keeps retry and claim helpers deterministic", func(t *testing.T) {
		now := time.Now()
		store := newMockworkerStore(t)
		worker := &Worker{
			store: store, logger: slog.New(slog.DiscardHandler), clock: func() time.Time { return now },
			claims: map[string]Job{}, config: WorkerConfig{PollInterval: time.Second, StaleRunningAge: time.Second},
		}
		assert.Equal(t, 500*time.Millisecond, worker.recoveryInterval())
		worker.trackClaim(Job{ID: fake.UUID().V4()})
		claims := worker.runningClaims()
		require.Len(t, claims, 1)
		store.EXPECT().RenewRunning(mock.Anything, claims[0], now).Return(nil).Once()
		worker.renewRunningClaims(t.Context())
		worker.releaseClaim(claims[0].ID)
		assert.Empty(t, worker.runningClaims())

		tracker := &runOnceTracker{}
		tracker.startDelivery(fake.UUID().V4())
		assert.False(t, tracker.isIdle())
		tracker.startRetry(fake.UUID().V4())
		assert.False(t, tracker.isIdle())
		for jobID := range tracker.active {
			tracker.finishDelivery(jobID)
		}
		for jobID := range tracker.pendingRetries {
			tracker.finishRetry(jobID)
		}
		assert.True(t, tracker.isIdle())
	})
}

func TestWorkerLifecycleUnit(t *testing.T) {
	fake := faker.New()
	type command struct {
		Requester string `json:"requester"`
	}
	register := func(t *testing.T, registry *Registry, run func(context.Context, Job, command) error) {
		t.Helper()
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[command]{
			JobType: "unit", Topic: "unit." + fake.UUID().V4(),
			Metadata: func(value command) (JobMetadata, error) {
				return JobMetadata{JobType: "unit", Requester: Requester{
					UserID: value.Requester, Source: RequesterSourceOperator,
				}}, nil
			},
			Run: run,
		}))
	}
	makeWorker := func(t *testing.T, registry *Registry, store *mockworkerStore) (*Worker, *mockworkerRouter) {
		t.Helper()
		router := newMockworkerRouter(t)
		factory := newMockworkerRouterFactory(t)
		factory.EXPECT().NewRouter(jobConsumerGroup).Return(workerRouterResult{router: router}, nil).Once()
		router.EXPECT().SetRetryLifecycle(mock.Anything).Return(nil).Once()
		now := time.Now()
		worker, err := NewWorker(WorkerDeps{
			Store: store, Registry: registry, Logger: slog.New(slog.DiscardHandler),
			Clock: func() time.Time { return now }, WorkerID: fake.UUID().V4(),
			Config:        WorkerConfig{PollInterval: time.Millisecond, StaleRunningAge: time.Millisecond},
			routerFactory: factory,
		})
		require.NoError(t, err)
		return worker, router
	}

	t.Run("validates router construction and lifecycle setup", func(t *testing.T) {
		_, err := NewWorker(WorkerDeps{})
		require.ErrorContains(t, err, "jobs store is required")
		store := newMockworkerStore(t)
		_, err = NewWorker(WorkerDeps{Store: store})
		require.ErrorContains(t, err, "jobs registry is required")
		_, err = NewWorker(WorkerDeps{Store: store, Registry: NewRegistry()})
		require.ErrorContains(t, err, "jobs router factory is required")

		factory := newMockworkerRouterFactory(t)
		factoryErr := errors.New(fake.UUID().V4())
		factory.EXPECT().NewRouter(jobConsumerGroup).Return(workerRouterResult{}, factoryErr).Once()
		_, err = NewWorker(WorkerDeps{Store: store, Registry: NewRegistry(), routerFactory: factory})
		require.ErrorIs(t, err, factoryErr)

		router := newMockworkerRouter(t)
		factory = newMockworkerRouterFactory(t)
		factory.EXPECT().NewRouter(jobConsumerGroup).Return(workerRouterResult{router: router}, nil).Once()
		lifecycleErr := errors.New(fake.UUID().V4())
		router.EXPECT().SetRetryLifecycle(mock.Anything).Return(lifecycleErr).Once()
		_, err = NewWorker(WorkerDeps{Store: store, Registry: NewRegistry(), routerFactory: factory})
		require.ErrorIs(t, err, lifecycleErr)

		adapterErr := errors.New(fake.UUID().V4())
		_, err = (appdispatchWorkerRouterFactory{newRouter: func(string) (*appdispatch.Router, error) {
			return nil, adapterErr
		}}).NewRouter(jobConsumerGroup)
		require.ErrorIs(t, err, adapterErr)
	})

	t.Run("runs router lifecycle without persistence", func(t *testing.T) {
		t.Run("installs handlers only once and stops", func(t *testing.T) {
			store := newMockworkerStore(t)
			registry := NewRegistry()
			register(t, registry, func(context.Context, Job, command) error { return nil })
			worker, router := makeWorker(t, registry, store)
			router.EXPECT().Handle(mock.Anything).Return(nil).Once()
			require.NoError(t, worker.installHandlers())
			require.NoError(t, worker.installHandlers())
			router.EXPECT().Close().Return(nil).Once()
			require.NoError(t, worker.Stop(t.Context()))
		})

		t.Run("propagates startup failures", func(t *testing.T) {
			store := newMockworkerStore(t)
			registry := NewRegistry()
			register(t, registry, func(context.Context, Job, command) error { return nil })
			worker, router := makeWorker(t, registry, store)
			handleErr := errors.New(fake.UUID().V4())
			router.EXPECT().Handle(mock.Anything).Return(handleErr).Once()
			require.ErrorIs(t, worker.Run(t.Context()), handleErr)

			store = newMockworkerStore(t)
			worker, _ = makeWorker(t, NewRegistry(), store)
			recoveryErr := errors.New(fake.UUID().V4())
			store.EXPECT().
				RecoverStaleRunning(mock.Anything, mock.Anything, time.Millisecond, defaultWorkerMaxAttempts).
				Return(recoveryErr).Once()
			require.ErrorIs(t, worker.Run(t.Context()), recoveryErr)
		})

		t.Run("returns router results from long-running and one-shot modes", func(t *testing.T) {
			store := newMockworkerStore(t)
			worker, router := makeWorker(t, NewRegistry(), store)
			store.EXPECT().
				RecoverStaleRunning(mock.Anything, mock.Anything, time.Millisecond, defaultWorkerMaxAttempts).
				Return(nil).Once()
			runErr := errors.New(fake.UUID().V4())
			router.EXPECT().Run(t.Context()).Return(runErr).Once()
			require.ErrorIs(t, worker.Run(t.Context()), runErr)

			store = newMockworkerStore(t)
			worker, router = makeWorker(t, NewRegistry(), store)
			store.EXPECT().
				RecoverStaleRunning(mock.Anything, mock.Anything, time.Millisecond, defaultWorkerMaxAttempts).
				Return(nil).Maybe()
			router.EXPECT().Run(mock.Anything).RunAndReturn(func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			}).Once()
			require.NoError(t, worker.RunOnce(t.Context()))

			store = newMockworkerStore(t)
			worker, router = makeWorker(t, NewRegistry(), store)
			store.EXPECT().
				RecoverStaleRunning(mock.Anything, mock.Anything, time.Millisecond, defaultWorkerMaxAttempts).
				Return(nil).Once()
			runOnceErr := errors.New(fake.UUID().V4())
			router.EXPECT().Run(mock.Anything).Return(runOnceErr).Once()
			require.ErrorIs(t, worker.RunOnce(t.Context()), runOnceErr)
			require.NoError(t, worker.runOnceResult(context.DeadlineExceeded))
		})
	})

	t.Run("covers observed delivery failures and retry finalization", func(t *testing.T) {
		makeObserved := func(t *testing.T, run func(context.Context, Job, command) error) (
			*Worker,
			observedHandler,
			appdispatch.Message,
			Job,
			Job,
			time.Time,
		) {
			t.Helper()
			registry := NewRegistry()
			register(t, registry, run)
			now := time.Now()
			message := appdispatch.Message{
				ID: fake.UUID().V4(), Payload: []byte(`{"requester":"` + fake.UUID().V4() + `"}`),
			}
			queued := Job{ID: message.ID, JobType: "unit", Status: JobStatusQueued}
			claimed := queued
			claimed.Status = JobStatusRunning
			return &Worker{
				store: newMockworkerStore(t), logger: slog.New(slog.DiscardHandler),
				clock: func() time.Time { return now }, workerID: fake.UUID().V4(), claims: map[string]Job{},
			}, registry.Handlers()[0], message, queued, claimed, now
		}

		t.Run("returns metadata and claim errors", func(t *testing.T) {
			worker, handler, message, queued, _, now := makeObserved(
				t,
				func(context.Context, Job, command) error { return nil },
			)
			message.Payload = []byte(`{`)
			require.Error(t, worker.processObserved(t.Context(), handler, message))

			claimErr := errors.New(fake.UUID().V4())
			store := worker.store.(*mockworkerStore)
			message.Payload = []byte(`{"requester":"` + fake.UUID().V4() + `"}`)
			store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
			store.EXPECT().ClaimQueued(mock.Anything, message.ID, worker.workerID, now).Return(nil, claimErr).Once()
			require.ErrorIs(t, worker.processObserved(t.Context(), handler, message), claimErr)

			runningErr := runningJobDeliveryError{jobID: fake.UUID().V4()}
			require.Error(t, runningErr)
			var nonRetryable appdispatch.NonRetryable = runningErr
			assert.NotNil(t, nonRetryable)
		})

		t.Run("requeues transient work and finalizes exhausted retries", func(t *testing.T) {
			executionErr := errors.New(fake.UUID().V4())
			runWithExecutionError := func(context.Context, Job, command) error {
				return executionErr
			}
			worker, handler, message, queued, claimed, now := makeObserved(t, runWithExecutionError)
			store := worker.store.(*mockworkerStore)
			store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
			store.EXPECT().ClaimQueued(mock.Anything, message.ID, worker.workerID, now).Return(&claimed, nil).Once()
			store.EXPECT().RequeueRunning(mock.Anything, claimed, now).Return(nil).Once()
			err := worker.processObserved(t.Context(), handler, message)
			var exhausted exhaustedRetryError
			require.ErrorAs(t, err, &exhausted)
			store.EXPECT().FinalizeRetryExhausted(mock.Anything, claimed.ID, now, mock.Anything).Return(nil).Once()
			require.NoError(t, exhausted.OnRetriesExhausted())
			assert.Equal(t, executionErr, exhausted.Unwrap())
			emptyExhausted := exhaustedRetryError{err: executionErr}
			require.NoError(t, emptyExhausted.OnRetriesExhausted())

			worker, handler, message, queued, claimed, now = makeObserved(t, runWithExecutionError)
			store = worker.store.(*mockworkerStore)
			requeueErr := errors.New(fake.UUID().V4())
			store.EXPECT().MaterializeQueued(mock.Anything, mock.Anything).Return(&queued, nil).Once()
			store.EXPECT().ClaimQueued(mock.Anything, message.ID, worker.workerID, now).Return(&claimed, nil).Once()
			store.EXPECT().RequeueRunning(mock.Anything, claimed, now).Return(requeueErr).Once()
			require.ErrorIs(t, worker.processObserved(t.Context(), handler, message), requeueErr)
		})
	})

	t.Run("persists terminal states through deterministic store outcomes", func(t *testing.T) {
		now := time.Now()
		state := newSucceededTerminalJobState(fake.UUID().V4(), now)
		job := Job{ID: fake.UUID().V4()}
		store := newMockworkerStore(t)
		worker := &Worker{store: store, logger: slog.New(slog.DiscardHandler)}
		store.EXPECT().persistTerminalState(mock.Anything, job, state).Return(ErrJobClaimLost).Once()
		require.ErrorIs(t, worker.persistTerminalState(t.Context(), job, state), ErrJobClaimLost)

		store = newMockworkerStore(t)
		worker = &Worker{store: store, logger: slog.New(slog.DiscardHandler)}
		store.EXPECT().persistTerminalState(mock.Anything, job, state).Return(errors.New(fake.UUID().V4())).Once()
		store.EXPECT().persistTerminalState(mock.Anything, job, state).Return(nil).Once()
		require.NoError(t, worker.persistTerminalState(t.Context(), job, state))

		store = newMockworkerStore(t)
		worker = &Worker{store: store, logger: slog.New(slog.DiscardHandler)}
		ctx, cancel := context.WithCancel(t.Context())
		persistErr := errors.New(fake.UUID().V4())
		store.EXPECT().persistTerminalState(mock.Anything, job, state).
			Run(func(context.Context, Job, terminalJobState) {
				cancel()
			}).Return(persistErr).Once()
		err := worker.persistTerminalState(ctx, job, state)
		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, err, persistErr)

		require.Error(t, worker.persistTerminal(t.Context(), job.ID, terminalJobState{}, func(context.Context) error {
			return nil
		}))
	})

	t.Run("tracks retry callbacks, lease renewal, and recovery exits", func(t *testing.T) {
		worker := &Worker{runOnce: &runOnceTracker{}, logger: slog.New(slog.DiscardHandler)}
		jobID := fake.UUID().V4()
		worker.startRunOnceRetry(jobID)
		assert.False(t, worker.runOnce.isIdle())
		worker.finishRunOnceRetry(jobID)
		assert.True(t, worker.runOnce.isIdle())

		worker.runOnce.startDelivery(jobID)
		worker.runOnce.startDelivery(jobID)
		worker.runOnce.finishDelivery(jobID)
		assert.False(t, worker.runOnce.isIdle())
		worker.runOnce.finishDelivery(jobID)
		assert.True(t, worker.runOnce.isIdle())

		now := time.Now()
		claim := Job{ID: jobID}
		store := newMockworkerStore(t)
		worker = &Worker{
			store: store, logger: slog.New(slog.DiscardHandler), clock: func() time.Time { return now },
			config: WorkerConfig{PollInterval: time.Millisecond, StaleRunningAge: 2 * time.Second},
			claims: map[string]Job{claim.ID: claim},
		}
		assert.Equal(t, time.Millisecond, worker.recoveryInterval())
		store.EXPECT().RenewRunning(mock.Anything, claim, now).Return(errors.New(fake.UUID().V4())).Once()
		worker.renewRunningClaims(t.Context())
		cancelledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		worker.recoverStaleRunningPeriodically(cancelledCtx, make(chan struct{}))
	})
}
