## Why

Signal Foundry needs a deterministic Data slice before analytics, strategy, governor, and execution work can rely on stable market and reference inputs. Data Layer v0 establishes the first product-owned boundary for validated ingestion, normalized records, replayable persistence, and read access.

## What Changes

- Introduce the `runtime/data` slice as the owner of market/reference data ingestion, normalization, quality state, persistence ports, and query/replay services.
- Introduce small shared domain types for instruments, venues, timeframes, candles, and trades needed by downstream slices.
- Add GORM-backed persistence models and migrations isolated from shared domain types, supporting SQLite first and PostgreSQL-compatible schema choices.
- Define v0 ingestion to upsert instruments by venue and symbol from normalized records before persisting dependent candles or trades.
- Add app wiring in `apps/signal-foundry` for data-layer configuration, auto-migration, and future jobs/API integration without putting AI in the deterministic path.
- Add unit and module-level tests that verify validation, mapping, persistence, idempotent ingestion, and `[start, end)` replay/query behavior.

## Capabilities

### New Capabilities

- `data-layer`: Canonical instrument, candle, and trade ingestion with normalization, quality state, replayable persistence, and deterministic `[start, end)` read access for downstream slices.

### Modified Capabilities

None.

## Impact

- Affects `runtime/` by adding the first product Data slice and small shared-domain types used across future slices.
- Affects `runtime/internal/gormsignalfoundry` only if shared GORM DSN/dialector helpers need minor reuse or extension.
- Affects `apps/signal-foundry/` by adding configuration and DI/startup wiring for data persistence and migrations.
- Does not add operator UI requirements in v0, though the design should leave room for future data health and ingestion status views.
- Adds or updates Go module tests; implementation completion will require the repository coding-task protocol, including `make affected-lint-test`.
