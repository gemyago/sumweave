package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const databaseStorageType = "database"

// Values is the typed snapshot of the complete supported base-YAML contract.
type Values struct {
	DefaultLogLevel         string        `mapstructure:"defaultLogLevel"`
	DataDir                 string        `mapstructure:"dataDir"`
	PProfListener           PProfListener `mapstructure:"pprofListener"`
	GracefulShutdownTimeout time.Duration `mapstructure:"gracefulShutdownTimeout"`
	HTTPServer              HTTPServer    `mapstructure:"httpServer"`
	HTTPClient              HTTPClient    `mapstructure:"httpClient"`
	OpenTelemetry           OpenTelemetry `mapstructure:"openTelemetry"`
	Auth                    Auth          `mapstructure:"auth"`
	WorkspaceFS             WorkspaceFS   `mapstructure:"workspacefs"`
	AgentRuntime            AgentRuntime  `mapstructure:"agentRuntime"`
	Application             Application   `mapstructure:"application"`
	Jobs                    Jobs          `mapstructure:"jobs"`
	Finance                 Finance       `mapstructure:"finance"`
	Skills                  Skills        `mapstructure:"skills"`
	JSONLogs                *bool         `mapstructure:"-"`
	LogsFile                *string       `mapstructure:"-"`
}

type PProfListener struct {
	Enabled bool   `mapstructure:"enabled"`
	Addr    string `mapstructure:"addr"`
}

type HTTPServer struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	TLS               TLS           `mapstructure:"tls"`
	IdleTimeout       time.Duration `mapstructure:"idleTimeout"`
	ReadHeaderTimeout time.Duration `mapstructure:"readHeaderTimeout"`
	ReadTimeout       time.Duration `mapstructure:"readTimeout"`
	WriteTimeout      time.Duration `mapstructure:"writeTimeout"`
	AccessLogsLevel   string        `mapstructure:"accessLogsLevel"`
}

type TLS struct {
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
}

type HTTPClient struct {
	RetryAfterFallbackDelay time.Duration `mapstructure:"retryAfterFallbackDelay"`
}

type OpenTelemetry struct {
	Enabled        bool              `mapstructure:"enabled"`
	RuntimeMetrics bool              `mapstructure:"runtimeMetrics"`
	Traces         OpenTelemetrySink `mapstructure:"traces"`
	Metrics        MetricsSink       `mapstructure:"metrics"`
	Logs           LogsSink          `mapstructure:"logs"`
}

type OpenTelemetrySink struct {
	Enabled      bool              `mapstructure:"enabled"`
	Endpoint     string            `mapstructure:"endpoint"`
	URLPath      string            `mapstructure:"urlPath"`
	Protocol     string            `mapstructure:"protocol"`
	SamplingRate float64           `mapstructure:"samplingRate"`
	Auth         OpenTelemetryAuth `mapstructure:"auth"`
}

type MetricsSink struct {
	Enabled        bool              `mapstructure:"enabled"`
	Endpoint       string            `mapstructure:"endpoint"`
	URLPath        string            `mapstructure:"urlPath"`
	Protocol       string            `mapstructure:"protocol"`
	ExportInterval time.Duration     `mapstructure:"exportInterval"`
	Auth           OpenTelemetryAuth `mapstructure:"auth"`
}

type LogsSink struct {
	Enabled              bool              `mapstructure:"enabled"`
	DefaultHandlerFanout bool              `mapstructure:"defaultHandlerFanout"`
	Endpoint             string            `mapstructure:"endpoint"`
	URLPath              string            `mapstructure:"urlPath"`
	Protocol             string            `mapstructure:"protocol"`
	Auth                 OpenTelemetryAuth `mapstructure:"auth"`
}

type OpenTelemetryAuth struct {
	Token     string `mapstructure:"token"`
	TokenType string `mapstructure:"tokenType"`
}

type Auth struct {
	JWTSigningKey   string        `mapstructure:"jwtSigningKey"`
	AccessTokenTTL  time.Duration `mapstructure:"accessTokenTTL"`
	RefreshTokenTTL time.Duration `mapstructure:"refreshTokenTTL"`
}

