package providers

import (
	"errors"
	"fmt"

	"github.com/gemyago/signal-foundry/finance/domain"
)

var (
	ErrConnectorIDRequired    = errors.New("connector id is required")
	ErrConnectorNotConfigured = errors.New("connector not configured")
	ErrProviderIDRequired     = errors.New("provider id is required")
	ErrProviderNotConfigured  = errors.New("provider not configured")
)

type StaticProviderProfileRegistry struct {
	profiles map[domain.ProviderID]ProviderProfile
}

func NewStaticProviderProfileRegistry(profiles ...ProviderProfile) *StaticProviderProfileRegistry {
	registry := &StaticProviderProfileRegistry{
		profiles: map[domain.ProviderID]ProviderProfile{},
	}
	for _, profile := range profiles {
		providerID := profile.ProviderID
		if providerID == "" {
			continue
		}
		if _, exists := registry.profiles[providerID]; exists {
			continue
		}
		profile.ProviderID = providerID
		registry.profiles[providerID] = profile
	}
	return registry
}

func (r *StaticProviderProfileRegistry) Resolve(providerID domain.ProviderID) (ProviderProfile, error) {
	if providerID == "" {
		return ProviderProfile{}, ErrProviderIDRequired
	}
	profile, ok := r.profiles[providerID]
	if !ok {
		return ProviderProfile{}, fmt.Errorf("%w: %s", ErrProviderNotConfigured, providerID)
	}
	return profile, nil
}

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
		connectorID := connector.ConnectorID()
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
	if connectorID == "" {
		return nil, ErrConnectorIDRequired
	}
	connector, ok := r.connectors[connectorID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrConnectorNotConfigured, connectorID)
	}
	return connector, nil
}
