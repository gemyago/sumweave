package persistence

import (
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
)

type syntheticProviderStateFixture struct {
	providerReference string
	accounts          []domain.SyntheticConfiguredAccount
	windowHistory     []domain.SyntheticWindowHistoryEntry
	sequenceCounters  []domain.SyntheticAccountInstantSequenceCounter
	createdAt         time.Time
	updatedAt         time.Time
}

type syntheticProviderStateOption func(*syntheticProviderStateFixture)

func defaultSyntheticProviderStateFixture(fake faker.Faker) syntheticProviderStateFixture {
	baseInstant := time.Date(2026, time.June, 24, 8, 15, 0, 0, time.FixedZone("CET", 2*60*60))
	createdAt := time.Date(2026, time.June, 24, 12, 30, 0, 0, time.FixedZone("EEST", 3*60*60))

	return syntheticProviderStateFixture{
		providerReference: "provider-ref-" + fake.UUID().V4(),
		accounts: []domain.SyntheticConfiguredAccount{
			makeRandomSyntheticConfiguredAccount(fake, "checking", "USD"),
			makeRandomSyntheticConfiguredAccount(fake, "checking", "USD"),
		},
		windowHistory: []domain.SyntheticWindowHistoryEntry{{
			Window: domain.SyntheticWindowKey{
				Start: baseInstant,
				End:   baseInstant.Add(48 * time.Hour),
			},
			RepeatCount: 2,
		}},
		sequenceCounters: []domain.SyntheticAccountInstantSequenceCounter{{
			AccountKey:   "account-a-" + fake.UUID().V4(),
			Instant:      baseInstant.Add(24 * time.Hour),
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
		ProviderReference: fixture.providerReference,
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

func makeRandomSyntheticProviderReference(fake faker.Faker) string {
	return "provider-ref-" + fake.UUID().V4()
}

func withSyntheticProviderReference(providerReference string) syntheticProviderStateOption {
	return func(fixture *syntheticProviderStateFixture) {
		fixture.providerReference = providerReference
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

func withSyntheticProviderWindowHistoryFrom(start time.Time) syntheticProviderStateOption {
	return func(fixture *syntheticProviderStateFixture) {
		fixture.windowHistory = []domain.SyntheticWindowHistoryEntry{{
			Window: domain.SyntheticWindowKey{
				Start: start,
				End:   start.Add(24 * time.Hour),
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
