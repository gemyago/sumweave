package financeapp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
)

type disabledFinanceCipher struct{}

func (disabledFinanceCipher) SealString(string) (credentials.Envelope, error) {
	return credentials.Envelope{}, errors.New("finance connection secret cipher is not configured")
}

func (disabledFinanceCipher) OpenString(credentials.Envelope) (string, error) {
	return "", errors.New("finance connection secret cipher is not configured")
}

// ModuleDeps contains the native finance settings and constructed
// collaborators needed by the finance application adapter.
type ModuleDeps struct {
	Database                        *persistence.Database
	CommandPublisher                *appdispatch.Publisher
	Registry                        *jobspkg.Registry
	HTTPClientFactory               *apphttpclient.ClientFactory
	RootLogger                      *slog.Logger
	JWTSigningKey                   string
	MonobankBaseURL                 string
	MonobankRetryAfterFallbackDelay time.Duration
	FrankfurterBaseURL              string
	EnableBanking                   financepkg.EnableBankingConfig
}

type fxRefreshJobService interface {
	RefreshRequiredFXRates(context.Context, financepkg.RefreshFXRatesParams) (financepkg.RefreshFXRatesResult, error)
}

type csvImportJobService interface {
	RunCSVImportJob(context.Context, financepkg.RunCSVImportJobParams) (financepkg.CSVImportRunResult, error)
}

type bankSyncJobService interface {
	RunBankConnectionSync(
		context.Context,
		financepkg.RunBankConnectionSyncParams,
	) (financepkg.BankConnectionSyncResult, error)
}

// NewDatabase opens the finance persistence adapter over the application SQL
// database. The application currently shares this database with jobs.
func NewDatabase(
	sqlDB *sql.DB,
	databaseDSN string,
	logger *slog.Logger,
) (*persistence.Database, error) {
	// TODO: We should make the DSN finance module specific.
	database, err := persistence.NewDatabase(sqlDB, databaseDSN, persistence.WithLogger(logger))
	if err != nil { // coverage-ignore // Persistence construction errors are covered by its package.
		return nil, fmt.Errorf("open finance database: %w", err)
	}
	return database, nil
}

// NewModule constructs finance for one explicit process root. Command
// publication and observed handler registration are optional root responsibilities;
// it never starts worker or scheduler loops.
func NewModule(deps ModuleDeps) (*financepkg.Finance, error) {
	cipher, err := makeFinanceCipher(deps.JWTSigningKey)
	if err != nil { // coverage-ignore // AES-GCM construction has no controllable failure after SHA-256 sizing.
		return nil, err
	}
	httpClient, err := newFinanceHTTPClient(deps.HTTPClientFactory)
	if err != nil {
		return nil, err
	}
	monobankHTTPClient, err := newMonobankHTTPClient(
		deps.HTTPClientFactory,
		deps.MonobankRetryAfterFallbackDelay,
	)
	if err != nil {
		return nil, err
	}
	financeConfig := &financepkg.Config{
		Database:               deps.Database,
		Logger:                 resolveFinanceLogger(deps.RootLogger),
		Now:                    time.Now,
		NewID:                  uuid.NewString,
		HTTPClient:             httpClient,
		ConnectionSecretCipher: cipher,
		Monobank: financepkg.MonobankConfig{
			BaseURL:    resolveMonobankBaseURL(deps.MonobankBaseURL),
			HTTPClient: monobankHTTPClient,
		},
		FXProviders: []financepkg.FXRatesProvider{
			financepkg.NewFrankfurterFXProvider(httpClient, deps.FrankfurterBaseURL),
		},
		DefaultFXProvider: financepkg.FXProviderFrankfurter,
		EnableBanking:     deps.EnableBanking,
	}
	if deps.CommandPublisher != nil {
		publisher := appdispatchSemanticCommandPublisher{publisher: deps.CommandPublisher}
		financeConfig.CommandPublisher = publisher
		financeConfig.ScheduledCommandPublisher = publisher
	}
	financeModule, err := financepkg.New(financeConfig)
	if err != nil {
		return nil, err
	}
	if deps.Registry != nil {
		registerErr := registerFinanceJobHandlers(
			deps.Registry,
			financeModule.FXService,
			financeModule.CSVImportService,
			financeModule.BankSyncService,
		)
		if registerErr != nil { // coverage-ignore // Registry behavior is exercised through the worker root.
			return nil, registerErr
		}
	}
	return financeModule, nil
}

func newFinanceHTTPClient(factory *apphttpclient.ClientFactory) (*http.Client, error) {
	if factory == nil {
		return nil, errors.New("finance HTTP client factory is required")
	}
	return factory.CreateClient(), nil
}

