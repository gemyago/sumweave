package financeapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type publisherStub struct{}

func (publisherStub) PublishInTx(context.Context, *sql.Tx, appdispatch.Message) error { return nil }

type financeAppRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f financeAppRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestNewFinanceDatabase(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-store-"+t.Name())
	sharedDB, err := sqlconn.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sharedDB.Close()) })

	database, err := NewDatabase(sharedDB, dsn, slog.New(slog.DiscardHandler))
	require.NoError(t, err)

	store := persistence.NewStore(database)
	_, err = store.ListTenantsForUser(t.Context(), "user-no-auto-migrate")
	require.Error(t, err)
	require.ErrorContains(t, err, "no such table")
}

func TestNewMonobankHTTPClient(t *testing.T) {
	makeFactory := func() *apphttpclient.ClientFactory {
		return apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{
			RootLogger: slog.New(slog.DiscardHandler),
		})
	}

	t.Run("uses a fallback-aware timeout budget", func(t *testing.T) {
		fallbackDelay := 61 * time.Second

		client, err := newMonobankHTTPClient(makeFactory(), fallbackDelay)

		require.NoError(t, err)
		assert.Equal(t, fallbackDelay+2*30*time.Second+time.Second, client.Timeout)
	})

	t.Run("rejects non-positive fallback delays", func(t *testing.T) {
		client, err := newMonobankHTTPClient(makeFactory(), 0)

		require.ErrorContains(t, err, "fallback delay must be positive")
		assert.Nil(t, client)
	})
}

func TestFinanceModuleHelpers(t *testing.T) {
	t.Run("handles optional and invalid constructor inputs", func(t *testing.T) {
		cipher, err := makeFinanceCipher("")
		require.NoError(t, err)
		_, err = cipher.SealString("secret")
		require.Error(t, err)
		_, err = cipher.OpenString(credentials.Envelope{})
		require.Error(t, err)

		_, err = NewDatabase(nil, "", slog.New(slog.DiscardHandler))
		require.Error(t, err)
		_, err = newFinanceHTTPClient(nil)
		require.Error(t, err)
		_, err = newMonobankHTTPClient(nil, time.Second)
		require.Error(t, err)
		_, err = newMonobankHTTPClient(
			apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{RootLogger: slog.New(slog.DiscardHandler)}),
			0,
		)
		require.Error(t, err)
		_, err = NewModule(ModuleDeps{})
		require.Error(t, err)
		assert.NotNil(t, resolveFinanceLogger(nil))
		assert.Equal(t, "https://api.monobank.ua", resolveMonobankBaseURL(" "))
	})

	t.Run("keeps absent behavior collaborators optional", func(t *testing.T) {
		registry := jobspkg.NewRegistry()
		require.NoError(t, registerFinanceJobHandlers(nil, nil, nil, nil))
		require.NoError(t, registerFXRefreshJobHandler(registry, nil))
		require.NoError(t, registerCSVImportJobHandler(registry, "csv", nil))
		require.NoError(t, registerBankSyncJobHandler(registry, nil))
		require.NoError(t, bankConnectionSyncScheduleWriter{}.UpsertBankConnectionSyncSchedule(
			t.Context(),
			financepkg.BankConnectionSyncSchedule{},
		))
		require.NoError(t, fxRefreshScheduleWriter{}.UpsertFXRefreshSchedule(
			t.Context(),
			financepkg.FXRefreshSchedule{},
		))
	})

	t.Run("wraps schedule writers around durable storage", func(t *testing.T) {
		dsn := fmt.Sprintf("file:finance-schedule-writers-%s?mode=memory&cache=shared", faker.New().UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := jobspkg.NewStore(db, dsn, jobspkg.StoreOpts{TablePrefix: "writers_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, bankConnectionSyncScheduleWriter{store: store}.UpsertBankConnectionSyncSchedule(
			t.Context(),
			financepkg.BankConnectionSyncSchedule{
				ScheduleID:   "bank-sync",
				ConnectionID: "connection",
				ActorUserID:  "actor",
				Interval:     time.Hour,
				Enabled:      true,
			},
		))
		written, err := store.GetSchedule(t.Context(), "bank-sync")
		require.NoError(t, err)
		require.NotNil(t, written.NextRunAt)
		nextRunAt := time.Now().Add(time.Hour)
		require.NoError(t, bankConnectionSyncScheduleWriter{store: store}.UpsertBankConnectionSyncSchedule(
			t.Context(),
			financepkg.BankConnectionSyncSchedule{
				ScheduleID:   "bank-sync",
				ConnectionID: "connection",
				ActorUserID:  "actor",
				Interval:     time.Hour,
				NextRunAt:    &nextRunAt,
			},
		))
		registry := jobspkg.NewRegistry()
		require.NoError(t, registerFinanceJobHandler(registry, "present", func() error { return nil }))
		require.NoError(t, registerFinanceJobHandler(registry, "present", func() error { return nil }))
	})

	t.Run("rejects a missing monobank delay before finance construction", func(t *testing.T) {
		_, err := NewModule(ModuleDeps{
			HTTPClientFactory: apphttpclient.NewClientFactory(
				apphttpclient.ClientFactoryDeps{RootLogger: slog.New(slog.DiscardHandler)},
			),
		})
		require.ErrorContains(t, err, "fallback delay")
	})
}