type WorkspaceFS struct {
	PlatformAgentsPath string          `mapstructure:"platformAgentsPath"`
	Exec               WorkspaceFSExec `mapstructure:"exec"`
}

type WorkspaceFSExec struct {
	Enabled           bool          `mapstructure:"enabled"`
	MaxOutputBytes    int64         `mapstructure:"maxOutputBytes"`
	DefaultTimeout    time.Duration `mapstructure:"defaultTimeout"`
	MaxConcurrentJobs int           `mapstructure:"maxConcurrentJobs"`
}

type AgentRuntime struct {
	Storage  AgentRuntimeStorage `mapstructure:"storage"`
	Database Database            `mapstructure:"database"`
}

type AgentRuntimeStorage struct {
	Type string `mapstructure:"type"`
}

type Application struct {
	Database Database `mapstructure:"database"`
}

type Database struct {
	DSN         string `mapstructure:"dsn"`
	TablePrefix string `mapstructure:"tablePrefix"`
}

type Jobs struct {
	Scheduler JobsScheduler `mapstructure:"scheduler"`
	Worker    JobsWorker    `mapstructure:"worker"`
}

type JobsScheduler struct {
	LoopInterval time.Duration `mapstructure:"loopInterval"`
}

type JobsWorker struct {
	PollInterval    time.Duration `mapstructure:"pollInterval"`
	MaxAttempts     int           `mapstructure:"maxAttempts"`
	StaleRunningAge time.Duration `mapstructure:"staleRunningAge"`
}

type Finance struct {
	Providers FinanceProviders `mapstructure:"providers"`
}

type FinanceProviders struct {
	Monobank      MonobankProvider      `mapstructure:"monobank"`
	Frankfurter   FrankfurterProvider   `mapstructure:"frankfurter"`
	EnableBanking EnableBankingProvider `mapstructure:"enableBanking"`
}

type MonobankProvider struct {
	BaseURL                 string        `mapstructure:"baseURL"`
	RetryAfterFallbackDelay time.Duration `mapstructure:"retryAfterFallbackDelay"`
}

type FrankfurterProvider struct {
	BaseURL string `mapstructure:"baseURL"`
}

type EnableBankingProvider struct {
	BaseURL         string `mapstructure:"baseURL"`
	CallbackBaseURL string `mapstructure:"callbackBaseURL"`
	AppID           string `mapstructure:"appID"`
	PrivateKeyPath  string `mapstructure:"privateKeyPath"`
	ASPSPName       string `mapstructure:"aspspName"`
	Country         string `mapstructure:"country"`
	PSUType         string `mapstructure:"psuType"`
	ValidDays       int    `mapstructure:"validDays"`
}

type Skills struct {
	Enabled           bool     `mapstructure:"enabled"`
	Paths             []string `mapstructure:"paths"`
	MaxSkillBytes     int      `mapstructure:"maxSkillBytes"`
	MaxCatalogEntries int      `mapstructure:"maxCatalogEntries"`
}

// CLIOverrides are typed command inputs applied after YAML and APP_ loading.
// Pointer fields preserve an omitted option separately from an explicit zero
// value, notably --json-logs=false.
type CLIOverrides struct {
	DefaultLogLevel *string
	JSONLogs        *bool
	LogsFile        *string
}

// ValuesLoadInput identifies the embedded environment layer and command inputs
// needed by future wireup roots without exposing Viper to those roots.
type ValuesLoadInput struct {
	Environment string
	CLI         CLIOverrides
}

// MigrationRootConfig contains only the configuration required to run schema
// migrations. It intentionally excludes authentication credentials and finance
// provider settings because the migration root does not construct those services.
type MigrationRootConfig struct {
	Environment             string
	DefaultLogLevel         string
	JSONLogs                *bool
	LogsFile                *string
	PProfListener           PProfListener
	GracefulShutdownTimeout time.Duration
	OpenTelemetry           OpenTelemetry
	AgentRuntime            AgentRuntime
	Application             Application
}

// WorkerRootConfig contains only the configuration required by the durable
// worker. It intentionally excludes HTTP, runtime, and scheduler settings.
type WorkerRootConfig struct {
	Environment             string
	DefaultLogLevel         string
	JSONLogs                *bool
	LogsFile                *string
	PProfListener           PProfListener
	GracefulShutdownTimeout time.Duration
	HTTPClient              HTTPClient
	OpenTelemetry           OpenTelemetry
	Auth                    Auth
	Application             Application
	Jobs                    Jobs
	Finance                 Finance
}

