package persistence

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
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
		assert.Equal(t, []domain.Transaction{insideLater, insideEarlier}, transactions)
	})

	t.Run("lists provider transaction matches scoped by connection and transaction ids", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		adapter := NewProviderWindowSyncPersistence(store)
		now := time.Now().UTC()
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
			makeMatch(fake, connectionID, providerAccountID, transactionIDTwo, now.Add(time.Second)),
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
		assert.Equal(t, []domain.ProviderTransactionMatch{firstMatch, secondMatch}, matches)
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
	})

	t.Run("commits on success and rolls back on callback error", func(t *testing.T) {
		makeFixture := func(fake faker.Faker, connectionID string) (
			domain.ConnectionProviderAccount,
			domain.BalanceSnapshot,
			domain.RawPayload,
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
			rawPayload := domain.RawPayload{
				ID:               "payload-" + fake.UUID().V4(),
				ConnectionID:     connectionID,
				Scope:            domain.RawPayloadScopeTransaction,
				ProviderObjectID: transaction.ID,
				PayloadJSON:      []byte(`{"id":"` + fake.UUID().V4() + `"}`),
				CapturedAt:       capturedAt,
			}
			match := makeMatch(fake, connectionID, account.ProviderAccountID, transaction.ID, capturedAt)
			match.Status = transaction.Status
			return account, snapshot, rawPayload, transaction, match
		}

		t.Run("commits persisted writes on success", func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			adapter := NewProviderWindowSyncPersistence(store)
			connectionID := "connection-success-" + fake.UUID().V4()
			account, snapshot, rawPayload, transaction, match := makeFixture(fake, connectionID)

			err := adapter.WithTransaction(t.Context(), func(applyStore providers.WindowSyncApplyStore) error {
				_, err := applyStore.SaveConnectionProviderAccount(t.Context(), account)
				require.NoError(t, err)
				_, err = applyStore.SaveBalanceSnapshot(t.Context(), snapshot)
				require.NoError(t, err)
				_, err = applyStore.SaveRawPayload(t.Context(), rawPayload)
				require.NoError(t, err)
				_, err = applyStore.SaveTransaction(t.Context(), transaction)
				require.NoError(t, err)
				_, err = applyStore.SaveProviderTransactionMatch(t.Context(), match)
				require.NoError(t, err)
				return nil
			})
			require.NoError(t, err)

			accounts, err := adapter.ListConnectionProviderAccounts(t.Context(), connectionID)
			require.NoError(t, err)
			assert.Equal(t, []domain.ConnectionProviderAccount{account}, accounts)

			snapshots, err := store.ListBalanceSnapshots(t.Context(), connectionID)
			require.NoError(t, err)
			assert.Equal(t, []domain.BalanceSnapshot{snapshot}, snapshots)

			payloads, err := store.ListRawPayloads(t.Context(), connectionID)
			require.NoError(t, err)
			assert.Equal(t, []domain.RawPayload{rawPayload}, payloads)

			loadedTransaction, err := store.GetTransaction(t.Context(), transaction.ID)
			require.NoError(t, err)
			require.NotNil(t, loadedTransaction)
			assert.Equal(t, transaction, *loadedTransaction)

			matches, err := adapter.ListProviderTransactionMatchesByTransactionIDs(
				t.Context(),
				connectionID,
				[]string{transaction.ID},
			)
			require.NoError(t, err)
			assert.Equal(t, []domain.ProviderTransactionMatch{match}, matches)
		})

		t.Run("rolls back persisted writes on callback error", func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			adapter := NewProviderWindowSyncPersistence(store)
			connectionID := "connection-rollback-" + fake.UUID().V4()
			account, snapshot, rawPayload, transaction, match := makeFixture(fake, connectionID)
			expectedErr := errors.New("callback-failed-" + fake.UUID().V4())

			err := adapter.WithTransaction(t.Context(), func(applyStore providers.WindowSyncApplyStore) error {
				_, err := applyStore.SaveConnectionProviderAccount(t.Context(), account)
				require.NoError(t, err)
				_, err = applyStore.SaveBalanceSnapshot(t.Context(), snapshot)
				require.NoError(t, err)
				_, err = applyStore.SaveRawPayload(t.Context(), rawPayload)
				require.NoError(t, err)
				_, err = applyStore.SaveTransaction(t.Context(), transaction)
				require.NoError(t, err)
				_, err = applyStore.SaveProviderTransactionMatch(t.Context(), match)
				require.NoError(t, err)
				return expectedErr
			})
			require.ErrorIs(t, err, expectedErr)

			accounts, err := adapter.ListConnectionProviderAccounts(t.Context(), connectionID)
			require.NoError(t, err)
			assert.Empty(t, accounts)

			snapshots, err := store.ListBalanceSnapshots(t.Context(), connectionID)
			require.NoError(t, err)
			assert.Empty(t, snapshots)

			payloads, err := store.ListRawPayloads(t.Context(), connectionID)
			require.NoError(t, err)
			assert.Empty(t, payloads)

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
		})
	})
}
