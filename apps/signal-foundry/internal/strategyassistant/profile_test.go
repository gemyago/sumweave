package strategyassistant

import (
	"testing"

	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategyAssistantProfileCreateParams(t *testing.T) {
	t.Run("builds regular profile guidance with workflow and safety boundaries", func(t *testing.T) {
		profile := ProfileCreateParams("provider/model")

		require.Equal(t, StrategyAssistantProfileName, profile.Name)
		assert.Equal(t, "Strategy assistant", profile.DisplayName)
		assert.Equal(t, agent.ExecutionModeRegular, profile.ExecutionSettings.ModeOrDefault())
		assert.Equal(t, "provider/model", profile.ExecutionSettings.DefaultModel)
		assert.Contains(t, profile.Instructions, "Discover data scope first")
		assert.Contains(t, profile.Instructions, "validate")
		assert.Contains(t, profile.Instructions, "Save immutable versions")
		assert.Contains(t, profile.Instructions, "Evaluate saved ready versions")
		assert.Contains(t, profile.Instructions, "Critique reports and evidence")
		assert.Contains(t, profile.Instructions, "No live trading")
		assert.Contains(t, profile.Instructions, "Do not claim production readiness")
		assert.Contains(t, profile.Instructions, "Do not bypass validation")
		assert.Contains(t, profile.ToolRefs, toolNameEvaluationRunBacktest)
		assert.Contains(t, profile.ToolRefs, "skills_list")
		assert.Contains(t, profile.ToolRefs, "skills_read")
	})

	t.Run("falls back to deterministic seed model when omitted", func(t *testing.T) {
		profile := ProfileCreateParams(" ")

		assert.Equal(t, StrategyAssistantProfileSeedDefaultModel, profile.ExecutionSettings.DefaultModel)
	})
}
