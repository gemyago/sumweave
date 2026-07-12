package domain

import "time"

const SyntheticProviderStateVersion1 = 1

type SyntheticProviderState struct {
	ProviderReference string
	Envelope          SyntheticProviderStateEnvelope
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type SyntheticProviderStateEnvelope struct {
	Version            int
	ConfiguredAccounts []SyntheticConfiguredAccount
	WindowHistory      []SyntheticWindowHistoryEntry
	SequenceCounters   []SyntheticAccountInstantSequenceCounter
}

type SyntheticConfiguredAccount struct {
	Key      string
	Name     string
	Currency string
}

type SyntheticWindowKey struct {
	Start time.Time
	End   time.Time
}

type SyntheticWindowHistoryEntry struct {
	Window      SyntheticWindowKey
	RepeatCount int
}

type SyntheticAccountInstantSequenceCounter struct {
	AccountKey   string
	Instant      time.Time
	NextSequence int
}
