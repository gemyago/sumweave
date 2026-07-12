package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/gemyago/signal-foundry/runtime/audit"
	"github.com/gemyago/signal-foundry/runtime/backtest"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/execution"
	rtgovernor "github.com/gemyago/signal-foundry/runtime/governor"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"go.uber.org/dig"
)

type DatabaseMigrationDeps struct {
	dig.In

	RootLogger *slog.Logger

	AgentRuntimeStorageType         string `name:"config.agentRuntime.storage.type"`
	AgentRuntimeDatabaseDSN         string `name:"config.agentRuntime.database.dsn"`
	AgentRuntimeDatabaseTablePrefix string `name:"config.agentRuntime.database.tablePrefix"`

	DataLayerDatabaseDSN         string `name:"config.dataLayer.database.dsn"`
	DataLayerDatabaseTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
	DataLayerSQLDB               *sql.DB

	DataStore *data.DatabaseStore
}

// DatabaseMigrator runs the explicit backend schema setup flow.
type DatabaseMigrator struct {
	rootLogger *slog.Logger

	agentRuntimeStorageType         string
	agentRuntimeDatabaseDSN         string
	agentRuntimeDatabaseTablePrefix string

	dataLayerDatabaseDSN         string
	dataLayerDatabaseTablePrefix string
	dataLayerSQLDB               *sql.DB

	dataStore *data.DatabaseStore
}

func newDatabaseMigrator(deps DatabaseMigrationDeps) *DatabaseMigrator {
	return &DatabaseMigrator{
		rootLogger: deps.RootLogger,

		agentRuntimeStorageType:         deps.AgentRuntimeStorageType,
		agentRuntimeDatabaseDSN:         deps.AgentRuntimeDatabaseDSN,
		agentRuntimeDatabaseTablePrefix: deps.AgentRuntimeDatabaseTablePrefix,

		dataLayerDatabaseDSN:         deps.DataLayerDatabaseDSN,
		dataLayerDatabaseTablePrefix: deps.DataLayerDatabaseTablePrefix,
		dataLayerSQLDB:               deps.DataLayerSQLDB,

		dataStore: deps.DataStore,
	}
}

type componentMigrationError struct {
	component string
	err       error
}

func (e *componentMigrationError) Error() string {
	return fmt.Sprintf("migrate %s schema", e.component)
}

func (e *componentMigrationError) Unwrap() error {
	return e.err
}

func (m *DatabaseMigrator) Migrate(ctx context.Context) error {
	for _, step := range []struct {
		component string
		run       func(context.Context) error
	}{
		{component: "agent runtime", run: m.migrateAgentRuntime},
		{component: "data layer", run: m.migrateDataLayer},
		{component: "app dispatch transport", run: m.migrateAppDispatch},
		{component: "durable jobs", run: m.migrateJobs},
		{component: "finance", run: m.migrateFinance},
		{component: "strategy artifacts", run: m.migrateStrategyArtifacts},
		{component: "strategy version registry", run: m.migrateStrategyVersionRegistry},
		{component: "evaluation governor policy", run: m.migrateEvaluationGovernorPolicy},
		{component: "evaluation audit", run: m.migrateEvaluationAudit},
		{component: "evaluation execution", run: m.migrateEvaluationExecution},
		{component: "evaluation backtest", run: m.migrateEvaluationBacktest},
	} {
		if err := m.runStep(ctx, step.component, step.run); err != nil {
			return err
		}
	}

	return nil
}

func (m *DatabaseMigrator) runStep(
	ctx context.Context,
	component string,
	run func(context.Context) error,
) error {
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

	providersConfigSvc, err := agent.NewDatabaseProvidersConfigService(
		m.agentRuntimeDatabaseDSN,
		m.rootLogger,
		m.agentRuntimeDatabaseTablePrefix,
	)
	if err != nil {
		return fmt.Errorf("create providers config service: %w", err)
	}

	agentProfilesSvc, err := agent.NewDatabaseAgentProfilesService(
		m.agentRuntimeDatabaseDSN,
		m.rootLogger,
		m.agentRuntimeDatabaseTablePrefix,
	)
	if err != nil {
		return fmt.Errorf("create database agent profiles service: %w", err)
	}

	runner, err := agent.NewRunner(
		agent.RunnerArgs{
			ProvidersConfigService: providersConfigSvc,
			AgentProfilesService:   agentProfilesSvc,
		},
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
	if err = agentProfilesSvc.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate agent profiles database: %w", err)
	}
	providersConfigMigrator, ok := providersConfigSvc.(interface{ AutoMigrate() error })
	if !ok {
		return errors.New("database providers config service does not support auto migration")
	}
	if err = providersConfigMigrator.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate providers config database: %w", err)
	}

	return nil
}

