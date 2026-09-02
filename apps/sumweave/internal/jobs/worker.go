package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
)

const (
	terminalPersistenceRetryInterval    = 100 * time.Millisecond
	terminalPersistenceRecoveryInterval = time.Second
)

type workerStore interface {
	MaterializeQueued(context.Context, Job) (*Job, error)
	ClaimQueued(context.Context, string, string, time.Time) (*Job, error)
	RequeueRunning(context.Context, Job, time.Time) error
	persistTerminalState(context.Context, Job, terminalJobState) error
	FinalizeRetryExhausted(context.Context, string, time.Time, terminalJobState) error
	RenewRunning(context.Context, Job, time.Time) error
	RecoverStaleRunning(context.Context, time.Time, time.Duration, int) error
}

type workerRouter interface {
	SetRetryLifecycle(appdispatch.RetryLifecycle) error
	Handle(appdispatch.Handler) error
	Run(context.Context) error
	Close() error
}

type workerRouterFactory interface {
	NewRouter(string) (workerRouterResult, error)
}

type workerRouterResult struct {
	router workerRouter
}

type appdispatchWorkerRouterFactory struct {
	newRouter func(string) (*appdispatch.Router, error)
}

func (f appdispatchWorkerRouterFactory) NewRouter(consumerGroup string) (workerRouterResult, error) {
	router, err := f.newRouter(consumerGroup)
	return workerRouterResult{router: router}, err
}

type WorkerDeps struct {
	Store         workerStore
	Registry      *Registry
	Logger        *slog.Logger
	Clock         func() time.Time
	Config        WorkerConfig
	WorkerID      string
	RouterFactory *appdispatch.RouterFactory
	routerFactory workerRouterFactory
}

type Worker struct {
	store     workerStore
	registry  *Registry
	logger    *slog.Logger
	clock     func() time.Time
	config    WorkerConfig
	workerID  string
	router    workerRouter
	mu        sync.Mutex
	installed bool
	runOnce   *runOnceTracker
	claims    map[string]Job
}

func NewWorker(deps WorkerDeps) (*Worker, error) {
	if deps.Store == nil {
		return nil, errors.New("jobs store is required")
	}
	if deps.Registry == nil {
		return nil, errors.New("jobs registry is required")
	}
	if deps.RouterFactory == nil && deps.routerFactory == nil {
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
	routerFactory := deps.routerFactory
	if routerFactory == nil {
		routerFactory = appdispatchWorkerRouterFactory{newRouter: deps.RouterFactory.NewRouter}
	}
	routerResult, err := routerFactory.NewRouter(jobConsumerGroup)
	if err != nil {
		return nil, fmt.Errorf("create jobs router: %w", err)
	}
	router := routerResult.router
	worker := &Worker{
		store: deps.Store, registry: deps.Registry, logger: deps.Logger, clock: deps.Clock,
		config: normalizeWorkerConfig(deps.Config), workerID: deps.WorkerID, router: router,
		claims: make(map[string]Job),
	}
	if lifecycleErr := router.SetRetryLifecycle(appdispatch.RetryLifecycle{
		OnRetry: worker.startRunOnceRetry, OnRetriesExhausted: worker.finishRunOnceRetry,
	}); lifecycleErr != nil {
		return nil, fmt.Errorf("set jobs retry lifecycle: %w", lifecycleErr)
	}
	return worker, nil
}

func (w *Worker) Run(
	ctx context.Context,
) error {
	if err := w.installHandlers(); err != nil {
		return err
	}
	if err := w.recoverStaleRunning(ctx); err != nil {
		return err
	}
	stopRecovery, recoveryDone := w.startPeriodicRecovery(ctx)
	err := w.router.Run(ctx)
	close(stopRecovery)
	<-recoveryDone
	return err
}

func (w *Worker) RunOnce(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := w.installHandlers(); err != nil {
		return err
	}
	if err := w.recoverStaleRunning(runCtx); err != nil {
		return err
	}
	stopRecovery, recoveryDone := w.startPeriodicRecovery(runCtx)
	defer func() {
		close(stopRecovery)
		<-recoveryDone
	}()
	tracker := &runOnceTracker{}
	w.mu.Lock()
	w.runOnce = tracker
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.runOnce = nil
		w.mu.Unlock()
	}()
	runDone := make(chan error, 1)
	go func() { runDone <- w.router.Run(runCtx) }()
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()
	idlePolls := 0
	for {
		select {
		case err := <-runDone:
			return w.runOnceResult(err)
		case <-ticker.C:
			if !tracker.isIdle() {
				idlePolls = 0
				continue
			}
			idlePolls++
			if idlePolls < 2 {
				continue
			}
			cancel()
			return w.runOnceResult(<-runDone)
		}
	}
}

