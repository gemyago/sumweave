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
)

// AgentRuntimeMigrator applies agent-runtime schema migrations.
type AgentRuntimeMigrator interface {
	Migrate() error
}

type componentMigrator interface {
	Migrate(context.Context) error
}

type componentMigratorFactory func() (componentMigrator, error)

type autoMigrator interface {
	AutoMigrate() error
}

type componentMigratorFunc func(context.Context) error

func (f componentMigratorFunc) Migrate(ctx context.Context) error {
	return f(ctx)
}

// AgentRuntimeMigratorFunc adapts a migration function for wireup.
type AgentRuntimeMigratorFunc func() error

// Migrate applies the configured agent-runtime migration.
func (f AgentRuntimeMigratorFunc) Migrate() error {
	return f()
}

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
	AgentRuntimeMigrator            AgentRuntimeMigrator
	AuthenticationMigrator          componentMigrator
	AppDispatchMigrator             componentMigrator
	JobsMigrator                    componentMigrator
	FinanceMigrator                 componentMigrator
}

// DatabaseMigrator runs the explicit schema setup flow for application subsystems.
type DatabaseMigrator struct {
	rootLogger              *slog.Logger
	agentRuntimeStorageType string
	agentRuntimeMigrator    AgentRuntimeMigrator
	authenticationMigrator  componentMigrator
	appDispatchMigrator     componentMigrator
	jobsMigrator            componentMigrator
	financeMigrator         componentMigrator
}

