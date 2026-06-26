package providers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/finance/domain"
)

var (
	ErrConnectorIDRequired    = errors.New("connector id is required")
	ErrConnectorNotConfigured = errors.New("connector not configured")
)

type StaticConnectorRegistry struct {
	connectors map[domain.ProviderConnectorID]Connector
}

func NewStaticConnectorRegistry(connectors ...Connector) *StaticConnectorRegistry {
	registry := &StaticConnectorRegistry{
		connectors: map[domain.ProviderConnectorID]Connector{},
	}
	for _, connector := range connectors {
		if connector == nil {
			continue
		}
		connectorID := normalizeConnectorID(connector.ConnectorID())
		if connectorID == "" {
			continue
		}
		if _, exists := registry.connectors[connectorID]; exists {
			continue
		}
		registry.connectors[connectorID] = connector
	}
	return registry
}

//nolint:ireturn // The registry contract resolves connectors behind the shared interface seam.
func (r *StaticConnectorRegistry) Resolve(connectorID domain.ProviderConnectorID) (Connector, error) {
	resolvedID := normalizeConnectorID(connectorID)
	if resolvedID == "" {
		return nil, ErrConnectorIDRequired
	}
	connector, ok := r.connectors[resolvedID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrConnectorNotConfigured, resolvedID)
	}
	return connector, nil
}

func normalizeConnectorID(connectorID domain.ProviderConnectorID) domain.ProviderConnectorID {
	return domain.ProviderConnectorID(strings.TrimSpace(string(connectorID)))
}
