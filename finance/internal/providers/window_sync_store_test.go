package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProviderWindowSyncStore(t *testing.T) {
	type persistenceFixture struct {
		accounts        []domain.ConnectionProviderAccount
		bankConnection  *domain.BankConnection
		financeAccounts map[string]domain.Account
		transactions    []domain.Transaction
		matches         []domain.ProviderTransactionMatch
		identityMatches []ProviderTransactionIdentityMatch

		listAccountsErr         error
		listTransactionsErr     error
		listMatchesErr          error
		listIdentityMatchesErr  error
		saveBalanceErr          error
		saveProviderSnapshotErr error
		getFinanceAccountErr    error
		getBankConnectionErr    error
		missingBankConnection   bool
		missingFinanceAccount   bool
		saveFinanceAccountErr   error
		saveTransactionErr      error
		appendSyncStateErr      error
		withTransactionErr      error

		listAccountsConnectionIDs      []string
		listTransactionAccountIDs      [][]string
		listTransactionWindows         []domain.ProviderSyncWindow
		listMatchConnectionIDs         []string
		listMatchTransactionIDs        [][]string
		listIdentityMatchConnectionIDs []string
		listIdentityMatchIdentities    [][]ProviderTransactionIdentity

		savedAccounts          []domain.ConnectionProviderAccount
		savedFinanceAccounts   []domain.Account
		savedSnapshots         []domain.BalanceSnapshot
		savedProviderSnapshots []domain.ProviderSnapshot
		savedTransactions      []domain.Transaction
		savedMatches           []domain.ProviderTransactionMatch
		savedSyncStates        []domain.ProviderSyncState
		transactionalAccounts  map[string]domain.ConnectionProviderAccount

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

		persistence.EXPECT().
			ListProviderTransactionIdentityMatches(mock.Anything, mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				connectionID string,
				identities []ProviderTransactionIdentity,
			) ([]ProviderTransactionIdentityMatch, error) {
				fixture.operationOrder = append(fixture.operationOrder, "listIdentityMatches")
				fixture.listIdentityMatchConnectionIDs = append(
					fixture.listIdentityMatchConnectionIDs,
					connectionID,
				)
				fixture.listIdentityMatchIdentities = append(
					fixture.listIdentityMatchIdentities,
					append([]ProviderTransactionIdentity(nil), identities...),
				)
				if fixture.listIdentityMatchesErr != nil {
					return nil, fixture.listIdentityMatchesErr
				}
				return append([]ProviderTransactionIdentityMatch(nil), fixture.identityMatches...), nil
			}).Maybe()

		applyStore.EXPECT().
			AppendSyncState(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, state domain.ProviderSyncState) error {
				fixture.operationOrder = append(fixture.operationOrder, "appendSyncState")
				if fixture.appendSyncStateErr != nil {
					return fixture.appendSyncStateErr
				}
				fixture.savedSyncStates = append(fixture.savedSyncStates, state)
				return nil
			}).Maybe()

		applyStore.EXPECT().
			SaveConnectionProviderAccount(mock.Anything, mock.Anything).
			RunAndReturn(func(
				_ context.Context,
				account domain.ConnectionProviderAccount,
			) (domain.ConnectionProviderAccount, error) {
				fixture.operationOrder = append(fixture.operationOrder, "saveAccount")
				fixture.savedAccounts = append(fixture.savedAccounts, account)
				if fixture.transactionalAccounts != nil {
					existing, found := fixture.transactionalAccounts[account.ProviderAccountID]
					if found {
						existing.Name = account.Name
						existing.Currency = account.Currency
						existing.IBAN = account.IBAN
						existing.MaskedPAN = account.MaskedPAN
						existing.LastSuccessfulSyncAt = account.LastSuccessfulSyncAt
						existing.UpdatedAt = account.UpdatedAt
						fixture.transactionalAccounts[account.ProviderAccountID] = existing
						return existing, nil
					}
					fixture.transactionalAccounts[account.ProviderAccountID] = account
				}
				return account, nil
			}).Maybe()

		applyStore.EXPECT().
			GetBankConnection(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, connectionID string) (*domain.BankConnection, error) {
				fixture.operationOrder = append(fixture.operationOrder, "getBankConnection")
				if fixture.getBankConnectionErr != nil {
					return nil, fixture.getBankConnectionErr
				}
				if fixture.missingBankConnection {
					//nolint:nilnil // A missing connection is a valid persistence lookup result.
					return nil, nil
				}
				if fixture.bankConnection == nil {
					return &domain.BankConnection{
						ID:       connectionID,
						TenantID: "tenant-" + connectionID,
					}, nil
				}
				copyConnection := *fixture.bankConnection
				return &copyConnection, nil
			}).Maybe()

		applyStore.EXPECT().
			GetAccount(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, accountID string) (*domain.Account, error) {
				fixture.operationOrder = append(fixture.operationOrder, "getFinanceAccount")
				if fixture.getFinanceAccountErr != nil {
					return nil, fixture.getFinanceAccountErr
				}
				if fixture.missingFinanceAccount {
					//nolint:nilnil // A missing account is a valid persistence lookup result.
					return nil, nil
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
			SaveProviderSnapshot(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, snapshot domain.ProviderSnapshot) (domain.ProviderSnapshot, error) {
				fixture.operationOrder = append(fixture.operationOrder, "saveProviderSnapshot")
				if fixture.saveProviderSnapshotErr != nil {
					return domain.ProviderSnapshot{}, fixture.saveProviderSnapshotErr
				}
				fixture.savedProviderSnapshots = append(fixture.savedProviderSnapshots, snapshot)
				return snapshot, nil
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

		snapshot, err := store.LoadExistingWindow(t.Context(), connection, window, nil)
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

	t.Run("adds provider identity matches outside the window to the snapshot", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		window := makeRandomProviderSyncWindow(fake)
		account := makeProviderAccount(fake, connection.ConnectionID)
		outsideTransaction := makeTransaction(
			fake,
			"tenant-"+fake.UUID().V4(),
			account.FinanceAccountID,
			window.Start.Add(-time.Hour),
		)
		outsideMatch := makeMatch(
			fake,
			connection.ConnectionID,
			account.ProviderAccountID,
			outsideTransaction.ID,
			window.Start,
		)
		identity := ProviderTransactionIdentity{
			ProviderAccountID:     account.ProviderAccountID,
			ProviderTransactionID: outsideMatch.ProviderTransactionID,
		}
		fixture := &persistenceFixture{
			accounts: []domain.ConnectionProviderAccount{account},
			identityMatches: []ProviderTransactionIdentityMatch{{
				Transaction: outsideTransaction,
				Match:       outsideMatch,
			}},
		}
		persistence := makePersistence(t, fixture)
		store, err := NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)

		snapshot, err := store.LoadExistingWindow(
			t.Context(),
			connection,
			window,
			[]ProviderTransactionIdentity{identity},
		)
		require.NoError(t, err)

		assert.Empty(t, snapshot.Transactions)
		assert.Empty(t, snapshot.Matches)
		assert.Equal(t, []domain.Transaction{outsideTransaction}, snapshot.IdentityTransactions)
		assert.Equal(t, []domain.ProviderTransactionMatch{outsideMatch}, snapshot.IdentityMatches)
		assert.Equal(
			t,
			[]string{"listAccounts", "listTransactions", "listMatches", "listIdentityMatches"},
			fixture.operationOrder,
		)
		assert.Equal(t, []string{connection.ConnectionID}, fixture.listIdentityMatchConnectionIDs)
		assert.Equal(t, [][]ProviderTransactionIdentity{{identity}}, fixture.listIdentityMatchIdentities)
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

		_, err = store.LoadExistingWindow(t.Context(), connection, window, nil)
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
		_, err = store.LoadExistingWindow(t.Context(), connection, window, nil)
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
		_, err = store.LoadExistingWindow(t.Context(), connection, window, nil)
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
			bankConnection: &domain.BankConnection{
				ID:       connection.ConnectionID,
				TenantID: "tenant-connection-" + fake.UUID().V4(),
			},
			financeAccounts: map[string]domain.Account{
				existingFinanceAccount.ID: existingFinanceAccount,
			},
			transactions: []domain.Transaction{existingTransaction},
			matches:      []domain.ProviderTransactionMatch{existingMatch},
		}
		persistence := makePersistence(t, fixture)
		generatedIDs := []string{"balance-id", "created-transaction-id", "created-match-id"}
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

		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:          connection,
			SnapshotWindow:      window,
			AccountObservations: []domain.ProviderAccountObservation{accountObservation},
			BalanceObservations: []domain.ProviderBalanceObservation{balanceObservation},
			TransactionActions:  []ProviderTransactionAction{createAction, updateAction},
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
				"listIdentityMatches",
				"getBankConnection",
				"saveAccount",
				"getFinanceAccount",
				"saveFinanceAccount",
				"saveBalance",
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

		require.Len(t, fixture.savedTransactions, 2)
		assert.Equal(t, domain.Transaction{
			ID:               "created-transaction-id",
			TenantID:         fixture.bankConnection.TenantID,
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

		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
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

	t.Run("keeps an immutable mapping winner across stale first-sync snapshots", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		now := time.Date(2026, time.August, 18, 13, 0, 0, 0, time.UTC)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		fixture := &persistenceFixture{
			bankConnection: &domain.BankConnection{
				ID:       connection.ConnectionID,
				TenantID: "tenant-" + fake.UUID().V4(),
			},
			transactionalAccounts: map[string]domain.ConnectionProviderAccount{},
		}
		persistence := makePersistence(t, fixture)
		generatedIDs := []string{
			"finance-account-first-" + fake.UUID().V4(),
			"mapping-first-" + fake.UUID().V4(),
			"finance-account-stale-" + fake.UUID().V4(),
			"mapping-stale-" + fake.UUID().V4(),
		}
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
		plan := ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: makeRandomProviderSyncWindow(fake),
			AccountObservations: []domain.ProviderAccountObservation{{
				Connection:        connection,
				ProviderAccountID: providerAccountID,
				Name:              "account-" + fake.Lorem().Word(),
				Currency:          "PLN",
			}},
		}

		firstStats, err := store.ApplySync(t.Context(), plan, ApplyPlan{})
		require.NoError(t, err)
		secondStats, err := store.ApplySync(t.Context(), plan, ApplyPlan{})
		require.NoError(t, err)

		assert.Equal(t, domain.ProviderSyncStats{CreatedAccounts: 1}, firstStats)
		assert.Equal(t, domain.ProviderSyncStats{}, secondStats)
		require.Len(t, fixture.savedFinanceAccounts, 1)
		require.Len(t, fixture.transactionalAccounts, 1)
		assert.Equal(
			t,
			fixture.savedFinanceAccounts[0].ID,
			fixture.transactionalAccounts[providerAccountID].FinanceAccountID,
		)
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

		now := time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC)
		connectionTenantID := "tenant-connection-" + fake.UUID().V4()
		fixture := &persistenceFixture{bankConnection: &domain.BankConnection{
			ID:       connection.ConnectionID,
			TenantID: connectionTenantID,
		}}
		persistence := makePersistence(t, fixture)
		financeAccountID := "finance-account-" + fake.UUID().V4()
		providerAccountRowID := "provider-account-row-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()
		matchID := "match-" + fake.UUID().V4()
		generatedIDs := []string{financeAccountID, providerAccountRowID, transactionID, matchID}
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

		firstTransaction := domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     observedAccount.ProviderAccountID,
			ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           int64(-fake.IntBetween(100, 90000)),
			Currency:              observedAccount.Currency,
			Description:           "transaction-" + fake.Lorem().Word(),
			EffectiveAt:           now.Add(-time.Hour),
			Fingerprint:           "fingerprint-" + fake.UUID().V4(),
		}
		createAction := ProviderTransactionAction{
			Type:        ProviderTransactionActionTypeCreate,
			Observation: firstTransaction,
		}
		stats, err := store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:          connection,
			SnapshotWindow:      window,
			AccountObservations: []domain.ProviderAccountObservation{observedAccount, observedAccount},
			TransactionActions:  []ProviderTransactionAction{createAction},
		}, ApplyPlan{TransactionWrites: []ApplyTransactionWrite{{Action: createAction}}})
		require.NoError(t, err)
		assert.Equal(t, domain.ProviderSyncStats{CreatedAccounts: 1}, stats)
		require.Len(t, fixture.savedFinanceAccounts, 1)
		assert.Equal(t, domain.Account{
			ID:       financeAccountID,
			TenantID: connectionTenantID,
			Name:     observedAccount.Name,
			Currency: observedAccount.Currency,
			Kind:     domain.AccountKindLinked,
			LinkedAccount: &domain.LinkedAccount{
				Provider:          string(connection.ProviderID),
				ProviderAccountID: observedAccount.ProviderAccountID,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}, fixture.savedFinanceAccounts[0])
		require.Len(t, fixture.savedAccounts, 2)
		assert.Equal(t, fixture.savedFinanceAccounts[0].ID, fixture.savedAccounts[0].FinanceAccountID)
		require.Len(t, fixture.savedTransactions, 1)
		assert.Equal(t, connectionTenantID, fixture.savedTransactions[0].TenantID)
		assert.Equal(t, financeAccountID, fixture.savedTransactions[0].AccountID)
		assert.Equal(t, []string{
			"listAccounts", "listTransactions", "listMatches", "listIdentityMatches", "getBankConnection",
			"saveAccount", "saveFinanceAccount", "getFinanceAccount", "saveAccount",
			"getFinanceAccount", "saveTransaction", "saveMatch",
		}, fixture.operationOrder)

		fixture = &persistenceFixture{listAccountsErr: errors.New("apply-load-snapshot-" + fake.UUID().V4())}
		persistence = makePersistence(t, fixture)
		store, err = NewProviderWindowSyncStore(persistence)
		require.NoError(t, err)
		_, err = store.ApplySync(
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
		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
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
		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
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

		createAction = ProviderTransactionAction{
			Type:        ProviderTransactionActionTypeCreate,
			Observation: writeObservation,
		}
		_, err = store.ApplySync(
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
		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
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

		_, err := resolveProviderAccount(providerAccountsByProviderID(accounts), "missing-provider")
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

	t.Run(
		"attaches connection account balance and fingerprint-identified transaction snapshots in the apply transaction",
		func(t *testing.T) {
			fake := faker.New()
			connection := makeRandomProviderConnectionRef(
				fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking,
			)
			now := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
			providerAccount := makeProviderAccount(fake, connection.ConnectionID)
			financeAccount := domain.Account{
				ID:       providerAccount.FinanceAccountID,
				TenantID: "tenant-" + fake.UUID().V4(),
				Kind:     domain.AccountKindLinked,
				LinkedAccount: &domain.LinkedAccount{
					ProviderAccountID: providerAccount.ProviderAccountID,
				},
			}
			fixture := &persistenceFixture{
				accounts: []domain.ConnectionProviderAccount{providerAccount},
				bankConnection: &domain.BankConnection{
					ID:       connection.ConnectionID,
					TenantID: financeAccount.TenantID,
				},
				financeAccounts: map[string]domain.Account{financeAccount.ID: financeAccount},
				transactions: []domain.Transaction{{
					ID:        "existing-" + fake.UUID().V4(),
					TenantID:  financeAccount.TenantID,
					AccountID: financeAccount.ID,
				}},
			}
			persistence := makePersistence(t, fixture)
			store, err := NewProviderWindowSyncStore(
				persistence,
				WithWindowSyncStoreNow(func() time.Time { return now }),
			)
			require.NoError(t, err)
			observation := domain.ProviderTransactionObservation{
				Connection:        connection,
				ProviderAccountID: providerAccount.ProviderAccountID,
				Status:            domain.TransactionStatusBooked,
				Currency:          "PLN",
				Description:       "transaction-" + fake.Lorem().Word(),
				EffectiveAt:       now,
				Fingerprint:       "fingerprint-" + fake.UUID().V4(),
			}
			action := ProviderTransactionAction{
				Type:        ProviderTransactionActionTypeCreate,
				Observation: observation,
			}
			plan := ProviderDiffPlan{
				Connection:     connection,
				SnapshotWindow: domain.ProviderSyncWindow{Start: now.Add(-time.Hour), End: now.Add(time.Hour)},
				SnapshotObservations: []domain.ProviderSnapshotObservation{
					{
						Kind:             domain.ProviderSnapshotKindConnection,
						ProviderObjectID: "connection-" + fake.UUID().V4(),
						DocumentJSON:     []byte(`{"connection":true}`),
						CapturedAt:       now,
					},
					{
						Kind:              domain.ProviderSnapshotKindAccount,
						ProviderObjectID:  providerAccount.ProviderAccountID,
						ProviderAccountID: providerAccount.ProviderAccountID,
						DocumentJSON:      []byte(`{"account":true}`),
						CapturedAt:        now,
					},
					{
						Kind:              domain.ProviderSnapshotKindAccountBalance,
						ProviderObjectID:  providerAccount.ProviderAccountID,
						ProviderAccountID: providerAccount.ProviderAccountID,
						DocumentJSON:      []byte(`{"balance":true}`),
						CapturedAt:        now,
					},
					{
						Kind:              domain.ProviderSnapshotKindTransaction,
						ProviderObjectID:  observation.Fingerprint,
						ProviderAccountID: providerAccount.ProviderAccountID,
						DocumentJSON:      []byte(`{"transaction":true}`),
						CapturedAt:        now,
					},
				},
				TransactionActions: []ProviderTransactionAction{action},
			}
			_, err = store.ApplySync(t.Context(), plan, ApplyPlan{
				TransactionWrites: []ApplyTransactionWrite{{Action: action}},
			})
			require.NoError(t, err)
			require.Len(t, fixture.savedProviderSnapshots, 4)
			assert.ElementsMatch(t,
				[]domain.ProviderSnapshotKind{
					domain.ProviderSnapshotKindConnection,
					domain.ProviderSnapshotKindAccount,
					domain.ProviderSnapshotKindAccountBalance,
					domain.ProviderSnapshotKindTransaction,
				},
				[]domain.ProviderSnapshotKind{
					fixture.savedProviderSnapshots[0].Kind,
					fixture.savedProviderSnapshots[1].Kind,
					fixture.savedProviderSnapshots[2].Kind,
					fixture.savedProviderSnapshots[3].Kind,
				},
			)
			transactionSnapshot := fixture.savedProviderSnapshots[3]
			require.Len(t, fixture.savedTransactions, 1)
			assert.Equal(t, observation.Fingerprint, transactionSnapshot.ProviderObjectID)
			assert.Equal(t, fixture.savedTransactions[0].ID, transactionSnapshot.FinanceTransactionID)
		},
	)

	t.Run("returns connection snapshot lookup failures from the apply transaction", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking,
		)
		observation := domain.ProviderSnapshotObservation{
			Kind:             domain.ProviderSnapshotKindConnection,
			ProviderObjectID: "connection-" + fake.UUID().V4(),
			DocumentJSON:     []byte(`{"connection":true}`),
			CapturedAt:       time.Now(),
		}
		apply := func(store *ProviderWindowSyncStore) error {
			_, err := store.ApplySync(t.Context(), ProviderDiffPlan{
				Connection:           connection,
				SnapshotWindow:       makeRandomProviderSyncWindow(fake),
				SnapshotObservations: []domain.ProviderSnapshotObservation{observation},
			}, ApplyPlan{})
			return err
		}

		expectedErr := errors.New("get-bank-connection-" + fake.UUID().V4())
		fixture := &persistenceFixture{getBankConnectionErr: expectedErr}
		store, err := NewProviderWindowSyncStore(makePersistence(t, fixture))
		require.NoError(t, err)
		err = apply(store)
		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "get bank connection for sync apply")

		fixture = &persistenceFixture{missingBankConnection: true}
		store, err = NewProviderWindowSyncStore(makePersistence(t, fixture))
		require.NoError(t, err)
		err = apply(store)
		require.ErrorContains(t, err, "bank connection not found for sync apply")
	})

	t.Run("fails the atomic apply when writing a provider snapshot fails", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking,
		)
		providerAccount := makeProviderAccount(fake, connection.ConnectionID)
		financeAccount := domain.Account{
			ID:       providerAccount.FinanceAccountID,
			TenantID: "tenant-" + fake.UUID().V4(),
			Kind:     domain.AccountKindLinked,
		}
		expectedErr := errors.New("save-provider-snapshot-" + fake.UUID().V4())
		fixture := &persistenceFixture{
			accounts:                []domain.ConnectionProviderAccount{providerAccount},
			financeAccounts:         map[string]domain.Account{financeAccount.ID: financeAccount},
			saveProviderSnapshotErr: expectedErr,
		}
		store, err := NewProviderWindowSyncStore(makePersistence(t, fixture))
		require.NoError(t, err)

		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: makeRandomProviderSyncWindow(fake),
			SnapshotObservations: []domain.ProviderSnapshotObservation{{
				Kind:              domain.ProviderSnapshotKindAccount,
				ProviderObjectID:  providerAccount.ProviderAccountID,
				ProviderAccountID: providerAccount.ProviderAccountID,
				DocumentJSON:      []byte(`{"account":true}`),
				CapturedAt:        time.Now(),
			}},
		}, ApplyPlan{})
		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "save provider snapshot")
		assert.Empty(t, fixture.savedProviderSnapshots)
	})

	t.Run("rejects account snapshots without a mapped provider account", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking,
		)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		fixture := &persistenceFixture{}
		store, err := NewProviderWindowSyncStore(makePersistence(t, fixture))
		require.NoError(t, err)

		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: makeRandomProviderSyncWindow(fake),
			SnapshotObservations: []domain.ProviderSnapshotObservation{{
				Kind:              domain.ProviderSnapshotKindAccount,
				ProviderObjectID:  providerAccountID,
				ProviderAccountID: providerAccountID,
				DocumentJSON:      []byte(`{"account":true}`),
				CapturedAt:        time.Now(),
			}},
		}, ApplyPlan{})
		require.ErrorContains(t, err, "provider account mapping not found")
		assert.Empty(t, fixture.savedProviderSnapshots)
	})

	t.Run("appends successful state after window writes with aggregate statistics", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking,
		)
		attemptedAt := time.Now().Add(-time.Minute)
		completedAt := time.Now()
		state := domain.ProviderSyncState{
			Connection:  connection,
			AttemptedAt: &attemptedAt,
			SucceededAt: &completedAt,
			Window:      makeRandomProviderSyncWindow(fake),
			RunID:       "run-" + fake.UUID().V4(),
			JobID:       "job-" + fake.UUID().V4(),
			AggregateStats: domain.ProviderSyncStats{
				ObservedAccounts: fake.IntBetween(1, 3),
			},
		}
		windowStats := domain.ProviderSyncStats{
			CreatedAccounts:      fake.IntBetween(1, 3),
			CreatedTransactions:  fake.IntBetween(1, 3),
			UpdatedTransactions:  fake.IntBetween(1, 3),
			ObservedTransactions: fake.IntBetween(1, 3),
		}
		fixture := &persistenceFixture{}
		store, err := NewProviderWindowSyncStore(makePersistence(t, fixture))
		require.NoError(t, err)

		stats, err := store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: state.Window,
		}, ApplyPlan{Stats: windowStats}, state)
		require.NoError(t, err)
		assert.Equal(t, windowStats, stats)
		require.Len(t, fixture.savedSyncStates, 1)
		assert.Equal(
			t,
			mergeProviderSyncStats(state.AggregateStats, windowStats),
			fixture.savedSyncStates[0].AggregateStats,
		)
		assert.Equal(t, "appendSyncState", fixture.operationOrder[len(fixture.operationOrder)-1])
	})

	t.Run("returns journal failures after applying window writes", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake, domain.ProviderIDMonobank, domain.ProviderConnectorIDMonobank,
		)
		expectedErr := errors.New("append-state-" + fake.UUID().V4())
		fixture := &persistenceFixture{appendSyncStateErr: expectedErr}
		store, err := NewProviderWindowSyncStore(makePersistence(t, fixture))
		require.NoError(t, err)

		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: makeRandomProviderSyncWindow(fake),
		}, ApplyPlan{}, domain.ProviderSyncState{Connection: connection})
		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "append successful provider sync state")
		assert.Empty(t, fixture.savedSyncStates)
	})

	t.Run("rejects account snapshots whose finance account no longer exists", func(t *testing.T) {
		fake := faker.New()
		connection := makeRandomProviderConnectionRef(
			fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking,
		)
		providerAccount := makeProviderAccount(fake, connection.ConnectionID)
		fixture := &persistenceFixture{
			accounts:              []domain.ConnectionProviderAccount{providerAccount},
			missingFinanceAccount: true,
		}
		store, err := NewProviderWindowSyncStore(makePersistence(t, fixture))
		require.NoError(t, err)

		_, err = store.ApplySync(t.Context(), ProviderDiffPlan{
			Connection:     connection,
			SnapshotWindow: makeRandomProviderSyncWindow(fake),
			SnapshotObservations: []domain.ProviderSnapshotObservation{{
				Kind:              domain.ProviderSnapshotKindAccount,
				ProviderObjectID:  providerAccount.ProviderAccountID,
				ProviderAccountID: providerAccount.ProviderAccountID,
				DocumentJSON:      []byte(`{"account":true}`),
				CapturedAt:        time.Now(),
			}},
		}, ApplyPlan{})
		require.ErrorContains(t, err, "finance account not found for provider snapshot")
		assert.Empty(t, fixture.savedProviderSnapshots)
	})
}