func newMonobankHTTPClient(
	factory *apphttpclient.ClientFactory,
	fallbackDelay time.Duration,
) (*http.Client, error) {
	if factory == nil {
		return nil, errors.New("finance HTTP client factory is required")
	}
	if fallbackDelay <= 0 {
		return nil, errors.New("monobank Retry-After fallback delay must be positive")
	}
	timeout := fallbackDelay + 2*30*time.Second + time.Second
	return factory.CreateClient(
		apphttpclient.WithRetryAfterFallbackDelay(fallbackDelay),
		apphttpclient.WithTimeout(timeout),
	), nil
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
	cipher, err := credentials.NewAESGCMCipher(sum[:], "sumweave-finance")
	if err != nil { // coverage-ignore // AES-GCM construction has no controllable failure after SHA-256 sizing.
		return nil, fmt.Errorf("create finance cipher: %w", err)
	}
	return cipher, nil
}

func registerFinanceJobHandlers(
	registry *jobspkg.Registry,
	fxService fxRefreshJobService,
	csvImportService csvImportJobService,
	bankSyncService bankSyncJobService,
) error {
	if registry == nil {
		return nil
	}
	return errors.Join(
		registerCSVImportJobHandler(
			registry,
			financepkg.TransactionCSVImportCommandTopic,
			csvImportService,
		),
		registerCSVImportJobHandler(
			registry,
			financepkg.AccountCSVImportCommandTopic,
			csvImportService,
		),
		registerBankSyncJobHandler(registry, bankSyncService),
		registerFXRefreshJobHandler(registry, fxService),
	)
}

func registerFXRefreshJobHandler(registry *jobspkg.Registry, service fxRefreshJobService) error {
	if service == nil {
		return nil
	}
	return registerFinanceJobHandler(registry, financepkg.FXRatesRefreshCommandTopic, func() error {
		return jobspkg.RegisterTypedHandler(
			registry,
			jobspkg.TypedHandlerSpec[financepkg.FXRatesRefreshCommand]{
				JobType: jobspkg.JobType(financepkg.FXRefreshJobType),
				Topic:   financepkg.FXRatesRefreshCommandTopic,
				Metadata: func(input financepkg.FXRatesRefreshCommand) (jobspkg.JobMetadata, error) { // coverage-ignore // Invoked through worker integration after finance composition.
					return jobMetadata(
						jobspkg.JobType(financepkg.FXRefreshJobType),
						input.Requester,
					)
				},
				Run: func(ctx context.Context, _ jobspkg.Job, input financepkg.FXRatesRefreshCommand) error { // coverage-ignore // Invoked through worker integration after finance composition.
					return runFXRefreshJob(ctx, service, input)
				},
			},
		)
	})
}

func registerCSVImportJobHandler(
	registry *jobspkg.Registry,
	topic string,
	service csvImportJobService,
) error {
	if service == nil {
		return nil
	}
	jobType := jobspkg.JobType(financepkg.CSVImportJobTypeTransactions)
	if topic == financepkg.AccountCSVImportCommandTopic {
		jobType = jobspkg.JobType(financepkg.CSVImportJobTypeAccounts)
	}
	return registerFinanceJobHandler(registry, topic, func() error {
		return jobspkg.RegisterTypedHandler(
			registry,
			jobspkg.TypedHandlerSpec[financepkg.CSVImportCommand]{
				JobType: jobType,
				Topic:   topic,
				Metadata: func(input financepkg.CSVImportCommand) (jobspkg.JobMetadata, error) { // coverage-ignore // Invoked through worker integration after finance composition.
					return jobMetadata(jobType, input.Requester)
				},
				Run: func( // coverage-ignore // Invoked through worker integration after finance composition.
					ctx context.Context,
					job jobspkg.Job,
					input financepkg.CSVImportCommand,
				) error {
					return runCSVImportJob(ctx, service, job, input)
				},
			},
		)
	})
}

func registerBankSyncJobHandler(registry *jobspkg.Registry, service bankSyncJobService) error {
	if service == nil {
		return nil
	}
	return registerFinanceJobHandler(
		registry,
		financepkg.BankConnectionSyncCommandTopic,
		func() error {
			return jobspkg.RegisterTypedHandler(
				registry,
				jobspkg.TypedHandlerSpec[financepkg.BankConnectionSyncCommand]{
					JobType: jobspkg.JobType(financepkg.BankConnectionSyncJobType),
					Topic:   financepkg.BankConnectionSyncCommandTopic,
					Metadata: func(input financepkg.BankConnectionSyncCommand) (jobspkg.JobMetadata, error) { // coverage-ignore // Invoked through worker integration after finance composition.
						metadata, err := jobMetadata(
							jobspkg.JobType(financepkg.BankConnectionSyncJobType),
							input.Requester,
						)
						metadata.ScheduledAt = input.ScheduledAt
						metadata.ScheduledNextRunAt = input.ScheduledNextRunAt
						return metadata, err
					},
					Run: func(ctx context.Context, job jobspkg.Job, input financepkg.BankConnectionSyncCommand) error { // coverage-ignore // Invoked through worker integration after finance composition.
						return runBankSyncJob(ctx, service, job, input)
					},
				},
			)
		},
	)
}

