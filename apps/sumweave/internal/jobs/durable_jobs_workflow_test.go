package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDurableJobsWorkflow(t *testing.T) {
	fake := faker.New()
	type input struct {
		AccountID string `json:"accountId"`
	}
	type result struct {
		Imported int `json:"imported"`
	}
	type progress struct {
		Stage string `json:"stage"`
	}
	makeStore := func(t *testing.T) (*Store, string) {
		t.Helper()
		dsn := fmt.Sprintf("file:jobs-durable-%s?mode=memory&cache=shared", fake.UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := NewStore(db, dsn, StoreOpts{TablePrefix: "finance_jobs_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store, dsn
	}
	makeRouterFactory := func(t *testing.T, dsn string, tablePrefix string) *appdispatch.RouterFactory {
		t.Helper()
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		config := appdispatch.Config{DatabaseDSN: dsn, TablePrefix: tablePrefix, PollInterval: time.Millisecond}
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), config, db))
		publisher, err := appdispatch.NewPublisher(config, db, slog.Default())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		factory, err := appdispatch.NewRouterFactory(config, db, publisher, slog.Default())
		require.NoError(t, err)
		return factory
	}
	makeJob := func(now time.Time) Job {
		return Job{
			ID:      "job-" + fake.UUID().V4(),
			JobType: JobType("finance.csv_import"),
			Status:  JobStatusQueued,
			Requester: Requester{
				UserID: "user-" + fake.UUID().V4(),
				Source: RequesterSourceOperator,
			},
			IdempotencyKey: "key-" + fake.UUID().V4(),
			InputHash:      hashBytes([]byte(fake.UUID().V4())),
			InputJSON: json.RawMessage(
				fmt.Sprintf(`{"accountId":%q}`, "account-"+fake.UUID().V4()),
			),
			CreatedAt:   now,
			UpdatedAt:   now,
			QueuedAt:    now,
			MaxAttempts: 3,
		}
	}
	registerHandler := func(t *testing.T, registry *Registry, jobType JobType, opts TypedHandlerSpec[input, result, progress]) {
		t.Helper()
		opts.JobType = jobType
		if opts.Run == nil && opts.RunJob == nil {
			opts.Run = func(_ context.Context, value input, setProgress func(progress) error) (result, error) {
				require.NoError(t, setProgress(progress{Stage: "importing"}))
				return result{Imported: len(value.AccountID)}, nil
			}
		}
		require.NoError(t, RegisterTypedHandler(registry, opts))
	}
	makeService := func(
		t *testing.T,
		store *Store,
		registry *Registry,
		publisher dispatchPublisher,
		now time.Time,
	) *Service {
		t.Helper()
		service, err := NewService(ServiceDeps{
			Store:       store,
			Registry:    registry,
			Publisher:   publisher,
			IDGenerator: ident.NewMockGenerator(),
			Clock:       func() time.Time { return now },
		})
		require.NoError(t, err)
		return service
	}

	t.Run("registry and payload helpers support typed finance execution", func(t *testing.T) {
		registry := NewRegistry()
		require.Error(t, registry.Register(nil))
		require.Error(
			t,
			RegisterTypedHandler[input, result, progress](
				nil,
				TypedHandlerSpec[input, result, progress]{},
			),
		)
		require.Error(
			t,
			RegisterTypedHandler(
				registry,
				TypedHandlerSpec[input, result, progress]{JobType: JobType("finance.invalid")},
			),
		)
		registerHandler(
			t,
			registry,
			JobType("finance.csv_import"),
			TypedHandlerSpec[input, result, progress]{
				SupportsCancel: true, SupportsRetry: true, MaxAttempts: 4,
			},
		)
		require.Error(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
			JobType: JobType(
				"finance.csv_import",
			), Run: func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
		}))
		_, err := registry.Handler(JobType("finance.missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
		_, err = registry.handlerByExecutionKind(executionKind("finance.missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
		var nilRegistry *Registry
		_, err = nilRegistry.Handler(JobType("finance.missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)

		handler, err := registry.handlerByExecutionKind(
			executionKind("finance.csv_import"),
		)
		require.NoError(t, err)
		payload, err := EncodeJobPayload(input{AccountID: "account-" + fake.UUID().V4()})
		require.NoError(t, err)
		var recorded json.RawMessage
		output, err := handler.execute(
			t.Context(),
			Job{InputJSON: payload},
			func(value json.RawMessage) error { recorded = value; return nil },
		)
		require.NoError(t, err)
		assert.JSONEq(t, `{"stage":"importing"}`, string(recorded))
		encodedOutput, err := resultJSONFromValue(output)
		require.NoError(t, err)
		assert.Contains(t, string(encodedOutput), "imported")
		_, err = handler.execute(
			t.Context(),
			Job{InputJSON: json.RawMessage("{")},
			func(json.RawMessage) error { return nil },
		)
		require.Error(t, err)

		assert.Equal(
			t,
			defaultWorkerPollInterval,
			normalizeWorkerConfig(WorkerConfig{}).PollInterval,
		)
		assert.Equal(t, defaultWorkerMaxAttempts, normalizeWorkerConfig(WorkerConfig{}).MaxAttempts)
		assert.Equal(t, defaultWorkerDrainTimeout, normalizeWorkerConfig(WorkerConfig{}).DrainTimeout)
		assert.Equal(t, defaultListLimit, normalizeListParams(ListParams{}).Limit)
		assert.Equal(
			t,
			maxListLimit,
			normalizeListParams(ListParams{Limit: maxListLimit + 1}).Limit,
		)
		assert.True(t, IsIdempotencyConflict(&idempotencyConflictError{key: fake.UUID().V4()}))
		assert.False(t, IsIdempotencyConflict(errors.New(fake.Lorem().Sentence(3))))
		assert.Nil(t, jobErrorFromExecution(nil))
		assert.Equal(
			t,
			"job execution failed",
			jobErrorFromExecution(errors.New("sql "+fake.Lorem().Sentence(3))).Details,
		)
		assert.Equal(
			t,
			"job execution failed",
			jobErrorFromExecution(errors.New("SELECT "+fake.Lorem().Sentence(3))).Details,
		)
		assert.Equal(t, "…", truncateBounded(fake.Lorem().Sentence(8), len("…")))
		assert.Equal(t, "x", truncateBounded("xx", 1))
	})

	t.Run("store persists generic job state, schedules, and safe transitions", func(t *testing.T) {
		_, err := NewStore(nil, fake.UUID().V4(), StoreOpts{})
		require.Error(t, err)
		db, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		_, err = NewStore(db, " ", StoreOpts{})
		require.Error(t, err)
		assert.True(t, isSQLiteDSN("file:"+fake.UUID().V4()))
		assert.False(t, isSQLiteDSN("postgres://"+fake.UUID().V4()))

		store, _ := makeStore(t)
		now := time.Now()
		job := makeJob(now)
		created, err := store.Create(t.Context(), job)
		require.NoError(t, err)
		assert.Equal(t, job.ID, created.ID)
		loaded, err := store.Get(t.Context(), " "+job.ID+" ")
		require.NoError(t, err)
		assert.Equal(t, job.ID, loaded.ID)
		_, err = store.Get(t.Context(), fake.UUID().V4())
		require.ErrorIs(t, err, ErrJobNotFound)

		idempotent := makeJob(now.Add(time.Second))
		first, createdNew, err := store.CreateIdempotent(t.Context(), idempotent)
		require.NoError(t, err)
		assert.True(t, createdNew)
		second, createdNew, err := store.CreateIdempotent(t.Context(), idempotent)
		require.NoError(t, err)
		assert.False(t, createdNew)
		assert.Equal(t, first.ID, second.ID)
		conflict := idempotent
		conflict.ID = "job-" + fake.UUID().V4()
		conflict.InputHash = hashBytes([]byte(fake.UUID().V4()))
		_, _, err = store.CreateIdempotent(t.Context(), conflict)
		require.True(t, IsIdempotencyConflict(err))
		conflict.IdempotencyKey = " "
		_, _, err = store.CreateIdempotent(t.Context(), conflict)
		require.ErrorIs(t, err, ErrNoIdempotency)

		claimed, err := store.ClaimQueued(
			t.Context(),
			job.ID,
			"worker-"+fake.UUID().V4(),
			now.Add(time.Minute),
		)
		require.NoError(t, err)
		assert.Equal(t, JobStatusRunning, claimed.Status)
		_, err = store.ClaimQueued(t.Context(), job.ID, fake.UUID().V4(), now.Add(time.Minute))
		require.ErrorIs(t, err, ErrJobNotQueued)
		require.Error(
			t,
			store.MarkSucceeded(
				t.Context(),
				job.ID,
				fake.UUID().V4(),
				result{Imported: 1},
				time.Time{},
			),
		)
		require.NoError(
			t,
			store.UpdateProgress(
				t.Context(),
				job.ID,
				json.RawMessage(`{"stage":"saved"}`),
				now.Add(2*time.Minute),
			),
		)
		require.NoError(
			t,
			store.MarkSucceeded(
				t.Context(),
				job.ID,
				fake.UUID().V4(),
				[]byte(`{"imported":1}`),
				now.Add(3*time.Minute),
			),
		)
		succeeded, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusSucceeded, succeeded.Status)
		require.Error(t, store.MarkCanceled(t.Context(), job.ID, time.Time{}))
		require.NoError(t, store.MarkCanceled(t.Context(), job.ID, now.Add(4*time.Minute)))
		require.Error(t, store.MarkFailed(t.Context(), job.ID, fake.UUID().V4(), nil, time.Time{}))
		require.NoError(
			t,
			store.MarkFailed(
				t.Context(),
				job.ID,
				fake.UUID().V4(),
				&JobError{
					Code:    fake.UUID().V4(),
					Summary: fake.Lorem().Sentence(3),
					Details: fake.Lorem().Paragraph(20),
				},
				now.Add(5*time.Minute),
			),
		)

		page, err := store.List(
			t.Context(),
			ListParams{
				Statuses: []JobStatus{JobStatusFailed},
				JobTypes: []JobType{job.JobType},
				Sources:  []RequesterSource{RequesterSourceOperator},
				Limit:    1,
			},
		)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		_, err = store.List(t.Context(), ListParams{Cursor: "%"})
		require.Error(t, err)
		found, err := store.FindByIdempotencyKey(
			t.Context(),
			idempotent.Requester,
			idempotent.JobType,
			idempotent.IdempotencyKey,
		)
		require.NoError(t, err)
		assert.Equal(t, idempotent.ID, found.ID)
		_, err = store.FindByIdempotencyKey(
			t.Context(),
			idempotent.Requester,
			idempotent.JobType,
			" ",
		)
		require.ErrorIs(t, err, ErrNoIdempotency)

		running := makeJob(now.Add(6 * time.Minute))
		running.Status, running.AttemptCount = JobStatusRunning, 1
		exhausted := makeJob(now.Add(7 * time.Minute))
		exhausted.Status, exhausted.AttemptCount = JobStatusRunning, 3
		_, err = store.Create(t.Context(), running)
		require.NoError(t, err)
		_, err = store.Create(t.Context(), exhausted)
		require.NoError(t, err)
		require.NoError(t, store.RecoverStaleRunning(t.Context(), now.Add(8*time.Minute), 3))
		requeued, err := store.Get(t.Context(), running.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusQueued, requeued.Status)
		failed, err := store.Get(t.Context(), exhausted.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, failed.Status)

		due := now.Add(-time.Minute)
		schedule := Schedule{
			ID:        "schedule-" + fake.UUID().V4(),
			JobType:   JobType("finance.fx_rates_sync"),
			Requester: Requester{UserID: fake.UUID().V4(), Source: RequesterSourceOperator},
			InputJSON: json.RawMessage(`{}`),
			Interval:  time.Hour,
			NextRunAt: &due,
			Enabled:   true,
		}
		require.NoError(t, store.UpsertSchedule(t.Context(), schedule))
		dueSchedules, err := store.ListDueSchedules(t.Context(), now)
		require.NoError(t, err)
		require.Len(t, dueSchedules, 1)
		loadedSchedule, err := store.GetSchedule(t.Context(), schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, schedule.ID, loadedSchedule.ID)
		schedule.Enabled = false
		require.NoError(t, store.UpsertSchedule(t.Context(), schedule))
		dueSchedules, err = store.ListDueSchedules(t.Context(), now)
		require.NoError(t, err)
		assert.Empty(t, dueSchedules)
		require.Error(t, validateScheduleTimestamps(Schedule{Enabled: true}))
		require.Error(t, validateRequiredTimestamp("now", time.Time{}))
		assert.Nil(t, (*StoreTx)(nil).SQLTx())
		require.NoError(
			t,
			store.WithTx(
				t.Context(),
				func(tx *StoreTx) error {
					_, createErr := tx.Create(t.Context(), makeJob(now.Add(9*time.Minute)))
					return createErr
				},
			),
		)
	})

	t.Run("service and scheduler preserve durable finance job semantics", func(t *testing.T) {
		now := time.Now()
		transactionContext := context.WithoutCancel(t.Context())
		store, dsn := makeStore(t)
		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		require.NoError(t, appdispatch.AutoMigrate(
			t.Context(),
			appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "finance_jobs_"},
			sqlDB,
		))
		registry := NewRegistry()
		registerHandler(
			t,
			registry,
			JobType("finance.bank_connection_sync"),
			TypedHandlerSpec[input, result, progress]{SupportsCancel: true, SupportsRetry: true},
		)
		publisher, err := appdispatch.NewPublisher(
			appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "finance_jobs_"},
			sqlDB,
			slog.Default(),
		)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		service := makeService(t, store, registry, publisher, now)
		job, err := service.Enqueue(
			transactionContext,
			EnqueueParams{
				JobType: JobType("finance.bank_connection_sync"),
				Requester: Requester{
					UserID: " " + fake.UUID().V4() + " ",
					Source: RequesterSourceOperator,
				},
				Input:          input{AccountID: fake.UUID().V4()},
				IdempotencyKey: fake.UUID().V4(),
			},
		)
		require.NoError(t, err)
		var envelopePayload []byte
		require.NoError(t, sqlDB.QueryRowContext(
			t.Context(),
			`SELECT payload FROM finance_jobs_app_dispatch_messages WHERE topic=?`,
			jobExecutionTopic,
		).Scan(&envelopePayload))
		var envelope executionEnvelope
		require.NoError(t, json.Unmarshal(envelopePayload, &envelope))
		assert.Equal(t, jobEnvelopeVersion, envelope.Version)
		assert.Equal(t, executionKind(job.JobType), envelope.Kind)
		assert.Equal(t, job.ID, envelope.ObservableJobID)
		canceled, err := service.Cancel(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusCanceled, canceled.Status)
		retried, err := service.Retry(transactionContext, job.ID)
		require.NoError(t, err)
		assert.NotEqual(t, job.ID, retried.ID)
		_, err = service.Get(t.Context(), fake.UUID().V4())
		require.Error(t, err)
		_, err = service.Enqueue(
			transactionContext,
			EnqueueParams{
				JobType:   job.JobType,
				Requester: Requester{UserID: fake.UUID().V4()},
				Input:     input{},
			},
		)
		require.Error(t, err)
		_, err = service.Enqueue(
			t.Context(),
			EnqueueParams{
				JobType:   JobType("finance.missing"),
				Requester: Requester{UserID: fake.UUID().V4(), Source: RequesterSourceOperator},
				Input:     input{},
			},
		)
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
		_, err = NewService(ServiceDeps{})
		require.Error(t, err)

		due := now.Add(-time.Minute)
		require.NoError(
			t,
			store.UpsertSchedule(
				t.Context(),
				Schedule{
					ID:        "schedule-" + fake.UUID().V4(),
					JobType:   job.JobType,
					Requester: job.Requester,
					InputJSON: job.InputJSON,
					Interval:  time.Hour,
					NextRunAt: &due,
					Enabled:   true,
				},
			),
		)
		scheduler, err := NewScheduler(
			SchedulerDeps{Store: store, Service: service, Clock: func() time.Time { return now }},
		)
		require.NoError(t, err)
		count, err := scheduler.EnqueueDue(transactionContext)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		count, err = scheduler.EnqueueDue(transactionContext)
		require.NoError(t, err)
		assert.Zero(t, count)
		_, err = NewScheduler(SchedulerDeps{})
		require.Error(t, err)
		assert.NotEmpty(t, scheduleJobIdempotencyKey(fake.UUID().V4(), due))
	})

	t.Run(
		"worker executes, records failures, and protects terminal duplicate deliveries",
		func(t *testing.T) {
			now := time.Now()
			store, dsn := makeStore(t)
			registry := NewRegistry()
			registerHandler(
				t,
				registry,
				JobType("finance.csv_import"),
				TypedHandlerSpec[input, result, progress]{},
			)
			worker, err := NewWorker(
				WorkerDeps{
					Store:         store,
					Registry:      registry,
					Logger:        slog.Default(),
					Clock:         func() time.Time { return now },
					WorkerID:      "worker-" + fake.UUID().V4(),
					RouterFactory: makeRouterFactory(t, dsn, "finance_jobs_"),
				},
			)
			require.NoError(t, err)
			job := makeJob(now)
			_, err = store.Create(t.Context(), job)
			require.NoError(t, err)
			require.NoError(t, worker.ProcessJob(t.Context(), job.ID))
			executed, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusSucceeded, executed.Status)
			require.NoError(t, worker.ProcessJob(t.Context(), job.ID))
			for _, status := range []JobStatus{JobStatusRunning, JobStatusFailed, JobStatusCanceled} {
				duplicate := makeJob(now.Add(time.Duration(len(status)) * time.Minute))
				duplicate.Status = status
				_, err = store.Create(t.Context(), duplicate)
				require.NoError(t, err)
				require.NoError(t, worker.ProcessJob(t.Context(), duplicate.ID))
				persisted, getErr := store.Get(t.Context(), duplicate.ID)
				require.NoError(t, getErr)
				assert.Equal(t, status, persisted.Status)
			}
			_, err = NewWorker(WorkerDeps{})
			require.Error(t, err)
			require.NoError(t, worker.ProcessJob(t.Context(), fake.UUID().V4()))
			require.NoError(t, worker.Stop(t.Context()))

			unknown := makeJob(now.Add(time.Minute))
			unknown.JobType = JobType("finance.unknown")
			_, err = store.Create(t.Context(), unknown)
			require.NoError(t, err)
			require.NoError(t, worker.ProcessJob(t.Context(), unknown.ID))
			persistedUnknown, err := store.Get(t.Context(), unknown.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusFailed, persistedUnknown.Status)

			mockStore := newMockworkerStore(t)
			mockStore.EXPECT().
				RecoverStaleRunning(mock.Anything, mock.Anything, mock.Anything).
				Return(errors.New(fake.Lorem().Sentence(3))).
				Times(3)
			brokenWorker, err := NewWorker(
				WorkerDeps{
					Store:         mockStore,
					Registry:      registry,
					Config:        WorkerConfig{Enabled: true},
					RouterFactory: makeRouterFactory(t, dsn, "finance_jobs_broken_"),
				},
			)
			require.NoError(t, err)
			require.Error(t, brokenWorker.Start(t.Context()))
			require.Error(t, brokenWorker.Run(t.Context()))
			require.Error(t, brokenWorker.RunOnce(t.Context()))
		},
	)

	t.Run("worker claims lifecycle before recovery side effects", func(t *testing.T) {
		for _, owner := range []struct {
			name string
			run  func(*Worker, context.Context) error
		}{
			{name: "Start", run: func(worker *Worker, ctx context.Context) error { return worker.Start(ctx) }},
			{name: "Run", run: func(worker *Worker, ctx context.Context) error { return worker.Run(ctx) }},
			{name: "RunOnce", run: func(worker *Worker, ctx context.Context) error { return worker.RunOnce(ctx) }},
		} {
			t.Run(owner.name, func(t *testing.T) {
				_, dsn := makeStore(t)
				mockStore := newMockworkerStore(t)
				recoveryStarted := make(chan struct{})
				releaseRecovery := make(chan struct{})
				recoveryFailure := errors.New(fake.Lorem().Sentence(3))
				mockStore.EXPECT().
					RecoverStaleRunning(mock.Anything, mock.Anything, mock.Anything).
					Run(func(context.Context, time.Time, int) {
						close(recoveryStarted)
						<-releaseRecovery
					}).
					Return(recoveryFailure).
					Once()
				worker, err := NewWorker(WorkerDeps{
					Store:         mockStore,
					Registry:      NewRegistry(),
					Logger:        slog.New(slog.DiscardHandler),
					Config:        WorkerConfig{Enabled: true, PollInterval: time.Millisecond},
					RouterFactory: makeRouterFactory(t, dsn, "finance_jobs_lifecycle_"),
				})
				require.NoError(t, err)

				ownerResult := make(chan error, 1)
				go func() { ownerResult <- owner.run(worker, t.Context()) }()
				<-recoveryStarted

				for _, run := range []func(context.Context) error{worker.Start, worker.Run, worker.RunOnce} {
					require.EqualError(t, run(t.Context()), "jobs worker is already running")
				}

				close(releaseRecovery)
				require.ErrorIs(t, <-ownerResult, recoveryFailure)
				worker.mu.Lock()
				assert.Nil(t, worker.lifecycle)
				worker.mu.Unlock()

				mockStore.EXPECT().
					RecoverStaleRunning(mock.Anything, mock.Anything, mock.Anything).
					Return(nil).
					Once()
				require.NoError(t, worker.RunOnce(t.Context()))
			})
		}
	})

	t.Run(
		"worker executor handles non-observable and observable dispatch outcomes",
		func(t *testing.T) {
			registry := NewRegistry()
			registerHandler(
				t,
				registry,
				JobType("finance.executor"),
				TypedHandlerSpec[input, result, progress]{},
			)
			store := newMockworkerStore(t)
			executor := &workerExecutor{
				store:    store,
				registry: registry,
				logger:   slog.Default(),
				clock:    time.Now,
				workerID: fake.UUID().V4(),
			}
			payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
			require.NoError(t, err)
			require.NoError(
				t,
				executor.processEnvelope(
					t.Context(),
					executionEnvelope{
						Kind:    executionKind("finance.executor"),
						Payload: payload,
					},
				),
			)
			store.EXPECT().
				Get(mock.Anything, mock.Anything).
				Return((*Job)(nil), ErrJobNotFound).
				Once()
			store.EXPECT().
				UpdateProgress(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			store.EXPECT().
				persistTerminalState(mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			require.NoError(
				t,
				executor.processEnvelope(
					t.Context(),
					executionEnvelope{
						Kind:            executionKind("finance.executor"),
						Payload:         payload,
						ObservableJobID: fake.UUID().V4(),
					},
				),
			)
			store.EXPECT().
				persistTerminalState(mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			require.NoError(
				t,
				executor.processEnvelope(
					t.Context(),
					executionEnvelope{
						Kind:            executionKind("finance.missing"),
						ObservableJobID: fake.UUID().V4(),
					},
				),
			)
			store.EXPECT().
				Get(mock.Anything, mock.Anything).
				Return((*Job)(nil), errors.New(fake.Lorem().Sentence(3))).
				Once()
			_, _, err = executor.prepareObservableJob(t.Context(), fake.UUID().V4())
			require.Error(t, err)

			queued := makeJob(time.Now())
			store.EXPECT().Get(mock.Anything, queued.ID).Return(&queued, nil).Once()
			store.EXPECT().ClaimQueued(mock.Anything, queued.ID, executor.workerID, mock.Anything).
				Return(nil, ErrJobNotQueued).Once()
			claimed, skip, err := executor.prepareObservableJob(t.Context(), queued.ID)
			require.NoError(t, err)
			assert.Nil(t, claimed)
			assert.True(t, skip)
		},
	)

	t.Run("worker executor retries terminal persistence before acknowledging", func(t *testing.T) {
		registry := NewRegistry()
		registerHandler(
			t,
			registry,
			JobType("finance.persistence_retry"),
			TypedHandlerSpec[input, result, progress]{
				Run: func(_ context.Context, value input, _ func(progress) error) (result, error) {
					return result{Imported: len(value.AccountID)}, nil
				},
			},
		)
		store := newMockworkerStore(t)
		executor := &workerExecutor{
			store:    store,
			registry: registry,
			logger:   slog.New(slog.DiscardHandler),
			clock:    time.Now,
			workerID: fake.UUID().V4(),
		}
		job := makeJob(time.Now())
		job.JobType = JobType("finance.persistence_retry")
		payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
		require.NoError(t, err)
		store.EXPECT().Get(mock.Anything, job.ID).Return(&job, nil).Once()
		store.EXPECT().ClaimQueued(mock.Anything, job.ID, executor.workerID, mock.Anything).Return(&job, nil).Once()
		store.EXPECT().
			persistTerminalState(mock.Anything, job.ID, mock.Anything).
			Return(errors.New(fake.Lorem().Sentence(3))).Once()
		store.EXPECT().
			persistTerminalState(mock.Anything, job.ID, mock.Anything).
			Return(nil).Once()

		require.NoError(t, executor.processEnvelope(t.Context(), executionEnvelope{
			Kind:            executionKind(job.JobType),
			Payload:         payload,
			ObservableJobID: job.ID,
		}))
	})

	t.Run("worker executor retries the prepared terminal outcome until persistence recovers", func(t *testing.T) {
		registry := NewRegistry()
		jobType := JobType("finance.persistence_permanent")
		registerHandler(t, registry, jobType, TypedHandlerSpec[input, result, progress]{
			Run: func(_ context.Context, value input, _ func(progress) error) (result, error) {
				return result{Imported: len(value.AccountID)}, nil
			},
		})
		store := newMockworkerStore(t)
		executor := &workerExecutor{
			store:    store,
			registry: registry,
			logger:   slog.New(slog.DiscardHandler),
			clock:    time.Now,
			workerID: fake.UUID().V4(),
		}
		job := makeJob(time.Now())
		job.JobType = jobType
		payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
		require.NoError(t, err)
		store.EXPECT().Get(mock.Anything, job.ID).Return(&job, nil).Once()
		store.EXPECT().ClaimQueued(mock.Anything, job.ID, executor.workerID, mock.Anything).Return(&job, nil).Once()
		store.EXPECT().
			persistTerminalState(mock.Anything, job.ID, mock.Anything).
			Return(errors.New(fake.Lorem().Sentence(3))).Once()
		store.EXPECT().
			persistTerminalState(mock.Anything, job.ID, mock.Anything).
			Return(nil).Once()

		require.NoError(t, executor.processEnvelope(t.Context(), executionEnvelope{
			Kind: executionKind(job.JobType), Payload: payload, ObservableJobID: job.ID,
		}))
	})

	t.Run("live worker retains terminal outcomes across polling windows and respects drain bounds", func(t *testing.T) {
		const pollInterval = 20 * time.Millisecond
		makeLiveWorker := func(
			t *testing.T,
			jobType JobType,
			drainTimeout time.Duration,
			executions *atomic.Int32,
			run func(context.Context, input, func(progress) error) (any, error),
			blockTerminalStatus JobStatus,
		) (*Store, *Worker, *appdispatch.RouterFactory, *sql.DB, Job, string, string) {
			t.Helper()
			store, dsn := makeStore(t)
			dispatchDB, err := sqlconn.Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, dispatchDB.Close()) })
			config := appdispatch.Config{
				DatabaseDSN: dsn, TablePrefix: "finance_jobs_", PollInterval: pollInterval,
			}
			require.NoError(t, appdispatch.AutoMigrate(t.Context(), config, dispatchDB))
			publisher, err := appdispatch.NewPublisher(config, dispatchDB, slog.New(slog.DiscardHandler))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, publisher.Close()) })
			factory, err := appdispatch.NewRouterFactory(
				config, dispatchDB, publisher, slog.New(slog.DiscardHandler),
			)
			require.NoError(t, err)
			registry := NewRegistry()
			if run == nil {
				run = func(_ context.Context, value input, _ func(progress) error) (any, error) {
					executions.Add(1)
					return result{Imported: len(value.AccountID)}, nil
				}
			}
			require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, any, progress]{
				JobType: jobType,
				Run:     run,
			}))
			workerID := fake.UUID().V4()
			job := makeJob(time.Now())
			job.JobType = jobType
			payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
			require.NoError(t, err)
			job.InputJSON = payload
			_, err = store.Create(t.Context(), job)
			require.NoError(t, err)
			triggerName := ""
			if blockTerminalStatus != "" {
				triggerName = "terminal_failure_" + fake.UUID().V4()
				require.NoError(t, store.db.Exec(
					`CREATE TRIGGER "`+triggerName+`" BEFORE UPDATE OF status ON "`+store.tableName+
						`" WHEN NEW.status = '`+string(blockTerminalStatus)+`' BEGIN SELECT RAISE(ABORT, 'terminal persistence unavailable'); END`,
				).Error)
			}
			worker, err := NewWorker(WorkerDeps{
				Store:    store,
				Registry: registry,
				Logger:   slog.New(slog.DiscardHandler),
				WorkerID: workerID,
				Config: WorkerConfig{
					Enabled: true, PollInterval: pollInterval, DrainTimeout: drainTimeout,
				},
				RouterFactory: factory,
			})
			require.NoError(t, err)
			require.NoError(t, worker.Start(t.Context()))
			envelopePayload, err := json.Marshal(executionEnvelope{
				Version: jobEnvelopeVersion, Kind: executionKind(job.JobType),
				Payload: payload, ObservableJobID: job.ID,
			})
			require.NoError(t, err)
			require.NoError(t, publisher.Publish(
				t.Context(), appdispatch.NewMessage(jobExecutionTopic, envelopePayload),
			))
			return store, worker, factory, dispatchDB, job, triggerName, dsn
		}

		t.Run("recovers to terminal without redelivery or duplicate execution", func(t *testing.T) {
			var executions atomic.Int32
			store, worker, _, dispatchDB, job, triggerName, _ := makeLiveWorker(
				t, JobType("finance.persistence_recovery"), time.Second, &executions, nil, JobStatusSucceeded,
			)
			require.Eventually(t, func() bool { return executions.Load() == 1 }, time.Second, time.Millisecond)
			time.Sleep(6 * pollInterval)
			assert.Equal(t, int32(1), executions.Load())
			pending, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusRunning, pending.Status)
			var offset int64
			require.NoError(t, dispatchDB.QueryRowContext(
				t.Context(),
				`SELECT offset_acked FROM finance_jobs_app_dispatch_offsets WHERE topic=? AND consumer_group=?`,
				jobExecutionTopic,
				jobConsumerGroup,
			).Scan(&offset))
			assert.Zero(t, offset)
			require.NoError(t, store.db.Exec(`DROP TRIGGER "`+triggerName+`"`).Error)
			require.Eventually(t, func() bool {
				completed, getErr := store.Get(t.Context(), job.ID)
				return getErr == nil && completed.Status == JobStatusSucceeded
			}, 3*time.Second, time.Millisecond)
			require.Eventually(t, func() bool {
				queryErr := dispatchDB.QueryRowContext(
					t.Context(),
					`SELECT offset_acked FROM finance_jobs_app_dispatch_offsets WHERE topic=? AND consumer_group=?`,
					jobExecutionTopic,
					jobConsumerGroup,
				).Scan(&offset)
				return queryErr == nil && offset > 0
			}, time.Second, time.Millisecond)
			assert.Equal(t, int32(1), executions.Load())
			require.NoError(t, worker.Stop(t.Context()))
		})

		t.Run("persistent failure stops within the configured drain bound", func(t *testing.T) {
			const drainTimeout = 150 * time.Millisecond
			var executions atomic.Int32
			store, worker, _, dispatchDB, job, triggerName, _ := makeLiveWorker(
				t, JobType("finance.persistence_shutdown"), drainTimeout, &executions, nil, JobStatusSucceeded,
			)
			require.Eventually(t, func() bool { return executions.Load() == 1 }, time.Second, time.Millisecond)
			time.Sleep(6 * pollInterval)
			started := time.Now()
			require.NoError(t, worker.Stop(t.Context()))
			assert.Less(t, time.Since(started), 3*drainTimeout)
			assert.Equal(t, int32(1), executions.Load())
			pending, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusRunning, pending.Status)
			var offset int64
			require.NoError(t, dispatchDB.QueryRowContext(
				t.Context(),
				`SELECT offset_acked FROM finance_jobs_app_dispatch_offsets WHERE topic=? AND consumer_group=?`,
				jobExecutionTopic,
				jobConsumerGroup,
			).Scan(&offset))
			assert.Zero(t, offset)
			require.NoError(t, store.db.Exec(`DROP TRIGGER "`+triggerName+`"`).Error)
			require.NoError(t, store.RecoverStaleRunning(t.Context(), time.Now(), defaultWorkerMaxAttempts))
			recovered, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusQueued, recovered.Status)
		})

		t.Run("once bounds terminal persistence failure without leaking its worker lifecycle", func(t *testing.T) {
			const drainTimeout = 150 * time.Millisecond
			var executions atomic.Int32
			store, dsn := makeStore(t)
			dispatchDB, err := sqlconn.Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, dispatchDB.Close()) })
			config := appdispatch.Config{
				DatabaseDSN: dsn, TablePrefix: "finance_jobs_", PollInterval: pollInterval,
			}
			require.NoError(t, appdispatch.AutoMigrate(t.Context(), config, dispatchDB))
			publisher, err := appdispatch.NewPublisher(config, dispatchDB, slog.New(slog.DiscardHandler))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, publisher.Close()) })
			factory, err := appdispatch.NewRouterFactory(config, dispatchDB, publisher, slog.New(slog.DiscardHandler))
			require.NoError(t, err)
			registry := NewRegistry()
			jobType := JobType("finance.persistence_once")
			require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
				JobType: jobType,
				Run: func(_ context.Context, value input, _ func(progress) error) (result, error) {
					executions.Add(1)
					return result{Imported: len(value.AccountID)}, nil
				},
			}))
			job := makeJob(time.Now())
			job.JobType = jobType
			payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
			require.NoError(t, err)
			job.InputJSON = payload
			_, err = store.Create(t.Context(), job)
			require.NoError(t, err)
			triggerName := "terminal_failure_once_" + fake.UUID().V4()
			require.NoError(t, store.db.Exec(
				`CREATE TRIGGER "`+triggerName+`" BEFORE UPDATE OF status ON "`+store.tableName+
					`" WHEN NEW.status = 'succeeded' BEGIN SELECT RAISE(ABORT, 'terminal persistence unavailable'); END`,
			).Error)
			worker, err := NewWorker(WorkerDeps{
				Store:    store,
				Registry: registry,
				Logger:   slog.New(slog.DiscardHandler),
				WorkerID: fake.UUID().V4(),
				Config: WorkerConfig{
					Enabled: true, PollInterval: pollInterval, DrainTimeout: drainTimeout,
				},
				RouterFactory: factory,
			})
			require.NoError(t, err)
			envelopePayload, err := json.Marshal(executionEnvelope{
				Version:         jobEnvelopeVersion,
				Kind:            executionKind(job.JobType),
				Payload:         payload,
				ObservableJobID: job.ID,
			})
			require.NoError(t, err)
			require.NoError(t, publisher.Publish(
				t.Context(), appdispatch.NewMessage(jobExecutionTopic, envelopePayload),
			))

			started := time.Now()
			runDone := make(chan error, 1)
			go func() { runDone <- worker.RunOnce(t.Context()) }()
			require.Eventually(t, func() bool { return executions.Load() == 1 }, time.Second, time.Millisecond)
			select {
			case runErr := <-runDone:
				require.NoError(t, runErr)
			case <-time.After(2*pollInterval + 4*drainTimeout):
				t.Fatal("RunOnce did not return after its bounded drain")
			}
			assert.Less(t, time.Since(started), 2*pollInterval+4*drainTimeout)
			assert.Equal(t, int32(1), executions.Load())
			worker.mu.Lock()
			assert.Nil(t, worker.lifecycle)
			worker.mu.Unlock()

			pending, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusRunning, pending.Status)
			var offset int64
			require.NoError(t, dispatchDB.QueryRowContext(
				t.Context(),
				`SELECT offset_acked FROM finance_jobs_app_dispatch_offsets WHERE topic=? AND consumer_group=?`,
				jobExecutionTopic,
				jobConsumerGroup,
			).Scan(&offset))
			assert.Zero(t, offset)
			require.NoError(t, store.RecoverStaleRunning(t.Context(), time.Now(), defaultWorkerMaxAttempts))
			recovered, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusQueued, recovered.Status)
		})

		t.Run("result encoding failure becomes durable failed state before acknowledgement", func(t *testing.T) {
			const drainTimeout = time.Second
			var executions atomic.Int32
			store, worker, factory, dispatchDB, job, triggerName, dsn := makeLiveWorker(
				t,
				JobType("finance.result_encoding_failure"),
				drainTimeout,
				&executions,
				func(_ context.Context, _ input, _ func(progress) error) (any, error) {
					executions.Add(1)
					return func() {}, nil
				},
				JobStatusFailed,
			)
			require.Eventually(t, func() bool { return executions.Load() == 1 }, time.Second, time.Millisecond)
			time.Sleep(6 * pollInterval)
			assert.Equal(t, int32(1), executions.Load())
			pending, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusRunning, pending.Status)
			var offset int64
			require.NoError(t, dispatchDB.QueryRowContext(
				t.Context(),
				`SELECT offset_acked FROM finance_jobs_app_dispatch_offsets WHERE topic=? AND consumer_group=?`,
				jobExecutionTopic,
				jobConsumerGroup,
			).Scan(&offset))
			assert.Zero(t, offset)
			require.NoError(t, store.db.Exec(`DROP TRIGGER "`+triggerName+`"`).Error)
			require.Eventually(t, func() bool {
				completed, getErr := store.Get(t.Context(), job.ID)
				return getErr == nil && completed.Status == JobStatusFailed
			}, 3*time.Second, time.Millisecond)
			failed, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			require.NotNil(t, failed.Error)
			assert.Equal(t, "job_result_encoding_failed", failed.Error.Code)
			assert.Empty(t, failed.ResultJSON)
			require.Eventually(t, func() bool {
				queryErr := dispatchDB.QueryRowContext(
					t.Context(),
					`SELECT offset_acked FROM finance_jobs_app_dispatch_offsets WHERE topic=? AND consumer_group=?`,
					jobExecutionTopic,
					jobConsumerGroup,
				).Scan(&offset)
				return queryErr == nil && offset > 0
			}, time.Second, time.Millisecond)
			require.NoError(t, worker.Stop(t.Context()))
			restartDB, err := sqlconn.Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, restartDB.Close()) })
			restartedStore, err := NewStore(restartDB, dsn, StoreOpts{TablePrefix: "finance_jobs_"})
			require.NoError(t, err)
			restarted, err := NewWorker(WorkerDeps{
				Store:         restartedStore,
				Registry:      worker.registry,
				Logger:        slog.New(slog.DiscardHandler),
				WorkerID:      fake.UUID().V4(),
				Config:        WorkerConfig{Enabled: true, PollInterval: pollInterval, DrainTimeout: drainTimeout},
				RouterFactory: factory,
			})
			require.NoError(t, err)
			require.NoError(t, restarted.Start(t.Context()))
			time.Sleep(6 * pollInterval)
			assert.Equal(t, int32(1), executions.Load())
			afterRestart, err := restartedStore.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusFailed, afterRestart.Status)
			require.NoError(t, restarted.Stop(t.Context()))
		})
	})

	t.Run("worker executor stops terminal persistence retry when the drain context ends", func(t *testing.T) {
		registry := NewRegistry()
		jobType := JobType("finance.persistence_cancel")
		registerHandler(t, registry, jobType, TypedHandlerSpec[input, result, progress]{
			Run: func(_ context.Context, value input, _ func(progress) error) (result, error) {
				return result{Imported: len(value.AccountID)}, nil
			},
		})
		store := newMockworkerStore(t)
		executor := &workerExecutor{
			store:    store,
			registry: registry,
			logger:   slog.New(slog.DiscardHandler),
			clock:    time.Now,
			workerID: fake.UUID().V4(),
		}
		job := makeJob(time.Now())
		job.JobType = jobType
		payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
		require.NoError(t, err)
		store.EXPECT().Get(mock.Anything, job.ID).Return(&job, nil).Once()
		store.EXPECT().ClaimQueued(mock.Anything, job.ID, executor.workerID, mock.Anything).Return(&job, nil).Once()
		persistenceStarted := make(chan struct{})
		store.EXPECT().
			persistTerminalState(mock.Anything, job.ID, mock.Anything).
			Run(func(context.Context, string, terminalJobState) { close(persistenceStarted) }).
			Return(errors.New(fake.Lorem().Sentence(3))).Once()
		ctx, cancel := context.WithCancel(t.Context())
		result := make(chan error, 1)
		go func() {
			result <- executor.processEnvelope(ctx, executionEnvelope{
				Kind: executionKind(job.JobType), Payload: payload, ObservableJobID: job.ID,
			})
		}()
		<-persistenceStarted
		cancel()
		select {
		case resultErr := <-result:
			require.ErrorIs(t, resultErr, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("terminal persistence retry did not stop when its context ended")
		}
	})

	t.Run("store surfaces persistence failures without hiding them", func(t *testing.T) {
		store, _ := makeStore(t)
		now := time.Now()
		job := makeJob(now)
		require.NoError(t, store.db.Exec("DROP TABLE "+store.tableName).Error)
		_, err := store.Create(t.Context(), job)
		require.Error(t, err)
		_, _, err = store.CreateIdempotent(t.Context(), job)
		require.Error(t, err)
		_, err = store.Get(t.Context(), job.ID)
		require.Error(t, err)
		_, err = store.FindByIdempotencyKey(
			t.Context(),
			job.Requester,
			job.JobType,
			job.IdempotencyKey,
		)
		require.Error(t, err)
		_, err = store.List(t.Context(), ListParams{Limit: 1})
		require.Error(t, err)
		_, err = store.ClaimQueued(t.Context(), job.ID, fake.UUID().V4(), now)
		require.Error(t, err)
		require.Error(t, store.MarkSucceeded(t.Context(), job.ID, fake.UUID().V4(), result{}, now))
		require.Error(t, store.MarkCanceled(t.Context(), job.ID, now))
		require.Error(t, store.UpdateProgress(t.Context(), job.ID, json.RawMessage(`{}`), now))
		require.Error(t, store.MarkFailed(t.Context(), job.ID, fake.UUID().V4(), nil, now))
		require.Error(t, store.RecoverStaleRunning(t.Context(), now, 1))
		store, _ = makeStore(t)
		require.NoError(t, store.db.Exec("DROP TABLE "+store.scheduleTableName()).Error)
		_, err = store.GetSchedule(t.Context(), fake.UUID().V4())
		require.Error(t, err)
		_, err = store.ListDueSchedules(t.Context(), now)
		require.Error(t, err)
		require.Error(
			t,
			store.UpsertSchedule(t.Context(), Schedule{ID: fake.UUID().V4(), Enabled: true}),
		)
	})

	t.Run(
		"worker consumes finance dispatch messages through the durable transport",
		func(t *testing.T) {
			store, dsn := makeStore(t)
			sqlDB, err := store.db.DB()
			require.NoError(t, err)
			require.NoError(
				t,
				appdispatch.AutoMigrate(
					t.Context(),
					appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "finance_jobs_"},
					sqlDB,
				),
			)
			registry := NewRegistry()
			registerHandler(
				t,
				registry,
				JobType("finance.dispatch_import"),
				TypedHandlerSpec[input, result, progress]{},
			)
			payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
			require.NoError(t, err)
			now := time.Now()
			job := makeJob(now)
			job.JobType, job.InputJSON = JobType("finance.dispatch_import"), payload
			_, err = store.Create(t.Context(), job)
			require.NoError(t, err)
			publisher, err := appdispatch.NewPublisher(
				appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "finance_jobs_"},
				sqlDB,
				slog.Default(),
			)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, publisher.Close()) })
			factory, err := appdispatch.NewRouterFactory(
				appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "finance_jobs_", PollInterval: time.Millisecond},
				sqlDB,
				publisher,
				slog.Default(),
			)
			require.NoError(t, err)
			worker, err := NewWorker(WorkerDeps{
				Store: store, Registry: registry, RouterFactory: factory,
				Config: WorkerConfig{Enabled: true, PollInterval: time.Millisecond}, WorkerID: fake.UUID().V4(),
			})
			require.NoError(t, err)
			envelopePayload, err := json.Marshal(executionEnvelope{
				Version:         jobEnvelopeVersion,
				Kind:            executionKind(job.JobType),
				Payload:         payload,
				ObservableJobID: job.ID,
			})
			require.NoError(t, err)
			require.NoError(
				t,
				publisher.Publish(
					t.Context(),
					appdispatch.NewMessage(jobExecutionTopic, envelopePayload),
				),
			)
			require.NoError(t, worker.Start(t.Context()))
			require.Eventually(t, func() bool {
				completed, getErr := store.Get(t.Context(), job.ID)
				return getErr == nil && completed.Status == JobStatusSucceeded
			}, 5*time.Second, time.Millisecond)
			require.NoError(t, worker.Stop(t.Context()))
		},
	)

	t.Run(
		"registry scheduler and service expose validation and callback failures",
		func(t *testing.T) {
			transactionContext := context.WithoutCancel(t.Context())
			registry := NewRegistry()
			registerHandler(
				t,
				registry,
				JobType("finance.callback"),
				TypedHandlerSpec[input, result, progress]{
					DispatchKind: "finance.callback.dispatch",
					RunJob: func(_ context.Context, _ Job, value input, _ func(progress) error) (result, error) {
						return result{Imported: len(value.AccountID)}, nil
					},
					OnScheduled: func(context.Context, Job) error { return errors.New(fake.Lorem().Sentence(3)) },
				},
			)
			require.Error(
				t,
				RegisterTypedHandler(
					registry,
					TypedHandlerSpec[input, result, progress]{
						JobType:      JobType("finance.other"),
						DispatchKind: "finance.callback.dispatch",
						Run:          func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
					},
				),
			)
			handler, err := registry.Handler(JobType("finance.callback"))
			require.NoError(t, err)
			payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
			require.NoError(t, err)
			_, err = handler.execute(
				transactionContext,
				Job{InputJSON: payload},
				func(json.RawMessage) error { return nil },
			)
			require.NoError(t, err)

			store, dsn := makeStore(t)
			sqlDB, err := store.db.DB()
			require.NoError(t, err)
			dispatchConfig := appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "finance_jobs_"}
			require.NoError(t, appdispatch.AutoMigrate(t.Context(), dispatchConfig, sqlDB))
			_, err = NewService(ServiceDeps{Store: store})
			require.Error(t, err)
			_, err = NewService(ServiceDeps{Store: store, IDGenerator: ident.NewMockGenerator()})
			require.Error(t, err)
			publisher, err := appdispatch.NewPublisher(dispatchConfig, sqlDB, slog.Default())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, publisher.Close()) })
			service, err := NewService(
				ServiceDeps{
					Store:       store,
					IDGenerator: ident.NewMockGenerator(),
					Publisher:   publisher,
					Registry:    registry,
				},
			)
			require.NoError(t, err)
			_, err = sqlDB.Exec(
				`CREATE TRIGGER finance_jobs_dispatch_write_failure BEFORE INSERT ON finance_jobs_app_dispatch_messages BEGIN SELECT RAISE(ABORT, 'dispatch write failed'); END`,
			)
			require.NoError(t, err)
			_, err = service.Enqueue(
				transactionContext,
				EnqueueParams{
					JobType:   JobType("finance.callback"),
					Requester: Requester{UserID: fake.UUID().V4(), Source: RequesterSourceOperator},
					Input:     input{AccountID: fake.UUID().V4()},
				},
			)
			require.Error(t, err)
			_, err = sqlDB.Exec(`DROP TRIGGER finance_jobs_dispatch_write_failure`)
			require.NoError(t, err)

			now := time.Now()
			due := now.Add(-time.Minute)
			require.NoError(
				t,
				store.UpsertSchedule(
					t.Context(),
					Schedule{
						ID:      fake.UUID().V4(),
						JobType: JobType("finance.callback"),
						Requester: Requester{
							UserID: fake.UUID().V4(),
							Source: RequesterSourceOperator,
						},
						InputJSON: payload,
						Interval:  time.Hour,
						NextRunAt: &due,
						Enabled:   true,
					},
				),
			)
			service, err = NewService(
				ServiceDeps{
					Store:       store,
					IDGenerator: ident.NewMockGenerator(),
					Publisher:   publisher,
					Registry:    registry,
					Clock:       func() time.Time { return now },
				},
			)
			require.NoError(t, err)
			scheduler, err := NewScheduler(
				SchedulerDeps{
					Store:   store,
					Service: service,
					Clock:   func() time.Time { return now },
				},
			)
			require.NoError(t, err)
			_, err = scheduler.EnqueueDue(transactionContext)
			require.Error(t, err)
			_, err = NewScheduler(SchedulerDeps{Store: store})
			require.Error(t, err)
			created, err := scheduler.enqueueDueSchedule(t.Context(), Schedule{Enabled: false}, now)
			require.NoError(t, err)
			assert.False(t, created)
			created, err = scheduler.enqueueDueSchedule(t.Context(), Schedule{Enabled: true}, now)
			require.NoError(t, err)
			assert.False(t, created)
			_, err = service.Enqueue(
				transactionContext,
				EnqueueParams{
					JobType:   JobType("finance.callback"),
					Requester: Requester{UserID: fake.UUID().V4(), Source: RequesterSourceOperator},
					Input:     func() {},
				},
			)
			require.Error(t, err)
			result, err := service.List(t.Context(), ListParams{Limit: 1})
			require.NoError(t, err)
			require.Len(t, result.Items, 1)
			assert.Equal(t, JobType("finance.callback"), result.Items[0].JobType)
		},
	)
}
