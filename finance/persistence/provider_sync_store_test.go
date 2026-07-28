package persistence

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderSyncStore(t *testing.T) {
	makeStore := func(t *testing.T) *Store {
		t.Helper()
		database := openTestDatabase(t)
		store := NewStore(database)
		return store
	}

	t.Run("orders provider synchronization records by canonical timestamps", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456, time.FixedZone("zero", 0))
		require.True(t, earlier.Before(later))
		tenantID := "tenant-" + fake.UUID().V4()

		makeConnection := func(id string, createdAt time.Time) domain.BankConnection {
			return domain.BankConnection{
				ID: id, TenantID: tenantID, Provider: "provider-" + fake.Lorem().Word(),
				DisplayName: fake.Company().Name(), ProviderReference: "ref-" + fake.UUID().V4(),
				SecretID: "secret-" + fake.UUID().V4(), State: domain.BankConnectionStateActive,
				CreatedAt: createdAt, UpdatedAt: createdAt,
			}
		}
		earlierConnection := makeConnection("connection-earlier-"+fake.UUID().V4(), earlier)
		laterConnection := makeConnection("connection-later-"+fake.UUID().V4(), later)
		for _, connection := range []domain.BankConnection{laterConnection, earlierConnection} {
			_, err := store.SaveBankConnection(t.Context(), connection)
			require.NoError(t, err)
		}
		connections, err := store.ListBankConnections(t.Context(), tenantID)
		require.NoError(t, err)
		require.Equal(
			t,
			[]string{earlierConnection.ID, laterConnection.ID},
			[]string{connections[0].ID, connections[1].ID},
		)

		connectionID := earlierConnection.ID
		makeProviderAccount := func(id string, createdAt time.Time) domain.ConnectionProviderAccount {
			return domain.ConnectionProviderAccount{
				ID: id, ConnectionID: connectionID, ProviderAccountID: "provider-account-" + fake.UUID().V4(),
				Name: fake.Person().Name(), Currency: "USD", CreatedAt: createdAt, UpdatedAt: createdAt,
			}
		}
		earlierAccount := makeProviderAccount("account-earlier-"+fake.UUID().V4(), earlier)
		laterAccount := makeProviderAccount("account-later-"+fake.UUID().V4(), later)
		for _, account := range []domain.ConnectionProviderAccount{laterAccount, earlierAccount} {
			_, err = store.SaveConnectionProviderAccount(t.Context(), account)
			require.NoError(t, err)
		}
		accounts, err := store.ListConnectionProviderAccounts(t.Context(), connectionID)
		require.NoError(t, err)
		require.Equal(t, []string{earlierAccount.ID, laterAccount.ID}, []string{accounts[0].ID, accounts[1].ID})

		for _, snapshot := range []domain.BalanceSnapshot{
			{ID: "snapshot-earlier-" + fake.UUID().V4(), ConnectionID: connectionID, ProviderAccountID: earlierAccount.ProviderAccountID, Currency: "USD", CapturedAt: earlier},
			{ID: "snapshot-later-" + fake.UUID().V4(), ConnectionID: connectionID, ProviderAccountID: earlierAccount.ProviderAccountID, Currency: "USD", CapturedAt: later},
		} {
			_, err = store.SaveBalanceSnapshot(t.Context(), snapshot)
			require.NoError(t, err)
		}
		snapshots, err := store.ListBalanceSnapshots(t.Context(), connectionID)
		require.NoError(t, err)
		require.Equal(t, later.Format(time.RFC3339Nano), snapshots[0].CapturedAt.Format(time.RFC3339Nano))

		for _, payload := range []domain.RawPayload{
			{ID: "payload-later-" + fake.UUID().V4(), ConnectionID: connectionID, Scope: domain.RawPayloadScopeTransaction, ProviderObjectID: "payload-later-" + fake.UUID().V4(), PayloadJSON: []byte(`{}`), CapturedAt: later},
			{ID: "payload-earlier-" + fake.UUID().V4(), ConnectionID: connectionID, Scope: domain.RawPayloadScopeTransaction, ProviderObjectID: "payload-earlier-" + fake.UUID().V4(), PayloadJSON: []byte(`{}`), CapturedAt: earlier},
		} {
			_, err = store.SaveRawPayload(t.Context(), payload)
			require.NoError(t, err)
		}
		payloads, err := store.ListRawPayloads(t.Context(), connectionID)
		require.NoError(t, err)
		require.Equal(t, earlier.Format(time.RFC3339Nano), payloads[0].CapturedAt.Format(time.RFC3339Nano))

		current := payloads[0]
		current.ID = "payload-current-" + fake.UUID().V4()
		current.PayloadJSON = []byte(`{"value":"current","clientSecret":"not-stored"}`)
		current.CapturedAt = later.Add(time.Minute)
		updatedPayload, err := store.SaveRawPayload(t.Context(), current)
		require.NoError(t, err)
		assert.Equal(t, payloads[0].ID, updatedPayload.ID)
		assert.JSONEq(t, `{"value":"current"}`, string(updatedPayload.PayloadJSON))
		payloads, err = store.ListRawPayloads(t.Context(), connectionID)
		require.NoError(t, err)
		require.Len(t, payloads, 2)

		fingerprint := "fingerprint-" + fake.UUID().V4()
		earlierMatch := domain.ProviderTransactionMatch{
			ID: "match-earlier-" + fake.UUID().V4(), ConnectionID: connectionID,
			ProviderAccountID: earlierAccount.ProviderAccountID, Fingerprint: fingerprint,
			TransactionID: "transaction-earlier-" + fake.UUID().V4(), Status: domain.TransactionStatusPending,
			CreatedAt: earlier, UpdatedAt: earlier,
		}
		laterMatch := earlierMatch
		laterMatch.ID = "match-later-" + fake.UUID().V4()
		laterMatch.TransactionID = "transaction-later-" + fake.UUID().V4()
		laterMatch.CreatedAt = later
		laterMatch.UpdatedAt = later
		for _, match := range []domain.ProviderTransactionMatch{earlierMatch, laterMatch} {
			_, err = store.SaveProviderTransactionMatch(t.Context(), match)
			require.NoError(t, err)
		}
		resolved, err := store.GetProviderTransactionMatchByFingerprint(
			t.Context(), connectionID, earlierAccount.ProviderAccountID, fingerprint,
		)
		require.NoError(t, err)
		require.Equal(t, laterMatch.ID, resolved.ID)
		require.Equal(t, later.Format(time.RFC3339Nano), resolved.UpdatedAt.Format(time.RFC3339Nano))
	})

	t.Run("chooses pending link start creation and expiry by canonical timestamps", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456, time.FixedZone("zero", 0))
		now := later.Add(15 * time.Minute)
		require.True(t, earlier.Before(later))
		state := "state-" + fake.UUID().V4()
		provider := "provider-" + fake.UUID().V4()
		makeStart := func(
			id string,
			tenantID string,
			createdAt time.Time,
			expiresAt time.Time,
		) domain.PendingBankConnectionLinkStart {
			return domain.PendingBankConnectionLinkStart{
				ID:               id,
				TenantID:         tenantID,
				ActorUserID:      "actor-" + fake.UUID().V4(),
				Provider:         provider,
				ConnectorID:      domain.ProviderConnectorIDEnableBanking,
				State:            state,
				CallbackURL:      "http://localhost/" + fake.UUID().V4(),
				AuthorizationURL: "https://example.test/" + fake.UUID().V4(),
				ExpiresAt:        expiresAt,
				CreatedAt:        createdAt,
				UpdatedAt:        createdAt,
			}
		}
		earlierStart := makeStart(
			"earlier-"+fake.UUID().V4(),
			"tenant-earlier-"+fake.UUID().V4(),
			earlier,
			now.Add(time.Hour),
		)
		laterStart := makeStart(
			"later-"+fake.UUID().V4(),
			"tenant-later-"+fake.UUID().V4(),
			later,
			now.Add(time.Hour),
		)
		for _, start := range []domain.PendingBankConnectionLinkStart{earlierStart, laterStart} {
			_, err := store.SavePendingBankConnectionLinkStart(t.Context(), start)
			require.NoError(t, err)
		}
		resolved, err := store.GetPendingBankConnectionLinkStartByState(t.Context(), provider, state)
		require.NoError(t, err)
		require.Equal(t, laterStart.ID, resolved.ID)

		expired := makeStart("expired-"+fake.UUID().V4(), "tenant-expired-"+fake.UUID().V4(), earlier, later)
		_, err = store.SavePendingBankConnectionLinkStart(t.Context(), expired)
		require.NoError(t, err)
		consumed, err := store.ConsumePendingBankConnectionLinkStart(
			t.Context(), expired.TenantID, expired.ActorUserID, expired.Provider, expired.State, now,
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Nil(t, consumed)
	})

	t.Run(
		"persists bank connections schedules accounts snapshots raw payloads and sync matches",
		func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			now := time.Now().UTC()

			missingConnection, err := store.GetBankConnection(
				t.Context(),
				"missing-"+fake.UUID().V4(),
			)
			require.ErrorIs(t, err, ErrBankConnectionNotFound)
			assert.Nil(t, missingConnection)

			connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
				ID:                "connection-" + fake.UUID().V4(),
				TenantID:          "tenant-" + fake.UUID().V4(),
				Provider:          "provider-" + fake.Lorem().Word(),
				ConnectorID:       domain.ProviderConnectorIDMonobank,
				DisplayName:       "display-" + fake.Lorem().Word(),
				ProviderReference: "ref-" + fake.UUID().V4(),
				ExternalID:        "external-" + fake.UUID().V4(),
				SecretID:          "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
				CreatedAt:         now,
				UpdatedAt:         now,
			})
			require.NoError(t, err)

			loadedConnection, err := store.GetBankConnection(t.Context(), connection.ID)
			require.NoError(t, err)
			require.NotNil(t, loadedConnection)
			assert.Equal(t, connection.ConnectorID, loadedConnection.ConnectorID)

			connections, err := store.ListBankConnections(t.Context(), connection.TenantID)
			require.NoError(t, err)
			require.Len(t, connections, 1)

			missingSchedule, err := store.GetBankConnectionSchedule(t.Context(), connection.ID)
			require.ErrorIs(t, err, ErrBankConnectionScheduleNotFound)
			assert.Nil(t, missingSchedule)

			schedule, err := store.SaveBankConnectionSchedule(
				t.Context(),
				domain.BankConnectionSchedule{
					ConnectionID: connection.ID,
					Interval:     time.Hour,
					NextRunAt:    &now,
					Enabled:      true,
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			)
			require.NoError(t, err)
			assert.True(t, schedule.Enabled)

			loadedSchedule, err := store.GetBankConnectionSchedule(t.Context(), connection.ID)
			require.NoError(t, err)
			require.NotNil(t, loadedSchedule)

			accounts, err := store.ListConnectionProviderAccounts(t.Context(), connection.ID)
			require.NoError(t, err)
			assert.Empty(t, accounts)

			providerAccount, err := store.SaveConnectionProviderAccount(
				t.Context(),
				domain.ConnectionProviderAccount{
					ID:                "provider-account-row-" + fake.UUID().V4(),
					ConnectionID:      connection.ID,
					ProviderAccountID: "provider-account-" + fake.UUID().V4(),
					FinanceAccountID:  "finance-account-" + fake.UUID().V4(),
					Name:              "main",
					Currency:          "USD",
					IBAN:              "PL61109010140000071219812874",
					MaskedPAN:         "4444",
					CreatedAt:         now,
					UpdatedAt:         now,
				},
			)
			require.NoError(t, err)

			accounts, err = store.ListConnectionProviderAccounts(t.Context(), connection.ID)
			require.NoError(t, err)
			require.Len(t, accounts, 1)

			_, err = store.SaveBalanceSnapshot(t.Context(), domain.BalanceSnapshot{
				ID:                  "snapshot-" + fake.UUID().V4(),
				ConnectionID:        connection.ID,
				ProviderAccountID:   providerAccount.ProviderAccountID,
				FinanceAccountID:    providerAccount.FinanceAccountID,
				Currency:            "USD",
				CurrentBalanceMinor: 100,
				CapturedAt:          now,
			})
			require.NoError(t, err)

			snapshots, err := store.ListBalanceSnapshots(t.Context(), connection.ID)
			require.NoError(t, err)
			require.Len(t, snapshots, 1)

			_, err = store.SaveRawPayload(t.Context(), domain.RawPayload{
				ID:               "payload-" + fake.UUID().V4(),
				ConnectionID:     connection.ID,
				Scope:            domain.RawPayloadScopeTransaction,
				ProviderObjectID: "txn-" + fake.UUID().V4(),
				PayloadJSON:      []byte(`{"ok":true}`),
				CapturedAt:       now,
			})
			require.NoError(t, err)

			payloads, err := store.ListRawPayloads(t.Context(), connection.ID)
			require.NoError(t, err)
			require.Len(t, payloads, 1)

			missingRun, err := store.GetBankConnectionSyncRun(
				t.Context(),
				connection.ID,
				"sync-missing",
			)
			require.ErrorIs(t, err, ErrBankConnectionSyncRunNotFound)
			assert.Nil(t, missingRun)

			claimed, err := store.ClaimBankConnectionSyncRun(t.Context(), domain.BankConnectionSyncRun{
				ID:           "claim-" + fake.UUID().V4(),
				ConnectionID: connection.ID,
				SyncKey:      "sync-claim-" + fake.UUID().V4(),
				JobID:        "job-" + fake.UUID().V4(),
				CreatedAt:    now,
			})
			require.NoError(t, err)
			assert.True(t, claimed)

			claimed, err = store.ClaimBankConnectionSyncRun(t.Context(), domain.BankConnectionSyncRun{
				ID:           "claim-duplicate-" + fake.UUID().V4(),
				ConnectionID: connection.ID,
				SyncKey:      "sync-claim-duplicate",
				JobID:        "job-" + fake.UUID().V4(),
				CreatedAt:    now,
			})
			require.NoError(t, err)
			assert.True(t, claimed)
			claimed, err = store.ClaimBankConnectionSyncRun(t.Context(), domain.BankConnectionSyncRun{
				ID:           "claim-duplicate-2-" + fake.UUID().V4(),
				ConnectionID: connection.ID,
				SyncKey:      "sync-claim-duplicate",
				JobID:        "job-" + fake.UUID().V4(),
				CreatedAt:    now,
			})
			require.NoError(t, err)
			assert.False(t, claimed)

			_, err = store.SaveBankConnectionSyncRun(t.Context(), domain.BankConnectionSyncRun{
				ID:           "run-" + fake.UUID().V4(),
				ConnectionID: connection.ID,
				SyncKey:      "sync-" + fake.UUID().V4(),
				JobID:        "job-" + fake.UUID().V4(),
				CreatedAt:    now,
			})
			require.NoError(t, err)

			loadedRun, err := store.GetBankConnectionSyncRun(
				t.Context(),
				connection.ID,
				"sync-"+fake.UUID().V4(),
			)
			require.ErrorIs(t, err, ErrBankConnectionSyncRunNotFound)
			assert.Nil(t, loadedRun)

			match, err := store.SaveProviderTransactionMatch(
				t.Context(),
				domain.ProviderTransactionMatch{
					ID:                "match-" + fake.UUID().V4(),
					ConnectionID:      connection.ID,
					ProviderAccountID: providerAccount.ProviderAccountID,
					Fingerprint:       "fingerprint-" + fake.UUID().V4(),
					TransactionID:     "transaction-" + fake.UUID().V4(),
					Status:            domain.TransactionStatusPending,
					CreatedAt:         now,
					UpdatedAt:         now,
				},
			)
			require.NoError(t, err)

			loadedByFingerprint, err := store.GetProviderTransactionMatchByFingerprint(
				t.Context(),
				connection.ID,
				providerAccount.ProviderAccountID,
				match.Fingerprint,
			)
			require.NoError(t, err)
			require.NotNil(t, loadedByFingerprint)

			missingByProviderID, err := store.GetProviderTransactionMatchByProviderID(
				t.Context(),
				connection.ID,
				providerAccount.ProviderAccountID,
				"missing-provider-id",
			)
			require.ErrorIs(t, err, ErrProviderTransactionMatchNotFound)
			assert.Nil(t, missingByProviderID)

			match.ProviderTransactionID = "provider-txn-" + fake.UUID().V4()
			_, err = store.SaveProviderTransactionMatch(t.Context(), match)
			require.NoError(t, err)

			loadedByProviderID, err := store.GetProviderTransactionMatchByProviderID(
				t.Context(),
				connection.ID,
				providerAccount.ProviderAccountID,
				match.ProviderTransactionID,
			)
			require.NoError(t, err)
			require.NotNil(t, loadedByProviderID)

			nilByProviderID, err := store.GetProviderTransactionMatchByProviderID(
				t.Context(),
				connection.ID,
				providerAccount.ProviderAccountID,
				"",
			)
			require.ErrorIs(t, err, ErrProviderTransactionMatchNotFound)
			assert.Nil(t, nilByProviderID)
			nilByFingerprint, err := store.GetProviderTransactionMatchByFingerprint(
				t.Context(),
				connection.ID,
				providerAccount.ProviderAccountID,
				"",
			)
			require.ErrorIs(t, err, ErrProviderTransactionMatchNotFound)
			assert.Nil(t, nilByFingerprint)
		},
	)

	t.Run("deletes connection-owned provider sync metadata without touching other links", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		now := time.Now().UTC()

		secretOne, err := store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "secret-1-" + fake.UUID().V4(),
			Provider:  "provider-one",
			Reference: "ref-1",
			Envelope: credentials.Envelope{
				KeyVersion: "kv",
				Algorithm:  "alg",
				Nonce:      "nonce-1",
				Ciphertext: "cipher-1",
			},
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err)
		secretTwo, err := store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "secret-2-" + fake.UUID().V4(),
			Provider:  "provider-two",
			Reference: "ref-2",
			Envelope: credentials.Envelope{
				KeyVersion: "kv",
				Algorithm:  "alg",
				Nonce:      "nonce-2",
				Ciphertext: "cipher-2",
			},
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err)

		connectionOne, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-1-" + fake.UUID().V4(),
			TenantID:          "tenant-" + fake.UUID().V4(),
			Provider:          "provider-one",
			DisplayName:       "display-1",
			ProviderReference: "ref-1",
			ExternalID:        "ext-1",
			SecretID:          secretOne.ID,
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
		require.NoError(t, err)
		connectionTwo, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-2-" + fake.UUID().V4(),
			TenantID:          connectionOne.TenantID,
			Provider:          "provider-two",
			DisplayName:       "display-2",
			ProviderReference: "ref-2",
			ExternalID:        "ext-2",
			SecretID:          secretTwo.ID,
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
		require.NoError(t, err)

		for _, connection := range []domain.BankConnection{connectionOne, connectionTwo} {
			_, err = store.SaveBankConnectionSchedule(t.Context(), domain.BankConnectionSchedule{
				ConnectionID: connection.ID,
				Interval:     time.Hour,
				NextRunAt:    &now,
				Enabled:      true,
				CreatedAt:    now,
				UpdatedAt:    now,
			})
			require.NoError(t, err)
			account, saveAccountErr := store.SaveConnectionProviderAccount(
				t.Context(),
				domain.ConnectionProviderAccount{
					ID:                "provider-account-row-" + fake.UUID().V4(),
					ConnectionID:      connection.ID,
					ProviderAccountID: "provider-account-" + fake.UUID().V4(),
					FinanceAccountID:  "finance-account-" + fake.UUID().V4(),
					Name:              "main",
					Currency:          "USD",
					CreatedAt:         now,
					UpdatedAt:         now,
				},
			)
			require.NoError(t, saveAccountErr)
			_, err = store.SaveBalanceSnapshot(t.Context(), domain.BalanceSnapshot{
				ID:                  "snapshot-" + fake.UUID().V4(),
				ConnectionID:        connection.ID,
				ProviderAccountID:   account.ProviderAccountID,
				FinanceAccountID:    account.FinanceAccountID,
				Currency:            "USD",
				CurrentBalanceMinor: 100,
				CapturedAt:          now,
			})
			require.NoError(t, err)
			_, err = store.SaveRawPayload(t.Context(), domain.RawPayload{
				ID:               "payload-" + fake.UUID().V4(),
				ConnectionID:     connection.ID,
				Scope:            domain.RawPayloadScopeTransaction,
				ProviderObjectID: "provider-object-" + fake.UUID().V4(),
				PayloadJSON:      []byte(`{"ok":true}`),
				CapturedAt:       now,
			})
			require.NoError(t, err)
			_, err = store.SaveBankConnectionSyncRun(t.Context(), domain.BankConnectionSyncRun{
				ID:           "run-" + fake.UUID().V4(),
				ConnectionID: connection.ID,
				SyncKey:      "sync-" + fake.UUID().V4(),
				JobID:        "job-" + fake.UUID().V4(),
				CreatedAt:    now,
			})
			require.NoError(t, err)
			_, err = store.SaveProviderTransactionMatch(t.Context(), domain.ProviderTransactionMatch{
				ID:                "match-" + fake.UUID().V4(),
				ConnectionID:      connection.ID,
				ProviderAccountID: account.ProviderAccountID,
				Fingerprint:       "fingerprint-" + fake.UUID().V4(),
				TransactionID:     "transaction-" + fake.UUID().V4(),
				Status:            domain.TransactionStatusPending,
				CreatedAt:         now,
				UpdatedAt:         now,
			})
			require.NoError(t, err)
		}

		require.NoError(t, store.DeleteProviderTransactionMatches(t.Context(), connectionOne.ID))
		require.NoError(t, store.DeleteBankConnectionSyncRuns(t.Context(), connectionOne.ID))
		require.NoError(t, store.DeleteRawPayloads(t.Context(), connectionOne.ID))
		require.NoError(t, store.DeleteBalanceSnapshots(t.Context(), connectionOne.ID))
		require.NoError(t, store.DeleteConnectionProviderAccounts(t.Context(), connectionOne.ID))
		require.NoError(t, store.DeleteBankConnectionSchedule(t.Context(), connectionOne.ID))
		require.NoError(t, store.DeleteBankConnection(t.Context(), connectionOne.ID))
		require.NoError(t, store.DeleteConnectionSecret(t.Context(), secretOne.ID))

		deletedConnection, err := store.GetBankConnection(t.Context(), connectionOne.ID)
		require.ErrorIs(t, err, ErrBankConnectionNotFound)
		assert.Nil(t, deletedConnection)
		deletedSchedule, err := store.GetBankConnectionSchedule(t.Context(), connectionOne.ID)
		require.ErrorIs(t, err, ErrBankConnectionScheduleNotFound)
		assert.Nil(t, deletedSchedule)
		accounts, err := store.ListConnectionProviderAccounts(t.Context(), connectionOne.ID)
		require.NoError(t, err)
		assert.Empty(t, accounts)
		snapshots, err := store.ListBalanceSnapshots(t.Context(), connectionOne.ID)
		require.NoError(t, err)
		assert.Empty(t, snapshots)
		payloads, err := store.ListRawPayloads(t.Context(), connectionOne.ID)
		require.NoError(t, err)
		assert.Empty(t, payloads)
		run, err := store.GetBankConnectionSyncRun(t.Context(), connectionOne.ID, "missing")
		require.ErrorIs(t, err, ErrBankConnectionSyncRunNotFound)
		assert.Nil(t, run)
		secret, err := store.GetConnectionSecret(t.Context(), secretOne.ID)
		require.ErrorIs(t, err, ErrConnectionSecretNotFound)
		assert.Nil(t, secret)

		preservedConnection, err := store.GetBankConnection(t.Context(), connectionTwo.ID)
		require.NoError(t, err)
		require.NotNil(t, preservedConnection)
		preservedSchedule, err := store.GetBankConnectionSchedule(t.Context(), connectionTwo.ID)
		require.NoError(t, err)
		require.NotNil(t, preservedSchedule)
		preservedAccounts, err := store.ListConnectionProviderAccounts(t.Context(), connectionTwo.ID)
		require.NoError(t, err)
		require.Len(t, preservedAccounts, 1)
	})

	t.Run("surfaces database errors across provider sync lookups and deletes", func(t *testing.T) {
		store := makeStore(t)
		sqlDB, err := store.DB().DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, err = store.GetBankConnection(t.Context(), "connection-1")
		require.ErrorContains(t, err, "get bank connection")
		_, err = store.ListBankConnections(t.Context(), "tenant-1")
		require.ErrorContains(t, err, "list bank connections")
		_, err = store.GetBankConnectionSchedule(t.Context(), "connection-1")
		require.ErrorContains(t, err, "get bank connection schedule")
		require.ErrorContains(
			t,
			store.DeleteBankConnectionSchedule(t.Context(), "connection-1"),
			"delete bank connection schedule",
		)
		require.ErrorContains(
			t,
			store.DeleteConnectionProviderAccounts(t.Context(), "connection-1"),
			"delete connection provider accounts",
		)
	})

	t.Run("persists and consumes pending bank link starts by tenant actor and state", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		now := time.Date(2026, time.June, 22, 12, 0, 0, 0, time.UTC)

		pendingStart, err := store.SavePendingBankConnectionLinkStart(
			t.Context(),
			domain.PendingBankConnectionLinkStart{
				ID:                "pending-" + fake.UUID().V4(),
				TenantID:          "tenant-" + fake.UUID().V4(),
				ActorUserID:       "actor-" + fake.UUID().V4(),
				Provider:          "pko",
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				State:             "state-" + fake.UUID().V4(),
				CallbackURL:       "http://localhost:5173/#/finance/connections",
				AuthorizationURL:  "https://example.test/auth/" + fake.UUID().V4(),
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				StartResult: domain.PendingBankConnectionLinkStartResult{
					State:            "start-state-" + fake.UUID().V4(),
					AuthorizationURL: "https://example.test/start/" + fake.UUID().V4(),
					RawPayloads: []domain.ProviderRawPayloadObservation{
						{
							Connection: domain.ProviderConnectionRef{
								ConnectionID:      "connection-" + fake.UUID().V4(),
								ProviderID:        domain.ProviderIDPKO,
								ConnectorID:       domain.ProviderConnectorIDEnableBanking,
								ProviderReference: "provider-ref-raw-" + fake.UUID().V4(),
								ExternalID:        "external-raw-" + fake.UUID().V4(),
							},
							Scope:            domain.RawPayloadScopeConnection,
							ProviderObjectID: "payload-" + fake.UUID().V4(),
							PayloadJSON:      []byte(`{"step":"start"}`),
							CapturedAt:       now.Add(2 * time.Minute),
						},
					},
				},
				ExpiresAt: now.Add(15 * time.Minute),
				CreatedAt: now,
				UpdatedAt: now,
			},
		)
		require.NoError(t, err)
		resolved, err := store.GetPendingBankConnectionLinkStartByState(
			t.Context(),
			pendingStart.Provider,
			pendingStart.State,
		)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, pendingStart.CallbackURL, resolved.CallbackURL)
		assert.Equal(t, pendingStart.ConnectorID, resolved.ConnectorID)
		assert.Equal(t, pendingStart.StartResult, resolved.StartResult)
		resolved, err = store.GetPendingBankConnectionLinkStartByState(
			t.Context(),
			pendingStart.Provider,
			"missing-state-"+fake.UUID().V4(),
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Nil(t, resolved)

		consumed, err := store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			pendingStart.TenantID,
			"actor-other-"+fake.UUID().V4(),
			pendingStart.Provider,
			pendingStart.State,
			now.Add(time.Minute),
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Nil(t, consumed)

		consumed, err = store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			"tenant-other-"+fake.UUID().V4(),
			pendingStart.ActorUserID,
			pendingStart.Provider,
			pendingStart.State,
			now.Add(time.Minute),
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Nil(t, consumed)

		consumed, err = store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			pendingStart.TenantID,
			pendingStart.ActorUserID,
			pendingStart.Provider,
			"missing-state-"+fake.UUID().V4(),
			now.Add(time.Minute),
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Nil(t, consumed)

		consumed, err = store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			pendingStart.TenantID,
			pendingStart.ActorUserID,
			pendingStart.Provider,
			pendingStart.State,
			now.Add(time.Minute),
		)
		require.NoError(t, err)
		require.NotNil(t, consumed)
		assert.Equal(t, pendingStart.ProviderReference, consumed.ProviderReference)
		assert.Equal(t, pendingStart.ConnectorID, consumed.ConnectorID)
		assert.Equal(t, pendingStart.StartResult, consumed.StartResult)
		require.NotNil(t, consumed.ConsumedAt)

		consumed, err = store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			pendingStart.TenantID,
			pendingStart.ActorUserID,
			pendingStart.Provider,
			pendingStart.State,
			now.Add(2*time.Minute),
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Nil(t, consumed)

		err = store.RestorePendingBankConnectionLinkStart(
			t.Context(),
			pendingStart.TenantID,
			pendingStart.ActorUserID,
			pendingStart.Provider,
			pendingStart.State,
			now.Add(3*time.Minute),
		)
		require.NoError(t, err)

		consumed, err = store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			pendingStart.TenantID,
			pendingStart.ActorUserID,
			pendingStart.Provider,
			pendingStart.State,
			now.Add(4*time.Minute),
		)
		require.NoError(t, err)
		require.NotNil(t, consumed)
		require.NotNil(t, consumed.ConsumedAt)

		expiredStart, err := store.SavePendingBankConnectionLinkStart(
			t.Context(),
			domain.PendingBankConnectionLinkStart{
				ID:                "expired-" + fake.UUID().V4(),
				TenantID:          "tenant-expired-" + fake.UUID().V4(),
				ActorUserID:       "actor-expired-" + fake.UUID().V4(),
				Provider:          "pko",
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				State:             "state-expired-" + fake.UUID().V4(),
				CallbackURL:       "http://localhost:5173/#/finance/connections",
				AuthorizationURL:  "https://example.test/auth/expired/" + fake.UUID().V4(),
				ProviderReference: "provider-ref-expired-" + fake.UUID().V4(),
				ExpiresAt:         now.Add(-time.Minute),
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		)
		require.NoError(t, err)

		consumed, err = store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			expiredStart.TenantID,
			expiredStart.ActorUserID,
			expiredStart.Provider,
			expiredStart.State,
			now,
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Nil(t, consumed)

		defaultTimedStart, err := store.SavePendingBankConnectionLinkStart(
			t.Context(),
			domain.PendingBankConnectionLinkStart{
				ID:                "default-timed-" + fake.UUID().V4(),
				TenantID:          "tenant-default-" + fake.UUID().V4(),
				ActorUserID:       "actor-default-" + fake.UUID().V4(),
				Provider:          "pko",
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				State:             "state-default-" + fake.UUID().V4(),
				CallbackURL:       "http://localhost:5173/#/finance/connections",
				AuthorizationURL:  "https://example.test/auth/default/" + fake.UUID().V4(),
				ProviderReference: "provider-ref-default-" + fake.UUID().V4(),
				ExpiresAt:         now.Add(5 * time.Minute),
			},
		)
		require.NoError(t, err)
		assert.False(t, defaultTimedStart.CreatedAt.IsZero())
		assert.Equal(t, defaultTimedStart.CreatedAt, defaultTimedStart.UpdatedAt)

		err = store.RestorePendingBankConnectionLinkStart(
			t.Context(),
			pendingStart.TenantID,
			pendingStart.ActorUserID,
			pendingStart.Provider,
			"missing-state-"+fake.UUID().V4(),
			now.Add(5*time.Minute),
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
	})

	t.Run("surfaces write failures on read only databases", func(t *testing.T) {
		fake := faker.New()
		path := fmt.Sprintf("%s/%s.db", t.TempDir(), fake.UUID().V4())
		require.NoError(t, os.WriteFile(path, []byte{}, 0o600))
		dsn := "file:" + path + "?mode=ro"
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()
		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		store := NewStore(database)

		_, err = store.SaveBankConnection(t.Context(), domain.BankConnection{ID: "id"})
		require.Error(t, err)
		_, err = store.SavePendingBankConnectionLinkStart(
			t.Context(),
			domain.PendingBankConnectionLinkStart{
				ID:          "pending",
				TenantID:    "tenant",
				ActorUserID: "actor",
				Provider:    "pko",
				State:       "state",
				CallbackURL: "http://localhost:5173/#/finance/connections",
			},
		)
		require.Error(t, err)
		_, err = store.SaveBankConnectionSchedule(
			t.Context(),
			domain.BankConnectionSchedule{ConnectionID: "id"},
		)
		require.Error(t, err)
		_, err = store.SaveConnectionProviderAccount(
			t.Context(),
			domain.ConnectionProviderAccount{
				ID:                "id",
				ConnectionID:      "cid",
				ProviderAccountID: "pid",
			},
		)
		require.Error(t, err)
		_, err = store.SaveBalanceSnapshot(t.Context(), domain.BalanceSnapshot{ID: "id"})
		require.Error(t, err)
		_, err = store.SaveRawPayload(t.Context(), domain.RawPayload{ID: "id"})
		require.Error(t, err)
		_, err = store.SaveBankConnectionSyncRun(
			t.Context(),
			domain.BankConnectionSyncRun{ID: "id", ConnectionID: "cid", SyncKey: "key"},
		)
		require.Error(t, err)
		_, err = store.SaveProviderTransactionMatch(
			t.Context(),
			domain.ProviderTransactionMatch{ID: "id"},
		)
		require.Error(t, err)
	})

	t.Run("surfaces sync run and match query errors when database is unavailable", func(t *testing.T) {
		store := makeStore(t)
		sqlDB, err := store.DB().DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		claimed, err := store.ClaimBankConnectionSyncRun(t.Context(), domain.BankConnectionSyncRun{
			ID:           "run-closed",
			ConnectionID: "connection-closed",
			SyncKey:      "sync-closed",
			CreatedAt:    time.Now().UTC(),
		})
		require.Error(t, err)
		assert.False(t, claimed)

		_, err = store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			"tenant-closed",
			"actor-closed",
			"pko",
			"state-closed",
			time.Now().UTC(),
		)
		require.Error(t, err)

		_, err = store.GetBankConnectionSyncRun(t.Context(), "connection-closed", "sync-closed")
		require.Error(t, err)
		_, err = store.GetProviderTransactionMatchByProviderID(t.Context(), "connection", "account", "provider-txn")
		require.Error(t, err)
		_, err = store.GetProviderTransactionMatchByFingerprint(t.Context(), "connection", "account", "fingerprint")
		require.Error(t, err)
	})
}
