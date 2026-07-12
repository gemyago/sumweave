package financeapp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	apphttpclient "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/google/uuid"
	"go.uber.org/dig"
)

type disabledFinanceCipher struct{}

func (disabledFinanceCipher) SealString(string) (credentials.Envelope, error) {
	return credentials.Envelope{}, errors.New("finance connection secret cipher is not configured")
}

func (disabledFinanceCipher) OpenString(credentials.Envelope) (string, error) {
	return "", errors.New("finance connection secret cipher is not configured")
}

type databaseDeps struct {
	dig.In

	DatabaseDSN string `name:"config.dataLayer.database.dsn"`
	RootLogger  *slog.Logger
	SQLDB       *sql.DB
}

//nolint:golines // dig tags stay clearer inline on the dependency struct.
type financeServiceDeps struct {
	dig.In

	Database             *persistence.Database
	Store                *persistence.Store
	Jobs                 *jobspkg.Service
	JobsStore            *jobspkg.Store
	Registry             *jobspkg.Registry
	HTTPClientFactory    *apphttpclient.ClientFactory
	RootLogger           *slog.Logger
	JWT                  string        `name:"auth.jwtKey" optional:"true"`
	MonoURL              string        `name:"config.finance.providers.monobank.baseURL" optional:"true"`
	MonoSleep            time.Duration `name:"config.finance.providers.monobank.sleepBetweenRequests" optional:"true"`
	EnableURL            string        `name:"config.finance.providers.enableBanking.baseURL" optional:"true"`
	EnableAppID          string        `name:"config.finance.providers.enableBanking.appID" optional:"true"`
	EnablePrivateKeyPath string        `name:"config.finance.providers.enableBanking.privateKeyPath" optional:"true"`
	EnableASPSPName      string        `name:"config.finance.providers.enableBanking.aspspName" optional:"true"`
	EnableCountry        string        `name:"config.finance.providers.enableBanking.country" optional:"true"`
	EnablePSUType        string        `name:"config.finance.providers.enableBanking.psuType" optional:"true"`
	EnableValidDays      int           `name:"config.finance.providers.enableBanking.validDays" optional:"true"`
}

func newDatabase(deps databaseDeps) (*persistence.Database, error) {
	// TODO: We should make the DSN finance module specific
	database, err := persistence.NewDatabase(deps.SQLDB, deps.DatabaseDSN, persistence.WithLogger(deps.RootLogger))
	if err != nil {
		return nil, fmt.Errorf("open finance database: %w", err)
	}
	return database, nil
}

func newFinanceModuleFromDI(deps financeServiceDeps) (*financepkg.Finance, error) {
	cipher, err := makeFinanceCipher(deps.JWT)
	if err != nil {
		return nil, err
	}
	httpClient, err := newFinanceHTTPClient(deps.HTTPClientFactory)
	if err != nil {
		return nil, err
	}
	financeModule, err := financepkg.New(&financepkg.Config{
		Database:               deps.Database,
		Logger:                 resolveFinanceLogger(deps.RootLogger),
		Now:                    time.Now,
		NewID:                  uuid.NewString,
		HTTPClient:             httpClient,
		ConnectionSecretCipher: cipher,
		CSVImportJobEnqueuer:   csvImportJobEnqueuer{jobs: deps.Jobs},
		BankSyncJobEnqueuer:    bankConnectionSyncJobEnqueuer{jobs: deps.Jobs},
		BankSyncScheduleWriter: bankConnectionSyncScheduleWriter{store: deps.JobsStore},
		FXJobEnqueuer:          fxSyncJobEnqueuer{jobs: deps.Jobs},
		Monobank: financepkg.MonobankConfig{
			BaseURL: resolveMonobankBaseURL(deps.MonoURL),
		},
		EnableBanking: buildEnableBankingConfig(deps),
	})
	if err != nil {
		return nil, err
	}
	registerErr := registerFinanceJobHandlers(
		deps.Registry,
		financeModule.FXService,
		financeModule.CSVImportService,
		financeModule.BankSyncService,
	)
	if registerErr != nil {
		return nil, registerErr
	}
	return financeModule, nil
}

func newFinanceHTTPClient(factory *apphttpclient.ClientFactory) (*http.Client, error) {
	if factory == nil {
		return nil, errors.New("finance HTTP client factory is required")
	}
	return factory.CreateClient(), nil
}

