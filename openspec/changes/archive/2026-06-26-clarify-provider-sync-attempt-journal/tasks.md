## 1. Update provider sync state contract and orchestration

- [x] 1.1 Update `finance/domain` and `finance/internal/providers` so the journal seam loads the latest state rather than the latest succeeded state, rename `SuccessfulWindow` to `Window`, and do it with TDD flow: write or update failing tests first, then implement the contract and field rename, then verify the focused finance tests.
- [x] 1.2 Update orchestrator tests and implementation so planning happens before chunk execution, target-window policy receives the latest loaded state directly, and each chunk builds its own concrete state with the exact attempted window only during execution, and do it with TDD flow: write failing tests first, then implement, then verify the focused finance tests.

## 2. Update persistence journal semantics

- [x] 2.1 Update `finance/persistence` journal tests to require latest-state loading, required attempt-window columns, explicit failed-row round trips, and connection-scoped latest-row selection, and do it with TDD flow: write failing tests first, then implement the renamed store/model behavior, then verify the focused finance tests.
- [x] 2.2 Rename journal persistence fields from successful-window naming to neutral window naming, keep the journal append-only, and do it with TDD flow: write failing tests first, then implement the schema/model mapping changes, then verify the focused finance tests.

## 3. Update finance sync documentation and spec language

- [x] 3.1 Update the finance provider sync architecture doc and any adjacent terminology to describe the journal as latest-attempt state rather than succeeded-only snapshots, and do it with TDD flow where applicable: add or update any doc-linked verification first, then implement the documentation/spec wording changes, then verify the affected checks.
