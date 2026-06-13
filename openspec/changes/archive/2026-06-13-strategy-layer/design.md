## Context

Signal Foundry's product architecture defines the deterministic path as `Data -> Analytics -> Strategy -> Governor -> Execution`. The runtime now has a canonical `data` slice plus a deterministic `analytics` slice, but there is still no runtime boundary that turns analytics into strategy outputs that governor and execution work can later consume.

The missing piece is a strategy slice that stays deterministic, consumes canonical analytics rather than vendor payloads, and emits reusable candidate actions without leaking order-placement concerns, persistence details, or AI-assisted research into the critical path. The first implementation should fit the current codebase style: small public surface, consumer-defined interfaces, concrete constructors, explicit validation, and focused runtime-only tests.

## Goals / Non-Goals

**Goals:**

- Add a deterministic `runtime/strategy` package that evaluates rule-based strategy logic from canonical analytics inputs.
- Add shared domain records for cross-slice strategy identity and ordered candidate actions so downstream deterministic slices can consume a stable contract.
- Keep service evaluation request types and evaluation-only parameter wrappers in `runtime/strategy`, not in the shared `runtime/domain` kernel.
- Support an initial moving-average crossover strategy built from existing moving-average analytics outputs.
- Preserve the existing runtime time semantics: UTC-normalized values, stable ordering, and explicit `[start, end)` evaluation ranges.
- Make decision-time, input-range, and quality propagation semantics explicit for downstream governor and execution work.

**Non-Goals:**

- No external HTTP API for strategy evaluation in the initial change.
- No strategy persistence, migrations, backfill jobs, or materialized signal tables.
- No AI model involvement, venue-specific behavior, live network access, or order placement side effects.
- No generic strategy DSL, portfolio optimizer, or multi-instrument orchestration framework.
- No `apps/signal-foundry` dependency injection wiring unless a current backend consumer is introduced in the same implementation change.

## Decisions

1. Create a dedicated `runtime/strategy` slice.

   Strategy should sit beside `runtime/analytics`, not inside it. Analytics owns derived market calculations; strategy owns deterministic decision logic built on those calculations. Keeping the slice separate matches `docs/ARCHITECTURE.md` and gives future governor work a clear upstream dependency.

   Alternative considered: add strategy methods directly to `runtime/analytics`. That would be simpler at first, but it blurs the slice boundary and makes later governance/policy integration harder to reason about.

2. Use on-demand evaluation for v0.

   The service should evaluate strategies from analytics results returned for the caller's requested range. This avoids new storage, invalidation, backfill, and provenance complexity before the first strategy contract is proven.

   Alternative considered: persist strategy outputs immediately. That could help with large backtests later, but it introduces schema and synchronization concerns before usage justifies them.

3. Define only cross-slice strategy records in `runtime/domain`.

   Add minimal shared domain concepts for strategy identity and candidate action records. These types should stay independent from GORM, HTTP payloads, venue payloads, AI prompt content, and execution-order details so governor and later UI work can depend on them cleanly.

   Evaluation request types and evaluation-only parameter wrappers belong in `runtime/strategy`, following the existing analytics pattern where slice requests stay with the slice and only reusable identity/output records enter `runtime/domain`.

   Alternative considered: keep all strategy output types inside `runtime/strategy`. That would reduce shared-domain additions, but it would force future consumers to import the implementation package or duplicate cross-slice contract types.

4. Accept an analytics calculation contract in the strategy service.

   The strategy service should define the narrow analytics dependency it needs near the consumer, likely an interface that can calculate candle-derived analytics series for a requested instrument, timeframe, indicator, and range. The constructor should return a concrete `*Service`, following the repo's Go conventions.

   Alternative considered: depend directly on `*analytics.Service`. That would work for simple wiring, but a consumer-defined interface keeps strategy decoupled from unrelated analytics implementation details and makes tests smaller.

