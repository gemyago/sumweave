//go:build postgres_test

package finance

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderSnapshotService(t *testing.T) {
	fake := faker.New()

	type fixture struct {
		ownerUserID  string
		outsiderID   string
		tenant       domain.Tenant
		account      domain.Account
		otherAccount domain.Account
		transaction  domain.Transaction
		connection   domain.BankConnection
		coreStore    *persistence.Store
		service      *ProviderSnapshotService
		store        *persistence.ProviderSnapshotStore
		now          time.Time
	}

	makeFixture := func(t *testing.T) fixture {
		t.Helper()
		database := openTestDatabase(t)
		coreStore := persistence.NewStore(database)
		now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.FixedZone("fixture", 2*60*60))
		tenant := domain.Tenant{
			ID: "tenant-" + fake.UUID().V4(), Name: "tenant-" + fake.Lorem().Word(), DisplayCurrency: "PLN",
			CreatedAt: now, UpdatedAt: now,
		}
		ownerUserID := "user-" + fake.UUID().V4()
		_, err := coreStore.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)
		_, err = coreStore.SaveTenantMembership(t.Context(), domain.TenantMembership{
			TenantID: tenant.ID, UserID: ownerUserID, JoinedAt: now, CreatedAt: now,
		})
		require.NoError(t, err)
		makeAccount := func(prefix string) domain.Account {
			return domain.Account{
				ID: "account-" + prefix + "-" + fake.UUID().V4(), TenantID: tenant.ID,
				Name: prefix + "-" + fake.Lorem().Word(), Currency: "PLN", Kind: domain.AccountKindLinked,
				CreatedAt: now, UpdatedAt: now,
			}
		}
		account := makeAccount("primary")
		otherAccount := makeAccount("other")
		_, err = coreStore.SaveAccount(t.Context(), account)
		require.NoError(t, err)
		_, err = coreStore.SaveAccount(t.Context(), otherAccount)
		require.NoError(t, err)
		transaction := domain.Transaction{
			ID: "transaction-" + fake.UUID().V4(), TenantID: tenant.ID, AccountID: account.ID,
			Source: domain.TransactionSourceProvider, Status: domain.TransactionStatusBooked,
			Kind: domain.TransactionKindRegular, AmountMinor: -123, Currency: "PLN",
			Description: "transaction-" + fake.Lorem().Word(), EffectiveAt: now, CreatedAt: now, UpdatedAt: now,
		}
		_, err = coreStore.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)
		connection := domain.BankConnection{
			ID: "connection-" + fake.UUID().V4(), TenantID: tenant.ID, Provider: "pko",
			ConnectorID: domain.ProviderConnectorIDEnableBanking, DisplayName: "connection-" + fake.Lorem().Word(),
			ProviderReference: "reference-" + fake.UUID().V4(), SecretID: "secret-" + fake.UUID().V4(),
			State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now,
		}
		_, err = coreStore.SaveBankConnection(t.Context(), connection)
		require.NoError(t, err)
		return fixture{
			ownerUserID: ownerUserID, outsiderID: "outsider-" + fake.UUID().V4(), tenant: tenant,
			account: account, otherAccount: otherAccount, transaction: transaction, connection: connection,
			coreStore: coreStore, store: persistence.NewProviderSnapshotStore(database), now: now,
			service: NewProviderSnapshotService(persistence.NewProviderSnapshotStore(database)),
		}
	}

	makeSnapshot := func(data fixture, subject domain.ProviderSnapshotSubject) domain.ProviderSnapshot {
		snapshot := domain.ProviderSnapshot{
			ID:               "snapshot-" + fake.UUID().V4(),
			TenantID:         data.tenant.ID,
			ConnectionID:     data.connection.ID,
			Subject:          subject,
			FinanceAccountID: data.account.ID,
			Kind:             domain.ProviderSnapshotKindAccount,
			ProviderObjectID: "provider-object-" + fake.UUID().V4(),
			DocumentJSON: []byte(
				`{"visible":"ok","accessToken":"not-stored","api_key":"not-stored","accessKey":"not-stored","password":"not-stored","credentials":"not-stored","nested":{"privateKey":"not-stored","passphrase":"not-stored"}}`,
			),
			CapturedAt: data.now,
		}
		if subject == domain.ProviderSnapshotSubjectTransaction {
			snapshot.FinanceTransactionID = data.transaction.ID
			snapshot.Kind = domain.ProviderSnapshotKindTransaction
		}
		return snapshot
	}

	t.Run("authorizes account and transaction reads with metadata lists and sanitized details", func(t *testing.T) {
		data := makeFixture(t)
		accountSnapshot, err := data.store.SaveProviderSnapshot(
			t.Context(),
			makeSnapshot(data, domain.ProviderSnapshotSubjectAccount),
		)
		require.NoError(t, err)
		transactionSnapshot, err := data.store.SaveProviderSnapshot(
			t.Context(),
			makeSnapshot(data, domain.ProviderSnapshotSubjectTransaction),
		)
		require.NoError(t, err)

		accountItems, err := data.service.ListAccountProviderSnapshots(t.Context(), ListAccountProviderSnapshotsParams{
			ActorUserID: data.ownerUserID,
			TenantID:    data.tenant.ID,
			AccountID:   data.account.ID,
		})
		require.NoError(t, err)
		require.Len(t, accountItems, 1)
		assert.Equal(t, accountSnapshot.ID, accountItems[0].ID)
		assert.Empty(t, accountItems[0].DocumentJSON)

		accountDetail, err := data.service.GetAccountProviderSnapshot(
			t.Context(),
			GetAccountProviderSnapshotParams{
				ActorUserID: data.ownerUserID,
				TenantID:    data.tenant.ID,
				AccountID:   data.account.ID,
				SnapshotID:  accountSnapshot.ID,
			},
		)
		require.NoError(t, err)
		assert.JSONEq(t, `{"visible":"ok","nested":{}}`, string(accountDetail.DocumentJSON))

		transactionItems, err := data.service.ListTransactionProviderSnapshots(
			t.Context(),
			ListTransactionProviderSnapshotsParams{
				ActorUserID:   data.ownerUserID,
				TenantID:      data.tenant.ID,
				TransactionID: data.transaction.ID,
			},
		)
		require.NoError(t, err)
		require.Len(t, transactionItems, 1)
		assert.Equal(t, transactionSnapshot.ID, transactionItems[0].ID)
		assert.Empty(t, transactionItems[0].DocumentJSON)

		_, err = data.service.GetTransactionProviderSnapshot(
			t.Context(),
			GetTransactionProviderSnapshotParams{
				ActorUserID:   data.ownerUserID,
				TenantID:      data.tenant.ID,
				TransactionID: data.transaction.ID,
				SnapshotID:    transactionSnapshot.ID,
			},
		)
		require.NoError(t, err)
		_, err = data.service.ListAccountProviderSnapshots(t.Context(), ListAccountProviderSnapshotsParams{
			ActorUserID: data.outsiderID,
			TenantID:    data.tenant.ID,
			AccountID:   data.account.ID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = data.service.GetAccountProviderSnapshot(
			t.Context(),
			GetAccountProviderSnapshotParams{
				ActorUserID: data.ownerUserID,
				TenantID:    data.tenant.ID,
				AccountID:   data.otherAccount.ID,
				SnapshotID:  accountSnapshot.ID,
			},
		)
		require.ErrorIs(t, err, ErrProviderSnapshotNotFound)
	})

	t.Run("deletes every connection-owned snapshot with the connection metadata", func(t *testing.T) {
		data := makeFixture(t)
		_, err := data.coreStore.SaveBankConnection(t.Context(), data.connection)
		require.NoError(t, err)
		_, err = data.store.SaveProviderSnapshot(
			t.Context(),
			makeSnapshot(data, domain.ProviderSnapshotSubjectAccount),
		)
		require.NoError(t, err)
		connectionSnapshot := makeSnapshot(data, domain.ProviderSnapshotSubjectAccount)
		connectionSnapshot.ID = "snapshot-connection-" + fake.UUID().V4()
		connectionSnapshot.Subject = domain.ProviderSnapshotSubjectConnection
		connectionSnapshot.Kind = domain.ProviderSnapshotKindConnection
		connectionSnapshot.FinanceAccountID = ""
		_, err = data.store.SaveProviderSnapshot(t.Context(), connectionSnapshot)
		require.NoError(t, err)

		syncService := NewBankSyncService(
			data.coreStore,
			newMockbankSyncOrchestrator(t),
			WithBankSyncServiceSnapshotDeleter(data.store),
		)
		require.NoError(t, syncService.DeleteBankConnection(t.Context(), DeleteBankConnectionParams{
			ActorUserID:  data.ownerUserID,
			TenantID:     data.tenant.ID,
			ConnectionID: data.connection.ID,
		}))
		items, err := data.store.ListProviderSnapshotsByConnection(t.Context(), data.connection.ID)
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("maps ownership and lookup failures without exposing another tenant data", func(t *testing.T) {
		data := makeFixture(t)
		missingAccountID := "account-missing-" + fake.UUID().V4()
		missingTransactionID := "transaction-missing-" + fake.UUID().V4()
		missingSnapshotID := "snapshot-missing-" + fake.UUID().V4()
		_, err := data.service.ListAccountProviderSnapshots(
			t.Context(),
			ListAccountProviderSnapshotsParams{
				ActorUserID: data.ownerUserID,
				TenantID:    data.tenant.ID,
				AccountID:   missingAccountID,
			},
		)
		require.ErrorIs(t, err, ErrAccountNotFound)
		_, err = data.service.GetAccountProviderSnapshot(
			t.Context(),
			GetAccountProviderSnapshotParams{
				ActorUserID: data.outsiderID,
				TenantID:    data.tenant.ID,
				AccountID:   data.account.ID,
				SnapshotID:  missingSnapshotID,
			},
		)
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = data.service.ListTransactionProviderSnapshots(
			t.Context(),
			ListTransactionProviderSnapshotsParams{
				ActorUserID:   data.outsiderID,
				TenantID:      data.tenant.ID,
				TransactionID: data.transaction.ID,
			},
		)
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = data.service.ListTransactionProviderSnapshots(
			t.Context(),
			ListTransactionProviderSnapshotsParams{
				ActorUserID:   data.ownerUserID,
				TenantID:      data.tenant.ID,
				TransactionID: missingTransactionID,
			},
		)
		require.ErrorIs(t, err, ErrTransactionNotFound)
		_, err = data.service.GetTransactionProviderSnapshot(
			t.Context(),
			GetTransactionProviderSnapshotParams{
				ActorUserID:   data.outsiderID,
				TenantID:      data.tenant.ID,
				TransactionID: data.transaction.ID,
				SnapshotID:    missingSnapshotID,
			},
		)
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = data.service.GetTransactionProviderSnapshot(
			t.Context(),
			GetTransactionProviderSnapshotParams{
				ActorUserID:   data.ownerUserID,
				TenantID:      data.tenant.ID,
				TransactionID: data.transaction.ID,
				SnapshotID:    missingSnapshotID,
			},
		)
		require.ErrorIs(t, err, ErrProviderSnapshotNotFound)
	})
}
