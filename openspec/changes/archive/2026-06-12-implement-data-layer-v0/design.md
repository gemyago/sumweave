## Context

Signal Foundry's source-of-truth architecture defines the deterministic path as `Data -> Analytics -> Strategy -> Governor -> Execution`, with AI outside the critical path. The repository currently has the runtime foundation and backend process, but no product-owned Data slice for canonical market/reference data.

The runtime module is the intended home for core product logic. The backend app should wire process concerns such as configuration, migrations, and startup, while the runtime owns deterministic domain behavior. Persistence starts with GORM, must support SQLite for local development and PostgreSQL later, and must keep GORM models separate from shared domain types.

## Goals / Non-Goals

**Goals:**

- Add a `runtime/data` slice that owns validated ingestion, normalization, quality state, persistence ports, and deterministic query/replay services.
- Keep v0 intentionally narrow but complete for market data by supporting both candles and trades.
- Add a small `runtime/domain` shared kernel for canonical data concepts that downstream slices can reuse without importing persistence details.
- Provide database-backed storage through GORM with explicit schemas, explicit column names, UTC timestamps, and idempotent writes.
- Wire the backend app to configure, construct, and migrate the data layer without adding UI or AI execution-path behavior.
- Keep exported contracts small and concrete, following the repo's "accept interfaces, return structs" convention.

**Non-Goals:**

- No analytics, strategy, governor, execution, backtesting, or live trading behavior.
- No operator UI for ingestion management or data health in v0.
- No venue-specific ingestion framework beyond a narrow source/provenance model.
- No AI-assisted data repair or AI-generated trading decisions.
- No high-throughput optimization beyond disciplined schemas and query boundaries.

## Decisions

### Decision: Create `runtime/domain` for shared data concepts

Add a small shared kernel package for stable product types such as `Venue`, `InstrumentID`, `Symbol`, `AssetClass`, `Timeframe`, `Candle`, `Trade`, `DataQuality`, and `TimeRange`.

Rationale: downstream slices need canonical concepts, but those concepts must not carry GORM tags or storage-only fields. Keeping them in `domain` follows the architecture source of truth and avoids coupling analytics/strategy to the Data slice internals.

Alternative considered: define all models inside `runtime/data`. That keeps scope smaller initially, but it would force downstream slices either to import the Data slice for basic records or duplicate vocabulary later.

### Decision: Put deterministic behavior in `runtime/data`

Add a `runtime/data` package with concrete services for ingestion and reads. Dependencies such as stores and clocks are accepted as consumer-defined interfaces near the consuming service, and constructors return concrete structs.

Rationale: Data is the first product slice and should be usable by both app wiring and future runtime orchestration. Keeping behavior in the slice avoids pushing product rules into the backend app or GORM model methods.

Alternative considered: keep the full data layer internal until another slice consumes it. That would reduce public surface, but the backend app must construct and migrate the layer, and the architecture already identifies Data as a real product slice.

### Decision: v0 includes both candles and trades

Scope Data Layer v0 to canonical instrument reference data plus the first two replayable market-record types: candles and trades. Both record families share the same validation, persistence, and deterministic read/replay contract in v0.

Rationale: the current proposal, tasks, and spec direction already assume both record types, and downstream deterministic slices will need both aggregated and event-level reads. Keeping both in v0 is the smallest coherent scope that avoids designing the ingestion and replay contracts twice.

Alternative considered: ship candles-only first. That would reduce initial surface area, but it would leave the spec and tasks out of sync with the intended deterministic market-data foundation and force early follow-up churn in contracts, schema keys, and replay semantics.

### Decision: Use GORM storage behind slice-owned models

Implement a database store in `runtime/data` with unexported persistence models, explicit table/column names, uniqueness constraints for idempotent ingestion, and mappers to/from `domain` records.

Rationale: the repo architecture already selects GORM, SQLite, and PostgreSQL-compatible schemas. Unexported persistence models keep tags and database concerns away from `domain`.

Alternative considered: use raw SQL immediately. Raw SQL could be faster and more explicit, but GORM matches current architecture and existing runtime persistence helpers. Hot paths can move to lower-level SQL later without changing slice contracts.

### Decision: Make ingestion idempotent by natural keys

