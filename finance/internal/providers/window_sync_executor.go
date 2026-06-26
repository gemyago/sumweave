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

// Execute is a placeholder seam for later fetch, diff, and apply orchestration.
func (c *WindowSyncExecutor) Execute(
	_ context.Context,
	request WindowSyncRequest,
) (WindowSyncResult, error) {
	if err := c.resolveConnector(request.Connection.ConnectorID); err != nil {
		return WindowSyncResult{}, err
	}

	runID := request.JobID
	if c.runIDGenerator != nil {
		generatedRunID := c.runIDGenerator()
		if generatedRunID != "" {
			runID = generatedRunID
		}
	}

	return WindowSyncResult{
		RunID: runID,
		Stats: domain.ProviderSyncStats{},
	}, nil
}

func (c *WindowSyncExecutor) resolveConnector(connectorID domain.ProviderConnectorID) error {
	if normalizeConnectorID(connectorID) == "" {
		return ErrConnectorIDRequired
	}
	if c.connectorRegistry == nil {
		return fmt.Errorf("%w: %s", ErrConnectorNotConfigured, connectorID)
	}
	if _, err := c.connectorRegistry.Resolve(connectorID); err != nil {
		return fmt.Errorf("resolve sync connector: %w", err)
	}
	return nil
}
