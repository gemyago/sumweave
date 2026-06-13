## Why

Signal Foundry now has canonical `data` and deterministic `analytics`, but the critical path still lacks the `strategy` slice that turns reusable analytics into candidate trading actions. Adding the strategy layer now gives future governor and execution work a stable deterministic contract without pulling AI, venue mechanics, or ad hoc decision logic into the critical path.

## What Changes

- Introduce a new deterministic `strategy-layer` capability between `analytics-layer` and future `governor` work.
- Add shared domain concepts only for cross-slice strategy identity and candidate action records that downstream deterministic slices can consume.
- Add a runtime strategy service that keeps evaluation request types and evaluation-only parameter wrappers in `runtime/strategy`, consumes canonical analytics inputs, and evaluates rule-based strategy logic with stable ordering and explicit event timing semantics.
- Start with an initial deterministic moving-average crossover strategy built from existing candle analytics outputs.
- Keep strategy evaluation independent from AI-assisted research, venue HTTP clients, order placement, and persistence concerns.
- Do not require strategy storage, backfill jobs, backend DI wiring, or new public HTTP routes for the initial slice.

## Capabilities

### New Capabilities

- `strategy-layer`: Deterministic strategy evaluation that turns canonical analytics inputs into reusable candidate actions for downstream governor/execution work.

### Modified Capabilities

- None.

## Impact

- Affected code: narrowly scoped `runtime/domain` additions, a new `runtime/strategy` package, and focused runtime tests.
- APIs: no external HTTP API changes are required for the initial layer.
- Dependencies: no new third-party service, AI model, or live venue dependency is required.
- Systems: strategy will consume existing deterministic analytics behavior and preserve the product path `Data -> Analytics -> Strategy -> Governor -> Execution`.
- Deferred: `apps/signal-foundry` wiring remains out of scope unless implementation introduces a current backend consumer that genuinely needs the strategy service.
