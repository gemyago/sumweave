## Why

Signal Foundry already has protected data browsing, constrained strategy workspace, deterministic evaluation/backtest, runtime chat, tool registry, profiles, and skills primitives. The missing alpha slice is a focused product-domain tool pack that lets a strategy-assistant profile use those existing services from chat to discover data, validate/save strategy versions, run backtests, read evidence, and summarize the next iteration while keeping AI outside the deterministic trading path.

## What Changes

- Add an internal-alpha strategy assistant tool pack registered with the existing runtime `agent.ToolsRegistry` and backed by app/runtime services rather than HTTP loopback or raw SQL.
- Expose bounded data tools for candle availability, normalized candle reads, and candle-linked raw payload evidence metadata.
- Expose strategy tools for version list/get, strict DSL validation, duplicate-as-candidate, and alpha direct version save through the existing workspace path.
- Expose evaluation tools for deterministic backtest create/list/detail/report/evidence using saved strategy versions and existing governor-backed evaluation services.
- Add default strategy-assistant profile guidance or seeding plus bundled strategy workflow skills/prompts discoverable through `skills_list` / `skills_read`.
- Keep fine-grained profile tool filtering, operator confirmation gates, durable AI draft approval, per-tool audit models, async evaluation jobs, richer UI workflow, live trading, and raw database access out of scope.

## Capabilities

### New Capabilities

- `ai-strategy-assistant-tools`: Internal-alpha product-domain agent tools for data browsing, strategy validation/versioning, deterministic evaluation/backtest, compact evidence reading, profile guidance, and workflow skills.

### Modified Capabilities

- None. Existing `historical-data-browser`, `strategy-workspace`, `backtest-evaluation`, `paper-backtest-flow`, and skills support are reused through service calls rather than changing their core semantics.

## Impact

- Affects `apps/signal-foundry/internal` runtime/app wiring to register the tool pack before agent runs build tool stubs.
- Affects app service adapter code around data read/lineage, `StrategyWorkspaceService`, and `EvaluationWorkspaceService` with explicit JSON-schema-friendly tool DTOs and focused tests.
- May affect runtime agent tests only to the extent needed to assert tool registration compatibility; no runtime public contract expansion should be required beyond existing `agent.ToolDef` / `ToolsRegistry` usage.
- Affects repo-local skills content under the configured `.agents/skills` path and profile docs/seed behavior for `strategy-assistant`.
- Affects `apps/signal-ui` minimally for chat/profile polish and tool result visibility/link references; no new complex strategy workflow UI is required.
- No live trading, manual order placement, exchange credential, wallet, autonomous promotion, arbitrary executable strategy code, raw SQL/database tool, or AI call inside deterministic evaluation is introduced.
