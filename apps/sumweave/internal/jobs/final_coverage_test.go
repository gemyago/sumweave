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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFinalStoreCoverage(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T) *Store {
		t.Helper()
		dsn := fmt.Sprintf("file:final-jobs-%s?mode=memory&cache=shared", fake.UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := NewStore(db, dsn, StoreOpts{TablePrefix: "final_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}
	makeJob := func(now time.Time) Job {
		return Job{
			ID:          fake.UUID().V4(),
			JobType:     JobType("finance.final"),
			Status:      JobStatusQueued,
			Requester:   Requester{UserID: fake.UUID().V4(), Source: RequesterSourceOperator},
			InputJSON:   json.RawMessage(`{}`),
			CreatedAt:   now,
			UpdatedAt:   now,
			QueuedAt:    now,
			MaxAttempts: 2,
		}
	}

	t.Run("validates persistence boundaries and cursor data", func(t *testing.T) {
		store := makeStore(t)
		now := time.Now()
		job := makeJob(now)
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, store.WithTx(t.Context(), nil))
		_, err := store.ClaimQueued(t.Context(), job.ID, fake.UUID().V4(), time.Time{})
		require.Error(t, err)
		_, err = store.Create(t.Context(), job)
		require.NoError(t, err)
		require.NoError(t, store.MarkSucceeded(t.Context(), job.ID, fake.UUID().V4(), nil, now))
		require.NoError(t, store.MarkSucceeded(t.Context(), job.ID, fake.UUID().V4(), json.RawMessage(`{}`), now))
		require.NoError(
			t,
			store.MarkSucceeded(
				t.Context(), job.ID, fake.UUID().V4(), struct{ Value string }{Value: fake.UUID().V4()}, now,
			),
		)
		_, err = store.FindByIdempotencyKey(t.Context(), job.Requester, job.JobType, fake.UUID().V4())
		require.ErrorIs(t, err, ErrJobNotFound)
		_, err = store.List(t.Context(), ListParams{Cursor: encodeCursor(now, " ")})
		require.Error(t, err)
		_, _, err = decodeCursor("")
		require.Error(t, err)
		_, _, err = decodeCursor("bm90LWEtdGltZXxqb2I")
		require.Error(t, err)
		require.Error(t, validateScheduleTimestamps(Schedule{Enabled: false, NextRunAt: &now}))
		zero := time.Time{}
		require.Error(t, validateScheduleTimestamps(Schedule{LastEnqueuedAt: &zero}))
	})
}

func TestFinalServiceSchedulerWorkerCoverage(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T) *Store {
		t.Helper()
		dsn := fmt.Sprintf("file:final-worker-%s?mode=memory&cache=shared", fake.UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := NewStore(db, dsn, StoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}
	register := func(t *testing.T, registry *Registry, jobType JobType, run func(context.Context, struct{}, func(struct{}) error) (struct{}, error)) {
		t.Helper()
		require.NoError(
			t,
			RegisterTypedHandler(
				registry,
				TypedHandlerSpec[struct{}, struct{}, struct{}]{JobType: jobType, Run: run},
			),
		)
	}

	t.Run("service preserves conflict and store error outcomes", func(t *testing.T) {
		store := makeStore(t)
		registry := NewRegistry()
		jobType := JobType("finance.service.final")
		register(
			t,
			registry,
			jobType,
			func(context.Context, struct{}, func(struct{}) error) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		publisher := newMockdispatchPublisher(t)
		publisher.EXPECT().PublishInTx(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		service, err := NewService(ServiceDeps{
			Store: store, IDGenerator: ident.NewMockGenerator(), Publisher: publisher, Registry: registry,
		})
		require.NoError(t, err)
		job, err := service.Enqueue(t.Context(), EnqueueParams{
			JobType: jobType, Requester: Requester{Source: RequesterSourceOperator}, Input: struct{}{},
		})
		require.NoError(t, err)
		_, err = service.Cancel(t.Context(), job.ID)
		require.Error(t, err)
		_, err = service.Retry(t.Context(), job.ID)
		require.Error(t, err)
		_, err = NewService(ServiceDeps{
			Store: store, IDGenerator: ident.NewMockGenerator(), Publisher: publisher,
		})
		require.NoError(t, err)
	})

	t.Run("scheduler reports list and transaction failures", func(t *testing.T) {
		store := makeStore(t)
		registry := NewRegistry()
		register(
			t,
			registry,
			JobType("finance.scheduler.final"),
			func(context.Context, struct{}, func(struct{}) error) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		publisher := newMockdispatchPublisher(t)
		publisher.EXPECT().PublishInTx(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		service, err := NewService(ServiceDeps{
			Store: store, IDGenerator: ident.NewMockGenerator(), Publisher: publisher, Registry: registry,
		})
		require.NoError(t, err)
		scheduler, err := NewScheduler(SchedulerDeps{Store: store, Service: service})
		require.NoError(t, err)
		require.NoError(
			t,
			store.db.Exec("DROP TABLE "+store.scheduleTableName()).Error,
		)
		_, err = scheduler.EnqueueDue(t.Context())
		require.Error(t, err)
	})

	t.Run("worker handles disabled, consumer setup, and execution outcomes", func(t *testing.T) {
		registry := NewRegistry()
		register(
			t,
			registry,
			JobType("finance.worker.final"),
			func(context.Context, struct{}, func(struct{}) error) (struct{}, error) {
				return struct{}{}, errors.New(fake.Lorem().Sentence(3))
			},
		)
		store := newMockworkerStore(t)
		store.EXPECT().RecoverStaleRunning(mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)
		worker, err := NewWorker(WorkerDeps{Store: store, Registry: registry, Config: WorkerConfig{Enabled: false}})
		require.NoError(t, err)
		require.NoError(t, worker.Start(t.Context()))
		worker.config.Enabled = true
		require.Error(t, worker.Run(t.Context()))
		require.Error(t, worker.RunOnce(t.Context()))
		store.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, errors.New(fake.Lorem().Sentence(3))).Once()
		require.Error(t, worker.ProcessJob(t.Context(), fake.UUID().V4()))

		executor := &workerExecutor{
			store: store, registry: registry, logger: slog.Default(), clock: time.Now, workerID: fake.UUID().V4(),
		}
		require.Error(
			t,
			executor.processEnvelope(
				t.Context(), appdispatch.Envelope{Kind: appdispatch.ExecutionKind("finance.worker.final")},
			),
		)
		require.Error(
			t,
			executor.processEnvelope(
				t.Context(), appdispatch.Envelope{Kind: appdispatch.ExecutionKind("finance.unknown.final")},
			),
		)
	})
}
