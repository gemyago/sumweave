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
	SequenceCounters   []SyntheticAccountDaySequenceCounter
}

type SyntheticConfiguredAccount struct {
	Key      string
	Name     string
	Currency string
}

type SyntheticWindowKey struct {
	NormalizedStartUTC        time.Time
	NormalizedEndExclusiveUTC time.Time
}

type SyntheticWindowHistoryEntry struct {
	Window      SyntheticWindowKey
	RepeatCount int
}

type SyntheticAccountDaySequenceCounter struct {
	AccountKey   string
	DayUTC       time.Time
	NextSequence int
}
