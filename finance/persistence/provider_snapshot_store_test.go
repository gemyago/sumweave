package persistence

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderSnapshotStore(t *testing.T) {
	fake := faker.New()
	providerSnapshotIDs := func(items []domain.ProviderSnapshot) []string {
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		return ids
	}
	saveConnection := func(
		t *testing.T,
		store *Store,
		snapshot domain.ProviderSnapshot,
	) {
		t.Helper()
		_, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID: snapshot.ConnectionID, TenantID: snapshot.TenantID, Provider: "pko",
			ConnectorID: domain.ProviderConnectorIDEnableBanking, DisplayName: "connection-" + fake.Lorem().Word(),
			ProviderReference: "reference-" + fake.UUID().V4(), SecretID: "secret-" + fake.UUID().V4(),
			State: domain.BankConnectionStateActive, CreatedAt: snapshot.CapturedAt, UpdatedAt: snapshot.CapturedAt,
		})
		require.NoError(t, err)
	}

	makeSnapshot := func() domain.ProviderSnapshot {
		return domain.ProviderSnapshot{
			ID:               "snapshot-" + fake.UUID().V4(),
			TenantID:         "tenant-" + fake.UUID().V4(),
			ConnectionID:     "connection-" + fake.UUID().V4(),
			Subject:          domain.ProviderSnapshotSubjectAccount,
			FinanceAccountID: "account-" + fake.UUID().V4(),
			Kind:             domain.ProviderSnapshotKindAccount,
			ProviderObjectID: "provider-account-" + fake.UUID().V4(),
			DocumentJSON:     []byte(`{"value":"first","accessToken":"not-stored"}`),
			CapturedAt:       time.Date(2026, time.August, 14, 10, 0, 0, 0, time.FixedZone("fixture", 2*60*60)),
		}
	}

	t.Run("replaces only a newer snapshot with the identical current identity", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewProviderSnapshotStore(database)
		coreStore := NewStore(database)
		first := makeSnapshot()
		saveConnection(t, coreStore, first)
		_, err := coreStore.SaveAccount(t.Context(), domain.Account{
			ID:        first.FinanceAccountID,
			TenantID:  first.TenantID,
			Name:      "account-" + fake.Lorem().Word(),
			Currency:  "PLN",
			Kind:      domain.AccountKindLinked,
			CreatedAt: first.CapturedAt,
			UpdatedAt: first.CapturedAt,
		})
		require.NoError(t, err)
		firstSaved, err := store.SaveProviderSnapshot(t.Context(), first)
		require.NoError(t, err)

		latest := first
		latest.ID = "snapshot-retry-" + fake.UUID().V4()
		latest.DocumentJSON = []byte(`{"value":"latest","privateKey":"not-stored"}`)
		latest.CapturedAt = first.CapturedAt.Add(time.Minute)
		latestSaved, err := store.SaveProviderSnapshot(t.Context(), latest)
		require.NoError(t, err)
		assert.Equal(t, firstSaved.ID, latestSaved.ID)
		assert.JSONEq(t, `{"value":"latest"}`, string(latestSaved.DocumentJSON))

		stale := latest
		stale.ID = "snapshot-stale-" + fake.UUID().V4()
		stale.DocumentJSON = []byte(`{"value":"stale"}`)
		stale.CapturedAt = first.CapturedAt
		staleSaved, err := store.SaveProviderSnapshot(t.Context(), stale)
		require.NoError(t, err)
		assert.Equal(t, latestSaved, staleSaved)
	})

	t.Run("replaces the stable Monobank connection snapshot identity", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewProviderSnapshotStore(database)
		coreStore := NewStore(database)
		first := makeSnapshot()
		first.Subject = domain.ProviderSnapshotSubjectConnection
		first.FinanceAccountID = ""
		first.Kind = domain.ProviderSnapshotKindConnection
		first.ProviderObjectID = "client-info"
		first.DocumentJSON = []byte(`{"name":"first"}`)
		saveConnection(t, coreStore, first)
		firstSaved, err := store.SaveProviderSnapshot(t.Context(), first)
		require.NoError(t, err)

		latest := first
		latest.ID = "snapshot-retry-" + fake.UUID().V4()
		latest.DocumentJSON = []byte(`{"name":"latest"}`)
		latest.CapturedAt = first.CapturedAt.Add(time.Minute)
		latestSaved, err := store.SaveProviderSnapshot(t.Context(), latest)
		require.NoError(t, err)
		assert.Equal(t, firstSaved.ID, latestSaved.ID)
		assert.JSONEq(t, `{"name":"latest"}`, string(latestSaved.DocumentJSON))

		snapshots, err := store.ListProviderSnapshotsByConnection(t.Context(), first.ConnectionID)
		require.NoError(t, err)
		require.Len(t, snapshots, 1)
		assert.Equal(t, "client-info", snapshots[0].ProviderObjectID)
	})

	t.Run("keeps snapshots distinct across subject provider object and kind", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewProviderSnapshotStore(database)
		coreStore := NewStore(database)
		account := makeSnapshot()
		saveConnection(t, coreStore, account)
		_, err := coreStore.SaveAccount(t.Context(), domain.Account{
			ID:        account.FinanceAccountID,
			TenantID:  account.TenantID,
			Name:      "account-" + fake.Lorem().Word(),
			Currency:  "PLN",
			Kind:      domain.AccountKindLinked,
			CreatedAt: account.CapturedAt,
			UpdatedAt: account.CapturedAt,
		})
		require.NoError(t, err)
		accountSaved, err := store.SaveProviderSnapshot(t.Context(), account)
		require.NoError(t, err)

		balance := account
		balance.ID = "snapshot-balance-" + fake.UUID().V4()
		balance.Kind = domain.ProviderSnapshotKindAccountBalance
		balance.DocumentJSON = []byte(`{"balance":"current"}`)
		balanceSaved, err := store.SaveProviderSnapshot(t.Context(), balance)
		require.NoError(t, err)

		otherObject := account
		otherObject.ID = "snapshot-object-" + fake.UUID().V4()
		otherObject.ProviderObjectID = "provider-account-other-" + fake.UUID().V4()
		otherObject.DocumentJSON = []byte(`{"account":"other"}`)
		otherObjectSaved, err := store.SaveProviderSnapshot(t.Context(), otherObject)
		require.NoError(t, err)

		transaction := account
		transaction.ID = "snapshot-transaction-" + fake.UUID().V4()
		transaction.Subject = domain.ProviderSnapshotSubjectTransaction
		transaction.Kind = domain.ProviderSnapshotKindTransaction
		transaction.FinanceTransactionID = "transaction-" + fake.UUID().V4()
		transaction.DocumentJSON = []byte(`{"transaction":"observed"}`)
		_, err = coreStore.SaveTransaction(t.Context(), domain.Transaction{
			ID:          transaction.FinanceTransactionID,
			TenantID:    transaction.TenantID,
			AccountID:   transaction.FinanceAccountID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -123,
			Currency:    "PLN",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: transaction.CapturedAt,
			CreatedAt:   transaction.CapturedAt,
			UpdatedAt:   transaction.CapturedAt,
		})
		require.NoError(t, err)
		transactionSaved, err := store.SaveProviderSnapshot(t.Context(), transaction)
		require.NoError(t, err)

		items, err := store.ListProviderSnapshotsByConnection(t.Context(), account.ConnectionID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{
			accountSaved.ID, balanceSaved.ID, otherObjectSaved.ID, transactionSaved.ID,
		}, providerSnapshotIDs(items))
		accountItems, err := store.ListAccountProviderSnapshots(t.Context(), account.TenantID, account.FinanceAccountID)
		require.NoError(t, err)
		require.Len(t, accountItems, 3)
		_, err = store.GetAccountProviderSnapshot(
			t.Context(),
			account.TenantID,
			account.FinanceAccountID,
			accountSaved.ID,
		)
		require.NoError(t, err)
		transactionItems, err := store.ListTransactionProviderSnapshots(
			t.Context(),
			transaction.TenantID,
			transaction.FinanceTransactionID,
		)
		require.NoError(t, err)
		require.Len(t, transactionItems, 1)
		_, err = store.GetTransactionProviderSnapshot(
			t.Context(),
			transaction.TenantID,
			transaction.FinanceTransactionID,
			transactionSaved.ID,
		)
		require.NoError(t, err)
	})

	t.Run("wraps database failures and validates snapshots before persistence", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewProviderSnapshotStore(database)
		snapshot := makeSnapshot()
		sqlDB, err := database.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		_, err = store.IsTenantMember(t.Context(), snapshot.TenantID, "user-"+fake.UUID().V4())
		require.ErrorContains(t, err, "check provider snapshot tenant membership")
		invalid := snapshot
		invalid.ID = ""
		_, err = store.SaveProviderSnapshot(t.Context(), invalid)
		require.ErrorContains(t, err, "validate provider snapshot")
		_, err = store.SaveProviderSnapshot(t.Context(), snapshot)
		require.ErrorContains(t, err, "check provider snapshot connection")
		err = store.DeleteProviderSnapshotsByConnection(t.Context(), snapshot.ConnectionID)
		require.ErrorContains(t, err, "delete provider snapshots")
		_, err = store.ListProviderSnapshotsByConnection(t.Context(), snapshot.ConnectionID)
		require.ErrorContains(t, err, "list provider snapshots by connection")
		_, err = store.ListAccountProviderSnapshots(t.Context(), snapshot.TenantID, snapshot.FinanceAccountID)
		require.ErrorContains(t, err, "check provider snapshot account")
		_, err = store.ListTransactionProviderSnapshots(t.Context(), snapshot.TenantID, "transaction-"+fake.UUID().V4())
		require.ErrorContains(t, err, "check provider snapshot transaction")
	})

	t.Run("rejects cross-tenant and mismatched finance attachments", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewProviderSnapshotStore(database)
		coreStore := NewStore(database)
		now := time.Date(2026, time.August, 14, 11, 0, 0, 0, time.FixedZone("fixture", 2*60*60))
		tenantID := "tenant-" + fake.UUID().V4()
		otherTenantID := "tenant-other-" + fake.UUID().V4()
		connection := domain.BankConnection{
			ID: "connection-" + fake.UUID().V4(), TenantID: tenantID, Provider: "pko",
			ConnectorID: domain.ProviderConnectorIDEnableBanking, DisplayName: "connection-" + fake.Lorem().Word(),
			ProviderReference: "reference-" + fake.UUID().V4(), SecretID: "secret-" + fake.UUID().V4(),
			State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now,
		}
		otherConnection := connection
		otherConnection.ID = "connection-other-" + fake.UUID().V4()
		otherConnection.TenantID = otherTenantID
		otherConnection.ProviderReference = "reference-other-" + fake.UUID().V4()
		for _, item := range []domain.BankConnection{connection, otherConnection} {
			_, err := coreStore.SaveBankConnection(t.Context(), item)
			require.NoError(t, err)
		}
		account := domain.Account{
			ID: "account-" + fake.UUID().V4(), TenantID: tenantID, Name: "account-" + fake.Lorem().Word(),
			Currency: "PLN", Kind: domain.AccountKindLinked, CreatedAt: now, UpdatedAt: now,
		}
		otherAccount := account
		otherAccount.ID = "account-other-" + fake.UUID().V4()
		otherAccount.TenantID = otherTenantID
		transactionAccount := account
		transactionAccount.ID = "account-transaction-" + fake.UUID().V4()
		for _, item := range []domain.Account{account, otherAccount, transactionAccount} {
			_, err := coreStore.SaveAccount(t.Context(), item)
			require.NoError(t, err)
		}
		transaction := domain.Transaction{
			ID: "transaction-" + fake.UUID().V4(), TenantID: tenantID, AccountID: transactionAccount.ID,
			Source: domain.TransactionSourceProvider, Status: domain.TransactionStatusBooked,
			Kind: domain.TransactionKindRegular, AmountMinor: -123, Currency: "PLN",
			Description: "transaction-" + fake.Lorem().Word(), EffectiveAt: now, CreatedAt: now, UpdatedAt: now,
		}
		_, err := coreStore.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)

		makeAccountSnapshot := func() domain.ProviderSnapshot {
			return domain.ProviderSnapshot{
				ID:               "snapshot-" + fake.UUID().V4(),
				TenantID:         tenantID,
				ConnectionID:     connection.ID,
				Subject:          domain.ProviderSnapshotSubjectAccount,
				FinanceAccountID: account.ID,
				Kind:             domain.ProviderSnapshotKindAccount,
				ProviderObjectID: "provider-object-" + fake.UUID().V4(),
				DocumentJSON:     []byte(`{"value":"observed"}`),
				CapturedAt:       now,
			}
		}
		connectionSnapshot := makeAccountSnapshot()
		connectionSnapshot.Subject = domain.ProviderSnapshotSubjectConnection
		connectionSnapshot.Kind = domain.ProviderSnapshotKindConnection
		connectionSnapshot.FinanceAccountID = ""
		_, err = store.SaveProviderSnapshot(t.Context(), connectionSnapshot)
		require.NoError(t, err)
		unsupportedSubject := connectionSnapshot
		unsupportedSubject.Subject = domain.ProviderSnapshotSubject("unsupported")
		require.Error(t, store.requireSnapshotOwnership(t.Context(), unsupportedSubject))
		crossTenantConnection := makeAccountSnapshot()
		crossTenantConnection.ConnectionID = otherConnection.ID
		_, err = store.SaveProviderSnapshot(t.Context(), crossTenantConnection)
		require.ErrorIs(t, err, ErrBankConnectionNotFound)
		crossTenantAccount := makeAccountSnapshot()
		crossTenantAccount.FinanceAccountID = otherAccount.ID
		_, err = store.SaveProviderSnapshot(t.Context(), crossTenantAccount)
		require.ErrorIs(t, err, ErrAccountNotFound)
		missingTransactionAccount := makeAccountSnapshot()
		missingTransactionAccount.Subject = domain.ProviderSnapshotSubjectTransaction
		missingTransactionAccount.Kind = domain.ProviderSnapshotKindTransaction
		missingTransactionAccount.FinanceAccountID = "account-missing-" + fake.UUID().V4()
		missingTransactionAccount.FinanceTransactionID = transaction.ID
		_, err = store.SaveProviderSnapshot(t.Context(), missingTransactionAccount)
		require.ErrorIs(t, err, ErrAccountNotFound)
		mismatchedTransaction := makeAccountSnapshot()
		mismatchedTransaction.Subject = domain.ProviderSnapshotSubjectTransaction
		mismatchedTransaction.Kind = domain.ProviderSnapshotKindTransaction
		mismatchedTransaction.FinanceTransactionID = transaction.ID
		_, err = store.SaveProviderSnapshot(t.Context(), mismatchedTransaction)
		require.ErrorIs(t, err, ErrTransactionNotFound)
	})
}