func TestCSVImportJobEnqueuer(t *testing.T) {
	fake := faker.New()
	dsn := fmt.Sprintf("file:finance-csv-import-jobs-%s?mode=memory&cache=shared", fake.UUID().V4())
	sqlDB, err := sqlconn.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	store, err := jobspkg.NewStore(sqlDB, dsn, jobspkg.StoreOpts{TablePrefix: "finance_csv_"})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate())
	registry := jobspkg.NewRegistry()
	require.NoError(t, jobspkg.RegisterTypedHandler(
		registry,
		jobspkg.TypedHandlerSpec[csvImportJobInput, financepkg.CSVImportRunResult, struct{}]{
			JobType:       jobspkg.JobType(financepkg.CSVImportJobTypeTransactions),
			SupportsRetry: true,
			Run: func(context.Context, csvImportJobInput, func(struct{}) error) (financepkg.CSVImportRunResult, error) {
				return financepkg.CSVImportRunResult{}, nil
			},
		},
	))
	jobs, err := jobspkg.NewService(jobspkg.ServiceDeps{
		Store: store, IDGenerator: ident.NewMockGenerator(), Publisher: publisherStub{}, Registry: registry,
	})
	require.NoError(t, err)
	enqueuer := csvImportJobEnqueuer{jobs: jobs}
	request := financepkg.CSVImportJobRequest{
		JobType:        financepkg.CSVImportJobTypeTransactions,
		ImportID:       "import-" + fake.UUID().V4(),
		TenantID:       "tenant-" + fake.UUID().V4(),
		ActorID:        "user-" + fake.UUID().V4(),
		IdempotencyKey: "finance.csv-import:" + fake.UUID().V4(),
	}

	first, err := enqueuer.EnqueueCSVImport(t.Context(), request)
	require.NoError(t, err)
	second, err := enqueuer.EnqueueCSVImport(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	persisted, err := store.Get(t.Context(), first.ID)
	require.NoError(t, err)
	assert.Equal(t, request.IdempotencyKey, persisted.IdempotencyKey)
	var input csvImportJobInput
	require.NoError(t, json.Unmarshal(persisted.InputJSON, &input))
	assert.Equal(t, request.ImportID, input.ImportID)
}

