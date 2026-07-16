package config

import (
	"fmt"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"github.com/spf13/viper"
	"go.uber.org/dig"
)

type configValueProvider struct {
	cfg        *viper.Viper
	configPath string
	diPath     string
}

func provideConfigValue(cfg *viper.Viper, path string) configValueProvider {
	if !cfg.IsSet(path) {
		panic(fmt.Errorf("config key not found: %s", path))
	}
	return configValueProvider{cfg, path, "config." + path}
}

func (p configValueProvider) asInt() di.ConstructorWithOpts {
	return di.ProvideValue(p.cfg.GetInt(p.configPath), dig.Name(p.diPath))
}

func (p configValueProvider) asInt32() di.ConstructorWithOpts {
	return di.ProvideValue(p.cfg.GetInt32(p.configPath), dig.Name(p.diPath))
}

func (p configValueProvider) asInt64() di.ConstructorWithOpts {
	return di.ProvideValue(p.cfg.GetInt64(p.configPath), dig.Name(p.diPath))
}

func (p configValueProvider) asString() di.ConstructorWithOpts {
	return di.ProvideValue(p.cfg.GetString(p.configPath), dig.Name(p.diPath))
}

func (p configValueProvider) asBool() di.ConstructorWithOpts {
	return di.ProvideValue(p.cfg.GetBool(p.configPath), dig.Name(p.diPath))
}

func (p configValueProvider) asDuration() di.ConstructorWithOpts {
	return di.ProvideValue(p.cfg.GetDuration(p.configPath), dig.Name(p.diPath))
}

func (p configValueProvider) asFloat64() di.ConstructorWithOpts {
	return di.ProvideValue(p.cfg.GetFloat64(p.configPath), dig.Name(p.diPath))
}

func (p configValueProvider) asStringSlice() di.ConstructorWithOpts {
	return di.ProvideValue(p.cfg.GetStringSlice(p.configPath), dig.Name(p.diPath))
}