// SchedulerRootConfig contains only the configuration required by the
// one-shot scheduler. It intentionally excludes HTTP serving and worker settings.
type SchedulerRootConfig struct {
	Environment             string
	DefaultLogLevel         string
	JSONLogs                *bool
	LogsFile                *string
	PProfListener           PProfListener
	GracefulShutdownTimeout time.Duration
	HTTPClient              HTTPClient
	OpenTelemetry           OpenTelemetry
	Auth                    Auth
	Application             Application
	Finance                 Finance
	Scheduler               JobsScheduler
}

// HTTPRootConfig contains the configuration required to build the API-only
// HTTP application. It excludes worker and scheduler settings because API
// construction does not own either process capability.
type HTTPRootConfig struct {
	Environment             string
	DefaultLogLevel         string
	JSONLogs                *bool
	LogsFile                *string
	DataDir                 string
	PProfListener           PProfListener
	GracefulShutdownTimeout time.Duration
	HTTPServer              HTTPServer
	HTTPClient              HTTPClient
	OpenTelemetry           OpenTelemetry
	Auth                    Auth
	WorkspaceFS             WorkspaceFS
	AgentRuntime            AgentRuntime
	Application             Application
	Finance                 Finance
	Skills                  Skills
}

// UsersRootConfig contains only the application database settings needed by
// user administration commands.
type UsersRootConfig struct {
	Environment string
	Application Application
}

// FinanceFixturesRootConfig contains only the persistent settings used by the
// finance fixture generator. Its JWT and Monobank values remain optional
// because fixture generation has safe local fallbacks for both.
type FinanceFixturesRootConfig struct {
	Environment string
	Application Application
	Auth        Auth
	Finance     Finance
}

// MigrationRoot projects and validates the configuration required by db-migrate.
func (values *Values) MigrationRoot(environment string) (MigrationRootConfig, error) {
	root := MigrationRootConfig{
		Environment:             environment,
		DefaultLogLevel:         values.DefaultLogLevel,
		JSONLogs:                values.JSONLogs,
		LogsFile:                values.LogsFile,
		PProfListener:           values.PProfListener,
		GracefulShutdownTimeout: values.GracefulShutdownTimeout,
		OpenTelemetry:           values.OpenTelemetry,
		AgentRuntime:            values.AgentRuntime,
		Application:             values.Application,
	}
	if strings.TrimSpace(root.Application.Database.DSN) == "" {
		return MigrationRootConfig{}, errors.New("migration application database dsn is required")
	}
	if root.AgentRuntime.Storage.Type == databaseStorageType &&
		strings.TrimSpace(root.AgentRuntime.Database.DSN) == "" {
		return MigrationRootConfig{}, errors.New(
			"migration agent runtime database dsn is required for database storage",
		)
	}
	return root, nil
}

// WorkerRoot projects and validates the configuration required to compose the
// durable worker and finance command handlers.
func (values *Values) WorkerRoot(environment string) (WorkerRootConfig, error) {
	root := WorkerRootConfig{
		Environment:             environment,
		DefaultLogLevel:         values.DefaultLogLevel,
		JSONLogs:                values.JSONLogs,
		LogsFile:                values.LogsFile,
		PProfListener:           values.PProfListener,
		GracefulShutdownTimeout: values.GracefulShutdownTimeout,
		HTTPClient:              values.HTTPClient,
		OpenTelemetry:           values.OpenTelemetry,
		Auth:                    values.Auth,
		Application:             values.Application,
		Jobs:                    values.Jobs,
		Finance:                 values.Finance,
	}
	if strings.TrimSpace(root.Application.Database.DSN) == "" {
		return WorkerRootConfig{}, errors.New("worker application database dsn is required")
	}
	if strings.TrimSpace(root.Auth.JWTSigningKey) == "" {
		return WorkerRootConfig{}, errors.New("worker auth JWT signing key is required")
	}
	if strings.TrimSpace(root.Finance.Providers.Monobank.BaseURL) == "" {
		return WorkerRootConfig{}, errors.New("worker monobank base URL is required")
	}
	if root.Finance.Providers.Monobank.RetryAfterFallbackDelay <= 0 {
		return WorkerRootConfig{}, errors.New("worker monobank Retry-After fallback delay must be positive")
	}
	enableBanking := root.Finance.Providers.EnableBanking
	for _, required := range []struct {
		name  string
		value string
	}{
		{"base URL", enableBanking.BaseURL},
		{"app ID", enableBanking.AppID},
		{"private key path", enableBanking.PrivateKeyPath},
		{"ASPSP name", enableBanking.ASPSPName},
		{"country", enableBanking.Country},
		{"PSU type", enableBanking.PSUType},
	} {
		if strings.TrimSpace(required.value) == "" {
			return WorkerRootConfig{}, fmt.Errorf("worker enable banking %s is required", required.name)
		}
	}
	if enableBanking.ValidDays <= 0 {
		return WorkerRootConfig{}, errors.New("worker enable banking valid days must be positive")
	}
	return root, nil
}

