//go:build postgres_test

package internal

import (
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationDatabaseRuntimeComposition(t *testing.T) {
	values, err := config.LoadValues(config.ValuesLoadInput{Environment: "test"})
	require.NoError(t, err)
	rootConfig, err := values.HTTPRoot("test")
	require.NoError(t, err)

	deps := RuntimeDeps{
		RootLogger:                      telemetry.RootTestLogger(),
		DataDir:                         t.TempDir(),
		PlatformAgentsPath:              t.TempDir(),
		AgentRuntimeStorageType:         rootConfig.AgentRuntime.Storage.Type,
		AgentRuntimeDatabaseDSN:         rootConfig.AgentRuntime.Database.DSN,
		AgentRuntimeDatabaseTablePrefix: rootConfig.AgentRuntime.Database.TablePrefix,
		SkillsMaxSkillBytes:             4096,
		SkillsMaxCatalogEntries:         10,
		ToolsRegistry:                   agent.NewToolsRegistry(),
	}

	runtime, err := newRuntime(deps)
	require.NoError(t, err)
	assert.NotNil(t, runtime.Runner)
	services, err := newRuntimeServices(deps)
	require.NoError(t, err)
	assert.NotNil(t, services.agentProfilesSvc)
}
