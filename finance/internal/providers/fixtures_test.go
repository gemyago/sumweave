package providers

import (
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
)

func makeRandomProviderConnectionRef(
	fake faker.Faker,
	providerID domain.ProviderID,
	connectorID domain.ProviderConnectorID,
) domain.ProviderConnectionRef {
	return domain.ProviderConnectionRef{
		ConnectionID:      "connection-" + fake.UUID().V4(),
		ProviderID:        providerID,
		ConnectorID:       connectorID,
		ProviderReference: "provider-ref-" + fake.UUID().V4(),
		ExternalID:        "external-" + fake.UUID().V4(),
	}
}

func makeRandomProviderSyncWindow(fake faker.Faker) domain.ProviderSyncWindow {
	start := fake.Time().Past().UTC().Truncate(time.Second)
	end := fake.Time().Recent().UTC().Truncate(time.Second)
	if !end.After(start) {
		start, end = end, start
	}
	return domain.ProviderSyncWindow{
		Start: start,
		End:   end,
	}
}

func makeRandomProviderSyncStats(fake faker.Faker) domain.ProviderSyncStats {
	observedTransactions := fake.IntBetween(1, 8)

	return domain.ProviderSyncStats{
		ObservedAccounts:             fake.IntBetween(1, 4),
		ObservedTransactions:         observedTransactions,
		CreatedTransactions:          fake.IntBetween(0, observedTransactions),
		UpdatedTransactions:          fake.IntBetween(0, observedTransactions),
		AmbiguousCreatedTransactions: fake.IntBetween(0, observedTransactions),
	}
}

func makeRandomProviderSyncIssue(fake faker.Faker) domain.ProviderSyncIssue {
	return domain.ProviderSyncIssue{
		Code:                  "issue-" + fake.UUID().V4(),
		Severity:              domain.ProviderSyncIssueSeverityWarning,
		Summary:               "summary-" + fake.Lorem().Sentence(3),
		ProviderAccountID:     "account-" + fake.UUID().V4(),
		ProviderTransactionID: "txn-" + fake.UUID().V4(),
	}
}

func makeRandomProviderSyncState(
	fake faker.Faker,
	connection domain.ProviderConnectionRef,
) domain.ProviderSyncState {
	lastWindow := makeRandomProviderSyncWindow(fake)
	lastSuccessAt := lastWindow.End.Add(time.Hour)
	lastAttemptAt := lastSuccessAt.Add(time.Hour)

	return domain.ProviderSyncState{
		Connection:     connection,
		AttemptedAt:    &lastAttemptAt,
		SucceededAt:    &lastSuccessAt,
		Window:         lastWindow,
		RunID:          "run-" + fake.UUID().V4(),
		JobID:          "job-" + fake.UUID().V4(),
		ErrorSummary:   "error-" + fake.Lorem().Sentence(3),
		AggregateStats: makeRandomProviderSyncStats(fake),
	}
}

func makeRandomWindowSyncResult(fake faker.Faker) WindowSyncResult {
	return WindowSyncResult{
		RunID:  "run-" + fake.UUID().V4(),
		Stats:  makeRandomProviderSyncStats(fake),
		Issues: []domain.ProviderSyncIssue{makeRandomProviderSyncIssue(fake)},
	}
}

func makeRandomConnectionSecret(fake faker.Faker, providerID domain.ProviderID) domain.ConnectionSecret {
	return domain.ConnectionSecret{
		ID:        "secret-" + fake.UUID().V4(),
		Provider:  string(providerID),
		Reference: "reference-" + fake.UUID().V4(),
		Envelope: credentials.Envelope{
			KeyVersion: "v1",
			Algorithm:  credentials.AlgorithmAESGCM,
			Nonce:      "nonce-" + fake.UUID().V4(),
			Ciphertext: "ciphertext-" + fake.UUID().V4(),
		},
	}
}
