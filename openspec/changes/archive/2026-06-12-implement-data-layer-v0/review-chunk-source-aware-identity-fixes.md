# Chunk Review: source-aware-identity-fixes

Implementation and review history for follow-up chunk `source-aware-identity-fixes`.

## 2026-06-12 Follow-up fix implementation

Verdict: follow-up fixes applied for this chunk.

### Implemented

- Added persisted source-aware identity columns for candle and trade rows so GORM uniqueness and upsert conflict targets match the approved v0 identity rules instead of collapsing different source records together.
- Updated candle persistence to keep distinct rows for the same instrument/timeframe bucket when they arrive from different sources, while preserving idempotent updates for the same source record.
- Updated trade persistence to fall back to event-time identity when `provenance.recordID` is blank, while still using source record identity when a record ID is present.
- Tightened runtime/data SQLite-backed tests to explicitly cover multi-source candles at the same bucket, blank-trade-record-ID fallback behavior, and the adjusted schema/index columns that make those cases work.

### Checks

- `go test ./runtime/data`
- `go test ./runtime/data -run TestDatabaseStore -count=1`
- `make affected-lint-test`

### OpenSpec updates

- Added this follow-up chunk review record for the applied persistence and test fixes.
- Updated `manager-status.md` to mark the follow-up fix chunk complete after repository-level verification passed.

### Artifact cleanup

- Clean. No ad-hoc repository artifacts were created.
