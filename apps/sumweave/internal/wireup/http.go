package wireup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdhttp "net/http"

	"github.com/gemyago/sumweave/apps/sumweave/internal"
	apphttp "github.com/gemyago/sumweave/apps/sumweave/internal/api/http"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1controllers"
	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/financeapp"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/agent"
)

// HTTPOptions are command or embedder inputs for the API-only HTTP root.
type HTTPOptions struct {
	Environment     string
	DefaultLogLevel *string
	JSONLogs        *bool
	LogsFile        *string
}

// HTTPRoot owns the fully composed API application and no durable worker or
// scheduler resources.
type HTTPRoot struct {
	Handler       stdhttp.Handler
	Server        *server.HTTPServer
	Runner        *agent.Runner
	ToolsRegistry *agent.ToolsRegistry
	rootLogger    *slog.Logger
	shutdownHooks *lifecycle.ShutdownHooks
}

// BuildHTTP loads typed configuration and eagerly constructs the API-only HTTP
// application. It never starts the durable worker.
func BuildHTTP(
	ctx context.Context,
	options HTTPOptions,
) (*HTTPRoot, error) {
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
		return nil, fmt.Errorf("load HTTP configuration: %w", err)
	}
	rootConfig, err := values.HTTPRoot(environment)
	if err != nil {
		return nil, err
	}
	return buildHTTP(ctx, rootConfig)
}

