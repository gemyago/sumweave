## Why

Raw venue payloads currently disappear once they are normalized into canonical data records, which makes ingestion replay and audit trails harder to trust. Signal Foundry needs a minimal lineage path in `runtime/data` so operators and future jobs can tie canonical records back to ingestion runs, raw payloads, normalization attempts, and persisted batches.

## What Changes

- Add data-layer lineage records for `IngestionRun`, `RawVenuePayload`, `NormalizationRun`, and `DataBatch` without moving vendor transport concerns into the data slice.
- Persist raw payload bodies plus non-sensitive source metadata, normalization outcomes, and batch summaries through the existing GORM-backed data store.
- Link persisted canonical candle and trade rows to a data batch where lineage-aware ingestion is used, while keeping canonical market data domain records free of GORM tags and raw payload bodies.
- Add batch-scoped read access sufficient to audit a persisted data batch and replay which canonical rows came from its lineage chain; direct run-level audit lookups are out of scope for this change.
- Add tests for validation failures across all lineage record types, UTC normalization, idempotent lineage persistence, migration behavior, stable audit ordering, and batch-linked canonical record persistence.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `data-layer`: Add minimal raw ingestion lineage and persistence for ingestion runs, raw venue payloads, normalization runs, and data batches.

## Impact

- Affects `runtime/data` service contracts, GORM models, migrations, mappers, and tests.
- May add data-layer-specific domain records in `runtime/data`; avoid adding raw payload concepts to shared `runtime/domain` unless implementation proves they are cross-slice contracts.
- Affects existing candle/trade persistence only by adding optional batch lineage linkage for lineage-aware ingestion paths.
- Does not add ingestion scheduling, backend HTTP/API routes, UI, venue adapter rewrites, external blob storage, or AI-assisted data repair.
- Completion will require the repository coding-task protocol for runtime code changes, including `make affected-lint-test` from the repository root.
