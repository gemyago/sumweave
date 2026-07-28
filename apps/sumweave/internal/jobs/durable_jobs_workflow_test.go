package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	makeService := func(t *testing.T, store *Store, registry *Registry, now time.Time) (*Service, *mockdispatchPublisher) {
		t.Helper()
		publisher := newMockdispatchPublisher(t)
		publisher.EXPECT().
			PublishInTx(mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Maybe()
		service, err := NewService(ServiceDeps{
			Store:       store,
			Registry:    registry,
			Publisher:   publisher,
			IDGenerator: ident.NewMockGenerator(),
			Clock:       func() time.Time { return now },
		})
		require.NoError(t, err)
		return service, publisher
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
		_, err = registry.HandlerByExecutionKind(appdispatch.ExecutionKind("finance.missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
		var nilRegistry *Registry
		_, err = nilRegistry.Handler(JobType("finance.missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)

		handler, err := registry.HandlerByExecutionKind(
			appdispatch.ExecutionKind("finance.csv_import"),
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
		assert.Contains(t, string(output), "imported")
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
		store, _ := makeStore(t)
		registry := NewRegistry()
		registerHandler(
			t,
			registry,
			JobType("finance.bank_connection_sync"),
			TypedHandlerSpec[input, result, progress]{SupportsCancel: true, SupportsRetry: true},
		)
		service, _ := makeService(t, store, registry, now)
		job, err := service.Enqueue(
			t.Context(),
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
		canceled, err := service.Cancel(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusCanceled, canceled.Status)
		retried, err := service.Retry(t.Context(), job.ID)
		require.NoError(t, err)
		assert.NotEqual(t, job.ID, retried.ID)
		_, err = service.Get(t.Context(), fake.UUID().V4())
		require.Error(t, err)
		_, err = service.Enqueue(
			t.Context(),
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
		count, err := scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		count, err = scheduler.EnqueueDue(t.Context())
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
			store, _ := makeStore(t)
			registry := NewRegistry()
			registerHandler(
				t,
				registry,
				JobType("finance.csv_import"),
				TypedHandlerSpec[input, result, progress]{GuardDuplicateDelivery: true},
			)
			worker, err := NewWorker(
				WorkerDeps{
					Store:    store,
					Registry: registry,
					Logger:   slog.Default(),
					Clock:    func() time.Time { return now },
					WorkerID: "worker-" + fake.UUID().V4(),
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
			_, err = NewWorker(WorkerDeps{})
			require.Error(t, err)
			require.NoError(t, worker.ProcessJob(t.Context(), fake.UUID().V4()))
			require.Error(t, worker.ensureConsumer())
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
					Store:    mockStore,
					Registry: registry,
					Config:   WorkerConfig{Enabled: true},
				},
			)
			require.NoError(t, err)
			require.Error(t, brokenWorker.Start(t.Context()))
			require.Error(t, brokenWorker.Run(t.Context()))
			require.Error(t, brokenWorker.RunOnce(t.Context()))
		},
	)

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
					appdispatch.Envelope{
						Kind:    appdispatch.ExecutionKind("finance.executor"),
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
				MarkSucceeded(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			require.NoError(
				t,
				executor.processEnvelope(
					t.Context(),
					appdispatch.Envelope{
						Kind:            appdispatch.ExecutionKind("finance.executor"),
						Payload:         payload,
						ObservableJobID: fake.UUID().V4(),
					},
				),
			)
			store.EXPECT().
				MarkFailed(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			require.NoError(
				t,
				executor.processEnvelope(
					t.Context(),
					appdispatch.Envelope{
						Kind:            appdispatch.ExecutionKind("finance.missing"),
						ObservableJobID: fake.UUID().V4(),
					},
				),
			)
			store.EXPECT().
				Get(mock.Anything, mock.Anything).
				Return((*Job)(nil), errors.New(fake.Lorem().Sentence(3)))
			handler, err := registry.Handler(JobType("finance.executor"))
			require.NoError(t, err)
			_, _, err = executor.prepareObservableJob(t.Context(), fake.UUID().V4(), handler)
			require.Error(t, err)
		},
	)

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
			worker, err := NewWorker(
				WorkerDeps{
					Store:          store,
					Registry:       registry,
					DispatchDB:     sqlDB,
					DispatchConfig: DispatchConfig{DatabaseDSN: dsn, TablePrefix: "finance_jobs_"},
					Config:         WorkerConfig{Enabled: true, PollInterval: time.Millisecond},
					WorkerID:       fake.UUID().V4(),
				},
			)
			require.NoError(t, err)
			publisher, err := appdispatch.NewPublisher(
				appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "finance_jobs_"},
				sqlDB,
				slog.Default(),
			)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, publisher.Close()) })
			require.NoError(
				t,
				publisher.Publish(
					t.Context(),
					appdispatch.Envelope{
						Version:         appdispatch.EnvelopeVersionV1,
						Kind:            appdispatch.ExecutionKind(job.JobType),
						Payload:         payload,
						ObservableJobID: job.ID,
					},
				),
			)
			require.NoError(t, worker.RunOnce(t.Context()))
			completed, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusSucceeded, completed.Status)
			require.NoError(t, worker.Start(t.Context()))
			require.NoError(t, worker.Stop(t.Context()))
		},
	)

	t.Run(
		"registry scheduler and service expose validation and callback failures",
		func(t *testing.T) {
			registry := NewRegistry()
			registerHandler(
				t,
				registry,
				JobType("finance.callback"),
				TypedHandlerSpec[input, result, progress]{
					DispatchKind: appdispatch.ExecutionKind("finance.callback.dispatch"),
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
						DispatchKind: appdispatch.ExecutionKind("finance.callback.dispatch"),
						Run:          func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
					},
				),
			)
			handler, err := registry.Handler(JobType("finance.callback"))
			require.NoError(t, err)
			payload, err := EncodeJobPayload(input{AccountID: fake.UUID().V4()})
			require.NoError(t, err)
			_, err = handler.execute(
				t.Context(),
				Job{InputJSON: payload},
				func(json.RawMessage) error { return nil },
			)
			require.NoError(t, err)

			store, _ := makeStore(t)
			_, err = NewService(ServiceDeps{Store: store})
			require.Error(t, err)
			_, err = NewService(ServiceDeps{Store: store, IDGenerator: ident.NewMockGenerator()})
			require.Error(t, err)
			publisher := newMockdispatchPublisher(t)
			publisher.EXPECT().
				PublishInTx(mock.Anything, mock.Anything, mock.Anything).
				Return(errors.New(fake.Lorem().Sentence(3)))
			service, err := NewService(
				ServiceDeps{
					Store:       store,
					IDGenerator: ident.NewMockGenerator(),
					Publisher:   publisher,
					Registry:    registry,
				},
			)
			require.NoError(t, err)
			_, err = service.Enqueue(
				t.Context(),
				EnqueueParams{
					JobType:   JobType("finance.callback"),
					Requester: Requester{UserID: fake.UUID().V4(), Source: RequesterSourceOperator},
					Input:     input{AccountID: fake.UUID().V4()},
				},
			)
			require.Error(t, err)

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
			publisher = newMockdispatchPublisher(t)
			publisher.EXPECT().PublishInTx(mock.Anything, mock.Anything, mock.Anything).Return(nil)
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
			_, err = scheduler.EnqueueDue(t.Context())
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
				t.Context(),
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
