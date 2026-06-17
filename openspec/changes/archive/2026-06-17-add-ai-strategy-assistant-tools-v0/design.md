## Context

The architecture source of truth allows AI-assisted research, drafting, critique, explanation, and summaries, while the deterministic path remains `Data -> Analytics -> Strategy -> Governor -> Execution`. Current code already includes the primitives this feature should reuse: runtime `agent.ToolDef` / `ToolsRegistry`, app-owned data browser services, strategy workspace services with strict Strategy DSL v0 canonicalization, durable evaluation/backtest services, profile storage, and optional skills discovery.

This change should expose a narrow internal-alpha tool layer over existing product services. It should not create a second strategy API, raw database interface, trading command surface, or alternative deterministic path.

## Goals / Non-Goals

**Goals:**

- Let a strategy-assistant chat profile discover available normalized candles, fetch bounded candle slices, and inspect candle-linked evidence metadata.
- Let the assistant validate constrained Strategy DSL v0 definitions, list/open saved versions, duplicate versions as editable candidates, and save valid immutable versions directly for alpha.
- Let the assistant run deterministic backtests from saved ready strategy versions and read bounded history/detail/report/evidence views, including honest failed-run reasons when local data is missing.
- Register tools globally for alpha through existing `ToolsRegistry`; defer profile-level tool authorization.
- Provide reusable strategy workflow skills/prompts and default profile guidance so operators can start from the existing chat UI.

**Non-Goals:**

- No live trading, manual order placement, wallet/signing, private venue, credential, raw SQL/database, or arbitrary executable strategy-code tool.
- No bypass of StrategyArtifact validation/canonicalization, saved-artifact runtime input derivation, governor policy, or deterministic evaluation ordering.
- No operator confirmation gates, persisted AI draft approval workflow, per-tool audit tables, async evaluation queue, skill management UI, or richer strategy comparison workspace in this slice.
- No requirement to add a new public runtime package contract beyond existing agent tool registration primitives.

## Decisions

1. Keep the tool pack in the app boundary over product services.

   Implement a small app-owned registration component, for example under `apps/signal-foundry/internal/strategyassistant`, that accepts the existing `agent.ToolsRegistry`, data read/lineage services, `StrategyWorkspaceService`, `EvaluationWorkspaceService`, and logger. The component should register explicit `agent.NewToolDef` handlers with JSON-schema-friendly request/response structs. Runtime `agent` should remain the generic tool registry/runner layer, not a dependency on product strategy/evaluation services.

2. Call services directly, not HTTP or SQL.

   Tool handlers should call app/runtime services directly to preserve existing validation, error mapping, artifact canonicalization, evaluation derivation, governor checks, and persistence behavior. They must not round-trip through `/api/v1` and must not expose raw GORM, SQL, table names, or arbitrary query capabilities to the model.

3. Register globally for internal alpha.

   The v0 runner can expose the strategy assistant tools to all regular profiles once the app has registered them. This intentionally defers `toolRefs` enforcement and fine-grained permissions. ACP/stdio profiles should not gain new behavior beyond whatever the existing runner already routes.

4. Bound outputs for model usability and safety.

   Data and evidence tools should use existing pagination/range validation and add AI-friendly caps where needed. Candle reads should require an exact venue, symbol, asset class, timeframe, and half-open UTC range, then return at most the tool cap with a truncation/next-step hint rather than flooding chat. Evidence/report tools should return compact rows and counts, not raw payload bodies or invented metrics.

   All tools should share a small machine-actionable result convention. Recoverable domain failures should be returned as safe structured results, not leaked service internals: `error.code`, `error.message`, optional `error.fieldErrors[]`, and optional safe `error.retryable` / `error.details` for the model's next action. Required stable codes include validation/not-found/conflict/data-unavailable/not-ready/unsaved-version/missing-artifact style cases. Bounded responses that omit available rows must include `truncation.isTruncated`, `truncation.limit`, `truncation.returned`, optional total/cursor/range continuation fields, and a concise next-step hint. Tool descriptions and handler tests must document these fields and confirm SQL/table/GORM internals are never exposed.

