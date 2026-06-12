package internal

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"github.com/gemyago/signal-foundry/tools/skills"
	"github.com/gemyago/signal-foundry/tools/workspacefs"
	"go.uber.org/dig"
)

const storageTypeDatabase = "database"

type RuntimeDeps struct {
	dig.In

	RootLogger *slog.Logger

	// DataDir is the base directory for agent data storage.
	// Defaults to "data" relative to the working directory.
	DataDir string `name:"config.dataDir"`

	// Exec tool configuration for workspacefs.
	ExecEnabled           bool          `name:"config.workspacefs.exec.enabled"`
	ExecMaxOutputBytes    int64         `name:"config.workspacefs.exec.maxOutputBytes"`
	ExecDefaultTimeout    time.Duration `name:"config.workspacefs.exec.defaultTimeout"`
	ExecMaxConcurrentJobs int           `name:"config.workspacefs.exec.maxConcurrentJobs"`

	// Agent runtime persistence (sessions, providers config, etc.)
	AgentRuntimeStorageType         string `name:"config.agentRuntime.storage.type"`
	AgentRuntimeDatabaseDSN         string `name:"config.agentRuntime.database.dsn"`
	AgentRuntimeDatabaseTablePrefix string `name:"config.agentRuntime.database.tablePrefix"`
	AgentRuntimeDatabaseAutoMigrate bool   `name:"config.agentRuntime.database.autoMigrate"`

	// Data-layer persistence (canonical instruments, candles, trades)
	DataLayerDatabaseDSN         string `name:"config.dataLayer.database.dsn"`
	DataLayerDatabaseTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
	DataLayerDatabaseAutoMigrate bool   `name:"config.dataLayer.database.autoMigrate"`

	// Skills configuration
	SkillsEnabled           bool     `name:"config.skills.enabled"`
	SkillsPaths             []string `name:"config.skills.paths"`
	SkillsMaxSkillBytes     int      `name:"config.skills.maxSkillBytes"`
	SkillsMaxCatalogEntries int      `name:"config.skills.maxCatalogEntries"`

	// Runtime dependencies
	ToolsRegistry        *agent.ToolsRegistry
	DataStore            *data.DatabaseStore
	DataIngestionService *data.IngestionService
	DataReadService      *data.ReadService
}

type Runtime struct {
	Runner               *agent.Runner
	HTTPHandler          http.Handler
	ToolsRegistry        *agent.ToolsRegistry
	DataStore            *data.DatabaseStore
	DataIngestionService *data.IngestionService
	DataReadService      *data.ReadService
}

type runtimeServices struct {
	providersConfigSvc agent.ProvidersConfigService
	agentProfilesSvc   agent.AgentProfilesService
}

func newProvidersConfigService(deps RuntimeDeps) (agent.ProvidersConfigService, error) { //nolint:ireturn
	if deps.AgentRuntimeStorageType == storageTypeDatabase {
		svc, err := agent.NewDatabaseProvidersConfigService(
			deps.AgentRuntimeDatabaseDSN,
			deps.RootLogger,
			deps.AgentRuntimeDatabaseTablePrefix,
		)
		if err != nil {
			return nil, fmt.Errorf("create database providers config service: %w", err)
		}
		return svc, nil
	}
	svc, err := agent.NewFileProvidersConfigService(deps.DataDir, deps.RootLogger)
	if err != nil {
		return nil, fmt.Errorf("create providers config service: %w", err)
	}
	return svc, nil
}

func newAgentProfilesService(deps RuntimeDeps) (agent.AgentProfilesService, error) { //nolint:ireturn
	if deps.AgentRuntimeStorageType == storageTypeDatabase {
		svc, err := agent.NewDatabaseAgentProfilesService(
			deps.AgentRuntimeDatabaseDSN,
			deps.RootLogger,
			deps.AgentRuntimeDatabaseTablePrefix,
		)
		if err != nil {
			return nil, fmt.Errorf("create database agent profiles service: %w", err)
		}
		return svc, nil
	}

	svc, err := agent.NewFileAgentProfilesService(deps.DataDir, deps.RootLogger)
	if err != nil {
		return nil, fmt.Errorf("create agent profiles service: %w", err)
	}
	return svc, nil
}

func autoMigrateRuntimeServices(
	runner *agent.Runner,
	agentProfilesSvc agent.AgentProfilesService,
) error {
	if err := runner.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate database: %w", err)
	}
	if err := agentProfilesSvc.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate agent profiles database: %w", err)
	}
	return nil
}

func autoMigrateDataLayerStore(store *data.DatabaseStore) error {
	if err := store.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate data-layer database: %w", err)
	}

	return nil
}

