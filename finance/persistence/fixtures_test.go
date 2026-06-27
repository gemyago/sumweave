package persistence

import (
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
)

type syntheticProviderStateFixture struct {
	connectionID     string
	accounts         []domain.SyntheticConfiguredAccount
	windowHistory    []domain.SyntheticWindowHistoryEntry
	sequenceCounters []domain.SyntheticAccountDaySequenceCounter
	createdAt        time.Time
	updatedAt        time.Time
}

type syntheticProviderStateOption func(*syntheticProviderStateFixture)

func defaultSyntheticProviderStateFixture(fake faker.Faker) syntheticProviderStateFixture {
	baseDay := time.Date(2026, time.June, 24, 0, 0, 0, 0, time.FixedZone("CET", 2*60*60))
	createdAt := time.Date(2026, time.June, 24, 12, 30, 0, 0, time.FixedZone("EEST", 3*60*60))

	return syntheticProviderStateFixture{
		connectionID: "connection-" + fake.UUID().V4(),
		accounts: []domain.SyntheticConfiguredAccount{
			makeRandomSyntheticConfiguredAccount(fake, "checking", "USD"),
			makeRandomSyntheticConfiguredAccount(fake, "checking", "USD"),
		},
		windowHistory: []domain.SyntheticWindowHistoryEntry{{
			Window: domain.SyntheticWindowKey{
				NormalizedStartUTC:        baseDay,
				NormalizedEndExclusiveUTC: baseDay.Add(48 * time.Hour),
			},
			RepeatCount: 2,
		}},
		sequenceCounters: []domain.SyntheticAccountDaySequenceCounter{{
			AccountKey:   "account-a-" + fake.UUID().V4(),
			DayUTC:       baseDay.Add(24 * time.Hour),
			NextSequence: 4,
		}},
		createdAt: createdAt,
		updatedAt: createdAt.Add(90 * time.Minute),
	}
}

func makeRandomSyntheticProviderState(
	fake faker.Faker,
	opts ...syntheticProviderStateOption,
) domain.SyntheticProviderState {
	fixture := defaultSyntheticProviderStateFixture(fake)
	for _, opt := range opts {
		opt(&fixture)
	}

	return domain.SyntheticProviderState{
		ConnectionID: fixture.connectionID,
		Envelope: domain.SyntheticProviderStateEnvelope{
			Version:            domain.SyntheticProviderStateVersion1,
			ConfiguredAccounts: fixture.accounts,
			WindowHistory:      fixture.windowHistory,
			SequenceCounters:   fixture.sequenceCounters,
		},
		CreatedAt: fixture.createdAt,
		UpdatedAt: fixture.updatedAt,
	}
}

func makeRandomSyntheticConnectionID(fake faker.Faker) string {
	return "connection-" + fake.UUID().V4()
}

func withSyntheticProviderConnectionID(connectionID string) syntheticProviderStateOption {
	return func(fixture *syntheticProviderStateFixture) {
		fixture.connectionID = connectionID
	}
}

func withSyntheticProviderCreatedAt(createdAt time.Time) syntheticProviderStateOption {
	return func(fixture *syntheticProviderStateFixture) {
		fixture.createdAt = createdAt
	}
}

func withSyntheticProviderUpdatedAt(updatedAt time.Time) syntheticProviderStateOption {
	return func(fixture *syntheticProviderStateFixture) {
		fixture.updatedAt = updatedAt
	}
}

func withSyntheticProviderSingleAccount(
	fake faker.Faker,
	namePrefix string,
	currency string,
) syntheticProviderStateOption {
	return func(fixture *syntheticProviderStateFixture) {
		fixture.accounts = []domain.SyntheticConfiguredAccount{
			makeRandomSyntheticConfiguredAccount(fake, namePrefix, currency),
		}
		fixture.windowHistory = nil
		fixture.sequenceCounters = nil
	}
}

func withSyntheticProviderWindowHistoryFrom(start time.Time) syntheticProviderStateOption {
	return func(fixture *syntheticProviderStateFixture) {
		fixture.windowHistory = []domain.SyntheticWindowHistoryEntry{{
			Window: domain.SyntheticWindowKey{
				NormalizedStartUTC:        start,
				NormalizedEndExclusiveUTC: start.Add(24 * time.Hour),
			},
			RepeatCount: 1,
		}}
	}
}

func withSyntheticProviderEmptyEnvelope() syntheticProviderStateOption {
	return func(fixture *syntheticProviderStateFixture) {
		fixture.accounts = nil
		fixture.windowHistory = nil
		fixture.sequenceCounters = nil
		fixture.createdAt = time.Time{}
		fixture.updatedAt = time.Time{}
	}
}

func makeRandomSyntheticConfiguredAccount(
	fake faker.Faker,
	namePrefix string,
	currency string,
) domain.SyntheticConfiguredAccount {
	return domain.SyntheticConfiguredAccount{
		Key:      "account-" + fake.UUID().V4(),
		Name:     namePrefix + "-" + fake.Lorem().Word(),
		Currency: currency,
	}
}