func newTenantServiceFromDI(module *financepkg.Finance) *financepkg.TenantService {
	return module.TenantService
}

func newCatalogServiceFromDI(module *financepkg.Finance) *financepkg.CatalogService {
	return module.CatalogService
}

func newLedgerServiceFromDI(module *financepkg.Finance) *financepkg.LedgerService {
	return module.LedgerService
}

func newReportingServiceFromDI(module *financepkg.Finance) *financepkg.ReportingService {
	return module.ReportingService
}

func newFXServiceFromDI(module *financepkg.Finance) *financepkg.FXService {
	return module.FXService
}

func newCSVImportServiceFromDI(module *financepkg.Finance) *financepkg.CSVImportService {
	return module.CSVImportService
}

func newBankConnectionServiceFromDI(module *financepkg.Finance) *financepkg.BankConnectionService {
	return module.BankConnectionService
}

func newSyntheticLinkStateServiceFromDI(module *financepkg.Finance) *financepkg.SyntheticLinkStateService {
	return module.SyntheticLinkStateService
}

func newBankSyncServiceFromDI(module *financepkg.Finance) *financepkg.BankSyncService {
	return module.BankSyncService
}

func buildEnableBankingConfig(deps financeServiceDeps) financepkg.EnableBankingConfig {
	return financepkg.EnableBankingConfig{
		BaseURL:        strings.TrimSpace(deps.EnableURL),
		AppID:          strings.TrimSpace(deps.EnableAppID),
		PrivateKeyPath: strings.TrimSpace(deps.EnablePrivateKeyPath),
		ASPSPs: []financepkg.EnableBankingASPSP{{
			ProviderID: domain.ProviderIDPKO,
			Name:       strings.TrimSpace(deps.EnableASPSPName),
			Country:    strings.TrimSpace(deps.EnableCountry),
			PSUType:    strings.TrimSpace(deps.EnablePSUType),
			ValidDays:  deps.EnableValidDays,
		}},
	}
}

func resolveFinanceLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return logger
}

func resolveMonobankBaseURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "https://api.monobank.ua"
	}
	return trimmed
}

func makeFinanceCipher(jwtKey string) (
	interface {
		SealString(string) (credentials.Envelope, error)
		OpenString(credentials.Envelope) (string, error)
	}, error) {
	trimmed := strings.TrimSpace(jwtKey)
	if trimmed == "" {
		return disabledFinanceCipher{}, nil
	}
	sum := sha256.Sum256([]byte(trimmed))
	cipher, err := credentials.NewAESGCMCipher(sum[:], "signal-foundry-finance")
	if err != nil {
		return nil, fmt.Errorf("create finance cipher: %w", err)
	}
	return cipher, nil
}

type csvImportJobInput struct {
	ImportID string `json:"importId"`
}

type bankConnectionSyncJobInput struct {
	ConnectionID string     `json:"connectionId"`
	Reason       string     `json:"reason"`
	WindowStart  *time.Time `json:"windowStart,omitempty"`
	WindowEnd    *time.Time `json:"windowEnd,omitempty"`
}

func registerFinanceJobHandlers(
	registry *jobspkg.Registry,
	fxService *financepkg.FXService,
	csvImportService *financepkg.CSVImportService,
	bankSyncService *financepkg.BankSyncService,
) error {
	if registry == nil {
		return nil
	}
	return errors.Join(
		registerCSVImportJobHandler(
			registry,
			jobspkg.JobType(financepkg.CSVImportJobTypeTransactions),
			csvImportService,
		),
		registerCSVImportJobHandler(
			registry,
			jobspkg.JobType(financepkg.CSVImportJobTypeAccounts),
			csvImportService,
		),
		registerBankSyncJobHandler(registry, bankSyncService),
		registerFXSyncJobHandler(registry, fxService),
	)
}

func registerCSVImportJobHandler(
	registry *jobspkg.Registry,
	jobType jobspkg.JobType,
	service *financepkg.CSVImportService,
) error {
	if service == nil {
		return nil
	}
	return registerFinanceJobHandler(registry, jobType, func() error {
		return jobspkg.RegisterTypedHandler(
			registry,
			jobspkg.TypedHandlerSpec[csvImportJobInput, financepkg.CSVImportRunResult, struct{}]{
				JobType:       jobType,
				SupportsRetry: true,
				Run: func(ctx context.Context, input csvImportJobInput, _ func(struct{}) error) (financepkg.CSVImportRunResult, error) {
					return service.RunCSVImportJob(ctx, financepkg.RunCSVImportJobParams{ImportID: input.ImportID})
				},
			},
		)
	})
}

