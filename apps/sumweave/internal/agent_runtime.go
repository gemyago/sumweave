package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gemyago/sumweave/runtime/agent"
	"github.com/gemyago/sumweave/runtime/httpapi"
	"github.com/gemyago/sumweave/tools/skills"
	"github.com/gemyago/sumweave/tools/workspacefs"
)

const storageTypeDatabase = "database"

type RuntimeDeps struct {
	RootLogger                      *slog.Logger
	DataDir                         string
	PlatformAgentsPath              string
	ExecEnabled                     bool
	ExecMaxOutputBytes              int64
	ExecDefaultTimeout              time.Duration
	ExecMaxConcurrentJobs           int
	AgentRuntimeStorageType         string
	AgentRuntimeDatabaseDSN         string
	AgentRuntimeDatabaseTablePrefix string
	SkillsEnabled                   bool
	SkillsPaths                     []string
	SkillsMaxSkillBytes             int
	SkillsMaxCatalogEntries         int
	ToolsRegistry                   *agent.ToolsRegistry
}

type Runtime struct {
	Runner        *agent.Runner
	HTTPHandler   http.Handler
	ToolsRegistry *agent.ToolsRegistry
}

type runtimeServices struct {
	providersConfigSvc agent.ProvidersConfigService
	agentProfilesSvc   agent.AgentProfilesService
}

func newProvidersConfigService(deps RuntimeDeps) (agent.ProvidersConfigService, error) { //nolint:ireturn
	if deps.AgentRuntimeStorageType == storageTypeDatabase {
		if deps.AgentRuntimeDatabaseDSN == "" {
			return nil, errors.New("agent runtime database dsn is required")
		}
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
		if deps.AgentRuntimeDatabaseDSN == "" {
			return nil, errors.New("agent runtime database dsn is required")
		}
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
	providersConfigSvc, err := newProvidersConfigService(deps)
	if err != nil {
		return nil, err
	}
	agentProfilesSvc, err := newAgentProfilesService(deps)
	if err != nil {
		return nil, err
	}
	return &runtimeServices{providersConfigSvc: providersConfigSvc, agentProfilesSvc: agentProfilesSvc}, nil
}

func workspacefsRegisterOptions(deps RuntimeDeps) ([]workspacefs.RegisterToolsOpt, error) {
	agentTempDir := filepath.Join(deps.DataDir, "agent-temp")
	if err := os.MkdirAll(agentTempDir, 0o700); err != nil {
		return nil, fmt.Errorf("create workspacefs agent temp directory: %w", err)
	}
	opts := []workspacefs.RegisterToolsOpt{workspacefs.WithWorkspaces([]workspacefs.WorkspaceConfig{
		{Identifier: "agent-temp", Description: "Agent can store temporary files here", Path: agentTempDir},
		{
			Identifier:  "platform-agents",
			Description: "Bundled platform-agent docs and skills live here",
			Path:        deps.PlatformAgentsPath,
		},
	}), workspacefs.WithLogger(deps.RootLogger)}
	if deps.ExecEnabled {
		opts = append(
			opts,
			workspacefs.WithExec(
				workspacefs.ExecOptions{
					MaxOutputBytes:    deps.ExecMaxOutputBytes,
					DefaultTimeout:    deps.ExecDefaultTimeout,
					MaxConcurrentJobs: deps.ExecMaxConcurrentJobs,
				},
			),
		)
	}
	return opts, nil
}

func buildRunnerOpts(deps RuntimeDeps, toolsRegistry *agent.ToolsRegistry) ([]agent.RunnerOpt, error) {
	storageOpt := agent.WithFileSystemStorage(deps.DataDir)
	if deps.AgentRuntimeStorageType == storageTypeDatabase {
		storageOpt = agent.WithDatabaseStorage(deps.AgentRuntimeDatabaseDSN)
	}
	opts := []agent.RunnerOpt{
		agent.WithLogger(deps.RootLogger),
		storageOpt,
		agent.WithDatabaseTablePrefix(deps.AgentRuntimeDatabaseTablePrefix),
		agent.WithToolsRegistry(toolsRegistry),
	}
	if !deps.SkillsEnabled {
		return opts, nil
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
	return append(opts, agent.WithSystemPromptFragments(skillSet.BuildSystemPromptFragments()...)), nil
}

func newRuntime(deps RuntimeDeps) (*Runtime, error) {
	opts, err := workspacefsRegisterOptions(deps)
	if err != nil {
		return nil, err
	}
	if err = workspacefs.RegisterTools(deps.ToolsRegistry, opts...); err != nil {
		return nil, fmt.Errorf("register workspacefs tools: %w", err)
	}
	services, err := newRuntimeServices(deps)
	if err != nil {
		return nil, err
	}
	runnerOpts, err := buildRunnerOpts(deps, deps.ToolsRegistry)
	if err != nil {
		return nil, err
	}
	runner, err := agent.NewRunner(
		agent.RunnerArgs{
			ProvidersConfigService: services.providersConfigSvc,
			AgentProfilesService:   services.agentProfilesSvc,
		},
		runnerOpts...)
	if err != nil {
		return nil, fmt.Errorf("create agent runner: %w", err)
	}
	handler, err := httpapi.NewHandler(
		httpapi.HandlerArgs{
			Runner:                 runner,
			ProvidersConfigService: services.providersConfigSvc,
			AgentProfilesService:   services.agentProfilesSvc,
			ModelsLister:           runner.ModelsLocator(),
		},
		httpapi.WithLogger(deps.RootLogger),
	)
	if err != nil {
		return nil, fmt.Errorf("create agent http handler: %w", err)
	}
	return &Runtime{Runner: runner, HTTPHandler: handler, ToolsRegistry: deps.ToolsRegistry}, nil
}

// NewRuntime constructs the agent runtime for an explicit application root.
func NewRuntime(deps RuntimeDeps) (*Runtime, error) {
	return newRuntime(deps)
}
