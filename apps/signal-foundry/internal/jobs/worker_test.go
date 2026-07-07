package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
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

type observationStoreStub struct {
	base                *Store
	markSucceededErr    error
	updateProgressErr   error
	markSucceededCalls  int
	updateProgressCalls int
}

func (s *observationStoreStub) Get(ctx context.Context, jobID string) (*Job, error) {
	return s.base.Get(ctx, jobID)
}

func (s *observationStoreStub) List(ctx context.Context, params ListParams) (ListResult, error) {
	return s.base.List(ctx, params)
}

func (s *observationStoreStub) ClaimQueued(
	ctx context.Context,
	jobID string,
	workerID string,
	claimedAt time.Time,
) (*Job, error) {
	return s.base.ClaimQueued(ctx, jobID, workerID, claimedAt)
}

func (s *observationStoreStub) MarkSucceeded(
	ctx context.Context,
	jobID string,
	workerID string,
	result any,
	completedAt time.Time,
) error {
	s.markSucceededCalls++
	if s.markSucceededErr != nil {
		return s.markSucceededErr
	}
	return s.base.MarkSucceeded(ctx, jobID, workerID, result, completedAt)
}

func (s *observationStoreStub) MarkFailed(
	ctx context.Context,
	jobID string,
	workerID string,
	jobErr *JobError,
	completedAt time.Time,
) error {
	return s.base.MarkFailed(ctx, jobID, workerID, jobErr, completedAt)
}

func (s *observationStoreStub) UpdateProgress(
	ctx context.Context,
	jobID string,
	progressJSON json.RawMessage,
	updatedAt time.Time,
) error {
	s.updateProgressCalls++
	if s.updateProgressErr != nil {
		return s.updateProgressErr
	}
	return s.base.UpdateProgress(ctx, jobID, progressJSON, updatedAt)
}

func (s *observationStoreStub) RecoverStaleRunning(ctx context.Context, now time.Time, maxAttempts int) error {
	return s.base.RecoverStaleRunning(ctx, now, maxAttempts)
}

type workerStoreListErrorStub struct{ err error }

func (s workerStoreListErrorStub) Get(context.Context, string) (*Job, error) {
	return nil, ErrJobNotFound
}

func (s workerStoreListErrorStub) List(context.Context, ListParams) (ListResult, error) {
	return ListResult{}, s.err
}

func (s workerStoreListErrorStub) ClaimQueued(context.Context, string, string, time.Time) (*Job, error) {
	return nil, s.err
}

func (s workerStoreListErrorStub) MarkSucceeded(context.Context, string, string, any, time.Time) error {
	return s.err
}

func (s workerStoreListErrorStub) MarkFailed(context.Context, string, string, *JobError, time.Time) error {
	return s.err
}

func (s workerStoreListErrorStub) UpdateProgress(context.Context, string, json.RawMessage, time.Time) error {
	return s.err
}

func (s workerStoreListErrorStub) RecoverStaleRunning(context.Context, time.Time, int) error {
	return s.err
}

type workerStorePrepareErrorStub struct {
	job      *Job
	getErr   error
	claimErr error
}

func (s workerStorePrepareErrorStub) Get(context.Context, string) (*Job, error) {
	return s.job, s.getErr
}

func (workerStorePrepareErrorStub) List(context.Context, ListParams) (ListResult, error) {
	return ListResult{}, nil
}

func (s workerStorePrepareErrorStub) ClaimQueued(context.Context, string, string, time.Time) (*Job, error) {
	return nil, s.claimErr
}

func (workerStorePrepareErrorStub) MarkSucceeded(context.Context, string, string, any, time.Time) error {
	return nil
}

func (workerStorePrepareErrorStub) MarkFailed(context.Context, string, string, *JobError, time.Time) error {
	return nil
}

func (workerStorePrepareErrorStub) UpdateProgress(context.Context, string, json.RawMessage, time.Time) error {
	return nil
}

