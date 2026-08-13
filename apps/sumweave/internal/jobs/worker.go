package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/google/uuid"
)

const (
	terminalPersistenceRetryInterval    = 100 * time.Millisecond
	terminalPersistenceRecoveryInterval = time.Second
)

type workerStore interface {
	Get(context.Context, string) (*Job, error)
	ClaimQueued(context.Context, string, string, time.Time) (*Job, error)
	persistTerminalState(context.Context, string, terminalJobState) error
	UpdateProgress(context.Context, string, json.RawMessage, time.Time) error
	RecoverStaleRunning(context.Context, time.Time, int) error
}
type WorkerDeps struct {
	Store         workerStore
	Registry      *Registry
	Logger        *slog.Logger
	Clock         func() time.Time
	Config        WorkerConfig
	WorkerID      string
	RouterFactory *appdispatch.RouterFactory
}
type Worker struct {
	store     workerStore
	registry  *Registry
	logger    *slog.Logger
	clock     func() time.Time
	config    WorkerConfig
	workerID  string
	router    *appdispatch.Router
	mu        sync.Mutex
	lifecycle *workerLifecycle
	wg        sync.WaitGroup
}

type workerLifecycle struct {
	drainCtx    context.Context
	drainCancel context.CancelFunc
	stop        context.CancelFunc
	trackRun    bool
}

func NewWorker(deps WorkerDeps) (*Worker, error) {
	if deps.Store == nil {
		return nil, errors.New("jobs store is required")
	}
	if deps.Registry == nil {
		return nil, errors.New("jobs registry is required")
	}
	if deps.RouterFactory == nil {
		return nil, errors.New("jobs router factory is required")
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
	worker := &Worker{
		store:    deps.Store,
		registry: deps.Registry,
		logger:   deps.Logger,
		clock:    deps.Clock,
		config:   normalizeWorkerConfig(deps.Config),
		workerID: deps.WorkerID,
	}
	router, err := deps.RouterFactory.NewRouter(jobConsumerGroup)
	if err != nil {
		return nil, fmt.Errorf("create jobs router: %w", err)
	}
	handler, err := appdispatch.NewHandler(
		jobExecutionTopic,
		func(ctx context.Context, message appdispatch.Message) error {
			deliveryCtx, cancel := worker.deliveryContext(ctx)
			defer cancel()
			var envelope executionEnvelope
			if decodeErr := json.Unmarshal(message.Payload, &envelope); decodeErr != nil {
				return fmt.Errorf("decode job execution envelope: %w", decodeErr)
			}
			if envelope.Version != jobEnvelopeVersion {
				return fmt.Errorf("unsupported job envelope version: %s", envelope.Version)
			}
			return worker.processEnvelope(deliveryCtx, envelope)
		},
	)
	if err != nil {
		_ = router.Close()
		return nil, fmt.Errorf("create jobs execution handler: %w", err)
	}
	if err = router.Handle(handler); err != nil {
		_ = router.Close()
		return nil, fmt.Errorf("register jobs execution handler: %w", err)
	}
	worker.router = router
	return worker, nil
}
func (w *Worker) Start(ctx context.Context) error {
	if !w.config.Enabled {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	finishDrain, err := w.startDrain(runCtx, cancel, true)
	if err != nil {
		cancel()
		return err
	}
	if err = w.store.RecoverStaleRunning(runCtx, w.clock(), w.config.MaxAttempts); err != nil {
		cancel()
		finishDrain()
		return err
	}
	go func() {
		defer finishDrain()
		runErr := w.router.Run(runCtx)
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			w.logger.ErrorContext(runCtx, "jobs worker consumer stopped", "error", runErr)
		}
	}()
	return nil
}
func (w *Worker) Run(ctx context.Context) error {
	finishDrain, err := w.startDrain(ctx, nil, false)
	if err != nil {
		return err
	}
	defer finishDrain()
	if err = w.store.RecoverStaleRunning(ctx, w.clock(), w.config.MaxAttempts); err != nil {
		return err
	}
	return w.router.Run(ctx)
}
func (w *Worker) RunOnce(ctx context.Context) error {
	runCtx, cancel := context.WithTimeout(ctx, 2*w.config.PollInterval)
	defer cancel()
	finishDrain, err := w.startDrain(runCtx, nil, false)
	if err != nil {
		return err
	}
	defer finishDrain()
	if err = w.store.RecoverStaleRunning(runCtx, w.clock(), w.config.MaxAttempts); err != nil {
		return err
	}
	err = w.router.Run(runCtx)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
func (w *Worker) Stop(ctx context.Context) error {
	w.mu.Lock()
	lifecycle := w.lifecycle
	w.mu.Unlock()
	if lifecycle != nil && lifecycle.stop != nil {
		lifecycle.stop()
	}
	if lifecycle != nil {
		drainCtx, cancelDrain := context.WithTimeout(context.WithoutCancel(ctx), w.config.DrainTimeout)
		defer cancelDrain()
		stopDrain := context.AfterFunc(drainCtx, lifecycle.drainCancel)
		defer stopDrain()
	}
	w.wg.Wait()
	return w.router.Close()
}

func (w *Worker) startDrain(parent context.Context, stop context.CancelFunc, trackRun bool) (func(), error) {
	drainCtx, drainCancel := context.WithCancel(context.WithoutCancel(parent))
	done := make(chan struct{})
	watcherDone := make(chan struct{})
	lifecycle := &workerLifecycle{
		drainCtx: drainCtx, drainCancel: drainCancel, stop: stop, trackRun: trackRun,
	}
	w.mu.Lock()
	if w.lifecycle != nil {
		w.mu.Unlock()
		drainCancel()
		return nil, errors.New("jobs worker is already running")
	}
	if trackRun {
		w.wg.Add(1)
	}
	w.lifecycle = lifecycle
	w.mu.Unlock()
	go func() {
		defer close(watcherDone)
		select {
		case <-done:
			return
		case <-parent.Done():
		}
		timer := time.NewTimer(w.config.DrainTimeout)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			drainCancel()
		}
	}()
	var finishOnce sync.Once
	return func() {
		finishOnce.Do(func() {
			close(done)
			drainCancel()
			<-watcherDone
			w.mu.Lock()
			if w.lifecycle == lifecycle {
				w.lifecycle = nil
			}
			w.mu.Unlock()
			if lifecycle.trackRun {
				w.wg.Done()
			}
		})
	}, nil
}

func (w *Worker) deliveryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	deliveryCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	w.mu.Lock()
	lifecycle := w.lifecycle
	w.mu.Unlock()
	if lifecycle == nil {
		return deliveryCtx, cancel
	}
	stopDrain := context.AfterFunc(lifecycle.drainCtx, cancel)
	return deliveryCtx, func() {
		stopDrain()
		cancel()
	}
}
func (w *Worker) ProcessJob(ctx context.Context, jobID string) error {
	job, err := w.store.Get(ctx, jobID)
	if errors.Is(err, ErrJobNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	err = w.processEnvelope(
		ctx,
		executionEnvelope{
			Version:         jobEnvelopeVersion,
			Kind:            dispatchKindForJobType(job.JobType),
			Payload:         job.InputJSON,
			ObservableJobID: job.ID,
		},
	)
	var runningErr runningJobDeliveryError
	if errors.As(err, &runningErr) {
		return nil
	}
	return err
}

type workerExecutor struct {
	store    workerStore
	registry *Registry
	logger   *slog.Logger
	clock    func() time.Time
	workerID string
}

type runningJobDeliveryError struct{ jobID string }

func (e runningJobDeliveryError) Error() string {
	return fmt.Sprintf("job %s is still running and requires recovery", e.jobID)
}
func (runningJobDeliveryError) NonRetryable() {}

func (w *Worker) processEnvelope(ctx context.Context, envelope executionEnvelope) error {
	return (&workerExecutor{store: w.store, registry: w.registry, logger: w.logger.WithGroup("workerExecutor"), clock: w.clock, workerID: w.workerID}).processEnvelope(
		ctx,
		envelope,
	)
}
func (w *workerExecutor) processEnvelope(ctx context.Context, envelope executionEnvelope) error {
	ctx = telemetry.SetLogAttributesToContext(
		ctx,
		telemetry.LogAttributes{CorrelationID: slog.StringValue(uuid.NewString())},
	)
	handler, err := w.registry.handlerByExecutionKind(envelope.Kind)
	if err != nil {
		if envelope.ObservableJobID != "" {
			return w.persistFailedTerminalState(ctx, envelope.ObservableJobID, jobErrorFromExecution(err))
		}
		return err
	}
	job := Job{JobType: handler.jobType(), InputJSON: envelope.Payload}
	observableJob, skip, err := w.prepareObservableJob(ctx, envelope.ObservableJobID)
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
		return w.persistFailedTerminalState(ctx, envelope.ObservableJobID, jobErrorFromExecution(runErr))
	}
	if envelope.ObservableJobID == "" {
		return nil
	}
	resultJSON, encodeErr := resultJSONFromValue(result)
	if encodeErr != nil {
		return w.persistFailedTerminalState(ctx, envelope.ObservableJobID, jobResultEncodingError(encodeErr))
	}
	return w.persistPreparedTerminalState(
		ctx,
		envelope.ObservableJobID,
		newSucceededTerminalJobState(w.workerID, resultJSON, w.clock()),
	)
}