func runFXRefreshJob(
	ctx context.Context,
	service fxRefreshJobService,
	input financepkg.FXRatesRefreshCommand,
) error {
	_, err := service.RefreshRequiredFXRates(ctx, financepkg.RefreshFXRatesParams{Provider: input.Provider})
	return handledFinanceFailure(err)
}

func runCSVImportJob(
	ctx context.Context,
	service csvImportJobService,
	job jobspkg.Job,
	input financepkg.CSVImportCommand,
) error {
	_, err := service.RunCSVImportJob(ctx, financepkg.RunCSVImportJobParams{ImportID: input.ImportID, JobID: job.ID})
	return handledFinanceFailure(err)
}

func runBankSyncJob(
	ctx context.Context,
	service bankSyncJobService,
	job jobspkg.Job,
	input financepkg.BankConnectionSyncCommand,
) error {
	_, err := service.RunBankConnectionSync(ctx, makeRunBankConnectionSyncParams(job, input))
	return handledFinanceFailure(err)
}

func makeRunBankConnectionSyncParams(
	job jobspkg.Job,
	input financepkg.BankConnectionSyncCommand,
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

func registerFinanceJobHandler(
	registry *jobspkg.Registry,
	topic string,
	register func() error,
) error {
	if _, err := registry.Handler(topic); err == nil {
		return nil
	} else if !errors.Is(err, jobspkg.ErrHandlerNotRegistered) { // coverage-ignore // Registry returns only its documented not-registered error here.
		return err
	}
	return register()
}

func jobMetadata(
	jobType jobspkg.JobType,
	requester financepkg.CommandRequester,
) (jobspkg.JobMetadata, error) {
	if strings.TrimSpace(requester.Source) == "" {
		return jobspkg.JobMetadata{}, errors.New("finance command requester source is required")
	}
	return jobspkg.JobMetadata{
		JobType: jobType,
		Requester: jobspkg.Requester{
			UserID: requester.UserID,
			Source: jobspkg.RequesterSource(requester.Source),
		},
	}, nil
}

func handledFinanceFailure(err error) error {
	failure, ok := financepkg.TerminalFailureFrom(err)
	if !ok {
		return err
	}
	return appdispatch.NewBusinessFailure(err, failure.Code, failure.Summary, failure.Details)
}

type appdispatchPublisher interface {
	PublishRequest(context.Context, appdispatch.PublicationRequest) (appdispatch.PublicationReference, error)
	PublishRequestInTx(
		context.Context,
		*sql.Tx,
		appdispatch.PublicationRequest,
	) (appdispatch.PublicationReference, error)
}

type appdispatchSemanticCommandPublisher struct{ publisher appdispatchPublisher }

func (p appdispatchSemanticCommandPublisher) PublishSemanticCommand(
	ctx context.Context,
	command financepkg.SemanticCommand,
) (financepkg.DispatchReference, error) {
	reference, err := p.publisher.PublishRequest(ctx, appdispatch.PublicationRequest{
		Topic: command.Topic, Payload: command.Payload, IdempotencyKey: command.IdempotencyKey,
	})
	if err != nil {
		return financepkg.DispatchReference{}, fmt.Errorf(
			"publish finance semantic command: %w",
			err,
		)
	}
	return financepkg.DispatchReference{MessageID: reference.MessageID}, nil
}

func (p appdispatchSemanticCommandPublisher) PublishScheduledSemanticCommand(
	ctx context.Context,
	tx *sql.Tx,
	command financepkg.SemanticCommand,
) (financepkg.DispatchReference, error) {
	reference, err := p.publisher.PublishRequestInTx(ctx, tx, appdispatch.PublicationRequest{
		Topic: command.Topic, Payload: command.Payload, IdempotencyKey: command.IdempotencyKey,
	})
	if err != nil {
		return financepkg.DispatchReference{}, fmt.Errorf("publish scheduled finance semantic command: %w", err)
	}
	return financepkg.DispatchReference{MessageID: reference.MessageID}, nil
}
