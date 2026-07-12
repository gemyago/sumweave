package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/flows"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/google/uuid"
)

type historicalBackfillRunner interface {
	Run(
		ctx context.Context,
		request flows.HistoricalRawCandleBackfillRequest,
	) (flows.HistoricalRawCandleBackfillResult, error)
}

type workerStore interface {
	Get(context.Context, string) (*Job, error)
	List(context.Context, ListParams) (ListResult, error)
	ClaimQueued(context.Context, string, string, time.Time) (*Job, error)
	MarkSucceeded(context.Context, string, string, any, time.Time) error
	MarkFailed(context.Context, string, string, *JobError, time.Time) error
	UpdateProgress(context.Context, string, json.RawMessage, time.Time) error
	RecoverStaleRunning(context.Context, time.Time, int) error
}

type WorkerDeps struct {
	Store          workerStore
	Runner         historicalBackfillRunner
	Registry       *Registry
	Logger         *slog.Logger
	Clock          func() time.Time
	Config         WorkerConfig
	WorkerID       string
	DispatchDB     *sql.DB
	DispatchConfig DispatchConfig
}

type Worker struct {
	store          workerStore
	registry       *Registry
	logger         *slog.Logger
	clock          func() time.Time
	config         WorkerConfig
	workerID       string
	dispatchDB     *sql.DB
	dispatchConfig DispatchConfig
	consumer       *appdispatch.Consumer
	stop           context.CancelFunc
	wg             sync.WaitGroup
}

func NewWorker(deps WorkerDeps) (*Worker, error) {
	if deps.Store == nil {
		return nil, errors.New("jobs store is required")
	}
	if deps.Registry == nil {
		if deps.Runner == nil {
			return nil, errors.New("jobs registry is required")
		}
		deps.Registry = NewRegistry()
		if err := RegisterHistoricalBackfillHandler(deps.Registry, deps.Runner); err != nil {
			return nil, err
		}
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Clock == nil {
		deps.Clock = time.Now
	}
	if deps.WorkerID == "" {
		deps.WorkerID = "jobs-worker"
	}
	return &Worker{
		store:          deps.Store,
		registry:       deps.Registry,
		logger:         deps.Logger,
		clock:          deps.Clock,
		config:         normalizeWorkerConfig(deps.Config),
		workerID:       deps.WorkerID,
		dispatchDB:     deps.DispatchDB,
		dispatchConfig: deps.DispatchConfig,
	}, nil
}

func (w *Worker) Start(ctx context.Context) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.store.RecoverStaleRunning(ctx, w.clock(), w.config.MaxAttempts); err != nil {
		return err
	}
	if err := w.ensureConsumer(); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.stop = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.consumer.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			w.logger.ErrorContext(runCtx, "jobs worker consumer stopped", "error", err)
		}
	}()
	return nil
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.store.RecoverStaleRunning(ctx, w.clock(), w.config.MaxAttempts); err != nil {
		return err
	}
	if err := w.ensureConsumer(); err != nil {
		return err
	}
	return w.consumer.Run(ctx)
}

func (w *Worker) RunOnce(ctx context.Context) error {
	if err := w.store.RecoverStaleRunning(ctx, w.clock(), w.config.MaxAttempts); err != nil {
		return err
	}
	if err := w.ensureConsumer(); err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, 2*w.config.PollInterval)
	defer cancel()
	err := w.consumer.Run(runCtx)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (w *Worker) Stop(_ context.Context) error {
	if w.stop != nil {
		w.stop()
	}
	w.wg.Wait()
	if w.consumer == nil {
		return nil
	}
	return w.consumer.Close()
}

