package providers

import (
	"context"
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

type stubSnapshotWindowPolicy struct {
	window         domain.ProviderSyncWindow
	err            error
	determineCalls []domain.ProviderSyncWindow
}

func (p *stubSnapshotWindowPolicy) Determine(
	requestedWindow domain.ProviderSyncWindow,
) (domain.ProviderSyncWindow, error) {
	p.determineCalls = append(p.determineCalls, requestedWindow)
	if p.err != nil {
		return domain.ProviderSyncWindow{}, p.err
	}
	return p.window, nil
}

type stubWindowSyncStore struct {
	snapshot          ExistingWindowSnapshot
	loadErr           error
	applyErr          error
	loadCalls         []domain.ProviderSyncWindow
	loadConnections   []domain.ProviderConnectionRef
	appliedDiffPlans  []ProviderDiffPlan
	appliedApplyPlans []ApplyPlan
}

func (r *stubWindowSyncStore) LoadExistingWindow(
	_ context.Context,
	connection domain.ProviderConnectionRef,
	window domain.ProviderSyncWindow,
) (ExistingWindowSnapshot, error) {
	r.loadConnections = append(r.loadConnections, connection)
	r.loadCalls = append(r.loadCalls, window)
	if r.loadErr != nil {
		return ExistingWindowSnapshot{}, r.loadErr
	}
	return r.snapshot, nil
}

func (r *stubWindowSyncStore) ApplySync(
	_ context.Context,
	diffPlan ProviderDiffPlan,
	applyPlan ApplyPlan,
) error {
	r.appliedDiffPlans = append(r.appliedDiffPlans, diffPlan)
	r.appliedApplyPlans = append(r.appliedApplyPlans, applyPlan)
	if r.applyErr != nil {
		return r.applyErr
	}
	return nil
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

	makeStore := func() *stubWindowSyncStore {
		return &stubWindowSyncStore{
			snapshot: ExistingWindowSnapshot{},
		}
	}

	t.Run("coordinate", func(t *testing.T) {
		t.Run("resolves monobank connections by technical connector id", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				connector: &stubConnector{connectorID: domain.ProviderConnectorIDMonobank},
			}
			request := makeRequest(fake, domain.ProviderIDMonobank, domain.ProviderConnectorIDMonobank)
			store := makeStore()

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithWindowSyncStore(store),
				WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			)
			require.NoError(t, err)

			result, err := executor.Execute(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(t, []domain.ProviderConnectorID{domain.ProviderConnectorIDMonobank}, registry.resolveCalls)
			assert.NotEmpty(t, result.RunID)
			assert.Equal(t, request.Connection, result.Batch.Connection)
			assert.Equal(t, request.RequestedWindow, result.Batch.RequestedWindow)
			assert.Equal(t, domain.ProviderSyncStats{}, result.Stats)
			assert.Equal(t, []domain.ProviderSyncWindow{request.RequestedWindow}, store.loadCalls)
		})

		t.Run("resolves pko connections through enable banking connector id", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				connector: &stubConnector{connectorID: domain.ProviderConnectorIDEnableBanking},
			}
			request := makeRequest(fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking)
			store := makeStore()

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithWindowSyncStore(store),
				WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			)
			require.NoError(t, err)

			_, err = executor.Execute(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(
				t,
				[]domain.ProviderConnectorID{domain.ProviderConnectorIDEnableBanking},
				registry.resolveCalls,
			)
		})

		t.Run("returns empty connector id errors from registry resolution", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				err: ErrConnectorIDRequired,
			}
			request := makeRequest(fake, domain.ProviderIDMonobank, "")

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithWindowSyncStore(makeStore()),
			)
			require.NoError(t, err)

			_, err = executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, ErrConnectorIDRequired)
			assert.Equal(t, []domain.ProviderConnectorID{""}, registry.resolveCalls)
			assert.NotContains(t, err.Error(), request.Secret.Reference)
			assert.NotContains(t, err.Error(), request.Secret.Envelope.Ciphertext)
		})

		t.Run("passes literal whitespace connector ids through registry resolution", func(t *testing.T) {
			fake := faker.New()
			whitespaceConnectorID := domain.ProviderConnectorID("   ")
			registry := &stubConnectorRegistry{
				err: fmt.Errorf("%w: %s", ErrConnectorNotConfigured, whitespaceConnectorID),
			}
			request := makeRequest(fake, domain.ProviderIDMonobank, whitespaceConnectorID)

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithWindowSyncStore(makeStore()),
			)
			require.NoError(t, err)

			_, err = executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, ErrConnectorNotConfigured)
			assert.Equal(t, []domain.ProviderConnectorID{whitespaceConnectorID}, registry.resolveCalls)
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

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithWindowSyncStore(makeStore()),
			)
			require.NoError(t, err)

			_, err = executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, ErrConnectorNotConfigured)
			assert.Equal(t, []domain.ProviderConnectorID{unknownConnectorID}, registry.resolveCalls)
			assert.Zero(t, connector.fetchCalls)
			assert.NotContains(t, err.Error(), request.Secret.Reference)
			assert.NotContains(t, err.Error(), request.Secret.Envelope.Ciphertext)
		})

		t.Run("fetches snapshot plans apply handoff and returns planned stats", func(t *testing.T) {
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
			snapshotWindow := domain.ProviderSyncWindow{
				Start: request.RequestedWindow.Start.Add(-6 * time.Hour),
				End:   request.RequestedWindow.End.Add(6 * time.Hour),
			}
			snapshotPolicy := &stubSnapshotWindowPolicy{window: snapshotWindow}
			store := &stubWindowSyncStore{
				snapshot: ExistingWindowSnapshot{
					Connection:     request.Connection,
					SnapshotWindow: snapshotWindow,
				},
			}

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(&stubConnectorRegistry{connector: connector}),
				WithSnapshotWindowPolicy(snapshotPolicy),
				WithWindowSyncStore(store),
				WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			)
			require.NoError(t, err)

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
			assert.Equal(t, []domain.ProviderSyncWindow{request.RequestedWindow}, snapshotPolicy.determineCalls)
			assert.Equal(t, []domain.ProviderConnectionRef{request.Connection}, store.loadConnections)
			assert.Equal(t, []domain.ProviderSyncWindow{snapshotWindow}, store.loadCalls)
			require.Len(t, store.appliedDiffPlans, 1)
			require.Len(t, store.appliedApplyPlans, 1)
			assert.Equal(t, request.RequestedWindow, store.appliedDiffPlans[0].RequestedWindow)
			assert.Equal(t, snapshotWindow, store.appliedDiffPlans[0].SnapshotWindow)
			require.Len(t, store.appliedDiffPlans[0].TransactionActions, 1)
			require.Len(t, store.appliedApplyPlans[0].TransactionWrites, 1)
			assert.Equal(
				t,
				domain.ProviderSyncStats{
					ObservedAccounts:     1,
					ObservedTransactions: 1,
					CreatedTransactions:  1,
				},
				result.Stats,
			)
			assert.Nil(t, result.Issues)

			diffPlan := NewDiffPlanner().Plan(
				result.Batch,
				ExistingWindowSnapshot{
					Connection:     request.Connection,
					SnapshotWindow: request.RequestedWindow,
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

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(&stubConnectorRegistry{connector: connector}),
				WithWindowSyncStore(makeStore()),
			)
			require.NoError(t, err)

			_, err = executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, expectedErr)
			assert.Equal(t, 1, connector.fetchCalls)
			assert.ErrorContains(t, err, "fetch sync batch")
		})

		t.Run("returns snapshot policy errors before store load", func(t *testing.T) {
			fake := faker.New()
			request := makeRequest(fake, domain.ProviderIDSynthetic, domain.ProviderConnectorIDSynthetic)
			expectedErr := fmt.Errorf("snapshot-policy-%s", fake.UUID().V4())
			connector := &stubConnector{connectorID: domain.ProviderConnectorIDSynthetic}
			snapshotPolicy := &stubSnapshotWindowPolicy{err: expectedErr}
			store := makeStore()

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(&stubConnectorRegistry{connector: connector}),
				WithSnapshotWindowPolicy(snapshotPolicy),
				WithWindowSyncStore(store),
			)
			require.NoError(t, err)

			_, err = executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, expectedErr)
			assert.Equal(t, 1, connector.fetchCalls)
			assert.Empty(t, store.loadCalls)
			assert.ErrorContains(t, err, "determine snapshot window")
		})

		t.Run("fails when the window sync store is missing", func(t *testing.T) {
			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(&stubConnectorRegistry{
					connector: &stubConnector{connectorID: domain.ProviderConnectorIDSynthetic},
				}),
			)
			require.ErrorIs(t, err, ErrWindowSyncStoreRequired)
			assert.Nil(t, executor)
		})

		t.Run("returns snapshot load errors after fetch", func(t *testing.T) {
			fake := faker.New()
			request := makeRequest(fake, domain.ProviderIDSynthetic, domain.ProviderConnectorIDSynthetic)
			expectedErr := fmt.Errorf("load-snapshot-%s", fake.UUID().V4())
			connector := &stubConnector{connectorID: domain.ProviderConnectorIDSynthetic}
			store := &stubWindowSyncStore{loadErr: expectedErr}

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(&stubConnectorRegistry{connector: connector}),
				WithWindowSyncStore(store),
			)
			require.NoError(t, err)

			_, err = executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, expectedErr)
			assert.Equal(t, []domain.ProviderSyncWindow{request.RequestedWindow}, store.loadCalls)
			assert.ErrorContains(t, err, "load existing snapshot")
		})

		t.Run("returns apply errors after planning", func(t *testing.T) {
			fake := faker.New()
			request := makeRequest(fake, domain.ProviderIDSynthetic, domain.ProviderConnectorIDSynthetic)
			expectedErr := fmt.Errorf("apply-sync-%s", fake.UUID().V4())
			connector := &stubConnector{
				connectorID: domain.ProviderConnectorIDSynthetic,
				fetchResult: domain.ProviderSyncBatch{
					Transactions: []domain.ProviderTransactionObservation{{
						Connection:            request.Connection,
						ProviderAccountID:     "provider-account-" + fake.UUID().V4(),
						ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
						Status:                domain.TransactionStatusBooked,
						AmountMinor:           -123,
						Currency:              "USD",
						Description:           "transaction-" + fake.Lorem().Word(),
						EffectiveAt:           request.RequestedWindow.Start.Add(time.Hour),
						Fingerprint:           "fingerprint-" + fake.UUID().V4(),
					}},
				},
			}
			store := &stubWindowSyncStore{
				snapshot: ExistingWindowSnapshot{},
				applyErr: expectedErr,
			}

			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(&stubConnectorRegistry{connector: connector}),
				WithWindowSyncStore(store),
			)
			require.NoError(t, err)

			_, err = executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, expectedErr)
			require.Len(t, store.appliedDiffPlans, 1)
			require.Len(t, store.appliedApplyPlans, 1)
			assert.ErrorContains(t, err, "apply sync")
		})

		t.Run("fails for invalid requested windows before fetch", func(t *testing.T) {
			fake := faker.New()
			connector := &stubConnector{connectorID: domain.ProviderConnectorIDSynthetic}

			testCases := map[string]domain.ProviderSyncWindow{
				"zero start": {
					End: time.Now().UTC(),
				},
				"zero end": {
					Start: time.Now().UTC(),
				},
				"inverted bounds": {
					Start: time.Now().UTC(),
					End:   time.Now().UTC().Add(-time.Minute),
				},
			}

			for name, requestedWindow := range testCases {
				t.Run(name, func(t *testing.T) {
					request := makeRequest(
						fake,
						domain.ProviderIDSynthetic,
						domain.ProviderConnectorIDSynthetic,
					)
					request.RequestedWindow = requestedWindow
					executor, err := NewWindowSyncExecutor(
						WithConnectorRegistry(&stubConnectorRegistry{connector: connector}),
						WithWindowSyncStore(makeStore()),
					)
					require.NoError(t, err)

					_, err = executor.Execute(t.Context(), request)
					require.ErrorIs(t, err, ErrInvalidRequestedWindow)
					assert.Zero(t, connector.fetchCalls)
				})
			}
		})
	})

	t.Run("constructor", func(t *testing.T) {
		t.Run("fails when the connector registry override is nil", func(t *testing.T) {
			executor, err := NewWindowSyncExecutor(
				WithConnectorRegistry(nil),
				WithWindowSyncStore(makeStore()),
			)
			require.ErrorIs(t, err, ErrConnectorRegistryRequired)
			assert.Nil(t, executor)
		})
	})

	t.Run("with connectors wires supported technical connectors through registry-backed setup", func(t *testing.T) {
		fake := faker.New()
		monobankConnector := &stubConnector{connectorID: domain.ProviderConnectorIDMonobank}
		enableBankingConnector := &stubConnector{connectorID: domain.ProviderConnectorIDEnableBanking}

		executor, err := NewWindowSyncExecutor(
			WithConnectors(monobankConnector, enableBankingConnector, monobankConnector),
			WithWindowSyncStore(makeStore()),
			WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
		)
		require.NoError(t, err)

		_, err = executor.Execute(
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
