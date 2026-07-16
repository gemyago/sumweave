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

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/auth"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	apphttpclient "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

type publisherStub struct{}

func (publisherStub) PublishInTx(context.Context, *sql.Tx, appdispatch.Envelope) error { return nil }

type financeAppRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f financeAppRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestNewFinanceStoreFromDI(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-store-"+t.Name())
	sharedDB, err := sqlconn.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sharedDB.Close()) })

	database, err := newDatabase(databaseDeps{
		DatabaseDSN: dsn,
		SQLDB:       sharedDB,
	})
	require.NoError(t, err)

	store := persistence.NewStore(database)
	_, err = store.ListTenantsForUser(t.Context(), "user-no-auto-migrate")
	require.Error(t, err)
	require.ErrorContains(t, err, "no such table")
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

func TestFXSyncJobHandler(t *testing.T) {
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
	registry := jobspkg.NewRegistry()
	require.NoError(t, registerFXSyncJobHandler(registry, fxService))
	jobsService, err := jobspkg.NewService(jobspkg.ServiceDeps{
		Store: jobsStore, IDGenerator: ident.NewMockGenerator(), Publisher: publisherStub{}, Registry: registry,
	})
	require.NoError(t, err)
	enqueuer := fxSyncJobEnqueuer{jobs: jobsService}
	jobRef, err := enqueuer.EnqueueFXSync(t.Context(), financepkg.FXSyncJobRequest{
		JobType:   financepkg.FXSyncJobType,
		Requester: financepkg.FXSyncRequester{Source: financepkg.FXSyncRequesterSourceSystem},
		Input: financepkg.SyncFXRatesParams{
			BaseCurrencies: []string{"EUR"}, QuoteCurrency: "PLN", StartDate: rateDate, EndDate: rateDate,
		},
	})
	require.NoError(t, err)
	worker, err := jobspkg.NewWorker(jobspkg.WorkerDeps{
		Store: jobsStore, Registry: registry, WorkerID: "worker-" + fake.UUID().V4(),
	})
	require.NoError(t, err)
	require.NoError(t, worker.ProcessJob(t.Context(), jobRef.ID))

	job, err := jobsStore.Get(t.Context(), jobRef.ID)
	require.NoError(t, err)
	assert.Equal(t, jobspkg.JobStatusSucceeded, job.Status)
	storedRates, err := financeStore.ListFXRates(t.Context(), persistence.ListFXRatesParams{
		Provider: financepkg.FXProviderFrankfurter, BaseCurrency: "EUR", QuoteCurrency: "PLN",
	})
	require.NoError(t, err)
	require.Len(t, storedRates, 1)
	assert.InDelta(t, 4.25, storedRates[0].Rate, 0.00001)
}