func TestFXRefreshJobHandler(t *testing.T) {
	fake := faker.New()
	financeDSN := fmt.Sprintf("file:finance-fx-%s?mode=memory&cache=shared", fake.UUID().V4())
	financeSQLDB, err := sqlconn.Open(financeDSN)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, financeSQLDB.Close()) })
	financeDatabase, err := persistence.NewDatabase(financeSQLDB, financeDSN)
	require.NoError(t, err)
	require.NoError(t, persistence.NewMigrator(financeDatabase).Migrate(t.Context()))
	financeStore := persistence.NewStore(financeDatabase)

	rateDate := time.Date(2026, time.June, 9, 0, 0, 0, 0, time.UTC)
	fxService := financepkg.NewFXService(
		financeStore,
		financepkg.WithFXServiceRequiredPairs(persistence.NewFXPairDiscoveryStore(financeDatabase)),
		financepkg.WithFXServiceProviders(financepkg.NewStaticFXProvider(
			financepkg.FXProviderFrankfurter,
			[]domain.FXRate{{
				Provider:      financepkg.FXProviderFrankfurter,
				BaseCurrency:  "EUR",
				QuoteCurrency: "PLN",
				RateDate:      rateDate,
				Rate:          4.25,
			}},
		)),
	)

	jobsDSN := fmt.Sprintf("file:finance-fx-jobs-%s?mode=memory&cache=shared", fake.UUID().V4())
	jobsSQLDB, err := sqlconn.Open(jobsDSN)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, jobsSQLDB.Close()) })
	jobsStore, err := jobspkg.NewStore(jobsSQLDB, jobsDSN, jobspkg.StoreOpts{TablePrefix: "finance_fx_"})
	require.NoError(t, err)
	require.NoError(t, jobsStore.AutoMigrate())
	dispatchConfig := appdispatch.Config{
		DatabaseDSN: jobsDSN, TablePrefix: "finance_fx_", PollInterval: time.Millisecond,
	}
	require.NoError(t, appdispatch.AutoMigrate(t.Context(), dispatchConfig, jobsSQLDB))
	dispatchPublisher, err := appdispatch.NewPublisher(dispatchConfig, jobsSQLDB, slog.Default())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dispatchPublisher.Close()) })
	routerFactory, err := appdispatch.NewRouterFactory(dispatchConfig, jobsSQLDB, dispatchPublisher, slog.Default())
	require.NoError(t, err)
	registry := jobspkg.NewRegistry()
	require.NoError(t, registerFXRefreshJobHandler(registry, fxService))
	jobsService, err := jobspkg.NewService(jobspkg.ServiceDeps{
		Store: jobsStore, IDGenerator: ident.NewMockGenerator(), Publisher: publisherStub{}, Registry: registry,
	})
	require.NoError(t, err)
	enqueuer := fxRefreshJobEnqueuer{jobs: jobsService}
	jobRef, err := enqueuer.EnqueueFXRefresh(t.Context(), financepkg.FXRefreshJobRequest{
		JobType:   financepkg.FXRefreshJobType,
		Requester: financepkg.FXSyncRequester{Source: financepkg.FXSyncRequesterSourceSystem},
		Input:     financepkg.RefreshFXRatesParams{},
	})
	require.NoError(t, err)
	worker, err := jobspkg.NewWorker(jobspkg.WorkerDeps{
		Store: jobsStore, Registry: registry, WorkerID: "worker-" + fake.UUID().V4(), RouterFactory: routerFactory,
	})
	require.NoError(t, err)
	require.NoError(t, worker.ProcessJob(t.Context(), jobRef.ID))

	job, err := jobsStore.Get(t.Context(), jobRef.ID)
	require.NoError(t, err)
	assert.Equal(t, jobspkg.JobStatusSucceeded, job.Status)
	storedRates, err := financeStore.ListFXRates(t.Context(), persistence.ListFXRatesParams{})
	require.NoError(t, err)
	assert.Empty(t, storedRates)
}

