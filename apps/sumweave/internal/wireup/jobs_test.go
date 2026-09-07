package wireup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/appevents"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/financeapp"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:cyclop // Process-root integration keeps its coupled real fixture together.
func TestBuildProcessRoots(t *testing.T) {
	fake := faker.New()
	t.Chdir("../..")
	type observedCommand struct {
		Requester string `json:"requester"`
	}
	type phase3Transport struct {
		database       *sql.DB
		config         appdispatch.Config
		publisher      *appdispatch.Publisher
		eventPublisher *appevents.Publisher
		observedGroup  string
		automaticGroup string
		matchingGroup  string
	}
	type enrichmentFixture struct {
		event            financeapp.BankSyncWindowCompletedEvent
		classificationID string
		firstMatchingID  string
		secondMatchingID string
	}
	newTransport := func(t *testing.T) *phase3Transport {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		database, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, database.Close()) })
		transportConfig := appdispatch.Config{
			DatabaseDSN: dsn, TablePrefix: "sumweave_", PollInterval: time.Millisecond,
		}
		publisher, err := appdispatch.NewPublisher(transportConfig, database, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		eventPublisher, err := appevents.NewPublisher(publisher)
		require.NoError(t, err)
		return &phase3Transport{database: database, config: transportConfig, publisher: publisher,
			eventPublisher: eventPublisher, observedGroup: "jobs.workers.v1.phase3." + fake.UUID().V4(),
			automaticGroup: financeapp.ClassificationConsumerGroup + ".phase3." + fake.UUID().V4(),
			matchingGroup:  financeapp.TransferMatchingConsumerGroup + ".phase3." + fake.UUID().V4()}
	}
	checkpointConsumer := func(t *testing.T, transport *phase3Transport, consumerGroup, topic string) {
		t.Helper()
		var (
			offset        int64
			transactionID string
		)
		require.NoError(t, transport.database.QueryRowContext(
			t.Context(),
			`SELECT COALESCE(MAX("offset"), 0), COALESCE(MAX(transaction_id), '0'::xid8)::text
			FROM sumweave_app_dispatch_messages WHERE topic=$1`,
			topic,
		).Scan(&offset, &transactionID))
		_, err := transport.database.ExecContext(t.Context(), `
			INSERT INTO sumweave_app_dispatch_offsets
				(consumer_group, topic, offset_acked, last_processed_transaction_id)
			VALUES ($1, $2, $3, $4::xid8)
			ON CONFLICT (consumer_group, topic) DO UPDATE
			SET offset_acked=EXCLUDED.offset_acked, last_processed_transaction_id=EXCLUDED.last_processed_transaction_id`,
			consumerGroup,
			topic,
			offset,
			transactionID,
		)
		require.NoError(t, err)
	}
	newPhase3WorkerRoot := func(t *testing.T, transport *phase3Transport) *WorkerRoot {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		store, err := jobspkg.NewStore(transport.database, dsn, jobspkg.StoreOpts{TablePrefix: "sumweave_jobs_"})
		require.NoError(t, err)
		logger := slog.New(slog.DiscardHandler)
		routerFactory, err := appdispatch.NewRouterFactory(
			transport.config,
			transport.database,
			transport.publisher,
			logger,
		)
		require.NoError(t, err)
		registry := jobspkg.NewRegistry()
		worker, err := jobspkg.NewWorker(jobspkg.WorkerDeps{
			Store: store, Registry: registry, RouterFactory: routerFactory, Logger: logger,
			Config: jobspkg.WorkerConfig{PollInterval: 20 * time.Millisecond}, WorkerID: fake.UUID().V4(),
			ConsumerGroup: transport.observedGroup,
		})
		require.NoError(t, err)
		financeDatabase, err := financeapp.NewDatabase(transport.database, dsn, logger)
		require.NoError(t, err)
		values, err := config.LoadValues(config.ValuesLoadInput{Environment: "test"})
		require.NoError(t, err)
		rootConfig, err := values.WorkerRoot("test")
		require.NoError(t, err)
		financeModule, err := buildFinanceModule(financeModuleBuildDeps{
			Database: financeDatabase, CommandPublisher: transport.publisher,
			HTTPClientFactory: apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{RootLogger: logger}),
			Logger:            logger, JWTSigningKey: rootConfig.Auth.JWTSigningKey, Finance: rootConfig.Finance,
		})
		require.NoError(t, err)
		automaticRouter, err := routerFactory.NewRouter(transport.automaticGroup)
		require.NoError(t, err)
		require.NoError(t, financeapp.RegisterAutomaticClassificationHandler(
			automaticRouter, financeModule.ClassificationService, logger,
		))
		matchingRouter, err := routerFactory.NewRouter(transport.matchingGroup)
		require.NoError(t, err)
		require.NoError(t, financeapp.RegisterAutomaticTransferMatchingHandler(
			matchingRouter, financeModule.TransferMatchingService, logger,
		))
		shutdownHooks := lifecycle.NewShutdownHooks(lifecycle.ShutdownHooksDeps{
			RootLogger: logger, GracefulShutdownTimeout: time.Second,
		})
		root := &WorkerRoot{
			Worker:                          worker,
			Registry:                        registry,
			automaticClassificationRouter:   automaticRouter,
			automaticTransferMatchingRouter: matchingRouter,
			pollInterval:                    20 * time.Millisecond,
			shutdownHooks:                   shutdownHooks,
		}
		shutdownHooks.Register("phase3-worker-routers", root.Stop)
		return root
	}
	seedClassification := func(t *testing.T, database *sql.DB) (financeapp.BankSyncWindowCompletedEvent, string) {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		financeDatabase, err := financeapp.NewDatabase(database, dsn, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		store := persistence.NewStore(financeDatabase)
		now := time.Date(2026, time.September, 8, 14, 0, 0, 0, time.FixedZone("test", 2*60*60))
		tenant := domain.Tenant{
			ID: "tenant-" + fake.UUID().V4(), Name: "tenant-" + fake.Lorem().Word(),
			DisplayCurrency: "USD", CreatedAt: now, UpdatedAt: now,
		}
		_, err = store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)
		account := domain.Account{
			ID: "account-" + fake.UUID().V4(), TenantID: tenant.ID, Name: "account-" + fake.Lorem().Word(),
			Currency: "USD", Kind: domain.AccountKindManual, CreatedAt: now, UpdatedAt: now,
		}
		_, err = store.SaveAccount(t.Context(), account)
		require.NoError(t, err)
		category := domain.Category{
			ID: "category-" + fake.UUID().V4(), TenantID: tenant.ID, Name: "category-" + fake.Lorem().Word(),
			Kind: domain.CategoryKindExpense, CreatedAt: now, UpdatedAt: now,
		}
		_, err = store.SaveCategory(t.Context(), category)
		require.NoError(t, err)
		condition := "condition-" + fake.UUID().V4()
		ruleStore := persistence.NewClassificationRuleStore(financeDatabase)
		_, err = ruleStore.AppendClassificationRule(t.Context(), domain.ClassificationRule{
			ID: "rule-" + fake.UUID().V4(), TenantID: tenant.ID, CategoryID: category.ID,
			MatchType: domain.ClassificationMatchTypeContains, Condition: condition, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		transaction := domain.Transaction{
			ID: "transaction-" + fake.UUID().V4(), TenantID: tenant.ID, AccountID: account.ID,
			Source: domain.TransactionSourceProvider, Status: domain.TransactionStatusBooked,
			Kind: domain.TransactionKindRegular, AmountMinor: -1, Currency: "USD",
			Description: "description-" + condition, EffectiveAt: now, CreatedAt: now, UpdatedAt: now,
		}
		_, err = store.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)
		return financeapp.BankSyncWindowCompletedEvent{
			TenantID: tenant.ID, ConnectionID: "connection-" + fake.UUID().V4(),
			RangeStart: now.Add(-time.Hour), RangeEndExclusive: now.Add(time.Hour),
			SourceSyncMessageID: "sync-message-" + fake.UUID().V4(),
		}, transaction.ID
	}
	seedEnrichments := func(t *testing.T, database *sql.DB) enrichmentFixture {
		t.Helper()
		event, classificationID := seedClassification(t, database)
		financeDatabase, err := financeapp.NewDatabase(
			database,
			os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN"),
			slog.New(slog.DiscardHandler),
		)
		require.NoError(t, err)
		store := persistence.NewStore(financeDatabase)
		classification, err := store.GetTransaction(t.Context(), classificationID)
		require.NoError(t, err)
		require.NotNil(t, classification)
		account := domain.Account{
			ID:        "account-" + fake.UUID().V4(),
			TenantID:  classification.TenantID,
			Name:      "account-" + fake.Lorem().Word(),
			Currency:  classification.Currency,
			Kind:      domain.AccountKindManual,
			CreatedAt: classification.CreatedAt,
			UpdatedAt: classification.UpdatedAt,
		}
		_, err = store.SaveAccount(t.Context(), account)
		require.NoError(t, err)
		firstMatching := domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    classification.TenantID,
			AccountID:   classification.AccountID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -2,
			Currency:    classification.Currency,
			Description: "matching-" + fake.UUID().V4(),
			EffectiveAt: classification.EffectiveAt,
			CreatedAt:   classification.CreatedAt,
			UpdatedAt:   classification.UpdatedAt,
		}
		_, err = store.SaveTransaction(t.Context(), firstMatching)
		require.NoError(t, err)
		secondMatching := domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    classification.TenantID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: 2,
			Currency:    classification.Currency,
			Description: "matching-" + fake.UUID().V4(),
			EffectiveAt: classification.EffectiveAt.Add(time.Hour),
			CreatedAt:   classification.CreatedAt,
			UpdatedAt:   classification.UpdatedAt,
		}
		_, err = store.SaveTransaction(t.Context(), secondMatching)
		require.NoError(t, err)
		eligible, err := persistence.NewTransferPairStore(financeDatabase).ListEligibleTransferMatchingTransactions(
			t.Context(),
			persistence.ListEligibleTransferMatchingTransactionsParams{
				TenantID:          event.TenantID,
				RangeStart:        event.RangeStart,
				RangeEndExclusive: event.RangeEndExclusive,
			},
		)
		require.NoError(t, err)
		require.ElementsMatch(t, []persistence.TransferMatchingTransaction{
			{
				ID:          classification.ID,
				AccountID:   classification.AccountID,
				Currency:    classification.Currency,
				AmountMinor: classification.AmountMinor,
				EffectiveAt: classification.EffectiveAt,
			},
			{
				ID:          firstMatching.ID,
				AccountID:   firstMatching.AccountID,
				Currency:    firstMatching.Currency,
				AmountMinor: firstMatching.AmountMinor,
				EffectiveAt: firstMatching.EffectiveAt,
			},
			{
				ID:          secondMatching.ID,
				AccountID:   secondMatching.AccountID,
				Currency:    secondMatching.Currency,
				AmountMinor: secondMatching.AmountMinor,
				EffectiveAt: secondMatching.EffectiveAt,
			},
		}, eligible)
		return enrichmentFixture{
			event:            event,
			classificationID: classificationID,
			firstMatchingID:  firstMatching.ID,
			secondMatchingID: secondMatching.ID,
		}
	}
	enrichmentsComplete := func(database *sql.DB, fixture enrichmentFixture) (bool, error) {
		financeDatabase, err := financeapp.NewDatabase(
			database,
			os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN"),
			slog.New(slog.DiscardHandler),
		)
		if err != nil {
			return false, fmt.Errorf("open finance database: %w", err)
		}
		store := persistence.NewStore(financeDatabase)
		classification, err := store.GetTransaction(t.Context(), fixture.classificationID)
		if err != nil {
			return false, fmt.Errorf("get classification transaction: %w", err)
		}
		if classification == nil {
			return false, errors.New("classification transaction is missing")
		}
		firstMatching, err := store.GetTransaction(t.Context(), fixture.firstMatchingID)
		if err != nil {
			return false, fmt.Errorf("get first matching transaction: %w", err)
		}
		if firstMatching == nil {
			return false, errors.New("first matching transaction is missing")
		}
		secondMatching, err := store.GetTransaction(t.Context(), fixture.secondMatchingID)
		if err != nil {
			return false, fmt.Errorf("get second matching transaction: %w", err)
		}
		if secondMatching == nil {
			return false, errors.New("second matching transaction is missing")
		}
		return classification.CategoryID != nil && firstMatching.TransferGroupID != nil &&
			secondMatching.TransferGroupID != nil && *firstMatching.TransferGroupID == *secondMatching.TransferGroupID, nil
	}
	waitForEnrichments := func(t *testing.T, database *sql.DB, fixture enrichmentFixture) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		var lastErr error
		for time.Now().Before(deadline) {
			complete, err := enrichmentsComplete(database, fixture)
			if err != nil {
				lastErr = err
			} else if complete {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		require.NoError(t, lastErr)
		t.Fatal("timed out waiting for classification and transfer matching")
	}
	registerObservedCommand := func(t *testing.T, root *WorkerRoot, topic string, run func(context.Context) error) {
		t.Helper()
		require.NoError(t, jobspkg.RegisterTypedHandler(root.Registry, jobspkg.TypedHandlerSpec[observedCommand]{
			JobType: jobspkg.JobType("finance.phase3." + fake.UUID().V4()), Topic: topic,
			Metadata: func(value observedCommand) (jobspkg.JobMetadata, error) {
				return jobspkg.JobMetadata{JobType: "finance.phase3", Requester: jobspkg.Requester{
					UserID: value.Requester, Source: jobspkg.RequesterSourceOperator,
				}}, nil
			},
			Run: func(ctx context.Context, _ jobspkg.Job, _ observedCommand) error { return run(ctx) },
		}))
	}
	publishObservedCommand := func(t *testing.T, publisher *appdispatch.Publisher, topic string) {
		t.Helper()
		payload, err := json.Marshal(observedCommand{Requester: "user-" + fake.UUID().V4()})
		require.NoError(t, err)
		require.NoError(t, publisher.Publish(t.Context(), appdispatch.NewMessage(topic, payload)))
	}
	storedTransaction := func(t *testing.T, database *sql.DB, transactionID string) domain.Transaction {
		t.Helper()
		financeDatabase, err := financeapp.NewDatabase(
			database,
			os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN"),
			slog.New(slog.DiscardHandler),
		)
		require.NoError(t, err)
		transaction, err := persistence.NewStore(financeDatabase).GetTransaction(t.Context(), transactionID)
		require.NoError(t, err)
		require.NotNil(t, transaction)
		return *transaction
	}

	t.Run("worker builds observed finance handlers without HTTP or scheduler", func(t *testing.T) {
		root, err := BuildWorker(t.Context(), WorkerOptions{Environment: "test"})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, root.Close(t.Context())) })
		require.NotNil(t, root.Worker)
		require.NotNil(t, root.Registry)
		require.NotNil(t, root.automaticClassificationRouter)
		require.NotNil(t, root.automaticTransferMatchingRouter)
		for _, topic := range []string{
			financepkg.FXRatesRefreshCommandTopic,
			financepkg.TransactionCSVImportCommandTopic,
			financepkg.BankConnectionSyncCommandTopic,
		} {
			_, handlerErr := root.Registry.Handler(topic)
			require.NoError(t, handlerErr)
		}
	})

	t.Run(
		"normal lifecycle runs observed jobs and both real automatic enrichments then closes consumers",
		func(t *testing.T) {
			transport := newTransport(t)
			root := newPhase3WorkerRoot(t, transport)
			t.Cleanup(func() { require.NoError(t, root.Close(t.Context())) })
			checkpointConsumer(t, transport, transport.automaticGroup, financeapp.BankSyncWindowCompletedEventTopic)
			checkpointConsumer(t, transport, transport.matchingGroup, financeapp.BankSyncWindowCompletedEventTopic)
			fixture := seedEnrichments(t, transport.database)
			observedTopic := "finance.phase3.normal." + fake.UUID().V4()
			observed := make(chan struct{}, 1)
			closedTopic := "finance.phase3.closed." + fake.UUID().V4()
			closedObserved := make(chan struct{}, 1)
			registerObservedCommand(t, root, observedTopic, func(context.Context) error {
				observed <- struct{}{}
				return nil
			})
			registerObservedCommand(t, root, closedTopic, func(context.Context) error {
				closedObserved <- struct{}{}
				return nil
			})
			runCtx, cancel := context.WithCancel(t.Context())
			runDone := make(chan error, 1)
			go func() { runDone <- root.Run(runCtx) }()
			time.Sleep(100 * time.Millisecond)
			publishObservedCommand(t, transport.publisher, observedTopic)
			require.NoError(t, transport.eventPublisher.Publish(t.Context(), fixture.event))
			select {
			case <-observed:
			case <-time.After(8 * time.Second):
				t.Fatal("observed worker did not process its command")
			}
			waitForEnrichments(t, transport.database, fixture)
			cancel()
			require.NoError(t, <-runDone)
			require.NoError(t, root.Close(t.Context()))

			closedEvent, closedTransactionID := seedClassification(t, transport.database)
			publishObservedCommand(t, transport.publisher, closedTopic)
			require.NoError(t, transport.eventPublisher.Publish(t.Context(), closedEvent))
			time.Sleep(50 * time.Millisecond)
			select {
			case <-closedObserved:
				t.Fatal("closed observed worker consumed a message")
			default:
			}
			assert.Nil(t, storedTransaction(t, transport.database, closedTransactionID).CategoryID)
		},
	)

	t.Run("once drains observed work before its emitted automatic event", func(t *testing.T) {
		transport := newTransport(t)
		fixture := seedEnrichments(t, transport.database)
		observedTopic := "finance.phase3.once." + fake.UUID().V4()
		checkpointConsumer(t, transport, transport.observedGroup, observedTopic)
		checkpointConsumer(t, transport, transport.automaticGroup, financeapp.BankSyncWindowCompletedEventTopic)
		checkpointConsumer(t, transport, transport.matchingGroup, financeapp.BankSyncWindowCompletedEventTopic)

		root := newPhase3WorkerRoot(t, transport)
		t.Cleanup(func() { require.NoError(t, root.Close(t.Context())) })
		emitted := make(chan struct{}, 1)
		registerObservedCommand(t, root, observedTopic, func(ctx context.Context) error {
			if err := transport.eventPublisher.Publish(ctx, fixture.event); err != nil {
				return err
			}
			emitted <- struct{}{}
			return nil
		})
		publishObservedCommand(t, transport.publisher, observedTopic)
		time.Sleep(50 * time.Millisecond)
		require.NoError(t, root.RunOnce(t.Context()))
		select {
		case <-emitted:
		default:
			t.Fatal("observed worker did not emit the automatic classification event")
		}
		complete, err := enrichmentsComplete(transport.database, fixture)
		require.NoError(t, err)
		require.True(t, complete)
	})

	t.Run("once waits for a long automatic-classification router delivery before idle drain", func(t *testing.T) {
		transport := newTransport(t)
		slowTopic := "finance.phase3.automatic-classification.slow." + fake.UUID().V4()
		checkpointConsumer(t, transport, transport.automaticGroup, slowTopic)
		root := newPhase3WorkerRoot(t, transport)
		t.Cleanup(func() { require.NoError(t, root.Close(t.Context())) })
		started := make(chan struct{}, 1)
		release := make(chan struct{})
		handler, handlerErr := appdispatch.NewHandler(
			slowTopic,
			func(ctx context.Context, _ appdispatch.Message) error {
				started <- struct{}{}
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		)
		require.NoError(t, handlerErr)
		require.NoError(t, root.automaticClassificationRouter.Handle(handler))
		require.NoError(t, transport.publisher.Publish(
			t.Context(),
			appdispatch.NewMessage(slowTopic, []byte(fake.UUID().V4())),
		))
		runDone := make(chan error, 1)
		go func() { runDone <- root.RunOnce(t.Context()) }()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("automatic classification router did not start its delivery")
		}
		select {
		case err := <-runDone:
			t.Fatalf("RunOnce stopped active automatic classification handling: %v", err)
		case <-time.After(5 * root.pollInterval):
		}
		close(release)
		select {
		case err := <-runDone:
			t.Fatalf("RunOnce skipped the automatic router idle drain: %v", err)
		case <-time.After(root.pollInterval / 2):
		}
		select {
		case err := <-runDone:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("RunOnce did not finish after automatic classification became idle")
		}
	})

	t.Run(
		"committed events survive consumer restart and duplicate delivery skips the categorized transaction",
		func(t *testing.T) {
			transport := newTransport(t)
			checkpointConsumer(t, transport, transport.automaticGroup, financeapp.BankSyncWindowCompletedEventTopic)
			event, transactionID := seedClassification(t, transport.database)
			require.NoError(t, transport.eventPublisher.Publish(t.Context(), event))
			time.Sleep(50 * time.Millisecond)
			restartedRoot := newPhase3WorkerRoot(t, transport)
			require.NoError(t, restartedRoot.RunOnce(t.Context()))
			first := storedTransaction(t, transport.database, transactionID)
			require.NotNil(t, first.CategoryID)
			require.NoError(t, restartedRoot.Close(t.Context()))
			require.NoError(t, transport.eventPublisher.Publish(t.Context(), event))
			duplicateRoot := newPhase3WorkerRoot(t, transport)
			t.Cleanup(func() { require.NoError(t, duplicateRoot.Close(t.Context())) })
			require.NoError(t, duplicateRoot.RunOnce(t.Context()))
			second := storedTransaction(t, transport.database, transactionID)
			assert.Equal(t, first.CategoryID, second.CategoryID)
			assert.True(t, first.UpdatedAt.Equal(second.UpdatedAt))
		},
	)

	t.Run("scheduler builds prepared finance schedules without worker or HTTP", func(t *testing.T) {
		root, err := BuildScheduler(t.Context(), SchedulerOptions{Environment: "test"})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, root.Close(t.Context())) })
		require.NotNil(t, root)
		_, err = root.EnqueueDue(t.Context())
		require.NoError(t, err)
	})

	t.Run("rejects root settings before opening process resources", func(t *testing.T) {
		_, err := BuildWorker(t.Context(), WorkerOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)
		_, err = BuildScheduler(t.Context(), SchedulerOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)

		t.Setenv("APP_APPLICATION_DATABASE_DSN", "")
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", "")
		values, err := config.LoadValues(config.ValuesLoadInput{Environment: "production"})
		require.NoError(t, err)
		_, err = values.WorkerRoot("production")
		require.ErrorContains(t, err, "application database dsn")
	})
}
