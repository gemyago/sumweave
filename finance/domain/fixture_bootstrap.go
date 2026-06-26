package domain

import "time"

type FixtureBootstrapRun struct {
	ID        string
	Seed      int64
	Scenario  string
	StartedAt time.Time
}

type FixtureScenarioRecord struct {
	Name       string
	StableID   string
	OccurredAt time.Time
}