func (w *Worker) runOnceResult(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (w *Worker) Stop(context.Context) error {
	return w.router.Close()
}

func (w *Worker) installHandlers() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.installed {
		return nil
	}
	for _, registered := range w.registry.Handlers() {
		handler := registered
		transportHandler, err := appdispatch.NewHandler(
			handler.topic(),
			func(ctx context.Context, message appdispatch.Message) error {
				return w.processObserved(ctx, handler, message)
			},
		)
		if err != nil {
			return fmt.Errorf("create observed handler: %w", err)
		}
		if err = w.router.Handle(transportHandler); err != nil {
			return fmt.Errorf("register observed handler: %w", err)
		}
	}
	w.installed = true
	return nil
}

type runningJobDeliveryError struct{ jobID string }

func (e runningJobDeliveryError) Error() string {
	return fmt.Sprintf("job %s is still running and requires recovery", e.jobID)
}

// NonRetryable leaves a concurrent delivery pending for stale-running recovery
// instead of executing the command concurrently.
func (runningJobDeliveryError) NonRetryable() {}

func (w *Worker) processObserved(ctx context.Context, handler observedHandler, message appdispatch.Message) error {
	tracker := w.runOnceTracker()
	if tracker != nil {
		tracker.startDelivery(message.ID)
		defer tracker.finishDelivery(message.ID)
	}
	metadata, err := handler.metadata(message.Payload)
	if err != nil {
		return err
	}
	now := w.clock()
	job, err := w.store.MaterializeQueued(ctx, Job{
		ID:                 message.ID,
		JobType:            metadata.JobType,
		Status:             JobStatusQueued,
		Requester:          canonicalizeRequester(metadata.Requester),
		CreatedAt:          now,
		UpdatedAt:          now,
		QueuedAt:           now,
		ScheduleID:         metadata.ScheduleID,
		ScheduledAt:        metadata.ScheduledAt,
		ScheduledNextRunAt: metadata.ScheduledNextRunAt,
	})
	if err != nil {
		return fmt.Errorf("materialize observed job: %w", err)
	}
	if job.Status == JobStatusRunning {
		return runningJobDeliveryError{jobID: job.ID}
	}
	if job.Status != JobStatusQueued {
		return nil
	}
	claimed, err := w.store.ClaimQueued(ctx, job.ID, w.workerID, w.clock())
	if errors.Is(
		err,
		ErrJobNotQueued,
	) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("claim observed job: %w", err)
	}
	w.trackClaim(*claimed)
	defer w.releaseClaim(claimed.ID)
	return w.executeClaimedObserved(ctx, handler, message, claimed)
}

func (w *Worker) executeClaimedObserved(
	ctx context.Context,
	handler observedHandler,
	message appdispatch.Message,
	claimed *Job,
) error {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			if requeueErr := w.store.RequeueRunning(ctx, *claimed, w.clock()); requeueErr != nil {
				panic(fmt.Errorf("requeue panicked observed job: %w", requeueErr))
			}
			panic(panicValue)
		}
	}()
	if err := handler.execute(ctx, *claimed, message.Payload); err != nil {
		if failure, ok := appdispatch.BusinessFailureFrom(err); ok {
			return w.persistTerminalState(ctx, *claimed, newFailedTerminalJobState(
				w.workerID, jobErrorFromBusinessFailure(failure), w.clock(),
			))
		}
		queuedAt := w.clock()
		if requeueErr := w.store.RequeueRunning(ctx, *claimed, queuedAt); requeueErr != nil {
			return fmt.Errorf("requeue transient observed job: %w", requeueErr)
		}
		return exhaustedRetryError{
			err: err,
			finalize: func() error {
				finalizeErr := w.finalizeRetryExhausted(ctx, claimed.ID, queuedAt, newFailedTerminalJobState(
					w.workerID, jobErrorFromExecution(err), w.clock(),
				))
				return finalizeErr
			},
		}
	}
	return w.persistTerminalState(
		ctx,
		*claimed,
		newSucceededTerminalJobState(w.workerID, w.clock()),
	)
}

type exhaustedRetryError struct {
	err      error
	finalize func() error
}

func (e exhaustedRetryError) Error() string { return e.err.Error() }

func (e exhaustedRetryError) Unwrap() error { return e.err }

func (e exhaustedRetryError) OnRetriesExhausted() error {
	if e.finalize == nil {
		return nil
	}
	return e.finalize()
}

type runOnceTracker struct {
	mu             sync.Mutex
	active         map[string]int
	pendingRetries map[string]struct{}
}

