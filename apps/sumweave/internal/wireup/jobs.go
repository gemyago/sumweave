package wireup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal"
	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/financeapp"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	financepkg "github.com/gemyago/sumweave/finance"
)

// WorkerOptions are command inputs used to load the durable worker root.
type WorkerOptions struct {
	Environment     string
	DefaultLogLevel *string
	JSONLogs        *bool
	LogsFile        *string
	DisablePProf    bool
}

// SchedulerOptions are command inputs used to load the one-shot scheduler root.
type SchedulerOptions struct {
	Environment     string
	DefaultLogLevel *string
	JSONLogs        *bool
	LogsFile        *string
	DisablePProf    bool
}

// WorkerRoot owns the message router, observed lifecycle store, and finance
// command handlers. It deliberately exposes no HTTP or scheduler capability.
type WorkerRoot struct {
	Worker   *jobspkg.Worker
	Registry *jobspkg.Registry

	shutdownHooks *lifecycle.ShutdownHooks
}

// SchedulerRoot owns the dispatch publisher and finance schedule operations.
// It deliberately exposes no HTTP or worker capability.
type SchedulerRoot struct {
	bankSchedules         scheduleEnqueuer
	fxSchedules           scheduleEnqueuer
	SchedulerLoopInterval time.Duration

	shutdownHooks *lifecycle.ShutdownHooks
}

type scheduleEnqueuer interface {
	EnqueueDue(context.Context) (int, error)
}

// BuildWorker loads typed configuration and composes the dedicated durable
// worker. It never starts HTTP or scheduler loops.
func BuildWorker(ctx context.Context, options WorkerOptions) (*WorkerRoot, error) { // coverage-ignore
	environment := options.Environment
	if environment == "" {
		environment = localEnvironment
	}
	values, err := loadProcessValues(environment, options.DefaultLogLevel, options.JSONLogs, options.LogsFile)
	if err != nil {
		return nil, fmt.Errorf("load worker configuration: %w", err)
	}
	rootConfig, err := values.WorkerRoot(environment)
	if err != nil {
		return nil, err
	}
	if options.DisablePProf {
		rootConfig.PProfListener.Enabled = false
	}
	return buildWorker(ctx, rootConfig)
}

// BuildScheduler loads typed configuration and composes the dedicated one-shot
// scheduler. It never starts HTTP or worker loops.
func BuildScheduler(ctx context.Context, options SchedulerOptions) (*SchedulerRoot, error) { // coverage-ignore
	environment := options.Environment
	if environment == "" {
		environment = localEnvironment
	}
	values, err := loadProcessValues(environment, options.DefaultLogLevel, options.JSONLogs, options.LogsFile)
	if err != nil {
		return nil, fmt.Errorf("load scheduler configuration: %w", err)
	}
	rootConfig, err := values.SchedulerRoot(environment)
	if err != nil {
		return nil, err
	}
	if options.DisablePProf {
		rootConfig.PProfListener.Enabled = false
	}
	return buildScheduler(ctx, rootConfig)
}

func loadProcessValues(
	environment string,
	defaultLogLevel *string,
	jsonLogs *bool,
	logsFile *string,
) (config.Values, error) { // coverage-ignore
	return config.LoadValues(config.ValuesLoadInput{
		Environment: environment,
		CLI: config.CLIOverrides{
			DefaultLogLevel: defaultLogLevel,
			JSONLogs:        jsonLogs,
			LogsFile:        logsFile,
		},
	})
}

type processInfrastructureConfig struct {
	environment             string
	defaultLogLevel         string
	jsonLogs                *bool
	logsFile                *string
	pprofListener           config.PProfListener
	gracefulShutdownTimeout time.Duration
	openTelemetry           config.OpenTelemetry
	application             config.Application
}

type processInfrastructure struct {
	database             *sql.DB
	rootLogger           *slog.Logger
	httpTransportFactory telemetry.OtelHTTPTransportFactory
	shutdownHooks        *lifecycle.ShutdownHooks
}

