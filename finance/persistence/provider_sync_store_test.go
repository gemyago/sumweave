//go:build postgres_test

package persistence

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
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

	t.Run("preserves provider mapping ownership during metadata refresh", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		now := time.Date(2026, time.August, 18, 14, 0, 0, 0, time.UTC)
		first := domain.ConnectionProviderAccount{
			ID:                "mapping-first-" + fake.UUID().V4(),
			ConnectionID:      "connection-" + fake.UUID().V4(),
			ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			FinanceAccountID:  "finance-account-first-" + fake.UUID().V4(),
			Name:              "account-" + fake.Lorem().Word(),
			Currency:          "PLN",
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		savedFirst, err := store.SaveConnectionProviderAccount(t.Context(), first)
		require.NoError(t, err)
		competing := first
		competing.ID = "mapping-competing-" + fake.UUID().V4()
		competing.FinanceAccountID = "finance-account-competing-" + fake.UUID().V4()
		competing.Name = "account-refreshed-" + fake.Lorem().Word()
		competing.UpdatedAt = now.Add(time.Minute)
		savedCompeting, err := store.SaveConnectionProviderAccount(t.Context(), competing)
		require.NoError(t, err)

		assert.Equal(t, savedFirst.FinanceAccountID, savedCompeting.FinanceAccountID)
		assert.Equal(t, savedFirst.ID, savedCompeting.ID)
		assert.Equal(t, competing.Name, savedCompeting.Name)
		mappings, err := store.ListConnectionProviderAccounts(t.Context(), first.ConnectionID)
		require.NoError(t, err)
		assert.Equal(t, []domain.ConnectionProviderAccount{savedCompeting}, mappings)
	})

	t.Run("orders provider synchronization records by canonical timestamps", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123000, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456000, time.FixedZone("zero", 0))
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
		require.True(t, later.Equal(snapshots[0].CapturedAt))

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
		require.True(t, later.Equal(resolved.UpdatedAt))
	})

	t.Run("chooses pending link start creation and expiry by canonical timestamps", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123000, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456000, time.FixedZone("zero", 0))
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
		"persists bank connections schedules accounts snapshots and sync matches",
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
					DocumentJSON:     []byte(`{"step":"start"}`),
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
		store := makeStore(t)
		sqlDB, err := store.DB().DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

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
		_, err = store.SaveProviderTransactionMatch(
			t.Context(),
			domain.ProviderTransactionMatch{ID: "id"},
		)
		require.Error(t, err)
	})

	t.Run("surfaces match query errors when database is unavailable", func(t *testing.T) {
		store := makeStore(t)
		sqlDB, err := store.DB().DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, err = store.ConsumePendingBankConnectionLinkStart(
			t.Context(),
			"tenant-closed",
			"actor-closed",
			"pko",
			"state-closed",
			time.Now().UTC(),
		)
		require.Error(t, err)

		_, err = store.GetProviderTransactionMatchByProviderID(t.Context(), "connection", "account", "provider-txn")
		require.Error(t, err)
		_, err = store.GetProviderTransactionMatchByFingerprint(t.Context(), "connection", "account", "fingerprint")
		require.Error(t, err)
	})
}
