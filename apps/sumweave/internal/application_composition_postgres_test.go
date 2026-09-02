//go:build postgres_test

package internal

import (
	"os"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
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

	t.Run("application database opens the prepared runtime-role schema and registers cleanup", func(t *testing.T) {
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		hooks := lifecycle.NewTestShutdownHooks()
		database, openErr := NewApplicationSQLDB(dsn, hooks)
		require.NoError(t, openErr)
		require.NotNil(t, database)
		require.NoError(t, hooks.PerformShutdown(t.Context()))
	})
}