//nolint:gocognit // Telemetry construction is deliberately centralized for process roots.
func newProcessInfrastructure(
	ctx context.Context,
	name string,
	rootConfig processInfrastructureConfig,
) (_ *processInfrastructure, err error) { // coverage-ignore
	var shutdownHooks *lifecycle.ShutdownHooks
	defer func() {
		if err == nil || shutdownHooks == nil {
			return
		}
		if shutdownErr := shutdownHooks.PerformShutdown(context.WithoutCancel(ctx)); shutdownErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up failed %s root: %w", name, shutdownErr))
		}
	}()
	logLevel, err := parseLogLevel(rootConfig.defaultLogLevel)
	if err != nil {
		return nil, err
	}
	otelConfig, tracesConfig, metricsConfig, logsConfig := makeTelemetryConfigs(rootConfig.openTelemetry)
	resource, err := telemetry.NewResource(ctx, telemetry.ResourceDeps{Environment: rootConfig.environment})
	if err != nil {
		return nil, fmt.Errorf("create %s telemetry resource: %w", name, err)
	}
	loggerProvider, err := telemetry.NewLoggerProvider(ctx, telemetry.LoggerProviderDeps{
		Resource: resource, Config: otelConfig, LogsConfig: logsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s logger provider: %w", name, err)
	}
	rootLoggerOptions := telemetry.NewRootLoggerOpts().
		WithJSONLogs(rootConfig.jsonLogs != nil && *rootConfig.jsonLogs).
		WithLogLevel(logLevel).
		WithOTELConfigs(otelConfig, logsConfig, loggerProvider)
	if rootConfig.logsFile != nil {
		rootLoggerOptions.WithOptionalOutputFile(*rootConfig.logsFile)
	}
	rootLogger := telemetry.NewRootLogger(rootLoggerOptions)
	shutdownHooks = lifecycle.NewShutdownHooks(lifecycle.ShutdownHooksDeps{
		RootLogger: rootLogger, GracefulShutdownTimeout: rootConfig.gracefulShutdownTimeout,
	})
	if otelConfig.Enabled && logsConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-logger", loggerProvider)
	}
	tracerProvider, err := telemetry.NewTracerProvider(ctx, telemetry.TracerProviderDeps{
		Resource: resource, Config: otelConfig, TracesConfig: tracesConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s tracer provider: %w", name, err)
	}
	if otelConfig.Enabled && tracesConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-tracer", tracerProvider)
	}
	meterProvider, err := telemetry.NewMeterProvider(ctx, telemetry.MeterProviderDeps{
		Resource: resource, Config: otelConfig, MetricsConfig: metricsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s meter provider: %w", name, err)
	}
	if otelConfig.Enabled && metricsConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-meter", meterProvider)
	}
	if err = telemetry.StartPProfListener(telemetry.PProfListenerDeps{
		Enabled: rootConfig.pprofListener.Enabled, Addr: rootConfig.pprofListener.Addr,
	}); err != nil {
		return nil, fmt.Errorf("start %s pprof listener: %w", name, err)
	}
	if err = telemetry.OTELSetup(telemetry.SetupDeps{
		OTELConfig: otelConfig, OTELMetricsConfig: metricsConfig, OTELTracesConfig: tracesConfig,
		OTELLogsConfig: logsConfig, ShutdownHooks: shutdownHooks, MeterProvider: meterProvider,
		TracerProvider: tracerProvider, LoggerProvider: loggerProvider, RootLogger: rootLogger,
		RootLoggerOpts: rootLoggerOptions, ShutdownHooksRegistered: true,
	}); err != nil {
		return nil, fmt.Errorf("set up %s telemetry: %w", name, err)
	}
	database, err := internal.OpenApplicationSQLDB(rootConfig.application.Database.DSN)
	if err != nil {
		return nil, err
	}
	return &processInfrastructure{
		database:   database,
		rootLogger: rootLogger,
		httpTransportFactory: telemetry.NewOtelHTTPTransportFactory(
			telemetry.OtelHTTPTransportFactoryDeps{
				MeterProvider:     meterProvider,
				TracerProvider:    tracerProvider,
				TextMapPropagator: telemetry.NewTextMapPropagator(),
				OTELConfig:        otelConfig,
			},
		),
		shutdownHooks: shutdownHooks,
	}, nil
}

