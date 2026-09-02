package internal

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/agent"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFinalAgentRuntimeCoverage(t *testing.T) {
	fake := faker.New()
	makeDeps := func(t *testing.T) RuntimeDeps {
		t.Helper()
		return RuntimeDeps{
			RootLogger:                      telemetry.RootTestLogger(),
			DataDir:                         t.TempDir(),
			PlatformAgentsPath:              t.TempDir(),
			AgentRuntimeStorageType:         "file",
			AgentRuntimeDatabaseTablePrefix: "agent_",
			SkillsMaxSkillBytes:             1024,
			SkillsMaxCatalogEntries:         4,
			ToolsRegistry:                   agent.NewToolsRegistry(),
		}
	}
	t.Run("registers optional execution tools and stops before invalid file services", func(t *testing.T) {
		deps := makeDeps(t)
		deps.ExecEnabled, deps.ExecMaxOutputBytes, deps.ExecDefaultTimeout, deps.ExecMaxConcurrentJobs = true, 1024, time.Second, 1
		opts, err := workspacefsRegisterOptions(deps)
		require.NoError(t, err)
		require.Len(t, opts, 3)
		deps.DataDir = filepath.Join(t.TempDir(), fake.UUID().V4())
		require.NoError(t, os.WriteFile(deps.DataDir, []byte(fake.UUID().V4()), 0o600))
		_, err = newRuntime(deps)
		require.Error(t, err)
	})
	t.Run(
		"builds a catalog from an absent optional skills root and rejects invalid database runtime",
		func(t *testing.T) {
			deps := makeDeps(t)
			deps.SkillsEnabled = true
			deps.SkillsPaths = []string{filepath.Join(t.TempDir(), fake.UUID().V4())}
			_, err := buildRunnerOpts(deps, agent.NewToolsRegistry())
			require.NoError(t, err)
			deps = makeDeps(t)
			deps.AgentRuntimeStorageType = storageTypeDatabase
			_, err = newRuntime(deps)
			require.Error(t, err)
			filePath := filepath.Join(t.TempDir(), fake.UUID().V4())
			require.NoError(t, os.WriteFile(filePath, []byte(fake.UUID().V4()), 0o600))
			fileDeps := makeDeps(t)
			fileDeps.DataDir = filePath
			_, err = newProvidersConfigService(fileDeps)
			require.Error(t, err)
			_, err = newAgentProfilesService(fileDeps)
			require.Error(t, err)
		},
	)
}

