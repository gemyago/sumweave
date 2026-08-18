package providers

import "github.com/gemyago/sumweave/finance/domain"

const providerSyncWindowChunkDays = 30

type OldestFirstWindowChunkPolicy struct{}

func NewOldestFirstWindowChunkPolicy() *OldestFirstWindowChunkPolicy {
	return &OldestFirstWindowChunkPolicy{}
}

func (p *OldestFirstWindowChunkPolicy) Split(
	target domain.ProviderSyncWindow,
) ([]domain.ProviderSyncWindow, error) {
	if err := validateSyncWindow(target, ErrInvalidProviderSyncTargetWindow); err != nil {
		return nil, err
	}

	var windows []domain.ProviderSyncWindow
	for start := target.Start; start.Before(target.End); {
		end := start.AddDate(0, 0, providerSyncWindowChunkDays)
		if end.After(target.End) {
			end = target.End
		}
		windows = append(windows, domain.ProviderSyncWindow{Start: start, End: end})
		start = end
	}

	return windows, nil
}
