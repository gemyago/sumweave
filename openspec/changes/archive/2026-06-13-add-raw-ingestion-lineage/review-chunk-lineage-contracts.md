# Chunk Review: lineage-contracts

Implementation and review history for chunk `lineage-contracts`.

## 2026-06-14 Chunk finalization review

Verdict: clean for chunk scope.

### Implemented

- Added `runtime/data/lineage.go` with canonical domain records and constructors for:
  - `IngestionRun`, `RawVenuePayload`, `NormalizationRun`, `DataBatch`
  - stable identity checks, enums canonicalization, parent id presence checks where local validation applies
  - UTC timestamp normalization and metadata/body defensive copy/canonicalization behavior.
- Added `runtime/data/lineage_service.go` with a focused service contract for lineage upsert/audit/replay operations and lightweight canonicalization + store delegation.
- Added focused line-level and service tests in:
  - `runtime/data/lineage_test.go`
  - `runtime/data/lineage_service_test.go`
- Updated OpenSpec bookkeeping in:
  - `openspec/changes/add-raw-ingestion-lineage/tasks.md` (tasks `1.1` and `1.2` marked complete)
  - `openspec/changes/add-raw-ingestion-lineage/manager-status.md` (lineage-contracts implementation run recorded)

### Checks

- `go test ./...` (executed from `runtime` module directory)
- `make affected-lint-test`

### Findings

1. **Non-blocking follow-up**: parent-link error propagation is only explicitly tested for raw payloads.
   - `runtime/data/lineage_service_test.go` verifies `ErrLineageParentNotFound` via `RecordRawVenuePayload` only.
   - `RecordNormalizationRun` and `RecordDataBatch` do not have matching error-path tests for missing parent lineage rows.
   - Follow-up needed in this chunk/following chunk (currently `gorm-lineage-persistence`) to ensure unknown-parent handling is covered consistently across all lineage child records.

### Completion Protocol Status

- Runtime module protocol: pass — `go test ./...` and `make affected-lint-test` succeeded with no new findings.
- AGENTS update check: pass — no command/workflow/architecture changes required.

### Artifact Cleanup Status

- Clean. No ad-hoc artifacts were created.

### Commit Status

- No commit created by this review step.

### Continue Decision

- Safe to continue, with follow-up coverage note above.
