package synthetic

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubProviderStateStore struct {
	state      *domain.SyntheticProviderState
	getErr     error
	saveErr    error
	savedState *domain.SyntheticProviderState
	getCalls   []string
	saveCalls  []domain.SyntheticProviderState
}

func (s *stubProviderStateStore) GetSyntheticProviderState(
	_ context.Context,
	providerReference string,
) (*domain.SyntheticProviderState, error) {
	s.getCalls = append(s.getCalls, providerReference)
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.state == nil {
		return nil, ErrProviderStateNotFound
	}
	copyState := *s.state
	copyState.Envelope = cloneEnvelope(s.state.Envelope)
	return &copyState, nil
}

func (s *stubProviderStateStore) SaveSyntheticProviderState(
	_ context.Context,
	state domain.SyntheticProviderState,
) (domain.SyntheticProviderState, error) {
	s.saveCalls = append(s.saveCalls, state)
	if s.saveErr != nil {
		return domain.SyntheticProviderState{}, s.saveErr
	}
	copyState := state
	copyState.Envelope = cloneEnvelope(state.Envelope)
	s.savedState = &copyState
	s.state = &copyState
	return copyState, nil
}

func TestConnector(t *testing.T) {
	makeRandomIntn := func(values ...int) func(int) int {
		index := 0
		return func(bound int) int {
			if bound <= 0 {
				return 0
			}
			if len(values) == 0 {
				return 0
			}
			value := values[index%len(values)]
			index++
			if value < 0 {
				value = -value
			}
			return value % bound
		}
	}

	makeConnection := func(fake faker.Faker) domain.ProviderConnectionRef {
		return domain.ProviderConnectionRef{
			ConnectionID:      "connection-" + fake.UUID().V4(),
			ProviderID:        domain.ProviderIDSynthetic,
			ConnectorID:       domain.ProviderConnectorIDSynthetic,
			ProviderReference: "provider-ref-" + fake.UUID().V4(),
			ExternalID:        "external-" + fake.UUID().V4(),
		}
	}

	t.Run("fetch generates first-window account balance transaction and raw payload observations", func(t *testing.T) {
		fake := faker.New()
		connection := makeConnection(fake)
		duplicateName := "wallet-" + fake.Lorem().Word()
		duplicateCurrency := "USD"
		requestedWindow := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 20, 15, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60)),
			End:   time.Date(2026, time.June, 22, 8, 45, 0, 0, time.FixedZone("UTC-4", -4*60*60)),
		}
		stateStore := &stubProviderStateStore{
			state: &domain.SyntheticProviderState{
				ProviderReference: connection.ProviderReference,
				Envelope: domain.SyntheticProviderStateEnvelope{
					Version: domain.SyntheticProviderStateVersion1,
					ConfiguredAccounts: []domain.SyntheticConfiguredAccount{{
						Key:      "synthetic-account-a-" + fake.UUID().V4(),
						Name:     duplicateName,
						Currency: duplicateCurrency,
					}, {
						Key:      "synthetic-account-b-" + fake.UUID().V4(),
						Name:     duplicateName,
						Currency: duplicateCurrency,
					}},
				},
				CreatedAt: time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC),
			},
		}
		now := time.Date(2026, time.June, 26, 10, 0, 0, 0, time.UTC)
		connector := NewConnector(
			stateStore,
			WithConnectorNow(func() time.Time { return now }),
			WithConnectorRandomIntn(makeRandomIntn(
				0, 1, 9, 30, 0, 0, 99, 0,
				1, 2, 11, 45, 0, 1, 149, 1, 13, 15, 0, 0, 249, 0,
				0, 3, 20, 5, 0, 1, 199, 1,
				1, 4, 6, 10, 0, 0, 349, 0, 8, 25, 0, 1, 449, 1,
				0, 5, 16, 55, 0, 0, 299, 0,
			)),
		)

		batch, err := connector.Fetch(t.Context(), providers.FetchRequest{
			Connection:      connection,
			RequestedWindow: requestedWindow,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{connection.ProviderReference}, stateStore.getCalls)

		require.Len(t, batch.Accounts, 2)
		require.Len(t, batch.Balances, 2)
		assert.GreaterOrEqual(t, len(batch.Transactions), 6)
		assert.LessOrEqual(t, len(batch.Transactions), 12)
		require.Len(t, batch.RawPayloads, len(batch.Accounts)+len(batch.Transactions))
		assert.Equal(t, connection, batch.Accounts[0].Connection)
		assert.Equal(t, connection, batch.Balances[0].Connection)

		normalizedWindow := newWindowKey(requestedWindow)
		generatedDays := []time.Time{
			normalizedWindow.NormalizedStartUTC,
			normalizedWindow.NormalizedStartUTC.Add(24 * time.Hour),
			normalizedWindow.NormalizedStartUTC.Add(48 * time.Hour),
		}
		accountCurrencies := map[string]string{}
		for _, account := range batch.Accounts {
			accountCurrencies[account.ProviderAccountID] = account.Currency
			assert.Contains(t, account.ProviderAccountID, connection.ConnectionID)
		}
		assert.NotEqual(t, batch.Accounts[0].ProviderAccountID, batch.Accounts[1].ProviderAccountID)
		assert.Equal(t, batch.Accounts[0].Name, batch.Accounts[1].Name)
		assert.Equal(t, batch.Accounts[0].Currency, batch.Accounts[1].Currency)

		seenTransactionIDs := map[string]struct{}{}
		seenDaysByAccount := map[string]map[string]int{}
		for _, transaction := range batch.Transactions {
			_, duplicate := seenTransactionIDs[transaction.ProviderTransactionID]
			assert.False(t, duplicate)
			seenTransactionIDs[transaction.ProviderTransactionID] = struct{}{}
			assert.Equal(t, domain.TransactionStatusBooked, transaction.Status)
			assert.Equal(t, accountCurrencies[transaction.ProviderAccountID], transaction.Currency)
			require.NotNil(t, transaction.ProviderOriginal)
			assert.Equal(t, transaction.AmountMinor, transaction.ProviderOriginal.AmountMinor)
			assert.Equal(t, transaction.Currency, transaction.ProviderOriginal.Currency)
			assert.NotEmpty(t, transaction.Fingerprint)
			dayKey := transaction.EffectiveAt.UTC().Format(time.DateOnly)
			if _, ok := seenDaysByAccount[transaction.ProviderAccountID]; !ok {
				seenDaysByAccount[transaction.ProviderAccountID] = map[string]int{}
			}
			seenDaysByAccount[transaction.ProviderAccountID][dayKey]++
		}
		for _, account := range batch.Accounts {
			require.Len(t, seenDaysByAccount[account.ProviderAccountID], len(generatedDays))
			for _, day := range generatedDays {
				count := seenDaysByAccount[account.ProviderAccountID][day.Format(time.DateOnly)]
				assert.GreaterOrEqual(t, count, 1)
				assert.LessOrEqual(t, count, 2)
			}
		}

		require.NotNil(t, stateStore.savedState)
		assert.Equal(t, connection.ProviderReference, stateStore.savedState.ProviderReference)
		require.Len(t, stateStore.savedState.Envelope.WindowHistory, 1)
		assert.Equal(t, normalizedWindow, stateStore.savedState.Envelope.WindowHistory[0].Window)
		assert.Equal(t, 1, stateStore.savedState.Envelope.WindowHistory[0].RepeatCount)
		require.Len(t, stateStore.savedState.Envelope.SequenceCounters, len(batch.Accounts)*len(generatedDays))
		assert.Equal(t, now, stateStore.savedState.UpdatedAt)
	})

	t.Run("fetch preserves repeated-window sequencing and exposes guardrails", func(t *testing.T) {
		fake := faker.New()
		connection := makeConnection(fake)
		requestedWindow := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 20, 15, 0, 0, 0, time.UTC),
			End:   time.Date(2026, time.June, 22, 0, 0, 0, 0, time.UTC),
		}
		normalizedWindow := newWindowKey(requestedWindow)
		lastDay := normalizedWindow.NormalizedEndExclusiveUTC.Add(-24 * time.Hour)
		firstAccountKey := "synthetic-account-a-" + fake.UUID().V4()
		secondAccountKey := "synthetic-account-b-" + fake.UUID().V4()
		stateStore := &stubProviderStateStore{state: &domain.SyntheticProviderState{
			ProviderReference: connection.ProviderReference,
			Envelope: domain.SyntheticProviderStateEnvelope{
				Version: domain.SyntheticProviderStateVersion1,
				ConfiguredAccounts: []domain.SyntheticConfiguredAccount{
					{Key: firstAccountKey, Name: "wallet-a-" + fake.Lorem().Word(), Currency: "USD"},
					{Key: secondAccountKey, Name: "wallet-b-" + fake.Lorem().Word(), Currency: "USD"},
				},
				WindowHistory: []domain.SyntheticWindowHistoryEntry{{
					Window:      normalizedWindow,
					RepeatCount: 1,
				}},
				SequenceCounters: []domain.SyntheticAccountDaySequenceCounter{
					{AccountKey: firstAccountKey, DayUTC: lastDay, NextSequence: 3},
					{AccountKey: secondAccountKey, DayUTC: lastDay, NextSequence: 8},
					{AccountKey: firstAccountKey, DayUTC: lastDay.Add(-24 * time.Hour), NextSequence: 5},
				},
			},
		}}
		connector := NewConnector(
			stateStore,
			WithConnectorNow(func() time.Time { return time.Date(2026, time.June, 26, 10, 0, 0, 0, time.UTC) }),
			WithConnectorRandomIntn(makeRandomIntn(
				1, 0, 10, 15, 0, 1, 59, 1,
				0, 0, 20, 45, 0, 0, 80, 0,
				2, 1, 9, 30, 0, 0, 99, 0,
				1, 0, 23, 50, 0, 1, 149, 1,
			)),
		)
		batch, err := connector.Fetch(t.Context(), providers.FetchRequest{
			Connection:      connection,
			RequestedWindow: requestedWindow,
		})
		require.NoError(t, err)
		require.NotEmpty(t, batch.Transactions)
		for _, transaction := range batch.Transactions {
			assert.Equal(t, lastDay.Format(time.DateOnly), transaction.EffectiveAt.UTC().Format(time.DateOnly))
		}
		require.NotNil(t, stateStore.savedState)
		assert.Equal(t, 2, stateStore.savedState.Envelope.WindowHistory[0].RepeatCount)
		for _, counter := range stateStore.savedState.Envelope.SequenceCounters {
			if counter.AccountKey == firstAccountKey && counter.DayUTC.Equal(lastDay) {
				assert.GreaterOrEqual(t, counter.NextSequence, 4)
			}
		}

		connectorWithoutStore := NewConnector(nil)
		_, err = connectorWithoutStore.Fetch(t.Context(), providers.FetchRequest{Connection: connection})
		require.ErrorIs(t, err, ErrProviderStateStoreRequired)

		startResult, err := connectorWithoutStore.StartLink(t.Context(), providers.StartLinkRequest{})
		require.NoError(t, err)
		assert.NotEmpty(t, startResult.State)
		assert.Equal(t, startResult.State, startResult.ProviderReference)
		assert.Equal(t, "#/finance/connections/synthetic?state="+startResult.State, startResult.AuthorizationURL)
		_, err = connectorWithoutStore.FinishLink(t.Context(), providers.FinishLinkRequest{})
		require.ErrorIs(t, err, ErrProviderStateStoreRequired)
		_, err = connectorWithoutStore.LinkToken(t.Context(), providers.LinkTokenRequest{})
		require.ErrorIs(t, err, ErrConnectorLinkUnsupported)
		assert.Equal(t, domain.ProviderConnectorIDSynthetic, connectorWithoutStore.ConnectorID())
		assert.Equal(t, providers.ConnectorCapabilities{
			SupportsStartLink:  true,
			SupportsFinishLink: true,
			SupportsFetch:      true,
		}, connectorWithoutStore.Capabilities())
	})

	t.Run("supports local synthetic start and finish lifecycle", func(t *testing.T) {
		fake := faker.New()
		state := "state-" + fake.UUID().V4()
		stateStore := &stubProviderStateStore{}
		connector := NewConnector(
			stateStore,
			WithConnectorStateGenerator(func() string { return state }),
		)

		startResult, err := connector.StartLink(t.Context(), providers.StartLinkRequest{
			BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
		})
		require.NoError(t, err)
		assert.Equal(t, state, startResult.State)
		assert.Equal(t, state, startResult.ProviderReference)
		assert.Equal(t, "#/finance/connections/synthetic?state="+state, startResult.AuthorizationURL)
		assert.Equal(t, providers.ConnectorCapabilities{
			SupportsStartLink:  true,
			SupportsFinishLink: true,
			SupportsFetch:      true,
		}, connector.Capabilities())

		stateStore.state = &domain.SyntheticProviderState{ProviderReference: state}
		_, err = connector.FinishLink(t.Context(), providers.FinishLinkRequest{State: state})
		require.ErrorContains(t, err, "configured synthetic state")
		assert.Equal(t, []string{state}, stateStore.getCalls)

		stateStore.state = &domain.SyntheticProviderState{
			ProviderReference: state,
			Envelope: domain.SyntheticProviderStateEnvelope{
				Version: domain.SyntheticProviderStateVersion1,
				ConfiguredAccounts: []domain.SyntheticConfiguredAccount{{
					Key:      "synthetic-account-" + fake.UUID().V4(),
					Name:     "wallet-" + fake.Lorem().Word(),
					Currency: "USD",
				}},
			},
		}

		finishResult, err := connector.FinishLink(t.Context(), providers.FinishLinkRequest{State: state})
		require.NoError(t, err)
		assert.Equal(t, ConnectionDisplayName, finishResult.DisplayName)
		assert.Equal(t, state, finishResult.ProviderReference)
		assert.Empty(t, finishResult.Secret)
		assert.Equal(t, domain.BankConnectionStateActive, finishResult.State)
		assert.Equal(t, []string{state, state}, stateStore.getCalls)
	})

	t.Run("wraps provider state errors and keeps helpers deterministic", func(t *testing.T) {
		fake := faker.New()
		connection := makeConnection(fake)
		stateStore := &stubProviderStateStore{
			state: &domain.SyntheticProviderState{
				ProviderReference: connection.ProviderReference,
				Envelope: domain.SyntheticProviderStateEnvelope{
					Version: domain.SyntheticProviderStateVersion1,
					ConfiguredAccounts: []domain.SyntheticConfiguredAccount{{
						Key:      "synthetic-account-" + fake.UUID().V4(),
						Name:     "wallet-" + fake.Lorem().Word(),
						Currency: "USD",
					}},
				},
			},
			saveErr: fmt.Errorf("save-%s", fake.UUID().V4()),
		}
		connector := NewConnector(
			stateStore,
			WithConnectorRandomIntn(makeRandomIntn(0, 1, 9, 0, 0, 0, 49, 0, 10, 30, 0, 1, 59, 1)),
		)
		_, err := connector.Fetch(t.Context(), providers.FetchRequest{
			Connection: connection,
			RequestedWindow: domain.ProviderSyncWindow{
				Start: time.Date(2026, time.June, 24, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60)),
				End:   time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC),
			},
		})
		require.ErrorContains(t, err, "save synthetic provider state")

		missingStateStore := &stubProviderStateStore{}
		missingConnector := NewConnector(missingStateStore)
		_, err = missingConnector.Fetch(t.Context(), providers.FetchRequest{Connection: connection})
		require.ErrorIs(t, err, ErrProviderStateNotFound)

		failingStore := &stubProviderStateStore{getErr: fmt.Errorf("get-%s", fake.UUID().V4())}
		_, err = NewConnector(failingStore).Fetch(t.Context(), providers.FetchRequest{Connection: connection})
		require.ErrorContains(t, err, "load synthetic provider state")

		assert.Equal(
			t,
			[]time.Time{time.Date(2026, time.June, 24, 0, 0, 0, 0, time.UTC)},
			windowDays(newWindowKey(domain.ProviderSyncWindow{
				Start: time.Date(2026, time.June, 24, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60)),
				End:   time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC),
			})),
		)
		assert.Equal(t, -1, windowHistoryIndex(nil, domain.SyntheticWindowKey{}))
		assert.Equal(t, "value-with-bad-chars", sanitizeIDPart(" value:with/bad-chars "))
		assert.Contains(t, providerAccountID(connection, "account:key"), connection.ConnectionID)
	})

	t.Run("covers connector options helper branches", func(t *testing.T) {
		connector := NewConnector(
			&stubProviderStateStore{},
			WithConnectorLogger(nil),
			WithConnectorNow(nil),
			WithConnectorRandomIntn(nil),
		)
		require.NotNil(t, connector.logger)
		require.NotNil(t, connector.now)
		require.NotNil(t, connector.randomIntn)
	})

	t.Run("covers helper error and normalization branches", func(t *testing.T) {
		connector := NewConnector(nil)
		assert.Zero(t, connector.randomBounded(0))
		connector.randomIntn = func(int) int { return -9 }
		assert.Equal(t, 1, connector.randomBounded(4))

		counters := []domain.SyntheticAccountDaySequenceCounter{{
			AccountKey:   "account-1",
			DayUTC:       time.Date(2026, time.June, 24, 0, 0, 0, 0, time.UTC),
			NextSequence: 0,
		}}
		assert.Equal(t, 1, nextSequence(&counters, "account-1", counters[0].DayUTC))
		assert.Equal(t, 2, counters[0].NextSequence)

		_, err := payloadJSON(make(chan int))
		require.ErrorContains(t, err, "marshal synthetic payload")
	})
}
