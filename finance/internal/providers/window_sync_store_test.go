package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProviderWindowSyncStore(t *testing.T) {
	type persistenceFixture struct {
		accounts        []domain.ConnectionProviderAccount
		financeAccounts map[string]domain.Account
		transactions    []domain.Transaction
		matches         []domain.ProviderTransactionMatch

		listAccountsErr       error
		listTransactionsErr   error
		listMatchesErr        error
		saveBalanceErr        error
		getFinanceAccountErr  error
		saveFinanceAccountErr error
		saveTransactionErr    error
		withTransactionErr    error

		listAccountsConnectionIDs []string
		listTransactionAccountIDs [][]string
		listTransactionWindows    []domain.ProviderSyncWindow
		listMatchConnectionIDs    []string
		listMatchTransactionIDs   [][]string

		savedAccounts        []domain.ConnectionProviderAccount
		savedFinanceAccounts []domain.Account
		savedSnapshots       []domain.BalanceSnapshot
		savedRawPayloads     []domain.RawPayload
		savedTransactions    []domain.Transaction
		savedMatches         []domain.ProviderTransactionMatch

		operationOrder []string

		withTransactionCalls int
	}

	makePersistence := func(t *testing.T, fixture *persistenceFixture) *MockWindowSyncPersistence {
		t.Helper()
		persistence := NewMockWindowSyncPersistence(t)
		applyStore := NewMockWindowSyncApplyStore(t)

		persistence.EXPECT().
			ListConnectionProviderAccounts(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, connectionID string) ([]domain.ConnectionProviderAccount, error) {
				fixture.operationOrder = append(fixture.operationOrder, "listAccounts")
				fixture.listAccountsConnectionIDs = append(fixture.listAccountsConnectionIDs, connectionID)
				if fixture.listAccountsErr != nil {
					return nil, fixture.listAccountsErr
				}
				return append([]domain.ConnectionProviderAccount(nil), fixture.accounts...), nil
			}).Maybe()

		persistence.EXPECT().
			ListProviderTransactionsInWindow(mock.Anything, mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				financeAccountIDs []string,
				window domain.ProviderSyncWindow,
			) ([]domain.Transaction, error) {
				fixture.operationOrder = append(fixture.operationOrder, "listTransactions")
				fixture.listTransactionAccountIDs = append(
					fixture.listTransactionAccountIDs,
					append([]string(nil), financeAccountIDs...),
				)
				fixture.listTransactionWindows = append(fixture.listTransactionWindows, window)
				if fixture.listTransactionsErr != nil {
					return nil, fixture.listTransactionsErr
				}
				return append([]domain.Transaction(nil), fixture.transactions...), nil
			}).Maybe()

		persistence.EXPECT().
			ListProviderTransactionMatchesByTransactionIDs(mock.Anything, mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				connectionID string,
				transactionIDs []string,
			) ([]domain.ProviderTransactionMatch, error) {
				fixture.operationOrder = append(fixture.operationOrder, "listMatches")
				fixture.listMatchConnectionIDs = append(fixture.listMatchConnectionIDs, connectionID)
				fixture.listMatchTransactionIDs = append(
					fixture.listMatchTransactionIDs,
					append([]string(nil), transactionIDs...),
				)
				if fixture.listMatchesErr != nil {
					return nil, fixture.listMatchesErr
				}
				return append([]domain.ProviderTransactionMatch(nil), fixture.matches...), nil
			}).Maybe()

		applyStore.EXPECT().
			SaveConnectionProviderAccount(mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				account domain.ConnectionProviderAccount,
			) (domain.ConnectionProviderAccount, error) {
				fixture.operationOrder = append(fixture.operationOrder, "saveAccount")
				fixture.savedAccounts = append(fixture.savedAccounts, account)
				return account, nil
			}).Maybe()

		applyStore.EXPECT().
			GetAccount(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, accountID string) (*domain.Account, error) {
				fixture.operationOrder = append(fixture.operationOrder, "getFinanceAccount")
				if fixture.getFinanceAccountErr != nil {
					return nil, fixture.getFinanceAccountErr
				}
				if fixture.financeAccounts == nil {
					return &domain.Account{ID: accountID}, nil
				}
				account, ok := fixture.financeAccounts[accountID]
				if !ok {
					return &domain.Account{ID: accountID}, nil
				}
				copyAccount := account
				return &copyAccount, nil
			}).Maybe()

		applyStore.EXPECT().
			SaveAccount(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, account domain.Account) (domain.Account, error) {
				fixture.operationOrder = append(fixture.operationOrder, "saveFinanceAccount")
				if fixture.saveFinanceAccountErr != nil {
					return domain.Account{}, fixture.saveFinanceAccountErr
				}
				fixture.savedFinanceAccounts = append(fixture.savedFinanceAccounts, account)
				return account, nil
			}).Maybe()

		applyStore.EXPECT().
			SaveBalanceSnapshot(mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				snapshot domain.BalanceSnapshot,
			) (domain.BalanceSnapshot, error) {
				fixture.operationOrder = append(fixture.operationOrder, "saveBalance")
				if fixture.saveBalanceErr != nil {
					return domain.BalanceSnapshot{}, fixture.saveBalanceErr
				}
				fixture.savedSnapshots = append(fixture.savedSnapshots, snapshot)
				return snapshot, nil
			}).Maybe()

		applyStore.EXPECT().
			SaveRawPayload(mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				payload domain.RawPayload,
			) (domain.RawPayload, error) {
				fixture.operationOrder = append(fixture.operationOrder, "saveRawPayload")
				fixture.savedRawPayloads = append(fixture.savedRawPayloads, payload)
				return payload, nil
			}).Maybe()

		applyStore.EXPECT().
			SaveTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				transaction domain.Transaction,
			) (domain.Transaction, error) {
				fixture.operationOrder = append(fixture.operationOrder, "saveTransaction")
				if fixture.saveTransactionErr != nil {
					return domain.Transaction{}, fixture.saveTransactionErr
				}
				fixture.savedTransactions = append(fixture.savedTransactions, transaction)
				return transaction, nil
			}).Maybe()

		applyStore.EXPECT().
			SaveProviderTransactionMatch(mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				match domain.ProviderTransactionMatch,
			) (domain.ProviderTransactionMatch, error) {
				fixture.operationOrder = append(fixture.operationOrder, "saveMatch")
				fixture.savedMatches = append(fixture.savedMatches, match)
				return match, nil
			}).Maybe()

		persistence.EXPECT().
			WithTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, fn func(WindowSyncApplyStore) error) error {
				fixture.withTransactionCalls++
				if fixture.withTransactionErr != nil {
					return fixture.withTransactionErr
				}
				return fn(applyStore)
			}).Maybe()

		return persistence
	}

	makeProviderAccount := func(fake faker.Faker, connectionID string) domain.ConnectionProviderAccount {
		createdAt := time.Date(2026, time.June, 20, 9, 0, 0, 0, time.UTC)
		return domain.ConnectionProviderAccount{
			ID:                "provider-account-row-" + fake.UUID().V4(),
			ConnectionID:      connectionID,
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			FinanceAccountID:  "finance-account-" + fake.UUID().V4(),
			Name:              "account-" + fake.Lorem().Word(),
			Currency:          "PLN",
			IBAN:              "PL61109010140000071219812874",
			MaskedPAN:         "4444",
			CreatedAt:         createdAt,
			UpdatedAt:         createdAt.Add(time.Minute),
		}
	}

	makeTransaction := func(fake faker.Faker, tenantID string, accountID string, effectiveAt time.Time) domain.Transaction {
		return domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    tenantID,
			AccountID:   accountID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: int64(-fake.IntBetween(100, 90000)),
			Currency:    "PLN",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: effectiveAt,
			CreatedAt:   effectiveAt.Add(-time.Hour),
			UpdatedAt:   effectiveAt.Add(-30 * time.Minute),
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

	t.Run("loads existing window by composing provider-owned snapshot queries", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		window := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC),
		}
		firstAccount := makeProviderAccount(fake, connection.ConnectionID)
		secondAccount := makeProviderAccount(fake, connection.ConnectionID)
		firstTransaction := makeTransaction(
			fake,
			"tenant-"+fake.UUID().V4(),
			firstAccount.FinanceAccountID,
			window.Start.Add(24*time.Hour),
		)
		secondTransaction := makeTransaction(
			fake,
			"tenant-"+fake.UUID().V4(),
			secondAccount.FinanceAccountID,
			window.Start.Add(12*time.Hour),
		)
		firstMatch := makeMatch(
			fake,
			connection.ConnectionID,
			firstAccount.ProviderAccountID,
			firstTransaction.ID,
			window.Start.Add(30*time.Hour),
		)
		secondMatch := makeMatch(
			fake,
			connection.ConnectionID,
			secondAccount.ProviderAccountID,
			secondTransaction.ID,
			window.Start.Add(31*time.Hour),
		)

		fixture := &persistenceFixture{
			accounts:     []domain.ConnectionProviderAccount{firstAccount, secondAccount},
			transactions: []domain.Transaction{firstTransaction, secondTransaction},
			matches:      []domain.ProviderTransactionMatch{firstMatch, secondMatch},
		}
		persistence := makePersistence(t, fixture)
		store, err := NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)

		snapshot, err := store.LoadExistingWindow(t.Context(), connection, window)
		require.NoError(t, err)

		assert.Equal(t, ExistingWindowSnapshot{
			Connection:     connection,
			SnapshotWindow: window,
			Accounts:       []domain.ConnectionProviderAccount{firstAccount, secondAccount},
			Transactions:   []domain.Transaction{firstTransaction, secondTransaction},
			Matches:        []domain.ProviderTransactionMatch{firstMatch, secondMatch},
		}, snapshot)
		assert.Equal(t, []string{"listAccounts", "listTransactions", "listMatches"}, fixture.operationOrder)
		assert.Equal(t, []string{connection.ConnectionID}, fixture.listAccountsConnectionIDs)
		assert.Equal(
			t,
			[][]string{{firstAccount.FinanceAccountID, secondAccount.FinanceAccountID}},
			fixture.listTransactionAccountIDs,
		)
		assert.Equal(t, []domain.ProviderSyncWindow{window}, fixture.listTransactionWindows)
		assert.Equal(t, []string{connection.ConnectionID}, fixture.listMatchConnectionIDs)
		assert.Equal(
			t,
			[][]string{{firstTransaction.ID, secondTransaction.ID}},
			fixture.listMatchTransactionIDs,
		)
	})

	t.Run("returns constructor and snapshot read errors", func(t *testing.T) {
		store, err := NewProviderWindowSyncStore(nil)
		require.ErrorIs(t, err, ErrWindowSyncPersistenceRequired)
		assert.Nil(t, store)

		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		window := makeRandomProviderSyncWindow(fake)
		expectedErr := errors.New("list-accounts-" + fake.UUID().V4())
		fixture := &persistenceFixture{listAccountsErr: expectedErr}
		persistence := makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)

		_, err = store.LoadExistingWindow(t.Context(), connection, window)
		require.ErrorIs(t, err, expectedErr)
		assert.Equal(t, []string{"listAccounts"}, fixture.operationOrder)

		fixture = &persistenceFixture{
			accounts: []domain.ConnectionProviderAccount{{
				FinanceAccountID: "finance-account-" + fake.UUID().V4(),
			}},
			listTransactionsErr: errors.New("list-transactions-" + fake.UUID().V4()),
		}
		persistence = makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)
		_, err = store.LoadExistingWindow(t.Context(), connection, window)
		require.ErrorContains(t, err, "list provider transactions in window")

		fixture = &persistenceFixture{
			accounts: []domain.ConnectionProviderAccount{{
				FinanceAccountID: "finance-account-" + fake.UUID().V4(),
			}},
			transactions:   []domain.Transaction{{ID: "transaction-" + fake.UUID().V4()}},
			listMatchesErr: errors.New("list-matches-" + fake.UUID().V4()),
		}
		persistence = makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)
		_, err = store.LoadExistingWindow(t.Context(), connection, window)
		require.ErrorContains(t, err, "list provider transaction matches by transaction ids")
	})

	t.Run("applies sync inside one transaction using canonical save primitives", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		now := time.Date(2026, time.June, 28, 11, 0, 0, 0, time.UTC)
		window := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, time.June, 29, 0, 0, 0, 0, time.UTC),
		}
		existingAccount := makeProviderAccount(fake, connection.ConnectionID)
		existingTransaction := makeTransaction(
			fake,
			"tenant-"+fake.UUID().V4(),
			existingAccount.FinanceAccountID,
			now.Add(-48*time.Hour),
		)
		existingFinanceAccount := domain.Account{
			ID:       existingAccount.FinanceAccountID,
			TenantID: existingTransaction.TenantID,
			Name:     existingAccount.Name,
			Currency: existingAccount.Currency,
			Kind:     domain.AccountKindLinked,
			LinkedAccount: &domain.LinkedAccount{
				Provider:          string(connection.ProviderID),
				ProviderAccountID: existingAccount.ProviderAccountID,
			},
			CreatedAt: now.Add(-72 * time.Hour),
			UpdatedAt: now.Add(-36 * time.Hour),
		}
		existingMatch := makeMatch(
			fake,
			connection.ConnectionID,
			existingAccount.ProviderAccountID,
			existingTransaction.ID,
			now.Add(-36*time.Hour),
		)

		accountObservation := domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: existingAccount.ProviderAccountID,
			Name:              "renamed-" + fake.Lorem().Word(),
			Currency:          existingAccount.Currency,
			IBAN:              "PL27114020040000300201355387",
			MaskedPAN:         "1234",
		}
		balanceObservation := domain.ProviderBalanceObservation{
			Connection:            connection,
			ProviderAccountID:     existingAccount.ProviderAccountID,
			Currency:              existingAccount.Currency,
			CurrentBalanceMinor:   int64(fake.IntBetween(100, 90000)),
			AvailableBalanceMinor: func() *int64 { value := int64(fake.IntBetween(100, 90000)); return &value }(),
			CapturedAt:            now.Add(-time.Hour),
		}
		rawPayloadObservation := domain.ProviderRawPayloadObservation{
			Connection:       connection,
			Scope:            domain.RawPayloadScopeTransaction,
			ProviderObjectID: "provider-object-" + fake.UUID().V4(),
			PayloadJSON:      []byte(`{"id":"` + fake.UUID().V4() + `"}`),
			CapturedAt:       now.Add(-30 * time.Minute),
		}
		createObservation := domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     existingAccount.ProviderAccountID,
			ProviderTransactionID: "provider-transaction-create-" + fake.UUID().V4(),
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           int64(-fake.IntBetween(100, 90000)),
			Currency:              "PLN",
			Description:           "created-" + fake.Lorem().Word(),
			EffectiveAt:           now.Add(-2 * time.Hour),
			Fingerprint:           "fingerprint-create-" + fake.UUID().V4(),
		}
		updateObservation := domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     existingAccount.ProviderAccountID,
			ProviderTransactionID: existingMatch.ProviderTransactionID,
			Status:                domain.TransactionStatusPending,
			AmountMinor:           existingTransaction.AmountMinor,
			Currency:              existingTransaction.Currency,
			Description:           "updated-" + fake.Lorem().Word(),
			EffectiveAt:           existingTransaction.EffectiveAt.Add(time.Hour),
			Fingerprint:           existingMatch.Fingerprint,
		}
		mergedTransaction := existingTransaction
		mergedTransaction.Status = updateObservation.Status
		mergedTransaction.Description = updateObservation.Description
		mergedTransaction.EffectiveAt = updateObservation.EffectiveAt
		mergedTransaction.ProviderOriginal = buildProviderOriginal(updateObservation)

		createAction := ProviderTransactionAction{
			Type:          ProviderTransactionActionTypeCreate,
			MatchStrategy: ProviderTransactionMatchStrategyNew,
			Observation:   createObservation,
		}
		updateAction := ProviderTransactionAction{
			Type:                ProviderTransactionActionTypeUpdate,
			MatchStrategy:       ProviderTransactionMatchStrategyProviderID,
			Observation:         updateObservation,
			ExistingTransaction: &existingTransaction,
		}

		fixture := &persistenceFixture{
			accounts: []domain.ConnectionProviderAccount{existingAccount},
			financeAccounts: map[string]domain.Account{
				existingFinanceAccount.ID: existingFinanceAccount,
			},
			transactions: []domain.Transaction{existingTransaction},
			matches:      []domain.ProviderTransactionMatch{existingMatch},
		}
		persistence := makePersistence(t, fixture)
		generatedIDs := []string{"balance-id", "raw-id", "created-transaction-id", "created-match-id"}
		store, err := NewProviderWindowSyncStore(
			persistence,
			WithWindowSyncStoreNow(func() time.Time { return now }),
			WithWindowSyncStoreIDGenerator(func() string {
				id := generatedIDs[0]
				generatedIDs = generatedIDs[1:]
				return id
			}),
		)
		require.NoError(t, err)

		err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:             connection,
			SnapshotWindow:         window,
			AccountObservations:    []domain.ProviderAccountObservation{accountObservation},
			BalanceObservations:    []domain.ProviderBalanceObservation{balanceObservation},
			TransactionActions:     []ProviderTransactionAction{createAction, updateAction},
			RawPayloadObservations: []domain.ProviderRawPayloadObservation{rawPayloadObservation},
		}, ApplyPlan{
			TransactionWrites: []ApplyTransactionWrite{
				{Action: createAction},
				{Action: updateAction, MergedTransaction: &mergedTransaction},
			},
		})
		require.NoError(t, err)

		require.Equal(t, 1, fixture.withTransactionCalls)
		assert.Equal(
			t,
			[]string{
				"listAccounts",
				"listTransactions",
				"listMatches",
				"saveAccount",
				"getFinanceAccount",
				"saveFinanceAccount",
				"saveBalance",
				"saveRawPayload",
				"saveTransaction",
				"saveMatch",
				"saveTransaction",
				"saveMatch",
			},
			fixture.operationOrder,
		)

		require.Len(t, fixture.savedAccounts, 1)
		assert.Equal(t, domain.ConnectionProviderAccount{
			ID:                   existingAccount.ID,
			ConnectionID:         connection.ConnectionID,
			ProviderAccountID:    existingAccount.ProviderAccountID,
			FinanceAccountID:     existingAccount.FinanceAccountID,
			Name:                 accountObservation.Name,
			Currency:             accountObservation.Currency,
			IBAN:                 accountObservation.IBAN,
			MaskedPAN:            accountObservation.MaskedPAN,
			LastSuccessfulSyncAt: timePointerOrNil(now),
			CreatedAt:            existingAccount.CreatedAt,
			UpdatedAt:            now,
		}, fixture.savedAccounts[0])
		require.Len(t, fixture.savedFinanceAccounts, 1)
		expectedFinanceAccount := existingFinanceAccount
		expectedFinanceAccount.Name = accountObservation.Name
		expectedFinanceAccount.Currency = accountObservation.Currency
		expectedFinanceAccount.UpdatedAt = now
		assert.Equal(t, expectedFinanceAccount, fixture.savedFinanceAccounts[0])

		require.Len(t, fixture.savedSnapshots, 1)
		assert.Equal(t, domain.BalanceSnapshot{
			ID:                    "balance-id",
			ConnectionID:          connection.ConnectionID,
			ProviderAccountID:     existingAccount.ProviderAccountID,
			FinanceAccountID:      existingAccount.FinanceAccountID,
			Currency:              balanceObservation.Currency,
			CurrentBalanceMinor:   balanceObservation.CurrentBalanceMinor,
			AvailableBalanceMinor: balanceObservation.AvailableBalanceMinor,
			CapturedAt:            balanceObservation.CapturedAt,
		}, fixture.savedSnapshots[0])

		require.Len(t, fixture.savedRawPayloads, 1)
		assert.Equal(t, domain.RawPayload{
			ID:               "raw-id",
			ConnectionID:     connection.ConnectionID,
			Scope:            rawPayloadObservation.Scope,
			ProviderObjectID: rawPayloadObservation.ProviderObjectID,
			PayloadJSON:      rawPayloadObservation.PayloadJSON,
			CapturedAt:       rawPayloadObservation.CapturedAt,
		}, fixture.savedRawPayloads[0])

		require.Len(t, fixture.savedTransactions, 2)
		assert.Equal(t, domain.Transaction{
			ID:               "created-transaction-id",
			TenantID:         existingTransaction.TenantID,
			AccountID:        existingAccount.FinanceAccountID,
			Source:           domain.TransactionSourceProvider,
			Status:           createObservation.Status,
			Kind:             domain.TransactionKindRegular,
			AmountMinor:      createObservation.AmountMinor,
			Currency:         createObservation.Currency,
			Description:      createObservation.Description,
			EffectiveAt:      createObservation.EffectiveAt,
			CreatedAt:        now,
			UpdatedAt:        now,
			ProviderOriginal: buildProviderOriginal(createObservation),
		}, fixture.savedTransactions[0])
		assert.Equal(t, domain.Transaction{
			ID:               mergedTransaction.ID,
			TenantID:         mergedTransaction.TenantID,
			AccountID:        mergedTransaction.AccountID,
			Source:           mergedTransaction.Source,
			Status:           mergedTransaction.Status,
			Kind:             mergedTransaction.Kind,
			AmountMinor:      mergedTransaction.AmountMinor,
			Currency:         mergedTransaction.Currency,
			Description:      mergedTransaction.Description,
			EffectiveAt:      mergedTransaction.EffectiveAt,
			CreatedAt:        mergedTransaction.CreatedAt,
			UpdatedAt:        now,
			ProviderOriginal: mergedTransaction.ProviderOriginal,
		}, fixture.savedTransactions[1])

		require.Len(t, fixture.savedMatches, 2)
		assert.Equal(t, domain.ProviderTransactionMatch{
			ID:                    "created-match-id",
			ConnectionID:          connection.ConnectionID,
			ProviderAccountID:     createObservation.ProviderAccountID,
			ProviderTransactionID: createObservation.ProviderTransactionID,
			Fingerprint:           createObservation.Fingerprint,
			TransactionID:         "created-transaction-id",
			Status:                createObservation.Status,
			CreatedAt:             now,
			UpdatedAt:             now,
		}, fixture.savedMatches[0])
		assert.Equal(t, domain.ProviderTransactionMatch{
			ID:                    existingMatch.ID,
			ConnectionID:          connection.ConnectionID,
			ProviderAccountID:     updateObservation.ProviderAccountID,
			ProviderTransactionID: updateObservation.ProviderTransactionID,
			Fingerprint:           updateObservation.Fingerprint,
			TransactionID:         existingTransaction.ID,
			Status:                mergedTransaction.Status,
			CreatedAt:             existingMatch.CreatedAt,
			UpdatedAt:             now,
		}, fixture.savedMatches[1])
	})

	t.Run("preserves custom linked finance account name during provider metadata refresh", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		now := time.Date(2026, time.July, 7, 12, 0, 0, 0, time.UTC)
		window := makeRandomProviderSyncWindow(fake)
		existingAccount := makeProviderAccount(fake, connection.ConnectionID)
		financeAccount := domain.Account{
			ID:       existingAccount.FinanceAccountID,
			TenantID: "tenant-" + fake.UUID().V4(),
			Name:     "custom-" + fake.Lorem().Word(),
			Currency: existingAccount.Currency,
			Kind:     domain.AccountKindLinked,
			LinkedAccount: &domain.LinkedAccount{
				Provider:          string(connection.ProviderID),
				ProviderAccountID: existingAccount.ProviderAccountID,
			},
			CreatedAt: now.Add(-48 * time.Hour),
			UpdatedAt: now.Add(-24 * time.Hour),
		}
		fixture := &persistenceFixture{
			accounts: []domain.ConnectionProviderAccount{existingAccount},
			financeAccounts: map[string]domain.Account{
				financeAccount.ID: financeAccount,
			},
		}
		persistence := makePersistence(t, fixture)
		store, err := NewProviderWindowSyncStore(
			persistence,
			WithWindowSyncStoreNow(func() time.Time { return now }),
		)
		require.NoError(t, err)

		err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: window,
			AccountObservations: []domain.ProviderAccountObservation{{
				Connection:        connection,
				ProviderAccountID: existingAccount.ProviderAccountID,
				Name:              "provider-" + fake.Lorem().Word(),
				Currency:          existingAccount.Currency,
			}},
		}, ApplyPlan{})
		require.NoError(t, err)

		require.Len(t, fixture.savedAccounts, 1)
		assert.Empty(t, fixture.savedFinanceAccounts)
	})

	t.Run("returns apply errors when mappings or canonical writes fail", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		window := makeRandomProviderSyncWindow(fake)
		observedAccount := domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			Name:              "account-" + fake.Lorem().Word(),
			Currency:          "PLN",
		}

		fixture := &persistenceFixture{}
		persistence := makePersistence(t, fixture)
		store, err := NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)

		err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:          connection,
			SnapshotWindow:      window,
			AccountObservations: []domain.ProviderAccountObservation{observedAccount},
		}, ApplyPlan{})
		require.ErrorContains(t, err, "provider account mapping not found")
		assert.Equal(t, []string{"listAccounts", "listTransactions", "listMatches"}, fixture.operationOrder)

		fixture = &persistenceFixture{listAccountsErr: errors.New("apply-load-snapshot-" + fake.UUID().V4())}
		persistence = makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)
		err = store.ApplySync(
			t.Context(),
			ProviderDiffPlan{Connection: connection, SnapshotWindow: window},
			ApplyPlan{},
		)
		require.ErrorContains(t, err, "load existing apply snapshot")

		existingAccount := domain.ConnectionProviderAccount{
			ID:                "provider-account-row-" + fake.UUID().V4(),
			ConnectionID:      connection.ConnectionID,
			ProviderAccountID: observedAccount.ProviderAccountID,
			FinanceAccountID:  "finance-account-" + fake.UUID().V4(),
			CreatedAt:         time.Now().UTC(),
		}
		existingFinanceAccount := domain.Account{
			ID:       existingAccount.FinanceAccountID,
			TenantID: "tenant-" + fake.UUID().V4(),
			Name:     existingAccount.Name,
			Currency: existingAccount.Currency,
			Kind:     domain.AccountKindLinked,
			LinkedAccount: &domain.LinkedAccount{
				Provider:          string(connection.ProviderID),
				ProviderAccountID: existingAccount.ProviderAccountID,
			},
			CreatedAt: time.Now().UTC().Add(-time.Hour),
			UpdatedAt: time.Now().UTC().Add(-time.Minute),
		}
		observedLinkedAccount := observedAccount
		observedLinkedAccount.Name = "provider-name-" + fake.Lorem().Word()
		observedLinkedAccount.Currency = "EUR"

		expectedErr := errors.New("get-finance-account-" + fake.UUID().V4())
		fixture = &persistenceFixture{
			accounts:             []domain.ConnectionProviderAccount{existingAccount},
			getFinanceAccountErr: expectedErr,
		}
		persistence = makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)
		err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:          connection,
			SnapshotWindow:      window,
			AccountObservations: []domain.ProviderAccountObservation{observedLinkedAccount},
		}, ApplyPlan{})
		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "get linked finance account")

		expectedErr = errors.New("save-finance-account-" + fake.UUID().V4())
		fixture = &persistenceFixture{
			accounts: []domain.ConnectionProviderAccount{existingAccount},
			financeAccounts: map[string]domain.Account{
				existingFinanceAccount.ID: existingFinanceAccount,
			},
			saveFinanceAccountErr: expectedErr,
		}
		persistence = makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)
		err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:          connection,
			SnapshotWindow:      window,
			AccountObservations: []domain.ProviderAccountObservation{observedLinkedAccount},
		}, ApplyPlan{})
		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "save linked finance account")

		writeObservation := domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     existingAccount.ProviderAccountID,
			ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           int64(-fake.IntBetween(100, 90000)),
			Currency:              "PLN",
			Description:           "transaction-" + fake.Lorem().Word(),
			EffectiveAt:           time.Now().UTC(),
			Fingerprint:           "fingerprint-" + fake.UUID().V4(),
		}
		expectedErr = errors.New("save-transaction-" + fake.UUID().V4())
		fixture = &persistenceFixture{
			accounts: []domain.ConnectionProviderAccount{existingAccount},
			transactions: []domain.Transaction{{
				ID:        "transaction-" + fake.UUID().V4(),
				TenantID:  "tenant-" + fake.UUID().V4(),
				AccountID: existingAccount.FinanceAccountID,
			}},
			saveTransactionErr: expectedErr,
		}
		persistence = makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)

		createAction := ProviderTransactionAction{
			Type:        ProviderTransactionActionTypeCreate,
			Observation: writeObservation,
		}
		err = store.ApplySync(
			t.Context(),
			ProviderDiffPlan{
				Connection:         connection,
				SnapshotWindow:     window,
				TransactionActions: []ProviderTransactionAction{createAction},
			},
			ApplyPlan{TransactionWrites: []ApplyTransactionWrite{{Action: createAction}}},
		)
		require.ErrorIs(t, err, expectedErr)

		fixture = &persistenceFixture{
			accounts:       []domain.ConnectionProviderAccount{existingAccount},
			saveBalanceErr: errors.New("save-balance-" + fake.UUID().V4()),
		}
		persistence = makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)
		err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: window,
			BalanceObservations: []domain.ProviderBalanceObservation{{
				Connection:        connection,
				ProviderAccountID: existingAccount.ProviderAccountID,
				Currency:          "PLN",
				CapturedAt:        time.Now().UTC(),
			}},
		}, ApplyPlan{})
		require.ErrorContains(t, err, "save balance snapshot")
	})

	t.Run("covers helper branches for match derivation and id collection", func(t *testing.T) {
		fake := faker.New()
		effectiveAt := time.Now().UTC()
		transactions := []domain.Transaction{
			{ID: "txn-a", AccountID: "account-a", TenantID: "tenant-a"},
			{ID: "txn-a", AccountID: "account-a", TenantID: "tenant-a"},
			{AccountID: "account-b"},
		}
		accounts := []domain.ConnectionProviderAccount{
			{ProviderAccountID: "provider-a", FinanceAccountID: "account-a"},
			{ProviderAccountID: "provider-b", FinanceAccountID: "account-a"},
			{ProviderAccountID: "provider-c"},
		}
		assert.Equal(t, []string{"account-a"}, mappedFinanceAccountIDs(accounts))
		assert.Equal(t, []string{"txn-a"}, transactionIDs(transactions))

		_, err := tenantIDForFinanceAccount("missing-account", transactions)
		require.ErrorContains(t, err, "tenant id not found")
		_, err = resolveProviderAccount(providerAccountsByProviderID(accounts), "missing-provider")
		require.ErrorContains(t, err, "provider account mapping not found")
		assert.Nil(t, timePointerOrNil(time.Time{}))

		action := ProviderTransactionAction{
			MatchStrategy: ProviderTransactionMatchStrategyFingerprint,
			Observation: domain.ProviderTransactionObservation{
				ProviderAccountID: "provider-a",
				Fingerprint:       "fingerprint-a",
			},
			ExistingTransaction: &domain.Transaction{ID: "txn-a", EffectiveAt: effectiveAt},
		}
		match := existingSnapshotMatch(action, []domain.ProviderTransactionMatch{{
			ID:                "match-a",
			TransactionID:     "txn-a",
			ProviderAccountID: "provider-a",
			Fingerprint:       "fingerprint-a",
		}})
		require.NotNil(t, match)
		assert.Equal(t, "match-a", match.ID)

		unsupported := ProviderTransactionAction{
			MatchStrategy:       ProviderTransactionMatchStrategyAmbiguous,
			Observation:         action.Observation,
			ExistingTransaction: action.ExistingTransaction,
		}
		assert.Nil(t, existingSnapshotMatch(unsupported, []domain.ProviderTransactionMatch{{
			ID:                "match-b",
			TransactionID:     "txn-a",
			ProviderAccountID: "provider-a",
			Fingerprint:       "fingerprint-a",
		}}))

		persistence := makePersistence(t, &persistenceFixture{})
		store, err := NewProviderWindowSyncStore(
			persistence,
			WithWindowSyncStoreIDGenerator(func() string { return "generated-" + fake.UUID().V4() }),
			WithWindowSyncStoreNow(func() time.Time { return effectiveAt }),
		)
		require.NoError(t, err)
		matchIdentity, createdAt := store.providerTransactionMatchIdentity(nil, effectiveAt)
		assert.NotEmpty(t, matchIdentity)
		assert.Equal(t, effectiveAt, createdAt)
	})
}
