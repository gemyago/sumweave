## Context

The current runtime has canonical data ingestion/read services, ingestion-run and raw-payload lineage, Hyperliquid `/info` raw evidence capture, and an ingestion flow that links read-result raw payload IDs to persisted canonical records. `apps/signal-foundry` already has Cobra command scaffolding and DI/config wiring for the data store, lineage service, local raw payload blob store, Hyperliquid venue, and ingestion flow.

The requested slice is a manual operator command, not a scheduler or product UI. It should compose existing runtime behavior and keep vendor mechanics at `runtime/venueedge`, durable data behavior at `runtime/data`, and cross-cutting orchestration in a thin runtime flow package.

## Goals / Non-Goals

**Goals:**

- Provide a reusable historical raw candle backfill runner that is independent of Cobra parsing.
- Support Hyperliquid perps candles over explicit half-open `[start, end)` ranges.
- Record ingestion-run status transitions and deterministic lifecycle metadata before/after venue reads and on failure after start.
- Capture raw Hyperliquid candle payload evidence with the provided run ID, persist canonical candles idempotently, and link raw payload IDs to candles.
- Produce deterministic gap/completeness reports from persisted candle readback.
- Add a CLI command with stable output suitable for tests and operator use.

**Non-Goals:**

- No historical trades, trade paging, or expansion of `recentTrades` beyond its current latest-window limitation.
- No scheduler, recurring jobs, backend API route, UI workflow, wallet signing, private Hyperliquid endpoint, order/execution behavior, or AI-assisted path.
- No external object storage, blob retention/compression/GC, or production migration hardening beyond the existing GORM/SQLite-first posture.
- No broad multi-venue framework beyond narrow interfaces needed by this runner.

## Decisions

1. Put reusable orchestration in `runtime/flows` unless implementation finds an existing better orchestration home.

   The runner should be a runtime-level flow because it composes data, lineage, venue, and readback concerns, but should not live in `runtime/data` or `runtime/venueedge` where it would blur slice ownership. A package such as `runtime/flows` can expose request/result/report types while keeping dependencies as small consumer-defined interfaces.

   Alternative considered: implement the job directly in Cobra. That would make CLI tests harder and prevent reuse by future scheduler or API surfaces.

2. Keep the runner dependency-injected and testable.

   The runner should accept interfaces for ingestion-run recording, candle ingestion flow, Hyperliquid venue construction, candle replay/readback, and clock/time source. Constructors may return concrete structs, following repository Go conventions. The CLI layer builds concrete dependencies from existing app wiring.

   Alternative considered: have the runner instantiate stores and HTTP clients directly. That would duplicate app configuration behavior and make mocked tests brittle.

3. Treat `--run-id` as the stable ingestion-run identity and raw evidence context.

   The runner records an `IngestionRun` as `started` before the first venue call, with deterministic lifecycle fields: provided run ID, stable historical raw candle backfill source, venue `hyperliquid-perps`, status `started`, UTC `StartedAt` from the runner clock, unset completion time, record count `0`, and empty error summary. It then constructs/receives a Hyperliquid venue configured with `RawEvidenceIngestionRun = runID` and updates the same run to `succeeded` or `failed`. Repeated writes rely on existing ingestion-run idempotency; canonical candle persistence remains idempotent while raw evidence appends for each new HTTP exchange.

   Alternative considered: generate run IDs automatically. That would reduce CLI burden but break deterministic operator reruns and acceptance criteria.

4. Use existing venue ingestion flow and read/replay services for persistence and gap reporting.

   The backfill should call `IngestionFlow.IngestCandles` so existing raw-to-candle linkage behavior is reused. After ingestion, it should query/replay persisted candles for the requested instrument/timeframe/range and compare expected interval boundaries to persisted candle starts. This keeps gap reporting grounded in durable state rather than only the latest venue response.

   Alternative considered: compute the report only from fetched candles. That would miss idempotent persisted state and persistence failures.

5. Keep CLI output stable and compact.

   Human-readable output should use deterministic key/value or clearly ordered lines with run ID, requested scope, persisted count, expected count, gap count, first/last persisted boundaries, and a capped missing-interval preview. The result type may retain full missing intervals while CLI output caps them.

   Raw payload count is optional: include it only when an existing cheap run-scoped lineage read or already-collected response metadata can provide it. If it is not cheaply available, the report and CLI output should omit the field consistently rather than print a placeholder or add broad audit scans for this slice.

   Alternative considered: JSON-only output. The task asks for human-readable default output; a future `--format json` can be separate if needed.

6. Reject unsupported scope early.

   The CLI/runner validation must allow only `hyperliquid-perps`, canonical future/perps asset class mapping, supported Hyperliquid candle timeframes, positive half-open ranges, non-empty run IDs, and zero-or-positive page size. If a data-type path is introduced during implementation, `trades` must return a clear unsupported historical-trade error rather than calling `recentTrades`.

   Alternative considered: rely on lower-level adapter errors. Early validation gives operators clearer failures and prevents accidental trade-history expansion.

## Risks / Trade-offs

- Large backfills can run for a long time → keep v0 manual and visible; checkpoint/resume remains a follow-up.
- Gap reports can be large → cap CLI output while keeping total counts in the report.
- Raw blob storage grows on repeated runs → expected v0 behavior; retention/GC remains out of scope.
- Hyperliquid historical candle window/page behavior may differ live → use mocked-HTTP tests for deterministic coverage and avoid live network requirements in default tests.
- Partial failures can leave a failed run with persisted candles and raw evidence → update the ingestion run with best-known persisted count and concise error summary; durable resume is a later ticket.

## Migration Plan

1. Add runner request/result/report types and validation tests in the chosen runtime flow package.
2. Add ingestion-run lifecycle, Hyperliquid venue construction/candle ingestion, and failure status behavior behind dependency-injected interfaces.
3. Add persisted-candle readback and gap-report computation with raw-payload-count inclusion/omission and capped CLI rendering semantics documented in tests.
4. Wire `apps/signal-foundry` command structure (`data backfill-raw-candles`) to parse flags, resolve configured services, call the runner, and print deterministic output.
5. Use mocked HTTP and temporary SQLite/blob paths in integration-style tests; no manual/live step is required for default completion.

## Open Questions

- None blocking planning.