5. Model v0 output as candidate market-stance actions with explicit timing and provenance.

   The initial strategy output should be a reusable candidate action record, not a venue order. Each action should identify the strategy evaluation that produced it, the candidate stance (`long` or `short`), the UTC decision time, the combined half-open input range behind the action, and propagated quality. This gives downstream governor work a stable event contract without prematurely choosing order fields, sizing, or venue routing.

   Alternative considered: emit raw crossover booleans only. That would be minimal, but it pushes too much translation work into downstream slices and weakens the shared-domain contract.

6. Start with moving-average crossover as the first strategy kind.

   The initial strategy should request two moving-average analytics series for the same instrument, timeframe, and range: a fast window and a slow window. The `runtime/strategy` request/parameter wrapper must require positive windows with `fastWindow < slowWindow`. The service should align points by shared analytics point time and emit candidate actions only when the fast average crosses the slow average between consecutive aligned points.

   Alternative considered: build a generic rule engine or threshold system first. That would be more flexible, but it would add abstraction without proving the basic strategy boundary.

7. Use explicit range semantics with no hidden pre-range lookback.

   The strategy service should request analytics using the caller's exact `[start, end)` range and evaluate only from the aligned analytics points it receives. The first aligned point establishes the initial in-range state but does not emit an action because there is no prior in-range state to compare against. This keeps evaluation deterministic and avoids hidden reads outside the requested range.

   Alternative considered: automatically extend analytics reads before `start` to seed initial state. That would detect earlier state transitions, but it hides extra data access and makes boundary behavior harder to reason about.

8. Propagate quality across the full crossover decision window.

   A crossover action depends on the previous and current aligned analytics point pairs. Action quality should therefore consider every analytics point that contributed to the state transition: if any contributing point is suspect, the action is suspect; otherwise if any contributing point is raw, the action is raw; otherwise it is validated.

   Alternative considered: derive action quality from only the current aligned point pair. That is simpler, but it hides the fact that crossover state depends on the previous pair as well.

9. Defer backend wiring until there is a real consumer.

   The initial strategy layer should be implemented and tested inside `runtime/` only. `apps/signal-foundry` wiring should be added later, or only in this change if implementation introduces a real route, job, or orchestrator that needs the service.

   Alternative considered: register strategy in the backend container immediately. That would create speculative app wiring with no current caller and would expand the change scope without user value.

## Risks / Trade-offs

- Missing pre-range state can suppress the first possible in-range action -> Keep the rule explicit and documented now; add seeded lookback only through a later spec change if a real consumer needs it.
- A single crossover strategy is intentionally narrow -> Use it to prove the slice boundary first, then extend through later spec changes once concrete strategy needs are known.
- On-demand evaluation may become expensive on large ranges -> Keep the v0 service stateless and measurable; add materialization only if usage proves the need.
- Candidate action records stop short of sizing or venue routing -> That is deliberate separation of concerns so strategy stays distinct from governor and execution.
- Quality propagation across previous and current analytics points may surprise callers who expect only current-state quality -> Keep the rule visible in the spec and tests so downstream consumers rely on explicit semantics.

## Migration Plan

1. Add shared strategy domain types without changing existing data or analytics behavior.
2. Add `runtime/strategy` service and focused tests using fake analytics dependencies.
3. Keep backend app wiring deferred unless implementation introduces a current consumer in the same change.
4. Rollback is file-level removal of the new domain additions, strategy package, and tests because no storage, migrations, or external API changes are required by default.

Implementation guardrails: the implementation tasks should treat the runtime-only scope as acceptance criteria. The change should not add strategy persistence, migrations, materialized signal tables, new public HTTP routes, or speculative `apps/signal-foundry` dependency injection wiring unless a real consumer is introduced and the OpenSpec plan is updated to justify that expanded scope.

## Open Questions

- Should a later strategy change introduce candidate `flat` actions, or should governor/execution infer exits from opposing `long` and `short` actions?
- Which next strategy kind should follow moving-average crossover once the initial boundary is proven?
- When backtesting or live orchestration work lands, should cross-slice execution live in a small runtime `runs/` or `flows/` area as suggested by `docs/ARCHITECTURE.md`?
