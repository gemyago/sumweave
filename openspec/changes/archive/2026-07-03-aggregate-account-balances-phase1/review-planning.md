# Planning Review

## Review Round 1 — 2026-07-03

- Reviewer: openspec-plan-reviewing
- Scope reviewed: `proposal.md`, `design.md`, `tasks.md`, `specs/finance-management/spec.md`, `manager-status.md`
- Verdict: needs-follow-up
- Ready for implementation: no

### Summary

The proposal and design are aligned on the core phase-1 shape: add a SQL aggregate balance read path, use it for account list/detail and standalone account-balance reads, and reuse it for dashboard account-balance reads where appropriate. The main planning gap is in `tasks.md`: it does not turn the proposal/design manual API-level verification requirement into a concrete implementation task.

### Findings

1. Missing task coverage for the required manual API-level verification flow.
   - `proposal.md` explicitly requires "an API-level manual e2e verification flow".
   - `design.md` defines a detailed verification procedure, including migration, PM2 startup, multi-account setup, mixed transaction kinds/statuses, list/detail checks, issue capture, fixes, and reruns.
   - `tasks.md` does not include that deliverable. Task `3.1` only restates generic TDD/lint/test completion behavior.
   - Required follow-up: add an explicit verification task covering the manual API-level flow from the design, or rewrite `3.1` so it clearly includes that concrete flow instead of only process reminders.

### Alignment Notes

- `proposal.md` and `design.md` are otherwise aligned on scope and non-goals.
- `tasks.md` correctly covers:
  - aggregate read-path creation for one-account and many-account reads
  - preserved balance semantics across transaction kinds/statuses
  - wiring account-list, account-detail, and standalone balance reads
  - HTTP/OpenAPI exposure of `bookedBalanceMinor` and `pendingBalanceMinor`
  - dashboard balance-read reuse as a separate follow-on step
- No non-consecutive scattered parent-task work was found.

### Chunk Plan

- Chunk 1: Parent task `1` only (`1.1`-`1.3`)
  - Reason: core aggregate read path, service wiring, and API exposure are tightly related and should land sequentially.
- Chunk 2: Parent task `2` only (`2.1`)
  - Reason: dashboard alignment depends on the aggregate read path from chunk 1 and may require cutoff-aware reuse.
- Chunk 3: Parent task `3` only (`3.1` after task coverage is fixed)
  - Reason: verification should stay last and should include the explicit manual API-level flow.

### Artifact Cleanup Check

- Clean.
- Present artifacts are standard OpenSpec change artifacts plus `specs/finance-management/spec.md`.
- `review-planning.md` was missing before this run and is now created as the standard planning review artifact.

### Commit Status

- No commit created.
- Reason: plan is not yet clean/ready for implementation because task coverage needs follow-up.

## Review Round 2 — 2026-07-03

- Reviewer: openspec-plan-reviewing
- Scope reviewed: `proposal.md`, `design.md`, `tasks.md`, `specs/finance-management/spec.md`, `manager-status.md`
- Verdict: complete
- Ready for implementation: yes

### Summary

Follow-up re-review is clean. The updated `tasks.md` now carries the manual API-level verification flow required by the proposal and defined in the design, and the proposal, design, tasks, and spec remain aligned on a bounded phase-1 aggregate-balance read implementation.

### Findings

- None.

### Alignment Notes

- `proposal.md` remains clear and bounded on phase-1 scope: SQL aggregate balance reads, account-list/detail balance exposure, standalone balance-read reuse, preserved ledger semantics, and no stored/materialized balances.
- `design.md` follows that scope and keeps implementation simple, including optional/minimal index consideration and dashboard reuse only where cutoff-aware semantics still fit.
- `tasks.md` now fully covers proposal and design commitments, including the explicit manual API-level e2e verification flow in task `3.1`.
- `specs/finance-management/spec.md` matches the planned account-list and account-detail balance behavior and tenant-scope requirements.
- No non-consecutive scattered parent-task work was found.

### Chunk Plan

- Chunk 1: Parent task `1` only (`1.1`-`1.3`)
  - Reason: the aggregate read path, service wiring, and HTTP/OpenAPI exposure are one tightly related implementation sequence.
- Chunk 2: Parent task `2` only (`2.1`)
  - Reason: dashboard reuse depends on chunk 1 and should stay as the next focused follow-on step.
- Chunk 3: Parent task `3` only (`3.1`)
  - Reason: verification should remain last after the implementation slices are complete.

### Artifact Cleanup Check

- Clean.
- Present artifacts are standard OpenSpec change artifacts plus `specs/finance-management/spec.md`.
- No ad-hoc repository artifacts were found in the change directory.

### Commit Status

- Commit created for the clean planning artifacts in this change directory.
