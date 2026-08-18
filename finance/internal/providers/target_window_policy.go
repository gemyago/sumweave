package providers

import (
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
)

const (
	initialBackfillYears = 3
	recentRefreshDays    = 30
)

var (
	ErrInvalidProviderSyncStateWindow  = errors.New("invalid provider sync state window")
	ErrInvalidProviderSyncTargetWindow = errors.New("invalid provider sync target window")
)

type TargetWindowRequest struct {
	Now         time.Time
	State       *domain.ProviderSyncState
	WindowStart *time.Time
	WindowEnd   *time.Time
}

type CheckpointTargetWindowPolicy struct{}

func NewCheckpointTargetWindowPolicy() *CheckpointTargetWindowPolicy {
	return &CheckpointTargetWindowPolicy{}
}

func (p *CheckpointTargetWindowPolicy) Determine(
	request TargetWindowRequest,
) (domain.ProviderSyncWindow, error) {
	targetEnd := request.Now
	if request.WindowEnd != nil {
		targetEnd = *request.WindowEnd
	}

	var targetStart time.Time
	if request.WindowStart != nil {
		targetStart = *request.WindowStart
	} else {
		var err error
		targetStart, err = p.determineStart(targetEnd, request.State)
		if err != nil {
			return domain.ProviderSyncWindow{}, err
		}
	}

	window := domain.ProviderSyncWindow{Start: targetStart, End: targetEnd}
	if err := validateSyncWindow(window, ErrInvalidProviderSyncTargetWindow); err != nil {
		return domain.ProviderSyncWindow{}, err
	}

	return window, nil
}

func (p *CheckpointTargetWindowPolicy) determineStart(
	targetEnd time.Time,
	state *domain.ProviderSyncState,
) (time.Time, error) {
	if state == nil {
		return targetEnd.AddDate(-initialBackfillYears, 0, 0), nil
	}

	if err := validateProviderSyncWindow(state.Window); err != nil {
		return time.Time{}, err
	}

	recentStart := targetEnd.AddDate(0, 0, -recentRefreshDays)
	checkpoint := state.Window.Start
	if state.SucceededAt != nil {
		checkpoint = state.Window.End
	}
	if checkpoint.Before(recentStart) {
		return checkpoint, nil
	}

	return recentStart, nil
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
