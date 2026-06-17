package strategyassistant

import (
	"strings"

	"github.com/gemyago/signal-foundry/runtime/agent"
)

const (
	StrategyAssistantProfileName             = "strategy-assistant"
	StrategyAssistantProfileSeedDefaultModel = "signal-foundry/override-required"
)

func ProfileCreateParams(defaultModel string) agent.CreateAgentProfileParams {
	model := strings.TrimSpace(defaultModel)
	if model == "" {
		model = StrategyAssistantProfileSeedDefaultModel
	}

	return agent.CreateAgentProfileParams{
		Name:        StrategyAssistantProfileName,
		DisplayName: "Strategy assistant",
		Role:        "Internal alpha strategy research and evaluation assistant",
		Instructions: strings.TrimSpace(`Operate as a bounded internal alpha strategy assistant.

Follow this loop unless the operator explicitly asks for something narrower:
1. Discover data scope first with candle availability and bounded candle/evidence reads.
2. Draft or refine one strategy definition, then validate before proposing a save.
3. Save immutable versions only when validation is clean and the operator wants a persisted version.
4. Evaluate saved ready versions with bounded backtests.
5. Critique reports and evidence before making any conclusion.

Safety boundaries:
- No live trading, order placement, wallet actions, or external execution.
- Do not claim production readiness, profitability, or risk approval from one backtest.
- Do not bypass validation, saved-version requirements, governor decisions, or missing-data failures.
- If evidence is missing or truncated, say so clearly and ask for the next bounded read.`),
		ToolRefs: []string{
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
			"skills_list",
			"skills_read",
			"workspacefs_list_workspaces",
			"workspacefs_list_directory",
			"workspacefs_directory_tree",
			"workspacefs_search_files",
			"workspacefs_read_text_file",
			"workspacefs_read_multiple_files",
			"workspacefs_write_file",
			"workspacefs_edit_file",
		},
		ExecutionSettings: agent.ExecutionSettings{
			DefaultModel: model,
		},
	}
}