func (m *DatabaseMigrator) migrateDataLayer(_ context.Context) error {
	if err := m.dataStore.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate data-layer database: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateAppDispatch(ctx context.Context) error {
	config := appdispatch.Config{
		DatabaseDSN: m.dataLayerDatabaseDSN,
		TablePrefix: m.dataLayerDatabaseTablePrefix,
	}
	migrator, err := appdispatch.NewMigrator(config, m.dataLayerSQLDB)
	if err != nil {
		return fmt.Errorf("create app dispatch migrator: %w", err)
	}
	if err = migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate app dispatch transport: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateJobs(_ context.Context) error {
	store, err := jobspkg.NewStore(m.dataLayerSQLDB, m.dataLayerDatabaseDSN, jobspkg.StoreOpts{
		TablePrefix: m.dataLayerDatabaseTablePrefix + "jobs_",
	})
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
		m.dataLayerSQLDB,
		m.dataLayerDatabaseDSN,
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

func (m *DatabaseMigrator) migrateStrategyArtifacts(_ context.Context) error {
	store, err := rtstrategy.NewArtifactDatabaseStore(
		m.dataLayerSQLDB,
		m.dataLayerDatabaseDSN,
		rtstrategy.ArtifactDatabaseStoreOpts{TablePrefix: m.dataLayerDatabaseTablePrefix + "strategy_"},
	)
	if err != nil {
		return fmt.Errorf("create strategy artifact store: %w", err)
	}
	if err = store.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate strategy artifact store: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateStrategyVersionRegistry(_ context.Context) error {
	artifactStore, err := rtstrategy.NewArtifactDatabaseStore(
		m.dataLayerSQLDB,
		m.dataLayerDatabaseDSN,
		rtstrategy.ArtifactDatabaseStoreOpts{TablePrefix: m.dataLayerDatabaseTablePrefix + "strategy_"},
	)
	if err != nil {
		return fmt.Errorf("create strategy artifact store: %w", err)
	}
	service, err := rtstrategy.NewVersionRegistryService(
		m.dataLayerSQLDB,
		m.dataLayerDatabaseDSN,
		rtstrategy.VersionRegistryServiceDeps{
			ArtifactStore: artifactStore,
			TablePrefix:   m.dataLayerDatabaseTablePrefix + "strategy_",
		},
	)
	if err != nil {
		return fmt.Errorf("create strategy version registry service: %w", err)
	}
	if err = service.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate strategy version registry service: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateEvaluationGovernorPolicy(_ context.Context) error {
	store, err := rtgovernor.NewArtifactDatabaseStore(
		m.dataLayerSQLDB,
		m.dataLayerDatabaseDSN,
		rtgovernor.ArtifactDatabaseStoreOpts{TablePrefix: m.dataLayerDatabaseTablePrefix + "evaluation_"},
	)
	if err != nil {
		return fmt.Errorf("create governor policy artifact store: %w", err)
	}
	if err = store.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate governor policy artifact store: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateEvaluationAudit(_ context.Context) error {
	store, err := audit.NewDatabaseStore(
		m.dataLayerSQLDB,
		m.dataLayerDatabaseDSN,
		audit.DatabaseStoreOpts{TablePrefix: m.dataLayerDatabaseTablePrefix + "evaluation_"},
	)
	if err != nil {
		return fmt.Errorf("create evaluation audit store: %w", err)
	}
	if err = store.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate evaluation audit store: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateEvaluationExecution(_ context.Context) error {
	store, err := execution.NewDatabaseStore(
		m.dataLayerSQLDB,
		m.dataLayerDatabaseDSN,
		execution.DatabaseStoreOpts{TablePrefix: m.dataLayerDatabaseTablePrefix + "evaluation_"},
	)
	if err != nil {
		return fmt.Errorf("create evaluation execution store: %w", err)
	}
	if err = store.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate evaluation execution store: %w", err)
	}
	return nil
}

func (m *DatabaseMigrator) migrateEvaluationBacktest(_ context.Context) error {
	store, err := backtest.NewDatabaseStore(
		m.dataLayerSQLDB,
		m.dataLayerDatabaseDSN,
		backtest.DatabaseStoreOpts{TablePrefix: m.dataLayerDatabaseTablePrefix + "evaluation_"},
	)
	if err != nil {
		return fmt.Errorf("create evaluation backtest store: %w", err)
	}
	if err = store.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate evaluation backtest store: %w", err)
	}
	return nil
}