func TestDatabaseMigrationOrchestration(t *testing.T) {
	fake := faker.New()

	t.Run("reports missing component dependencies before database work", func(t *testing.T) {
		migrator := &DatabaseMigrator{rootLogger: slog.Default()}
		require.NoError(t, migrator.migrateAgentRuntime(t.Context()))
		require.Error(t, migrator.migrateAuthentication(t.Context()))
		require.Error(t, migrator.migrateAppDispatch(t.Context()))
		require.Error(t, migrator.migrateJobs(t.Context()))
		require.Error(t, migrator.migrateFinance(t.Context()))
		migrator.agentRuntimeStorageType = storageTypeDatabase
		require.Error(t, migrator.migrateAgentRuntime(t.Context()))
	})

	t.Run("adapts component migration functions", func(t *testing.T) {
		expectedErr := errors.New(fake.UUID().V4())
		migrator := componentMigratorFunc(func(context.Context) error { return expectedErr })
		require.ErrorIs(t, migrator.Migrate(t.Context()), expectedErr)
		created, err := componentMigratorFactory(func() (componentMigrator, error) {
			return migrator, nil
		})()
		require.NoError(t, err)
		require.IsType(t, migrator, created)
		defaultMigrator := NewDatabaseMigrator(DatabaseMigrationDeps{RootLogger: slog.Default()})
		require.NotNil(t, defaultMigrator)
		err = newAuthenticationMigrator(nil, nil).Migrate(t.Context())
		require.Error(t, err)
	})

	t.Run("runs prepared concrete migration adapters without a database", func(t *testing.T) {
		for _, adapter := range []struct {
			name string
			new  func(componentMigratorFactory) componentMigrator
		}{
			{"app dispatch", func(factory componentMigratorFactory) componentMigrator { return newAppDispatchMigrator(factory) }},
			{"jobs", func(factory componentMigratorFactory) componentMigrator { return newJobsMigrator(factory) }},
			{"finance", func(factory componentMigratorFactory) componentMigrator { return newFinanceMigrator(factory) }},
		} {
			t.Run(adapter.name, func(t *testing.T) {
				prepared := newMockcomponentMigrator(t)
				factory := componentMigratorFactory(func() (componentMigrator, error) {
					return prepared, nil
				})
				prepared.EXPECT().Migrate(t.Context()).Return(nil).Once()
				require.NoError(t, adapter.new(factory).Migrate(t.Context()))

				createErr := errors.New(fake.UUID().V4())
				factory = func() (componentMigrator, error) { return nil, createErr }
				require.ErrorIs(t, adapter.new(factory).Migrate(t.Context()), createErr)

				migrateErr := errors.New(fake.UUID().V4())
				prepared = newMockcomponentMigrator(t)
				factory = func() (componentMigrator, error) { return prepared, nil }
				prepared.EXPECT().Migrate(t.Context()).Return(migrateErr).Once()
				require.ErrorIs(t, adapter.new(factory).Migrate(t.Context()), migrateErr)
			})
		}
	})

	t.Run("prepares concrete migration factories without schema DDL", func(t *testing.T) {
		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		dsn := "postgres://" + fake.UUID().V4()

		appDispatch, err := newAppDispatchMigratorFactory(dsn, fake.Lorem().Word(), db)()
		require.NoError(t, err)
		require.NotNil(t, appDispatch)

		jobs, err := newJobsMigratorFactory(dsn, fake.Lorem().Word(), db)()
		require.NoError(t, err)
		require.NotNil(t, jobs)

		finance, err := newFinanceMigratorFactory(dsn, db, slog.Default())()
		require.NoError(t, err)
		require.NotNil(t, finance)
		require.NoError(t, databaseMock.ExpectationsWereMet())
	})

	t.Run("returns concrete migration factory construction errors without schema DDL", func(t *testing.T) {
		_, err := newAppDispatchMigratorFactory(fake.UUID().V4(), fake.Lorem().Word(), nil)()
		require.Error(t, err)
		_, err = newJobsMigratorFactory(fake.UUID().V4(), fake.Lorem().Word(), nil)()
		require.Error(t, err)
		_, err = newFinanceMigratorFactory(fake.UUID().V4(), nil, slog.Default())()
		require.Error(t, err)
	})

	t.Run("runs prepared authentication schema adapters in order", func(t *testing.T) {
		users := newMockautoMigrator(t)
		refreshTokens := newMockautoMigrator(t)
		calls := []*mock.Call{
			users.EXPECT().AutoMigrate().Return(nil).Once(),
			refreshTokens.EXPECT().AutoMigrate().Return(nil).Once(),
		}
		mock.InOrder(calls...)
		require.NoError(t, newAuthenticationMigrator(users, refreshTokens).Migrate(t.Context()))

		expectedErr := errors.New(fake.UUID().V4())
		users = newMockautoMigrator(t)
		refreshTokens = newMockautoMigrator(t)
		users.EXPECT().AutoMigrate().Return(expectedErr).Once()
		require.ErrorIs(t, newAuthenticationMigrator(users, refreshTokens).Migrate(t.Context()), expectedErr)

		users = newMockautoMigrator(t)
		refreshTokens = newMockautoMigrator(t)
		users.EXPECT().AutoMigrate().Return(nil).Once()
		refreshTokens.EXPECT().AutoMigrate().Return(expectedErr).Once()
		require.ErrorIs(t, newAuthenticationMigrator(users, refreshTokens).Migrate(t.Context()), expectedErr)
	})

	t.Run("delegates and wraps agent runtime migration", func(t *testing.T) {
		expectedErr := errors.New(fake.UUID().V4())
		agentRuntimeMigrator := NewMockAgentRuntimeMigrator(t)
		migrator := &DatabaseMigrator{
			rootLogger: slog.Default(), agentRuntimeStorageType: storageTypeDatabase,
			agentRuntimeMigrator: agentRuntimeMigrator,
		}
		agentRuntimeMigrator.EXPECT().Migrate().Return(expectedErr).Once()
		require.ErrorIs(t, migrator.migrateAgentRuntime(t.Context()), expectedErr)
		require.ErrorIs(t, AgentRuntimeMigratorFunc(func() error { return expectedErr }).Migrate(), expectedErr)
		successfulMigrator := NewMockAgentRuntimeMigrator(t)
		successfulMigrator.EXPECT().Migrate().Return(nil).Once()
		migrator.agentRuntimeMigrator = successfulMigrator
		require.NoError(t, migrator.migrateAgentRuntime(t.Context()))
	})

	t.Run("runs component migrators in order without opening a database", func(t *testing.T) {
		authentication := newMockcomponentMigrator(t)
		appDispatch := newMockcomponentMigrator(t)
		jobs := newMockcomponentMigrator(t)
		finance := newMockcomponentMigrator(t)
		calls := []*mock.Call{
			authentication.EXPECT().Migrate(mock.Anything).Return(nil).Once(),
			appDispatch.EXPECT().Migrate(mock.Anything).Return(nil).Once(),
			jobs.EXPECT().Migrate(mock.Anything).Return(nil).Once(),
			finance.EXPECT().Migrate(mock.Anything).Return(nil).Once(),
		}
		mock.InOrder(calls...)

		migrator := NewDatabaseMigrator(DatabaseMigrationDeps{
			RootLogger:              slog.Default(),
			AgentRuntimeStorageType: "file",
			AuthenticationMigrator:  authentication,
			AppDispatchMigrator:     appDispatch,
			JobsMigrator:            jobs,
			FinanceMigrator:         finance,
		})
		require.NoError(t, migrator.Migrate(t.Context()))
	})

	t.Run("wraps component migration errors without executing schema work", func(t *testing.T) {
		expectedErr := errors.New(fake.UUID().V4())
		authentication := newMockcomponentMigrator(t)
		appDispatch := newMockcomponentMigrator(t)
		authentication.EXPECT().Migrate(mock.Anything).Return(nil).Once()
		appDispatch.EXPECT().Migrate(mock.Anything).Return(expectedErr).Once()
		migrator := NewDatabaseMigrator(DatabaseMigrationDeps{
			RootLogger:              slog.Default(),
			AgentRuntimeStorageType: "file",
			AuthenticationMigrator:  authentication,
			AppDispatchMigrator:     appDispatch,
			JobsMigrator:            newMockcomponentMigrator(t),
			FinanceMigrator:         newMockcomponentMigrator(t),
		})
		err := migrator.Migrate(t.Context())
		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "migrate app dispatch transport schema")
	})

	t.Run("returns component migration errors with their component", func(t *testing.T) {
		expectedErr := errors.New(fake.UUID().V4())
		migrator := &DatabaseMigrator{rootLogger: slog.Default()}
		err := migrator.runStep(t.Context(), "finance", func(context.Context) error { return expectedErr })
		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "migrate finance schema")
	})
}
