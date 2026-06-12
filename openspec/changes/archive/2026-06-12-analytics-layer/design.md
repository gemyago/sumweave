## Context

Signal Foundry's product architecture defines the deterministic path as `Data -> Analytics -> Strategy -> Governor -> Execution`. The current runtime already has canonical `domain` market data types and a `data` slice with deterministic candle/trade reads plus replay rows carrying stable identities.

The missing piece is a runtime analytics slice that transforms canonical market data into strategy-ready analytics without leaking venue mechanics, persistence details, or AI-assisted research into the critical path. The first implementation should fit the existing early-stage codebase: small public surface, consumer-defined interfaces, concrete constructors, explicit validation, and focused tests.

## Goals / Non-Goals

**Goals:**

- Add a deterministic `runtime/analytics` package that computes analytics from canonical candle replay data.
- Add shared domain output types for analytics series and points so future strategy code can consume a stable contract.
- Support an initial set of candle-derived calculations: moving average over close prices and period return over close prices.
- Preserve data-layer time semantics: UTC-normalized values, stable ordering, and `[start, end)` request ranges based on candle start times.
- Make output point-time and value-range semantics explicit for downstream strategy consumers.

**Non-Goals:**

- No external HTTP API for analytics in the initial change.
- No analytics persistence, migrations, backfill jobs, or materialized indicator tables.
- No AI model involvement, venue-specific behavior, live network access, or strategy decision logic.
- No broad indicator framework beyond the initial deterministic candle-derived calculations.
- No `apps/signal-foundry` dependency injection wiring unless a current backend consumer is introduced in the same implementation change.

## Decisions

1. Create a dedicated `runtime/analytics` slice.

   The analytics package should sit beside `runtime/data` rather than inside it. Data owns canonical ingestion, persistence, and replay; analytics owns derived deterministic calculations. Keeping the slice separate matches the architecture document and avoids turning the data layer into a generic computation module.

   Alternative considered: add analytics methods directly to `data.ReadService`. That would be simpler initially, but it blurs the product slice boundary and makes future strategy dependencies less explicit.

2. Use on-demand computation for v0.

   The service should compute analytics from the candle replay rows returned by the data layer for the requested range. This avoids a new storage model before there is a proven need for materialization, keeps rollback simple, and lets tests verify exact deterministic behavior from small fixture inputs.

   Alternative considered: persist analytics outputs immediately. That may be useful later for expensive indicators or backtests, but it would introduce schema, migration, invalidation, and provenance concerns before the first analytics contract is proven.

3. Define analytics outputs in `runtime/domain`.

   Add minimal shared domain concepts for analytics identity, series metadata, points, values, and quality. These types should not contain GORM tags, generated API metadata, venue payloads, or internal data-row identities. They should be small enough for strategy code to depend on later without importing the analytics implementation.

   Alternative considered: keep all output types inside `runtime/analytics`. That would reduce shared-domain additions, but future strategy code would either need to depend on the analytics package for data shapes or define duplicate records.

4. Accept a candle replay reader interface in the analytics service.

   The analytics service should define the narrow dependency it needs near the consumer, likely an interface matching the existing `ReplayCandles` behavior. Its constructor should return a concrete `*Service`, following the repo's Go convention of accepting interfaces and returning structs.

   Alternative considered: depend directly on `*data.ReadService`. That is acceptable for app wiring, but a consumer-defined interface keeps the slice testable and prevents analytics from coupling to unrelated data read methods.

5. Anchor analytics points to candle close time and expose the full contributing range.

   Each analytics point should have a point time equal to the end of the current candle's canonical `TimeRange`, normalized to UTC. The point should also expose the half-open value range covered by the input candles that contributed to that value. For moving average, the value range starts at the first candle in the window and ends at the current candle end. For period return, the value range starts at the lookback candle start and ends at the current candle end. This gives downstream consumers one stable point timestamp for ordering and a separate range for explainability, provenance, and later strategy alignment.

   Output ordering should follow point time ascending. If two output points have the same point time, the stable tie-breaker is the current/source replay candle identity ascending. The analytics service should preserve or derive that tie-breaker from the data replay rows rather than sorting by nondeterministic map or slice construction.

   Alternative considered: use candle start time as the analytics point time. That mirrors the data-layer read boundary, but it is less intuitive for indicators that are only known after the candle closes and can mislead downstream consumers into acting before the value is complete.

6. Emit only computable points.

   Moving average and period return calculations need warmup input. For v0, the service should return points only after enough candles are present in the requested replay range rather than implicitly reading outside the caller's requested range. This keeps the request semantics obvious and deterministic.

   Alternative considered: automatically expand the data query for lookback. That gives denser output at the beginning of a range, but it hides extra data access and requires more precise boundary rules.

7. Fail period-return requests on invalid denominators.

   Period return should require every emitted point's lookback close price to be strictly positive. If any otherwise-computable point in the requested replay range has a zero or negative lookback close, the service should reject the whole request with a validation/calculation error and return no partial series. This is stricter than omitting only the bad point, but it keeps downstream consumers from accidentally treating a sparse series as a valid market signal and matches the existing fail-fast validation style.

   Alternative considered: omit points with zero or negative lookback closes. That can be convenient for messy data, but it makes missing output ambiguous with warmup omission and hides a data-quality issue from strategy code.

8. Defer backend wiring until there is a current consumer.

   The initial analytics slice should be implemented and tested inside `runtime/` only. `apps/signal-foundry` wiring should be added later, or in this change only if an implementation task introduces a real backend consumer that needs the service resolved from the app container. Avoiding speculative app wiring keeps the change smaller and reduces coupling before there is an external route, job, or strategy integration.

## Risks / Trade-offs

- Sparse initial output during warmup -> Document and test that the first computable point appears only after the required number of candles.
- On-demand computation may become expensive for large ranges -> Keep the service stateless now; add materialization later only when usage proves the need.
- Minimal indicator scope may not cover all strategy needs -> Start with a small testable set and extend through later spec changes.
- Quality propagation can be ambiguous -> Define deterministic quality rules in the spec and keep them visible in domain output records.
- Failing period-return requests on invalid denominators may reject ranges with one bad candle -> Prefer explicit failure over silently sparse strategy inputs.

## Migration Plan

1. Add shared analytics domain types without changing existing data-layer behavior.
2. Add `runtime/analytics` service and tests using in-memory fake candle replay readers.
3. Keep backend app wiring deferred unless a current backend consumer is introduced.
4. Rollback is file-level removal of the new domain additions, analytics package, and tests because no persistence migration, app wiring, or external API is introduced by default.

## Open Questions

- Should a later change materialize analytics outputs for backtests, or is on-demand computation sufficient through the first strategy slice?
- Which additional indicators should be promoted after the first strategy use case is known?
