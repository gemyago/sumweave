# Planning Review

Planning review history for `implement-data-layer-v0`.

## 2026-06-12 Full review

Verdict: needs revision before implementation.

### Findings

1. Resolve the v0 scope and ingestion contract before implementation starts.
   - `design.md` still leaves the first record set open (`candles only, trades only, or both`) and whether ingestion may upsert instruments implicitly.
   - `tasks.md` and `specs/data-layer/spec.md` already assume both candles and trades, plus a `known or upsertable instrument` path.
   - This leaves parent tasks 2 through 4 underspecified and risks rework across validation, storage keys, and app wiring.

2. Define replay/query range semantics explicitly in the spec and design.
   - `tasks.md` says replay methods must respect inclusive/exclusive boundaries defined by the package.
   - `specs/data-layer/spec.md` only says records inside the range are included and outside are excluded, which does not tell implementers whether the contract is `[start,end)`, `[start,end]`, or something else.
   - Deterministic replay depends on that choice, especially for candles versus trades and for stable repeated reads/tests.

### Chunking recommendation

- Complexity: moderate to high; this is a new runtime slice plus database persistence and backend startup wiring, so later tasks depend directly on earlier contracts and schema choices.
- Sequencing: sequential; each parent task builds on the previous one and the open planning issues affect downstream tasks.
- `chunk1`: parent task 1 (`1.1-1.3`) -> sub-agent 1 -> slug `shared-domain-foundation`
- `chunk2`: parent task 2 (`2.1-2.3`) -> sub-agent 2 -> slug `runtime-data-contracts`
- `chunk3`: parent task 3 (`3.1-3.3`) -> sub-agent 3 -> slug `gorm-persistence`
- `chunk4`: parent task 4 (`4.1-4.4`) -> sub-agent 4 -> slug `deterministic-query-replay`
- `chunk5`: parent task 5 (`5.1-5.4`) -> sub-agent 5 -> slug `backend-app-wiring`
- `chunk6`: parent task 6 (`6.1-6.4`) -> sub-agent 6 -> slug `docs-and-verification`

### Artifact cleanup

- Clean. Present files are proposal/design/tasks/spec plus standard manager/review artifacts only.

### Commit gate

- No commit yet. The plan is not clean and ready for implementation because the findings above should be resolved first.

## 2026-06-12 Planning revision follow-up

Verdict: findings addressed; planning artifacts are ready for implementation.

### Scope

- Change slug: `implement-data-layer-v0`

### Triggering input

- User requested an in-place OpenSpec revision using this persisted planning review and the recorded manager guidance.

### Addressed findings

1. Resolved v0 scope explicitly to include both candles and trades across `proposal.md`, `design.md`, `tasks.md`, and `specs/data-layer/spec.md`.
2. Resolved the ingestion contract explicitly to allow instrument upsert by venue and symbol from normalized candle/trade records.
3. Defined deterministic query and replay ranges as explicit `[start, end)` semantics and propagated that boundary contract into tasks and spec scenarios.

### Continue decision

- Continue with implementation in the existing chunk order from this review.

### Affected follow-up chunks

- `shared-domain-foundation`
- `runtime-data-contracts`
- `gorm-persistence`
- `deterministic-query-replay`
- `backend-app-wiring`
- `docs-and-verification`

### Completion protocol status

- Non-coding artifact revision complete.

### Artifact cleanup status

- Clean. Only standard OpenSpec and review/status artifacts are present.

### Commit status

- No commit created in this revision task.

## 2026-06-12 Follow-up re-review

Verdict: clean and ready for implementation.

### Re-review focus

- Re-checked the previous scope/ingestion-contract finding against `proposal.md`, `design.md`, `tasks.md`, and `specs/data-layer/spec.md`.
- Re-checked the previous replay/query range finding against the same planning artifacts.
- Did a lighter pass for obvious new planning issues, task ordering, chunking, and artifact hygiene.

### Outcome

1. The v0 scope is now explicit and internally consistent: the change covers instruments plus both candles and trades, and ingestion may upsert instruments by venue and symbol from normalized market-data records.
2. Deterministic query and replay semantics are now explicit and propagated consistently as `[start, end)` boundaries, including the candle-start and trade-event timestamps used for filtering and ordering.
3. `tasks.md` is complete and logically ordered for implementation: shared domain first, then slice contracts, persistence, deterministic reads/replay, backend wiring, and final verification.
4. Chunking remains clean as one parent task per implementation chunk in existing parent-task order; no related work is scattered across non-consecutive parent tasks in a way that needs replanning.

### Chunking confirmation

- Complexity: moderate to high; this is still a new runtime slice plus persistence and backend wiring, but the blocking planning ambiguities have been resolved.
- Sequencing: sequential; each parent task still depends on contracts and schema choices established by earlier tasks.
- `chunk1`: parent task 1 (`1.1-1.3`) -> sub-agent 1 -> slug `shared-domain-foundation`
- `chunk2`: parent task 2 (`2.1-2.3`) -> sub-agent 2 -> slug `runtime-data-contracts`
- `chunk3`: parent task 3 (`3.1-3.3`) -> sub-agent 3 -> slug `gorm-persistence`
- `chunk4`: parent task 4 (`4.1-4.4`) -> sub-agent 4 -> slug `deterministic-query-replay`
- `chunk5`: parent task 5 (`5.1-5.4`) -> sub-agent 5 -> slug `backend-app-wiring`
- `chunk6`: parent task 6 (`6.1-6.4`) -> sub-agent 6 -> slug `docs-and-verification`

### Artifact cleanup

- Clean. Present files are limited to standard OpenSpec artifacts plus standard manager/review artifacts.

### Commit status

- Pending commit at this review gate because the plan is clean and the review/status artifacts changed in this re-review.