Candles and trades are stored with source-aware natural keys: venue, instrument, timeframe when applicable, event timestamp, source, and source record ID when available. Re-ingesting the same source record updates metadata or quality fields without creating duplicates.

Rationale: deterministic replay depends on repeatable ingestion and stable ordering. Natural keys also let import jobs resume safely.

Alternative considered: always append raw imported rows and deduplicate during reads. That preserves every vendor payload, but it makes downstream consumers responsible for duplicate handling and weakens deterministic replay.

### Decision: Ingestion may upsert instruments from normalized records

Allow candle and trade ingestion to upsert the referenced instrument by venue and symbol when the normalized record carries the canonical instrument fields required by the data layer. Prior explicit instrument creation remains valid, but it is not required for v0 ingestion success.

Rationale: this keeps the v0 ingestion contract small for import jobs while preserving deterministic identity rules at the slice boundary. A single canonical instrument row per venue and symbol also aligns with the planned instrument lookup and idempotent persistence behavior.

Alternative considered: require instruments to be created in a separate step before market-data ingestion. That would simplify some validation branches, but it would add orchestration complexity before the data layer has any other product-owned writer and would not match the existing tasks/spec direction.

### Decision: Query and replay ranges use `[start, end)` semantics

Define every candle and trade query or replay range as inclusive of `start` and exclusive of `end`. A record is included when its relevant event boundary is `>= start` and `< end`; it is excluded when it falls before `start` or at/after `end`.

For candles, the relevant boundary is the candle start timestamp. For trades, it is the trade event timestamp.

Rationale: `[start, end)` semantics make adjacent windows compose without overlap, avoid double-counting on repeated replay slices, and match common deterministic batch-processing conventions.

Alternative considered: fully inclusive ranges. Inclusive end bounds create ambiguity at window joins and make repeated adjacent reads harder to reason about for both candles and trades.

### Decision: Separate source provenance from venue integration

Represent source/provenance as data-layer metadata, not as a generic exchange adapter framework. The v0 ingestion API accepts already-normalized candidate records plus source metadata.

Rationale: venue-specific signing, symbol mapping, and transport belong at the venue edge. Data Layer v0 should validate and persist canonical records without committing to a large venue framework.

Alternative considered: build vendor adapters as part of v0. That would make demos easier, but it expands scope before the storage and query contracts are proven.

### Decision: App wiring is configuration and lifecycle only

Add backend configuration for data-layer database DSN, table prefix, and auto-migration. The app constructs the runtime data store/service and runs migrations on startup when enabled.

Rationale: the app owns process lifecycle and deployment configuration; runtime owns product behavior. This mirrors existing agent runtime storage wiring without making the backend app the source of data-layer business rules.

Alternative considered: reuse agent runtime database configuration directly. That would be quick locally, but it couples unrelated storage lifecycles and makes future operational separation harder.

## Risks / Trade-offs

- Schema churn in an early product slice -> Keep migrations explicit and accept breaking changes while the project is early, but isolate persistence models so domain contracts can stay cleaner.
- Exported package surface grows too quickly -> Export only the minimal constructors, params, and domain records needed by app wiring and future slices.
- GORM hides SQL behavior that matters for replay -> Use explicit column names, constraints, UTC normalization, and focused tests for ordering, idempotency, and query ranges.
- Range semantics can drift between reads and tests -> Specify `[start, end)` once in the slice contract and verify it across candle and trade queries plus replay-oriented reads.
- SQLite and PostgreSQL differ in time and conflict handling -> Keep schema choices portable, test SQLite in v0, and avoid dialect-specific behavior unless wrapped behind store methods.
- Data-quality semantics can become vague -> Start with a small enum/status model and provenance fields, then expand only when concrete ingestion jobs require it.

## Migration Plan

1. Add domain and data packages with in-memory or fake stores sufficient for service tests.
2. Add GORM persistence models, migrations, and database-backed tests using SQLite.
3. Wire backend configuration and startup migration behind a dedicated `dataLayer` config group.
4. Keep auto-migration enabled for local/test defaults and configurable for production.
5. Rollback by disabling data-layer startup wiring and leaving unused tables in place until an explicit cleanup migration is needed.

## Open Questions

- What table prefix should the backend default use for data-layer tables?
- Which production PostgreSQL migration workflow should replace or constrain GORM auto-migration when deployments become stricter?
