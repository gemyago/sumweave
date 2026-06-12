# Chunk Review: gorm-persistence

Implementation and review history for chunk `gorm-persistence`.

## 2026-06-12 Initial implementation

Verdict: complete for chunk scope.

### Implemented

- Added `runtime/data` GORM persistence models with explicit table names through the configured naming strategy, explicit column names, UTC-first timestamp handling, and unique natural-key indexes for instruments, candles, and trades.
- Implemented a concrete `DatabaseStore` with database construction, `AutoMigrate`, instrument upsert/lookup, candle upsert/query, trade upsert/query, and explicit domain/persistence mapper functions.
- Added SQLite-backed persistence tests covering migration, explicit column behavior, instrument upsert, idempotent candle ingestion, idempotent trade ingestion, and provenance/quality persistence through the real store and ingestion service path.

### Checks

- `go test ./runtime/data`
- `make affected-lint-test`

### OpenSpec updates

- Marked tasks `3.1`, `3.2`, and `3.3` complete in `tasks.md`.
- Updated `manager-status.md` to record chunk `gorm-persistence` as completed.

### Artifact cleanup

- Clean. No ad-hoc repository artifacts were created.

## 2026-06-12 Chunk finalization review

## Verdict

1. Trade idempotency key is too broad for the approved spec intent because the current `trades` uniqueness rule includes `price` and `size`, which can allow a second row when the same source record is re-ingested with corrected values.
2. The chunk artifacts do not yet explicitly record `/opsx-apply` confirmation, so the process evidence is incomplete even though the chunk otherwise follows the OpenSpec flow.

## Continue Decision

- not safe to continue

## Completion Protocol Status

- Runtime module protocol: pass — `go test ./runtime/data`, `make -C runtime lint`, and `make affected-lint-test` passed.
- AGENTS.md update check: pass — no AGENTS.md changes were needed for this scope.

## Artifact Cleanup Status

- clean

## Commit Status

- no commit created because follow-up fixes are required before the chunk is safe to continue

## Affected Follow-up Chunks

- `gorm-persistence`

## 2026-06-12 Follow-up fix implementation

Verdict: follow-up fixes applied for this chunk.

### Implemented

- Narrowed the `trades` natural key and upsert conflict target to the source-aware identity path: `instrument_id`, `provenance_source`, and `provenance_record_id`.
- Removed `price` and `size` from the trade uniqueness rule so re-ingesting the same source record with corrected trade values updates the existing row instead of creating a second one.
- Updated the SQLite-backed trade persistence test to verify a second ingest with corrected `price` and `size` remains idempotent.
- Added explicit `/opsx-apply` process evidence to this chunk artifact: the literal `/opsx-apply` command was not available in this environment, so the implementation used the repository OpenSpec artifacts and chunk-status files as the operative workflow after the user explicitly authorized that fallback.

### Checks

- `go test ./runtime/data`
- `make affected-lint-test`

### OpenSpec updates

- Updated this chunk review with explicit follow-up fix and `/opsx-apply` evidence.
- Updated `manager-status.md` to clear the review block and mark chunk `gorm-persistence` complete again.

### Artifact cleanup

- Clean. No ad-hoc repository artifacts were created.

## 2026-06-12 Follow-up finalization review

## Verdict

1. Clean. Trade idempotency now matches the approved source-aware spec intent because corrected trade values update the existing source-record row instead of widening the key.
2. `/opsx-apply` evidence is now explicit and truthful in the chunk artifact: the command was unavailable here, and the user-approved OpenSpec artifact workflow was used instead.

## Continue Decision

- safe to continue

## Completion Protocol Status

- Runtime module protocol: pass — `go test ./runtime/data` and `make affected-lint-test` passed after the fix.
- AGENTS.md update check: pass — no AGENTS.md changes were needed for this scope.
- `/opsx-apply` confirmation: pass — artifact now records the exact fallback that was used because the literal command was unavailable in this environment.

## Artifact Cleanup Status

- clean

## Commit Status

- follow-up commit created: `f46bc13` (`fix: narrow trade idempotent key and record opsx evidence`)

## Affected Follow-up Chunks

- `deterministic-query-replay`
