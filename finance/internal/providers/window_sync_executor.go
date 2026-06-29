package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/google/uuid"
)

var (
	ErrInvalidRequestedWindow    = errors.New("invalid requested window")
	ErrConnectorRegistryRequired = errors.New("connector registry is required")
	ErrWindowSyncStoreRequired   = errors.New("window sync store is required")
)

type ConnectorRegistry interface {
	Resolve(connectorID domain.ProviderConnectorID) (Connector, error)
}

type WindowSyncRequest struct {
	Connection      domain.ProviderConnectionRef
	Secret          domain.ConnectionSecret
	RequestedWindow domain.ProviderSyncWindow
	SyncState       *domain.ProviderSyncState
	JobID           string
	Reason          string
}

type WindowSyncResult struct {
	RunID  string
	Batch  domain.ProviderSyncBatch
	Stats  domain.ProviderSyncStats
	Issues []domain.ProviderSyncIssue
}

type WindowSyncStore interface {
	LoadExistingWindow(
		ctx context.Context,
		connection domain.ProviderConnectionRef,
		window domain.ProviderSyncWindow,
	) (ExistingWindowSnapshot, error)
	ApplySync(ctx context.Context, diffPlan ProviderDiffPlan, applyPlan ApplyPlan) error
}

type WindowSyncExecutorOption func(*WindowSyncExecutor)

type WindowSyncExecutor struct {
	connectorRegistry    ConnectorRegistry
	snapshotWindowPolicy SnapshotWindowPolicy
	windowSyncStore      WindowSyncStore
	diffPlanner          *DiffPlanner
	applyPlanner         *ApplyPlanner
	runIDGenerator       func() string
}

func WithConnectorRegistry(connectorRegistry ConnectorRegistry) WindowSyncExecutorOption {
	return func(executor *WindowSyncExecutor) {
		executor.connectorRegistry = connectorRegistry
	}
}

func WithConnectors(connectors ...Connector) WindowSyncExecutorOption {
	return func(executor *WindowSyncExecutor) {
		executor.connectorRegistry = NewStaticConnectorRegistry(connectors...)
	}
}

func WithRunIDGenerator(runIDGenerator func() string) WindowSyncExecutorOption {
	return func(executor *WindowSyncExecutor) {
		executor.runIDGenerator = runIDGenerator
	}
}

func WithSnapshotWindowPolicy(snapshotWindowPolicy SnapshotWindowPolicy) WindowSyncExecutorOption {
	return func(executor *WindowSyncExecutor) {
		executor.snapshotWindowPolicy = snapshotWindowPolicy
	}
}

func WithWindowSyncStore(windowSyncStore WindowSyncStore) WindowSyncExecutorOption {
	return func(executor *WindowSyncExecutor) {
		executor.windowSyncStore = windowSyncStore
	}
}

func NewWindowSyncExecutor(opts ...WindowSyncExecutorOption) (*WindowSyncExecutor, error) {
	executor := &WindowSyncExecutor{
		connectorRegistry:    NewStaticConnectorRegistry(),
		snapshotWindowPolicy: NewRequestedWindowSnapshotPolicy(),
		diffPlanner:          NewDiffPlanner(),
		applyPlanner:         NewApplyPlanner(nil),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(executor)
		}
	}
	if executor.runIDGenerator == nil {
		executor.runIDGenerator = uuid.NewString
	}
	if executor.connectorRegistry == nil {
		return nil, ErrConnectorRegistryRequired
	}
	if executor.windowSyncStore == nil {
		return nil, ErrWindowSyncStoreRequired
	}
	return executor, nil
}

func (c *WindowSyncExecutor) Execute(
	ctx context.Context,
	request WindowSyncRequest,
) (WindowSyncResult, error) {
	if err := validateSyncWindow(request.RequestedWindow, ErrInvalidRequestedWindow); err != nil {
		return WindowSyncResult{}, err
	}
	connector, err := c.connectorRegistry.Resolve(request.Connection.ConnectorID)
	if err != nil {
		return WindowSyncResult{}, fmt.Errorf("resolve sync connector: %w", err)
	}

	batch, err := connector.Fetch(ctx, FetchRequest{
		Connection:      request.Connection,
		Secret:          request.Secret,
		RequestedWindow: request.RequestedWindow,
		SyncState:       request.SyncState,
	})
	if err != nil {
		return WindowSyncResult{}, fmt.Errorf("fetch sync batch: %w", err)
	}
	batch.Connection = request.Connection
	batch.RequestedWindow = request.RequestedWindow

	snapshotWindow, err := c.snapshotWindowPolicy.Determine(request.RequestedWindow)
	if err != nil {
		return WindowSyncResult{}, fmt.Errorf("determine snapshot window: %w", err)
	}
	snapshot, err := c.windowSyncStore.LoadExistingWindow(ctx, request.Connection, snapshotWindow)
	if err != nil {
		return WindowSyncResult{}, fmt.Errorf("load existing snapshot: %w", err)
	}

	diffPlan := c.diffPlanner.Plan(batch, snapshot)
	applyPlan := c.applyPlanner.Plan(diffPlan)
	applyErr := c.windowSyncStore.ApplySync(ctx, diffPlan, applyPlan)
	if applyErr != nil {
		return WindowSyncResult{}, fmt.Errorf("apply sync: %w", applyErr)
	}

	return WindowSyncResult{
		RunID: c.runIDGenerator(),
		Batch: batch,
		Stats: applyPlan.Stats,
		Issues: append(
			[]domain.ProviderSyncIssue(nil),
			applyPlan.Issues...,
		),
	}, nil
}