// NewDatabaseMigrator constructs the migration runner from direct dependencies.
func NewDatabaseMigrator(deps DatabaseMigrationDeps) *DatabaseMigrator {
	migrator := &DatabaseMigrator{
		rootLogger:              deps.RootLogger,
		agentRuntimeStorageType: deps.AgentRuntimeStorageType,
		agentRuntimeMigrator:    deps.AgentRuntimeMigrator,
		authenticationMigrator:  deps.AuthenticationMigrator,
		appDispatchMigrator:     deps.AppDispatchMigrator,
		jobsMigrator:            deps.JobsMigrator,
		financeMigrator:         deps.FinanceMigrator,
	}
	if migrator.authenticationMigrator == nil {
		migrator.authenticationMigrator = newAuthenticationMigrator(deps.AuthUsers, deps.AuthRefreshTokens)
	}
	if migrator.appDispatchMigrator == nil {
		migrator.appDispatchMigrator = newAppDispatchMigrator(newAppDispatchMigratorFactory(
			deps.ApplicationDatabaseDSN,
			deps.ApplicationDatabaseTablePrefix,
			deps.ApplicationSQLDB,
		))
	}
	if migrator.jobsMigrator == nil {
		migrator.jobsMigrator = newJobsMigrator(newJobsMigratorFactory(
			deps.ApplicationDatabaseDSN,
			deps.ApplicationDatabaseTablePrefix,
			deps.ApplicationSQLDB,
		))
	}
	if migrator.financeMigrator == nil {
		migrator.financeMigrator = newFinanceMigrator(newFinanceMigratorFactory(
			deps.ApplicationDatabaseDSN,
			deps.ApplicationSQLDB,
			deps.RootLogger,
		))
	}
	return migrator
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
		{"agent runtime", m.migrateAgentRuntime}, {"authentication", m.migrateAuthentication}, {"app dispatch transport", m.migrateAppDispatch}, {"durable jobs", m.migrateJobs}, {"finance", m.migrateFinance},
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
	if m.agentRuntimeMigrator == nil {
		return errors.New("agent runtime migrator is required")
	}
	if err := m.agentRuntimeMigrator.Migrate(); err != nil {
		return fmt.Errorf("migrate agent runtime: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateAuthentication(ctx context.Context) error {
	if m.authenticationMigrator == nil {
		return errors.New("authentication migrator is required")
	}
	return m.authenticationMigrator.Migrate(ctx)
}

func (m *DatabaseMigrator) migrateAppDispatch(ctx context.Context) error {
	if m.appDispatchMigrator == nil {
		return errors.New("app dispatch migrator is required")
	}
	return m.appDispatchMigrator.Migrate(ctx)
}

func (m *DatabaseMigrator) migrateJobs(ctx context.Context) error {
	if m.jobsMigrator == nil {
		return errors.New("jobs migrator is required")
	}
	return m.jobsMigrator.Migrate(ctx)
}

func (m *DatabaseMigrator) migrateFinance(ctx context.Context) error {
	if m.financeMigrator == nil {
		return errors.New("finance migrator is required")
	}
	return m.financeMigrator.Migrate(ctx)
}

// The migration command is the sole owner of concrete schema migration coverage.
func newAuthenticationMigrator(
	users autoMigrator,
	refreshTokens autoMigrator,
) componentMigratorFunc {
	return componentMigratorFunc(func(context.Context) error {
		if users == nil || refreshTokens == nil {
			return errors.New("auth stores are required")
		}
		if err := users.AutoMigrate(); err != nil {
			return fmt.Errorf("auto migrate auth users: %w", err)
		}
		if err := refreshTokens.AutoMigrate(); err != nil {
			return fmt.Errorf("auto migrate auth refresh tokens: %w", err)
		}
		return nil
	})
}

// The migration command is the sole owner of concrete schema migration coverage.
func newAppDispatchMigrator(
	factory componentMigratorFactory,
) componentMigratorFunc {
	return componentMigratorFunc(func(ctx context.Context) error {
		migrator, err := factory()
		if err != nil {
			return fmt.Errorf("create app dispatch migrator: %w", err)
		}
		if err = migrator.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate app dispatch transport: %w", err)
		}
		return nil
	})
}

func newAppDispatchMigratorFactory(
	dsn, tablePrefix string,
	db *sql.DB,
) componentMigratorFactory {
	return func() (componentMigrator, error) {
		return appdispatch.NewMigrator(appdispatch.Config{DatabaseDSN: dsn, TablePrefix: tablePrefix}, db)
	}
}

// The migration command is the sole owner of concrete schema migration coverage.
func newJobsMigrator(
	factory componentMigratorFactory,
) componentMigratorFunc {
	return componentMigratorFunc(func(ctx context.Context) error {
		migrator, err := factory()
		if err != nil {
			return fmt.Errorf("create jobs store: %w", err)
		}
		return migrator.Migrate(ctx)
	})
}

func newJobsMigratorFactory(
	dsn, tablePrefix string,
	db *sql.DB,
) componentMigratorFactory {
	return func() (componentMigrator, error) {
		store, err := jobspkg.NewStore(db, dsn, jobspkg.StoreOpts{TablePrefix: tablePrefix + "jobs_"})
		if err != nil {
			return nil, err
		}
		return componentMigratorFunc(func(context.Context) error {
			if migrateErr := store.AutoMigrate(); migrateErr != nil {
				return fmt.Errorf("auto migrate jobs store: %w", migrateErr)
			}
			return nil
		}), nil
	}
}

// The migration command is the sole owner of concrete schema migration coverage.
func newFinanceMigrator(
	factory componentMigratorFactory,
) componentMigratorFunc {
	return componentMigratorFunc(func(ctx context.Context) error {
		migrator, err := factory()
		if err != nil {
			return fmt.Errorf("open finance database: %w", err)
		}
		return migrator.Migrate(ctx)
	})
}

func newFinanceMigratorFactory(
	dsn string,
	db *sql.DB,
	logger *slog.Logger,
) componentMigratorFactory {
	return func() (componentMigrator, error) {
		database, err := persistence.NewDatabase(db, dsn, persistence.WithLogger(logger))
		if err != nil {
			return nil, err
		}
		return persistence.NewMigrator(database), nil
	}
}
