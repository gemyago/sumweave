# Chunk Review: finance-sync-state-journal-persistence

Implementation and review history for chunk `finance-sync-state-journal-persistence`.

## 2026-06-26 Implementation

Verdict: complete for chunk scope.

### Implemented

- Renamed persisted window columns from `successful_window_start` / `successful_window_end` to `window_start` / `window_end`.
- Made attempt-window persistence required on every journal row.
- Updated journal mapping and tests so failed rows round-trip with a concrete attempted window and nullable `SucceededAt`.
- Confirmed the newest appended row is returned per connection regardless of success or failure.

### Checks

- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/providers ./finance/persistence`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec updates

- Marked tasks `2.1` and `2.2` complete in `tasks.md`.
- Updated `manager-status.md` with the chunk ledger.

## Completion Protocol Status

- Root coding protocol: pass after `make affected-lint-test`.
- `finance/AGENTS.md` protocol: pass; GORM-owned schema changes stayed inside finance persistence as required.
- `AGENTS.md` update: not needed.

## Artifact Cleanup Status

- Clean with respect to artifact type: only standard OpenSpec artifacts were added.

## Commit Status

- No commit created; chunk changes remain in the working tree.