//
//nolint:gocognit,gocyclo,cyclop,funlen // Construction order is deliberately visible at this root.
func buildHTTP(
	ctx context.Context,
	rootConfig config.HTTPRootConfig,
) (_ *HTTPRoot, err error) { // coverage-ignore
	var shutdownHooks *lifecycle.ShutdownHooks
	defer func() {
		if err == nil || shutdownHooks == nil {
			return
		}
		if shutdownErr := shutdownHooks.PerformShutdown(context.WithoutCancel(ctx)); shutdownErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up failed HTTP root: %w", shutdownErr))
		}
	}()

	logLevel, err := parseLogLevel(rootConfig.DefaultLogLevel)
	if err != nil {
		return nil, err
	}
	otelConfig, tracesConfig, metricsConfig, logsConfig := makeTelemetryConfigs(
		rootConfig.OpenTelemetry,
	)
	resource, err := telemetry.NewResource(
		ctx,
		telemetry.ResourceDeps{Environment: rootConfig.Environment},
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP telemetry resource: %w", err)
	}
	loggerProvider, err := telemetry.NewLoggerProvider(ctx, telemetry.LoggerProviderDeps{
		Resource: resource, Config: otelConfig, LogsConfig: logsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create HTTP logger provider: %w", err)
	}
	rootLoggerOptions := telemetry.NewRootLoggerOpts().
		WithJSONLogs(rootConfig.JSONLogs != nil && *rootConfig.JSONLogs).
		WithLogLevel(logLevel).
		WithOTELConfigs(otelConfig, logsConfig, loggerProvider)
	if rootConfig.LogsFile != nil {
		rootLoggerOptions.WithOptionalOutputFile(*rootConfig.LogsFile)
	}
	rootLogger := telemetry.NewRootLogger(rootLoggerOptions)
	shutdownHooks = lifecycle.NewShutdownHooks(lifecycle.ShutdownHooksDeps{
		RootLogger: rootLogger, GracefulShutdownTimeout: rootConfig.GracefulShutdownTimeout,
	})
	if otelConfig.Enabled && logsConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-logger", loggerProvider)
	}
	tracerProvider, err := telemetry.NewTracerProvider(ctx, telemetry.TracerProviderDeps{
		Resource: resource, Config: otelConfig, TracesConfig: tracesConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create HTTP tracer provider: %w", err)
	}
	if otelConfig.Enabled && tracesConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-tracer", tracerProvider)
	}
	meterProvider, err := telemetry.NewMeterProvider(ctx, telemetry.MeterProviderDeps{
		Resource: resource, Config: otelConfig, MetricsConfig: metricsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create HTTP meter provider: %w", err)
	}
	if otelConfig.Enabled && metricsConfig.Enabled {
		telemetry.RegisterShutdownHook(rootLogger, shutdownHooks, "otel-meter", meterProvider)
	}
	if err = telemetry.StartPProfListener(telemetry.PProfListenerDeps{
		Enabled: rootConfig.PProfListener.Enabled, Addr: rootConfig.PProfListener.Addr,
	}); err != nil {
		return nil, fmt.Errorf("start HTTP pprof listener: %w", err)
	}
	if err = telemetry.OTELSetup(telemetry.SetupDeps{
		OTELConfig: otelConfig, OTELMetricsConfig: metricsConfig, OTELTracesConfig: tracesConfig,
		OTELLogsConfig: logsConfig, ShutdownHooks: shutdownHooks, MeterProvider: meterProvider,
		TracerProvider: tracerProvider, LoggerProvider: loggerProvider, RootLogger: rootLogger,
		RootLoggerOpts: rootLoggerOptions, ShutdownHooksRegistered: true,
	}); err != nil {
		return nil, fmt.Errorf("set up HTTP telemetry: %w", err)
	}

	database, err := internal.OpenApplicationSQLDB(rootConfig.Application.Database.DSN)
	if err != nil {
		return nil, err
	}
	var publisher *appdispatch.Publisher
	shutdownHooks.Register(
		"api-messaging-and-application-db",
		func(context.Context) error {
			var messagingErr error
			if publisher != nil {
				messagingErr = publisher.Close()
			}
			return errors.Join(messagingErr, database.Close())
		},
	)
	ids := ident.NewDefaultGenerator()
	runtime, err := internal.NewRuntime(internal.RuntimeDeps{
		RootLogger:                      rootLogger,
		DataDir:                         rootConfig.DataDir,
		PlatformAgentsPath:              rootConfig.WorkspaceFS.PlatformAgentsPath,
		ExecEnabled:                     rootConfig.WorkspaceFS.Exec.Enabled,
		ExecMaxOutputBytes:              rootConfig.WorkspaceFS.Exec.MaxOutputBytes,
		ExecDefaultTimeout:              rootConfig.WorkspaceFS.Exec.DefaultTimeout,
		ExecMaxConcurrentJobs:           rootConfig.WorkspaceFS.Exec.MaxConcurrentJobs,
		AgentRuntimeStorageType:         rootConfig.AgentRuntime.Storage.Type,
		AgentRuntimeDatabaseDSN:         rootConfig.AgentRuntime.Database.DSN,
		AgentRuntimeDatabaseTablePrefix: rootConfig.AgentRuntime.Database.TablePrefix,
		SkillsEnabled:                   rootConfig.Skills.Enabled,
		SkillsPaths:                     rootConfig.Skills.Paths,
		SkillsMaxSkillBytes:             rootConfig.Skills.MaxSkillBytes,
		SkillsMaxCatalogEntries:         rootConfig.Skills.MaxCatalogEntries,
		ToolsRegistry:                   agent.NewToolsRegistry(),
	})
	if err != nil {
		return nil, fmt.Errorf("build agent runtime: %w", err)
	}
	userStore, err := auth.NewUserStore(auth.UserStoreDeps{
		SQLDB: database, DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_", IDGen: ids, Logger: rootLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("create HTTP auth user store: %w", err)
	}
	refreshTokenStore, err := auth.NewRefreshTokenStore(auth.RefreshTokenStoreDeps{
		SQLDB: database, DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_", Logger: rootLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("create HTTP refresh token store: %w", err)
	}
	jwtService, err := auth.NewJWTService(auth.JWTServiceDeps{
		SigningKey: rootConfig.Auth.JWTSigningKey, AccessTokenTTL: rootConfig.Auth.AccessTokenTTL, Logger: rootLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("create HTTP JWT service: %w", err)
	}
	authService := auth.NewAuthService(auth.ServiceDeps{
		UserStore: userStore, JWTService: jwtService, RefreshTokenStore: refreshTokenStore,
		PasswordHasher: auth.NewArgon2idHasher(), RefreshTokenTTL: rootConfig.Auth.RefreshTokenTTL, Logger: rootLogger,
	})
	userDirectory, err := app.NewUserDirectory(authService)
	if err != nil {
		return nil, fmt.Errorf("create HTTP user directory: %w", err)
	}
	transportFactory := telemetry.NewOtelHTTPTransportFactory(
		telemetry.OtelHTTPTransportFactoryDeps{
			MeterProvider:     meterProvider,
			TracerProvider:    tracerProvider,
			TextMapPropagator: telemetry.NewTextMapPropagator(),
			OTELConfig:        otelConfig,
		},
	)
	httpClientFactory := apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{
		RootLogger: rootLogger, RetryAfterFallbackDelay: rootConfig.HTTPClient.RetryAfterFallbackDelay,
		OtelHTTPTransportFactory: transportFactory,
	})
	jobsStore, err := jobspkg.NewStore(
		database,
		rootConfig.Application.Database.DSN,
		jobspkg.StoreOpts{TablePrefix: rootConfig.Application.Database.TablePrefix + "jobs_"},
	)
	if err != nil {
		return nil, fmt.Errorf("build HTTP jobs store: %w", err)
	}
	jobsService, err := jobspkg.NewService(jobspkg.ServiceDeps{Store: jobsStore})
	if err != nil {
		return nil, fmt.Errorf("build HTTP jobs service: %w", err)
	}
	publisher, err = appdispatch.NewPublisher(appdispatch.Config{
		DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix,
	},
		database,
		rootLogger,
	)
	if err != nil {
		return nil, fmt.Errorf("build HTTP dispatch publisher: %w", err)
	}
	financeDatabase, err := financeapp.NewDatabase(
		database,
		rootConfig.Application.Database.DSN,
		rootLogger,
	)
	if err != nil {
		return nil, fmt.Errorf("build HTTP finance database: %w", err)
	}
	financeModule, err := buildFinanceModule(financeModuleBuildDeps{
		Database:          financeDatabase,
		CommandPublisher:  publisher,
		HTTPClientFactory: httpClientFactory,
		Logger:            rootLogger,
		JWTSigningKey:     rootConfig.Auth.JWTSigningKey,
		Finance:           rootConfig.Finance,
	})
	if err != nil {
		return nil, fmt.Errorf("build HTTP finance module: %w", err)
	}

	authMiddleware := middleware.NewAuthMiddleware(middleware.AuthMiddlewareDeps{
		JWTValidator: jwtService,
		Logger:       rootLogger,
	})
	otelMiddleware := telemetry.NewOtelHTTPMiddleware(telemetry.OtelMiddlewareFactoryDeps{
		MeterProvider:     meterProvider,
		TracerProvider:    tracerProvider,
		TextMapPropagator: telemetry.NewTextMapPropagator(),
		OTELConfig:        otelConfig,
	})
	routerMiddleware := server.NewRouterMiddleware(server.RouterMiddlewareDeps{
		RootLogger:      rootLogger,
		AccessLogsLevel: rootConfig.HTTPServer.AccessLogsLevel,
		OTELMiddleware:  otelMiddleware,
		IDGen:           ids,
	})
	router := server.NewHTTPRouter(server.HTTPRouterDeps{Middleware: routerMiddleware})
	rootHandler := server.NewRootHandler(
		server.RootHandlerDeps{RootLogger: rootLogger, Router: router},
	)
	apphttp.SetupV1Routes(apphttp.V1RoutesDeps{
		HealthController: &v1controllers.HealthController{},
		AuthController: v1controllers.NewAuthController(v1controllers.AuthControllerDeps{
			AuthService:    authService,
			AuthMiddleware: authMiddleware,
		}),
		JobsController: v1controllers.NewJobsController(v1controllers.JobsControllerDeps{
			JobsService:    jobsService,
			AuthMiddleware: authMiddleware,
		}),
		FinanceController: v1controllers.NewFinanceController(v1controllers.FinanceControllerDeps{
			TenantService:                financeModule.TenantService,
			UserDirectory:                userDirectory,
			CatalogService:               financeModule.CatalogService,
			LedgerService:                financeModule.LedgerService,
			TransferDetailService:        financeModule.TransferDetailService,
			BankSyncService:              financeModule.BankSyncService,
			ReportingService:             financeModule.ReportingService,
			FXService:                    financeModule.FXService,
			ProviderSnapshotService:      financeModule.ProviderSnapshotService,
			CSVImportService:             financeModule.CSVImportService,
			BankConnectionService:        financeModule.BankConnectionService,
			SyntheticLinkStateService:    financeModule.SyntheticLinkStateService,
			AuthMiddleware:               authMiddleware,
			EnableBankingCallbackBaseURL: rootConfig.Finance.Providers.EnableBanking.CallbackBaseURL,
		}),
		RootHandler:           rootHandler,
		HTTPRouter:            router,
		AuthMiddleware:        authMiddleware,
		RuntimeHandler:        runtime.HTTPHandler,
		RootLogger:            rootLogger,
		BankConnectionService: financeModule.BankConnectionService,
	})
	httpServer := server.NewHTTPServer(server.HTTPServerDeps{
		ShutdownHooks:     shutdownHooks,
		RootLogger:        rootLogger,
		Host:              rootConfig.HTTPServer.Host,
		Port:              rootConfig.HTTPServer.Port,
		TLSCertFile:       rootConfig.HTTPServer.TLS.CertFile,
		TLSKeyFile:        rootConfig.HTTPServer.TLS.KeyFile,
		IdleTimeout:       rootConfig.HTTPServer.IdleTimeout,
		ReadHeaderTimeout: rootConfig.HTTPServer.ReadHeaderTimeout,
		ReadTimeout:       rootConfig.HTTPServer.ReadTimeout,
		WriteTimeout:      rootConfig.HTTPServer.WriteTimeout,
		AccessLogsLevel:   rootConfig.HTTPServer.AccessLogsLevel,
		Handler:           router,
		OTELMiddleware:    otelMiddleware,
	})
	return &HTTPRoot{
		Handler:       router,
		Server:        httpServer,
		Runner:        runtime.Runner,
		ToolsRegistry: runtime.ToolsRegistry,
		rootLogger:    rootLogger,
		shutdownHooks: shutdownHooks,
	}, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return slog.LevelInfo, fmt.Errorf("parse HTTP log level: %w", err)
	}
	return level, nil
}

func makeTelemetryConfigs(values config.OpenTelemetry) (
	telemetry.OTELConfig,
	telemetry.OTELTracesConfig,
	telemetry.OTELMetricsConfig,
	telemetry.OTELLogsConfig,
) {
	return telemetry.OTELConfig{
			Enabled: values.Enabled, RuntimeMetrics: values.RuntimeMetrics,
		}, telemetry.OTELTracesConfig{
			Enabled: values.Traces.Enabled, Endpoint: values.Traces.Endpoint, URLPath: values.Traces.URLPath,
			Protocol: values.Traces.Protocol, SamplingRate: values.Traces.SamplingRate,
			AuthToken: values.Traces.Auth.Token, AuthTokenType: values.Traces.Auth.TokenType,
		}, telemetry.OTELMetricsConfig{
			Enabled: values.Metrics.Enabled, Endpoint: values.Metrics.Endpoint, URLPath: values.Metrics.URLPath,
			Protocol: values.Metrics.Protocol, ExportInterval: values.Metrics.ExportInterval,
			AuthToken: values.Metrics.Auth.Token, AuthTokenType: values.Metrics.Auth.TokenType,
		}, telemetry.OTELLogsConfig{
			Enabled: values.Logs.Enabled, DefaultHandlerFanout: values.Logs.DefaultHandlerFanout,
			Endpoint: values.Logs.Endpoint, URLPath: values.Logs.URLPath, Protocol: values.Logs.Protocol,
			AuthToken: values.Logs.Auth.Token, AuthTokenType: values.Logs.Auth.TokenType,
		}
}

// StartHTTPServer starts the HTTP listener unless noop is requested. The
// caller owns signal cancellation through ctx; this root only owns resources.
func (root *HTTPRoot) StartHTTPServer(
	ctx context.Context,
	noop bool,
) error {
	if noop {
		root.rootLogger.InfoContext(ctx, "NOOP: Starting http server")
		return root.Close(context.WithoutCancel(ctx))
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- root.Server.Start(ctx) // coverage-ignore // server lifecycle is covered by the server package.
	}()
	select {
	case serverErr := <-serverErrors:
		return errors.Join(serverErr, root.Close(context.WithoutCancel(ctx)))
	case <-ctx.Done():
		if err := root.Close(context.WithoutCancel(ctx)); err != nil {
			return err
		}
		return <-serverErrors
	}
}

// Close releases resources when a caller builds but does not start this root.
func (root *HTTPRoot) Close(
	ctx context.Context,
) error { // coverage-ignore // normal startup owns this lifecycle path.
	return root.shutdownHooks.PerformShutdown(context.WithoutCancel(ctx))
}

// Logger returns the root logger for command orchestration.
func (root *HTTPRoot) Logger() *slog.Logger { return root.rootLogger }