//nolint:cyclop,gocyclo // Keeps closely related DI integration scenarios together.
func TestNewFinanceServiceFromDI(t *testing.T) {
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
		financeStore := persistence.NewStore(database)
		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(t, registry, makeSQLiteMemoryDSN("jobs-fx-refresh"))
		deps := financeServiceDeps{
			Database:             database,
			Store:                financeStore,
			Jobs:                 jobsService,
			JobsStore:            jobsStore,
			Registry:             registry,
			RootLogger:           slog.New(slog.DiscardHandler),
			HTTPClientFactory:    makeHTTPClientFactory(t, nil),
			JWT:                  "jwt-key-for-fx-refresh-tests",
			EnableURL:            "https://" + faker.New().Internet().Domain(),
			EnableAppID:          "app-" + faker.New().UUID().V4(),
			EnablePrivateKeyPath: makePrivateKeyPath(t),
			EnableASPSPName:      "ASPSP-" + faker.New().Company().Name(),
			EnableCountry:        "PL",
			EnablePSUType:        "personal",
			EnableValidDays:      90,
		}
		_, err := newFinanceModuleFromDI(deps)
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
		_, err = newFinanceModuleFromDI(deps)
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

		financeStore := persistence.NewStore(database)

		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			makeSQLiteMemoryDSN("jobs"),
		)

		financeModule, err := newFinanceModuleFromDI(financeServiceDeps{
			Database:   database,
			Store:      financeStore,
			Jobs:       jobsService,
			JobsStore:  jobsStore,
			Registry:   registry,
			RootLogger: nil,
			HTTPClientFactory: makeHTTPClientFactory(t, func(request *http.Request) {
				request.Header.Set("X-App-Client", expectedClientHeader)
			}),
			JWT:                  "jwt-key-for-finance-tests",
			MonoURL:              monoServer.URL,
			EnableURL:            enableServer.URL,
			EnableAppID:          "app-123",
			EnablePrivateKeyPath: enablePrivateKeyPath,
			EnableASPSPName:      "PKO Bank Polski",
			EnableCountry:        "PL",
			EnablePSUType:        "personal",
			EnableValidDays:      90,
		})
		require.NoError(t, err)
		bankConnections := financeModule.BankConnectionService
		bankSyncService := financeModule.BankSyncService
		tenantService := financeModule.TenantService

		tenant, err := tenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-finance",
			DisplayCurrency: "PLN",
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
		financeStore := persistence.NewStore(database)

		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			makeSQLiteMemoryDSN("jobs"),
		)

		financeModule, err := newFinanceModuleFromDI(financeServiceDeps{
			Database:             database,
			Store:                financeStore,
			Jobs:                 jobsService,
			JobsStore:            jobsStore,
			Registry:             registry,
			RootLogger:           nil,
			HTTPClientFactory:    makeHTTPClientFactory(t, nil),
			JWT:                  "jwt-key-for-finance-tests",
			EnableURL:            server.URL,
			EnableAppID:          "app-123",
			EnablePrivateKeyPath: privateKeyPath,
			EnableASPSPName:      "PKO Bank Polski",
			EnableCountry:        "PL",
			EnablePSUType:        "personal",
			EnableValidDays:      90,
		})
		require.NoError(t, err)
		bankConnections := financeModule.BankConnectionService
		bankSyncService := financeModule.BankSyncService
		tenantService := financeModule.TenantService

		tenant, err := tenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-signed-enable-banking",
			DisplayCurrency: "PLN",
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
		require.Equal(t, "session-123", connection.ExternalID)

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
		financeStore := persistence.NewStore(database)

		registry := jobspkg.NewRegistry()
		jobsService, jobsStore := makeJobsService(
			t,
			registry,
			makeSQLiteMemoryDSN("jobs"),
		)

		container := dig.New()
		require.NoError(
			t,
			di.ProvideAll(
				container,
				di.ProvideValue("test-configured-jwt-key", dig.Name("config.auth.jwtSigningKey")),
				di.ProvideValue(24*time.Hour, dig.Name("config.auth.accessTokenTTL")),
				di.ProvideValue(7*24*time.Hour, dig.Name("config.auth.refreshTokenTTL")),
				slog.Default,
				func() *persistence.Database { return database },
				func() *persistence.Store { return financeStore },
				func() *jobspkg.Service { return jobsService },
				func() *jobspkg.Store { return jobsStore },
				func() *jobspkg.Registry { return registry },
				func() *apphttpclient.ClientFactory { return makeHTTPClientFactory(t, nil) },
				di.ProvideValue(monoServer.URL, dig.Name("config.finance.providers.monobank.baseURL")),
				di.ProvideValue(time.Duration(0), dig.Name("config.finance.providers.monobank.sleepBetweenRequests")),
				di.ProvideValue(
					"https://api.enablebanking.com",
					dig.Name("config.finance.providers.enableBanking.baseURL"),
				),
				di.ProvideValue("app-auth-fallback", dig.Name("config.finance.providers.enableBanking.appID")),
				di.ProvideValue(
					enablePrivateKeyPath,
					dig.Name("config.finance.providers.enableBanking.privateKeyPath"),
				),
				di.ProvideValue("Mock ASPSP", dig.Name("config.finance.providers.enableBanking.aspspName")),
				di.ProvideValue("PL", dig.Name("config.finance.providers.enableBanking.country")),
				di.ProvideValue("personal", dig.Name("config.finance.providers.enableBanking.psuType")),
				di.ProvideValue(90, dig.Name("config.finance.providers.enableBanking.validDays")),
			),
		)
		require.NoError(t, auth.Register(container))
		require.NoError(t, container.Provide(newFinanceModuleFromDI))
		require.NoError(t, container.Provide(newTenantServiceFromDI))
		require.NoError(t, container.Provide(newBankSyncServiceFromDI))
		require.NoError(t, container.Provide(newBankConnectionServiceFromDI))
		require.NoError(t, container.Provide(newSyntheticLinkStateServiceFromDI))

		type resolvedDeps struct {
			dig.In

			JWTKey                    string `name:"auth.jwtKey"`
			TenantService             *financepkg.TenantService
			BankSyncService           *financepkg.BankSyncService
			BankConnectionService     *financepkg.BankConnectionService
			SyntheticLinkStateService *financepkg.SyntheticLinkStateService
		}

		var resolved resolvedDeps
		require.NoError(t, container.Invoke(func(deps resolvedDeps) {
			resolved = deps
		}))
		require.NotEmpty(t, resolved.JWTKey)
		require.NotNil(t, resolved.BankConnectionService)
		require.NotNil(t, resolved.SyntheticLinkStateService)

		require.Equal(t, "test-configured-jwt-key", resolved.JWTKey)

		tenant, err := resolved.TenantService.CreateTenant(t.Context(), financepkg.CreateTenantParams{
			ActorUserID:     "user-owner",
			Name:            "tenant-fallback",
			DisplayCurrency: "UAH",
		})
		require.NoError(t, err)

		connection, err := resolved.BankConnectionService.LinkTokenBankConnection(
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

		connections, err := resolved.BankSyncService.ListBankConnections(
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
		financeStore := persistence.NewStore(database)

		_, err := newFinanceModuleFromDI(financeServiceDeps{
			Database:          database,
			Store:             financeStore,
			HTTPClientFactory: makeHTTPClientFactory(t, nil),
			JWT:               "jwt-key-for-finance-tests",
			EnableURL:         "https://api.enablebanking.com",
			EnableASPSPName:   "Mock ASPSP",
			EnableCountry:     "PL",
			EnablePSUType:     "personal",
			EnableValidDays:   90,
		})
		require.ErrorContains(t, err, "validate enable banking config: app ID is required")
	})
}
