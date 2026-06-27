package providers

import (
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConnectorRegistry struct {
	connector    Connector
	err          error
	resolveCalls []domain.ProviderConnectorID
}

func (r *stubConnectorRegistry) Resolve(connectorID domain.ProviderConnectorID) (Connector, error) {
	r.resolveCalls = append(r.resolveCalls, connectorID)
	if r.err != nil {
		return nil, r.err
	}
	return r.connector, nil
}

func TestWindowSyncExecutor(t *testing.T) {
	makeRequest := func(
		fake faker.Faker,
		providerID domain.ProviderID,
		connectorID domain.ProviderConnectorID,
	) WindowSyncRequest {
		return WindowSyncRequest{
			Connection:      makeRandomProviderConnectionRef(fake, providerID, connectorID),
			Secret:          makeRandomConnectionSecret(fake, providerID),
			RequestedWindow: makeRandomProviderSyncWindow(fake),
			JobID:           "job-" + fake.UUID().V4(),
			Reason:          "manual",
		}
	}

	t.Run("coordinate", func(t *testing.T) {
		t.Run("resolves monobank connections by technical connector id", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				connector: &stubConnector{connectorID: domain.ProviderConnectorIDMonobank},
			}
			request := makeRequest(fake, domain.ProviderIDMonobank, domain.ProviderConnectorIDMonobank)

			executor := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			)

			result, err := executor.Execute(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(t, []domain.ProviderConnectorID{domain.ProviderConnectorIDMonobank}, registry.resolveCalls)
			assert.NotEmpty(t, result.RunID)
			assert.Equal(t, request.Connection, result.Batch.Connection)
			assert.Equal(t, request.RequestedWindow, result.Batch.RequestedWindow)
			assert.Equal(t, domain.ProviderSyncStats{}, result.Stats)
		})

		t.Run("resolves pko connections through enable banking connector id", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				connector: &stubConnector{connectorID: domain.ProviderConnectorIDEnableBanking},
			}
			request := makeRequest(fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking)

			executor := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			)

			_, err := executor.Execute(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(
				t,
				[]domain.ProviderConnectorID{domain.ProviderConnectorIDEnableBanking},
				registry.resolveCalls,
			)
		})

		t.Run("fails early for empty connector ids without calling registry", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				connector: &stubConnector{connectorID: domain.ProviderConnectorIDMonobank},
			}
			request := makeRequest(fake, domain.ProviderIDMonobank, "")

			executor := NewWindowSyncExecutor(WithConnectorRegistry(registry))

			_, err := executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, ErrConnectorIDRequired)
			assert.Empty(t, registry.resolveCalls)
			assert.NotContains(t, err.Error(), request.Secret.Reference)
			assert.NotContains(t, err.Error(), request.Secret.Envelope.Ciphertext)
		})

		t.Run("fails early for unconfigured connector ids before fetch", func(t *testing.T) {
			fake := faker.New()
			unknownConnectorID := domain.ProviderConnectorID("unknown-" + fake.UUID().V4())
			connector := &stubConnector{connectorID: domain.ProviderConnectorIDMonobank}
			registry := &stubConnectorRegistry{
				connector: connector,
				err:       fmt.Errorf("%w: %s", ErrConnectorNotConfigured, unknownConnectorID),
			}
			request := makeRequest(fake, domain.ProviderIDPKO, unknownConnectorID)

			executor := NewWindowSyncExecutor(WithConnectorRegistry(registry))

			_, err := executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, ErrConnectorNotConfigured)
			assert.Equal(t, []domain.ProviderConnectorID{unknownConnectorID}, registry.resolveCalls)
			assert.Zero(t, connector.fetchCalls)
			assert.NotContains(t, err.Error(), request.Secret.Reference)
			assert.NotContains(t, err.Error(), request.Secret.Envelope.Ciphertext)
		})

		t.Run("invokes connector fetch and returns batch-derived stats", func(t *testing.T) {
			fake := faker.New()
			request := makeRequest(fake, domain.ProviderIDSynthetic, domain.ProviderConnectorIDSynthetic)
			syncState := makeRandomProviderSyncState(fake, request.Connection)
			request.SyncState = &syncState
			capturedAt := request.RequestedWindow.End.Add(-time.Hour)
			effectiveAt := request.RequestedWindow.Start.Add(2 * time.Hour)
			providerOriginalEffectiveAt := effectiveAt.Add(-time.Hour)
			account := domain.ProviderAccountObservation{
				Connection:        request.Connection,
				ProviderAccountID: "provider-account-" + fake.UUID().V4(),
				Name:              "account-" + fake.Lorem().Word(),
				Currency:          "USD",
			}
			balance := domain.ProviderBalanceObservation{
				Connection:          request.Connection,
				ProviderAccountID:   account.ProviderAccountID,
				Currency:            account.Currency,
				CurrentBalanceMinor: 12_345,
				CapturedAt:          capturedAt,
			}
			transaction := domain.ProviderTransactionObservation{
				Connection:            request.Connection,
				ProviderAccountID:     account.ProviderAccountID,
				ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
				Status:                domain.TransactionStatusBooked,
				AmountMinor:           -456,
				Currency:              account.Currency,
				Description:           "transaction-" + fake.Lorem().Word(),
				EffectiveAt:           effectiveAt,
				Fingerprint:           "fingerprint-" + fake.UUID().V4(),
				ProviderOriginal: &domain.ProviderTransactionOriginal{
					AmountMinor: -456,
					Currency:    account.Currency,
					Description: "original-" + fake.Lorem().Word(),
					EffectiveAt: &providerOriginalEffectiveAt,
				},
			}
			rawPayload := domain.ProviderRawPayloadObservation{
				Connection:       request.Connection,
				Scope:            domain.RawPayloadScopeTransaction,
				ProviderObjectID: transaction.ProviderTransactionID,
				PayloadJSON:      []byte(`{"provider":"synthetic"}`),
				CapturedAt:       capturedAt,
			}
			connector := &stubConnector{
				connectorID: domain.ProviderConnectorIDSynthetic,
				fetchResult: domain.ProviderSyncBatch{
					Accounts:     []domain.ProviderAccountObservation{account},
					Balances:     []domain.ProviderBalanceObservation{balance},
					Transactions: []domain.ProviderTransactionObservation{transaction},
					RawPayloads:  []domain.ProviderRawPayloadObservation{rawPayload},
				},
			}

			executor := NewWindowSyncExecutor(
				WithConnectorRegistry(&stubConnectorRegistry{connector: connector}),
				WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			)

			result, err := executor.Execute(t.Context(), request)
			require.NoError(t, err)

			assert.Equal(t, 1, connector.fetchCalls)
			assert.Equal(t, request.Connection, connector.lastFetch.Connection)
			assert.Equal(t, request.Secret, connector.lastFetch.Secret)
			assert.Equal(t, request.RequestedWindow, connector.lastFetch.RequestedWindow)
			require.NotNil(t, connector.lastFetch.SyncState)
			assert.Equal(t, request.SyncState, connector.lastFetch.SyncState)
			assert.Equal(t, request.Connection, result.Batch.Connection)
			assert.Equal(t, request.RequestedWindow, result.Batch.RequestedWindow)
			require.Len(t, result.Batch.Accounts, 1)
			require.Len(t, result.Batch.Balances, 1)
			require.Len(t, result.Batch.Transactions, 1)
			require.Len(t, result.Batch.RawPayloads, 1)
			assert.Equal(t, account, result.Batch.Accounts[0])
			assert.Equal(t, balance, result.Batch.Balances[0])
			assert.Equal(t, transaction, result.Batch.Transactions[0])
			assert.Equal(t, rawPayload, result.Batch.RawPayloads[0])
			assert.Equal(t, domain.ProviderSyncStats{ObservedAccounts: 1, ObservedTransactions: 1}, result.Stats)
			assert.Nil(t, result.Issues)

			diffPlan := NewDiffPlanner().Plan(
				result.Batch,
				ExistingWindowSnapshot{
					Connection:      request.Connection,
					CandidateWindow: request.RequestedWindow,
				},
			)
			require.Len(t, diffPlan.AccountObservations, 1)
			require.Len(t, diffPlan.BalanceObservations, 1)
			require.Len(t, diffPlan.TransactionActions, 1)
			require.Len(t, diffPlan.RawPayloadObservations, 1)
			assert.Equal(t, account, diffPlan.AccountObservations[0])
			assert.Equal(t, balance, diffPlan.BalanceObservations[0])
			assert.Equal(t, transaction.Fingerprint, diffPlan.TransactionActions[0].Observation.Fingerprint)
			assert.Equal(t, transaction.ProviderOriginal, diffPlan.TransactionActions[0].Observation.ProviderOriginal)
			assert.Equal(t, rawPayload, diffPlan.RawPayloadObservations[0])
		})

		t.Run("returns fetch errors after connector resolution", func(t *testing.T) {
			fake := faker.New()
			request := makeRequest(fake, domain.ProviderIDSynthetic, domain.ProviderConnectorIDSynthetic)
			expectedErr := fmt.Errorf("fetch-%s", fake.UUID().V4())
			connector := &stubConnector{
				connectorID: domain.ProviderConnectorIDSynthetic,
				fetchErr:    expectedErr,
			}

			executor := NewWindowSyncExecutor(WithConnectorRegistry(&stubConnectorRegistry{connector: connector}))

			_, err := executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, expectedErr)
			assert.Equal(t, 1, connector.fetchCalls)
			assert.ErrorContains(t, err, "fetch sync batch")
		})
	})

	t.Run("with connectors wires supported technical connectors through registry-backed setup", func(t *testing.T) {
		fake := faker.New()
		monobankConnector := &stubConnector{connectorID: domain.ProviderConnectorIDMonobank}
		enableBankingConnector := &stubConnector{connectorID: domain.ProviderConnectorIDEnableBanking}

		executor := NewWindowSyncExecutor(
			WithConnectors(monobankConnector, enableBankingConnector, monobankConnector),
			WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
		)

		_, err := executor.Execute(
			t.Context(),
			makeRequest(fake, domain.ProviderIDMonobank, domain.ProviderConnectorIDMonobank),
		)
		require.NoError(t, err)

		_, err = executor.Execute(
			t.Context(),
			makeRequest(fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking),
		)
		require.NoError(t, err)
	})
}
