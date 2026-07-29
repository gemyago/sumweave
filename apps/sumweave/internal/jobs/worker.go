package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/google/uuid"
)

type workerStore interface {
	Get(context.Context, string) (*Job, error)
	ClaimQueued(context.Context, string, string, time.Time) (*Job, error)
	MarkSucceeded(context.Context, string, string, any, time.Time) error
	MarkFailed(context.Context, string, string, *JobError, time.Time) error
	UpdateProgress(context.Context, string, json.RawMessage, time.Time) error
	RecoverStaleRunning(context.Context, time.Time, int) error
}
type WorkerDeps struct {
	Store          workerStore
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
		return nil, errors.New("jobs registry is required")
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
	if w.dispatchDB == nil {
		return errors.New("dispatch sql database is required")
	}
	registry := newWorkerDispatchRegistry(
		w.registry,
		&workerExecutor{store: w.store, registry: w.registry, logger: w.logger, clock: w.clock, workerID: w.workerID},
	)
	consumer, err := appdispatch.NewConsumer(
		appdispatch.Config{DatabaseDSN: w.dispatchConfig.DatabaseDSN, TablePrefix: w.dispatchConfig.TablePrefix},
		w.dispatchDB,
		registry,
		w.logger,
	)
	if err != nil {
		return err
	}
	w.consumer = consumer
	return nil
}
func (w *Worker) ProcessJob(ctx context.Context, jobID string) error {
	job, err := w.store.Get(ctx, jobID)
	if errors.Is(err, ErrJobNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return w.processEnvelope(
		ctx,
		appdispatch.Envelope{
			Version:         appdispatch.EnvelopeVersionV1,
			Kind:            dispatchKindForJobType(job.JobType),
			Payload:         job.InputJSON,
			ObservableJobID: job.ID,
		},
	)
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
		if err := appdispatch.RegisterTypedHandler(
			dispatchRegistry,
			appdispatch.TypedHandlerSpec[json.RawMessage]{
				Kind: dispatchKind,
				Run: func(ctx context.Context, envelope appdispatch.Envelope, _ json.RawMessage) error {
					return executor.processEnvelope(ctx, envelope)
				},
			},
		); err != nil {
			panic(err)
		}
	}
	return dispatchRegistry
}
func (w *Worker) processEnvelope(ctx context.Context, envelope appdispatch.Envelope) error {
	return (&workerExecutor{store: w.store, registry: w.registry, logger: w.logger.WithGroup("workerExecutor"), clock: w.clock, workerID: w.workerID}).processEnvelope(
		ctx,
		envelope,
	)
}
func (w *workerExecutor) processEnvelope(ctx context.Context, envelope appdispatch.Envelope) error {
	ctx = telemetry.SetLogAttributesToContext(
		ctx,
		telemetry.LogAttributes{CorrelationID: slog.StringValue(uuid.NewString())},
	)
	handler, err := w.registry.HandlerByExecutionKind(envelope.Kind)
	if err != nil {
		if envelope.ObservableJobID != "" {
			return w.store.MarkFailed(ctx, envelope.ObservableJobID, w.workerID, jobErrorFromExecution(err), w.clock())
		}
		return err
	}
	job := Job{JobType: handler.jobType(), InputJSON: envelope.Payload}
	observableJob, skip, err := w.prepareObservableJob(ctx, envelope.ObservableJobID, handler)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	if observableJob != nil {
		job = *observableJob
		job.InputJSON = envelope.Payload
	}
	result, runErr := handler.execute(ctx, job, func(progress json.RawMessage) error {
		if envelope.ObservableJobID == "" {
			return nil
		}
		return w.store.UpdateProgress(ctx, envelope.ObservableJobID, progress, w.clock())
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
	return w.store.MarkSucceeded(ctx, envelope.ObservableJobID, w.workerID, result, w.clock())
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
	if errors.Is(err, ErrJobNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if handler.guardDuplicateDelivery() &&
		(job.Status == JobStatusSucceeded || job.Status == JobStatusFailed || job.Status == JobStatusCanceled) {
		return nil, true, nil
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
