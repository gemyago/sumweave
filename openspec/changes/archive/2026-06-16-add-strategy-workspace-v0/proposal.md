## Why

Signal Foundry has deterministic runtime foundations for strategy artifacts, governor policy, durable backtests, audit, and paper execution, but operators do not yet have a product workflow to create strategy candidates, run evaluations, and inspect evidence. This slice turns those foundations into a minimal human strategy workspace while preserving the architecture rule that AI stays outside the critical runtime path.

## What Changes

- Add a protected strategy workspace API for validating v0 strategy definitions, saving immutable strategy versions, listing/opening versions, duplicating versions, and exposing demo strategies.
- Add a small product-facing strategy registry for display names, versions, status, artifact hash, notes, source type, parent linkage, and created/updated metadata while keeping `StrategyArtifact` immutable.
- Seed or expose at least three demo moving-average-crossover strategies that validate through the same artifact path and are clearly labeled as demos, not recommendations.
- Add a protected evaluation API that resolves strategy version to artifact, derives runtime inputs from the artifact, uses a default safe governor policy when omitted, runs `DurableBacktestFlow`, and exposes evaluation history/detail evidence.
- Add protected UI routes for strategies, strategy editor/version detail, evaluations history, and evaluation detail using constrained forms and table-first report/evidence views.
- Add a compact evaluation detail payload suitable for future AI critique, without adding AI generation, AI runtime decisions, live trading controls, arbitrary executable strategy code, or autonomous promotion.

## Capabilities

### New Capabilities

- `strategy-workspace`: Human operator workflow for constrained strategy version management, demo strategies, deterministic evaluation runs, evaluation history/detail, and AI-ready audit metadata.

### Modified Capabilities

- None. Existing `strategy-dsl`, `strategy-artifacts`, `paper-backtest-flow`, `backtest-evaluation`, `governor-policy-artifacts`, `audit-records`, and `execution-layer` capabilities are reused rather than behaviorally changed.

## Impact

- Affects `runtime/strategy` for a small version registry/service backed by explicit GORM models and idempotent demo version creation.
- Affects runtime/app orchestration wiring for an evaluation application service that wraps existing `flows.DurableBacktestFlow` and read models for run history/detail evidence.
- Affects `apps/signal-foundry/internal/api/http/v1routes.yaml`, generated `v1routes/`, protected route registration, controllers, request/response DTO mapping, and backend tests.
- Affects `apps/signal-ui` routing/nav, strategy pages, evaluation pages, focused API wrappers, component tests, and `ui-wireframe.md` route/state documentation.
- No new external runtime dependency, AI provider dependency, live venue credential, wallet signing path, or trading endpoint is introduced.
