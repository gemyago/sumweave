package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValues(t *testing.T) {
	t.Run("exact-decodes every committed environment", func(t *testing.T) {
		t.Run("local", func(t *testing.T) {
			values, err := LoadValues(ValuesLoadInput{Environment: "local"})
			require.NoError(t, err)
			require.Equal(t, "DEBUG", values.DefaultLogLevel)
			require.Equal(t, "data/application.db", values.Application.Database.DSN)
			require.Equal(t, "local-secret-key", values.Auth.JWTSigningKey)
			require.Equal(t, time.Minute, values.HTTPServer.IdleTimeout)
			require.Equal(t, 5*time.Minute, values.Jobs.Worker.StaleRunningAge)
			require.Equal(t, []string{"../../.platform-agents/skills"}, values.Skills.Paths)
		})

		t.Run("test", func(t *testing.T) {
			values, err := LoadValues(ValuesLoadInput{Environment: "test"})
			require.NoError(t, err)
			require.Equal(t, ":memory:", values.Application.Database.DSN)
			require.Equal(t, "test-secret-key", values.Auth.JWTSigningKey)
			require.Equal(t, "https://enable-banking.test", values.Finance.Providers.EnableBanking.BaseURL)
			require.Equal(t, 90, values.Finance.Providers.EnableBanking.ValidDays)
		})

		t.Run("production preserves intentional empty values", func(t *testing.T) {
			values, err := LoadValues(ValuesLoadInput{Environment: "production"})
			require.NoError(t, err)
			require.Empty(t, values.Auth.JWTSigningKey)
			require.Empty(t, values.Application.Database.DSN)
			require.Empty(t, values.AgentRuntime.Database.DSN)
			require.Empty(t, values.HTTPServer.TLS.CertFile)
			require.Empty(t, values.HTTPServer.TLS.KeyFile)
		})
	})

	t.Run("uses APP automatic environment values for declared base keys", func(t *testing.T) {
		t.Setenv("APP_APPLICATION_DATABASE_DSN", "postgres://app:secret@db.example/sumweave")
		t.Setenv("APP_JOBS_WORKER_MAXATTEMPTS", "7")
		t.Setenv("APP_HTTPSERVER_WRITETIMEOUT", "45s")
		t.Setenv("APP_HTTPSERVER_TLS_CERTFILE", "certs/app.pem")
		t.Setenv("APP_HTTPSERVER_TLS_KEYFILE", "certs/app-key.pem")
		t.Setenv("APP_SKILLS_PATHS", "skills/one,skills/two")

		values, err := LoadValues(ValuesLoadInput{Environment: "test"})
		require.NoError(t, err)
		require.Equal(t, "postgres://app:secret@db.example/sumweave", values.Application.Database.DSN)
		require.Equal(t, 7, values.Jobs.Worker.MaxAttempts)
		require.Equal(t, 45*time.Second, values.HTTPServer.WriteTimeout)
		require.Equal(t, "certs/app.pem", values.HTTPServer.TLS.CertFile)
		require.Equal(t, "certs/app-key.pem", values.HTTPServer.TLS.KeyFile)
		require.Equal(t, []string{"skills/one", "skills/two"}, values.Skills.Paths)
	})

	t.Run("applies typed CLI overrides after exact decoding", func(t *testing.T) {
		logLevel := "WARN"
		jsonLogs := false
		logsFile := ""
		values, err := LoadValues(ValuesLoadInput{
			Environment: "test",
			CLI: CLIOverrides{
				DefaultLogLevel: &logLevel,
				JSONLogs:        &jsonLogs,
				LogsFile:        &logsFile,
			},
		})
		require.NoError(t, err)
		require.Equal(t, "WARN", values.DefaultLogLevel)
		require.NotNil(t, values.JSONLogs)
		require.False(t, *values.JSONLogs)
		require.NotNil(t, values.LogsFile)
		require.Empty(t, *values.LogsFile)

		values, err = LoadValues(ValuesLoadInput{Environment: "test"})
		require.NoError(t, err)
		require.Nil(t, values.JSONLogs)
		require.Nil(t, values.LogsFile)
	})

	t.Run("rejects unknown merged keys", func(t *testing.T) {
		cfg := New()
		require.NoError(t, load(cfg, NewLoadOpts().WithEnv("test")))
		require.NoError(t, cfg.MergeConfig(strings.NewReader("finance:\n  unexpectedProvider: true\n")))

		_, err := decodeValues(cfg)
		require.Error(t, err)
		require.ErrorContains(t, err, "unexpectedprovider")
	})

	t.Run("rejects malformed typed APP values", func(t *testing.T) {
		for _, testCase := range []struct {
			name  string
			key   string
			value string
		}{
			{name: "int", key: "APP_JOBS_WORKER_MAXATTEMPTS", value: "not-an-int"},
			{name: "duration", key: "APP_HTTPSERVER_WRITETIMEOUT", value: "not-a-duration"},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				t.Setenv(testCase.key, testCase.value)
				_, err := LoadValues(ValuesLoadInput{Environment: "test"})
				require.Error(t, err)
			})
		}
	})

	t.Run("projects only migration-required values", func(t *testing.T) {
		values := Values{
			Application:  Application{Database: Database{DSN: "application.sqlite"}},
			AgentRuntime: AgentRuntime{Storage: AgentRuntimeStorage{Type: "file"}},
		}
		root, err := values.MigrationRoot("test")
		require.NoError(t, err)
		require.Equal(t, "test", root.Environment)
		require.Empty(t, values.Auth.JWTSigningKey)
		require.Empty(t, values.Finance.Providers.EnableBanking.AppID)

		values.Application.Database.DSN = ""
		_, err = values.MigrationRoot("test")
		require.ErrorContains(t, err, "application database dsn")

		values.Application.Database.DSN = "application.sqlite"
		values.AgentRuntime.Storage.Type = "database"
		_, err = values.MigrationRoot("test")
		require.ErrorContains(t, err, "agent runtime database dsn")
	})

	t.Run("projects jobs settings and rejects incomplete finance startup settings", func(t *testing.T) {
		values := Values{
			Application: Application{Database: Database{DSN: "application.sqlite"}},
			Auth:        Auth{JWTSigningKey: "test-signing-key"},
			Finance: Finance{Providers: FinanceProviders{
				Monobank: MonobankProvider{BaseURL: "https://monobank.test", RetryAfterFallbackDelay: time.Second},
				EnableBanking: EnableBankingProvider{
					BaseURL: "https://enable-banking.test", AppID: "test-app", PrivateKeyPath: "test-key.pem",
					ASPSPName: "Test ASPSP", Country: "PL", PSUType: "personal", ValidDays: 90,
				},
			}},
		}
		root, err := values.WorkerRoot("test")
		require.NoError(t, err)
		require.Equal(t, "test", root.Environment)
		require.Equal(t, values.Jobs, root.Jobs)

		values.Auth.JWTSigningKey = ""
		_, err = values.WorkerRoot("test")
		require.ErrorContains(t, err, "JWT signing key")
		values.Auth.JWTSigningKey = "test-signing-key"
		values.Finance.Providers.EnableBanking.AppID = ""
		_, err = values.WorkerRoot("test")
		require.ErrorContains(t, err, "enable banking app ID")
		values.Finance.Providers.EnableBanking.AppID = "test-app"
		values.Finance.Providers.EnableBanking.ValidDays = 0
		_, err = values.WorkerRoot("test")
		require.ErrorContains(t, err, "valid days")
	})

	t.Run("projects scheduler finance settings and rejects incomplete values", func(t *testing.T) {
		values, err := LoadValues(ValuesLoadInput{Environment: "test"})
		require.NoError(t, err)
		root, err := values.SchedulerRoot("test")
		require.NoError(t, err)
		require.Equal(t, values.Finance, root.Finance)
		require.Equal(t, values.Auth, root.Auth)

		for _, mutate := range []func(*Values){
			func(value *Values) { value.Auth.JWTSigningKey = "" },
			func(value *Values) { value.Finance.Providers.Monobank.BaseURL = "" },
			func(value *Values) { value.Finance.Providers.Monobank.RetryAfterFallbackDelay = 0 },
			func(value *Values) { value.Finance.Providers.EnableBanking.ASPSPName = "" },
			func(value *Values) { value.Finance.Providers.EnableBanking.ValidDays = 0 },
		} {
			candidate, loadErr := LoadValues(ValuesLoadInput{Environment: "test"})
			require.NoError(t, loadErr)
			mutate(&candidate)
			_, rootErr := candidate.SchedulerRoot("test")
			require.Error(t, rootErr)
		}
	})

	t.Run("projects HTTP settings and validates API-only requirements", func(t *testing.T) {
		values, err := LoadValues(ValuesLoadInput{Environment: "test"})
		require.NoError(t, err)
		root, err := values.HTTPRoot("test")
		require.NoError(t, err)
		require.Equal(t, values.HTTPServer, root.HTTPServer)
		require.Equal(t, values.AgentRuntime, root.AgentRuntime)

		values.Auth.JWTSigningKey = ""
		_, err = values.HTTPRoot("test")
		require.ErrorContains(t, err, "JWT signing key")
		values.Auth.JWTSigningKey = "test-secret-key"
		values.AgentRuntime.Storage.Type = "database"
		values.AgentRuntime.Database.DSN = ""
		_, err = values.HTTPRoot("test")
		require.ErrorContains(t, err, "agent runtime database dsn")
		values.AgentRuntime.Storage.Type = "file"
		values.HTTPServer.TLS.CertFile = "certificate.pem"
		_, err = values.HTTPRoot("test")
		require.ErrorContains(t, err, "TLS certificate")
		values.HTTPServer.TLS.CertFile = ""
		for _, mutate := range []func(*Values){
			func(value *Values) { value.Finance.Providers.Monobank.BaseURL = "" },
			func(value *Values) { value.Finance.Providers.Monobank.RetryAfterFallbackDelay = 0 },
			func(value *Values) { value.Finance.Providers.EnableBanking.Country = "" },
			func(value *Values) { value.Finance.Providers.EnableBanking.ValidDays = 0 },
		} {
			candidate, loadErr := LoadValues(ValuesLoadInput{Environment: "test"})
			require.NoError(t, loadErr)
			mutate(&candidate)
			_, rootErr := candidate.HTTPRoot("test")
			require.Error(t, rootErr)
		}
	})

	t.Run("projects only user and finance fixture settings", func(t *testing.T) {
		values := Values{Application: Application{Database: Database{DSN: "application.sqlite"}}}
		usersRoot, err := values.UsersRoot("test")
		require.NoError(t, err)
		require.Equal(t, "test", usersRoot.Environment)
		require.Equal(t, values.Application, usersRoot.Application)

		fixturesRoot, err := values.FinanceFixturesRoot("test")
		require.NoError(t, err)
		require.Equal(t, "test", fixturesRoot.Environment)
		require.Empty(t, fixturesRoot.Auth.JWTSigningKey)
		require.Empty(t, fixturesRoot.Finance.Providers.Monobank.BaseURL)

		values.Application.Database.DSN = ""
		_, err = values.UsersRoot("test")
		require.ErrorContains(t, err, "users application database dsn")
		_, err = values.FinanceFixturesRoot("test")
		require.ErrorContains(t, err, "finance fixtures application database dsn")
	})
}