func buildWorker(
	ctx context.Context,
	rootConfig config.WorkerRootConfig,
) (_ *WorkerRoot, err error) { // coverage-ignore
	infrastructure, err := newProcessInfrastructure(ctx, "worker", processInfrastructureConfig{
		environment: rootConfig.Environment, defaultLogLevel: rootConfig.DefaultLogLevel, jsonLogs: rootConfig.JSONLogs,
		logsFile: rootConfig.LogsFile, pprofListener: rootConfig.PProfListener,
		gracefulShutdownTimeout: rootConfig.GracefulShutdownTimeout, openTelemetry: rootConfig.OpenTelemetry,
		application: rootConfig.Application,
	})
	if err != nil {
		return nil, err
	}
	var worker *jobspkg.Worker
	var publisher *appdispatch.Publisher
	infrastructure.shutdownHooks.Register("worker-router-and-application-db", func(shutdownCtx context.Context) error {
		var routerErr error
		if worker != nil {
			routerErr = worker.Stop(shutdownCtx)
		}
		var publisherErr error
		if publisher != nil {
			publisherErr = publisher.Close()
		}
		return errors.Join(routerErr, publisherErr, infrastructure.database.Close())
	})
	defer func() {
		if err == nil {
			return
		}
		if shutdownErr := infrastructure.shutdownHooks.PerformShutdown(context.WithoutCancel(ctx)); shutdownErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up failed worker root: %w", shutdownErr))
		}
	}()
	store, err := jobspkg.NewStore(infrastructure.database, rootConfig.Application.Database.DSN, jobspkg.StoreOpts{
		TablePrefix: rootConfig.Application.Database.TablePrefix + "jobs_",
	})
	if err != nil {
		return nil, fmt.Errorf("build worker jobs store: %w", err)
	}
	publisher, err = appdispatch.NewPublisher(appdispatch.Config{
		DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix,
	},
		infrastructure.database,
		infrastructure.rootLogger,
	)
	if err != nil {
		return nil, fmt.Errorf("build worker dispatch publisher: %w", err)
	}
	routerFactory, err := appdispatch.NewRouterFactory(appdispatch.Config{
		DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix,
	},
		infrastructure.database,
		publisher,
		infrastructure.rootLogger,
	)
	if err != nil {
		return nil, fmt.Errorf("build worker router factory: %w", err)
	}
	registry := jobspkg.NewRegistry()
	worker, err = jobspkg.NewWorker(jobspkg.WorkerDeps{
		Store: store, Registry: registry, Logger: infrastructure.rootLogger,
		Config: jobspkg.WorkerConfig{
			PollInterval: rootConfig.Jobs.Worker.PollInterval, MaxAttempts: rootConfig.Jobs.Worker.MaxAttempts,
			StaleRunningAge: rootConfig.Jobs.Worker.StaleRunningAge,
		},
		RouterFactory: routerFactory,
	})
	if err != nil {
		return nil, fmt.Errorf("build worker: %w", err)
	}
	financeDatabase, err := financeapp.NewDatabase(
		infrastructure.database, rootConfig.Application.Database.DSN, infrastructure.rootLogger,
	)
	if err != nil {
		return nil, fmt.Errorf("build worker finance database: %w", err)
	}
	httpClientFactory := apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{
		RootLogger: infrastructure.rootLogger, RetryAfterFallbackDelay: rootConfig.HTTPClient.RetryAfterFallbackDelay,
		OtelHTTPTransportFactory: infrastructure.httpTransportFactory,
	})
	_, err = buildFinanceModule(financeModuleBuildDeps{
		Database:          financeDatabase,
		Registry:          registry,
		HTTPClientFactory: httpClientFactory,
		Logger:            infrastructure.rootLogger,
		JWTSigningKey:     rootConfig.Auth.JWTSigningKey, Finance: rootConfig.Finance,
	})
	if err != nil {
		return nil, fmt.Errorf("build worker finance handlers: %w", err)
	}
	return &WorkerRoot{Worker: worker, Registry: registry, shutdownHooks: infrastructure.shutdownHooks}, nil
}

