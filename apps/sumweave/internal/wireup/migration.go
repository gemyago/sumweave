// Package wireup builds command-specific application capabilities.
package wireup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gemyago/sumweave/apps/sumweave/internal"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/agent"
)

const localEnvironment = "local"

// MigrationOptions are command inputs used to load the db-migrate root.
type MigrationOptions struct {
	Environment     string
	DefaultLogLevel *string
	JSONLogs        *bool
	LogsFile        *string
}

// MigrationRoot owns the resources required for one migration execution.
type MigrationRoot struct {
	migrator      *internal.DatabaseMigrator
	shutdownHooks *lifecycle.ShutdownHooks
}

// BuildMigration loads typed configuration and eagerly constructs only the
// dependencies required by db-migrate.
func BuildMigration(ctx context.Context, options MigrationOptions) (*MigrationRoot, error) {
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
		return nil, fmt.Errorf("load migration configuration: %w", err)
	}
	rootConfig, err := values.MigrationRoot(environment)
	if err != nil {
		return nil, err
	}
	return buildMigration(ctx, rootConfig)
}

//nolint:gocognit,funlen // The visible ordered construction is the wireup root's purpose.
func buildMigration(
	ctx context.Context,
	rootConfig config.MigrationRootConfig,
) (_ *MigrationRoot, err error) { // coverage-ignore
	var shutdownHooks *lifecycle.ShutdownHooks
	defer func() {
		if err == nil || shutdownHooks == nil {
			return
		}
		if shutdownErr := shutdownHooks.PerformShutdown(context.WithoutCancel(ctx)); shutdownErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up failed migration root: %w", shutdownErr))
		}
	}()

	var logLevel slog.Level
	if err = logLevel.UnmarshalText([]byte(rootConfig.DefaultLogLevel)); err != nil {
		return nil, fmt.Errorf("parse migration log level: %w", err)
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
		return nil, fmt.Errorf("create migration telemetry resource: %w", err)
	}
	loggerProvider, err := telemetry.NewLoggerProvider(ctx, telemetry.LoggerProviderDeps{
		Resource: resource, Config: otelConfig, LogsConfig: otelLogsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create migration logger provider: %w", err)
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
		RootLogger:              rootLogger,
		GracefulShutdownTimeout: rootConfig.GracefulShutdownTimeout,
	})
	if otelConfig.Enabled && otelLogsConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-logger", loggerProvider)
	}

	tracerProvider, err := telemetry.NewTracerProvider(ctx, telemetry.TracerProviderDeps{
		Resource: resource, Config: otelConfig, TracesConfig: otelTracesConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create migration tracer provider: %w", err)
	}
	if otelConfig.Enabled && otelTracesConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-tracer", tracerProvider)
	}
	meterProvider, err := telemetry.NewMeterProvider(ctx, telemetry.MeterProviderDeps{
		Resource: resource, Config: otelConfig, MetricsConfig: otelMetricsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create migration meter provider: %w", err)
	}
	if otelConfig.Enabled && otelMetricsConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-meter", meterProvider)
	}

	if err = telemetry.StartPProfListener(telemetry.PProfListenerDeps{
		Enabled: rootConfig.PProfListener.Enabled,
		Addr:    rootConfig.PProfListener.Addr,
	}); err != nil {
		return nil, fmt.Errorf("start migration pprof listener: %w", err)
	}
	if err = telemetry.OTELSetup(telemetry.SetupDeps{
		OTELConfig:              otelConfig,
		OTELMetricsConfig:       otelMetricsConfig,
		OTELTracesConfig:        otelTracesConfig,
		OTELLogsConfig:          otelLogsConfig,
		ShutdownHooks:           shutdownHooks,
		MeterProvider:           meterProvider,
		TracerProvider:          tracerProvider,
		LoggerProvider:          loggerProvider,
		RootLogger:              rootLogger,
		RootLoggerOpts:          rootLoggerOptions,
		ShutdownHooksRegistered: true,
	}); err != nil {
		return nil, fmt.Errorf("set up migration telemetry: %w", err)
	}

	database, err := internal.NewApplicationSQLDB(rootConfig.Application.Database.DSN, shutdownHooks)
	if err != nil {
		return nil, err
	}
	userStore, err := auth.NewUserStore(auth.UserStoreDeps{
		SQLDB:       database,
		DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_",
		IDGen:       ident.NewDefaultGenerator(),
		Logger:      rootLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("create migration auth user store: %w", err)
	}
	refreshTokenStore, err := auth.NewRefreshTokenStore(auth.RefreshTokenStoreDeps{
		SQLDB:       database,
		DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_",
		Logger:      rootLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("create migration auth refresh token store: %w", err)
	}

	return &MigrationRoot{
		migrator: internal.NewDatabaseMigrator(internal.DatabaseMigrationDeps{
			RootLogger:                      rootLogger,
			AgentRuntimeStorageType:         rootConfig.AgentRuntime.Storage.Type,
			AgentRuntimeDatabaseDSN:         rootConfig.AgentRuntime.Database.DSN,
			AgentRuntimeDatabaseTablePrefix: rootConfig.AgentRuntime.Database.TablePrefix,
			ApplicationDatabaseDSN:          rootConfig.Application.Database.DSN,
			ApplicationDatabaseTablePrefix:  rootConfig.Application.Database.TablePrefix,
			ApplicationSQLDB:                database,
			AuthUsers:                       userStore,
			AuthRefreshTokens:               refreshTokenStore,
			AgentRuntimeMigrator: internal.AgentRuntimeMigratorFunc(func() error {
				providers, providersErr := agent.NewDatabaseProvidersConfigService(
					rootConfig.AgentRuntime.Database.DSN,
					rootLogger,
					rootConfig.AgentRuntime.Database.TablePrefix,
				)
				if providersErr != nil {
					return fmt.Errorf("create providers config service: %w", providersErr)
				}
				profiles, profilesErr := agent.NewDatabaseAgentProfilesService(
					rootConfig.AgentRuntime.Database.DSN,
					rootLogger,
					rootConfig.AgentRuntime.Database.TablePrefix,
				)
				if profilesErr != nil {
					return fmt.Errorf("create database agent profiles service: %w", profilesErr)
				}
				runner, runnerErr := agent.NewRunner(
					agent.RunnerArgs{ProvidersConfigService: providers, AgentProfilesService: profiles},
					agent.WithLogger(rootLogger),
					agent.WithDatabaseStorage(rootConfig.AgentRuntime.Database.DSN),
					agent.WithDatabaseTablePrefix(rootConfig.AgentRuntime.Database.TablePrefix),
				)
				if runnerErr != nil {
					return fmt.Errorf("create agent runner: %w", runnerErr)
				}
				if migrateErr := runner.AutoMigrate(); migrateErr != nil {
					return fmt.Errorf("auto migrate sessions database: %w", migrateErr)
				}
				if migrateErr := profiles.AutoMigrate(); migrateErr != nil {
					return fmt.Errorf("auto migrate agent profiles database: %w", migrateErr)
				}
				migrator, ok := providers.(interface{ AutoMigrate() error })
				if !ok {
					return errors.New("database providers config service does not support auto migration")
				}
				if migrateErr := migrator.AutoMigrate(); migrateErr != nil {
					return fmt.Errorf("auto migrate providers config database: %w", migrateErr)
				}
				return nil
			}),
		}),
		shutdownHooks: shutdownHooks,
	}, nil
}

// Migrate runs the schema migrations and closes the resources owned by this root.
func (root *MigrationRoot) Migrate(ctx context.Context) error {
	migrationErr := root.migrator.Migrate(ctx)
	shutdownErr := root.shutdownHooks.PerformShutdown(context.WithoutCancel(ctx))
	if shutdownErr != nil {
		return errors.Join(migrationErr, fmt.Errorf("shut down migration root: %w", shutdownErr))
	}
	return migrationErr
}