func registerBankSyncJobHandler(
	registry *jobspkg.Registry,
	service *financepkg.BankSyncService,
) error {
	if service == nil {
		return nil
	}
	return registerFinanceJobHandler(registry, jobspkg.JobType(financepkg.BankConnectionSyncJobType), func() error {
		return jobspkg.RegisterTypedHandler(
			registry,
			jobspkg.TypedHandlerSpec[bankConnectionSyncJobInput, financepkg.BankConnectionSyncResult, struct{}]{
				JobType:       jobspkg.JobType(financepkg.BankConnectionSyncJobType),
				SupportsRetry: true,
				RunJob: func(ctx context.Context, job jobspkg.Job, input bankConnectionSyncJobInput, _ func(struct{}) error) (financepkg.BankConnectionSyncResult, error) {
					return service.RunBankConnectionSync(ctx, makeRunBankConnectionSyncParams(job, input))
				},
				OnScheduled: func(ctx context.Context, job jobspkg.Job) error {
					input, err := jobspkg.DecodeJobInput[bankConnectionSyncJobInput](job)
					if err != nil {
						return fmt.Errorf("decode scheduled bank sync input: %w", err)
					}
					if job.ScheduledAt == nil || job.ScheduledNextRunAt == nil {
						return errors.New("scheduled bank sync occurrence timestamps are required")
					}
					_, err = service.RecordBankConnectionSyncScheduled(
						ctx,
						financepkg.RecordBankConnectionSyncScheduledParams{
							ConnectionID: input.ConnectionID,
							JobID:        job.ID,
							ScheduledAt:  *job.ScheduledAt,
							NextRunAt:    *job.ScheduledNextRunAt,
						},
					)
					return err
				},
			},
		)
	})
}

func makeRunBankConnectionSyncParams(
	job jobspkg.Job,
	input bankConnectionSyncJobInput,
) financepkg.RunBankConnectionSyncParams {
	return financepkg.RunBankConnectionSyncParams{
		ConnectionID:       input.ConnectionID,
		JobID:              job.ID,
		Reason:             input.Reason,
		WindowStart:        input.WindowStart,
		WindowEnd:          input.WindowEnd,
		ScheduledAt:        job.ScheduledAt,
		ScheduledNextRunAt: job.ScheduledNextRunAt,
	}
}

func registerFXSyncJobHandler(
	registry *jobspkg.Registry,
	service *financepkg.FXService,
) error {
	if service == nil {
		return nil
	}
	return registerFinanceJobHandler(registry, jobspkg.JobType(financepkg.FXSyncJobType), func() error {
		return jobspkg.RegisterTypedHandler(
			registry,
			jobspkg.TypedHandlerSpec[financepkg.SyncFXRatesParams, financepkg.SyncFXRatesResult, struct{}]{
				JobType:       jobspkg.JobType(financepkg.FXSyncJobType),
				SupportsRetry: true,
				Run: func(ctx context.Context, input financepkg.SyncFXRatesParams, _ func(struct{}) error) (financepkg.SyncFXRatesResult, error) {
					return service.SyncFXRates(ctx, input)
				},
			},
		)
	})
}

func registerFinanceJobHandler(
	registry *jobspkg.Registry,
	jobType jobspkg.JobType,
	register func() error,
) error {
	if _, err := registry.Handler(jobType); err == nil {
		return nil
	} else if !errors.Is(err, jobspkg.ErrHandlerNotRegistered) {
		return err
	}
	return register()
}

type csvImportJobEnqueuer struct{ jobs *jobspkg.Service }

func (e csvImportJobEnqueuer) EnqueueCSVImport(
	ctx context.Context,
	request financepkg.CSVImportJobRequest,
) (financepkg.CSVImportJobRef, error) {
	job, err := e.jobs.Enqueue(ctx, jobspkg.EnqueueParams{
		JobType: jobspkg.JobType(request.JobType),
		Requester: jobspkg.Requester{
			UserID: strings.TrimSpace(request.ActorID),
			Source: jobspkg.RequesterSourceOperator,
		},
		Input: csvImportJobInput{ImportID: strings.TrimSpace(request.ImportID)},
	})
	if err != nil {
		return financepkg.CSVImportJobRef{}, err
	}
	return financepkg.CSVImportJobRef{ID: job.ID, JobType: string(job.JobType)}, nil
}