func (workerStorePrepareErrorStub) RecoverStaleRunning(context.Context, time.Time, int) error {
	return nil
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
			DispatchConfig: DispatchConfig{
				DatabaseDSN: filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
				TablePrefix: "wrk_",
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

	t.Run("persists safe bounded failures and repeats terminal processing without explicit guards", func(t *testing.T) {
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
		require.Len(t, runner.calls, 2)
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

	t.Run("processes explicitly addressed queued jobs and leaves others untouched", func(t *testing.T) {
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
		require.NoError(t, worker.ProcessJob(t.Context(), jobA.ID))
		require.Len(t, runner.calls, 1)
		queued, err := store.List(t.Context(), ListParams{Statuses: []JobStatus{JobStatusQueued}, Limit: 10})
		require.NoError(t, err)
		require.Len(t, queued.Items, 1)
		assert.Equal(t, jobB.ID, queued.Items[0].ID)
	})

	t.Run("returns constructor errors and ignores missing jobs", func(t *testing.T) {
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
		require.Len(t, runner.calls, 1)
	})

	t.Run("covers start disabled and consumer startup failures", func(t *testing.T) {
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
		dispatchDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{DatabaseDSN: dispatchDSN}))
		publisher, err := appdispatch.NewPublisher(
			appdispatch.Config{DatabaseDSN: dispatchDSN},
			slog.Default(),
		)
		require.NoError(t, err)
		defer func() { require.NoError(t, publisher.Close()) }()
		activeWorker, err := NewWorker(WorkerDeps{
			Store:          activeStore,
			Runner:         activeRunner,
			Config:         WorkerConfig{Enabled: true, PollInterval: 50 * time.Millisecond},
			DispatchConfig: DispatchConfig{DatabaseDSN: dispatchDSN},
		})
		require.NoError(t, err)
		require.NoError(t, publisher.Publish(t.Context(), appdispatch.Envelope{
			Version:         appdispatch.EnvelopeVersionV1,
			Kind:            DispatchKindHistoricalRawCandleBackfill,
			Payload:         mustEncodeWorkerPayload(t, activeJob.Input),
			ObservableJobID: activeJob.ID,
		}))
		require.NoError(t, activeWorker.RunOnce(t.Context()))
		require.NotEmpty(t, activeRunner.calls)
		require.NoError(t, store.db.Exec("DROP TABLE "+store.tableName).Error)
		brokenWorker := makeWorker(t, store, runner, time.Now().UTC())
		require.Error(t, brokenWorker.ProcessJob(t.Context(), "job-any"))
		brokenWorker.dispatchConfig = DispatchConfig{
			DatabaseDSN: filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
		}
		require.Error(t, brokenWorker.Start(t.Context()))
		require.Error(t, brokenWorker.ProcessJob(t.Context(), "job-any"))
	})

	t.Run("runs consumers directly and handles unknown dispatch kinds", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		job := makeJob(now)
		job.InputJSON = mustEncodeWorkerPayload(t, job.Input)
		_, err := store.Create(t.Context(), job)
		require.NoError(t, err)

		dispatchDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{
			DatabaseDSN: dispatchDSN,
			TablePrefix: "wrk_",
		}))
		publisher, err := appdispatch.NewPublisher(appdispatch.Config{
			DatabaseDSN: dispatchDSN,
			TablePrefix: "wrk_",
		}, slog.Default())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })

		runner := &stubRunner{result: flows.HistoricalRawCandleBackfillResult{RunID: job.Input.IngestionRunID}}
		worker, err := NewWorker(WorkerDeps{
			Store:          store,
			Runner:         runner,
			Clock:          func() time.Time { return now.Add(time.Minute) },
			Config:         WorkerConfig{Enabled: true, PollInterval: 10 * time.Millisecond},
			DispatchConfig: DispatchConfig{DatabaseDSN: dispatchDSN, TablePrefix: "wrk_"},
		})
		require.NoError(t, err)
		require.Nil(t, worker.consumer)

		require.NoError(t, publisher.Publish(t.Context(), appdispatch.Envelope{
			Version:         appdispatch.EnvelopeVersionV1,
			Kind:            DispatchKindHistoricalRawCandleBackfill,
			Payload:         mustEncodeWorkerPayload(t, job.Input),
			ObservableJobID: job.ID,
		}))
		runCtx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		err = worker.Run(runCtx)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.NotNil(t, worker.consumer)
		assert.Len(t, runner.calls, 1)

		missingJob := makeJob(now.Add(2 * time.Minute))
		missingJob.InputJSON = mustEncodeWorkerPayload(t, missingJob.Input)
		_, err = store.Create(t.Context(), missingJob)
		require.NoError(t, err)
		require.NoError(t, worker.processEnvelope(t.Context(), appdispatch.Envelope{
			Version:         appdispatch.EnvelopeVersionV1,
			Kind:            appdispatch.ExecutionKind("missing.dispatch.kind"),
			Payload:         mustEncodeWorkerPayload(t, missingJob.Input),
			ObservableJobID: missingJob.ID,
		}))
		persisted, err := store.Get(t.Context(), missingJob.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusFailed, persisted.Status)
		require.NotNil(t, persisted.Error)
		assert.Equal(t, "job_execution_failed", persisted.Error.Code)

		err = worker.processEnvelope(t.Context(), appdispatch.Envelope{
			Version: appdispatch.EnvelopeVersionV1,
			Kind:    appdispatch.ExecutionKind("missing.dispatch.kind"),
			Payload: mustEncodeWorkerPayload(t, missingJob.Input),
		})
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
	})

	t.Run("constructor defaults and runner factory paths are covered", func(t *testing.T) {
		store := makeStore(t)
		runner := &stubRunner{}
		worker, err := NewWorker(WorkerDeps{Store: store, Runner: runner})
		require.NoError(t, err)
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

	t.Run("consumer routes by dispatch kind and updates observable job metadata", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		job := makeJob(now)
		job.InputJSON = mustEncodeWorkerPayload(t, job.Input)
		_, err := store.Create(t.Context(), job)
		require.NoError(t, err)

		runner := &stubRunner{
			result: flows.HistoricalRawCandleBackfillResult{RunID: job.Input.IngestionRunID},
		}
		dispatchDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		publisher, err := appdispatch.NewPublisher(appdispatch.Config{
			DatabaseDSN: dispatchDSN,
			TablePrefix: "wrk_",
		}, slog.Default())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{
			DatabaseDSN: dispatchDSN,
			TablePrefix: "wrk_",
		}))

		worker, err := NewWorker(WorkerDeps{
			Store:          store,
			Runner:         runner,
			Logger:         slog.Default(),
			Clock:          func() time.Time { return now.Add(time.Minute) },
			Config:         WorkerConfig{Enabled: true, PollInterval: 10 * time.Millisecond},
			DispatchConfig: DispatchConfig{DatabaseDSN: dispatchDSN, TablePrefix: "wrk_"},
			WorkerID:       "worker-dispatch",
		})
		require.NoError(t, err)

		require.NoError(t, publisher.Publish(t.Context(), appdispatch.Envelope{
			Version:         appdispatch.EnvelopeVersionV1,
			Kind:            DispatchKindHistoricalRawCandleBackfill,
			Payload:         mustEncodeWorkerPayload(t, canonicalizeHistoricalInput(job.Input)),
			ObservableJobID: job.ID,
		}))
		require.NoError(t, worker.RunOnce(t.Context()))

		persisted, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusSucceeded, persisted.Status)
		require.Len(t, runner.calls, 1)
		assert.Equal(t, job.Input.IngestionRunID, runner.calls[0].RunID)
	})

	t.Run("processes queued jobs when dispatch and jobs share one sqlite database", func(t *testing.T) {
		now := time.Now().UTC()
		dsn := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		store, err := NewStore(dsn, StoreOpts{TablePrefix: "wrk_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{
			DatabaseDSN: dsn,
			TablePrefix: "wrk_",
		}))

		job := makeJob(now)
		job.InputJSON = mustEncodeWorkerPayload(t, job.Input)
		_, err = store.Create(t.Context(), job)
		require.NoError(t, err)

		runner := &stubRunner{result: flows.HistoricalRawCandleBackfillResult{RunID: job.Input.IngestionRunID}}
		worker, err := NewWorker(WorkerDeps{
			Store:  store,
			Runner: runner,
			Clock:  func() time.Time { return now.Add(time.Minute) },
			Config: WorkerConfig{Enabled: true, PollInterval: 250 * time.Millisecond},
			DispatchConfig: DispatchConfig{
				DatabaseDSN: dsn,
				JobsDSN:     dsn,
				TablePrefix: "wrk_",
			},
			WorkerID: "worker-shared-db",
		})
		require.NoError(t, err)

		publisher, err := appdispatch.NewPublisher(
			appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "wrk_"},
			slog.Default(),
		)
		require.NoError(t, err)
		defer func() { require.NoError(t, publisher.Close()) }()

		require.NoError(t, publisher.Publish(t.Context(), appdispatch.Envelope{
			Version:         appdispatch.EnvelopeVersionV1,
			Kind:            DispatchKindHistoricalRawCandleBackfill,
			Payload:         mustEncodeWorkerPayload(t, job.Input),
			ObservableJobID: job.ID,
		}))
		require.NoError(t, worker.RunOnce(t.Context()))

		persisted, err := store.Get(t.Context(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, JobStatusSucceeded, persisted.Status)
		require.Len(t, runner.calls, 1)
	})

	t.Run("sqlite polling helpers cover shared-db detection and idle exits", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		worker, err := NewWorker(WorkerDeps{
			Store:  store,
			Runner: &stubRunner{},
			Clock:  func() time.Time { return now },
			Config: WorkerConfig{Enabled: true, PollInterval: 10 * time.Millisecond},
			DispatchConfig: DispatchConfig{
				DatabaseDSN: filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
				JobsDSN:     filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			},
		})
		require.NoError(t, err)
		assert.False(t, worker.usesSQLitePolling())

		sharedDSN := filepath.Join(t.TempDir(), "shared.sqlite")
		worker.dispatchConfig = DispatchConfig{DatabaseDSN: sharedDSN, JobsDSN: sharedDSN}
		assert.True(t, worker.usesSQLitePolling())
		require.NoError(t, worker.Start(t.Context()))
		require.NoError(t, worker.Stop(t.Context()))

		runCtx, cancelRun := context.WithCancel(t.Context())
		cancelRun()
		require.ErrorIs(t, worker.Run(runCtx), context.Canceled)
		require.ErrorIs(t, worker.RunOnce(runCtx), context.Canceled)

		idleCtx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, worker.runSQLitePollingLoop(idleCtx), context.Canceled)
	})

	t.Run("sqlite polling reports list failures and non-observable execution errors", func(t *testing.T) {
		listErr := errors.New("list boom")
		worker := &Worker{
			store:    workerStoreListErrorStub{err: listErr},
			registry: NewRegistry(),
			logger:   slog.Default(),
		}
		_, err := worker.processNextQueuedJob(t.Context())
		require.ErrorIs(t, err, listErr)
		now := time.Now().UTC()

		registry := NewRegistry()
		require.NoError(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[HistoricalRawCandleBackfillInput, HistoricalRawCandleBackfillResult, struct{}]{
				JobType: JobTypeHistoricalRawCandleBackfill,
				Run: func(context.Context, HistoricalRawCandleBackfillInput, func(struct{}) error) (HistoricalRawCandleBackfillResult, error) {
					return HistoricalRawCandleBackfillResult{}, errors.New("execute boom")
				},
			},
		))
		worker.registry = registry
		input := HistoricalRawCandleBackfillInput{
			IngestionRunID: fake.UUID().V4(),
			Venue:          "hyperliquid-perps",
			Symbol:         "BTC",
			AssetClass:     "future",
			Timeframe:      "1h",
			Start:          now.Add(-time.Hour),
			End:            now,
			PageSize:       1,
		}
		err = worker.processEnvelope(t.Context(), appdispatch.Envelope{
			Version: appdispatch.EnvelopeVersionV1,
			Kind:    DispatchKindHistoricalRawCandleBackfill,
			Payload: mustEncodeWorkerPayload(t, input),
		})
		require.EqualError(t, err, "execute boom")

		prepareErr := errors.New("prepare boom")
		executor := &workerExecutor{
			store: workerStorePrepareErrorStub{
				job:      &Job{ID: "job-prepare", Status: JobStatusQueued},
				claimErr: prepareErr,
			},
			registry: NewRegistry(),
			logger:   slog.Default(),
			clock:    func() time.Time { return now },
			workerID: "worker-prepare",
		}
		require.NoError(t, RegisterTypedHandler(
			executor.registry,
			TypedHandlerSpec[HistoricalRawCandleBackfillInput, HistoricalRawCandleBackfillResult, struct{}]{
				JobType: JobTypeHistoricalRawCandleBackfill,
				Run: func(context.Context, HistoricalRawCandleBackfillInput, func(struct{}) error) (HistoricalRawCandleBackfillResult, error) {
					return HistoricalRawCandleBackfillResult{}, nil
				},
			},
		))
		handler, err := executor.registry.HandlerByExecutionKind(DispatchKindHistoricalRawCandleBackfill)
		require.NoError(t, err)
		_, _, err = executor.prepareObservableJob(t.Context(), "job-prepare", handler)
		require.ErrorIs(t, err, prepareErr)

		getErr := errors.New("get boom")
		executor.store = workerStorePrepareErrorStub{getErr: getErr}
		_, _, err = executor.prepareObservableJob(t.Context(), "job-prepare", handler)
		require.ErrorIs(t, err, getErr)
	})

	t.Run("reuses an already initialized dispatch consumer", func(t *testing.T) {
		store := makeStore(t)
		dispatchDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{DatabaseDSN: dispatchDSN}))
		worker, err := NewWorker(WorkerDeps{
			Store:          store,
			Runner:         &stubRunner{},
			DispatchConfig: DispatchConfig{DatabaseDSN: dispatchDSN},
		})
		require.NoError(t, err)
		require.NoError(t, worker.ensureConsumer())
		firstConsumer := worker.consumer
		require.NotNil(t, firstConsumer)
		require.NoError(t, worker.ensureConsumer())
		assert.Same(t, firstConsumer, worker.consumer)
		require.NoError(t, worker.Stop(t.Context()))
	})

	t.Run("duplicate-delivery guards apply only when the workflow opts in", func(t *testing.T) {
		now := time.Now().UTC()
		dispatchDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{
			DatabaseDSN: dispatchDSN,
			TablePrefix: "wrk_",
		}))
		publisher, err := appdispatch.NewPublisher(appdispatch.Config{
			DatabaseDSN: dispatchDSN,
			TablePrefix: "wrk_",
		}, slog.Default())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })

		t.Run("guarded handler skips terminal observable duplicates", func(t *testing.T) {
			store := makeStore(t)
			job := makeJob(now)
			job.Status = JobStatusSucceeded
			job.CompletedAt = ptrWorkerTime(now)
			job.InputJSON = mustEncodeWorkerPayload(t, job.Input)
			_, createErr := store.Create(t.Context(), job)
			require.NoError(t, createErr)

			registry := NewRegistry()
			calls := 0
			require.NoError(t, RegisterTypedHandler(
				registry,
				TypedHandlerSpec[
					HistoricalRawCandleBackfillInput,
					HistoricalRawCandleBackfillResult,
					struct{},
				]{
					JobType:                JobTypeHistoricalRawCandleBackfill,
					GuardDuplicateDelivery: true,
					Run: func(
						context.Context,
						HistoricalRawCandleBackfillInput,
						func(struct{}) error,
					) (HistoricalRawCandleBackfillResult, error) {
						calls++
						return HistoricalRawCandleBackfillResult{}, nil
					},
				},
			))
			worker, workerErr := NewWorker(WorkerDeps{
				Store:    store,
				Registry: registry,
				Config: WorkerConfig{
					PollInterval: 50 * time.Millisecond,
				},
				DispatchConfig: DispatchConfig{
					DatabaseDSN: dispatchDSN,
					TablePrefix: "wrk_",
				},
			})
			require.NoError(t, workerErr)
			require.NoError(t, publisher.Publish(t.Context(), appdispatch.Envelope{
				Version:         appdispatch.EnvelopeVersionV1,
				Kind:            DispatchKindHistoricalRawCandleBackfill,
				Payload:         mustEncodeWorkerPayload(t, job.Input),
				ObservableJobID: job.ID,
			}))
			require.NoError(t, worker.RunOnce(t.Context()))
			assert.Equal(t, 0, calls)
		})

		t.Run("unguarded handler may re-execute terminal observable duplicates", func(t *testing.T) {
			store := makeStore(t)
			job := makeJob(now.Add(time.Minute))
			job.Status = JobStatusSucceeded
			job.CompletedAt = ptrWorkerTime(now.Add(time.Minute))
			job.InputJSON = mustEncodeWorkerPayload(t, job.Input)
			_, createErr := store.Create(t.Context(), job)
			require.NoError(t, createErr)

			registry := NewRegistry()
			calls := 0
			require.NoError(t, RegisterTypedHandler(
				registry,
				TypedHandlerSpec[
					HistoricalRawCandleBackfillInput,
					HistoricalRawCandleBackfillResult,
					struct{},
				]{
					JobType: JobTypeHistoricalRawCandleBackfill,
					Run: func(
						context.Context,
						HistoricalRawCandleBackfillInput,
						func(struct{}) error,
					) (HistoricalRawCandleBackfillResult, error) {
						calls++
						return HistoricalRawCandleBackfillResult{}, nil
					},
				},
			))
			worker, workerErr := NewWorker(WorkerDeps{
				Store:    store,
				Registry: registry,
				Config: WorkerConfig{
					PollInterval: 50 * time.Millisecond,
				},
				DispatchConfig: DispatchConfig{
					DatabaseDSN: dispatchDSN,
					TablePrefix: "wrk_",
				},
			})
			require.NoError(t, workerErr)
			require.NoError(t, publisher.Publish(t.Context(), appdispatch.Envelope{
				Version:         appdispatch.EnvelopeVersionV1,
				Kind:            DispatchKindHistoricalRawCandleBackfill,
				Payload:         mustEncodeWorkerPayload(t, job.Input),
				ObservableJobID: job.ID,
			}))
			require.NoError(t, worker.RunOnce(t.Context()))
			assert.Equal(t, 1, calls)
		})
	})

	t.Run("successful business work is not replayed solely because observation writes fail", func(t *testing.T) {
		now := time.Now().UTC()
		baseStore := makeStore(t)
		job := makeJob(now)
		job.InputJSON = mustEncodeWorkerPayload(t, job.Input)
		_, err := baseStore.Create(t.Context(), job)
		require.NoError(t, err)

		dispatchDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{
			DatabaseDSN: dispatchDSN,
			TablePrefix: "wrk_",
		}))
		publisher, err := appdispatch.NewPublisher(appdispatch.Config{
			DatabaseDSN: dispatchDSN,
			TablePrefix: "wrk_",
		}, slog.Default())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })

		observationStore := &observationStoreStub{
			base:             baseStore,
			markSucceededErr: errors.New("mark succeeded boom"),
		}
		runner := &stubRunner{
			result: flows.HistoricalRawCandleBackfillResult{RunID: job.Input.IngestionRunID},
		}
		worker, err := NewWorker(WorkerDeps{
			Store:          observationStore,
			Runner:         runner,
			Config:         WorkerConfig{PollInterval: 10 * time.Millisecond},
			DispatchConfig: DispatchConfig{DatabaseDSN: dispatchDSN, TablePrefix: "wrk_"},
		})
		require.NoError(t, err)
		require.NoError(t, publisher.Publish(t.Context(), appdispatch.Envelope{
			Version:         appdispatch.EnvelopeVersionV1,
			Kind:            DispatchKindHistoricalRawCandleBackfill,
			Payload:         mustEncodeWorkerPayload(t, job.Input),
			ObservableJobID: job.ID,
		}))
		require.NoError(t, worker.RunOnce(t.Context()))
		assert.Len(t, runner.calls, 1)
	})

	t.Run("covers remaining worker error and progress branches", func(t *testing.T) {
		now := time.Now().UTC()

		t.Run("stop without consumer and runner registration error", func(t *testing.T) {
			store := makeStore(t)
			worker := makeWorker(t, store, &stubRunner{}, now)
			require.NoError(t, worker.Stop(t.Context()))
			require.Error(t, RegisterHistoricalBackfillHandler(NewRegistry(), nil))
		})

		t.Run("run and runonce surface startup errors", func(t *testing.T) {
			brokenStore := makeStore(t)
			require.NoError(t, brokenStore.db.Exec("DROP TABLE "+brokenStore.tableName).Error)
			worker := makeWorker(t, brokenStore, &stubRunner{}, now)
			require.Error(t, worker.Run(t.Context()))
			require.Error(t, worker.RunOnce(t.Context()))

			store := makeStore(t)
			job := makeJob(now)
			job.InputJSON = mustEncodeWorkerPayload(t, job.Input)
			_, err := store.Create(t.Context(), job)
			require.NoError(t, err)
			brokenDispatchWorker, err := NewWorker(WorkerDeps{
				Store:          store,
				Runner:         &stubRunner{},
				Config:         WorkerConfig{PollInterval: 10 * time.Millisecond},
				DispatchConfig: DispatchConfig{DatabaseDSN: filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")},
			})
			require.NoError(t, err)
			require.Error(t, brokenDispatchWorker.RunOnce(t.Context()))
		})

		t.Run("marks exhausted jobs failed before execution", func(t *testing.T) {
			store := makeStore(t)
			job := makeJob(now)
			job.Status = JobStatusRunning
			job.AttemptCount = 4
			job.MaxAttempts = 1
			startedAt := now.Add(-time.Minute)
			job.StartedAt = &startedAt
			job.LastAttemptAt = &startedAt
			_, err := store.Create(t.Context(), job)
			require.NoError(t, err)

			worker := makeWorker(t, store, &stubRunner{}, now)
			require.NoError(t, worker.ProcessJob(t.Context(), job.ID))

			persisted, err := store.Get(t.Context(), job.ID)
			require.NoError(t, err)
			assert.Equal(t, JobStatusFailed, persisted.Status)
			require.NotNil(t, persisted.Error)
			assert.Equal(t, "job_attempts_exhausted", persisted.Error.Code)
		})

		t.Run("ignores non observable progress and tolerates progress write failures", func(t *testing.T) {
			makeRegistry := func(t *testing.T) *Registry {
				t.Helper()
				registry := NewRegistry()
				require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[
					HistoricalRawCandleBackfillInput,
					HistoricalRawCandleBackfillResult,
					string,
				]{
					JobType: JobTypeHistoricalRawCandleBackfill,
					Run: func(
						_ context.Context,
						_ HistoricalRawCandleBackfillInput,
						setProgress func(string) error,
					) (HistoricalRawCandleBackfillResult, error) {
						require.NoError(t, setProgress("phase-running"))
						return HistoricalRawCandleBackfillResult{}, nil
					},
				}))
				return registry
			}

			store := makeStore(t)
			worker, err := NewWorker(WorkerDeps{Store: store, Registry: makeRegistry(t)})
			require.NoError(t, err)
			require.NoError(t, worker.processEnvelope(t.Context(), appdispatch.Envelope{
				Version: appdispatch.EnvelopeVersionV1,
				Kind:    DispatchKindHistoricalRawCandleBackfill,
				Payload: mustEncodeWorkerPayload(t, makeJob(now).Input),
			}))

			observableStore := makeStore(t)
			job := makeJob(now.Add(time.Minute))
			job.InputJSON = mustEncodeWorkerPayload(t, job.Input)
			_, err = observableStore.Create(t.Context(), job)
			require.NoError(t, err)
			worker, err = NewWorker(WorkerDeps{
				Store: &observationStoreStub{
					base:              observableStore,
					updateProgressErr: errors.New("progress write boom"),
				},
				Registry: makeRegistry(t),
				Logger:   slog.Default(),
				Clock:    func() time.Time { return now.Add(2 * time.Minute) },
			})
			require.NoError(t, err)
			require.NoError(t, worker.processEnvelope(t.Context(), appdispatch.Envelope{
				Version:         appdispatch.EnvelopeVersionV1,
				Kind:            DispatchKindHistoricalRawCandleBackfill,
				Payload:         mustEncodeWorkerPayload(t, job.Input),
				ObservableJobID: job.ID,
			}))
		})
	})

	t.Run("prepares observable jobs across duplicate and missing branches", func(t *testing.T) {
		now := time.Now().UTC()
		store := makeStore(t)
		executor := &workerExecutor{
			store:    store,
			registry: NewRegistry(),
			logger:   slog.Default(),
			clock:    func() time.Time { return now.Add(time.Minute) },
			workerID: "worker-prepare",
		}
		handler := &registeredTypedHandler[HistoricalRawCandleBackfillInput, HistoricalRawCandleBackfillResult, struct{}]{
			spec: TypedHandlerSpec[HistoricalRawCandleBackfillInput, HistoricalRawCandleBackfillResult, struct{}]{
				JobType:                JobTypeHistoricalRawCandleBackfill,
				GuardDuplicateDelivery: true,
				Run: func(context.Context, HistoricalRawCandleBackfillInput, func(struct{}) error) (HistoricalRawCandleBackfillResult, error) {
					return HistoricalRawCandleBackfillResult{}, nil
				},
			},
		}

		observableJob, skipExecution, err := executor.prepareObservableJob(t.Context(), "", handler)
		require.NoError(t, err)
		assert.Nil(t, observableJob)
		assert.False(t, skipExecution)

		observableJob, skipExecution, err = executor.prepareObservableJob(t.Context(), "missing-job", handler)
		require.NoError(t, err)
		assert.Nil(t, observableJob)
		assert.False(t, skipExecution)

		terminalJob := makeJob(now)
		terminalJob.Status = JobStatusSucceeded
		terminalJob.CompletedAt = ptrWorkerTime(now)
		_, err = store.Create(t.Context(), terminalJob)
		require.NoError(t, err)
		observableJob, skipExecution, err = executor.prepareObservableJob(t.Context(), terminalJob.ID, handler)
		require.NoError(t, err)
		assert.Nil(t, observableJob)
		assert.True(t, skipExecution)

		runningJob := makeJob(now.Add(time.Minute))
		runningJob.Status = JobStatusRunning
		startedAt := now.Add(time.Minute)
		runningJob.StartedAt = &startedAt
		runningJob.LastAttemptAt = &startedAt
		_, err = store.Create(t.Context(), runningJob)
		require.NoError(t, err)
		observableJob, skipExecution, err = executor.prepareObservableJob(
			t.Context(),
			runningJob.ID,
			&registeredTypedHandler[
				HistoricalRawCandleBackfillInput,
				HistoricalRawCandleBackfillResult,
				struct{},
			]{
				spec: TypedHandlerSpec[
					HistoricalRawCandleBackfillInput,
					HistoricalRawCandleBackfillResult,
					struct{},
				]{
					JobType: JobTypeHistoricalRawCandleBackfill,
					Run: func(
						context.Context,
						HistoricalRawCandleBackfillInput,
						func(struct{}) error,
					) (HistoricalRawCandleBackfillResult, error) {
						return HistoricalRawCandleBackfillResult{}, nil
					},
				},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, observableJob)
		assert.False(t, skipExecution)
		assert.Equal(t, JobStatusRunning, observableJob.Status)
	})
}

func mustEncodeWorkerPayload(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := EncodeJobPayload(value)
	require.NoError(t, err)
	return payload
}

func ptrWorkerTime(value time.Time) *time.Time {
	value = value.UTC()
	return &value
}
