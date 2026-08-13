package wireup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gemyago/sumweave/apps/sumweave/internal"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/financeapp"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
)

// JobsOptions are command inputs used to load a split jobs root.
type JobsOptions struct {
	Environment     string
	DefaultLogLevel *string
	JSONLogs        *bool
	LogsFile        *string
}

// JobsRoot owns the durable jobs capabilities and resources for one split-mode
// command. It deliberately exposes no HTTP capability.
type JobsRoot struct {
	Store     *jobspkg.Store
	Worker    *jobspkg.Worker
	Scheduler *jobspkg.Scheduler
	Registry  *jobspkg.Registry

	shutdownHooks *lifecycle.ShutdownHooks
}

// BuildJobs loads typed configuration and eagerly constructs jobs and finance.
// Finance registration finishes before this function returns; commands start
// worker or scheduler loops only after receiving the root.
func BuildJobs(ctx context.Context, options JobsOptions) (*JobsRoot, error) { // coverage-ignore
	environment := options.Environment
	if environment == "" {
		environment = localEnvironment
	}
	values, err := config.LoadValues(config.ValuesLoadInput{
		Environment: environment,
		CLI: config.CLIOverrides{
			DefaultLogLevel: options.DefaultLogLevel,
			JSONLogs:        options.JSONLogs,
			LogsFile:        options.LogsFile,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("load jobs configuration: %w", err)
	}
	rootConfig, err := values.JobsRoot(environment)
	if err != nil {
		return nil, err
	}
	return buildJobs(ctx, rootConfig)
}

//nolint:gocognit,funlen // Visible order is the wireup root's purpose.
func buildJobs(ctx context.Context, rootConfig config.JobsRootConfig) (_ *JobsRoot, err error) { // coverage-ignore
	var shutdownHooks *lifecycle.ShutdownHooks
	defer func() {
		if err == nil || shutdownHooks == nil {
			return
		}
		if shutdownErr := shutdownHooks.PerformShutdown(context.WithoutCancel(ctx)); shutdownErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up failed jobs root: %w", shutdownErr))
		}
	}()

	var logLevel slog.Level
	if err = logLevel.UnmarshalText([]byte(rootConfig.DefaultLogLevel)); err != nil {
		return nil, fmt.Errorf("parse jobs log level: %w", err)
	}
	otelConfig := telemetry.OTELConfig{
		Enabled:        rootConfig.OpenTelemetry.Enabled,
		RuntimeMetrics: rootConfig.OpenTelemetry.RuntimeMetrics,
	}
	otelTracesConfig := telemetry.OTELTracesConfig{
		Enabled:       rootConfig.OpenTelemetry.Traces.Enabled,
		Endpoint:      rootConfig.OpenTelemetry.Traces.Endpoint,
		URLPath:       rootConfig.OpenTelemetry.Traces.URLPath,
		Protocol:      rootConfig.OpenTelemetry.Traces.Protocol,
		SamplingRate:  rootConfig.OpenTelemetry.Traces.SamplingRate,
		AuthToken:     rootConfig.OpenTelemetry.Traces.Auth.Token,
		AuthTokenType: rootConfig.OpenTelemetry.Traces.Auth.TokenType,
	}
	otelMetricsConfig := telemetry.OTELMetricsConfig{
		Enabled:        rootConfig.OpenTelemetry.Metrics.Enabled,
		Endpoint:       rootConfig.OpenTelemetry.Metrics.Endpoint,
		URLPath:        rootConfig.OpenTelemetry.Metrics.URLPath,
		Protocol:       rootConfig.OpenTelemetry.Metrics.Protocol,
		ExportInterval: rootConfig.OpenTelemetry.Metrics.ExportInterval,
		AuthToken:      rootConfig.OpenTelemetry.Metrics.Auth.Token,
		AuthTokenType:  rootConfig.OpenTelemetry.Metrics.Auth.TokenType,
	}
	otelLogsConfig := telemetry.OTELLogsConfig{
		Enabled:              rootConfig.OpenTelemetry.Logs.Enabled,
		DefaultHandlerFanout: rootConfig.OpenTelemetry.Logs.DefaultHandlerFanout,
		Endpoint:             rootConfig.OpenTelemetry.Logs.Endpoint,
		URLPath:              rootConfig.OpenTelemetry.Logs.URLPath,
		Protocol:             rootConfig.OpenTelemetry.Logs.Protocol,
		AuthToken:            rootConfig.OpenTelemetry.Logs.Auth.Token,
		AuthTokenType:        rootConfig.OpenTelemetry.Logs.Auth.TokenType,
	}
	resource, err := telemetry.NewResource(ctx, telemetry.ResourceDeps{Environment: rootConfig.Environment})
	if err != nil {
		return nil, fmt.Errorf("create jobs telemetry resource: %w", err)
	}
	loggerProvider, err := telemetry.NewLoggerProvider(ctx, telemetry.LoggerProviderDeps{
		Resource: resource, Config: otelConfig, LogsConfig: otelLogsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create jobs logger provider: %w", err)
	}
	jsonLogs := false
	if rootConfig.JSONLogs != nil {
		jsonLogs = *rootConfig.JSONLogs
	}
	rootLoggerOptions := telemetry.NewRootLoggerOpts().
		WithJSONLogs(jsonLogs).
		WithLogLevel(logLevel).
		WithOTELConfigs(otelConfig, otelLogsConfig, loggerProvider)
	if rootConfig.LogsFile != nil {
		rootLoggerOptions.WithOptionalOutputFile(*rootConfig.LogsFile)
	}
	rootLogger := telemetry.NewRootLogger(rootLoggerOptions)
	shutdownHooks = lifecycle.NewShutdownHooks(lifecycle.ShutdownHooksDeps{
		RootLogger: rootLogger, GracefulShutdownTimeout: rootConfig.GracefulShutdownTimeout,
	})
	if otelConfig.Enabled && otelLogsConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-logger", loggerProvider)
	}
	tracerProvider, err := telemetry.NewTracerProvider(ctx, telemetry.TracerProviderDeps{
		Resource: resource, Config: otelConfig, TracesConfig: otelTracesConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create jobs tracer provider: %w", err)
	}
	if otelConfig.Enabled && otelTracesConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-tracer", tracerProvider)
	}
	meterProvider, err := telemetry.NewMeterProvider(ctx, telemetry.MeterProviderDeps{
		Resource: resource, Config: otelConfig, MetricsConfig: otelMetricsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create jobs meter provider: %w", err)
	}
	if otelConfig.Enabled && otelMetricsConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-meter", meterProvider)
	}
	if err = telemetry.StartPProfListener(telemetry.PProfListenerDeps{
		Enabled: rootConfig.PProfListener.Enabled, Addr: rootConfig.PProfListener.Addr,
	}); err != nil {
		return nil, fmt.Errorf("start jobs pprof listener: %w", err)
	}
	if err = telemetry.OTELSetup(telemetry.SetupDeps{
		OTELConfig: otelConfig, OTELMetricsConfig: otelMetricsConfig, OTELTracesConfig: otelTracesConfig,
		OTELLogsConfig: otelLogsConfig, ShutdownHooks: shutdownHooks, MeterProvider: meterProvider,
		TracerProvider: tracerProvider, LoggerProvider: loggerProvider, RootLogger: rootLogger,
		RootLoggerOpts: rootLoggerOptions, ShutdownHooksRegistered: true,
	}); err != nil {
		return nil, fmt.Errorf("set up jobs telemetry: %w", err)
	}

	database, err := internal.OpenApplicationSQLDB(rootConfig.Application.Database.DSN)
	if err != nil {
		return nil, err
	}
	var jobsModule *jobspkg.Module
	shutdownHooks.Register("jobs-messaging-and-application-db", func(shutdownCtx context.Context) error {
		var messagingErr error
		if jobsModule != nil {
			messagingErr = jobsModule.Close(shutdownCtx)
		}
		return errors.Join(messagingErr, database.Close())
	})
	transportFactory := telemetry.NewOtelHTTPTransportFactory(telemetry.OtelHTTPTransportFactoryDeps{
		MeterProvider: meterProvider, TracerProvider: tracerProvider,
		TextMapPropagator: telemetry.NewTextMapPropagator(), OTELConfig: otelConfig,
	})
	httpClientFactory := apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{
		RootLogger: rootLogger, RetryAfterFallbackDelay: rootConfig.HTTPClient.RetryAfterFallbackDelay,
		OtelHTTPTransportFactory: transportFactory,
	})
	jobsModule, err = jobspkg.NewModule(jobspkg.ModuleDeps{
		SQLDB: database, DatabaseDSN: rootConfig.Application.Database.DSN,
		DatabaseTablePrefix: rootConfig.Application.Database.TablePrefix, Logger: rootLogger,
		WorkerConfig: jobspkg.WorkerConfig{
			Enabled: rootConfig.Jobs.Worker.Enabled, PollInterval: rootConfig.Jobs.Worker.PollInterval,
			MaxAttempts: rootConfig.Jobs.Worker.MaxAttempts,
		},
		IDGenerator: ident.NewDefaultGenerator(),
	})
	if err != nil {
		return nil, fmt.Errorf("build jobs module: %w", err)
	}
	financeDatabase, err := financeapp.NewDatabase(database, rootConfig.Application.Database.DSN, rootLogger)
	if err != nil {
		return nil, fmt.Errorf("build finance database: %w", err)
	}
	_, err = buildFinanceModule(financeModuleBuildDeps{
		Database: financeDatabase, Jobs: jobsModule, HTTPClientFactory: httpClientFactory, Logger: rootLogger,
		JWTSigningKey: rootConfig.Auth.JWTSigningKey, Finance: rootConfig.Finance,
	})
	if err != nil {
		return nil, fmt.Errorf("build finance module: %w", err)
	}
	return &JobsRoot{
		Store:         jobsModule.Store,
		Worker:        jobsModule.Worker,
		Scheduler:     jobsModule.Scheduler,
		Registry:      jobsModule.Registry,
		shutdownHooks: shutdownHooks,
	}, nil
}

// Close releases resources registered while constructing this jobs root.
func (root *JobsRoot) Close(ctx context.Context) error {
	return root.shutdownHooks.PerformShutdown(context.WithoutCancel(ctx))
}
