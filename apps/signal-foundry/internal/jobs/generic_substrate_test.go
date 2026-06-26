package jobs

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericSubstrate(t *testing.T) {
	fake := faker.New()
	type financeInput struct {
		AccountID string `json:"accountId"`
		Scope     string `json:"scope"`
	}
	type financeProgress struct {
		Stage string `json:"stage"`
	}
	type financeResult struct {
		Imported int `json:"imported"`
	}

	makeStore := func(t *testing.T) *Store {
		t.Helper()
		store, err := NewStore(
			filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			StoreOpts{TablePrefix: "generic_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}

	t.Run("stores generic json payloads and dispatches typed handlers", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		registry := NewRegistry()
		var runCalls atomic.Int64
		require.NoError(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[financeInput, financeResult, financeProgress]{
				JobType:        JobType("finance.csv_import"),
				MaxAttempts:    4,
				SupportsCancel: true,
				SupportsRetry:  true,
				Run: func(_ context.Context, input financeInput, setProgress func(financeProgress) error) (financeResult, error) {
					runCalls.Add(1)
					require.NoError(t, setProgress(financeProgress{Stage: "validated"}))
					return financeResult{Imported: len(input.AccountID)}, nil
				},
			},
		))
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   &publisherStub{},
			Clock:       func() time.Time { return now },
			Registry:    registry,
		})
		require.NoError(t, err)
		job, err := svc.Enqueue(t.Context(), EnqueueParams{
			JobType:        JobType("finance.csv_import"),
			Requester:      Requester{UserID: "user-" + fake.UUID().V4(), Source: RequesterSourceOperator},
			Input:          financeInput{AccountID: "acct-" + fake.UUID().V4(), Scope: "tenant"},
			IdempotencyKey: "key-" + fake.UUID().V4(),
			CorrelationID:  "corr-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		assert.Equal(t, JobType("finance.csv_import"), job.JobType)
		worker, err := NewWorker(WorkerDeps{
			Store:    store,
			Registry: registry,
			Clock:    func() time.Time { return now.Add(time.Minute) },
			WorkerID: "worker-generic",
			Config:   WorkerConfig{MaxConcurrentHistoricalBackfill: 2},
		})
		require.NoError(t, err)
		require.NoError(t, worker.ProcessJob(t.Context(), job.ID))
		persisted, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusSucceeded, persisted.Status)
		assert.Equal(t, 4, persisted.MaxAttempts)
		require.NotNil(t, persisted.ProgressJSON)
		require.NotNil(t, persisted.ResultJSON)
		progress, err := DecodeJobProgress[financeProgress](*persisted)
		require.NoError(t, err)
		assert.Equal(t, financeProgress{Stage: "validated"}, progress)
		result, err := DecodeJobResult[financeResult](*persisted)
		require.NoError(t, err)
		assert.Equal(t, len("acct-")+36, result.Imported)
		assert.EqualValues(t, 1, runCalls.Load())
	})

	t.Run("supports safe cancel and retry only for handlers that allow it", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		registry := NewRegistry()
		require.NoError(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[financeInput, financeResult, financeProgress]{
				JobType:        JobType("finance.bank_connection_sync"),
				MaxAttempts:    2,
				SupportsCancel: true,
				SupportsRetry:  true,
				Run: func(context.Context, financeInput, func(financeProgress) error) (financeResult, error) {
					return financeResult{Imported: 1}, nil
				},
			},
		))
		require.NoError(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[HistoricalRawCandleBackfillInput, HistoricalRawCandleBackfillResult, struct{}]{
				JobType:     JobTypeHistoricalRawCandleBackfill,
				MaxAttempts: 2,
				Run: func(context.Context, HistoricalRawCandleBackfillInput, func(struct{}) error) (HistoricalRawCandleBackfillResult, error) {
					return HistoricalRawCandleBackfillResult{}, nil
				},
			},
		))
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   &publisherStub{},
			Clock:       func() time.Time { return now },
			Registry:    registry,
		})
		require.NoError(t, err)
		job, err := svc.Enqueue(t.Context(), EnqueueParams{
			JobType:   JobType("finance.bank_connection_sync"),
			Requester: Requester{UserID: "user-" + fake.UUID().V4(), Source: RequesterSourceOperator},
			Input:     financeInput{AccountID: "acct-" + fake.UUID().V4()},
		})
		require.NoError(t, err)
		canceled, err := svc.Cancel(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusCanceled, canceled.Status)
		retried, err := svc.Retry(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusQueued, retried.Status)
		assert.NotEqual(t, job.ID, retried.ID)
		historical, err := store.Create(t.Context(), Job{
			ID:          fake.UUID().V4(),
			JobType:     JobTypeHistoricalRawCandleBackfill,
			Status:      JobStatusQueued,
			Requester:   Requester{UserID: "u", Source: RequesterSourceOperator},
			CreatedAt:   now,
			UpdatedAt:   now,
			QueuedAt:    now,
			MaxAttempts: 2,
		})
		require.NoError(t, err)
		_, err = svc.Cancel(t.Context(), historical.ID)
		require.Error(t, err)
	})

	t.Run("scheduler enqueues due windows once without replaying immediate work through polling", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		registry := NewRegistry()
		var historicalCalls atomic.Int64
		var financeCalls atomic.Int64
		require.NoError(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[financeInput, financeResult, financeProgress]{
				JobType:     JobType("finance.fx_rates_sync"),
				MaxAttempts: 3,
				Run: func(context.Context, financeInput, func(financeProgress) error) (financeResult, error) {
					financeCalls.Add(1)
					return financeResult{Imported: 2}, nil
				},
			},
		))
		require.NoError(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[HistoricalRawCandleBackfillInput, HistoricalRawCandleBackfillResult, struct{}]{
				JobType:     JobTypeHistoricalRawCandleBackfill,
				MaxAttempts: 3,
				Run: func(context.Context, HistoricalRawCandleBackfillInput, func(struct{}) error) (HistoricalRawCandleBackfillResult, error) {
					historicalCalls.Add(1)
					return HistoricalRawCandleBackfillResult{IngestionRunID: "run"}, nil
				},
			},
		))
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   &publisherStub{},
			Clock:       func() time.Time { return now },
			Registry:    registry,
		})
		require.NoError(t, err)
		require.NoError(t, store.UpsertSchedule(t.Context(), Schedule{
			ID:        "fx-daily",
			JobType:   JobType("finance.fx_rates_sync"),
			Requester: Requester{UserID: "system", Source: RequesterSourceOperator},
			Interval:  time.Hour,
			NextRunAt: now.Add(-time.Minute),
			InputJSON: mustMarshalJSON(t, financeInput{AccountID: "acct-scheduled"}),
		}))
		_, err = svc.Enqueue(t.Context(), EnqueueParams{
			JobType:   JobTypeHistoricalRawCandleBackfill,
			Requester: Requester{UserID: "user-" + fake.UUID().V4(), Source: RequesterSourceOperator},
			Input: HistoricalRawCandleBackfillInput{
				IngestionRunID: fake.UUID().V4(),
				Venue:          "hyperliquid-perps",
				Symbol:         "BTC",
				AssetClass:     "future",
				Timeframe:      "1m",
				Start:          now.Add(-2 * time.Minute),
				End:            now.Add(-time.Minute),
				PageSize:       10,
			},
		})
		require.NoError(t, err)
		scheduler, err := NewScheduler(SchedulerDeps{
			Store:   store,
			Service: svc,
			Clock:   func() time.Time { return now },
		})
		require.NoError(t, err)
		count, err := scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		count, err = scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 0, count)
		queued, err := store.List(t.Context(), ListParams{Statuses: []JobStatus{JobStatusQueued}, Limit: 10})
		require.NoError(t, err)
		require.Len(t, queued.Items, 2)
		assert.EqualValues(t, 0, historicalCalls.Load())
		assert.EqualValues(t, 0, financeCalls.Load())
	})
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := EncodeJobPayload(value)
	require.NoError(t, err)
	return payload
}
