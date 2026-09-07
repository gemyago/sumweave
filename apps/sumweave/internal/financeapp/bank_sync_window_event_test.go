package financeapp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/appevents"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBankSyncWindowCompletedEvent(t *testing.T) {
	fake := faker.New()
	config := appdispatch.Config{
		DatabaseDSN: os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN"), TablePrefix: "sumweave_",
		PollInterval: 10 * time.Millisecond,
	}
	require.NotEmpty(t, config.DatabaseDSN)
	db, err := sql.Open("pgx", config.DatabaseDSN)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	logger := slog.New(slog.DiscardHandler)
	rawPublisher, err := appdispatch.NewPublisher(config, db, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rawPublisher.Close()) })

	makeEvent := func() BankSyncWindowCompletedEvent {
		start := time.Date(2026, time.August, 20, 7, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
		return BankSyncWindowCompletedEvent{
			TenantID: "tenant-" + fake.UUID().V4(), ConnectionID: "connection-" + fake.UUID().V4(),
			RangeStart: start, RangeEndExclusive: start.Add(24 * time.Hour),
			SourceSyncMessageID: "sync-message-" + fake.UUID().V4(),
		}
	}
	makeRouter := func(t *testing.T) *appdispatch.Router {
		t.Helper()
		factory, factoryErr := appdispatch.NewRouterFactory(config, db, rawPublisher, logger)
		require.NoError(t, factoryErr)
		router, routerErr := factory.NewRouter(ClassificationConsumerGroup + ".phase3-test")
		require.NoError(t, routerErr)
		return router
	}
	checkpointTopic := func(t *testing.T, consumerGroup string) {
		t.Helper()
		var (
			offset        int64
			transactionID string
		)
		require.NoError(t, db.QueryRowContext(t.Context(), `
			SELECT COALESCE(MAX("offset"), 0), COALESCE(MAX(transaction_id), '0'::xid8)::text
			FROM sumweave_app_dispatch_messages WHERE topic=$1`, BankSyncWindowCompletedEventTopic,
		).Scan(&offset, &transactionID))
		_, execErr := db.ExecContext(t.Context(), `
			INSERT INTO sumweave_app_dispatch_offsets (consumer_group, topic, offset_acked, last_processed_transaction_id)
			VALUES ($1, $2, $3, $4::xid8)
			ON CONFLICT (consumer_group, topic) DO UPDATE
			SET offset_acked=EXCLUDED.offset_acked, last_processed_transaction_id=EXCLUDED.last_processed_transaction_id`,
			consumerGroup, BankSyncWindowCompletedEventTopic, offset, transactionID,
		)
		require.NoError(t, execErr)
	}
	jobCount := func(t *testing.T) int {
		t.Helper()
		var tableName sql.NullString
		require.NoError(t, db.QueryRowContext(t.Context(), `SELECT to_regclass('sumweave_jobs')`).Scan(&tableName))
		if !tableName.Valid {
			return 0
		}
		var count int
		require.NoError(t, db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sumweave_jobs`).Scan(&count))
		return count
	}
	committedEventCount := func(t *testing.T) int {
		t.Helper()
		var count int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM sumweave_app_dispatch_messages WHERE topic=$1`,
			BankSyncWindowCompletedEventTopic,
		).Scan(&count))
		return count
	}
	ledgerTransactionCount := func(t *testing.T, tenantID string) int {
		t.Helper()
		var count int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM finance_transactions WHERE tenant_id=$1`,
			tenantID,
		).Scan(&count))
		return count
	}
	successfulCheckpointCount := func(t *testing.T, connectionID string) int {
		t.Helper()
		var count int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM finance_provider_sync_state_journal_records
				WHERE connection_id=$1 AND succeeded_at IS NOT NULL`,
			connectionID,
		).Scan(&count))
		return count
	}

	t.Run("transaction-bound event publication leaves no durable row after window rollback", func(t *testing.T) {
		eventPublisher, publisherErr := appevents.NewPublisher(rawPublisher)
		require.NoError(t, publisherErr)
		countBefore := committedEventCount(t)
		tx, beginErr := db.BeginTx(t.Context(), nil)
		require.NoError(t, beginErr)
		require.NoError(t, eventPublisher.PublishInTx(t.Context(), tx, makeEvent()))
		require.NoError(t, tx.Rollback())
		assert.Equal(t, countBefore, committedEventCount(t))
	})

	t.Run("rolls back requested-window ledger checkpoint and event when publication fails", func(t *testing.T) {
		now := time.Date(2026, time.September, 6, 9, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
		financeDatabase, databaseErr := persistence.NewDatabase(db, config.DatabaseDSN)
		require.NoError(t, databaseErr)
		financeStore := persistence.NewStore(financeDatabase)
		key := sha256.Sum256([]byte("phase3-window-rollback-" + fake.UUID().V4()))
		cipher, cipherErr := credentials.NewAESGCMCipher(key[:], "phase3-window-rollback")
		require.NoError(t, cipherErr)
		eventPublisher, eventPublisherErr := appevents.NewPublisher(rawPublisher)
		require.NoError(t, eventPublisherErr)
		publisherFailure := errors.New("window-event-publish-" + fake.UUID().V4())
		publisher := NewMockBankSyncWindowCompletionPublisher(t)
		publisher.EXPECT().
			PublishBankSyncWindowCompleted(mock.Anything, mock.Anything, mock.Anything).
			RunAndReturn(func(
				ctx context.Context,
				tx *sql.Tx,
				fact domain.BankSyncWindowCompleted,
			) error {
				require.NoError(t, eventPublisher.PublishInTx(ctx, tx, BankSyncWindowCompletedEvent{
					TenantID: fact.TenantID, ConnectionID: fact.ConnectionID,
					RangeStart: fact.RangeStart, RangeEndExclusive: fact.RangeEndExclusive,
					SourceSyncMessageID: fact.SourceSyncMessageID,
				}))
				return publisherFailure
			}).
			Once()
		financeModule, financeErr := financepkg.New(&financepkg.Config{
			Database: financeDatabase, Logger: logger, Now: func() time.Time { return now },
			NewID: uuid.NewString, HTTPClient: http.DefaultClient, ConnectionSecretCipher: cipher,
			BankSyncWindowPublisher: publisher,
			Monobank:                financepkg.MonobankConfig{BaseURL: "https://" + fake.Internet().Domain()},
			EnableBanking: financepkg.EnableBankingConfig{
				BaseURL: "https://" + fake.Internet().Domain(), AppID: "app-" + fake.UUID().V4(),
				PrivateKeyPath: "key-" + fake.UUID().V4() + ".pem",
				ASPSPs: []financepkg.EnableBankingASPSP{{
					ProviderID: domain.ProviderIDPKO, Name: "bank-" + fake.Company().Name(),
					Country: "PL", PSUType: "personal", ValidDays: 90,
				}},
			},
		})
		require.NoError(t, financeErr)

		ownerID := "owner-" + fake.UUID().V4()
		tenant, tenantErr := financeModule.TenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID: ownerID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "PLN",
		})
		require.NoError(t, tenantErr)
		secretEnvelope, secretErr := cipher.SealString("synthetic-secret-" + fake.UUID().V4())
		require.NoError(t, secretErr)
		secret, saveSecretErr := financeStore.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID: "secret-" + fake.UUID().V4(), Provider: string(domain.ProviderIDSynthetic),
			Reference: "reference-" + fake.UUID().V4(), Envelope: secretEnvelope, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, saveSecretErr)
		providerReference := "synthetic-reference-" + fake.UUID().V4()
		_, stateErr := persistence.NewSyntheticProviderStateStoreFromStore(financeStore).SaveSyntheticProviderState(
			t.Context(),
			domain.SyntheticProviderState{
				ProviderReference: providerReference,
				Envelope: domain.SyntheticProviderStateEnvelope{
					ConfiguredAccounts: []domain.SyntheticConfiguredAccount{{
						Key:      "account-" + fake.UUID().V4(),
						Name:     "Synthetic " + fake.Lorem().Word(),
						Currency: "PLN",
					}},
				},
				CreatedAt: now, UpdatedAt: now,
			},
		)
		require.NoError(t, stateErr)
		connection, connectionErr := financeStore.SaveBankConnection(t.Context(), domain.BankConnection{
			ID: "connection-" + fake.UUID().V4(), TenantID: tenant.ID, Provider: string(domain.ProviderIDSynthetic),
			ConnectorID: domain.ProviderConnectorIDSynthetic, ProviderReference: providerReference, SecretID: secret.ID,
			State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, connectionErr)
		ledgerCountBefore := ledgerTransactionCount(t, tenant.ID)
		successfulCheckpointCountBefore := successfulCheckpointCount(t, connection.ID)
		eventCountBefore := committedEventCount(t)
		windowStart := now.Add(-24 * time.Hour)

		syncParams := financepkg.RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-" + fake.UUID().V4(),
			Reason:       financepkg.BankConnectionSyncReasonManual,
			WindowStart:  &windowStart,
			WindowEnd:    &now,
		}
		_, syncErr := financeModule.BankSyncService.RunBankConnectionSync(t.Context(), syncParams)
		require.ErrorIs(t, syncErr, publisherFailure)

		assert.Equal(t, ledgerCountBefore, ledgerTransactionCount(t, tenant.ID))
		mappings, listMappingsErr := financeStore.ListConnectionProviderAccounts(t.Context(), connection.ID)
		require.NoError(t, listMappingsErr)
		assert.Empty(t, mappings)
		assert.Equal(t, successfulCheckpointCountBefore, successfulCheckpointCount(t, connection.ID))
		assert.Equal(t, eventCountBefore, committedEventCount(t))
	})

	t.Run("serializes exact requested bounds and classifies duplicate deliveries without jobs", func(t *testing.T) {
		event := makeEvent()
		payload, marshalErr := json.Marshal(event)
		require.NoError(t, marshalErr)
		var decoded BankSyncWindowCompletedEvent
		require.NoError(t, json.Unmarshal(payload, &decoded))
		assert.Equal(t, event.TenantID, decoded.TenantID)
		assert.Equal(t, event.ConnectionID, decoded.ConnectionID)
		assert.True(t, event.RangeStart.Equal(decoded.RangeStart))
		assert.True(t, event.RangeEndExclusive.Equal(decoded.RangeEndExclusive))
		assert.Equal(t, event.SourceSyncMessageID, decoded.SourceSyncMessageID)

		service := newMockclassificationJobService(t)
		delivered := make(chan financepkg.ClassificationParams, 2)
		service.EXPECT().
			Classify(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.ClassificationParams) (financepkg.ClassificationAttemptCounts, error) {
				if params.SourceSyncMessageID == event.SourceSyncMessageID {
					delivered <- params
				}
				return financepkg.ClassificationAttemptCounts{}, nil
			}).
			Maybe()
		router := makeRouter(t)
		jobCountBefore := jobCount(t)
		require.NoError(t, RegisterAutomaticClassificationHandler(router, service, logger))
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(func() {
			cancel()
			require.NoError(t, router.Close())
		})
		go func() { _ = router.Run(ctx) }()
		time.Sleep(100 * time.Millisecond)
		publisher, publisherErr := appevents.NewPublisher(rawPublisher)
		require.NoError(t, publisherErr)
		require.NoError(t, publisher.Publish(t.Context(), event))
		require.NoError(t, publisher.Publish(t.Context(), event))

		for range 2 {
			select {
			case params := <-delivered:
				assert.Equal(t, event.TenantID, params.TenantID)
				assert.True(t, event.RangeStart.Equal(params.RangeStart))
				assert.True(t, event.RangeEndExclusive.Equal(params.RangeEndExclusive))
				assert.Equal(t, event.SourceSyncMessageID, params.SourceSyncMessageID)
				assert.NotEmpty(t, params.MessageID)
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for automatic classification")
			}
		}

		assert.Equal(t, jobCountBefore, jobCount(t))
	})

	t.Run("matches exact requested bounds for every independent automatic delivery without jobs", func(t *testing.T) {
		event := makeEvent()
		service := newMocktransferMatchingJobService(t)
		delivered := make(chan financepkg.TransferMatchingParams, 2)
		service.EXPECT().
			Match(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.TransferMatchingParams) (financepkg.TransferMatchingAttemptCounts, error) {
				if params.SourceSyncMessageID == event.SourceSyncMessageID {
					delivered <- params
				}
				return financepkg.TransferMatchingAttemptCounts{}, nil
			}).
			Maybe()
		factory, factoryErr := appdispatch.NewRouterFactory(config, db, rawPublisher, logger)
		require.NoError(t, factoryErr)
		consumerGroup := TransferMatchingConsumerGroup + ".phase3-test." + fake.UUID().V4()
		router, routerErr := factory.NewRouter(consumerGroup)
		require.NoError(t, routerErr)
		checkpointTopic(t, consumerGroup)
		jobCountBefore := jobCount(t)
		require.NoError(t, RegisterAutomaticTransferMatchingHandler(router, service, logger))
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(func() {
			cancel()
			require.NoError(t, router.Close())
		})
		go func() { _ = router.Run(ctx) }()
		time.Sleep(100 * time.Millisecond)
		publisher, publisherErr := appevents.NewPublisher(rawPublisher)
		require.NoError(t, publisherErr)
		require.NoError(t, publisher.Publish(t.Context(), event))
		require.NoError(t, publisher.Publish(t.Context(), event))

		for range 2 {
			select {
			case params := <-delivered:
				assert.Equal(t, event.TenantID, params.TenantID)
				assert.True(t, event.RangeStart.Equal(params.RangeStart))
				assert.True(t, event.RangeEndExclusive.Equal(params.RangeEndExclusive))
				assert.Equal(t, event.SourceSyncMessageID, params.SourceSyncMessageID)
				assert.NotEmpty(t, params.MessageID)
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for automatic transfer matching")
			}
		}
		assert.Equal(t, jobCountBefore, jobCount(t))
	})

	t.Run("retries malformed and terminal events into the dead-letter stream without jobs", func(t *testing.T) {
		router := makeRouter(t)
		jobCountBefore := jobCount(t)
		terminalErr := financepkg.NewTerminalFailure(errors.New("missing category"), "missing_category", "missing", "")
		valid := makeEvent()
		service := newMockclassificationJobService(t)
		service.EXPECT().
			Classify(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, params financepkg.ClassificationParams) (financepkg.ClassificationAttemptCounts, error) {
				if params.SourceSyncMessageID == valid.SourceSyncMessageID {
					return financepkg.ClassificationAttemptCounts{}, terminalErr
				}
				return financepkg.ClassificationAttemptCounts{}, nil
			}).
			Maybe()
		require.NoError(t, RegisterAutomaticClassificationHandler(router, service, logger))
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(func() {
			cancel()
			require.NoError(t, router.Close())
		})
		go func() { _ = router.Run(ctx) }()
		time.Sleep(100 * time.Millisecond)
		malformed := appdispatch.NewMessage(BankSyncWindowCompletedEventTopic, []byte("not-json"))
		require.NoError(t, rawPublisher.Publish(t.Context(), malformed))
		publisher, publisherErr := appevents.NewPublisher(rawPublisher)
		require.NoError(t, publisherErr)
		require.NoError(t, publisher.Publish(t.Context(), valid))
		const deadLetterCountQuery = `SELECT COUNT(*) FROM sumweave_app_dispatch_messages
			WHERE topic=$1 AND metadata->>$2=$3`
		require.Eventually(t, func() bool {
			var count int
			err = db.QueryRowContext(
				t.Context(), deadLetterCountQuery, appdispatch.DeadLetterTopic,
				middleware.PoisonedTopicKey, BankSyncWindowCompletedEventTopic,
			).Scan(&count)
			return err == nil && count >= 2
		}, 8*time.Second, 50*time.Millisecond)
		assert.Equal(t, jobCountBefore, jobCount(t))
	})
}
