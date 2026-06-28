package providers

import (
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

const (
	initialBackfillYears = 3
	recentRefreshDays    = 30
)

var ErrInvalidProviderSyncStateWindow = errors.New("invalid provider sync state window")

type CheckpointTargetWindowPolicy struct{}

func NewCheckpointTargetWindowPolicy() *CheckpointTargetWindowPolicy {
	return &CheckpointTargetWindowPolicy{}
}

func (p *CheckpointTargetWindowPolicy) Determine(
	now time.Time,
	state *domain.ProviderSyncState,
) (domain.ProviderSyncWindow, error) {
	targetEnd := now
	if state == nil {
		return domain.ProviderSyncWindow{
			Start: targetEnd.AddDate(-initialBackfillYears, 0, 0),
			End:   targetEnd,
		}, nil
	}

	if err := validateProviderSyncWindow(state.Window); err != nil {
		return domain.ProviderSyncWindow{}, err
	}

	recentStart := targetEnd.AddDate(0, 0, -recentRefreshDays)
	checkpoint := state.Window.Start
	if state.SucceededAt != nil {
		checkpoint = state.Window.End
	}
	if checkpoint.Before(recentStart) {
		return domain.ProviderSyncWindow{
			Start: checkpoint,
			End:   targetEnd,
		}, nil
	}

	return domain.ProviderSyncWindow{
		Start: recentStart,
		End:   targetEnd,
	}, nil
}

func validateProviderSyncWindow(window domain.ProviderSyncWindow) error {
	return validateSyncWindow(window, ErrInvalidProviderSyncStateWindow)
}

func validateSyncWindow(window domain.ProviderSyncWindow, invalidErr error) error {
	if window.Start.IsZero() {
		return fmt.Errorf("%w: start is zero", invalidErr)
	}
	if window.End.IsZero() {
		return fmt.Errorf("%w: end is zero", invalidErr)
	}
	if !window.End.After(window.Start) {
		return fmt.Errorf("%w: end must be after start", invalidErr)
	}

	return nil
}
