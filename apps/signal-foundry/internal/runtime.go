package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/strategyassistant"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"github.com/gemyago/signal-foundry/tools/skills"
	"github.com/gemyago/signal-foundry/tools/workspacefs"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/dig"
)

const (
	storageTypeDatabase     = "database"
	postgresUndefinedTable  = "42P01"
	agentProfilesTableToken = "agent_profiles"
	platformAgentsWorkspace = "platform-agents"
)

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

	// Data-layer persistence (canonical instruments, candles, trades)
	DataLayerDatabaseDSN             string `name:"config.dataLayer.database.dsn"`
	DataLayerDatabaseTablePrefix     string `name:"config.dataLayer.database.tablePrefix"`
	DataLayerRawPayloadBlobStorePath string `name:"config.dataLayer.rawPayloadBlobStore.path"`

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
	DataLineageService   *data.LineageService
	JobsService          *jobspkg.Service
	StrategyWorkspace    *app.StrategyWorkspaceService
	EvaluationWorkspace  *app.EvaluationWorkspaceService
}

type Runtime struct {
	Runner               *agent.Runner
	HTTPHandler          http.Handler
	ToolsRegistry        *agent.ToolsRegistry
	DataStore            *data.DatabaseStore
	DataIngestionService *data.IngestionService
	DataReadService      *data.ReadService
	DataLineageService   *data.LineageService
	VenueIngestionFlow   *venueedge.IngestionFlow
	HyperliquidRecorder  venueedge.HyperliquidRawEvidenceRecorder
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
		newDataRawPayloadBlobStore,
		newDataIngestionService,
		newDataReadService,
		newDataLineageService,
		newRuntime,
	)
}

func workspacefsRegisterOptions(deps RuntimeDeps) ([]workspacefs.RegisterToolsOpt, error) {
	agentTempDir := filepath.Join(deps.DataDir, "agent-temp")
	if err := os.MkdirAll(agentTempDir, 0o700); err != nil {
		return nil, fmt.Errorf("create workspacefs agent temp directory: %w", err)
	}
	platformAgentsDir, err := bundledPlatformAgentsDir()
	if err != nil {
		return nil, err
	}

	registerOpts := []workspacefs.RegisterToolsOpt{
		workspacefs.WithWorkspaces([]workspacefs.WorkspaceConfig{
			{
				Identifier:  "agent-temp",
				Description: "Agent can store temporary files here",
				Path:        agentTempDir,
			},
			{
				Identifier:  platformAgentsWorkspace,
				Description: "Bundled platform-agent docs and skills live here",
				Path:        platformAgentsDir,
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

	return registerOpts, nil
}

func bundledPlatformAgentsDir() (string, error) {
	_, currentFile, _, ok := goruntime.Caller(0)
	if !ok {
		return "", errors.New("resolve bundled platform-agents directory: caller unavailable")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".platform-agents")), nil
}

func registerStrategyAssistantTools(deps RuntimeDeps, toolsRegistry *agent.ToolsRegistry) error {
	if deps.StrategyWorkspace == nil || deps.EvaluationWorkspace == nil {
		return nil
	}

	if registerErr := strategyassistant.RegisterTools(strategyassistant.RegisterDeps{
		Registry:            toolsRegistry,
		DataRead:            deps.DataReadService,
		DataLineage:         deps.DataLineageService,
		JobsService:         deps.JobsService,
		StrategyWorkspace:   deps.StrategyWorkspace,
		EvaluationWorkspace: deps.EvaluationWorkspace,
	}); registerErr != nil {
		return fmt.Errorf("register strategy assistant tools: %w", registerErr)
	}

	return nil
}

func buildRunnerOpts(deps RuntimeDeps, toolsRegistry *agent.ToolsRegistry) ([]agent.RunnerOpt, error) {
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
	if !deps.SkillsEnabled {
		return runnerOpts, nil
	}

	skillSet, err := skills.New(
		deps.SkillsPaths,
		skills.WithLogger(deps.RootLogger),
		skills.WithMaxSkillBytes(deps.SkillsMaxSkillBytes),
		skills.WithMaxCatalogEntries(deps.SkillsMaxCatalogEntries),
	)
	if err != nil {
		return nil, fmt.Errorf("build skills catalog: %w", err)
	}

	skillSet.RegisterTools(toolsRegistry)
	runnerOpts = append(runnerOpts, agent.WithSystemPromptFragments(skillSet.BuildSystemPromptFragments()...))

	return runnerOpts, nil
}

func newRuntime(deps RuntimeDeps) (*Runtime, error) {
	toolsRegistry := deps.ToolsRegistry
	registerOpts, err := workspacefsRegisterOptions(deps)
	if err != nil {
		return nil, err
	}

	if registerErr := workspacefs.RegisterTools(toolsRegistry, registerOpts...); registerErr != nil {
		return nil, fmt.Errorf("register workspacefs tools: %w", registerErr)
	}
	if err = registerStrategyAssistantTools(deps, toolsRegistry); err != nil {
		return nil, err
	}

	services, err := newRuntimeServices(deps)
	if err != nil {
		return nil, err
	}

	venueIngestionFlow, err := newVenueIngestionFlow(deps.DataIngestionService, deps.DataLineageService)
	if err != nil {
		return nil, err
	}

	hyperliquidRecorder, err := newHyperliquidRawEvidenceRecorder(deps.DataLineageService)
	if err != nil {
		return nil, err
	}

	runnerOpts, err := buildRunnerOpts(deps, toolsRegistry)
	if err != nil {
		return nil, err
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

	if err = ensureStrategyAssistantProfile(context.Background(), services.agentProfilesSvc); err != nil {
		return nil, err
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
		DataLineageService:   deps.DataLineageService,
		VenueIngestionFlow:   venueIngestionFlow,
		HyperliquidRecorder:  hyperliquidRecorder,
	}, nil
}

func ensureStrategyAssistantProfile(ctx context.Context, svc agent.AgentProfilesService) error {
	_, err := svc.Get(ctx, strategyassistant.StrategyAssistantProfileName)
	if err == nil {
		return nil
	}
	if isMissingAgentProfilesSchemaError(err) {
		return nil
	}
	if !errors.Is(err, agent.ErrAgentProfileNotFound) {
		return fmt.Errorf("get strategy assistant profile: %w", err)
	}

	_, err = svc.Create(ctx, strategyassistant.ProfileCreateParams(
		strategyassistant.StrategyAssistantProfileSeedDefaultModel,
	))
	if err == nil ||
		errors.Is(err, agent.ErrAgentProfileNameConflict) ||
		isMissingAgentProfilesSchemaError(err) {
		return nil
	}

	return fmt.Errorf("create strategy assistant profile: %w", err)
}

func isMissingAgentProfilesSchemaError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	if !strings.Contains(message, agentProfilesTableToken) {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == postgresUndefinedTable
	}

	return strings.Contains(message, strings.ToLower(postgresUndefinedTable)) ||
		strings.Contains(message, "no such table") ||
		(strings.Contains(message, "relation") && strings.Contains(message, "does not exist"))
}
