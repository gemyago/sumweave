## 1. Shared Domain Foundation

- [x] 1.1 Add `runtime/domain` market/reference data types for venue, instrument, asset class, symbol, timeframe, time range, candle, trade, data quality, and source provenance.
- [x] 1.2 Add domain tests covering UTC timestamp normalization, valid enum/string construction, and whole-value comparisons for canonical records.
- [x] 1.3 Verify domain types do not contain GORM tags, table names, or persistence-only fields.

## 2. Runtime Data Slice Contracts

- [x] 2.1 Add `runtime/data` ingestion and read service constructors that return concrete structs and accept store dependencies as consumer-defined interfaces.
- [x] 2.2 Implement validation for instrument upsert inputs, candles, trades, non-negative prices/sizes, candle time ranges, and required provenance, allowing ingestion to upsert instruments by venue and symbol from normalized records.
- [x] 2.3 Add service tests with local fakes for valid ingestion, validation failures, UTC normalization, and no persistence on rejected records.

## 3. GORM Persistence

- [x] 3.1 Add data-layer GORM models with explicit table names, explicit column names, UTC timestamp fields, and unique indexes for instrument and market-data natural keys.
- [x] 3.2 Implement database store construction, `AutoMigrate`, instrument upsert/lookup, candle upsert/query, trade upsert/query, and domain/persistence mappers.
- [x] 3.3 Add SQLite-backed store tests for migration, explicit column behavior, instrument upsert, idempotent candle ingestion, idempotent trade ingestion, and provenance/quality persistence.

## 4. Deterministic Query And Replay

- [x] 4.1 Implement candle reads by instrument, timeframe, and `[start, end)` time range ordered by start time ascending.
- [x] 4.2 Implement trade reads by instrument and `[start, end)` time range ordered by event time ascending with a stable tie-breaker.
- [x] 4.3 Implement replay-oriented read methods that return stable identities and use explicit `[start, end)` range semantics from the data package.
- [x] 4.4 Add query and replay tests covering ordering, boundary filtering at `start` and `end`, stable repeated reads, and returned quality state.

## 5. Backend App Wiring

- [x] 5.1 Add `dataLayer` configuration defaults for database DSN, table prefix, and auto-migration without reusing agent runtime storage settings.
- [x] 5.2 Register data-layer store/service constructors in `apps/signal-foundry` DI wiring while keeping business rules in `runtime/data`.
- [x] 5.3 Run data-layer auto-migration during backend startup when enabled and skip it when disabled.
- [x] 5.4 Add backend configuration and wiring tests for defaults, env overrides, enabled migration, and disabled migration.

## 6. Documentation And Verification

- [x] 6.1 Update module or project documentation only if implementation introduces new commands, workflows, or architecture decisions.
- [x] 6.2 Run focused runtime and backend tests while developing the implementation.
- [x] 6.3 Run `make affected-lint-test` from the repository root and resolve all lint/test failures.
- [x] 6.4 Confirm AGENTS.md updates are unnecessary or apply any needed rule/convention changes before reporting completion.
