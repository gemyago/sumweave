# Chunk Review: batch-audit-replay

Implementation and review history for chunk `batch-audit-replay`.

## 2026-06-14 Chunk implementation review

Verdict: clean for chunk scope.

### Implemented

- Added batch-linked ingestion entry points in `runtime/data/service.go`:
  - `IngestCandleForDataBatch` canonicalizes batch ID and delegates to candle store batch upsert.
  - `IngestTradeForDataBatch` canonicalizes batch ID and delegates to trade store batch upsert.
  - Non-lineage ingestion (`IngestCandle`, `IngestTrade`) continues to use the existing non-batch upsert path.
- Extended `runtime/data/database_store.go` lineage-aware persistence and replay for candle/trade rows:
  - Added `UpsertCandleForDataBatch` and `UpsertTradeForDataBatch` that check batch existence via `ensureDataBatchExists` when batchID is provided.
  - Added `ReplayCandlesByDataBatch` with stable order `start_at ASC, id ASC`.
  - Added `ReplayTradesByDataBatch` with stable order `event_time ASC, id ASC`.
- Added batch-scoped audit/read integration for lineage chain:
  - `GetDataBatchAudit` loads batch -> normalization run -> raw venue payloads -> ingestion run via `loadDataBatchAudit`.
  - Raw payload audit ordering now deterministic by SQL join: `ORDER BY raw.received_at ASC, raw.id ASC`.
- Added focused tests covering batch write/replay behavior and lineage/audit ordering in:
  - `runtime/data/database_store_lineage_test.go` (batch linkage, replay identity stability, deterministic audit ordering, parent checks)
  - `runtime/data/service_test.go` (batch ingest methods persist through batch-specific store methods with trimmed batch ID)
  - `runtime/data/lineage_service_test.go` (batch audit and batch replay delegation + batch ID canonicalization in service layer)

### Checks

- `go test ./...` (run from `runtime` module directory)

### Findings

1. No blocking findings for this chunk.
2. Scope remains compliant:
   - No run-level batch/audit/replay API was added.
   - Existing non-lineage ingestion paths remain unchanged and are still tested.

### Completion Protocol Status

- Runtime module protocol: pass — `go test ./...` completed successfully for updated `runtime` scope.
- AGENTS check: pass — no workflow/command/architecture updates were introduced in this chunk.

### Artifact Cleanup Status

- Clean. No ad-hoc artifacts were created.

### Commit Status

- No commit created in this review step.

### Continue Decision

- Safe to proceed to final OpenSpec consolidation.