func (w *Worker) ensureConsumer() error {
	if w.consumer != nil {
		return nil
	}
	config := appdispatch.Config{
		DatabaseDSN: w.dispatchConfig.DatabaseDSN,
		TablePrefix: w.dispatchConfig.TablePrefix,
	}
	registry := newWorkerDispatchRegistry(w.registry, &workerExecutor{
		store:    w.store,
		registry: w.registry,
		logger:   w.logger,
		clock:    w.clock,
		workerID: w.workerID,
	})
	if w.dispatchDB == nil {
		return errors.New("dispatch sql database is required")
	}
	consumer, err := appdispatch.NewConsumer(config, w.dispatchDB, registry, w.logger)
	if err != nil {
		return err
	}
	w.consumer = consumer
	return nil
}
func (w *Worker) ProcessJob(ctx context.Context, jobID string) error {
	job, err := w.store.Get(ctx, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil
		}
		return err
	}
	return w.processEnvelope(ctx, appdispatch.Envelope{
		Version:         appdispatch.EnvelopeVersionV1,
		Kind:            dispatchKindForJobType(job.JobType),
		Payload:         job.InputJSON,
		ObservableJobID: job.ID,
	})
}

type workerExecutor struct {
	store    workerStore
	registry *Registry
	logger   *slog.Logger
	clock    func() time.Time
	workerID string
}

func newWorkerDispatchRegistry(registry *Registry, executor *workerExecutor) *appdispatch.HandlerRegistry {
	dispatchRegistry := appdispatch.NewHandlerRegistry()
	registry.mu.RLock()
	kinds := make([]appdispatch.ExecutionKind, 0, len(registry.dispatchHandlers))
	for kind := range registry.dispatchHandlers {
		kinds = append(kinds, kind)
	}
	registry.mu.RUnlock()
	for _, kind := range kinds {
		dispatchKind := kind
		if err := appdispatch.RegisterTypedHandler(dispatchRegistry, appdispatch.TypedHandlerSpec[json.RawMessage]{
			Kind: dispatchKind,
			Run: func(ctx context.Context, envelope appdispatch.Envelope, _ json.RawMessage) error {
				return executor.processEnvelope(ctx, envelope)
			},
		}); err != nil {
			panic(err)
		}
	}
	return dispatchRegistry
}

func (w *Worker) processEnvelope(ctx context.Context, envelope appdispatch.Envelope) error {
	executor := &workerExecutor{
		store:    w.store,
		registry: w.registry,
		logger:   w.logger.WithGroup("workerExecutor"),
		clock:    w.clock,
		workerID: w.workerID,
	}
	return executor.processEnvelope(ctx, envelope)
}

func (w *workerExecutor) processEnvelope(ctx context.Context, envelope appdispatch.Envelope) error {
	ctx = telemetry.SetLogAttributesToContext(ctx, telemetry.LogAttributes{
		CorrelationID: slog.StringValue(uuid.NewString()),
	})
	w.logger.InfoContext(ctx, "processing message",
		slog.String("messageId", envelope.ObservableJobID),
		slog.String("requesterId", envelope.RequesterID),
		slog.String("correlationId", envelope.CorrelationID),
		slog.String("kind", string(envelope.Kind)),
	)
	handler, err := w.registry.HandlerByExecutionKind(envelope.Kind)
	if err != nil {
		if envelope.ObservableJobID != "" {
			return w.store.MarkFailed(ctx, envelope.ObservableJobID, w.workerID, jobErrorFromExecution(err), w.clock())
		}
		return err
	}
	job := Job{JobType: handler.jobType(), InputJSON: envelope.Payload}
	observableJob, skipExecution, claimErr := w.prepareObservableJob(ctx, envelope.ObservableJobID, handler)
	if claimErr != nil {
		return claimErr
	}
	if skipExecution {
		return nil
	}
	if observableJob != nil {
		job = *observableJob
		job.InputJSON = envelope.Payload
	}
	maxAttempts := job.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = handler.maxAttempts()
	}
	if job.AttemptCount > maxAttempts {
		return w.store.MarkFailed(ctx, job.ID, w.workerID, &JobError{
			Code:    "job_attempts_exhausted",
			Summary: "job attempts exhausted",
			Details: "job exceeded max attempts before execution",
		}, w.clock())
	}
	resultJSON, runErr := handler.execute(ctx, job, func(progressJSON json.RawMessage) error {
		if envelope.ObservableJobID == "" {
			return nil
		}
		if updateErr := w.store.UpdateProgress(
			ctx,
			envelope.ObservableJobID,
			progressJSON,
			w.clock(),
		); updateErr != nil {
			w.logger.WarnContext(
				ctx,
				"job progress observation update failed",
				"jobId",
				envelope.ObservableJobID,
				"error",
				updateErr,
			)
		}
		return nil
	})
	if runErr != nil {
		if envelope.ObservableJobID == "" {
			return runErr
		}
		return w.store.MarkFailed(ctx, envelope.ObservableJobID, w.workerID, jobErrorFromExecution(runErr), w.clock())
	}
	if envelope.ObservableJobID == "" {
		return nil
	}
	if markErr := w.store.MarkSucceeded(
		ctx,
		envelope.ObservableJobID,
		w.workerID,
		resultJSON,
		w.clock(),
	); markErr != nil {
		w.logger.WarnContext(
			ctx,
			"job terminal observation update failed after successful work",
			"jobId",
			envelope.ObservableJobID,
			"error",
			markErr,
		)
		return nil
	}
	return nil
}