// SchedulerRoot projects and validates the configuration required to compose
// the one-shot schedule publisher.
func (values *Values) SchedulerRoot(environment string) (SchedulerRootConfig, error) {
	root := SchedulerRootConfig{
		Environment:             environment,
		DefaultLogLevel:         values.DefaultLogLevel,
		JSONLogs:                values.JSONLogs,
		LogsFile:                values.LogsFile,
		PProfListener:           values.PProfListener,
		GracefulShutdownTimeout: values.GracefulShutdownTimeout,
		HTTPClient:              values.HTTPClient,
		OpenTelemetry:           values.OpenTelemetry,
		Auth:                    values.Auth,
		Application:             values.Application,
		Finance:                 values.Finance,
		Scheduler:               values.Jobs.Scheduler,
	}
	if strings.TrimSpace(root.Application.Database.DSN) == "" {
		return SchedulerRootConfig{}, errors.New("scheduler application database dsn is required")
	}
	if strings.TrimSpace(root.Auth.JWTSigningKey) == "" {
		return SchedulerRootConfig{}, errors.New("scheduler auth JWT signing key is required")
	}
	if strings.TrimSpace(root.Finance.Providers.Monobank.BaseURL) == "" {
		return SchedulerRootConfig{}, errors.New("scheduler monobank base URL is required")
	}
	if root.Finance.Providers.Monobank.RetryAfterFallbackDelay <= 0 {
		return SchedulerRootConfig{}, errors.New("scheduler monobank Retry-After fallback delay must be positive")
	}
	enableBanking := root.Finance.Providers.EnableBanking
	for _, required := range []struct {
		name  string
		value string
	}{
		{"base URL", enableBanking.BaseURL},
		{"app ID", enableBanking.AppID},
		{"private key path", enableBanking.PrivateKeyPath},
		{"ASPSP name", enableBanking.ASPSPName},
		{"country", enableBanking.Country},
		{"PSU type", enableBanking.PSUType},
	} {
		if strings.TrimSpace(required.value) == "" {
			return SchedulerRootConfig{}, fmt.Errorf("scheduler enable banking %s is required", required.name)
		}
	}
	if enableBanking.ValidDays <= 0 {
		return SchedulerRootConfig{}, errors.New("scheduler enable banking valid days must be positive")
	}
	return root, nil
}

