package domain

import "time"

// FXRefreshSchedule is the finance-owned due state for periodic FX refreshes.
type FXRefreshSchedule struct {
	ScheduleID      string
	Provider        string
	Interval        time.Duration
	NextRunAt       *time.Time
	LastScheduledAt *time.Time
	LastJobID       string
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