func (w *workerExecutor) persistFailedTerminalState(ctx context.Context, jobID string, jobErr *JobError) error {
	return w.persistPreparedTerminalState(ctx, jobID, newFailedTerminalJobState(w.workerID, jobErr, w.clock()))
}

func (w *workerExecutor) persistPreparedTerminalState(
	ctx context.Context,
	jobID string,
	state terminalJobState,
) error {
	if err := validateRequiredTimestamp("completedAt", state.completedAt); err != nil {
		return err
	}
	return w.persistTerminalState(ctx, jobID, string(state.status), func() error {
		return w.store.persistTerminalState(ctx, jobID, state)
	})
}

func (w *workerExecutor) persistTerminalState(
	ctx context.Context,
	jobID string,
	status string,
	persist func() error,
) error {
	retryInterval := terminalPersistenceRetryInterval
	for {
		err := persist()
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Join(ctxErr, fmt.Errorf("persist job terminal state: %w", err))
		}
		w.logger.ErrorContext(ctx, "persist job terminal state failed", "jobId", jobID, "status", status, "error", err)
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ctx.Err(), fmt.Errorf("persist job terminal state: %w", err))
		case <-timer.C:
			if retryInterval < terminalPersistenceRecoveryInterval {
				retryInterval *= 2
				if retryInterval > terminalPersistenceRecoveryInterval {
					retryInterval = terminalPersistenceRecoveryInterval
				}
			}
		}
	}
}

func (w *workerExecutor) prepareObservableJob(
	ctx context.Context,
	jobID string,
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
	if job.Status == JobStatusRunning {
		return nil, false, runningJobDeliveryError{jobID: jobID}
	}
	if job.Status != JobStatusQueued {
		return nil, true, nil
	}
	claimed, err := w.store.ClaimQueued(ctx, jobID, w.workerID, w.clock())
	if err == nil {
		return claimed, false, nil
	}
	if errors.Is(err, ErrJobNotQueued) {
		return nil, true, nil
	}
	return nil, false, err
}
