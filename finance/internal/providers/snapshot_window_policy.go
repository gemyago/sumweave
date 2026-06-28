package providers

import "github.com/gemyago/signal-foundry/finance/domain"

type SnapshotWindowPolicy interface {
	Determine(requestedWindow domain.ProviderSyncWindow) (domain.ProviderSyncWindow, error)
}

type RequestedWindowSnapshotPolicy struct{}

func NewRequestedWindowSnapshotPolicy() *RequestedWindowSnapshotPolicy {
	return &RequestedWindowSnapshotPolicy{}
}

func (p *RequestedWindowSnapshotPolicy) Determine(
	requestedWindow domain.ProviderSyncWindow,
) (domain.ProviderSyncWindow, error) {
	return requestedWindow, nil
}
