package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/gemyago/sumweave/runtime/agent"
)

type DatabaseMigrationDeps struct {
	RootLogger                      *slog.Logger
	AgentRuntimeStorageType         string
	AgentRuntimeDatabaseDSN         string
	AgentRuntimeDatabaseTablePrefix string
	ApplicationDatabaseDSN          string
	ApplicationDatabaseTablePrefix  string
	ApplicationSQLDB                *sql.DB
	AuthUsers                       *auth.UserStore
	AuthRefreshTokens               *auth.RefreshTokenStore
}

// DatabaseMigrator runs the explicit schema setup flow for application subsystems.
type DatabaseMigrator struct {
	rootLogger                      *slog.Logger
	agentRuntimeStorageType         string
	agentRuntimeDatabaseDSN         string
	agentRuntimeDatabaseTablePrefix string
	applicationDatabaseDSN          string
	applicationDatabaseTablePrefix  string
	applicationSQLDB                *sql.DB
	authUsers                       *auth.UserStore
	authRefreshTokens               *auth.RefreshTokenStore
}

// NewDatabaseMigrator constructs the migration runner from direct dependencies.
func NewDatabaseMigrator(deps DatabaseMigrationDeps) *DatabaseMigrator {
	return &DatabaseMigrator{
		rootLogger:                      deps.RootLogger,
		agentRuntimeStorageType:         deps.AgentRuntimeStorageType,
		agentRuntimeDatabaseDSN:         deps.AgentRuntimeDatabaseDSN,
		agentRuntimeDatabaseTablePrefix: deps.AgentRuntimeDatabaseTablePrefix,
		applicationDatabaseDSN:          deps.ApplicationDatabaseDSN,
		applicationDatabaseTablePrefix:  deps.ApplicationDatabaseTablePrefix,
		applicationSQLDB:                deps.ApplicationSQLDB,
		authUsers:                       deps.AuthUsers,
		authRefreshTokens:               deps.AuthRefreshTokens,
	}
}

type componentMigrationError struct {
	component string
	err       error
}

func (e *componentMigrationError) Error() string {
	return fmt.Sprintf("migrate %s schema", e.component)
}

func (e *componentMigrationError) Unwrap() error { return e.err }

func (m *DatabaseMigrator) Migrate(ctx context.Context) error {
	for _, step := range []struct {
		component string
		run       func(context.Context) error
	}{
		{"agent runtime", m.migrateAgentRuntime},
		{"authentication", m.migrateAuthentication},
		{"app dispatch transport", m.migrateAppDispatch},
		{"durable jobs", m.migrateJobs},
		{"finance", m.migrateFinance},
	} {
		if err := m.runStep(ctx, step.component, step.run); err != nil {
			return err
		}
	}
	return nil
}

func (m *DatabaseMigrator) runStep(ctx context.Context, component string, run func(context.Context) error) error {
	m.rootLogger.InfoContext(ctx, "running database migration", "component", component)
	if err := run(ctx); err != nil {
		m.rootLogger.ErrorContext(ctx, "database migration failed", "component", component)
		return &componentMigrationError{component: component, err: err}
	}
	m.rootLogger.InfoContext(ctx, "database migration complete", "component", component)
	return nil
}

func (m *DatabaseMigrator) migrateAgentRuntime(_ context.Context) error {
	if m.agentRuntimeStorageType != storageTypeDatabase {
		return nil
	}
	services, err := newRuntimeServices(RuntimeDeps{
		RootLogger:                      m.rootLogger,
		AgentRuntimeStorageType:         m.agentRuntimeStorageType,
		AgentRuntimeDatabaseDSN:         m.agentRuntimeDatabaseDSN,
		AgentRuntimeDatabaseTablePrefix: m.agentRuntimeDatabaseTablePrefix,
	})
	if err != nil {
		return fmt.Errorf("create agent runtime services: %w", err)
	}
	providers, profiles := services.providersConfigSvc, services.agentProfilesSvc
	runner, err := agent.NewRunner(
		agent.RunnerArgs{ProvidersConfigService: providers, AgentProfilesService: profiles},
		agent.WithLogger(m.rootLogger),
		agent.WithDatabaseStorage(m.agentRuntimeDatabaseDSN),
		agent.WithDatabaseTablePrefix(m.agentRuntimeDatabaseTablePrefix),
	)
	if err != nil {
		return fmt.Errorf("create agent runner: %w", err)
	}
	if err = runner.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate sessions database: %w", err)
	}
	if err = profiles.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate agent profiles database: %w", err)
	}
	//nolint:errcheck // The concrete database provider service supports AutoMigrate.
	migrator := providers.(interface{ AutoMigrate() error })
	if err = migrator.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate providers config database: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateAuthentication(_ context.Context) error {
	if m.authUsers == nil || m.authRefreshTokens == nil {
		return errors.New("auth stores are required")
	}
	if err := m.authUsers.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate auth users: %w", err)
	}
	if err := m.authRefreshTokens.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate auth refresh tokens: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateAppDispatch(ctx context.Context) error {
	migrator, err := appdispatch.NewMigrator(
		appdispatch.Config{DatabaseDSN: m.applicationDatabaseDSN, TablePrefix: m.applicationDatabaseTablePrefix},
		m.applicationSQLDB,
	)
	if err != nil {
		return fmt.Errorf("create app dispatch migrator: %w", err)
	}
	if err = migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate app dispatch transport: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateJobs(_ context.Context) error {
	store, err := jobspkg.NewStore(
		m.applicationSQLDB,
		m.applicationDatabaseDSN,
		jobspkg.StoreOpts{TablePrefix: m.applicationDatabaseTablePrefix + "jobs_"},
	)
	if err != nil {
		return fmt.Errorf("create jobs store: %w", err)
	}
	if err = store.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate jobs store: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateFinance(ctx context.Context) error {
	database, err := persistence.NewDatabase(
		m.applicationSQLDB,
		m.applicationDatabaseDSN,
		persistence.WithLogger(m.rootLogger),
	)
	if err != nil {
		return fmt.Errorf("open finance database: %w", err)
	}
	if err = persistence.NewMigrator(database).Migrate(ctx); err != nil {
		return fmt.Errorf("migrate finance schema: %w", err)
	}
	return nil
}