func Provide(container *dig.Container, cfg *viper.Viper) error {
	return di.ProvideAll(
		container,
		// env should only be used for tracing/debugging purposes
		provideConfigValue(cfg, "env").asString(),

		provideConfigValue(cfg, "dataDir").asString(),

		// pprof listener config
		provideConfigValue(cfg, "pprofListener.enabled").asBool(),
		provideConfigValue(cfg, "pprofListener.addr").asString(),

		provideConfigValue(cfg, "gracefulShutdownTimeout").asDuration(),

		// http server config
		provideConfigValue(cfg, "httpServer.host").asString(),
		provideConfigValue(cfg, "httpServer.port").asInt(),
		provideConfigValue(cfg, "httpServer.tls.certFile").asString(),
		provideConfigValue(cfg, "httpServer.tls.keyFile").asString(),
		provideConfigValue(cfg, "httpServer.idleTimeout").asDuration(),
		provideConfigValue(cfg, "httpServer.readHeaderTimeout").asDuration(),
		provideConfigValue(cfg, "httpServer.readTimeout").asDuration(),
		provideConfigValue(cfg, "httpServer.writeTimeout").asDuration(),
		provideConfigValue(cfg, "httpServer.accessLogsLevel").asString(),

		// opentelemetry config
		provideConfigValue(cfg, "openTelemetry.enabled").asBool(),
		provideConfigValue(cfg, "openTelemetry.runtimeMetrics").asBool(),
		provideConfigValue(cfg, "openTelemetry.traces.enabled").asBool(),
		provideConfigValue(cfg, "openTelemetry.traces.endpoint").asString(),
		provideConfigValue(cfg, "openTelemetry.traces.urlPath").asString(),
		provideConfigValue(cfg, "openTelemetry.traces.protocol").asString(),
		provideConfigValue(cfg, "openTelemetry.traces.samplingRate").asFloat64(),
		provideConfigValue(cfg, "openTelemetry.traces.auth.token").asString(),
		provideConfigValue(cfg, "openTelemetry.traces.auth.tokenType").asString(),

		provideConfigValue(cfg, "openTelemetry.metrics.enabled").asBool(),
		provideConfigValue(cfg, "openTelemetry.metrics.endpoint").asString(),
		provideConfigValue(cfg, "openTelemetry.metrics.urlPath").asString(),
		provideConfigValue(cfg, "openTelemetry.metrics.protocol").asString(),
		provideConfigValue(cfg, "openTelemetry.metrics.exportInterval").asDuration(),
		provideConfigValue(cfg, "openTelemetry.metrics.auth.token").asString(),
		provideConfigValue(cfg, "openTelemetry.metrics.auth.tokenType").asString(),

		provideConfigValue(cfg, "openTelemetry.logs.enabled").asBool(),
		provideConfigValue(cfg, "openTelemetry.logs.defaultHandlerFanout").asBool(),
		provideConfigValue(cfg, "openTelemetry.logs.endpoint").asString(),
		provideConfigValue(cfg, "openTelemetry.logs.urlPath").asString(),
		provideConfigValue(cfg, "openTelemetry.logs.protocol").asString(),
		provideConfigValue(cfg, "openTelemetry.logs.auth.token").asString(),
		provideConfigValue(cfg, "openTelemetry.logs.auth.tokenType").asString(),

		// auth config
		provideConfigValue(cfg, "auth.jwtSigningKey").asString(),
		provideConfigValue(cfg, "auth.accessTokenTTL").asDuration(),
		provideConfigValue(cfg, "auth.refreshTokenTTL").asDuration(),

		// workspacefs exec config
		provideConfigValue(cfg, "workspacefs.platformAgentsPath").asString(),
		provideConfigValue(cfg, "workspacefs.exec.enabled").asBool(),
		provideConfigValue(cfg, "workspacefs.exec.maxOutputBytes").asInt64(),
		provideConfigValue(cfg, "workspacefs.exec.defaultTimeout").asDuration(),
		provideConfigValue(cfg, "workspacefs.exec.maxConcurrentJobs").asInt(),

		// agent runtime persistence config
		provideConfigValue(cfg, "agentRuntime.storage.type").asString(),
		provideConfigValue(cfg, "agentRuntime.database.dsn").asString(),
		provideConfigValue(cfg, "agentRuntime.database.tablePrefix").asString(),

		// data layer persistence config
		provideConfigValue(cfg, "dataLayer.database.dsn").asString(),
		provideConfigValue(cfg, "dataLayer.database.tablePrefix").asString(),

		// jobs config
		provideConfigValue(cfg, "jobs.scheduler.loopInterval").asDuration(),
		provideConfigValue(cfg, "jobs.worker.enabled").asBool(),
		provideConfigValue(cfg, "jobs.worker.pollInterval").asDuration(),
		provideConfigValue(cfg, "jobs.worker.maxAttempts").asInt(),
		provideConfigValue(cfg, "jobs.worker.maxConcurrentHistoricalBackfills").asInt(),
		provideConfigValue(cfg, "jobs.historicalBackfill.maxIntervals").asInt(),
		provideConfigValue(cfg, "jobs.historicalBackfill.maxPageSize").asInt(),

		// finance providers config
		provideConfigValue(cfg, "finance.fixtures.database.dsn").asString(),
		provideConfigValue(cfg, "finance.fixtures.database.jobsTablePrefix").asString(),
		provideConfigValue(cfg, "finance.providers.monobank.baseURL").asString(),
		provideConfigValue(cfg, "finance.providers.monobank.sleepBetweenRequests").asDuration(),
		provideConfigValue(cfg, "finance.providers.enableBanking.baseURL").asString(),
		provideConfigValue(cfg, "finance.providers.enableBanking.callbackBaseURL").asString(),
		provideConfigValue(cfg, "finance.providers.enableBanking.appID").asString(),
		provideConfigValue(cfg, "finance.providers.enableBanking.privateKeyPath").asString(),
		provideConfigValue(cfg, "finance.providers.enableBanking.aspspName").asString(),
		provideConfigValue(cfg, "finance.providers.enableBanking.country").asString(),
		provideConfigValue(cfg, "finance.providers.enableBanking.psuType").asString(),
		provideConfigValue(cfg, "finance.providers.enableBanking.validDays").asInt(),

		// skills config
		provideConfigValue(cfg, "skills.enabled").asBool(),
		provideConfigValue(cfg, "skills.paths").asStringSlice(),
		provideConfigValue(cfg, "skills.maxSkillBytes").asInt(),
		provideConfigValue(cfg, "skills.maxCatalogEntries").asInt(),
	)
}
