package persistence

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProviderLinkPersistence(t *testing.T) {
	t.Run("atomically saves final connection snapshots and rolls back an invalid one", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		linkPersistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
		//nolint:golines // The atomic-link fixture keeps the coupled identity values together.
		connection := domain.BankConnection{ID: "connection-" + fake.UUID().V4(), TenantID: "tenant-" + fake.UUID().V4(), Provider: string(domain.ProviderIDPKO), ConnectorID: domain.ProviderConnectorIDEnableBanking, ProviderReference: "reference-" + fake.UUID().V4(), SecretID: "secret-" + fake.UUID().V4(), State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now}
		secret := domain.ConnectionSecret{ID: connection.SecretID, Provider: connection.Provider, Reference: connection.ProviderReference, Envelope: credentials.Envelope{KeyVersion: "v1", Algorithm: "test", Nonce: "nonce", Ciphertext: "ciphertext"}, CreatedAt: now, UpdatedAt: now}
		snapshot := &domain.ProviderSnapshot{ID: "snapshot-" + fake.UUID().V4(), TenantID: connection.TenantID, ConnectionID: connection.ID, Subject: domain.ProviderSnapshotSubjectConnection, Kind: domain.ProviderSnapshotKindConnection, ProviderObjectID: connection.ProviderReference, DocumentJSON: []byte(`{"session":"typed"}`), CapturedAt: now}
		saved, err := linkPersistence.SaveLinkedConnectionWithSnapshot(t.Context(), connection, secret, snapshot)
		require.NoError(t, err)
		items, err := NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(t.Context(), saved.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.ProviderSnapshot{*snapshot}, items)
		latestSnapshot := *snapshot
		latestSnapshot.ID = "snapshot-" + fake.UUID().V4()
		latestSnapshot.DocumentJSON = []byte(`{"session":"updated"}`)
		latestSnapshot.CapturedAt = now.Add(time.Minute)
		repeated, err := linkPersistence.SaveLinkedConnectionWithSnapshot(
			t.Context(), connection, secret, &latestSnapshot,
		)
		require.NoError(t, err)
		assert.Equal(t, saved.ID, repeated.ID)
		items, err = NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(t.Context(), saved.ID)
		require.NoError(t, err)
		expectedLatestSnapshot := latestSnapshot
		expectedLatestSnapshot.ID = snapshot.ID
		assert.Equal(t, []domain.ProviderSnapshot{expectedLatestSnapshot}, items)

		failedRepeatedSnapshot := latestSnapshot
		failedRepeatedSnapshot.ID = "snapshot-" + fake.UUID().V4()
		failedRepeatedSnapshot.DocumentJSON = []byte("not-json")
		_, err = linkPersistence.SaveLinkedConnectionWithSnapshot(
			t.Context(), connection, secret, &failedRepeatedSnapshot,
		)
		require.ErrorContains(t, err, "save linked connection provider snapshot")
		items, err = NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(t.Context(), saved.ID)
		require.NoError(t, err)
		assert.Equal(t, []domain.ProviderSnapshot{expectedLatestSnapshot}, items)

		connectionWithoutSnapshot := connection
		connectionWithoutSnapshot.ID = "connection-" + fake.UUID().V4()
		connectionWithoutSnapshot.ProviderReference = "reference-" + fake.UUID().V4()
		connectionWithoutSnapshot.SecretID = "secret-" + fake.UUID().V4()
		secretWithoutSnapshot := secret
		secretWithoutSnapshot.ID = connectionWithoutSnapshot.SecretID
		secretWithoutSnapshot.Reference = connectionWithoutSnapshot.ProviderReference
		savedWithoutSnapshot, err := linkPersistence.SaveLinkedConnectionWithSnapshot(
			t.Context(), connectionWithoutSnapshot, secretWithoutSnapshot, nil,
		)
		require.NoError(t, err)
		items, err = NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(
			t.Context(), savedWithoutSnapshot.ID,
		)
		require.NoError(t, err)
		assert.Empty(t, items)

		duplicateSecretConnection := connection
		duplicateSecretConnection.ID = "connection-" + fake.UUID().V4()
		duplicateSecretConnection.ProviderReference = "reference-" + fake.UUID().V4()
		duplicateSecretConnection.SecretID = secret.ID
		_, err = linkPersistence.SaveLinkedConnectionWithSnapshot(t.Context(), duplicateSecretConnection, secret, nil)
		require.ErrorContains(t, err, "create connection secret")
		var duplicateConnectionCount int64
		require.NoError(t, store.DB().Table((bankConnectionModel{}).TableName()).
			Where("id = ?", duplicateSecretConnection.ID).Count(&duplicateConnectionCount).Error)
		assert.Zero(t, duplicateConnectionCount)

		failedConnection := connection
		failedConnection.ID = "connection-" + fake.UUID().V4()
		failedConnection.ProviderReference = "reference-" + fake.UUID().V4()
		failedConnection.SecretID = "secret-" + fake.UUID().V4()
		failedSecret := secret
		failedSecret.ID = failedConnection.SecretID
		failedSecret.Reference = failedConnection.ProviderReference
		failedSecret.CreatedAt = time.Time{}
		failedSecret.UpdatedAt = time.Time{}
		failedSnapshot := *snapshot
		failedSnapshot.ID = "snapshot-" + fake.UUID().V4()
		failedSnapshot.ConnectionID = failedConnection.ID
		failedSnapshot.ProviderObjectID = failedConnection.ProviderReference
		failedSnapshot.DocumentJSON = []byte("not-json")
		_, err = linkPersistence.SaveLinkedConnectionWithSnapshot(
			t.Context(), failedConnection, failedSecret, &failedSnapshot,
		)
		require.ErrorContains(t, err, "save linked connection provider snapshot")
		var connectionCount int64
		require.NoError(t, store.DB().Table((bankConnectionModel{}).TableName()).
			Where("id = ?", failedConnection.ID).Count(&connectionCount).Error)
		assert.Zero(t, connectionCount)
		var secretCount int64
		require.NoError(t, store.DB().Table((connectionSecretModel{}).TableName()).
			Where("id = ?", failedSecret.ID).Count(&secretCount).Error)
		assert.Zero(t, secretCount)
	})
	t.Run("recovers the atomic linked connection snapshot winner across concurrent finishes", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		linkPersistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.August, 10, 18, 0, 0, 0, time.FixedZone("test", 2*60*60))
		tenantID := "tenant-" + fake.UUID().V4()
		reference := "reference-" + fake.UUID().V4()
		makeLinked := func(id string, secretID string) (domain.BankConnection, domain.ConnectionSecret, domain.ProviderSnapshot) {
			connection := domain.BankConnection{
				ID:                id,
				TenantID:          tenantID,
				Provider:          string(domain.ProviderIDPKO),
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				ProviderReference: reference,
				SecretID:          secretID,
				State:             domain.BankConnectionStateActive,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			secret := domain.ConnectionSecret{
				ID:        secretID,
				Provider:  string(domain.ProviderIDPKO),
				Reference: reference,
				Envelope: credentials.Envelope{
					KeyVersion: "v1", Algorithm: "test", Nonce: "nonce", Ciphertext: "ciphertext",
				},
				CreatedAt: now,
				UpdatedAt: now,
			}
			snapshot := domain.ProviderSnapshot{
				ID:               "snapshot-" + fake.UUID().V4(),
				TenantID:         tenantID,
				ConnectionID:     connection.ID,
				Subject:          domain.ProviderSnapshotSubjectConnection,
				Kind:             domain.ProviderSnapshotKindConnection,
				ProviderObjectID: reference,
				DocumentJSON:     []byte(`{"finish":"` + connection.ID + `"}`),
				CapturedAt:       now,
			}
			return connection, secret, snapshot
		}

		start := make(chan struct{})
		results := make(chan domain.BankConnection, 2)
		errorsByFinish := make(chan error, 2)
		candidateSnapshots := make([]domain.ProviderSnapshot, 0, 2)
		var finishes sync.WaitGroup
		for range 2 {
			connection, secret, snapshot := makeLinked(
				"connection-"+fake.UUID().V4(),
				"secret-"+fake.UUID().V4(),
			)
			candidateSnapshots = append(candidateSnapshots, snapshot)
			finishes.Go(func() {
				<-start
				saved, err := linkPersistence.SaveLinkedConnectionWithSnapshot(
					t.Context(), connection, secret, &snapshot,
				)
				if err != nil {
					errorsByFinish <- err
					return
				}
				results <- saved
			})
		}
		close(start)
		finishes.Wait()
		close(results)
		close(errorsByFinish)
		for err := range errorsByFinish {
			require.NoError(t, err)
		}
		var saved []domain.BankConnection
		for connection := range results {
			saved = append(saved, connection)
		}
		require.Len(t, saved, 2)
		assert.Equal(t, saved[0].ID, saved[1].ID)
		var connectionCount int64
		require.NoError(t, store.DB().Table((bankConnectionModel{}).TableName()).Count(&connectionCount).Error)
		assert.Equal(t, int64(1), connectionCount)
		var secretCount int64
		require.NoError(t, store.DB().Table((connectionSecretModel{}).TableName()).Count(&secretCount).Error)
		assert.Equal(t, int64(1), secretCount)
		snapshots, err := NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(
			t.Context(), saved[0].ID,
		)
		require.NoError(t, err)
		require.Len(t, snapshots, 1)
		require.NoError(t, snapshots[0].Validate())
		assert.Equal(t, saved[0].ID, snapshots[0].ConnectionID)
		assert.Equal(t, candidateSnapshots[0].TenantID, snapshots[0].TenantID)
		assert.Equal(t, candidateSnapshots[0].ProviderObjectID, snapshots[0].ProviderObjectID)
		assert.Contains(t, []string{
			string(candidateSnapshots[0].DocumentJSON),
			string(candidateSnapshots[1].DocumentJSON),
		}, string(snapshots[0].DocumentJSON))

		retry, retrySecret, _ := makeLinked("connection-"+fake.UUID().V4(), "secret-"+fake.UUID().V4())
		retried, err := linkPersistence.SaveLinkedConnectionWithSnapshot(t.Context(), retry, retrySecret, nil)
		require.NoError(t, err)
		assert.Equal(t, saved[0].ID, retried.ID)
	})

	t.Run("returns the committed snapshot winner after a concurrent insert race", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := NewStore(database)
		linkPersistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.August, 10, 19, 0, 0, 0, time.FixedZone("test", 2*60*60))
		tenantID := "tenant-" + fake.UUID().V4()
		reference := "reference-" + fake.UUID().V4()
		makeLinked := func(id string, secretID string, documentJSON []byte) (domain.BankConnection, domain.ConnectionSecret, domain.ProviderSnapshot) {
			connection := domain.BankConnection{
				ID:                id,
				TenantID:          tenantID,
				Provider:          string(domain.ProviderIDPKO),
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				ProviderReference: reference,
				SecretID:          secretID,
				State:             domain.BankConnectionStateActive,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			secret := domain.ConnectionSecret{
				ID:        secretID,
				Provider:  connection.Provider,
				Reference: reference,
				Envelope: credentials.Envelope{
					KeyVersion: "v1", Algorithm: "test", Nonce: "nonce", Ciphertext: "ciphertext",
				},
				CreatedAt: now,
				UpdatedAt: now,
			}
			snapshot := domain.ProviderSnapshot{
				ID:               "snapshot-" + fake.UUID().V4(),
				TenantID:         tenantID,
				ConnectionID:     connection.ID,
				Subject:          domain.ProviderSnapshotSubjectConnection,
				Kind:             domain.ProviderSnapshotKindConnection,
				ProviderObjectID: reference,
				DocumentJSON:     documentJSON,
				CapturedAt:       now,
			}
			return connection, secret, snapshot
		}
		winner, winnerSecret, winnerSnapshot := makeLinked(
			"connection-"+fake.UUID().V4(),
			"secret-"+fake.UUID().V4(),
			[]byte(`{"finish":"winner"}`),
		)
		savedWinner, err := linkPersistence.SaveLinkedConnectionWithSnapshot(
			t.Context(), winner, winnerSecret, &winnerSnapshot,
		)
		require.NoError(t, err)
		loser, loserSecret, loserSnapshot := makeLinked(
			"connection-"+fake.UUID().V4(),
			"secret-"+fake.UUID().V4(),
			[]byte(`{"finish":"loser"}`),
		)

		callbackName := fmt.Sprintf("hide-committed-winner-%s", fake.UUID().V4())
		callbackCalled := false
		require.NoError(t, database.db.Callback().Query().Before("gorm:query").Register(
			callbackName,
			func(tx *gorm.DB) {
				if callbackCalled || tx.Statement.Table != (bankConnectionModel{}).TableName() {
					return
				}
				tx.AddError(gorm.ErrRecordNotFound)
				callbackCalled = true
			},
		))
		t.Cleanup(func() {
			require.NoError(t, database.db.Callback().Query().Remove(callbackName))
		})

		recovered, err := linkPersistence.SaveLinkedConnectionWithSnapshot(
			t.Context(), loser, loserSecret, &loserSnapshot,
		)
		require.NoError(t, err)
		require.True(t, callbackCalled)
		assert.Equal(t, savedWinner.ID, recovered.ID)
		var connectionCount int64
		require.NoError(t, store.DB().Table((bankConnectionModel{}).TableName()).Count(&connectionCount).Error)
		assert.Equal(t, int64(1), connectionCount)
		var secretCount int64
		require.NoError(t, store.DB().Table((connectionSecretModel{}).TableName()).Count(&secretCount).Error)
		assert.Equal(t, int64(1), secretCount)
		snapshots, err := NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(
			t.Context(), recovered.ID,
		)
		require.NoError(t, err)
		require.Len(t, snapshots, 1)
		require.NoError(t, snapshots[0].Validate())
		assert.Equal(t, winnerSnapshot.ID, snapshots[0].ID)
		assert.Equal(t, winnerSnapshot.TenantID, snapshots[0].TenantID)
		assert.Equal(t, recovered.ID, snapshots[0].ConnectionID)
		assert.Equal(t, winnerSnapshot.Subject, snapshots[0].Subject)
		assert.Equal(t, winnerSnapshot.Kind, snapshots[0].Kind)
		assert.Equal(t, winnerSnapshot.ProviderObjectID, snapshots[0].ProviderObjectID)
		assert.Equal(t, winnerSnapshot.DocumentJSON, snapshots[0].DocumentJSON)
		assert.True(t, winnerSnapshot.CapturedAt.Equal(snapshots[0].CapturedAt))
	})

	t.Run("rolls back the secret when linked connection insertion fails", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		linkPersistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.August, 10, 18, 30, 0, 0, time.FixedZone("test", 2*60*60))
		connection := domain.BankConnection{
			ID: "connection-" + fake.UUID().V4(), TenantID: "tenant-" + fake.UUID().V4(),
			Provider: string(domain.ProviderIDPKO), ConnectorID: domain.ProviderConnectorIDEnableBanking,
			ProviderReference: "reference-" + fake.UUID().V4(), SecretID: "secret-" + fake.UUID().V4(),
			State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now,
		}
		secret := domain.ConnectionSecret{
			ID:        connection.SecretID,
			Provider:  connection.Provider,
			Reference: connection.ProviderReference,
			Envelope: credentials.Envelope{
				KeyVersion: "v1", Algorithm: "test", Nonce: "nonce", Ciphertext: "ciphertext",
			},
			CreatedAt: time.Time{},
			UpdatedAt: time.Time{},
		}
		triggerName := "fail_link_connection"
		require.NoError(t, store.DB().Exec(
			"CREATE TRIGGER "+triggerName+" BEFORE INSERT ON finance_bank_connections "+
				"WHEN NEW.id = '"+connection.ID+"' BEGIN SELECT RAISE(ABORT, 'connection insertion failed'); END",
		).Error)
		t.Cleanup(func() {
			require.NoError(t, store.DB().Exec("DROP TRIGGER "+triggerName).Error)
		})

		_, err := linkPersistence.SaveLinkedConnection(t.Context(), connection, secret)
		require.ErrorContains(t, err, "create bank connection")
		var secretCount int64
		require.NoError(t, store.DB().Table((connectionSecretModel{}).TableName()).Count(&secretCount).Error)
		assert.Zero(t, secretCount)
	})

	t.Run("scopes exact non-empty references and never deduplicates empty references", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		linkPersistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.August, 10, 18, 45, 0, 0, time.FixedZone("test", 2*60*60))
		tenantID := "tenant-" + fake.UUID().V4()
		providerReference := "reference-" + fake.UUID().V4()
		makeLinked := func(
			tenant string,
			provider string,
			connector domain.ProviderConnectorID,
			reference string,
		) (domain.BankConnection, domain.ConnectionSecret) {
			connectionID := "connection-" + fake.UUID().V4()
			secretID := "secret-" + fake.UUID().V4()
			return domain.BankConnection{
					ID:                connectionID,
					TenantID:          tenant,
					Provider:          provider,
					ConnectorID:       connector,
					ProviderReference: reference,
					SecretID:          secretID,
					State:             domain.BankConnectionStateActive,
					CreatedAt:         now,
					UpdatedAt:         now,
				}, domain.ConnectionSecret{
					ID:        secretID,
					Provider:  provider,
					Reference: reference,
					Envelope: credentials.Envelope{
						KeyVersion: "v1", Algorithm: "test", Nonce: "nonce", Ciphertext: "ciphertext",
					},
					CreatedAt: now,
					UpdatedAt: now,
				}
		}
		save := func(
			tenant string,
			provider string,
			connector domain.ProviderConnectorID,
			reference string,
		) domain.BankConnection {
			connection, secret := makeLinked(tenant, provider, connector, reference)
			saved, err := linkPersistence.SaveLinkedConnection(t.Context(), connection, secret)
			require.NoError(t, err)
			return saved
		}

		first := save(
			tenantID,
			string(domain.ProviderIDPKO),
			domain.ProviderConnectorIDEnableBanking,
			providerReference,
		)
		retry := save(
			tenantID,
			string(domain.ProviderIDPKO),
			domain.ProviderConnectorIDEnableBanking,
			providerReference,
		)
		withWhitespace := save(
			tenantID,
			string(domain.ProviderIDPKO),
			domain.ProviderConnectorIDEnableBanking,
			" "+providerReference,
		)
		otherTenant := save(
			"tenant-"+fake.UUID().V4(),
			string(domain.ProviderIDPKO),
			domain.ProviderConnectorIDEnableBanking,
			providerReference,
		)
		otherConnector := save(
			tenantID,
			string(domain.ProviderIDPKO),
			domain.ProviderConnectorIDMonobank,
			providerReference,
		)
		emptyFirst := save(tenantID, string(domain.ProviderIDMonobank), domain.ProviderConnectorIDMonobank, "")
		emptySecond := save(tenantID, string(domain.ProviderIDMonobank), domain.ProviderConnectorIDMonobank, "")

		assert.Equal(t, first.ID, retry.ID)
		assert.NotEqual(t, first.ID, withWhitespace.ID)
		assert.NotEqual(t, first.ID, otherTenant.ID)
		assert.NotEqual(t, first.ID, otherConnector.ID)
		assert.NotEqual(t, emptyFirst.ID, emptySecond.ID)
	})

	t.Run("stores parallel pending starts per connector and consumes matching connector only", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.June, 29, 18, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		actorUserID := "actor-" + fake.UUID().V4()
		providerID := string(domain.ProviderIDPKO)
		state := "state-" + fake.UUID().V4()

		first, err := persistence.SavePendingStart(t.Context(), domain.PendingBankConnectionLinkStart{
			ID:          "pending-a-" + fake.UUID().V4(),
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			Provider:    providerID,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			CallbackURL: "http://localhost:5173/#/finance/connections",
			ExpiresAt:   now.Add(15 * time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		second, err := persistence.SavePendingStart(t.Context(), domain.PendingBankConnectionLinkStart{
			ID:          "pending-b-" + fake.UUID().V4(),
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			Provider:    providerID,
			ConnectorID: domain.ProviderConnectorIDMonobank,
			State:       state,
			CallbackURL: "http://localhost:5173/#/finance/connections",
			ExpiresAt:   now.Add(15 * time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		consumed, err := persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDSynthetic,
			State:       state,
			ConsumedAt:  now.Add(time.Minute),
		})
		require.ErrorIs(t, err, providers.ErrPendingStartNotFound)
		assert.Nil(t, consumed)

		consumed, err = persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ConsumedAt:  now.Add(2 * time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, consumed)
		assert.Equal(t, first.ID, consumed.ID)
		assert.Equal(t, first.ConnectorID, consumed.ConnectorID)

		consumed, err = persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDMonobank,
			State:       state,
			ConsumedAt:  now.Add(3 * time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, consumed)
		assert.Equal(t, second.ID, consumed.ID)
		assert.Equal(t, second.ConnectorID, consumed.ConnectorID)
	})

	t.Run("restores consumed pending start and reuses it", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.June, 29, 19, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		actorUserID := "actor-" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()

		_, err := persistence.SavePendingStart(t.Context(), domain.PendingBankConnectionLinkStart{
			ID:          "pending-" + fake.UUID().V4(),
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			Provider:    string(domain.ProviderIDPKO),
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			CallbackURL: "http://localhost:5173/#/finance/connections",
			ExpiresAt:   now.Add(15 * time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		consumed, err := persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ConsumedAt:  now.Add(time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, consumed)

		require.NoError(
			t,
			persistence.RestorePendingStart(t.Context(), providers.RestorePendingStartRequest{
				TenantID:    tenantID,
				ActorUserID: actorUserID,
				ProviderID:  domain.ProviderIDPKO,
				ConnectorID: domain.ProviderConnectorIDEnableBanking,
				State:       state,
				RestoredAt:  now.Add(2 * time.Minute),
			}),
		)

		restored, err := persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ConsumedAt:  now.Add(3 * time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, restored)
		assert.Equal(t, consumed.ID, restored.ID)
	})

	t.Run("returns not found when restoring unknown pending start", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)

		err := persistence.RestorePendingStart(t.Context(), providers.RestorePendingStartRequest{
			TenantID:    "tenant-" + fake.UUID().V4(),
			ActorUserID: "actor-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       "state-" + fake.UUID().V4(),
			RestoredAt:  time.Now().UTC(),
		})
		require.ErrorIs(t, err, providers.ErrPendingStartNotFound)
	})

	t.Run("delegates bank connection operations", func(t *testing.T) {
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)
		fake := faker.New()

		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		observedAt := time.Now().UTC()

		savedConnection, err := persistence.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:        connectionID,
			TenantID:  tenantID,
			Provider:  string(domain.ProviderIDPKO),
			CreatedAt: observedAt,
			UpdatedAt: observedAt,
		})
		require.NoError(t, err)
		require.Equal(t, connectionID, savedConnection.ID)

		connections, err := persistence.ListBankConnections(t.Context(), tenantID)
		require.NoError(t, err)
		require.Len(t, connections, 1)
		assert.Equal(t, connectionID, connections[0].ID)
	})

	t.Run("renames only connection metadata without replacing concurrent fields", func(t *testing.T) {
		fake := faker.New()
		store := NewStore(openTestDatabase(t))
		persistence := NewProviderLinkPersistence(store)
		createdAt := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.FixedZone("test", 2*60*60))
		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		original := domain.BankConnection{
			ID:                connectionID,
			TenantID:          tenantID,
			Provider:          string(domain.ProviderIDPKO),
			ConnectorID:       domain.ProviderConnectorIDEnableBanking,
			DisplayName:       "Original " + fake.Company().Name(),
			ProviderReference: "reference-original-" + fake.UUID().V4(),
			SecretID:          "secret-original-" + fake.UUID().V4(),
			State:             domain.BankConnectionStateActive,
			LastSyncJobID:     "job-original-" + fake.UUID().V4(),
			LastSyncError:     "sync-original-" + fake.UUID().V4(),
			CreatedAt:         createdAt,
			UpdatedAt:         createdAt,
		}
		_, err := persistence.SaveBankConnection(t.Context(), original)
		require.NoError(t, err)

		reauthAt := createdAt.Add(time.Hour)
		lastSyncStartedAt := createdAt.Add(2 * time.Hour)
		lastSuccessfulSyncAt := createdAt.Add(3 * time.Hour)
		concurrent := original
		concurrent.ProviderReference = "reference-concurrent-" + fake.UUID().V4()
		concurrent.SecretID = "secret-concurrent-" + fake.UUID().V4()
		concurrent.State = domain.BankConnectionStateReauthRequired
		concurrent.Reauth = &domain.ConnectionReauthMetadata{
			RequiredAt: &reauthAt,
			Reason:     "reason-" + fake.Lorem().Word(),
		}
		concurrent.LastSyncJobID = "job-concurrent-" + fake.UUID().V4()
		concurrent.LastSyncStartedAt = &lastSyncStartedAt
		concurrent.LastSuccessfulSyncAt = &lastSuccessfulSyncAt
		concurrent.LastSyncError = "sync-concurrent-" + fake.UUID().V4()
		concurrent.UpdatedAt = createdAt.Add(4 * time.Hour)
		_, err = persistence.SaveBankConnection(t.Context(), concurrent)
		require.NoError(t, err)

		renamedAt := createdAt.Add(5 * time.Hour)
		renamedDisplayName := "Renamed " + fake.Company().Name()
		require.NoError(t, persistence.UpdateBankConnectionDisplayName(
			t.Context(), tenantID, connectionID, renamedDisplayName, renamedAt,
		))
		got, err := persistence.GetBankConnection(t.Context(), connectionID)
		require.NoError(t, err)
		expected := concurrent
		expected.DisplayName = renamedDisplayName
		expected.UpdatedAt = renamedAt
		gotMetadata := *got
		expectedMetadata := expected
		gotMetadata.CreatedAt = time.Time{}
		gotMetadata.UpdatedAt = time.Time{}
		gotMetadata.LastSyncStartedAt = nil
		gotMetadata.LastSuccessfulSyncAt = nil
		expectedMetadata.CreatedAt = time.Time{}
		expectedMetadata.UpdatedAt = time.Time{}
		expectedMetadata.LastSyncStartedAt = nil
		expectedMetadata.LastSuccessfulSyncAt = nil
		require.NotNil(t, gotMetadata.Reauth)
		require.NotNil(t, expectedMetadata.Reauth)
		gotReauth := *gotMetadata.Reauth
		gotReauth.RequiredAt = nil
		gotMetadata.Reauth = &gotReauth
		expectedReauth := *expectedMetadata.Reauth
		expectedReauth.RequiredAt = nil
		expectedMetadata.Reauth = &expectedReauth
		assert.Equal(t, expectedMetadata, gotMetadata)
		assert.True(t, expected.CreatedAt.Equal(got.CreatedAt))
		assert.True(t, expected.UpdatedAt.Equal(got.UpdatedAt))
		require.NotNil(t, got.LastSyncStartedAt)
		require.NotNil(t, got.LastSuccessfulSyncAt)
		assert.True(t, expected.LastSyncStartedAt.Equal(*got.LastSyncStartedAt))
		assert.True(t, expected.LastSuccessfulSyncAt.Equal(*got.LastSuccessfulSyncAt))
		require.NotNil(t, got.Reauth.RequiredAt)
		assert.True(t, expected.Reauth.RequiredAt.Equal(*got.Reauth.RequiredAt))

		err = persistence.UpdateBankConnectionDisplayName(
			t.Context(), "tenant-foreign-"+fake.UUID().V4(), connectionID, renamedDisplayName, renamedAt,
		)
		require.ErrorIs(t, err, ErrBankConnectionNotFound)
	})

	t.Run("returns persistence errors when tables are unavailable", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		require.NoError(t, database.db.WithContext(t.Context()).Migrator().DropTable(
			&bankConnectionModel{},
			&pendingBankConnectionLinkStartModel{},
		))
		persistence := NewProviderLinkPersistence(NewStore(database))

		_, err := persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    "tenant-" + fake.UUID().V4(),
			ActorUserID: "actor-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       "state-" + fake.UUID().V4(),
			ConsumedAt:  time.Now().UTC(),
		})
		require.Error(t, err)

		err = persistence.RestorePendingStart(t.Context(), providers.RestorePendingStartRequest{
			TenantID:    "tenant-" + fake.UUID().V4(),
			ActorUserID: "actor-" + fake.UUID().V4(),
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       "state-" + fake.UUID().V4(),
			RestoredAt:  time.Now().UTC(),
		})
		require.Error(t, err)

		_, err = persistence.SaveBankConnection(t.Context(), domain.BankConnection{ID: "missing-" + fake.UUID().V4()})
		require.Error(t, err)

		_, err = persistence.ListBankConnections(t.Context(), "tenant-"+fake.UUID().V4())
		require.Error(t, err)
	})

	t.Run("returns not found when consumed row disappears before read", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := NewStore(database)
		persistence := NewProviderLinkPersistence(store)
		now := time.Date(2026, time.June, 29, 20, 0, 0, 0, time.UTC)
		tenantID := "tenant-" + fake.UUID().V4()
		actorUserID := "actor-" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()

		_, err := persistence.SavePendingStart(t.Context(), domain.PendingBankConnectionLinkStart{
			ID:          "pending-" + fake.UUID().V4(),
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			Provider:    string(domain.ProviderIDPKO),
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ExpiresAt:   now.Add(15 * time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		require.NoError(t, err)

		callbackName := fmt.Sprintf("consume-consumed-not-found-%s", fake.UUID().V4())
		callbackCalled := false
		require.NoError(
			t,
			database.db.Callback().
				Query().
				Before("gorm:query").
				Register(callbackName, func(tx *gorm.DB) {
					if callbackCalled {
						return
					}
					if tx.Statement.Table != (pendingBankConnectionLinkStartModel{}).TableName() {
						return
					}
					tx.AddError(gorm.ErrRecordNotFound)
					callbackCalled = true
				}),
		)
		defer func() {
			require.NoError(t, database.db.Callback().Query().Remove(callbackName))
		}()

		_, err = persistence.ConsumePendingStart(t.Context(), providers.ConsumePendingStartRequest{
			TenantID:    tenantID,
			ActorUserID: actorUserID,
			ProviderID:  domain.ProviderIDPKO,
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       state,
			ConsumedAt:  now.Add(time.Minute),
		})
		require.ErrorIs(t, err, providers.ErrPendingStartNotFound)
		require.True(t, callbackCalled)
	})
}
