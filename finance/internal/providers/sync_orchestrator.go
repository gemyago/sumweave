package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

var (
	ErrSyncStateJournalRequired   = errors.New("sync state journal is required")
	ErrTargetWindowPolicyRequired = errors.New("target window policy is required")
	ErrWindowChunkPolicyRequired  = errors.New("window chunk policy is required")
	ErrWindowExecutorRequired     = errors.New("window executor is required")
)

type SyncStateJournal interface {
	LoadLastState(
		ctx context.Context,
		connection domain.ProviderConnectionRef,
	) (*domain.ProviderSyncState, error)
	AppendSyncState(ctx context.Context, state domain.ProviderSyncState) error
}

type TargetWindowPolicy interface {
	Determine(now time.Time, state *domain.ProviderSyncState) (domain.ProviderSyncWindow, error)
}

type WindowChunkPolicy interface {
	Split(window domain.ProviderSyncWindow) ([]domain.ProviderSyncWindow, error)
}

type WindowExecutor interface {
	Execute(ctx context.Context, request WindowSyncRequest) (WindowSyncResult, error)
}

type SyncOrchestrationRequest struct {
	Connection domain.ProviderConnectionRef
	Secret     domain.ConnectionSecret
	JobID      string
	Reason     string
}

type SyncOrchestrationResult struct {
	TargetWindow    domain.ProviderSyncWindow
	ExecutedWindows []domain.ProviderSyncWindow
	WindowResults   []WindowSyncResult
	Stats           domain.ProviderSyncStats
	Issues          []domain.ProviderSyncIssue
}

type SyncOrchestratorParams struct {
	SyncStateJournal   SyncStateJournal
	TargetWindowPolicy TargetWindowPolicy
	WindowChunkPolicy  WindowChunkPolicy
	WindowExecutor     WindowExecutor
}

type SyncOrchestratorOption func(*SyncOrchestrator)

type SyncOrchestrator struct {
	syncStateJournal   SyncStateJournal
	targetWindowPolicy TargetWindowPolicy
	windowChunkPolicy  WindowChunkPolicy
	windowExecutor     WindowExecutor
	logger             *slog.Logger
	now                func() time.Time
}

func WithLogger(logger *slog.Logger) SyncOrchestratorOption {
	return func(orchestrator *SyncOrchestrator) {
		orchestrator.logger = logger
	}
}

func WithNow(now func() time.Time) SyncOrchestratorOption {
	return func(orchestrator *SyncOrchestrator) {
		orchestrator.now = now
	}
}

func NewSyncOrchestrator(
	params SyncOrchestratorParams,
	opts ...SyncOrchestratorOption,
) (*SyncOrchestrator, error) {
	if params.SyncStateJournal == nil {
		return nil, ErrSyncStateJournalRequired
	}
	if params.TargetWindowPolicy == nil {
		return nil, ErrTargetWindowPolicyRequired
	}
	if params.WindowChunkPolicy == nil {
		return nil, ErrWindowChunkPolicyRequired
	}
	if params.WindowExecutor == nil {
		return nil, ErrWindowExecutorRequired
	}

	orchestrator := &SyncOrchestrator{
		syncStateJournal:   params.SyncStateJournal,
		targetWindowPolicy: params.TargetWindowPolicy,
		windowChunkPolicy:  params.WindowChunkPolicy,
		windowExecutor:     params.WindowExecutor,
		logger:             slog.New(slog.DiscardHandler),
		now:                time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(orchestrator)
		}
	}
	if orchestrator.logger == nil {
		orchestrator.logger = slog.New(slog.DiscardHandler)
	}
	return orchestrator, nil
}

// Orchestrate loads the latest sync state, plans chunk windows, and appends one state row per attempted chunk.
func (o *SyncOrchestrator) Orchestrate(
	ctx context.Context,
	request SyncOrchestrationRequest,
) (SyncOrchestrationResult, error) {
	o.logger.InfoContext(
		ctx,
		"start provider sync orchestration",
		slog.String("connection_id", request.Connection.ConnectionID),
		slog.String("provider_id", string(request.Connection.ProviderID)),
		slog.String("connector_id", string(request.Connection.ConnectorID)),
		slog.String("job_id", request.JobID),
		slog.String("reason", request.Reason),
	)

	lastState, err := o.syncStateJournal.LoadLastState(ctx, request.Connection)
	if err != nil {
		return SyncOrchestrationResult{}, fmt.Errorf("load sync state: %w", err)
	}

	targetWindow, err := o.targetWindowPolicy.Determine(o.now(), lastState)
	if err != nil {
		o.logger.ErrorContext(
			ctx,
			"provider sync target window planning failed",
			slog.String("connectionId", request.Connection.ConnectionID),
			slog.String("jobId", request.JobID),
			slog.String("error", err.Error()),
		)
		return SyncOrchestrationResult{}, fmt.Errorf("determine target window: %w", err)
	}

	requestedWindows, err := o.planRequestedWindows(
		ctx,
		request.Connection,
		request.JobID,
		targetWindow,
	)
	if err != nil {
		return SyncOrchestrationResult{}, err
	}

	result := SyncOrchestrationResult{
		TargetWindow: targetWindow,
	}
	for _, requestedWindow := range requestedWindows {
		windowResult, updatedStats, executeErr := o.executeRequestedWindow(
			ctx,
			request,
			requestedWindow,
			result.Stats,
		)
		if executeErr != nil {
			return SyncOrchestrationResult{}, executeErr
		}

		result.ExecutedWindows = append(result.ExecutedWindows, requestedWindow)
		result.WindowResults = append(result.WindowResults, windowResult)
		result.Stats = updatedStats
		result.Issues = append(result.Issues, windowResult.Issues...)
	}

	o.logger.InfoContext(
		ctx,
		"provider sync orchestration completed",
		slog.String("connection_id", request.Connection.ConnectionID),
		slog.Int("executed_windows", len(result.ExecutedWindows)),
		slog.Int("created_transactions", result.Stats.CreatedTransactions),
		slog.Int("updated_transactions", result.Stats.UpdatedTransactions),
	)

	return result, nil
}

