package jobs

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/flows"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRunner struct {
	mu     sync.Mutex
	calls  []flows.HistoricalRawCandleBackfillRequest
	result flows.HistoricalRawCandleBackfillResult
	err    error
}

func (r *stubRunner) Run(
	_ context.Context,
	request flows.HistoricalRawCandleBackfillRequest,
) (flows.HistoricalRawCandleBackfillResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, request)
	return r.result, r.err
}

func TestWorker(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T) *Store {
		t.Helper()
		store, err := NewStore(
			filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			StoreOpts{TablePrefix: "wrk_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}
	makeJob := func(now time.Time) Job {
		input := HistoricalRawCandleBackfillInput{
			IngestionRunID: "run-" + fake.UUID().V4(),
			Venue:          "hyperliquid-perps",
			Symbol:         "BTC",
			AssetClass:     "future",
			Timeframe:      "1h",
			Start:          now.Add(-2 * time.Hour),
			End:            now.Add(-time.Hour),
			PageSize:       100,
		}
		hash, err := HashInput(input)
		require.NoError(t, err)
		return Job{
			ID:      "job-" + fake.UUID().V4(),
			JobType: JobTypeHistoricalRawCandleBackfill,
			Status:  JobStatusQueued,
			Requester: Requester{
				UserID: "user-" + fake.UUID().V4(),
				Source: RequesterSourceOperator,
			},
			InputHash: hash,
			Input:     input,
			CreatedAt: now,
			UpdatedAt: now,
			QueuedAt:  now,
		}
	}
	makeWorker := func(t *testing.T, store *Store, runner *stubRunner, now time.Time) *Worker {
		t.Helper()
		worker, err := NewWorker(WorkerDeps{
			Store:  store,
			Runner: runner,
			Logger: slog.Default(),
			Clock: func() time.Time {
				return now
			},
			Config: WorkerConfig{
				Enabled:                         true,
				MaxAttempts:                     3,
				MaxConcurrentHistoricalBackfill: 1,
				PollInterval:                    time.Hour,
			},
			WorkerID: "worker-test",
		})
		require.NoError(t, err)
		return worker
	}

	t.Run("claims queued jobs, increments attempts, and persists succeeded state", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		job := makeJob(now)
		_, err := store.Create(t.Context(), job)
		require.NoError(t, err)
		runner := &stubRunner{result: flows.HistoricalRawCandleBackfillResult{
			RunID: job.Input.IngestionRunID,
			Report: flows.HistoricalRawCandleBackfillReport{
				PersistedCount:              2,
				ExpectedCount:               2,
				MissingIntervalPreviewLimit: 10,
			},
		}}
		worker := makeWorker(t, store, runner, now.Add(time.Minute))
		require.NoError(t, worker.ProcessJob(t.Context(), job.ID))
		persisted, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusSucceeded, persisted.Status)
		assert.Equal(t, 1, persisted.AttemptCount)
		assert.Equal(t, "worker-test", persisted.WorkerID)
		require.NotNil(t, persisted.Result)
		assert.Equal(t, job.Input.IngestionRunID, persisted.Result.IngestionRunID)
		require.Len(t, runner.calls, 1)
		assert.Equal(t, domain.Venue("hyperliquid-perps"), runner.calls[0].Venue)
	})

	t.Run("persists safe bounded failures and skips duplicate terminal processing", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		job := makeJob(now)
		_, err := store.Create(t.Context(), job)
		require.NoError(t, err)
		runner := &stubRunner{err: errors.New("gorm: query jobs where secret = 1")}
		worker := makeWorker(t, store, runner, now.Add(time.Minute))
		require.NoError(t, worker.ProcessJob(t.Context(), job.ID))
		persisted, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, persisted.Status)
		require.NotNil(t, persisted.Error)
		assert.Equal(t, "job_execution_failed", persisted.Error.Code)
		assert.NotContains(t, strings.ToLower(persisted.Error.Details), "gorm")
		require.Len(t, runner.calls, 1)
		require.NoError(t, worker.ProcessJob(t.Context(), job.ID))
		require.Len(t, runner.calls, 1)
	})

	t.Run("startup recovery requeues stale running below cap and fails exhausted rows", func(t *testing.T) {
		now := time.Now().UTC().Add(-time.Hour)
		store := makeStore(t)
		job := makeJob(now)
		startedAt := now.Add(5 * time.Minute)
		job.Status = JobStatusRunning
		job.AttemptCount = 1
		job.WorkerID = "stale"
		job.StartedAt = &startedAt
		job.LastAttemptAt = &startedAt
		jobExhausted := makeJob(now.Add(time.Minute))
		jobExhausted.Status = JobStatusRunning
		jobExhausted.AttemptCount = 3
		jobExhausted.WorkerID = "stale"
		jobExhausted.StartedAt = &startedAt
		jobExhausted.LastAttemptAt = &startedAt
		for _, item := range []Job{job, jobExhausted} {
			_, err := store.Create(t.Context(), item)
			require.NoError(t, err)
		}
		runner := &stubRunner{result: flows.HistoricalRawCandleBackfillResult{
			RunID: job.Input.IngestionRunID,
			Report: flows.HistoricalRawCandleBackfillReport{
				PersistedCount:              1,
				ExpectedCount:               1,
				MissingIntervalPreviewLimit: 10,
			},
		}}
		worker := makeWorker(t, store, runner, now.Add(30*time.Minute))
		require.NoError(t, worker.Start(t.Context()))
		defer func() { require.NoError(t, worker.Stop(t.Context())) }()
		requeued, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusQueued, requeued.Status)
		require.NotNil(t, requeued.Error)
		assert.Equal(t, "stale_running_requeued", requeued.Error.Code)
		exhausted, err := store.Get(t.Context(), jobExhausted.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, exhausted.Status)
		require.NotNil(t, exhausted.Error)
		assert.Equal(t, "stale_running_attempts_exhausted", exhausted.Error.Code)
	})

	t.Run("polls only one queued job with default conservative concurrency", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		jobA := makeJob(now)
		jobB := makeJob(now.Add(time.Second))
		for _, item := range []Job{jobA, jobB} {
			_, err := store.Create(t.Context(), item)
			require.NoError(t, err)
		}
		runner := &stubRunner{result: flows.HistoricalRawCandleBackfillResult{
			RunID: jobA.Input.IngestionRunID,
			Report: flows.HistoricalRawCandleBackfillReport{
				PersistedCount:              1,
				ExpectedCount:               1,
				MissingIntervalPreviewLimit: 10,
			},
		}}
		worker, err := NewWorker(WorkerDeps{
			Store:  store,
			Runner: runner,
			Logger: slog.Default(),
			Clock: func() time.Time {
				return now.Add(time.Minute)
			},
			Config: WorkerConfig{
				Enabled:                         true,
				MaxConcurrentHistoricalBackfill: 1,
				PollInterval:                    time.Hour,
			},
			WorkerID: "worker-test",
		})
		require.NoError(t, err)
		require.NoError(t, worker.pollOnce(t.Context()))
		require.Len(t, runner.calls, 1)
		queued, err := store.List(t.Context(), ListParams{Statuses: []JobStatus{JobStatusQueued}, Limit: 10})
		require.NoError(t, err)
		require.Len(t, queued.Items, 1)
	})

	t.Run("returns constructor errors and ignores missing or unclaimable jobs", func(t *testing.T) {
		_, err := NewWorker(WorkerDeps{})
		require.Error(t, err)
		store := makeStore(t)
		_, err = NewWorker(WorkerDeps{Store: store})
		require.Error(t, err)
		runner := &stubRunner{}
		worker := makeWorker(t, store, runner, time.Now().UTC())
		require.NoError(t, worker.ProcessJob(t.Context(), "missing"))
		job := makeJob(time.Now().UTC())
		job.Status = JobStatusRunning
		_, err = store.Create(t.Context(), job)
		require.NoError(t, err)
		require.NoError(t, worker.ProcessJob(t.Context(), job.ID))
		require.Empty(t, runner.calls)
	})

	t.Run("covers start disabled and polling failures", func(t *testing.T) {
		store := makeStore(t)
		runner := &stubRunner{}
		worker, err := NewWorker(WorkerDeps{
			Store:  store,
			Runner: runner,
			Config: WorkerConfig{Enabled: false},
		})
		require.NoError(t, err)
		require.NoError(t, worker.Start(t.Context()))
		activeStore := makeStore(t)
		activeJob := makeJob(time.Now().UTC())
		_, err = activeStore.Create(t.Context(), activeJob)
		require.NoError(t, err)
		activeRunner := &stubRunner{result: flows.HistoricalRawCandleBackfillResult{
			RunID: activeJob.Input.IngestionRunID,
		}}
		activeWorker, err := NewWorker(WorkerDeps{
			Store:  activeStore,
			Runner: activeRunner,
			Config: WorkerConfig{Enabled: true, PollInterval: time.Millisecond},
		})
		require.NoError(t, err)
		require.NoError(t, activeWorker.Start(t.Context()))
		activeWorker.SignalWake(activeJob.ID)
		time.Sleep(10 * time.Millisecond)
		require.NoError(t, activeWorker.Stop(t.Context()))
		require.NotEmpty(t, activeRunner.calls)
		require.NoError(t, store.db.Exec("DROP TABLE "+store.tableName).Error)
		brokenWorker := makeWorker(t, store, runner, time.Now().UTC())
		require.Error(t, brokenWorker.pollOnce(t.Context()))
		require.Error(t, brokenWorker.Start(t.Context()))
		require.Error(t, brokenWorker.ProcessJob(t.Context(), "job-any"))
	})

	t.Run("signal wake constructor defaults and runner factory paths are covered", func(t *testing.T) {
		store := makeStore(t)
		runner := &stubRunner{}
		worker, err := NewWorker(WorkerDeps{Store: store, Runner: runner})
		require.NoError(t, err)
		worker.SignalWake("job-1")
		for range cap(worker.wake) + 1 {
			worker.SignalWake("job")
		}
		assert.NotEmpty(t, worker.workerID)

		_, err = NewHistoricalBackfillRunner(nil, nil, nil)
		require.Error(t, err)
		_, err = NewHistoricalBackfillRunner(&data.LineageService{}, nil, nil)
		require.Error(t, err)

		dataStore, err := data.NewDatabaseStore(
			filepath.Join(t.TempDir(), "data.sqlite"),
			data.DatabaseStoreOpts{TablePrefix: "worker_data_"},
		)
		require.NoError(t, err)
		require.NoError(t, dataStore.AutoMigrate())
		blobStore, err := data.NewLocalRawPayloadBlobStore(filepath.Join(t.TempDir(), "payloads"))
		require.NoError(t, err)
		lineageService, err := data.NewLineageService(data.LineageServiceDeps{Store: dataStore, BlobStore: blobStore})
		require.NoError(t, err)
		startedRun, err := data.NewIngestionRun(data.IngestionRunParams{
			ID:          "run-a",
			Source:      "test-source",
			Venue:       venueedge.HyperliquidPerpsVenueName,
			Status:      data.IngestionRunStatusStarted,
			StartedAt:   time.Now().UTC(),
			RecordCount: 0,
		})
		require.NoError(t, err)
		_, err = lineageService.RecordIngestionRun(t.Context(), startedRun)
		require.NoError(t, err)
		readService, err := data.NewReadService(data.ReadServiceDeps{
			InstrumentStore: dataStore,
			CandleStore:     dataStore,
			TradeStore:      dataStore,
		})
		require.NoError(t, err)
		ingestionService, err := data.NewIngestionService(data.IngestionServiceDeps{
			InstrumentStore: dataStore,
			CandleStore:     dataStore,
			TradeStore:      dataStore,
		})
		require.NoError(t, err)
		ingestionFlow, err := venueedge.NewIngestionFlow(ingestionService)
		require.NoError(t, err)
		ingestionFlow.WithRawPayloadLineage(lineageService)
		_, err = NewHistoricalBackfillRunner(lineageService, readService, nil)
		require.Error(t, err)
		backfillRunner, err := NewHistoricalBackfillRunner(lineageService, readService, ingestionFlow)
		require.NoError(t, err)
		require.NotNil(t, backfillRunner)
		recorder := &hyperliquidRawEvidenceRecorder{lineageService: lineageService}
		payloadID, err := recorder.RecordHyperliquidRawEvidence(t.Context(), venueedge.HyperliquidRawEvidenceCapture{
			ID:                 "payload-a",
			IngestionRunID:     "run-a",
			Venue:              venueedge.HyperliquidPerpsVenueName,
			Endpoint:           "/info",
			RequestType:        "meta",
			RequestPayloadHash: "hash-a",
			RequestAt:          time.Now().UTC(),
			ResponseAt:         time.Now().UTC(),
			HTTPStatus:         200,
			ResponseBody:       []byte("{}"),
			ReceivedAt:         time.Now().UTC(),
		})
		require.NoError(t, err)
		assert.Equal(t, "payload-a", payloadID)
		_, err = recorder.RecordHyperliquidRawEvidence(t.Context(), venueedge.HyperliquidRawEvidenceCapture{
			ID:                 "payload-b",
			IngestionRunID:     "missing-run",
			Venue:              venueedge.HyperliquidPerpsVenueName,
			Endpoint:           "/info",
			RequestType:        "meta",
			RequestPayloadHash: "hash-b",
			RequestAt:          time.Now().UTC(),
			ResponseAt:         time.Now().UTC(),
			HTTPStatus:         200,
			ResponseBody:       []byte("{}"),
			ReceivedAt:         time.Now().UTC(),
		})
		require.Error(t, err)
		_, err = recorder.RecordHyperliquidRawEvidence(t.Context(), venueedge.HyperliquidRawEvidenceCapture{})
		require.Error(t, err)
	})
}
