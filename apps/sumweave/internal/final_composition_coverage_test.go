package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/agent"
	"github.com/jaswdr/faker/v2"
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
