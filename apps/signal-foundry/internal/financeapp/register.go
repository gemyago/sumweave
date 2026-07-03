package financeapp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
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
}

//nolint:golines // dig tags stay clearer inline on the dependency struct.
type financeServiceDeps struct {
	dig.In

	Database             *persistence.Database
	Store                *persistence.Store
	Jobs                 *jobspkg.Service
	JobsStore            *jobspkg.Store
	Registry             *jobspkg.Registry
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
	database, err := persistence.OpenDatabase(deps.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open finance database: %w", err)
	}
	return database, nil
}

func newFinanceServiceFromDI(deps financeServiceDeps) (*financepkg.Service, error) {
	opts := []financepkg.ServiceOption{
		financepkg.WithCSVImportJobEnqueuer(csvImportJobEnqueuer{jobs: deps.Jobs}),
		financepkg.WithBankSyncJobEnqueuer(bankConnectionSyncJobEnqueuer{jobs: deps.Jobs}),
		financepkg.WithBankConnectionSyncScheduleWriter(bankConnectionSyncScheduleWriter{store: deps.JobsStore}),
		financepkg.WithFXJobEnqueuer(fxSyncJobEnqueuer{jobs: deps.Jobs}),
		financepkg.WithLogger(deps.RootLogger),
	}
	if cipher, err := makeFinanceCipher(deps.JWT); err != nil {
		return nil, err
	} else if cipher != nil {
		opts = append(opts, financepkg.WithConnectionSecretCipher(cipher))
	}
	service := financepkg.NewService(deps.Store, opts...)
	if err := registerFinanceJobHandlers(deps.Registry, service); err != nil {
		return nil, err
	}
	return service, nil
}

func newBankConnectionServiceFromDI(
	deps financeServiceDeps,
) (*financepkg.BankConnectionService, error) {
	cipher, err := makeFinanceCipher(deps.JWT)
	if err != nil {
		return nil, err
	}
	financeModule, err := financepkg.New(&financepkg.Config{
		Database:               deps.Database,
		Logger:                 resolveFinanceLogger(deps.RootLogger),
		Now:                    time.Now,
		NewID:                  uuid.NewString,
		HTTPClient:             http.DefaultClient,
		ConnectionSecretCipher: cipher,
		Monobank: financepkg.MonobankConfig{
			BaseURL: resolveMonobankBaseURL(deps.MonoURL),
		},
		EnableBanking: buildEnableBankingConfig(deps),
	})
	if err != nil {
		return nil, err
	}
	return financeModule.BankConnectionService, nil
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

func registerFinanceJobHandlers(registry *jobspkg.Registry, service *financepkg.Service) error {
	if registry == nil || service == nil {
		return nil
	}
	for _, spec := range []struct {
		jobType  jobspkg.JobType
		register func() error
	}{
		{
			jobType: jobspkg.JobType(financepkg.CSVImportJobTypeTransactions),
			register: func() error {
				return jobspkg.RegisterTypedHandler(
					registry,
					jobspkg.TypedHandlerSpec[csvImportJobInput, financepkg.CSVImportRunResult, struct{}]{
						JobType:       jobspkg.JobType(financepkg.CSVImportJobTypeTransactions),
						SupportsRetry: true,
						Run: func(ctx context.Context, input csvImportJobInput, _ func(struct{}) error) (financepkg.CSVImportRunResult, error) {
							return service.RunCSVImportJob(ctx, financepkg.RunCSVImportJobParams{ImportID: input.ImportID})
						},
					},
				)
			},
		},
		{
			jobType: jobspkg.JobType(financepkg.CSVImportJobTypeAccounts),
			register: func() error {
				return jobspkg.RegisterTypedHandler(
					registry,
					jobspkg.TypedHandlerSpec[csvImportJobInput, financepkg.CSVImportRunResult, struct{}]{
						JobType:       jobspkg.JobType(financepkg.CSVImportJobTypeAccounts),
						SupportsRetry: true,
						Run: func(ctx context.Context, input csvImportJobInput, _ func(struct{}) error) (financepkg.CSVImportRunResult, error) {
							return service.RunCSVImportJob(ctx, financepkg.RunCSVImportJobParams{ImportID: input.ImportID})
						},
					},
				)
			},
		},
		{
			jobType: jobspkg.JobType(financepkg.BankConnectionSyncJobType),
			register: func() error {
				return jobspkg.RegisterTypedHandler(
					registry,
					jobspkg.TypedHandlerSpec[bankConnectionSyncJobInput, financepkg.BankConnectionSyncResult, struct{}]{
						JobType:       jobspkg.JobType(financepkg.BankConnectionSyncJobType),
						SupportsRetry: true,
						Run: func(ctx context.Context, input bankConnectionSyncJobInput, _ func(struct{}) error) (financepkg.BankConnectionSyncResult, error) {
							windowStart := time.Now().UTC().AddDate(0, 0, -30)
							windowEnd := time.Now().UTC()
							if input.WindowStart != nil {
								windowStart = input.WindowStart.UTC()
							}
							if input.WindowEnd != nil {
								windowEnd = input.WindowEnd.UTC()
							}
							return service.RunBankConnectionSync(ctx, financepkg.RunBankConnectionSyncParams{
								ConnectionID: input.ConnectionID,
								Reason:       input.Reason,
								WindowStart:  windowStart,
								WindowEnd:    windowEnd,
							})
						},
					},
				)
			},
		},
		{
			jobType: jobspkg.JobType(financepkg.FXSyncJobType),
			register: func() error {
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
			},
		},
	} {
		if _, err := registry.Handler(spec.jobType); err == nil {
			continue
		} else if !errors.Is(err, jobspkg.ErrHandlerNotRegistered) {
			return err
		}
		if err := spec.register(); err != nil {
			return err
		}
	}
	return nil
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
	var nextRunAt time.Time
	if schedule.Enabled {
		nextRunAt = time.Now().UTC()
	}
	if schedule.NextRunAt != nil {
		nextRunAt = schedule.NextRunAt.UTC()
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
		newFinanceServiceFromDI,
		newBankConnectionServiceFromDI,
	)
}