func (w *workerExecutor) prepareObservableJob(
	ctx context.Context,
	jobID string,
	handler typedHandler,
) (*Job, bool, error) {
	if jobID == "" {
		return nil, false, nil
	}
	job, err := w.store.Get(ctx, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if handler.guardDuplicateDelivery() {
		if job.Status == JobStatusSucceeded || job.Status == JobStatusFailed || job.Status == JobStatusCanceled {
			return nil, true, nil
		}
	}
	claimed, err := w.store.ClaimQueued(ctx, jobID, w.workerID, w.clock())
	if err == nil {
		return claimed, false, nil
	}
	if errors.Is(err, ErrJobNotQueued) {
		return job, false, nil
	}
	return nil, false, err
}

func RegisterHistoricalBackfillHandler(registry *Registry, runner historicalBackfillRunner) error {
	if runner == nil {
		return errors.New("historical backfill runner is required")
	}
	return RegisterTypedHandler(
		registry,
		TypedHandlerSpec[HistoricalRawCandleBackfillInput, HistoricalRawCandleBackfillResult, struct{}]{
			JobType:     JobTypeHistoricalRawCandleBackfill,
			MaxAttempts: defaultWorkerMaxAttempts,
			Run: func(ctx context.Context, input HistoricalRawCandleBackfillInput, _ func(struct{}) error) (HistoricalRawCandleBackfillResult, error) {
				input = canonicalizeHistoricalInput(input)
				request := flows.HistoricalRawCandleBackfillRequest{
					RunID:      input.IngestionRunID,
					Venue:      domain.Venue(input.Venue),
					Symbol:     domain.Symbol(input.Symbol),
					AssetClass: domain.AssetClass(input.AssetClass),
					Timeframe:  domain.Timeframe(input.Timeframe),
					TimeRange:  input.TimeRange,
					PageSize:   input.PageSize,
				}
				runResult, err := runner.Run(ctx, request)
				if err != nil {
					return HistoricalRawCandleBackfillResult{}, err
				}
				result := HistoricalRawCandleBackfillResult{
					IngestionRunID:            runResult.RunID,
					PersistedCount:            runResult.Report.PersistedCount,
					ExpectedCount:             runResult.Report.ExpectedCount,
					MissingIntervalCount:      runResult.Report.MissingIntervalCount,
					DuplicateNaturalKeyCount:  runResult.Report.DuplicateNaturalKeyCount,
					FirstPersistedStart:       runResult.Report.FirstPersistedStart,
					LastPersistedEnd:          runResult.Report.LastPersistedEnd,
					RawPayloadCount:           runResult.Report.RawPayloadCount,
					MissingIntervalPreviewCap: runResult.Report.MissingIntervalPreviewLimit,
				}
				for _, interval := range runResult.Report.MissingIntervalPreview {
					result.MissingIntervalPreview = append(result.MissingIntervalPreview, jobTimeRange{
						Start: interval.Start,
						End:   interval.End,
					})
				}
				return result, nil
			},
		},
	)
}

func NewHistoricalBackfillRunner(
	lineageService *data.LineageService,
	readService *data.ReadService,
	ingestionFlow *venueedge.IngestionFlow,
) (*flows.HistoricalRawCandleBackfillRunner, error) {
	if lineageService == nil {
		return nil, errors.New("data lineage service is required")
	}
	if readService == nil {
		return nil, errors.New("data read service is required")
	}
	if ingestionFlow == nil {
		return nil, errors.New("venue ingestion flow is required")
	}
	recorder := &hyperliquidRawEvidenceRecorder{lineageService: lineageService}
	return flows.NewHistoricalRawCandleBackfillRunner(
		flows.HistoricalRawCandleBackfillRunnerDeps{
			RecordIngestionRun: lineageService.RecordIngestionRun,
			BuildVenue: func(
				_ context.Context,
				params flows.HistoricalRawCandleBackfillVenueBuildParams,
			) (venueedge.MarketDataVenue, error) {
				return venueedge.NewHyperliquidPerpsVenue(venueedge.HyperliquidPerpsVenueParams{
					BaseURL:                 defaultHistoricalBackfillBaseURL,
					HTTPClient:              http.DefaultClient,
					RawEvidenceRecorder:     recorder,
					RawEvidenceIngestionRun: params.RawEvidenceIngestionRun,
				})
			},
			IngestCandles:          ingestionFlow.IngestCandles,
			ReadPersistedCandles:   readService.QueryCandles,
			ReplayPersistedCandles: readService.ReplayCandles,
		},
	)
}

type hyperliquidRawEvidenceRecorder struct {
	lineageService *data.LineageService
}

func (r *hyperliquidRawEvidenceRecorder) RecordHyperliquidRawEvidence(
	ctx context.Context,
	capture venueedge.HyperliquidRawEvidenceCapture,
) (string, error) {
	payload, err := data.NewRawVenuePayload(data.RawVenuePayloadParams{
		ID:                 capture.ID,
		IngestionRunID:     capture.IngestionRunID,
		Source:             string(capture.Venue) + "-rest",
		Venue:              capture.Venue,
		Endpoint:           capture.Endpoint,
		RequestType:        capture.RequestType,
		RequestPayloadHash: capture.RequestPayloadHash,
		RequestMetadata:    capture.RequestMetadata,
		RequestAt:          capture.RequestAt,
		ResponseAt:         capture.ResponseAt,
		HTTPStatus:         capture.HTTPStatus,
		ResponseBody:       capture.ResponseBody,
		EntityHint:         capture.EntityHint,
		Instrument:         rawEvidenceInstrumentRef(capture.Instrument),
		Timeframe:          capture.Timeframe,
		TimeRange:          capture.TimeRange,
		ReceivedAt:         capture.ReceivedAt,
	})
	if err != nil {
		return "", fmt.Errorf("build raw venue payload: %w", err)
	}
	persisted, err := r.lineageService.RecordRawVenuePayload(ctx, payload)
	if err != nil {
		return "", err
	}
	return persisted.ID, nil
}

func rawEvidenceInstrumentRef(instrument *domain.Instrument) *data.BatchInstrumentRef {
	if instrument == nil {
		return nil
	}

	return &data.BatchInstrumentRef{Symbol: instrument.Symbol, AssetClass: instrument.AssetClass}
}