// HTTPRoot projects and validates the API-only HTTP application settings.
func (values *Values) HTTPRoot(environment string) (HTTPRootConfig, error) {
	root := HTTPRootConfig{
		Environment: environment, DefaultLogLevel: values.DefaultLogLevel, JSONLogs: values.JSONLogs,
		LogsFile: values.LogsFile, DataDir: values.DataDir, PProfListener: values.PProfListener,
		GracefulShutdownTimeout: values.GracefulShutdownTimeout, HTTPServer: values.HTTPServer,
		HTTPClient: values.HTTPClient, OpenTelemetry: values.OpenTelemetry, Auth: values.Auth,
		WorkspaceFS: values.WorkspaceFS, AgentRuntime: values.AgentRuntime,
		Application: values.Application, Finance: values.Finance, Skills: values.Skills,
	}
	if strings.TrimSpace(root.Application.Database.DSN) == "" {
		return HTTPRootConfig{}, errors.New("HTTP application database dsn is required")
	}
	if strings.TrimSpace(root.Auth.JWTSigningKey) == "" {
		return HTTPRootConfig{}, errors.New("HTTP auth JWT signing key is required")
	}
	if root.AgentRuntime.Storage.Type == databaseStorageType &&
		strings.TrimSpace(root.AgentRuntime.Database.DSN) == "" {
		return HTTPRootConfig{}, errors.New("HTTP agent runtime database dsn is required for database storage")
	}
	if (root.HTTPServer.TLS.CertFile == "") != (root.HTTPServer.TLS.KeyFile == "") {
		return HTTPRootConfig{}, errors.New("HTTP TLS certificate and key files must be configured together")
	}
	if err := validateFinanceRoot(root.Finance); err != nil {
		return HTTPRootConfig{}, fmt.Errorf("HTTP %w", err)
	}
	return root, nil
}

// UsersRoot projects and validates the configuration for user administration.
func (values *Values) UsersRoot(environment string) (UsersRootConfig, error) {
	root := UsersRootConfig{Environment: environment, Application: values.Application}
	if strings.TrimSpace(root.Application.Database.DSN) == "" {
		return UsersRootConfig{}, errors.New("users application database dsn is required")
	}
	return root, nil
}

// FinanceFixturesRoot projects and validates the configuration for fixture
// generation without requiring live-provider or HTTP application settings.
func (values *Values) FinanceFixturesRoot(environment string) (FinanceFixturesRootConfig, error) {
	root := FinanceFixturesRootConfig{
		Environment: environment,
		Application: values.Application,
		Auth:        values.Auth,
		Finance:     values.Finance,
	}
	if strings.TrimSpace(root.Application.Database.DSN) == "" {
		return FinanceFixturesRootConfig{}, errors.New("finance fixtures application database dsn is required")
	}
	return root, nil
}

func validateFinanceRoot(finance Finance) error {
	if strings.TrimSpace(finance.Providers.Monobank.BaseURL) == "" {
		return errors.New("monobank base URL is required")
	}
	if finance.Providers.Monobank.RetryAfterFallbackDelay <= 0 {
		return errors.New("monobank Retry-After fallback delay must be positive")
	}
	enableBanking := finance.Providers.EnableBanking
	for _, required := range []struct{ name, value string }{
		{"enable banking base URL", enableBanking.BaseURL},
		{"enable banking app ID", enableBanking.AppID},
		{"enable banking private key path", enableBanking.PrivateKeyPath},
		{"enable banking ASPSP name", enableBanking.ASPSPName},
		{"enable banking country", enableBanking.Country},
		{"enable banking PSU type", enableBanking.PSUType},
	} {
		if strings.TrimSpace(required.value) == "" {
			return fmt.Errorf("%s is required", required.name)
		}
	}
	if enableBanking.ValidDays <= 0 {
		return errors.New("enable banking valid days must be positive")
	}
	return nil
}

// decodeValues exact-decodes already layered application configuration.
func decodeValues(cfg *viper.Viper) (Values, error) {
	var values Values
	if err := cfg.UnmarshalExact(&values); err != nil {
		return Values{}, fmt.Errorf("decode application configuration: %w", err)
	}
	return values, nil
}

// LoadValues loads the existing YAML and APP_ layers, exact-decodes them, then
// applies typed command overrides without Viper aliases or synthetic keys.
func LoadValues(input ValuesLoadInput) (Values, error) {
	cfg := New()
	if err := load(cfg, NewLoadOpts().WithEnv(input.Environment)); err != nil {
		return Values{}, err
	}

	values, err := decodeValues(cfg)
	if err != nil {
		return Values{}, err
	}
	values.ApplyCLIOverrides(input.CLI)
	return values, nil
}

// ApplyCLIOverrides applies command input only after the external configuration
// contract has exact-decoded.
func (values *Values) ApplyCLIOverrides(overrides CLIOverrides) {
	if overrides.DefaultLogLevel != nil {
		values.DefaultLogLevel = *overrides.DefaultLogLevel
	}
	values.JSONLogs = overrides.JSONLogs
	values.LogsFile = overrides.LogsFile
}
