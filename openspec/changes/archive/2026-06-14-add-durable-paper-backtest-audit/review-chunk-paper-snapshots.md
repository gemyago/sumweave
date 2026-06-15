# Chunk Review: paper-snapshots

Implementation and review history for chunk `paper-snapshots`.

## 2026-06-14 Chunk-4 verification pass

Verdict: **PASS** for chunk scope.

### Scope alignment

- Confirmed all edited artifacts are confined to chunk-4 requirements (`4.1` and `4.2`): deterministic position and portfolio projection, persistence, and query behavior.
- Files changed are in `runtime/execution`, `runtime/domain`, and `openspec/.../tasks.md`.
- No `BacktestRun`, `EvaluationReport`, or report-scope scaffolding files were added; chunk-5 backtest reporting is still untouched.
- No API/UI/flow orchestration integration was added in this chunk; persistence is projection-layer only.

### Implemented

- Added canonical snapshot domain types and validation:
  - `runtime/domain/execution_snapshot_types.go`
- Added snapshot projection service with deterministic ordering and validation:
  - `runtime/execution/snapshot_service.go`
- Added snapshot persistence model, idempotent writes, and deterministic queries:
  - `runtime/execution/snapshot_store.go`
- Added snapshot models to durable migration path:
  - `runtime/execution/database_store.go`
- Added chunk-local unit/SQLite coverage for both services and schema behavior:
  - `runtime/domain/execution_snapshot_types_test.go`
  - `runtime/execution/snapshot_service_test.go`
- Updated chunk checklist status in spec task file:
  - `openspec/changes/add-durable-paper-backtest-audit/tasks.md`

### TDD coverage checks

- Position projection tests cover:
  - event-time + fill-id sorting,
  - open/increase/reduce/flatten for long and short,
  - unsupported reversal,
  - realized/exposure math,
  - source fill linkage,
  - UTC-normalized event time,
  - SQLite-backed query filtering (strategy/instrument/mode/time).
- Portfolio projection tests cover:
  - gross/net exposure,
  - realized PnL aggregation,
  - optional unrealized PnL only when marks supplied,
  - metadata signaling for unrealized model assumptions,
  - SQLite schema/index verification,
  - mode/time filtering behavior.
- Snapshot metadata projection for position/portfolio now explicitly documents model assumptions used by deterministic projection/simulator assumptions.

### Determinism and minimality review

- Projection order: position snapshots are explicitly sorted by `(event_time ASC, fill_id ASC)` prior to fold.
- Persistence/write behavior: both snapshot stores are idempotent (`ON CONFLICT snapshot_id DO NOTHING`).
- Queries return stable sort keys (`ORDER BY event_time ASC, snapshot_id ASC` and strategy/strategy filters are scoped and deterministic).
- Schema is narrow (position + portfolio snapshot tables only) and includes only fields required by chunk-4 tasks.

### Verification results

Executed checks:

- `go test ./runtime/execution -run TestSnapshotService -count=1`
- `go test ./runtime/domain -run TestExecutionSnapshotTypes -count=1`
- `go test ./runtime/execution ./runtime/domain -count=1`
- `make affected-lint-test`

All executed commands passed.

### Findings

- No blocking issues found for chunk scope.
- Portfolio aggregation determinism now enforced by sorting open-position keys before reduction, so floating-point fold order is fixed and repeatable.

### Completion protocol status

- Scope conformance: ✅ passed (`4.1`, `4.2` addressed).
- Determinism/minimality: ✅ passed (position/portfolio projection points are explicitly sorted and writes are idempotent).
- Lint/test: ✅ pass (`make affected-lint-test` clean).
- OpenSpec consistency: ✅ passed (`tasks.md` and this review artifact updated).

### Continue decision

- Safe to continue to next chunk: `backtest-run-evaluation-report-scaffold`.

### Commit status

- Review artifact created for this chunk (`review-chunk-paper-snapshots.md`).
- Commit created for this chunk (`3c2f5fc`).
