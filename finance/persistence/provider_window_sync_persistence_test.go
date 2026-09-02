//go:build postgres_test

package persistence

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ providers.WindowSyncPersistence = (*ProviderWindowSyncPersistence)(nil)

func TestProviderWindowSyncPersistence(t *testing.T) {
	makeStore := func(t *testing.T) *Store {
		t.Helper()
		return NewStore(openTestDatabase(t))
	}

	makeProviderAccount := func(
		fake faker.Faker,
		connectionID string,
		financeAccountID string,
		createdAt time.Time,
	) domain.ConnectionProviderAccount {
		return domain.ConnectionProviderAccount{
			ID:                "provider-account-row-" + fake.UUID().V4(),
			ConnectionID:      connectionID,
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			FinanceAccountID:  financeAccountID,
			Name:              "account-" + fake.Lorem().Word(),
			Currency:          "PLN",
			IBAN:              "PL61109010140000071219812874",
			MaskedPAN:         "4444",
			CreatedAt:         createdAt,
			UpdatedAt:         createdAt.Add(time.Minute),
		}
	}

	makeTransaction := func(
		fake faker.Faker,
		accountID string,
		source domain.TransactionSource,
		effectiveAt time.Time,
	) domain.Transaction {
		return domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			AccountID:   accountID,
			Source:      source,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: int64(-fake.IntBetween(100, 90000)),
			Currency:    "PLN",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: effectiveAt,
			CreatedAt:   effectiveAt.Add(-time.Minute),
			UpdatedAt:   effectiveAt,
		}
	}

	makeMatch := func(
		fake faker.Faker,
		connectionID string,
		providerAccountID string,
		transactionID string,
		createdAt time.Time,
	) domain.ProviderTransactionMatch {
		return domain.ProviderTransactionMatch{
			ID:                    "match-" + fake.UUID().V4(),
			ConnectionID:          connectionID,
			ProviderAccountID:     providerAccountID,
			ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
			Fingerprint:           "fingerprint-" + fake.UUID().V4(),
			TransactionID:         transactionID,
			Status:                domain.TransactionStatusBooked,
			CreatedAt:             createdAt,
			UpdatedAt:             createdAt.Add(time.Minute),
		}
	}

	t.Run("delegates provider account listing through dedicated adapter", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		now := time.Now().UTC()
		connectionID := "connection-" + fake.UUID().V4()
		otherConnectionID := "other-connection-" + fake.UUID().V4()

		firstAccount, err := store.SaveConnectionProviderAccount(
			t.Context(),
			makeProviderAccount(fake, connectionID, "finance-account-1-"+fake.UUID().V4(), now),
		)
		require.NoError(t, err)
		secondAccount, err := store.SaveConnectionProviderAccount(
			t.Context(),
			makeProviderAccount(fake, connectionID, "finance-account-2-"+fake.UUID().V4(), now.Add(time.Second)),
		)
		require.NoError(t, err)
		_, err = store.SaveConnectionProviderAccount(
			t.Context(),
			makeProviderAccount(fake, otherConnectionID, "finance-account-3-"+fake.UUID().V4(), now.Add(2*time.Second)),
		)
		require.NoError(t, err)

		accounts, err := adapter.ListConnectionProviderAccounts(t.Context(), connectionID)
		require.NoError(t, err)
		assert.Equal(t, []domain.ConnectionProviderAccount{firstAccount, secondAccount}, accounts)
	})

	t.Run("lists provider source transactions scoped by finance accounts and window", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		windowStart := time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)
		window := domain.ProviderSyncWindow{Start: windowStart, End: windowStart.Add(72 * time.Hour)}
		accountIDOne := "finance-account-1-" + fake.UUID().V4()
		accountIDTwo := "finance-account-2-" + fake.UUID().V4()
		otherAccountID := "finance-account-other-" + fake.UUID().V4()

		insideLater, err := store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountIDOne, domain.TransactionSourceProvider, windowStart.Add(48*time.Hour)),
		)
		require.NoError(t, err)
		insideEarlier, err := store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountIDTwo, domain.TransactionSourceProvider, windowStart.Add(12*time.Hour)),
		)
		require.NoError(t, err)
		_, err = store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountIDOne, domain.TransactionSourceManual, windowStart.Add(24*time.Hour)),
		)
		require.NoError(t, err)
		_, err = store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, otherAccountID, domain.TransactionSourceProvider, windowStart.Add(24*time.Hour)),
		)
		require.NoError(t, err)
		_, err = store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountIDOne, domain.TransactionSourceProvider, windowStart.Add(-time.Hour)),
		)
		require.NoError(t, err)
		_, err = store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountIDOne, domain.TransactionSourceProvider, window.End),
		)
		require.NoError(t, err)

		transactions, err := adapter.ListProviderTransactionsInWindow(
			t.Context(),
			[]string{"  " + accountIDOne + "  ", accountIDTwo, "   "},
			window,
		)
		require.NoError(t, err)
		require.Len(t, transactions, 2)
		assert.Equal(t, []string{insideLater.ID, insideEarlier.ID}, []string{transactions[0].ID, transactions[1].ID})
		assert.True(t, insideLater.EffectiveAt.Equal(transactions[0].EffectiveAt))
		assert.True(t, insideEarlier.EffectiveAt.Equal(transactions[1].EffectiveAt))
	})

	t.Run("filters and orders provider transactions by canonical timestamp", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		accountID := "finance-account-" + fake.UUID().V4()
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123000, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456000, time.FixedZone("zero", 0))
		require.True(t, earlier.Before(later))

		earlierTransaction, err := store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountID, domain.TransactionSourceProvider, earlier),
		)
		require.NoError(t, err)
		laterTransaction, err := store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountID, domain.TransactionSourceProvider, later),
		)
		require.NoError(t, err)
		transactions, err := adapter.ListProviderTransactionsInWindow(
			t.Context(),
			[]string{accountID},
			domain.ProviderSyncWindow{Start: earlier.Add(-time.Minute), End: later.Add(time.Minute)},
		)
		require.NoError(t, err)
		require.Len(t, transactions, 2)
		assert.Equal(t, []string{laterTransaction.ID, earlierTransaction.ID}, []string{
			transactions[0].ID,
			transactions[1].ID,
		})
		assert.True(t, later.Equal(transactions[0].EffectiveAt))
		assert.True(t, earlier.Equal(transactions[1].EffectiveAt))

		boundaryTransactions, err := adapter.ListProviderTransactionsInWindow(
			t.Context(),
			[]string{accountID},
			domain.ProviderSyncWindow{Start: earlier.Add(-time.Minute), End: earlier.Add(time.Minute)},
		)
		require.NoError(t, err)
		require.Len(t, boundaryTransactions, 1)
		assert.Equal(t, earlierTransaction.ID, boundaryTransactions[0].ID)

		mixedOffsetAt := time.Date(2026, time.January, 1, 0, 0, 0, 789000, time.FixedZone("east", 2*60*60))
		mixedTransaction, err := store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountID, domain.TransactionSourceProvider, mixedOffsetAt),
		)
		require.NoError(t, err)
		boundaryTransactions, err = adapter.ListProviderTransactionsInWindow(
			t.Context(),
			[]string{accountID},
			domain.ProviderSyncWindow{
				Start: time.Date(2025, time.December, 31, 21, 30, 0, 0, time.UTC),
				End:   time.Date(2025, time.December, 31, 22, 30, 0, 0, time.UTC),
			},
		)
		require.NoError(t, err)
		require.Equal(t, []string{mixedTransaction.ID}, []string{boundaryTransactions[0].ID})
	})

	t.Run("lists provider transaction matches scoped by connection and transaction ids", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		now := time.Date(2025, time.December, 31, 23, 30, 0, 123000, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456000, time.FixedZone("zero", 0))
		require.True(t, now.Before(later))
		connectionID := "connection-" + fake.UUID().V4()
		otherConnectionID := "other-connection-" + fake.UUID().V4()
		providerAccountID := "provider-account-" + fake.UUID().V4()
		transactionIDOne := "transaction-1-" + fake.UUID().V4()
		transactionIDTwo := "transaction-2-" + fake.UUID().V4()
		otherTransactionID := "transaction-3-" + fake.UUID().V4()

		firstMatch, err := store.SaveProviderTransactionMatch(
			t.Context(),
			makeMatch(fake, connectionID, providerAccountID, transactionIDOne, now),
		)
		require.NoError(t, err)
		secondMatch, err := store.SaveProviderTransactionMatch(
			t.Context(),
			makeMatch(fake, connectionID, providerAccountID, transactionIDTwo, later),
		)
		require.NoError(t, err)
		_, err = store.SaveProviderTransactionMatch(
			t.Context(),
			makeMatch(fake, connectionID, providerAccountID, otherTransactionID, now.Add(2*time.Second)),
		)
		require.NoError(t, err)
		_, err = store.SaveProviderTransactionMatch(
			t.Context(),
			makeMatch(fake, otherConnectionID, providerAccountID, transactionIDOne, now.Add(3*time.Second)),
		)
		require.NoError(t, err)

		matches, err := adapter.ListProviderTransactionMatchesByTransactionIDs(
			t.Context(),
			"  "+connectionID+"  ",
			[]string{"  " + transactionIDOne + "  ", transactionIDTwo, "   "},
		)
		require.NoError(t, err)
		require.Len(t, matches, 2)
		assert.Equal(t, []string{firstMatch.ID, secondMatch.ID}, []string{matches[0].ID, matches[1].ID})
		assert.True(t, now.Equal(matches[0].CreatedAt))
		assert.True(t, later.Equal(matches[1].CreatedAt))
	})

	t.Run("lists provider transaction identity matches without effective time filtering", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		now := time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC)
		connectionID := "connection-" + fake.UUID().V4()
		otherConnectionID := "other-connection-" + fake.UUID().V4()
		providerAccountID := "provider-account-" + fake.UUID().V4()
		providerTransactionID := "provider-transaction-" + fake.UUID().V4()
		accountID := "finance-account-" + fake.UUID().V4()

		outsideTransaction, err := store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountID, domain.TransactionSourceProvider, now.Add(-48*time.Hour)),
		)
		require.NoError(t, err)
		matchingRow := makeMatch(fake, connectionID, providerAccountID, outsideTransaction.ID, now)
		matchingRow.ProviderTransactionID = providerTransactionID
		matchingRow, err = store.SaveProviderTransactionMatch(t.Context(), matchingRow)
		require.NoError(t, err)

		otherTransaction, err := store.SaveTransaction(
			t.Context(),
			makeTransaction(fake, accountID, domain.TransactionSourceProvider, now.Add(-24*time.Hour)),
		)
		require.NoError(t, err)
		otherProviderIDRow := makeMatch(
			fake,
			connectionID,
			providerAccountID,
			otherTransaction.ID,
			now.Add(time.Second),
		)
		_, err = store.SaveProviderTransactionMatch(t.Context(), otherProviderIDRow)
		require.NoError(t, err)
		otherConnectionRow := makeMatch(
			fake,
			otherConnectionID,
			providerAccountID,
			outsideTransaction.ID,
			now.Add(2*time.Second),
		)
		otherConnectionRow.ProviderTransactionID = providerTransactionID
		_, err = store.SaveProviderTransactionMatch(t.Context(), otherConnectionRow)
		require.NoError(t, err)
		otherAccountRow := makeMatch(
			fake,
			connectionID,
			"provider-account-other-"+fake.UUID().V4(),
			outsideTransaction.ID,
			now.Add(3*time.Second),
		)
		otherAccountRow.ProviderTransactionID = providerTransactionID
		_, err = store.SaveProviderTransactionMatch(t.Context(), otherAccountRow)
		require.NoError(t, err)

		items, err := adapter.ListProviderTransactionIdentityMatches(
			t.Context(),
			connectionID,
			[]providers.ProviderTransactionIdentity{{
				ProviderAccountID:     providerAccountID,
				ProviderTransactionID: providerTransactionID,
			}},
		)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.True(t, outsideTransaction.EffectiveAt.Equal(items[0].Transaction.EffectiveAt))
		assert.True(t, outsideTransaction.CreatedAt.Equal(items[0].Transaction.CreatedAt))
		assert.True(t, outsideTransaction.UpdatedAt.Equal(items[0].Transaction.UpdatedAt))
		assert.True(t, matchingRow.CreatedAt.Equal(items[0].Match.CreatedAt))
		assert.True(t, matchingRow.UpdatedAt.Equal(items[0].Match.UpdatedAt))
		expected := []providers.ProviderTransactionIdentityMatch{{
			Transaction: outsideTransaction,
			Match:       matchingRow,
		}}
		expected[0].Transaction.EffectiveAt = items[0].Transaction.EffectiveAt
		expected[0].Transaction.CreatedAt = items[0].Transaction.CreatedAt
		expected[0].Transaction.UpdatedAt = items[0].Transaction.UpdatedAt
		expected[0].Match.CreatedAt = items[0].Match.CreatedAt
		expected[0].Match.UpdatedAt = items[0].Match.UpdatedAt
		assert.Equal(t, expected, items)
	})

	t.Run("returns empty results when snapshot query inputs are empty", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		windowStart := time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)
		window := domain.ProviderSyncWindow{Start: windowStart, End: windowStart.Add(24 * time.Hour)}

		transactions, err := adapter.ListProviderTransactionsInWindow(
			t.Context(),
			[]string{"   ", "\t"},
			window,
		)
		require.NoError(t, err)
		assert.Empty(t, transactions)

		matches, err := adapter.ListProviderTransactionMatchesByTransactionIDs(
			t.Context(),
			"connection-"+fake.UUID().V4(),
			[]string{"   ", "\n"},
		)
		require.NoError(t, err)
		assert.Empty(t, matches)

		identityMatches, err := adapter.ListProviderTransactionIdentityMatches(
			t.Context(),
			"connection-"+fake.UUID().V4(),
			[]providers.ProviderTransactionIdentity{{ProviderAccountID: "\t"}},
		)
		require.NoError(t, err)
		assert.Empty(t, identityMatches)
	})

	t.Run("applies provider metadata refresh to persisted linked finance accounts", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		syncStore, err := providers.NewProviderWindowSyncStore(
			adapter,
			providers.WithWindowSyncStoreNow(func() time.Time {
				return time.Date(2026, time.July, 7, 18, 30, 0, 0, time.UTC)
			}),
		)
		require.NoError(t, err)

		now := time.Date(2026, time.July, 7, 17, 30, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		financeAccountID := "finance-account-" + fake.UUID().V4()
		providerAccountID := "provider-account-" + fake.UUID().V4()
		readableName := "provider-name-" + fake.Lorem().Word()

		_, err = store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:          connectionID,
			TenantID:    tenantID,
			Provider:    string(domain.ProviderIDPKO),
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       domain.BankConnectionStateActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		_, err = store.SaveAccount(t.Context(), domain.Account{
			ID:       financeAccountID,
			TenantID: tenantID,
			Name:     providerAccountID,
			Currency: "",
			Kind:     domain.AccountKindLinked,
			LinkedAccount: &domain.LinkedAccount{
				Provider:          string(domain.ProviderIDPKO),
				ProviderAccountID: providerAccountID,
			},
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		})
		require.NoError(t, err)
		_, err = store.SaveConnectionProviderAccount(t.Context(), domain.ConnectionProviderAccount{
			ID:                "provider-account-row-" + fake.UUID().V4(),
			ConnectionID:      connectionID,
			ProviderAccountID: providerAccountID,
			FinanceAccountID:  financeAccountID,
			Name:              providerAccountID,
			Currency:          "",
			CreatedAt:         now.Add(-time.Hour),
			UpdatedAt:         now.Add(-time.Hour),
		})
		require.NoError(t, err)

		_, err = syncStore.ApplySync(t.Context(), providers.ProviderDiffPlan{
			Connection: domain.ProviderConnectionRef{
				ConnectionID: connectionID,
				ProviderID:   domain.ProviderIDPKO,
				ConnectorID:  domain.ProviderConnectorIDEnableBanking,
			},
			SnapshotWindow: domain.ProviderSyncWindow{Start: now.Add(-24 * time.Hour), End: now},
			AccountObservations: []domain.ProviderAccountObservation{{
				ProviderAccountID: providerAccountID,
				Name:              readableName,
				Currency:          "EUR",
			}},
		}, providers.ApplyPlan{})
		require.NoError(t, err)

		loadedAccount, err := store.GetAccount(t.Context(), financeAccountID)
		require.NoError(t, err)
		require.NotNil(t, loadedAccount)
		assert.Equal(t, readableName, loadedAccount.Name)
		assert.Equal(t, "EUR", loadedAccount.Currency)
	})

	t.Run("creates first linked account mapping and transaction from durable connection ownership", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
		connection := domain.ProviderConnectionRef{
			ConnectionID:      "connection-" + fake.UUID().V4(),
			ProviderID:        domain.ProviderIDPKO,
			ConnectorID:       domain.ProviderConnectorIDEnableBanking,
			ProviderReference: "reference-" + fake.UUID().V4(),
		}
		tenantID := "tenant-" + fake.UUID().V4()
		_, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                connection.ConnectionID,
			TenantID:          tenantID,
			Provider:          string(connection.ProviderID),
			ConnectorID:       connection.ConnectorID,
			ProviderReference: connection.ProviderReference,
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
		require.NoError(t, err)

		syncStore, err := providers.NewProviderWindowSyncStore(
			adapter,
			providers.WithWindowSyncStoreNow(func() time.Time { return now }),
		)
		require.NoError(t, err)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		transactionObservation := domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     providerAccountID,
			ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           int64(-fake.IntBetween(100, 90000)),
			Currency:              "pln",
			Description:           "transaction-" + fake.Lorem().Word(),
			EffectiveAt:           now.Add(-time.Hour),
			Fingerprint:           "fingerprint-" + fake.UUID().V4(),
		}
		createAction := providers.ProviderTransactionAction{
			Type:        providers.ProviderTransactionActionTypeCreate,
			Observation: transactionObservation,
		}
		attemptedAt := now.Add(-time.Minute)
		completedAt := now
		successState := domain.ProviderSyncState{
			Connection:  connection,
			AttemptedAt: &attemptedAt,
			SucceededAt: &completedAt,
			Window: domain.ProviderSyncWindow{
				Start: now.Add(-24 * time.Hour),
				End:   now,
			},
			RunID: "run-" + fake.UUID().V4(),
			JobID: "job-" + fake.UUID().V4(),
		}
		stats, err := syncStore.ApplySync(t.Context(), providers.ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: domain.ProviderSyncWindow{Start: now.Add(-24 * time.Hour), End: now},
			AccountObservations: []domain.ProviderAccountObservation{{
				Connection:        connection,
				ProviderAccountID: providerAccountID,
				Name:              "account-" + fake.Lorem().Word(),
				Currency:          "pln",
			}},
			TransactionActions: []providers.ProviderTransactionAction{createAction},
		}, providers.ApplyPlan{
			TransactionWrites: []providers.ApplyTransactionWrite{{Action: createAction}},
			Stats: domain.ProviderSyncStats{
				ObservedAccounts:     1,
				ObservedTransactions: 1,
				CreatedTransactions:  1,
			},
		}, successState)
		require.NoError(t, err)
		assert.Equal(t, domain.ProviderSyncStats{
			ObservedAccounts:     1,
			CreatedAccounts:      1,
			ObservedTransactions: 1,
			CreatedTransactions:  1,
		}, stats)

		mappings, err := adapter.ListConnectionProviderAccounts(t.Context(), connection.ConnectionID)
		require.NoError(t, err)
		require.Len(t, mappings, 1)
		financeAccount, err := store.GetAccount(t.Context(), mappings[0].FinanceAccountID)
		require.NoError(t, err)
		require.NotNil(t, financeAccount)
		assert.Equal(t, tenantID, financeAccount.TenantID)
		assert.Equal(t, domain.AccountKindLinked, financeAccount.Kind)
		assert.Equal(t, &domain.LinkedAccount{
			Provider:          string(connection.ProviderID),
			ProviderAccountID: providerAccountID,
		}, financeAccount.LinkedAccount)

		transactions, err := store.ListTransactions(
			t.Context(),
			tenantID,
			mappings[0].FinanceAccountID,
			domain.TransactionSourceProvider,
			"",
			false,
		)
		require.NoError(t, err)
		require.Len(t, transactions, 1)
		assert.Equal(t, tenantID, transactions[0].TenantID)
		journalState, err := NewProviderSyncStateJournalStore(store).LoadLastState(t.Context(), connection)
		require.NoError(t, err)
		require.NotNil(t, journalState)
		assert.Equal(t, successState.RunID, journalState.RunID)
		assert.Equal(t, successState.JobID, journalState.JobID)
		assert.Equal(t, stats, journalState.AggregateStats)
	})

	t.Run(
		"applies an outside-window provider-id correction without replacing persisted identities",
		func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			adapter := NewProviderWindowSyncPersistence(store)
			now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
			connection := domain.ProviderConnectionRef{
				ConnectionID:      "connection-" + fake.UUID().V4(),
				ProviderID:        domain.ProviderIDPKO,
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				ProviderReference: "reference-" + fake.UUID().V4(),
			}
			window := domain.ProviderSyncWindow{Start: now.Add(-24 * time.Hour), End: now}
			tenantID := "tenant-" + fake.UUID().V4()
			financeAccountID := "finance-account-" + fake.UUID().V4()
			providerAccountID := "provider-account-" + fake.UUID().V4()
			_, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
				ID:                connection.ConnectionID,
				TenantID:          tenantID,
				Provider:          string(connection.ProviderID),
				ConnectorID:       connection.ConnectorID,
				ProviderReference: connection.ProviderReference,
				State:             domain.BankConnectionStateActive,
				CreatedAt:         now.Add(-48 * time.Hour),
				UpdatedAt:         now.Add(-48 * time.Hour),
			})
			require.NoError(t, err)
			_, err = store.SaveConnectionProviderAccount(t.Context(), domain.ConnectionProviderAccount{
				ID:                "provider-account-row-" + fake.UUID().V4(),
				ConnectionID:      connection.ConnectionID,
				ProviderAccountID: providerAccountID,
				FinanceAccountID:  financeAccountID,
				Name:              "account-" + fake.Lorem().Word(),
				Currency:          "PLN",
				CreatedAt:         now.Add(-48 * time.Hour),
				UpdatedAt:         now.Add(-48 * time.Hour),
			})
			require.NoError(t, err)
			existingTransaction, err := store.SaveTransaction(t.Context(), domain.Transaction{
				ID:          "transaction-" + fake.UUID().V4(),
				TenantID:    tenantID,
				AccountID:   financeAccountID,
				Source:      domain.TransactionSourceProvider,
				Status:      domain.TransactionStatusPending,
				Kind:        domain.TransactionKindRegular,
				AmountMinor: -int64(fake.IntBetween(100, 90000)),
				Currency:    "PLN",
				Description: "pending-" + fake.Lorem().Word(),
				EffectiveAt: window.Start.Add(-time.Hour),
				CreatedAt:   now.Add(-48 * time.Hour),
				UpdatedAt:   now.Add(-48 * time.Hour),
			})
			require.NoError(t, err)
			existingMatch, err := store.SaveProviderTransactionMatch(t.Context(), domain.ProviderTransactionMatch{
				ID:                    "match-" + fake.UUID().V4(),
				ConnectionID:          connection.ConnectionID,
				ProviderAccountID:     providerAccountID,
				ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
				Fingerprint:           "fingerprint-pending-" + fake.UUID().V4(),
				TransactionID:         existingTransaction.ID,
				Status:                existingTransaction.Status,
				CreatedAt:             now.Add(-48 * time.Hour),
				UpdatedAt:             now.Add(-48 * time.Hour),
			})
			require.NoError(t, err)

			observation := domain.ProviderTransactionObservation{
				Connection:            connection,
				ProviderAccountID:     providerAccountID,
				ProviderTransactionID: existingMatch.ProviderTransactionID,
				Status:                domain.TransactionStatusBooked,
				AmountMinor:           existingTransaction.AmountMinor,
				Currency:              existingTransaction.Currency,
				Description:           "booked-" + fake.Lorem().Word(),
				EffectiveAt:           window.Start.Add(time.Hour),
				Fingerprint:           "fingerprint-booked-" + fake.UUID().V4(),
			}
			mergedTransaction := existingTransaction
			mergedTransaction.Status = observation.Status
			mergedTransaction.Description = observation.Description
			mergedTransaction.EffectiveAt = observation.EffectiveAt
			mergedTransaction.UpdatedAt = now
			action := providers.ProviderTransactionAction{
				Type:                providers.ProviderTransactionActionTypeUpdate,
				MatchStrategy:       providers.ProviderTransactionMatchStrategyProviderID,
				Observation:         observation,
				ExistingTransaction: &existingTransaction,
			}
			syncStore, err := providers.NewProviderWindowSyncStore(
				adapter,
				providers.WithWindowSyncStoreNow(func() time.Time { return now }),
			)
			require.NoError(t, err)

			stats, err := syncStore.ApplySync(t.Context(), providers.ProviderDiffPlan{
				Connection:         connection,
				SnapshotWindow:     window,
				TransactionActions: []providers.ProviderTransactionAction{action},
			}, providers.ApplyPlan{
				TransactionWrites: []providers.ApplyTransactionWrite{{
					Action:            action,
					MergedTransaction: &mergedTransaction,
				}},
				Stats: domain.ProviderSyncStats{UpdatedTransactions: 1},
			})
			require.NoError(t, err)
			assert.Equal(t, domain.ProviderSyncStats{UpdatedTransactions: 1}, stats)

			loadedTransaction, err := store.GetTransaction(t.Context(), existingTransaction.ID)
			require.NoError(t, err)
			require.NotNil(t, loadedTransaction)
			assert.True(t, mergedTransaction.EffectiveAt.Equal(loadedTransaction.EffectiveAt))
			assert.True(t, mergedTransaction.CreatedAt.Equal(loadedTransaction.CreatedAt))
			assert.True(t, mergedTransaction.UpdatedAt.Equal(loadedTransaction.UpdatedAt))
			expectedTransaction := mergedTransaction
			expectedTransaction.EffectiveAt = loadedTransaction.EffectiveAt
			expectedTransaction.CreatedAt = loadedTransaction.CreatedAt
			expectedTransaction.UpdatedAt = loadedTransaction.UpdatedAt
			assert.Equal(t, expectedTransaction, *loadedTransaction)
			matches, err := adapter.ListProviderTransactionMatchesByTransactionIDs(
				t.Context(),
				connection.ConnectionID,
				[]string{existingTransaction.ID},
			)
			require.NoError(t, err)
			require.Len(t, matches, 1)
			assert.Equal(t, existingMatch.ID, matches[0].ID)
			assert.Equal(t, existingTransaction.ID, matches[0].TransactionID)
			assert.Equal(t, observation.ProviderTransactionID, matches[0].ProviderTransactionID)
		},
	)

	t.Run("commits on success and rolls back on callback error", func(t *testing.T) {
		makeFixture := func(fake faker.Faker, connectionID string) (
			domain.ConnectionProviderAccount,
			domain.BalanceSnapshot,
			domain.Transaction,
			domain.ProviderTransactionMatch,
		) {
			capturedAt := time.Date(2026, time.June, 24, 10, 0, 0, 0, time.UTC)
			account := makeProviderAccount(
				fake,
				connectionID,
				"finance-account-"+fake.UUID().V4(),
				capturedAt.Add(-2*time.Hour),
			)
			transaction := makeTransaction(
				fake,
				account.FinanceAccountID,
				domain.TransactionSourceProvider,
				capturedAt.Add(-time.Hour),
			)
			snapshot := domain.BalanceSnapshot{
				ID:                  "snapshot-" + fake.UUID().V4(),
				ConnectionID:        connectionID,
				ProviderAccountID:   account.ProviderAccountID,
				FinanceAccountID:    account.FinanceAccountID,
				Currency:            "PLN",
				CurrentBalanceMinor: int64(fake.IntBetween(100, 500000)),
				CapturedAt:          capturedAt,
			}
			match := makeMatch(fake, connectionID, account.ProviderAccountID, transaction.ID, capturedAt)
			match.Status = transaction.Status
			return account, snapshot, transaction, match
		}

		t.Run("commits persisted writes on success", func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			adapter := NewProviderWindowSyncPersistence(store)
			connectionID := "connection-success-" + fake.UUID().V4()
			account, snapshot, transaction, match := makeFixture(fake, connectionID)
			attemptedAt := time.Now()
			state := domain.ProviderSyncState{
				Connection:  domain.ProviderConnectionRef{ConnectionID: connectionID},
				AttemptedAt: &attemptedAt,
				Window: domain.ProviderSyncWindow{
					Start: attemptedAt.Add(-time.Hour),
					End:   attemptedAt,
				},
				JobID: "job-" + fake.UUID().V4(),
			}

			err := adapter.WithTransaction(t.Context(), func(applyStore providers.WindowSyncApplyStore) error {
				_, err := applyStore.SaveConnectionProviderAccount(t.Context(), account)
				require.NoError(t, err)
				_, err = applyStore.SaveBalanceSnapshot(t.Context(), snapshot)
				require.NoError(t, err)
				_, err = applyStore.SaveTransaction(t.Context(), transaction)
				require.NoError(t, err)
				_, err = applyStore.SaveProviderTransactionMatch(t.Context(), match)
				require.NoError(t, err)
				require.NoError(t, applyStore.AppendSyncState(t.Context(), state))
				return nil
			})
			require.NoError(t, err)

			accounts, err := adapter.ListConnectionProviderAccounts(t.Context(), connectionID)
			require.NoError(t, err)
			require.Len(t, accounts, 1)
			assert.Equal(t, account.ID, accounts[0].ID)
			assert.Equal(t, account.ConnectionID, accounts[0].ConnectionID)
			assert.Equal(t, account.ProviderAccountID, accounts[0].ProviderAccountID)
			assert.Equal(t, account.FinanceAccountID, accounts[0].FinanceAccountID)
			assert.Equal(t, account.Name, accounts[0].Name)
			assert.Equal(t, account.Currency, accounts[0].Currency)
			assert.Equal(t, account.IBAN, accounts[0].IBAN)
			assert.Equal(t, account.MaskedPAN, accounts[0].MaskedPAN)
			assert.True(t, account.CreatedAt.Equal(accounts[0].CreatedAt))
			assert.True(t, account.UpdatedAt.Equal(accounts[0].UpdatedAt))
			if account.LastSuccessfulSyncAt == nil || accounts[0].LastSuccessfulSyncAt == nil {
				assert.Equal(t, account.LastSuccessfulSyncAt, accounts[0].LastSuccessfulSyncAt)
			} else {
				assert.True(t, account.LastSuccessfulSyncAt.Equal(*accounts[0].LastSuccessfulSyncAt))
			}

			snapshots, err := store.ListBalanceSnapshots(t.Context(), connectionID)
			require.NoError(t, err)
			require.Len(t, snapshots, 1)
			assert.Equal(t, snapshot.ID, snapshots[0].ID)
			assert.Equal(t, snapshot.ConnectionID, snapshots[0].ConnectionID)
			assert.Equal(t, snapshot.ProviderAccountID, snapshots[0].ProviderAccountID)
			assert.Equal(t, snapshot.FinanceAccountID, snapshots[0].FinanceAccountID)
			assert.Equal(t, snapshot.Currency, snapshots[0].Currency)
			assert.Equal(t, snapshot.CurrentBalanceMinor, snapshots[0].CurrentBalanceMinor)
			assert.Equal(t, snapshot.AvailableBalanceMinor, snapshots[0].AvailableBalanceMinor)
			assert.True(t, snapshot.CapturedAt.Equal(snapshots[0].CapturedAt))

			loadedTransaction, err := store.GetTransaction(t.Context(), transaction.ID)
			require.NoError(t, err)
			require.NotNil(t, loadedTransaction)
			assert.True(t, transaction.EffectiveAt.Equal(loadedTransaction.EffectiveAt))
			assert.True(t, transaction.CreatedAt.Equal(loadedTransaction.CreatedAt))
			assert.True(t, transaction.UpdatedAt.Equal(loadedTransaction.UpdatedAt))
			expectedTransaction := transaction
			expectedTransaction.EffectiveAt = loadedTransaction.EffectiveAt
			expectedTransaction.CreatedAt = loadedTransaction.CreatedAt
			expectedTransaction.UpdatedAt = loadedTransaction.UpdatedAt
			assert.Equal(t, expectedTransaction, *loadedTransaction)

			matches, err := adapter.ListProviderTransactionMatchesByTransactionIDs(
				t.Context(),
				connectionID,
				[]string{transaction.ID},
			)
			require.NoError(t, err)
			require.Len(t, matches, 1)
			assert.Equal(t, match.ID, matches[0].ID)
			assert.Equal(t, match.ConnectionID, matches[0].ConnectionID)
			assert.Equal(t, match.ProviderAccountID, matches[0].ProviderAccountID)
			assert.Equal(t, match.ProviderTransactionID, matches[0].ProviderTransactionID)
			assert.Equal(t, match.Fingerprint, matches[0].Fingerprint)
			assert.Equal(t, match.TransactionID, matches[0].TransactionID)
			assert.Equal(t, match.Status, matches[0].Status)
			assert.True(t, match.CreatedAt.Equal(matches[0].CreatedAt))
			assert.True(t, match.UpdatedAt.Equal(matches[0].UpdatedAt))
			journalState, err := NewProviderSyncStateJournalStore(store).LoadLastState(
				t.Context(),
				state.Connection,
			)
			require.NoError(t, err)
			require.NotNil(t, journalState)
			assert.Equal(t, state.JobID, journalState.JobID)
		})

		t.Run("rolls back persisted writes on callback error", func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			adapter := NewProviderWindowSyncPersistence(store)
			connectionID := "connection-rollback-" + fake.UUID().V4()
			account, snapshot, transaction, match := makeFixture(fake, connectionID)
			expectedErr := errors.New("callback-failed-" + fake.UUID().V4())
			attemptedAt := time.Now()
			state := domain.ProviderSyncState{
				Connection:  domain.ProviderConnectionRef{ConnectionID: connectionID},
				AttemptedAt: &attemptedAt,
				Window: domain.ProviderSyncWindow{
					Start: attemptedAt.Add(-time.Hour),
					End:   attemptedAt,
				},
				JobID: "job-" + fake.UUID().V4(),
			}

			err := adapter.WithTransaction(t.Context(), func(applyStore providers.WindowSyncApplyStore) error {
				_, err := applyStore.SaveConnectionProviderAccount(t.Context(), account)
				require.NoError(t, err)
				_, err = applyStore.SaveBalanceSnapshot(t.Context(), snapshot)
				require.NoError(t, err)
				_, err = applyStore.SaveTransaction(t.Context(), transaction)
				require.NoError(t, err)
				_, err = applyStore.SaveProviderTransactionMatch(t.Context(), match)
				require.NoError(t, err)
				require.NoError(t, applyStore.AppendSyncState(t.Context(), state))
				return expectedErr
			})
			require.ErrorIs(t, err, expectedErr)

			accounts, err := adapter.ListConnectionProviderAccounts(t.Context(), connectionID)
			require.NoError(t, err)
			assert.Empty(t, accounts)

			snapshots, err := store.ListBalanceSnapshots(t.Context(), connectionID)
			require.NoError(t, err)
			assert.Empty(t, snapshots)

			loadedTransaction, err := store.GetTransaction(t.Context(), transaction.ID)
			require.ErrorIs(t, err, ErrTransactionNotFound)
			assert.Nil(t, loadedTransaction)

			matches, err := adapter.ListProviderTransactionMatchesByTransactionIDs(
				t.Context(),
				connectionID,
				[]string{transaction.ID},
			)
			require.NoError(t, err)
			assert.Empty(t, matches)
			journalState, err := NewProviderSyncStateJournalStore(store).LoadLastState(
				t.Context(),
				state.Connection,
			)
			require.NoError(t, err)
			assert.Nil(t, journalState)
		})
	})
}