func newRuntimeServices(deps RuntimeDeps) (*runtimeServices, error) {
	providersSvc, err := newProvidersConfigService(deps)
	if err != nil {
		return nil, err
	}

	agentProfilesSvc, err := newAgentProfilesService(deps)
	if err != nil {
		return nil, err
	}

	return &runtimeServices{
		providersConfigSvc: providersSvc,
		agentProfilesSvc:   agentProfilesSvc,
	}, nil
}

func registerRuntime(container *dig.Container) error {
	return di.ProvideAll(
		container,
		agent.NewToolsRegistry,
		newDataLayerStore,
		newDataIngestionService,
		newDataReadService,
		newRuntime,
	)
}

func newRuntime(deps RuntimeDeps) (*Runtime, error) {
	toolsRegistry := deps.ToolsRegistry

	registerOpts := []workspacefs.RegisterToolsOpt{
		workspacefs.WithWorkspaces([]workspacefs.WorkspaceConfig{
			{
				Identifier:  "agent-temp",
				Description: "Agent can store temporary files here",
				Path:        fmt.Sprintf("%s/agent-temp", deps.DataDir),
			},
		}),
		workspacefs.WithLogger(deps.RootLogger),
	}
	if deps.ExecEnabled {
		registerOpts = append(registerOpts, workspacefs.WithExec(workspacefs.ExecOptions{
			MaxOutputBytes:    deps.ExecMaxOutputBytes,
			DefaultTimeout:    deps.ExecDefaultTimeout,
			MaxConcurrentJobs: deps.ExecMaxConcurrentJobs,
		}))
	}

	if err := workspacefs.RegisterTools(toolsRegistry, registerOpts...); err != nil {
		return nil, fmt.Errorf("register workspacefs tools: %w", err)
	}

	services, err := newRuntimeServices(deps)
	if err != nil {
		return nil, err
	}

	storageOpt := agent.WithFileSystemStorage(deps.DataDir)
	if deps.AgentRuntimeStorageType == storageTypeDatabase {
		storageOpt = agent.WithDatabaseStorage(deps.AgentRuntimeDatabaseDSN)
	}

	runnerOpts := []agent.RunnerOpt{
		agent.WithLogger(deps.RootLogger),
		storageOpt,
		agent.WithDatabaseTablePrefix(deps.AgentRuntimeDatabaseTablePrefix),
		agent.WithToolsRegistry(toolsRegistry),
	}
	if deps.SkillsEnabled {
		skillSet, skillsErr := skills.New(
			deps.SkillsPaths,
			skills.WithLogger(deps.RootLogger),
			skills.WithMaxSkillBytes(deps.SkillsMaxSkillBytes),
			skills.WithMaxCatalogEntries(deps.SkillsMaxCatalogEntries),
		)
		if skillsErr != nil {
			return nil, fmt.Errorf("build skills catalog: %w", skillsErr)
		}
		skillSet.RegisterTools(toolsRegistry)
		runnerOpts = append(runnerOpts, agent.WithSystemPromptFragments(skillSet.BuildSystemPromptFragments()...))
	}

	runner, err := agent.NewRunner(
		agent.RunnerArgs{
			ProvidersConfigService: services.providersConfigSvc,
			AgentProfilesService:   services.agentProfilesSvc,
		},
		runnerOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("create agent runner: %w", err)
	}

	if deps.AgentRuntimeStorageType == storageTypeDatabase && deps.AgentRuntimeDatabaseAutoMigrate {
		if err = autoMigrateRuntimeServices(
			runner,
			services.agentProfilesSvc,
		); err != nil {
			return nil, err
		}
	}
	if deps.DataLayerDatabaseAutoMigrate {
		if err = autoMigrateDataLayerStore(deps.DataStore); err != nil {
			return nil, err
		}
	}

	httpHandler, err := httpapi.NewHandler(httpapi.HandlerArgs{
		Runner:                 runner,
		ProvidersConfigService: services.providersConfigSvc,
		AgentProfilesService:   services.agentProfilesSvc,
		ModelsLister:           runner.ModelsLocator(),
	}, httpapi.WithLogger(deps.RootLogger))
	if err != nil {
		return nil, fmt.Errorf("create http handler: %w", err)
	}

	return &Runtime{
		Runner:               runner,
		HTTPHandler:          httpHandler,
		ToolsRegistry:        toolsRegistry,
		DataStore:            deps.DataStore,
		DataIngestionService: deps.DataIngestionService,
		DataReadService:      deps.DataReadService,
	}, nil
}