func (o *SyncOrchestrator) planRequestedWindows(
	ctx context.Context,
	connection domain.ProviderConnectionRef,
	jobID string,
	targetWindow domain.ProviderSyncWindow,
) ([]domain.ProviderSyncWindow, error) {
	requestedWindows, err := o.windowChunkPolicy.Split(targetWindow)
	if err != nil {
		o.logger.ErrorContext(
			ctx,
			"provider sync window chunking failed",
			slog.String("connectionId", connection.ConnectionID),
			slog.String("jobId", jobID),
			slog.Time("targetStart", targetWindow.Start),
			slog.Time("targetEnd", targetWindow.End),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("split target window: %w", err)
	}

	o.logger.InfoContext(
		ctx,
		"planned provider sync orchestration windows",
		slog.String("connectionId", connection.ConnectionID),
		slog.Time("targetStart", targetWindow.Start),
		slog.Time("targetEnd", targetWindow.End),
		slog.Int("windowCount", len(requestedWindows)),
	)

	return requestedWindows, nil
}

func (o *SyncOrchestrator) executeRequestedWindow(
	ctx context.Context,
	request SyncOrchestrationRequest,
	requestedWindow domain.ProviderSyncWindow,
	aggregateStats domain.ProviderSyncStats,
) (WindowSyncResult, domain.ProviderSyncStats, error) {
	o.logger.InfoContext(
		ctx,
		"execute provider sync window",
		slog.String("connection_id", request.Connection.ConnectionID),
		slog.Time("requested_start", requestedWindow.Start),
		slog.Time("requested_end", requestedWindow.End),
	)

	attemptedAt := o.now()
	chunkState := domain.ProviderSyncState{
		Connection:     request.Connection,
		AttemptedAt:    &attemptedAt,
		Window:         requestedWindow,
		JobID:          request.JobID,
		AggregateStats: aggregateStats,
	}

	windowResult, executeErr := o.windowExecutor.Execute(ctx, WindowSyncRequest{
		Connection:      request.Connection,
		Secret:          request.Secret,
		RequestedWindow: requestedWindow,
		SyncState:       &chunkState,
		JobID:           request.JobID,
		Reason:          request.Reason,
	})
	if executeErr != nil {
		chunkState.ErrorSummary = executeErr.Error()
		if appendErr := o.syncStateJournal.AppendSyncState(ctx, chunkState); appendErr != nil {
			o.logger.ErrorContext(
				ctx,
				"append failed provider sync state failed",
				slog.String("connection_id", request.Connection.ConnectionID),
				slog.Time("requested_start", requestedWindow.Start),
				slog.Time("requested_end", requestedWindow.End),
				slog.String("error", appendErr.Error()),
			)

			return WindowSyncResult{}, domain.ProviderSyncStats{}, fmt.Errorf("append failed sync state: %w", appendErr)
		}
		o.logger.WarnContext(
			ctx,
			"provider sync window failed",
			slog.String("connection_id", request.Connection.ConnectionID),
			slog.Time("requested_start", requestedWindow.Start),
			slog.Time("requested_end", requestedWindow.End),
			slog.String("error", executeErr.Error()),
		)
		return WindowSyncResult{}, domain.ProviderSyncStats{}, fmt.Errorf("execute requested window: %w", executeErr)
	}

	completedAt := o.now()
	chunkState.SucceededAt = &completedAt
	chunkState.RunID = windowResult.RunID
	updatedStats := mergeProviderSyncStats(chunkState.AggregateStats, windowResult.Stats)
	chunkState.AggregateStats = updatedStats
	if appendErr := o.syncStateJournal.AppendSyncState(ctx, chunkState); appendErr != nil {
		o.logger.ErrorContext(
			ctx,
			"append provider sync state failed",
			slog.String("connection_id", request.Connection.ConnectionID),
			slog.Time("requested_start", requestedWindow.Start),
			slog.Time("requested_end", requestedWindow.End),
			slog.String("run_id", windowResult.RunID),
			slog.String("error", appendErr.Error()),
		)

		return WindowSyncResult{}, domain.ProviderSyncStats{}, fmt.Errorf("append sync state: %w", appendErr)
	}

	o.logger.InfoContext(
		ctx,
		"provider sync window succeeded",
		slog.String("connection_id", request.Connection.ConnectionID),
		slog.Time("requested_start", requestedWindow.Start),
		slog.Time("requested_end", requestedWindow.End),
		slog.String("run_id", windowResult.RunID),
	)

	return windowResult, updatedStats, nil
}

func mergeProviderSyncStats(left domain.ProviderSyncStats, right domain.ProviderSyncStats) domain.ProviderSyncStats {
	return domain.ProviderSyncStats{
		ObservedAccounts:             left.ObservedAccounts + right.ObservedAccounts,
		ObservedTransactions:         left.ObservedTransactions + right.ObservedTransactions,
		CreatedTransactions:          left.CreatedTransactions + right.CreatedTransactions,
		UpdatedTransactions:          left.UpdatedTransactions + right.UpdatedTransactions,
		AmbiguousCreatedTransactions: left.AmbiguousCreatedTransactions + right.AmbiguousCreatedTransactions,
	}
}
