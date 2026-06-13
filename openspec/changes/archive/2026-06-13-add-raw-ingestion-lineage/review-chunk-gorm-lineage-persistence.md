# Chunk Review: gorm-lineage-persistence

Implementation and review history for chunk `gorm-lineage-persistence`.

## 2026-06-14 Chunk implementation review

Verdict: clean for chunk scope.

### Implemented

- Added lineage persistence models and migration coverage in `runtime/data/database_store.go`:
  - `ingestion_runs`
  - `raw_venue_payloads`
  - `normalization_runs`
  - `normalization_run_raw_payload_links`
  - `data_batches`
- Added explicit schema constraints and indexes validated by migration tests:
  - explicit table/column names and UTC `created_at`/`updated_at` handling
  - unique indexes on lineage identities (`id` per entity)
  - normalized join uniqueness for raw payload links
- Implemented idempotent upserts and parent-guard behavior in `runtime/data/database_store.go`:
  - `UpsertIngestionRun`
  - `UpsertRawVenuePayload` (validates ingestion parent)
  - `UpsertNormalizationRun` (validates raw payload parents + link replacement)
  - `UpsertDataBatch` (validates normalization parent)
- Implemented lineage audit load path in `DatabaseStore`:
  - `GetDataBatchAudit` and helpers for loading normalization + raw payload + ingestion chain
  - deterministic raw payload ordering by `received_at` then `id`
- Added lineage persistence coverage in `runtime/data/database_store_lineage_test.go` for:
  - migration schema/index assertions
  - idempotent upsert semantics
  - UTC time normalization
  - missing-parent rejection at each lineage level
  - checksum/body persistence and secret-metadata redaction
  - stable audit ordering for mixed `received_at` / ID sequences

### Checks

- `go test ./...` (run from `runtime/data` directory)

### Findings

1. No blocking findings for the chunk scope.
   - Prior follow-up from `lineage-contracts` is addressed: unknown-parent handling now has explicit tests and database-level validation for payload, normalization, and data batch upserts.
   - Stable ordering in batch audit is enforced with deterministic SQL ordering and covered by test data that exercises same-timestamp tie-break behavior.

### Completion Protocol Status

- Runtime module protocol: pass — focused package tests run successfully after implementation.
- AGENTS check: pass — no command/workflow/architecture updates were introduced in this chunk.

### Artifact Cleanup Status

- Clean. No ad-hoc artifacts created.

### Commit Status

- No commit created in this review step.

### Continue Decision

- Safe to continue to `batch-audit-replay`.
