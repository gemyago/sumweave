package finance

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBankSyncServiceOrchestration(t *testing.T) {
	fake := faker.New()

	makeFixture := func(t *testing.T, connectorID domain.ProviderConnectorID) (
		*persistence.Store,
		*BankSyncService,
		*mockbankSyncOrchestrator,
		domain.BankConnection,
		domain.ConnectionSecret,
		time.Time,
	) {
		t.Helper()
		store := persistence.NewStore(openTestDatabase(t))
		now := time.Date(2026, time.August, 18, 10, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
		ownerID := "owner-" + fake.UUID().V4()
		tenant, err := NewTenantService(store).CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "PLN",
		})
		require.NoError(t, err)
		cipherKey := make([]byte, 32)
		for index := range cipherKey {
			cipherKey[index] = byte(index + 1)
		}
		cipher, err := credentials.NewAESGCMCipher(cipherKey, "bank-sync-orchestrator")
		require.NoError(t, err)
		envelope, err := cipher.SealString("secret-" + fake.UUID().V4())
		require.NoError(t, err)
		secret, err := store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        "secret-" + fake.UUID().V4(),
			Provider:  string(domain.ProviderIDPKO),
			Reference: "secret-reference-" + fake.UUID().V4(),
			Envelope:  envelope,
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err)
		persistedSecret, err := store.GetConnectionSecret(t.Context(), secret.ID)
		require.NoError(t, err)
		require.NotNil(t, persistedSecret)
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-" + fake.UUID().V4(),
			TenantID:          tenant.ID,
			Provider:          string(domain.ProviderIDPKO),
			ConnectorID:       connectorID,
			ProviderReference: "session-" + fake.UUID().V4(),
			SecretID:          secret.ID,
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now.Add(-time.Hour),
			UpdatedAt:         now.Add(-time.Hour),
		})
		require.NoError(t, err)
		orchestrator := newMockbankSyncOrchestrator(t)
		service := NewBankSyncService(
			store,
			orchestrator,
			WithBankSyncServiceNow(func() time.Time { return now }),
			WithBankSyncServiceLogger(slog.New(slog.DiscardHandler)),
		)
		return store, service, orchestrator, connection, *persistedSecret, now
	}

	t.Run("requires an orchestrator at construction", func(t *testing.T) {
		store := persistence.NewStore(openTestDatabase(t))
		require.PanicsWithValue(t, "bank sync orchestrator is required", func() {
			NewBankSyncService(store, nil)
		})
	})

	t.Run("passes persisted PKO Enable Banking identity encrypted secret and bounds", func(t *testing.T) {
		store, service, orchestrator, connection, secret, now := makeFixture(
			t,
			domain.ProviderConnectorIDEnableBanking,
		)
		windowStart := now.AddDate(0, 0, -5)
		windowEnd := now.Add(-time.Hour)
		expectedRequest := internalproviders.SyncOrchestrationRequest{
			Connection: domain.ProviderConnectionRef{
				ConnectionID:      connection.ID,
				ProviderID:        domain.ProviderIDPKO,
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				ProviderReference: connection.ProviderReference,
			},
			Secret:      secret,
			JobID:       "job-" + fake.UUID().V4(),
			Reason:      BankConnectionSyncReasonManual,
			WindowStart: &windowStart,
			WindowEnd:   &windowEnd,
		}
		orchestrator.EXPECT().Orchestrate(mock.Anything, expectedRequest).Return(
			internalproviders.SyncOrchestrationResult{Stats: domain.ProviderSyncStats{
				CreatedAccounts:     2,
				CreatedTransactions: 3,
				UpdatedTransactions: 4,
			}}, nil,
		)

		result, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        expectedRequest.JobID,
			Reason:       expectedRequest.Reason,
			WindowStart:  expectedRequest.WindowStart,
			WindowEnd:    expectedRequest.WindowEnd,
		})
		require.NoError(t, err)
		assert.Equal(t, BankConnectionSyncResult{
			ImportedAccounts: 2, ImportedTransactions: 3, UpdatedTransactions: 4,
		}, result)
		persisted, err := store.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, err)
		require.NotNil(t, persisted)
		assert.Equal(t, domain.BankConnectionStateActive, persisted.State)
		assert.Equal(t, expectedRequest.JobID, persisted.LastSyncJobID)
		require.NotNil(t, persisted.LastSyncStartedAt)
		assert.True(t, persisted.LastSyncStartedAt.Equal(now))
		require.NotNil(t, persisted.LastSuccessfulSyncAt)
		assert.True(t, persisted.LastSuccessfulSyncAt.Equal(now))
		assert.Empty(t, persisted.LastSyncError)
	})

	t.Run("records an unknown persisted connector orchestration failure", func(t *testing.T) {
		store, service, orchestrator, connection, secret, now := makeFixture(
			t,
			domain.ProviderConnectorID("unknown-"+fake.UUID().V4()),
		)
		orchestrationErr := errors.New("resolve sync connector: connector not configured")
		orchestrator.EXPECT().Orchestrate(mock.Anything, mock.MatchedBy(
			func(request internalproviders.SyncOrchestrationRequest) bool {
				return request.Connection.ConnectorID == connection.ConnectorID && request.Secret == secret
			},
		)).Return(internalproviders.SyncOrchestrationResult{}, orchestrationErr)

		_, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-" + fake.UUID().V4(),
			Reason:       BankConnectionSyncReasonManual,
		})
		require.ErrorIs(t, err, orchestrationErr)
		persisted, loadErr := store.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, loadErr)
		require.NotNil(t, persisted)
		require.NotNil(t, persisted.LastSyncStartedAt)
		assert.True(t, persisted.LastSyncStartedAt.Equal(now))
		assert.Empty(t, persisted.LastSuccessfulSyncAt)
		assert.Contains(t, persisted.LastSyncError, orchestrationErr.Error())
	})

	t.Run("classifies a terminal provider rejection after recording connection diagnostics", func(t *testing.T) {
		store, service, orchestrator, connection, _, _ := makeFixture(
			t,
			domain.ProviderConnectorIDEnableBanking,
		)
		providerErr := &ProviderResponseError{
			Provider: "provider-" + fake.Letter(), Operation: "sync", StatusCode: http.StatusUnauthorized,
		}
		orchestrator.EXPECT().Orchestrate(mock.Anything, mock.Anything).Return(
			internalproviders.SyncOrchestrationResult{}, providerErr,
		)

		_, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-" + fake.UUID().V4(),
		})

		failure, classified := TerminalFailureFrom(err)
		require.True(t, classified)
		assert.Equal(t, "bank_provider_rejected_request", failure.Code)
		persisted, loadErr := store.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, loadErr)
		assert.Contains(t, persisted.LastSyncError, "status 401")
	})

	t.Run("keeps schedule diagnostics current when an orchestration retry fails", func(t *testing.T) {
		store, service, orchestrator, connection, _, now := makeFixture(
			t,
			domain.ProviderConnectorIDEnableBanking,
		)
		scheduledAt := now.Add(-15 * time.Minute)
		nextRunAt := now.Add(15 * time.Minute)
		_, err := store.SaveBankConnectionSchedule(t.Context(), domain.BankConnectionSchedule{
			ConnectionID: connection.ID,
			Interval:     time.Hour,
			Enabled:      true,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		require.NoError(t, err)
		orchestrationErr := errors.New("failed-middle-window-" + fake.UUID().V4())
		params := RunBankConnectionSyncParams{
			ConnectionID:       connection.ID,
			JobID:              "job-" + fake.UUID().V4(),
			Reason:             BankConnectionSyncReasonScheduled,
			ScheduledAt:        &scheduledAt,
			ScheduledNextRunAt: &nextRunAt,
		}
		orchestrator.EXPECT().
			Orchestrate(mock.Anything, mock.Anything).
			Twice().
			Return(internalproviders.SyncOrchestrationResult{}, orchestrationErr)

		_, err = service.RunBankConnectionSync(t.Context(), params)
		require.ErrorIs(t, err, orchestrationErr)
		_, err = service.RunBankConnectionSync(t.Context(), params)
		require.ErrorIs(t, err, orchestrationErr)

		persistedConnection, err := store.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, err)
		require.NotNil(t, persistedConnection)
		assert.Equal(t, params.JobID, persistedConnection.LastSyncJobID)
		require.NotNil(t, persistedConnection.LastSyncStartedAt)
		assert.True(t, persistedConnection.LastSyncStartedAt.Equal(now))
		assert.Contains(t, persistedConnection.LastSyncError, orchestrationErr.Error())

		persistedSchedule, err := store.GetBankConnectionSchedule(t.Context(), connection.ID)
		require.NoError(t, err)
		require.NotNil(t, persistedSchedule)
		assert.Equal(t, params.JobID, persistedSchedule.LastJobID)
		require.NotNil(t, persistedSchedule.LastStartedAt)
		assert.True(t, persistedSchedule.LastStartedAt.Equal(now))
		require.NotNil(t, persistedSchedule.LastCompletedAt)
		assert.True(t, persistedSchedule.LastCompletedAt.Equal(now))
		require.NotNil(t, persistedSchedule.LastScheduledAt)
		assert.True(t, persistedSchedule.LastScheduledAt.Equal(scheduledAt))
		require.NotNil(t, persistedSchedule.NextRunAt)
		assert.True(t, persistedSchedule.NextRunAt.Equal(nextRunAt))
	})

	t.Run("preserves schedule, job, list, and cleanup operations around orchestration", func(t *testing.T) {
		store, service, _, connection, _, now := makeFixture(t, domain.ProviderConnectorIDEnableBanking)
		_, err := store.SaveTenantMembership(t.Context(), domain.TenantMembership{
			TenantID:  connection.TenantID,
			UserID:    connection.TenantID,
			JoinedAt:  now,
			CreatedAt: now,
		})
		require.NoError(t, err)
		publisher := NewMockSemanticCommandPublisher(t)
		var published SemanticCommand
		publisher.EXPECT().PublishSemanticCommand(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, command SemanticCommand) (DispatchReference, error) {
				published = command
				return DispatchReference{MessageID: "job-" + fake.UUID().V4()}, nil
			},
		).Once()
		WithBankSyncServiceCommandPublisher(publisher)(service)
		nextRunAt := now.Add(time.Hour)

		schedule, err := service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID:  connection.TenantID,
			TenantID:     connection.TenantID,
			ConnectionID: connection.ID,
			Interval:     time.Hour,
			NextRunAt:    nextRunAt,
		})
		require.NoError(t, err)
		assert.True(t, schedule.Enabled)

		paused, err := service.PauseBankConnectionSchedule(t.Context(), PauseBankConnectionScheduleParams{
			ActorUserID:  connection.TenantID,
			TenantID:     connection.TenantID,
			ConnectionID: connection.ID,
		})
		require.NoError(t, err)
		assert.False(t, paused.Enabled)

		resumed, err := service.ResumeBankConnectionSchedule(t.Context(), ResumeBankConnectionScheduleParams{
			ActorUserID:  connection.TenantID,
			TenantID:     connection.TenantID,
			ConnectionID: connection.ID,
			NextRunAt:    nextRunAt,
		})
		require.NoError(t, err)
		assert.True(t, resumed.Enabled)

		windowStart := now.Add(-time.Hour)
		job, err := service.TriggerBankConnectionSync(t.Context(), TriggerBankConnectionSyncParams{
			ActorUserID:  connection.TenantID,
			TenantID:     connection.TenantID,
			ConnectionID: connection.ID,
			Reason:       BankConnectionSyncReasonManual,
			WindowStart:  &windowStart,
			WindowEnd:    &now,
		})
		require.NoError(t, err)
		assert.Equal(t, BankConnectionSyncJobType, job.JobType)
		assert.Equal(t, BankConnectionSyncCommandTopic, published.Topic)
		var publishedPayload BankConnectionSyncCommand
		require.NoError(t, json.Unmarshal(published.Payload, &publishedPayload))
		assert.Equal(t, connection.ID, publishedPayload.ConnectionID)

		publisher.EXPECT().PublishSemanticCommand(mock.Anything, mock.Anything).Return(
			DispatchReference{}, assert.AnError,
		)
		_, err = service.TriggerBankConnectionSync(t.Context(), TriggerBankConnectionSyncParams{
			ActorUserID:  connection.TenantID,
			TenantID:     connection.TenantID,
			ConnectionID: connection.ID,
			Reason:       BankConnectionSyncReasonManual,
		})
		require.ErrorIs(t, err, assert.AnError)
		_, err = service.TriggerBankConnectionSync(t.Context(), TriggerBankConnectionSyncParams{
			ActorUserID:  connection.TenantID,
			TenantID:     connection.TenantID,
			ConnectionID: "missing-" + fake.UUID().V4(),
			Reason:       BankConnectionSyncReasonManual,
		})
		require.ErrorIs(t, err, ErrBankConnectionNotFound)

		connections, err := service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: connection.TenantID,
			TenantID:    connection.TenantID,
		})
		require.NoError(t, err)
		require.Len(t, connections, 1)

		require.NoError(t, service.DeleteBankConnection(t.Context(), DeleteBankConnectionParams{
			ActorUserID:  connection.TenantID,
			TenantID:     connection.TenantID,
			ConnectionID: connection.ID,
		}))
		deleted, err := store.GetBankConnection(t.Context(), connection.ID)
		require.ErrorIs(t, err, persistence.ErrBankConnectionNotFound)
		assert.Nil(t, deleted)
	})

	t.Run("serializes an idempotent link with deletion before removing owned state", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		linkPersistence := persistence.NewProviderLinkPersistence(store)
		now := time.Date(2026, time.August, 21, 12, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
		actorUserID := "actor-" + fake.UUID().V4()
		tenant, err := NewTenantService(store).CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "PLN",
		})
		require.NoError(t, err)
		connection := domain.BankConnection{
			ID:                "connection-" + fake.UUID().V4(),
			TenantID:          tenant.ID,
			Provider:          string(domain.ProviderIDPKO),
			ConnectorID:       domain.ProviderConnectorIDEnableBanking,
			ProviderReference: "reference-" + fake.UUID().V4(),
			SecretID:          "secret-" + fake.UUID().V4(),
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		secret := domain.ConnectionSecret{
			ID:        connection.SecretID,
			Provider:  connection.Provider,
			Reference: connection.ProviderReference,
			Envelope: credentials.Envelope{
				KeyVersion: "v1", Algorithm: "test", Nonce: "nonce", Ciphertext: "ciphertext",
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err = linkPersistence.SaveLinkedConnection(t.Context(), connection, secret)
		require.NoError(t, err)
		originalSnapshot := domain.ProviderSnapshot{
			ID:               "snapshot-" + fake.UUID().V4(),
			TenantID:         connection.TenantID,
			ConnectionID:     connection.ID,
			Subject:          domain.ProviderSnapshotSubjectConnection,
			Kind:             domain.ProviderSnapshotKindConnection,
			ProviderObjectID: connection.ProviderReference,
			DocumentJSON:     []byte(`{"session":"original"}`),
			CapturedAt:       now,
		}
		_, err = persistence.NewProviderSnapshotStoreFromStore(store).SaveProviderSnapshot(
			t.Context(), originalSnapshot,
		)
		require.NoError(t, err)
		nextRunAt := now.Add(24 * time.Hour)
		_, err = store.SaveBankConnectionSchedule(t.Context(), domain.BankConnectionSchedule{
			ConnectionID: connection.ID,
			Interval:     24 * time.Hour,
			NextRunAt:    &nextRunAt,
			Enabled:      true,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		require.NoError(t, err)

		linkLookupHeld := make(chan struct{})
		releaseLink := make(chan struct{})
		var releaseLinkOnce sync.Once
		var linkLookupBlocked atomic.Bool
		deleteAttempted := make(chan struct{})
		var deleteLockTimeout atomic.Bool
		lookupCallbackName := "hold-link-lookup-" + fake.UUID().V4()
		require.NoError(t, store.DB().Callback().Query().After("gorm:query").Register(
			lookupCallbackName,
			func(tx *gorm.DB) {
				if tx.Statement.Table != "finance_bank_connections" {
					return
				}
				if !linkLookupBlocked.CompareAndSwap(false, true) {
					return
				}
				close(linkLookupHeld)
				<-releaseLink
			},
		))
		t.Cleanup(func() {
			require.NoError(t, store.DB().Callback().Query().Remove(lookupCallbackName))
		})
		deleteCallbackName := "observe-connection-delete-" + fake.UUID().V4()
		require.NoError(t, store.DB().Callback().Delete().Before("gorm:delete").Register(
			deleteCallbackName,
			func(tx *gorm.DB) {
				if tx.Statement.Table != "finance_bank_connections" ||
					!deleteLockTimeout.CompareAndSwap(false, true) {
					return
				}
				if lockErr := tx.Exec("SET LOCAL lock_timeout = '100ms'").Error; lockErr != nil {
					tx.AddError(lockErr)
					return
				}
				close(deleteAttempted)
			},
		))
		t.Cleanup(func() {
			require.NoError(t, store.DB().Callback().Delete().Remove(deleteCallbackName))
		})

		retry := connection
		retry.ID = "connection-retry-" + fake.UUID().V4()
		retry.SecretID = "secret-retry-" + fake.UUID().V4()
		retrySecret := secret
		retrySecret.ID = retry.SecretID
		retrySnapshot := originalSnapshot
		retrySnapshot.ID = "snapshot-retry-" + fake.UUID().V4()
		retrySnapshot.DocumentJSON = []byte(`{"session":"retry"}`)
		retrySchedule := domain.BankConnectionSchedule{
			Interval: 24 * time.Hour, NextRunAt: &nextRunAt, Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
		linkDone := make(chan error, 1)
		deleteDone := make(chan error, 1)
		var linkStarted, linkFinished atomic.Bool
		var deleteStarted, deleteFinished atomic.Bool
		testContext, cancelTestContext := context.WithCancel(t.Context())
		releaseLinkAndWait := func() {
			releaseLinkOnce.Do(func() { close(releaseLink) })
		}
		waitForCompletion := func(name string, started, finished *atomic.Bool, done <-chan error) {
			if !started.Load() || finished.Load() {
				return
			}
			select {
			case <-done:
				finished.Store(true)
			case <-time.After(5 * time.Second):
				t.Errorf("%s goroutine did not finish during cleanup", name)
			}
		}
		t.Cleanup(func() {
			cancelTestContext()
			releaseLinkAndWait()
			waitForCompletion("link", &linkStarted, &linkFinished, linkDone)
			waitForCompletion("delete", &deleteStarted, &deleteFinished, deleteDone)
		})
		linkStarted.Store(true)
		go func() {
			_, linkErr := linkPersistence.SaveLinkedConnectionWithSnapshotAndSchedule(
				testContext, retry, retrySecret, &retrySnapshot, &retrySchedule,
			)
			linkFinished.Store(true)
			linkDone <- linkErr
		}()
		select {
		case <-linkLookupHeld:
		case <-time.After(5 * time.Second):
			t.Fatal("idempotent link did not reach its locked lookup")
		}

		orchestrator := newMockbankSyncOrchestrator(t)
		service := NewBankSyncService(
			store,
			orchestrator,
			WithBankSyncServiceSnapshotDeleter(persistence.NewProviderSnapshotStoreFromStore(store)),
		)
		deleteStarted.Store(true)
		go func() {
			deleteErr := service.DeleteBankConnection(testContext, DeleteBankConnectionParams{
				ActorUserID:  actorUserID,
				TenantID:     connection.TenantID,
				ConnectionID: connection.ID,
			})
			deleteFinished.Store(true)
			deleteDone <- deleteErr
		}()
		select {
		case <-deleteAttempted:
		case <-time.After(5 * time.Second):
			t.Fatal("delete did not reach the parent-row DELETE")
		}
		select {
		case deleteErr := <-deleteDone:
			var pgErr *pgconn.PgError
			require.ErrorAs(t, deleteErr, &pgErr)
			assert.Equal(t, "55P03", pgErr.Code, "DELETE must time out on the link's parent-row lock")
		case <-time.After(5 * time.Second):
			t.Fatal("DELETE did not time out while the link held the parent-row lock")
		}
		releaseLinkAndWait()
		select {
		case linkErr := <-linkDone:
			require.NoError(t, linkErr)
		case <-time.After(5 * time.Second):
			t.Fatal("idempotent link did not finish after its lock was released")
		}
		require.NoError(t, service.DeleteBankConnection(t.Context(), DeleteBankConnectionParams{
			ActorUserID:  actorUserID,
			TenantID:     connection.TenantID,
			ConnectionID: connection.ID,
		}))

		_, err = store.GetBankConnection(t.Context(), connection.ID)
		require.ErrorIs(t, err, persistence.ErrBankConnectionNotFound)
		_, err = store.GetBankConnectionSchedule(t.Context(), connection.ID)
		require.ErrorIs(t, err, persistence.ErrBankConnectionScheduleNotFound)
		snapshots, err := persistence.NewProviderSnapshotStoreFromStore(store).ListProviderSnapshotsByConnection(
			t.Context(), connection.ID,
		)
		require.NoError(t, err)
		assert.Empty(t, snapshots)
	})

	t.Run("validates schedule metadata and retains prior schedule projections", func(t *testing.T) {
		store, service, _, connection, _, now := makeFixture(t, domain.ProviderConnectorIDEnableBanking)
		_, err := store.SaveTenantMembership(t.Context(), domain.TenantMembership{
			TenantID: connection.TenantID, UserID: connection.TenantID, JoinedAt: now, CreatedAt: now,
		})
		require.NoError(t, err)
		nextRunAt := now.Add(time.Hour)
		first, err := service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID: connection.TenantID, TenantID: connection.TenantID, ConnectionID: connection.ID,
			Interval: time.Hour, NextRunAt: nextRunAt,
		})
		require.NoError(t, err)
		second, err := service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID: connection.TenantID, TenantID: connection.TenantID, ConnectionID: connection.ID,
			Interval: 2 * time.Hour, NextRunAt: nextRunAt.Add(time.Hour),
		})
		require.NoError(t, err)
		assert.True(t, first.CreatedAt.Equal(second.CreatedAt))

		zero := time.Time{}
		_, err = service.TriggerBankConnectionSync(t.Context(), TriggerBankConnectionSyncParams{WindowStart: &zero})
		require.Error(t, err)
		for _, params := range []RunBankConnectionSyncParams{
			{Reason: BankConnectionSyncReasonScheduled, ScheduledAt: &zero},
			{Reason: BankConnectionSyncReasonScheduled, ScheduledNextRunAt: &now},
			{Reason: BankConnectionSyncReasonScheduled, ScheduledAt: &now, ScheduledNextRunAt: &now},
		} {
			_, _, metadataErr := service.makeScheduledRunMetadata(t.Context(), connection, params, now)
			require.Error(t, metadataErr)
		}
		WithBankSyncServiceNow(func() time.Time { return now })(service)
		WithBankSyncServiceLogger(slog.New(slog.DiscardHandler))(service)
		WithBankSyncServiceSnapshotDeleter(persistence.NewProviderSnapshotStoreFromStore(store))(service)
		WithBankSyncServiceSyncStateJournalDeleter(persistence.NewProviderSyncStateJournalStore(store))(service)
	})

	t.Run("returns explicit lifecycle errors before orchestration", func(t *testing.T) {
		_, service, _, connection, _, _ := makeFixture(t, domain.ProviderConnectorIDEnableBanking)
		_, err := service.TriggerBankConnectionSync(t.Context(), TriggerBankConnectionSyncParams{})
		require.ErrorContains(t, err, "bank sync command publisher is required")
		_, err = service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: "missing-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrBankConnectionNotFound)
		_, err = service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: "outsider-" + fake.UUID().V4(), TenantID: connection.TenantID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = service.ListBankConnectionSyncedAccounts(t.Context(), ListBankConnectionSyncedAccountsParams{
			ActorUserID: "outsider-" + fake.UUID().V4(), TenantID: connection.TenantID, ConnectionID: connection.ID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})
}
