package strategyassistant

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterTools(t *testing.T) {
	t.Run("registers expected internal alpha tool contracts once", func(t *testing.T) {
		registry := agent.NewToolsRegistry()

		err := RegisterTools(RegisterDeps{Registry: registry})
		require.NoError(t, err)

		registered := registeredToolDefinitions(t, registry)
		names := make([]string, 0, len(registered))
		seenNames := make(map[string]struct{}, len(registered))
		for _, toolDef := range registered {
			names = append(names, toolDef.Name)
			seenNames[toolDef.Name] = struct{}{}

			lowerDescription := strings.ToLower(toolDef.Description)
			assert.Contains(t, lowerDescription, "internal alpha")
			assert.Contains(t, lowerDescription, "bounded")
		}

		expectedNames := []string{
			toolNameDataListCandleAvailability,
			toolNameDataGetCandles,
			toolNameDataGetCandleEvidence,
			toolNameStrategyListVersions,
			toolNameStrategyGetVersion,
			toolNameStrategyValidateDefinition,
			toolNameStrategyDuplicateVersion,
			toolNameStrategyCreateVersion,
			toolNameEvaluationRunBacktest,
			toolNameEvaluationListBacktests,
			toolNameEvaluationGetDetail,
			toolNameEvaluationGetReport,
			toolNameEvaluationGetEvidence,
		}
		assert.ElementsMatch(t, expectedNames, names)
		assert.Len(t, names, len(expectedNames))
		assert.Len(t, seenNames, len(expectedNames))

		forbiddenNameFragments := []string{"live", "trade", "manual", "order", "wallet", "sql"}
		for _, name := range names {
			lowerName := strings.ToLower(name)
			for _, fragment := range forbiddenNameFragments {
				assert.NotContains(t, lowerName, fragment)
			}
		}
	})

	t.Run("requires registry", func(t *testing.T) {
		err := RegisterTools(RegisterDeps{})
		require.Error(t, err)
		assert.Equal(t, "tools registry is required", err.Error())
	})
}

type reflectedToolDefinition struct {
	Name        string
	Description string
}

func registeredToolDefinitions(t *testing.T, registry *agent.ToolsRegistry) []reflectedToolDefinition {
	t.Helper()

	toolsField := reflect.ValueOf(registry).Elem().FieldByName("tools")
	require.True(t, toolsField.IsValid())
	toolsField = reflect.NewAt(toolsField.Type(), unsafe.Pointer(toolsField.UnsafeAddr())).Elem()

	registered := make([]reflectedToolDefinition, 0, toolsField.Len())
	for index := range toolsField.Len() {
		toolValue := reflect.ValueOf(toolsField.Index(index).Interface())
		registered = append(registered, reflectedToolDefinition{
			Name:        toolValue.FieldByName("Name").String(),
			Description: toolValue.FieldByName("Description").String(),
		})
	}

	return registered
}
