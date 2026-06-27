package providers

import (
	"context"
	"fmt"

	"github.com/gemyago/signal-foundry/finance/domain"
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

type WindowSyncExecutorOption func(*WindowSyncExecutor)

type WindowSyncExecutor struct {
	connectorRegistry ConnectorRegistry
	runIDGenerator    func() string
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

func NewWindowSyncExecutor(opts ...WindowSyncExecutorOption) *WindowSyncExecutor {
	executor := &WindowSyncExecutor{
		connectorRegistry: NewStaticConnectorRegistry(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(executor)
		}
	}
	return executor
}

func (c *WindowSyncExecutor) Execute(
	ctx context.Context,
	request WindowSyncRequest,
) (WindowSyncResult, error) {
	connector, err := c.resolveConnector(request.Connection.ConnectorID)
	if err != nil {
		return WindowSyncResult{}, err
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

	runID := request.JobID
	if c.runIDGenerator != nil {
		generatedRunID := c.runIDGenerator()
		if generatedRunID != "" {
			runID = generatedRunID
		}
	}

	return WindowSyncResult{
		RunID: runID,
		Batch: batch,
		Stats: domain.ProviderSyncStats{
			ObservedAccounts:     len(batch.Accounts),
			ObservedTransactions: len(batch.Transactions),
		},
	}, nil
}

//nolint:ireturn // Internal executor flow resolves connectors behind the shared connector seam.
func (c *WindowSyncExecutor) resolveConnector(connectorID domain.ProviderConnectorID) (Connector, error) {
	if normalizeConnectorID(connectorID) == "" {
		return nil, ErrConnectorIDRequired
	}
	if c.connectorRegistry == nil {
		return nil, fmt.Errorf("%w: %s", ErrConnectorNotConfigured, connectorID)
	}
	connector, err := c.connectorRegistry.Resolve(connectorID)
	if err != nil {
		return nil, fmt.Errorf("resolve sync connector: %w", err)
	}
	return connector, nil
}
