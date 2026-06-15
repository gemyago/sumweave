# Chunk Review: backtest-run-and-evaluation-report-scaffold

Implementation and review history for chunk `5.1` and `5.2` (`backtest-run-and-evaluation-report-scaffold`).

## 2026-06-14 Chunk-5 verification pass

Verdict: **PASS** for chunk scope.

### Scope alignment

- Confirmed changes are constrained to the chunk-5 scope (`5.1`, `5.2`): durable backtest lifecycle scaffold + evaluation report scaffold.
- Files are limited to:
  - `runtime/backtest/service.go`
  - `runtime/backtest/database_store.go`
  - `runtime/backtest/service_test.go`
  - `runtime/domain/backtest_types.go`
  - `openspec/changes/add-durable-paper-backtest-audit/tasks.md`
- No API route, UI, or flow orchestration glue was added.
- No earlier chunk behavior is modified in this scope (append-only for this chunk’s domain).

### Implemented

- Added compact dataset reference domain model + validation/canonicalization:
  - `runtime/domain/backtest_types.go`
- Added durable `BacktestRun` domain model + lifecycle/status enums, transitions validation helpers, and compact versioned metrics schema.
- Added `EvaluationReport` domain model + supported decision enum + compact metrics subset support.
- Implemented service-level orchestration for chunk-5 persistence and transitions:
  - `runtime/backtest/service.go`
- Implemented SQLite/GORM-backed persistence layer with explicit table and column names:
  - `runtime/backtest/database_store.go`
- Added scaffold tests covering:
  - dataset reference persistence + UTC timestamp behavior,
  - backtest lifecycle transitions (`pending -> running -> completed/failed`) + immutability via reloaded updates,
  - deterministic backtest report query behavior,
  - evaluation report derivable-metric assembly and metric omission behavior,
  - max-drawdown calculation from ordered portfolio snapshots,
  - explicit sqlite schema/unique indexes.

### TDD and verification checks

- `runtime/backtest/service_test.go` contains explicit assertions matching chunk requirements and includes checks for:
  - deterministic ordering,
  - UTC normalization,
  - forbidden transitions,
  - minimal/derived metric handling,
  - unique index existence and record counts.
- Executed:
  - `go test ./runtime/backtest ./runtime/domain -count=1`
  - `make affected-lint-test`

All executed commands passed.

### Determinism and minimality review

- Deterministic query ordering is explicit:
  - backtest runs: `ORDER BY created_at ASC, run_id ASC`,
  - evaluation reports: `ORDER BY created_at ASC, evaluation_id ASC`.
- Deterministic JSON assembly:
  - portfolio snapshots are sorted by event time + snapshot id before max-drawdown calculation,
  - decision/fill/gov metric inclusion is computed directly from passed evidence.
- Minimal metric policy is followed:
  - only compact schema fields and derivable metrics are persisted,
  - unsupported metrics are omitted (nil pointers preserved through persistence round-trip).

### Findings

- No blocking findings identified for this chunk scope.
- No cleanup required for scope-specific regressions.

### Completion protocol status

- Scope conformance: ✅ passed (`5.1`, `5.2` satisfied)
- Determinism/minimality: ✅ passed
- OpenSpec consistency: ✅ passed (`tasks.md` and this review file updated)
- Lint/test: ✅ passed (`make affected-lint-test` clean)

### Continue decision

- Safe to continue to next chunk: `backtest-flow-integration`.

### Commit status

- Review artifact created (`review-chunk-backtest-report-scaffold.md`).
- No commit created yet for this chunk (`none`).

## Post-commit status

- Commit created after review: `bc961f3` (`Add backtest report scaffold`)
