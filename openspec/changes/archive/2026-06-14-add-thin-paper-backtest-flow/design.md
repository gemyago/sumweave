## Context

`docs/ARCHITECTURE.md` defines the deterministic path as `Data -> Analytics -> Strategy -> Governor -> Execution` and states that cross-slice orchestration should stay thin and explicit in a small runtime area such as `runs/` or `flows/`. `docs/IMPLEMENTATION_STATUS.md` shows that `runtime/data`, `runtime/analytics`, `runtime/strategy`, `runtime/governor`, and `runtime/execution` are implemented as deterministic on-demand foundations, while backtesting and paper trading remain missing MVP capabilities.

This change plans only the first narrow proof path: one deterministic paper backtest flow that composes the existing slice services and returns an in-memory result. It assumes the Notion task title is authoritative because the page is inaccessible here.

## Goals / Non-Goals

**Goals:**
- Add a small runtime orchestration package, preferably `runtime/flows`, for one paper backtest path.
- Preserve the documented slice order and keep orchestration logic outside `execution`.
- Use replay candle data as the deterministic data source for analytics and paper fill prices.
- Use the existing moving-average crossover strategy, existing governor policy model, and existing local execution primitives.
- Return deterministic in-memory outputs suitable for tests and later product wiring.

**Non-Goals:**
- No backend dependency injection, HTTP routes, OpenAPI changes, or UI work.
- No live venue trading, credentials, signing, private endpoints, or live order adapters.
- No persisted backtest runs, execution ledger, audit trace store, migrations, or scheduled jobs.
- No new strategy kinds, governor policy product object, strategy artifact store, or analytics persistence.
- No AI/agent calls in the deterministic path.

## Decisions

### Use `runtime/flows` for orchestration

Place the new coordinator in a thin package such as `runtime/flows`, with names scoped to paper backtesting. This follows the architecture note that orchestration belongs in a small runtime area and prevents `execution` from becoming the owner of upstream workflow.

Alternative considered: put the flow under `runtime/execution`. This was rejected because execution owns approval-admitted command/order/fill behavior after governor approval, not data, analytics, or strategy orchestration.

Alternative considered: put the flow in `apps/signal-foundry`. This was rejected for the initial path because backend wiring is explicitly out of scope and the flow should first be testable as a core runtime capability.

### Keep the flow surface explicit and deterministic

The flow request should carry explicit run identity, instrument, timeframe, half-open time range, moving-average crossover parameters, governor policy, and fixed positive execution quantity. The result should include the canonical strategy evaluation, ordered governor decisions, and ordered paper execution records, including the deterministic command/order/fill/reconciliation identifiers associated with each approved decision. No defaults should depend on wall-clock time, random IDs, environment, network, or AI.

Alternative considered: infer run identifiers and quantities inside the flow. This was rejected because it would make repeated runs harder to compare and would blur strategy output with execution sizing.

### Compose existing slice services instead of duplicating slice logic

The flow should call existing slice boundaries in order. The data replay reader should feed analytics, analytics should feed strategy, strategy actions should feed governor, and approved governor decisions should feed execution. The orchestration package may define consumer-side interfaces for the services it calls, and constructors should accept dependencies as interfaces and return concrete structs.

Alternative considered: reimplement moving averages, crossover logic, policy checks, or command identity generation in the flow. This was rejected because it would duplicate slice behavior and weaken the value of the slice contracts.

### Use replay candle close prices for paper fills

For the initial paper path, every approved decision should become one local command, order, fill, and reconciliation. The fill price should come from the close price of the replay candle whose end time equals the approved decision time. Missing fill-price data should fail the run instead of inventing prices or silently skipping approved decisions.

Alternative considered: create commands only and skip orders/fills. This was rejected because it would not exercise the local paper execution lifecycle enough to qualify as a paper backtest path.

Alternative considered: inject a generic fill simulator. This was rejected for v0 because it widens the design before there is more than one paper fill rule.

### Derive every paper execution identity from deterministic inputs

The flow should enumerate approved governor decisions in the exact order returned by the governor and assign each approved decision a stable zero-based approved ordinal. That ordinal must remain the source order for the result's paper execution records; the flow should not iterate maps or otherwise reorder records.

For each approved decision, the flow should use only deterministic inputs when creating execution records:

- The execution event time should be the approved decision time, which also identifies the replay candle close used for the fill price.
- The command record should be created through the execution slice with the approved decision, fixed quantity, and deterministic event time; the existing execution service derives `CommandID` deterministically from those inputs.
- The local client order ID should be a canonical string derived from the stable run identity plus the approved ordinal, for example `paper-client:<run-id>:<approved-ordinal>`.
- The order record should be created through the execution slice with the deterministic command, venue, local client order ID, fixed quantity, and event time; the existing execution service derives `OrderID` deterministically from those inputs.
- The fill ID should be a canonical string derived from the stable run identity plus the approved ordinal, for example `paper-fill:<run-id>:<approved-ordinal>`; the fill record should use that fill ID, the deterministic order, fixed quantity, replay candle close price, and event time.
- The reconciliation should be created through the execution slice from the deterministic order and fill. Because the current domain reconciliation record has no standalone ID, the flow result should carry a flow-local reconciliation ID next to the reconciliation record, derived from the same run identity and approved ordinal, for example `paper-reconciliation:<run-id>:<approved-ordinal>`.

This guarantees repeated runs over the same request and replay data return identical command IDs, order IDs, client order IDs, fill IDs, flow-local reconciliation IDs, record contents, and record order. The flow should not use random IDs, current time, environment state, network state, or hidden mutable counters.

Alternative considered: make callers provide every command, order, fill, and reconciliation identifier. This was rejected because it makes the first path noisier without adding product value and duplicates deterministic identity behavior already owned by the execution slice for commands and orders.

## Risks / Trade-offs

- [Risk] The v0 fill rule is simplistic and may not model realistic fills. → Mitigation: document it as a deterministic paper rule and keep live trading and richer simulation out of scope.
- [Risk] Reading replay candles for both analytics and fill pricing may duplicate reads. → Mitigation: accept the simple design for v0; optimize later only if profiling or larger backtests require it.
- [Risk] Exporting a new runtime package expands public surface. → Mitigation: keep names paper-backtest-specific, prefer consumer-defined interfaces, and avoid adding shared domain records unless needed later.
- [Risk] The inaccessible Notion task might imply persistence or backend wiring. → Mitigation: explicitly scope this proposal to the thin runtime flow requested by the task title and local architecture docs.

## Migration Plan

No data migration or rollout sequencing is required. The implementation adds a new runtime package and tests. Rollback is removing the new package and its OpenSpec change before archive, or reverting the resulting commit after implementation.

## Open Questions

- Should the package be named `flows` or `runs`? The proposal recommends `runtime/flows` because the task title uses “flow,” but either name is acceptable if planning review prefers the other.
- Should v0 support short fills differently from long fills? The proposal assumes both use the same replay candle close price and positive fixed quantity because the existing execution domain does not model side-specific fill mechanics.