5. Preserve existing strategy save semantics.

   `sf_strategy_validate_definition` must use the same strict Strategy DSL v0 and StrategyArtifact canonicalization path as the app workspace. `sf_strategy_create_version` may use the existing human save path for alpha unless adding source metadata is already trivial; it must still produce immutable saved versions and must not persist arbitrary AI code or bypass artifact validation.

6. Evaluation tools derive runtime inputs from saved artifacts.

   `sf_evaluation_run_backtest` accepts strategy id/version, explicit UTC range, quantity, optional policy hash, and note. The backend still loads the saved version/artifact, derives instrument/timeframe/kind/parameters from the artifact, applies the explicit or fixed default governor policy, and runs the deterministic durable backtest flow. Missing local data must persist/return a failed run with replay/data-unavailable details rather than fabricating success.

   Evaluation creation must reject anything that is not a persisted ready version with an immutable artifact. Draft/edit candidates, unsaved definitions, saved non-ready versions, and versions whose artifact cannot be loaded must return stable structured errors and must not create or execute an evaluation run.

7. Make profile and skills useful without a workflow UI.

   Provide a `strategy-assistant` profile payload or idempotent seed with instructions that reinforce data discovery, strict validation/save, deterministic evaluation, evidence inspection, and no-live-trading boundaries. Add bundled workflow skills such as `strategy-research-loop`, `backtest-critique`, and `strategy-iteration` under the repo-local skills root so `skills_list` / `skills_read` can support the chat flow when skills are enabled. If config changes are needed for the smoke flow, make them part of implementation rather than a manual blocker.

8. Keep UI work minimal.

   The existing chat route should remain the primary UI. Polish should ensure operators can select or create the `strategy-assistant` profile, observe tool call/result events through the current chat/debug stream, and follow returned strategy/evaluation identifiers or route references. Do not add a new multi-pane strategy assistant workspace in this change.

9. Keep acceptance executable by default.

   The default acceptance path should be an automated smoke/integration check that can run in the local repo setup without a human manually clicking through the flow. It may use seeded fixtures, fakes, or deterministic local data to exercise list data availability, bounded candle reads, validate/save, evaluation creation, report/evidence reads, and skill discovery. Any remaining manual runbook is a conditional fallback deliverable only for coverage that cannot be automated in this slice, not the primary acceptance gate.

## Risks / Trade-offs

- Global alpha registration means non-strategy profiles can see these tools until profile filtering exists; this is accepted for internal alpha and mitigated by no live/raw-SQL tools.
- Synchronous evaluation tool calls may be slow for large ranges; v0 should rely on bounded ranges and compact output, deferring a job queue until measured need.
- Existing app services may expose some data through HTTP DTO mapping only; implementation may need small service-level mapping adapters, but should avoid duplicating validation logic.
- Skills are currently optional; implementation must ensure the default automated alpha smoke path actually discovers the bundled skills or configures the required enablement in-code.

## Migration Plan

1. Add app-domain tool DTOs/handlers and registration tests without changing persistence schemas.
2. Wire tool registration into app runtime construction before `agent.NewRunner` builds tool stubs.
3. Add or seed the strategy-assistant profile guidance and bundled skills; adjust default/local config only if required for the smoke path.
4. Add minimal chat UI/profile polish and route-link references if current UI does not make tool output usable.
5. Rollback removes tool registration/profile/skills while leaving existing strategy/evaluation/data records intact.

## Open Questions

- Whether to seed `strategy-assistant` automatically or ship a documented profile payload depends on the current profile service ergonomics; either is acceptable if the acceptance smoke flow can run without new workflow UI.
- Exact candle/evidence caps should be chosen during implementation based on current response sizes; they must be deterministic, documented in tool descriptions, and tested.
