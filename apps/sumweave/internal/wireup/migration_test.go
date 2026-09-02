package wireup

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/runtime/agent"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBuildMigration(t *testing.T) {
	fake := faker.New()

	t.Run("runs the injected migration orchestration before shutting down", func(t *testing.T) {
		migrator := newMockmigrationRunner(t)
		migrator.EXPECT().Migrate(t.Context()).Return(nil).Once()
		root := &MigrationRoot{migrator: migrator, shutdownHooks: lifecycle.NewTestShutdownHooks()}
		shutdownErr := errors.New(fake.Lorem().Sentence(3))
		root.shutdownHooks.Register("test", func(_ context.Context) error { return shutdownErr })
		require.ErrorIs(t, root.Migrate(t.Context()), shutdownErr)
	})

	t.Run("returns migration failures after running shutdown", func(t *testing.T) {
		expectedErr := errors.New(fake.Lorem().Sentence(3))
		migrator := newMockmigrationRunner(t)
		migrator.EXPECT().Migrate(t.Context()).Return(expectedErr).Once()
		root := &MigrationRoot{migrator: migrator, shutdownHooks: lifecycle.NewTestShutdownHooks()}
		require.ErrorIs(t, root.Migrate(t.Context()), expectedErr)
	})

	t.Run("reports typed configuration load and validation failures", func(t *testing.T) {
		_, err := BuildMigration(t.Context(), MigrationOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)

		t.Setenv("APP_APPLICATION_DATABASE_DSN", "")
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", "")
		_, err = BuildMigration(t.Context(), MigrationOptions{Environment: "production"})
		require.ErrorContains(t, err, "application database dsn")
	})

	t.Run("releases constructed resources when an unavailable database prevents migration", func(t *testing.T) {
		_, err := buildMigration(t.Context(), config.MigrationRootConfig{
			Environment:     fake.UUID().V4(),
			DefaultLogLevel: "info",
			Application: config.Application{Database: config.Database{
				DSN: "postgres://localhost:invalid/" + fake.UUID().V4(), TablePrefix: fake.Letter(),
			}},
		})
		require.ErrorContains(t, err, "create migration auth user store")
	})

	t.Run("applies optional log configuration before database work", func(t *testing.T) {
		jsonLogs := true
		logsFile := t.TempDir() + "/" + fake.UUID().V4()
		_, err := buildMigration(t.Context(), config.MigrationRootConfig{
			Environment: fake.UUID().V4(), DefaultLogLevel: "info", JSONLogs: &jsonLogs, LogsFile: &logsFile,
		})
		require.ErrorContains(t, err, "open application sql database")
	})

	t.Run("rejects unsupported telemetry exporters before database work", func(t *testing.T) {
		for _, openTelemetry := range []config.OpenTelemetry{
			{Enabled: true, Logs: config.LogsSink{Enabled: true, Protocol: "grpc"}},
			{Enabled: true, Traces: config.OpenTelemetrySink{Enabled: true, Protocol: "grpc"}},
			{Enabled: true, Metrics: config.MetricsSink{Enabled: true, Protocol: "grpc"}},
		} {
			_, err := buildMigration(t.Context(), config.MigrationRootConfig{
				Environment: fake.UUID().V4(), DefaultLogLevel: "info", OpenTelemetry: openTelemetry,
			})
			require.Error(t, err)
		}
	})

	t.Run("uses the local default and propagates configuration loader errors", func(t *testing.T) {
		loader := newMockmigrationConfigLoader(t)
		expectedErr := errors.New(fake.UUID().V4())
		loader.EXPECT().
			Load(MigrationOptions{}, localEnvironment).
			Return(config.MigrationRootConfig{}, expectedErr).
			Once()
		_, err := buildMigrationWithConfigLoader(t.Context(), MigrationOptions{}, loader)
		require.ErrorIs(t, err, expectedErr)

		loader = newMockmigrationConfigLoader(t)
		loader.EXPECT().Load(MigrationOptions{Environment: "test"}, "test").Return(
			config.MigrationRootConfig{DefaultLogLevel: fake.UUID().V4()}, nil,
		).Once()
		_, err = buildMigrationWithConfigLoader(t.Context(), MigrationOptions{Environment: "test"}, loader)
		require.ErrorContains(t, err, "parse migration log level")
		_, err = migrationConfigLoaderFunc(func(MigrationOptions, string) (config.MigrationRootConfig, error) {
			return config.MigrationRootConfig{}, expectedErr
		}).Load(MigrationOptions{}, localEnvironment)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("migrates prepared agent runtime components in order without DDL", func(t *testing.T) {
		sessions := newMockagentRuntimeSchemaMigrator(t)
		profiles := newMockagentRuntimeSchemaMigrator(t)
		providers := newMockagentRuntimeSchemaMigrator(t)
		mock.InOrder(
			sessions.EXPECT().AutoMigrate().Return(nil).Once(),
			profiles.EXPECT().AutoMigrate().Return(nil).Once(),
			providers.EXPECT().AutoMigrate().Return(nil).Once(),
		)
		require.NoError(t, agentRuntimeMigrationComponents{
			sessions: sessions, profiles: profiles, providers: providers,
		}.Migrate())
	})

	t.Run("delegates prepared agent runtime migration without DDL", func(t *testing.T) {
		preparer := newMockagentRuntimeMigrationPreparer(t)
		sessions := newMockagentRuntimeSchemaMigrator(t)
		profiles := newMockagentRuntimeSchemaMigrator(t)
		providers := newMockagentRuntimeSchemaMigrator(t)
		preparer.EXPECT().Prepare().Return(agentRuntimeMigrationComponents{
			sessions: sessions, profiles: profiles, providers: providers,
		}, nil).Once()
		mock.InOrder(
			sessions.EXPECT().AutoMigrate().Return(nil).Once(),
			profiles.EXPECT().AutoMigrate().Return(nil).Once(),
			providers.EXPECT().AutoMigrate().Return(nil).Once(),
		)
		require.NoError(t, newAgentRuntimeMigrator(preparer).Migrate())

		expectedErr := errors.New(fake.UUID().V4())
		preparer = newMockagentRuntimeMigrationPreparer(t)
		preparer.EXPECT().Prepare().Return(agentRuntimeMigrationComponents{}, expectedErr).Once()
		require.ErrorIs(t, newAgentRuntimeMigrator(preparer).Migrate(), expectedErr)
	})

	t.Run("reports prepared agent runtime construction errors without DDL", func(t *testing.T) {
		makeServices := func(t *testing.T) (agent.ProvidersConfigService, agent.AgentProfilesService) {
			t.Helper()
			providers, err := agent.NewFileProvidersConfigService(t.TempDir(), slog.Default())
			require.NoError(t, err)
			profiles, err := agent.NewFileAgentProfilesService(t.TempDir(), slog.Default())
			require.NoError(t, err)
			return providers, profiles
		}

		t.Run("providers", func(t *testing.T) {
			expectedErr := errors.New(fake.UUID().V4())
			_, err := newAgentRuntimeMigrationPreparer(agentRuntimeMigrationComponentConstructors{
				providers: func() (agent.ProvidersConfigService, error) { return nil, expectedErr },
			}).Prepare()
			require.ErrorIs(t, err, expectedErr)
		})
		t.Run("prepared components", func(t *testing.T) {
			providers, profiles := makeServices(t)
			providerMigrator := newMockagentRuntimeSchemaMigrator(t)
			components, err := newAgentRuntimeMigrationPreparer(agentRuntimeMigrationComponentConstructors{
				providers: func() (agent.ProvidersConfigService, error) { return providers, nil },
				profiles:  func() (agent.AgentProfilesService, error) { return profiles, nil },
				runner: func(agent.ProvidersConfigService, agent.AgentProfilesService) (*agent.Runner, error) {
					return agent.NewRunner(agent.RunnerArgs{
						ProvidersConfigService: providers, AgentProfilesService: profiles,
					})
				},
				providerMigrator: func(agent.ProvidersConfigService) (agentRuntimeSchemaMigrator, bool) {
					return providerMigrator, true
				},
			}).Prepare()
			require.NoError(t, err)
			require.NotNil(t, components.sessions)
			require.Same(t, profiles, components.profiles)
			require.Same(t, providerMigrator, components.providers)
		})
		t.Run("profiles", func(t *testing.T) {
			providers, _ := makeServices(t)
			expectedErr := errors.New(fake.UUID().V4())
			_, err := newAgentRuntimeMigrationPreparer(agentRuntimeMigrationComponentConstructors{
				providers: func() (agent.ProvidersConfigService, error) { return providers, nil },
				profiles:  func() (agent.AgentProfilesService, error) { return nil, expectedErr },
			}).Prepare()
			require.ErrorIs(t, err, expectedErr)
		})
		t.Run("runner", func(t *testing.T) {
			providers, profiles := makeServices(t)
			expectedErr := errors.New(fake.UUID().V4())
			_, err := newAgentRuntimeMigrationPreparer(agentRuntimeMigrationComponentConstructors{
				providers: func() (agent.ProvidersConfigService, error) { return providers, nil },
				profiles:  func() (agent.AgentProfilesService, error) { return profiles, nil },
				runner: func(agent.ProvidersConfigService, agent.AgentProfilesService) (*agent.Runner, error) {
					return nil, expectedErr
				},
			}).Prepare()
			require.ErrorIs(t, err, expectedErr)
		})
		t.Run("providers migration capability", func(t *testing.T) {
			providers, profiles := makeServices(t)
			_, err := newAgentRuntimeMigrationPreparer(agentRuntimeMigrationComponentConstructors{
				providers: func() (agent.ProvidersConfigService, error) { return providers, nil },
				profiles:  func() (agent.AgentProfilesService, error) { return profiles, nil },
				runner: func(agent.ProvidersConfigService, agent.AgentProfilesService) (*agent.Runner, error) {
					return agent.NewRunner(agent.RunnerArgs{
						ProvidersConfigService: providers,
						AgentProfilesService:   profiles,
					})
				},
			}).Prepare()
			require.ErrorContains(t, err, "does not support auto migration")
		})
	})

	t.Run("returns prepared agent runtime migration errors without DDL", func(t *testing.T) {
		for _, component := range []string{"sessions", "profiles", "providers"} {
			t.Run(component, func(t *testing.T) {
				expectedErr := errors.New(fake.UUID().V4())
				sessions := newMockagentRuntimeSchemaMigrator(t)
				profiles := newMockagentRuntimeSchemaMigrator(t)
				providers := newMockagentRuntimeSchemaMigrator(t)
				switch component {
				case "sessions":
					sessions.EXPECT().AutoMigrate().Return(expectedErr).Once()
				case "profiles":
					sessions.EXPECT().AutoMigrate().Return(nil).Once()
					profiles.EXPECT().AutoMigrate().Return(expectedErr).Once()
				case "providers":
					sessions.EXPECT().AutoMigrate().Return(nil).Once()
					profiles.EXPECT().AutoMigrate().Return(nil).Once()
					providers.EXPECT().AutoMigrate().Return(expectedErr).Once()
				}
				require.ErrorIs(t, agentRuntimeMigrationComponents{
					sessions: sessions, profiles: profiles, providers: providers,
				}.Migrate(), expectedErr)
			})
		}
	})

	t.Run("surfaces database preparer construction without starting a database", func(t *testing.T) {
		preparer := newDatabaseAgentRuntimeMigrationPreparer(
			"postgres://localhost:invalid/"+fake.UUID().V4(), fake.Letter(), slog.New(slog.DiscardHandler),
		)
		_, err := preparer.Prepare()
		require.Error(t, err)
	})

	t.Run("prepares database components through injected construction factories", func(t *testing.T) {
		providers, profiles := func(t *testing.T) (agent.ProvidersConfigService, agent.AgentProfilesService) {
			t.Helper()
			providers, err := agent.NewFileProvidersConfigService(t.TempDir(), slog.New(slog.DiscardHandler))
			require.NoError(t, err)
			profiles, err := agent.NewFileAgentProfilesService(t.TempDir(), slog.New(slog.DiscardHandler))
			require.NoError(t, err)
			return providers, profiles
		}(t)
		runnerErr := errors.New(fake.UUID().V4())
		preparer := newDatabaseAgentRuntimeMigrationPreparerWithFactories(
			fake.UUID().V4(), fake.Letter(), slog.New(slog.DiscardHandler), agentRuntimeDatabaseMigrationFactories{
				providers: func(string, *slog.Logger, string) (agent.ProvidersConfigService, error) {
					return providers, nil
				},
				profiles: func(string, *slog.Logger, string) (agent.AgentProfilesService, error) {
					return profiles, nil
				},
				runner: func(
					agent.ProvidersConfigService,
					agent.AgentProfilesService,
					*slog.Logger,
					string,
					string,
				) (*agent.Runner, error) {
					return nil, runnerErr
				},
			},
		)
		_, err := preparer.Prepare()
		require.ErrorIs(t, err, runnerErr)
		_, err = newDatabaseAgentRuntimeRunner(
			providers,
			profiles,
			slog.New(slog.DiscardHandler),
			"postgres://localhost:invalid/"+fake.UUID().V4(),
			fake.Letter(),
		)
		require.Error(t, err)
	})
}