//nolint:cyclop,gocyclo // Keeps closely related finance integration scenarios together.
func TestNewFinanceModule(t *testing.T) {
	memoryDSNOrdinal := 0
	makeSQLiteMemoryDSN := func(prefix string) string {
		memoryDSNOrdinal++
		return fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", prefix, memoryDSNOrdinal)
	}

	t.Run("make run bank connection sync params keeps omitted windows unset", func(t *testing.T) {
		jobID := "job-" + faker.New().UUID().V4()
		params := makeRunBankConnectionSyncParams(jobspkg.Job{ID: jobID}, bankConnectionSyncJobInput{
			ConnectionID: "connection-1",
			Reason:       "manual",
		})
		assert.Equal(t, jobID, params.JobID)
		assert.Nil(t, params.WindowStart)
		assert.Nil(t, params.WindowEnd)

		windowStart := time.Date(2026, time.June, 1, 9, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
		windowEnd := time.Date(2026, time.June, 15, 18, 0, 0, 0, time.FixedZone("UTC-5", -5*60*60))
		scheduledAt := time.Date(2026, time.June, 1, 9, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60))
		nextRunAt := scheduledAt.Add(15 * time.Minute)
		params = makeRunBankConnectionSyncParams(jobspkg.Job{
			ID: jobID, ScheduledAt: &scheduledAt, ScheduledNextRunAt: &nextRunAt,
		}, bankConnectionSyncJobInput{
			ConnectionID: "connection-2",
			Reason:       "manual",
			WindowStart:  &windowStart,
			WindowEnd:    &windowEnd,
		})
		assert.Equal(t, &windowStart, params.WindowStart)
		assert.Equal(t, &windowEnd, params.WindowEnd)
		assert.Equal(t, &scheduledAt, params.ScheduledAt)
		assert.Equal(t, &nextRunAt, params.ScheduledNextRunAt)
	})

	openSharedDB := func(t *testing.T, dsn string) *sql.DB {
		t.Helper()
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		return db
	}

	makeDatabase := func(t *testing.T, dsn string) *persistence.Database {
		t.Helper()
		database, err := persistence.NewDatabase(openSharedDB(t, dsn), dsn)
		require.NoError(t, err)
		return database
	}

	makeJobsService := func(t *testing.T, registry *jobspkg.Registry, dsn string) (*jobspkg.Service, *jobspkg.Store) {
		t.Helper()

		store, err := jobspkg.NewStore(openSharedDB(t, dsn), dsn, jobspkg.StoreOpts{TablePrefix: "jobs_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		service, err := jobspkg.NewService(jobspkg.ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   publisherStub{},
			Registry:    registry,
		})
		require.NoError(t, err)

		return service, store
	}
	makeJobsRouterFactory := func(t *testing.T, dsn string) *appdispatch.RouterFactory {
		t.Helper()
		db := openSharedDB(t, dsn)
		config := appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "jobs_", PollInterval: time.Millisecond}
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), config, db))
		publisher, err := appdispatch.NewPublisher(config, db, slog.Default())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		factory, err := appdispatch.NewRouterFactory(config, db, publisher, slog.Default())
		require.NoError(t, err)
		return factory
	}

	makePrivateKeyPath := func(t *testing.T) string {
		t.Helper()

		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		privateKeyPath := filepath.Join(t.TempDir(), "enable-banking-private-key.pem")
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
		require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))

		return privateKeyPath
	}

	makeHTTPClientFactory := func(
		t *testing.T,
		mutateRequest func(*http.Request),
	) *apphttpclient.ClientFactory {
		t.Helper()

		return apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{
			RootLogger: slog.New(slog.DiscardHandler),
			OtelHTTPTransportFactory: func(next http.RoundTripper) http.RoundTripper {
				return financeAppRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
					cloned := request.Clone(request.Context())
					cloned.Header = request.Header.Clone()
					if mutateRequest != nil {
						mutateRequest(cloned)
					}
					return next.RoundTrip(cloned)
				})
			},
		})
	}

	t.Run("starts one daily FX refresh schedule and keeps its persisted next run", func(t *testing.T) {
		database := makeDatabase(t, makeSQLiteMemoryDSN("finance-fx-refresh"))
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		registry := jobspkg.NewRegistry()
		jobsDSN := makeSQLiteMemoryDSN("jobs-fx-refresh")
		jobsService, jobsStore := makeJobsService(t, registry, jobsDSN)
		deps := ModuleDeps{
			Database:                        database,
			Jobs:                            jobsService,
			JobsStore:                       jobsStore,
			Registry:                        registry,
			RootLogger:                      slog.New(slog.DiscardHandler),
			HTTPClientFactory:               makeHTTPClientFactory(t, nil),
			JWTSigningKey:                   "jwt-key-for-fx-refresh-tests",
			MonobankRetryAfterFallbackDelay: 61 * time.Second,
			EnableBanking: financepkg.EnableBankingConfig{
				BaseURL:        "https://" + faker.New().Internet().Domain(),
				AppID:          "app-" + faker.New().UUID().V4(),
				PrivateKeyPath: makePrivateKeyPath(t),
				ASPSPs: []financepkg.EnableBankingASPSP{
					{
						ProviderID: domain.ProviderIDPKO,
						Name:       "ASPSP-" + faker.New().Company().Name(),
						Country:    "PL",
						PSUType:    "personal",
						ValidDays:  90,
					},
				},
			},
		}
		_, err := NewModule(deps)
		require.NoError(t, err)
		initialSchedule, err := jobsStore.GetSchedule(t.Context(), financepkg.FXDailyRefreshScheduleID)
		require.NoError(t, err)
		require.NotNil(t, initialSchedule.NextRunAt)
		assert.Equal(t, jobspkg.JobType(financepkg.FXRefreshJobType), initialSchedule.JobType)

		scheduler, err := jobspkg.NewScheduler(jobspkg.SchedulerDeps{
			Store: jobsStore, Service: jobsService, Clock: func() time.Time { return *initialSchedule.NextRunAt },
		})
		require.NoError(t, err)
		enqueued, err := scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 1, enqueued)
		enqueued, err = scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Zero(t, enqueued)
		scheduledJobs, err := jobsStore.List(t.Context(), jobspkg.ListParams{
			JobTypes: []jobspkg.JobType{jobspkg.JobType(financepkg.FXRefreshJobType)},
		})
		require.NoError(t, err)
		require.Len(t, scheduledJobs.Items, 1)
		worker, err := jobspkg.NewWorker(jobspkg.WorkerDeps{
			Store: jobsStore, Registry: registry, WorkerID: "worker-" + faker.New().UUID().V4(),
			RouterFactory: makeJobsRouterFactory(t, jobsDSN),
		})
		require.NoError(t, err)
		require.NoError(t, worker.ProcessJob(t.Context(), scheduledJobs.Items[0].ID))
		processed, err := jobsStore.Get(t.Context(), scheduledJobs.Items[0].ID)
		require.NoError(t, err)
		assert.Equal(t, jobspkg.JobStatusSucceeded, processed.Status)

		failedInput, err := json.Marshal(financepkg.RefreshFXRatesParams{Provider: "missing-provider"})
		require.NoError(t, err)
		failedJob, err := jobsService.EnqueueJSON(t.Context(), jobspkg.EnqueueJSONParams{
			JobType:   jobspkg.JobType(financepkg.FXRefreshJobType),
			Requester: jobspkg.Requester{Source: jobspkg.RequesterSourceOperator},
			InputJSON: failedInput,
		})
		require.NoError(t, err)
		require.NoError(t, worker.ProcessJob(t.Context(), failedJob.ID))
		failedJob, err = jobsStore.Get(t.Context(), failedJob.ID)
		require.NoError(t, err)
		assert.Equal(t, jobspkg.JobStatusFailed, failedJob.Status)
		_, err = jobsService.Retry(t.Context(), failedJob.ID)
		require.NoError(t, err)

		advancedSchedule, err := jobsStore.GetSchedule(t.Context(), financepkg.FXDailyRefreshScheduleID)
		require.NoError(t, err)
		require.NotNil(t, advancedSchedule.NextRunAt)
		persistedNextRunAt := *advancedSchedule.NextRunAt
		deps.Registry = jobspkg.NewRegistry()
		_, err = NewModule(deps)
		require.NoError(t, err)
		reconstructedSchedule, err := jobsStore.GetSchedule(t.Context(), financepkg.FXDailyRefreshScheduleID)
		require.NoError(t, err)
		require.NotNil(t, reconstructedSchedule.NextRunAt)
		assert.Equal(t, persistedNextRunAt, *reconstructedSchedule.NextRunAt)
	})

	t.Run("registers monobank and pko product choices and keeps sync job-backed", func(t *testing.T) {
		monoToken := "mono-token-test"
		expectedClientHeader := "finance-app-client"
		monoClientInfoBody := `{"name":"mono","accounts":[{"id":"mono-acc-1","type":"black","currencyCode":980,"iban":"UA123","balance":101}]}`
		monoTransactionsBody := `[{"id":"mono-txn-1","time":1717203600,"description":"mono txn","currencyCode":980,"amount":-250}]`
		monoServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/personal/client-info":
				if got := request.Header.Get("X-Token"); got != monoToken {
					t.Errorf("monobank link token header = %q, want %q", got, monoToken)
				}
				_, _ = writer.Write([]byte(monoClientInfoBody))
			case "/personal/statement/mono-acc-1/1717200000/1717286400":
				if got := request.Header.Get("X-Token"); got != monoToken {
					t.Errorf("monobank sync token header = %q, want %q", got, monoToken)
				}
				_, _ = writer.Write([]byte(monoTransactionsBody))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer monoServer.Close()

		enablePrivateKeyPath := makePrivateKeyPath(t)
		enableAuthBody := `{"url":"https://bank.example/auth","authorization_id":"provider-ref-1","psu_id_hash":"psu-hash-1"}`
		enableSessionBody := `{"session_id":"session-1","accounts":[{"uid":"pko-acc-1","name":"PKO","currency":"PLN","account_id":{"iban":"PL123"}}],"aspsp":{"name":"PKO"},"access":{"valid_until":"2026-07-01T00:00:00Z"}}`
		enableAccountsBody := `{"accounts":["pko-acc-1"],"accounts_data":[{"uid":"pko-acc-1","name":"PKO","currency":"PLN","account_id":{"iban":"PL123"}}],"aspsp":{"name":"PKO"},"status":"AUTHORIZED"}`
		enableBalancesBody := `{"balances":[{"balance_amount":{"amount":"12.00","currency":"PLN"},"balance_type":"closingBooked"},{"balance_amount":{"amount":"11.00","currency":"PLN"},"balance_type":"interimAvailable"}]}`
		enableTransactionsBody := `{"transactions":[{"entry_reference":"pko-txn-1","transaction_id":"pko-details-1","status":"BOOK","transaction_amount":{"amount":"5.00","currency":"PLN"},"credit_debit_indicator":"DBIT","remittance_information":["pko txn"],"booking_date":"2026-06-02"}]}`
		enableServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if got := request.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
				t.Errorf("enable banking authorization header = %q", got)
			}
			if got := request.Header.Get("X-App-Client"); got != expectedClientHeader {
				writer.WriteHeader(http.StatusBadGateway)
				_, _ = writer.Write([]byte(`{"message":"missing app-created client header"}`))
				return
			}
			switch {
			case request.Method == http.MethodPost && request.URL.Path == "/auth":
				_, _ = writer.Write([]byte(enableAuthBody))
			case request.Method == http.MethodPost && request.URL.Path == "/sessions":
				_, _ = writer.Write([]byte(enableSessionBody))
			case request.Method == http.MethodGet && request.URL.Path == "/sessions/session-1":
				_, _ = writer.Write([]byte(enableAccountsBody))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/pko-acc-1/balances":
				_, _ = writer.Write([]byte(enableBalancesBody))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/pko-acc-1/transactions":
				_, _ = writer.Write([]byte(enableTransactionsBody))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer enableServer.Close()

		database := makeDatabase(t, makeSQLiteMemoryDSN("finance"))
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))

		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			makeSQLiteMemoryDSN("jobs"),
		)

		financeModule, err := NewModule(ModuleDeps{
			Database:   database,
			Jobs:       jobsService,
			JobsStore:  jobsStore,
			Registry:   registry,
			RootLogger: nil,
			HTTPClientFactory: makeHTTPClientFactory(t, func(request *http.Request) {
				request.Header.Set("X-App-Client", expectedClientHeader)
			}),
			JWTSigningKey:                   "jwt-key-for-finance-tests",
			MonobankBaseURL:                 monoServer.URL,
			MonobankRetryAfterFallbackDelay: 61 * time.Second,
			EnableBanking: financepkg.EnableBankingConfig{
				BaseURL:        enableServer.URL,
				AppID:          "app-123",
				PrivateKeyPath: enablePrivateKeyPath,
				ASPSPs: []financepkg.EnableBankingASPSP{{
					ProviderID: domain.ProviderIDPKO,
					Name:       "PKO Bank Polski",
					Country:    "PL",
					PSUType:    "personal",
					ValidDays:  90,
				}},
			},
		})
		require.NoError(t, err)
		bankConnections := financeModule.BankConnectionService
		bankSyncService := financeModule.BankSyncService
		tenantService := financeModule.TenantService

		tenant, err := tenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-finance",
			DisplayCurrency: "PLN",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		monobankConnection, err := bankConnections.LinkTokenBankConnection(
			t.Context(),
			financepkg.LinkTokenBankConnectionParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "monobank",
				Token:       monoToken,
			},
		)
		require.NoError(t, err)
		require.Equal(t, "monobank", monobankConnection.Provider)

		start, err := bankConnections.StartBankConnectionLink(
			t.Context(),
			financepkg.StartBankConnectionLinkParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "pko",
				RedirectURL: "https://app.example.test/#/finance/connections",
			},
		)
		require.NoError(t, err)

		pkoConnection, err := bankConnections.FinishBankConnectionLink(
			t.Context(),
			financepkg.FinishBankConnectionLinkParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "pko",
				State:       start.State,
				Code:        "code-1",
			},
		)
		require.NoError(t, err)
		require.Equal(t, "pko", pkoConnection.Provider)

		connections := []string{monobankConnection.ID, pkoConnection.ID}
		for _, connectionID := range connections {
			jobRef, triggerErr := bankSyncService.TriggerBankConnectionSync(
				t.Context(),
				financepkg.TriggerBankConnectionSyncParams{
					ActorUserID:  "user-owner",
					TenantID:     tenant.ID,
					ConnectionID: connectionID,
					Reason:       financepkg.BankConnectionSyncReasonManual,
				},
			)
			require.NoError(t, triggerErr)

			job, getErr := jobsStore.Get(t.Context(), jobRef.ID)
			require.NoError(t, getErr)

			var input bankConnectionSyncJobInput
			require.NoError(t, json.Unmarshal(job.InputJSON, &input))
			require.Equal(t, connectionID, input.ConnectionID)
		}

		connectionsView, err := bankSyncService.ListBankConnections(
			t.Context(),
			financepkg.ListBankConnectionsParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
			},
		)
		require.NoError(t, err)
		require.Len(t, connectionsView, 2)
		providers := []string{
			connectionsView[0].Connection.Provider,
			connectionsView[1].Connection.Provider,
		}
		require.ElementsMatch(t, []string{"monobank", "pko"}, providers)

		states := []domain.BankConnectionState{
			connectionsView[0].Connection.State,
			connectionsView[1].Connection.State,
		}
		require.Contains(t, states, domain.BankConnectionStateActive)
	})

	t.Run("enables signed enable banking provider from injected config", func(t *testing.T) {
		privateKeyPath := makePrivateKeyPath(t)
		privateKeyPEM, err := os.ReadFile(privateKeyPath)
		require.NoError(t, err)
		block, _ := pem.Decode(privateKeyPEM)
		require.NotNil(t, block)
		parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		require.NoError(t, err)
		privateKey, ok := parsedKey.(*rsa.PrivateKey)
		require.True(t, ok)
		authBody := `{"url":"https://bank.example.test/authorize","authorization_id":"auth-123"}`
		sessionBody := `{"session_id":"session-123","accounts":[` +
			`{"uid":"acc-1","name":"ROR","currency":"PLN","account_id":{"iban":"PL123"}}]}`
		sessionDetailsBody := `{"accounts":["acc-1"],"accounts_data":[` +
			`{"uid":"acc-1","name":"ROR","currency":"PLN","account_id":{"iban":"PL123"}}],` +
			`"status":"AUTHORIZED"}`
		balancesBody := `{"balances":[` +
			`{"balance_type":"closingBooked","balance_amount":{"amount":"100.00","currency":"PLN"}}]}`
		transactionsBody := `{"transactions":[` +
			`{"entry_reference":"txn-1","transaction_id":"txn-details-1","status":"BOOK",` +
			`"booking_date":"2026-06-02","transaction_amount":{"amount":"10.00","currency":"PLN"},` +
			`"credit_debit_indicator":"DBIT","remittance_information":["Signed flow txn"]}]}`

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			authorization := strings.TrimSpace(request.Header.Get("Authorization"))
			if !strings.HasPrefix(authorization, "Bearer ") {
				t.Errorf("authorization header = %q", authorization)
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			tokenString := strings.TrimPrefix(authorization, "Bearer ")
			parser := jwt.NewParser(jwt.WithoutClaimsValidation())
			token, parseErr := parser.Parse(tokenString, func(_ *jwt.Token) (any, error) {
				return &privateKey.PublicKey, nil
			})
			if parseErr != nil {
				t.Errorf("parse jwt: %v", parseErr)
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			assert.True(t, token.Valid)
			assert.Equal(t, "app-123", token.Header["kid"])

			switch {
			case request.Method == http.MethodPost && request.URL.Path == "/auth":
				_, _ = writer.Write([]byte(authBody))
			case request.Method == http.MethodPost && request.URL.Path == "/sessions":
				_, _ = writer.Write([]byte(sessionBody))
			case request.Method == http.MethodGet && request.URL.Path == "/sessions/session-123":
				_, _ = writer.Write([]byte(sessionDetailsBody))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/acc-1/balances":
				_, _ = writer.Write([]byte(balancesBody))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/acc-1/transactions":
				_, _ = writer.Write([]byte(transactionsBody))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()

		database := makeDatabase(t, makeSQLiteMemoryDSN("finance"))
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			makeSQLiteMemoryDSN("jobs"),
		)

		financeModule, err := NewModule(ModuleDeps{
			Database:                        database,
			Jobs:                            jobsService,
			JobsStore:                       jobsStore,
			Registry:                        registry,
			RootLogger:                      nil,
			HTTPClientFactory:               makeHTTPClientFactory(t, nil),
			JWTSigningKey:                   "jwt-key-for-finance-tests",
			MonobankRetryAfterFallbackDelay: 61 * time.Second,
			EnableBanking: financepkg.EnableBankingConfig{
				BaseURL:        server.URL,
				AppID:          "app-123",
				PrivateKeyPath: privateKeyPath,
				ASPSPs: []financepkg.EnableBankingASPSP{{
					ProviderID: domain.ProviderIDPKO,
					Name:       "PKO Bank Polski",
					Country:    "PL",
					PSUType:    "personal",
					ValidDays:  90,
				}},
			},
		})
		require.NoError(t, err)
		bankConnections := financeModule.BankConnectionService
		bankSyncService := financeModule.BankSyncService
		tenantService := financeModule.TenantService

		tenant, err := tenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-signed-enable-banking",
			DisplayCurrency: "PLN",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		start, err := bankConnections.StartBankConnectionLink(t.Context(), financepkg.StartBankConnectionLinkParams{
			ActorUserID: "user-owner",
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://backend.example.test/enable-banking/callback",
		})
		require.NoError(t, err)

		connection, err := bankConnections.FinishBankConnectionLink(
			t.Context(),
			financepkg.FinishBankConnectionLinkParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "pko",
				State:       start.State,
				Code:        "code-1",
			},
		)
		require.NoError(t, err)

		jobRef, err := bankSyncService.TriggerBankConnectionSync(
			t.Context(),
			financepkg.TriggerBankConnectionSyncParams{
				ActorUserID:  "user-owner",
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
				Reason:       financepkg.BankConnectionSyncReasonManual,
			},
		)
		require.NoError(t, err)

		job, err := jobsStore.Get(t.Context(), jobRef.ID)
		require.NoError(t, err)
		var input bankConnectionSyncJobInput
		require.NoError(t, json.Unmarshal(job.InputJSON, &input))
		require.Equal(t, connection.ID, input.ConnectionID)
	})

	t.Run("uses configured auth signing key fallback for monobank token linking", func(t *testing.T) {
		monoToken := "mono-token-fallback"
		monoServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/personal/client-info" {
				t.Errorf("monobank link path = %q, want %q", request.URL.Path, "/personal/client-info")
			}
			if got := request.Header.Get("X-Token"); got != monoToken {
				t.Errorf("monobank fallback token header = %q, want %q", got, monoToken)
			}
			_, _ = writer.Write([]byte(
				`{"name":"mono","accounts":[{"id":"mono-acc-fallback","type":"black","currencyCode":980,"iban":"UA456","balance":101}]}`,
			))
		}))
		defer monoServer.Close()
		enablePrivateKeyPath := makePrivateKeyPath(t)

		database := makeDatabase(t, makeSQLiteMemoryDSN("finance"))
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			makeSQLiteMemoryDSN("jobs"),
		)

		financeModule, err := NewModule(ModuleDeps{
			Database: database, Jobs: jobsService, JobsStore: jobsStore, Registry: registry,
			HTTPClientFactory: makeHTTPClientFactory(t, nil), RootLogger: slog.Default(),
			JWTSigningKey: "test-configured-jwt-key", MonobankBaseURL: monoServer.URL,
			MonobankRetryAfterFallbackDelay: 61 * time.Second,
			EnableBanking: financepkg.EnableBankingConfig{
				BaseURL:        "https://api.enablebanking.com",
				AppID:          "app-auth-fallback",
				PrivateKeyPath: enablePrivateKeyPath,
				ASPSPs: []financepkg.EnableBankingASPSP{{
					ProviderID: domain.ProviderIDPKO,
					Name:       "Mock ASPSP",
					Country:    "PL",
					PSUType:    "personal",
					ValidDays:  90,
				}},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, financeModule.BankConnectionService)
		require.NotNil(t, financeModule.SyntheticLinkStateService)

		tenant, err := financeModule.TenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-fallback",
			DisplayCurrency: "UAH",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		connection, err := financeModule.BankConnectionService.LinkTokenBankConnection(
			t.Context(),
			financepkg.LinkTokenBankConnectionParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
				Provider:    "monobank",
				Token:       monoToken,
			},
		)
		require.NoError(t, err)
		require.Equal(t, "monobank", connection.Provider)

		connections, err := financeModule.BankSyncService.ListBankConnections(
			t.Context(),
			financepkg.ListBankConnectionsParams{
				ActorUserID: "user-owner",
				TenantID:    tenant.ID,
			},
		)
		require.NoError(t, err)
		require.Len(t, connections, 1)
		require.Equal(t, "monobank", connections[0].Connection.Provider)
	})

	t.Run("rejects omitted enable banking credentials", func(t *testing.T) {
		database := makeDatabase(t, makeSQLiteMemoryDSN("finance"))
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		_, err := NewModule(ModuleDeps{
			Database:                        database,
			HTTPClientFactory:               makeHTTPClientFactory(t, nil),
			JWTSigningKey:                   "jwt-key-for-finance-tests",
			MonobankRetryAfterFallbackDelay: 61 * time.Second,
			EnableBanking: financepkg.EnableBankingConfig{
				BaseURL: "https://api.enablebanking.com",
				ASPSPs: []financepkg.EnableBankingASPSP{{
					ProviderID: domain.ProviderIDPKO,
					Name:       "Mock ASPSP",
					Country:    "PL",
					PSUType:    "personal",
					ValidDays:  90,
				}},
			},
		})
		require.ErrorContains(t, err, "validate enable banking config: app ID is required")
	})
}