func buildScheduler(
	ctx context.Context,
	rootConfig config.SchedulerRootConfig,
) (_ *SchedulerRoot, err error) { // coverage-ignore
	infrastructure, err := newProcessInfrastructure(ctx, "scheduler", processInfrastructureConfig{
		environment: rootConfig.Environment, defaultLogLevel: rootConfig.DefaultLogLevel, jsonLogs: rootConfig.JSONLogs,
		logsFile: rootConfig.LogsFile, pprofListener: rootConfig.PProfListener,
		gracefulShutdownTimeout: rootConfig.GracefulShutdownTimeout, openTelemetry: rootConfig.OpenTelemetry,
		application: rootConfig.Application,
	})
	if err != nil {
		return nil, err
	}
	var publisher *appdispatch.Publisher
	infrastructure.shutdownHooks.Register("scheduler-publisher-and-application-db", func(context.Context) error {
		var publisherErr error
		if publisher != nil {
			publisherErr = publisher.Close()
		}
		return errors.Join(publisherErr, infrastructure.database.Close())
	})
	defer func() {
		if err == nil {
			return
		}
		if shutdownErr := infrastructure.shutdownHooks.PerformShutdown(context.WithoutCancel(ctx)); shutdownErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up failed scheduler root: %w", shutdownErr))
		}
	}()
	publisher, err = appdispatch.NewPublisher(appdispatch.Config{
		DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix,
	},
		infrastructure.database,
		infrastructure.rootLogger,
	)
	if err != nil {
		return nil, fmt.Errorf("build scheduler dispatch publisher: %w", err)
	}
	financeDatabase, err := financeapp.NewDatabase(
		infrastructure.database, rootConfig.Application.Database.DSN, infrastructure.rootLogger,
	)
	if err != nil {
		return nil, fmt.Errorf("build scheduler finance database: %w", err)
	}
	httpClientFactory := apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{
		RootLogger: infrastructure.rootLogger, RetryAfterFallbackDelay: rootConfig.HTTPClient.RetryAfterFallbackDelay,
		OtelHTTPTransportFactory: infrastructure.httpTransportFactory,
	})
	financeModule, err := buildFinanceModule(financeModuleBuildDeps{
		Database: financeDatabase, CommandPublisher: publisher, HTTPClientFactory: httpClientFactory,
		Logger: infrastructure.rootLogger, JWTSigningKey: rootConfig.Auth.JWTSigningKey, Finance: rootConfig.Finance,
	})
	if err != nil {
		return nil, fmt.Errorf("build scheduler finance schedules: %w", err)
	}
	if err = financeModule.FXRefreshScheduleService.EnsureDailySchedule(
		ctx,
		financepkg.FXProviderFrankfurter,
	); err != nil {
		return nil, fmt.Errorf("ensure scheduler fx refresh schedule: %w", err)
	}
	return &SchedulerRoot{
		bankSchedules: financeModule.BankConnectionScheduleService,
		fxSchedules:   financeModule.FXRefreshScheduleService, SchedulerLoopInterval: rootConfig.Scheduler.LoopInterval,
		shutdownHooks: infrastructure.shutdownHooks,
	}, nil
}

// EnqueueDue invokes each finance-owned due service. It only publishes semantic
// commands; observed workers execute the underlying finance work.
func (root *SchedulerRoot) EnqueueDue(ctx context.Context) (int, error) {
	bankCount, err := root.bankSchedules.EnqueueDue(ctx)
	if err != nil { // coverage-ignore // Finance schedule services cover publication-failure propagation.
		return bankCount, fmt.Errorf("enqueue due bank schedules: %w", err)
	}
	fxCount, err := root.fxSchedules.EnqueueDue(ctx)
	if err != nil { // coverage-ignore // Finance schedule services cover publication-failure propagation.
		return bankCount + fxCount, fmt.Errorf("enqueue due fx schedules: %w", err)
	}
	return bankCount + fxCount, nil
}

// Close releases resources registered while constructing this worker root.
func (root *WorkerRoot) Close(ctx context.Context) error { // coverage-ignore
	return root.shutdownHooks.PerformShutdown(context.WithoutCancel(ctx))
}

// Close releases resources registered while constructing this scheduler root.
func (root *SchedulerRoot) Close(ctx context.Context) error { // coverage-ignore
	return root.shutdownHooks.PerformShutdown(context.WithoutCancel(ctx))
}