type bankConnectionSyncJobEnqueuer struct{ jobs *jobspkg.Service }

func (e bankConnectionSyncJobEnqueuer) EnqueueBankConnectionSync(
	ctx context.Context,
	request financepkg.BankConnectionSyncJobRequest,
) (financepkg.BankConnectionSyncJobRef, error) {
	job, err := e.jobs.Enqueue(ctx, jobspkg.EnqueueParams{
		JobType: jobspkg.JobType(request.JobType),
		Requester: jobspkg.Requester{
			UserID: strings.TrimSpace(request.Actor),
			Source: jobspkg.RequesterSourceOperator,
		},
		Input: bankConnectionSyncJobInput{
			ConnectionID: strings.TrimSpace(request.Input.ConnectionID),
			Reason:       strings.TrimSpace(request.Input.Reason),
			WindowStart:  request.Input.WindowStart,
			WindowEnd:    request.Input.WindowEnd,
		},
	})
	if err != nil {
		return financepkg.BankConnectionSyncJobRef{}, err
	}
	return financepkg.BankConnectionSyncJobRef{ID: job.ID, JobType: string(job.JobType)}, nil
}

type fxSyncJobEnqueuer struct{ jobs *jobspkg.Service }

type bankConnectionSyncScheduleWriter struct{ store *jobspkg.Store }

func (w bankConnectionSyncScheduleWriter) UpsertBankConnectionSyncSchedule(
	ctx context.Context,
	schedule financepkg.BankConnectionSyncSchedule,
) error {
	if w.store == nil {
		return nil
	}
	inputJSON, err := json.Marshal(bankConnectionSyncJobInput{
		ConnectionID: strings.TrimSpace(schedule.ConnectionID),
		Reason:       financepkg.BankConnectionSyncReasonScheduled,
	})
	if err != nil {
		return fmt.Errorf("encode bank connection sync schedule: %w", err)
	}
	var nextRunAt *time.Time
	if schedule.Enabled {
		now := time.Now()
		nextRunAt = &now
	}
	if schedule.NextRunAt != nil {
		nextRunAt = schedule.NextRunAt
	}
	return w.store.UpsertSchedule(ctx, jobspkg.Schedule{
		ID:      strings.TrimSpace(schedule.ScheduleID),
		JobType: jobspkg.JobType(financepkg.BankConnectionSyncJobType),
		Requester: jobspkg.Requester{
			UserID: strings.TrimSpace(schedule.ActorUserID),
			Source: jobspkg.RequesterSourceOperator,
		},
		InputJSON: inputJSON,
		Interval:  schedule.Interval,
		NextRunAt: nextRunAt,
		Enabled:   schedule.Enabled,
	})
}

func (e fxSyncJobEnqueuer) EnqueueFXSync(
	ctx context.Context,
	request financepkg.FXSyncJobRequest,
) (financepkg.FXSyncJobRef, error) {
	job, err := e.jobs.Enqueue(ctx, jobspkg.EnqueueParams{
		JobType: jobspkg.JobType(request.JobType),
		Requester: jobspkg.Requester{
			UserID: strings.TrimSpace(request.Requester.UserID),
			Source: jobspkg.RequesterSource(strings.TrimSpace(request.Requester.Source)),
		},
		Input: request.Input,
	})
	if err != nil {
		return financepkg.FXSyncJobRef{}, err
	}
	return financepkg.FXSyncJobRef{ID: job.ID, JobType: string(job.JobType)}, nil
}

func Register(container *dig.Container) error {
	return di.ProvideAll(
		container,
		newDatabase,
		persistence.NewStore,
		newFinanceModuleFromDI,
		newTenantServiceFromDI,
		newCatalogServiceFromDI,
		newLedgerServiceFromDI,
		newReportingServiceFromDI,
		newFXServiceFromDI,
		newCSVImportServiceFromDI,
		newBankConnectionServiceFromDI,
		newSyntheticLinkStateServiceFromDI,
		newBankSyncServiceFromDI,
	)
}