func (t *runOnceTracker) startDelivery(jobID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active == nil {
		t.active = make(map[string]int)
	}
	t.active[jobID]++
	delete(t.pendingRetries, jobID)
}

func (t *runOnceTracker) finishDelivery(jobID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active[jobID] <= 1 {
		delete(t.active, jobID)
		return
	}
	t.active[jobID]--
}

func (t *runOnceTracker) startRetry(jobID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pendingRetries == nil {
		t.pendingRetries = make(map[string]struct{})
	}
	t.pendingRetries[jobID] = struct{}{}
}

func (t *runOnceTracker) finishRetry(jobID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pendingRetries, jobID)
}

func (t *runOnceTracker) isIdle() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.active) == 0 && len(t.pendingRetries) == 0
}

func (w *Worker) runOnceTracker() *runOnceTracker {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.runOnce
}

func (w *Worker) startRunOnceRetry(messageID string) {
	if tracker := w.runOnceTracker(); tracker != nil {
		tracker.startRetry(messageID)
	}
}

func (w *Worker) finishRunOnceRetry(messageID string) {
	if tracker := w.runOnceTracker(); tracker != nil {
		tracker.finishRetry(messageID)
	}
}

func (w *Worker) recoverStaleRunning(ctx context.Context) error {
	return w.store.RecoverStaleRunning(
		ctx,
		w.clock(),
		w.config.StaleRunningAge,
		w.config.MaxAttempts,
	)
}

func (w *Worker) startPeriodicRecovery(ctx context.Context) (chan struct{}, <-chan struct{}) {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.recoverStaleRunningPeriodically(ctx, stop)
	}()
	return stop, done
}

func (w *Worker) recoverStaleRunningPeriodically(ctx context.Context, stop <-chan struct{}) {
	ticker := time.NewTicker(w.recoveryInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			if !w.runPeriodicRecoveryCycle(ctx) {
				return
			}
		}
	}
}

func (w *Worker) runPeriodicRecoveryCycle(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	w.renewRunningClaims(ctx)
	if ctx.Err() != nil {
		return false
	}
	if err := w.recoverStaleRunning(ctx); err != nil {
		w.logger.ErrorContext(ctx, "recover stale running jobs failed", "error", err)
	}
	return true
}

func (w *Worker) recoveryInterval() time.Duration {
	interval := w.config.PollInterval
	if renewalInterval := w.config.StaleRunningAge / 2; renewalInterval > 0 && renewalInterval < interval {
		return renewalInterval
	}
	return interval
}

func (w *Worker) trackClaim(claim Job) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.claims == nil {
		w.claims = make(map[string]Job)
	}
	w.claims[claim.ID] = claim
}

func (w *Worker) releaseClaim(jobID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.claims, jobID)
}

func (w *Worker) runningClaims() []Job {
	w.mu.Lock()
	defer w.mu.Unlock()
	claims := make([]Job, 0, len(w.claims))
	for _, claim := range w.claims {
		claims = append(claims, claim)
	}
	return claims
}

func (w *Worker) renewRunningClaims(ctx context.Context) {
	for _, claim := range w.runningClaims() {
		if err := w.store.RenewRunning(ctx, claim, w.clock()); err != nil {
			w.logger.ErrorContext(ctx, "renew running job claim failed", "jobId", claim.ID, "error", err)
		}
	}
}

func (w *Worker) persistTerminalState(
	ctx context.Context,
	claim Job,
	state terminalJobState,
) error {
	return w.persistTerminal(ctx, claim.ID, state, func(persistCtx context.Context) error {
		return w.store.persistTerminalState(persistCtx, claim, state)
	})
}

func (w *Worker) finalizeRetryExhausted(
	ctx context.Context,
	jobID string,
	queuedAt time.Time,
	state terminalJobState,
) error {
	return w.persistTerminal(ctx, jobID, state, func(persistCtx context.Context) error {
		return w.store.FinalizeRetryExhausted(persistCtx, jobID, queuedAt, state)
	})
}

func (w *Worker) persistTerminal(
	ctx context.Context,
	jobID string,
	state terminalJobState,
	persist func(context.Context) error,
) error {
	if err := validateRequiredTimestamp("completedAt", state.completedAt); err != nil {
		return err
	}
	retryInterval := terminalPersistenceRetryInterval
	for {
		err := persist(ctx)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrJobClaimLost) {
			return err
		}
		if ctx.Err() != nil {
			return errors.Join(ctx.Err(), fmt.Errorf("persist job terminal state: %w", err))
		}
		w.logger.ErrorContext(
			ctx,
			"persist job terminal state failed",
			"jobId",
			jobID,
			"status",
			state.status,
			"error",
			err,
		)
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
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
