package strategyassistant

import (
	"testing"

	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategyAssistantProfileCreateParams(t *testing.T) {
	t.Run("builds regular profile guidance with explicit operating workflow", func(t *testing.T) {
		profile := ProfileCreateParams("provider/model")

		require.Equal(t, StrategyAssistantProfileName, profile.Name)
		assert.Equal(t, "Strategy assistant", profile.DisplayName)
		assert.Equal(t, agent.ExecutionModeRegular, profile.ExecutionSettings.ModeOrDefault())
		assert.Equal(t, "provider/model", profile.ExecutionSettings.DefaultModel)
		assert.Contains(t, profile.Instructions, "Signal Foundry strategy assistant")
		assert.Contains(t, profile.Instructions, "Product tools are authoritative")
		assert.Contains(t, profile.Instructions, "skills_list")
		assert.Contains(t, profile.Instructions, "strategy-dsl-v0")
		assert.Contains(t, profile.Instructions, "replay-data-unavailable")
		assert.Contains(t, profile.Instructions, "Do not duplicate an ingestion job")
		assert.Contains(t, profile.ToolRefs, toolNameJobsStartHistoricalDataBackfill)
		assert.Contains(t, profile.ToolRefs, toolNameJobsList)
		assert.Contains(t, profile.ToolRefs, toolNameJobsGet)
		assert.Contains(t, profile.ToolRefs, toolNameEvaluationRunBacktest)
		assert.Contains(t, profile.ToolRefs, "skills_list")
		assert.Contains(t, profile.ToolRefs, "skills_read")
		assert.NotContains(t, profile.ToolRefs, "workspacefs_write_file")
		assert.NotContains(t, profile.ToolRefs, "workspacefs_edit_file")
	})

	t.Run("falls back to deterministic seed model when omitted", func(t *testing.T) {
		profile := ProfileCreateParams(" ")

		assert.Equal(t, StrategyAssistantProfileSeedDefaultModel, profile.ExecutionSettings.DefaultModel)
	})
}
