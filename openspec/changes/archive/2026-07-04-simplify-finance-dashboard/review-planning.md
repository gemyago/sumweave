# Planning Review

## Round 1

- Date: 2026-07-04
- Reviewer: OpenSpec plan-reviewing worker
- Result: needs-follow-up
- Ready for implementation: No

### Verdict

- `proposal.md` is clear and bounded for a UI-only dashboard simplification.
- `design.md` follows the proposal and keeps the scope inside existing finance UI/data constraints.
- `tasks.md` is mostly sequenced in parent-task order, but the plan is not yet ready for implementation because two planning gaps remain.

### Findings

1. Active-tenant route coverage is underspecified in `tasks.md` versus the spec delta.
   - `specs/finance-operator-ui/spec.md` requires the shared active tenant to carry across tenant-scoped finance routes and finance-context deep links, including Accounts, Transactions, Categories, Connections, Imports, and Jobs routes.
   - `tasks.md` 1.1 and 1.2 only explicitly prove `#/finance` entry and dashboard reload behavior.
   - As written, implementation could satisfy the task list while still missing cross-route persistence and deep-link behavior required by the spec.
   - Fix by adding task coverage under parent task 1 for shared tenant-context reuse across representative non-dashboard routes and direct deep-link entry, or by narrowing the spec if that broader behavior is not intended in this change.

2. The plan makes durable artifacts depend on a temporary repo asset path.
   - `tasks.md` uses `tmp/dashboard-redesign/reference-moneta-dashboard.png` as the global reference and task 4.2 asks durable docs to explicitly name that tmp path.
   - `tmp/` is project-scoped temporary storage, not a durable documentation or OpenSpec artifact location.
   - This is acceptable as a working input during planning, but it should not become a required durable doc reference.
   - Fix by moving the image to a stable docs/design asset location or by converting the visual guidance into durable written acceptance criteria inside the OpenSpec/design artifacts.

### Chunk Plan

Strict ordered implementation chunks, preserving parent-task order:

1. Chunk 1: parent task 1 only — Tenant and shell simplification.
2. Chunk 2: parent task 2 only — Dashboard information architecture.
3. Chunk 3: parent task 3 only — Attention and diagnostics.
4. Chunk 4: parent task 4 only — Responsive/doc/verification follow-through.

Notes:

- This chunking is implementation-safe because it keeps all parent tasks consecutive and unsplit.
- Do not combine non-consecutive parent tasks.
- If tenant-context coverage is expanded, keep that added work inside parent task 1 rather than scattering it into later chunks.

### Artifact Cleanup Status

- OpenSpec change directory cleanup: clean. Only standard planning/review artifacts and the expected spec delta are present.
- External artifact check: follow-up needed. `tmp/dashboard-redesign/reference-moneta-dashboard.png` exists outside the OpenSpec change directory and is currently treated as a planning dependency.
- Cleanup verdict: not fully clean until the durable plan/docs stop depending on a tmp-path artifact or that artifact is relocated to a durable location.

### Commit Status

- No commit created.
- Reason: the plan is not yet clean/ready for implementation, so the planning commit gate was not met.

## Round 2

- Date: 2026-07-04
- Reviewer: OpenSpec plan-reviewing worker
- Result: complete
- Ready for implementation: Yes

### Verdict

- The previous planning gaps are now closed.
- `proposal.md`, `design.md`, `tasks.md`, and the spec delta are aligned on a UI-only dashboard simplification with shell-owned tenant context.
- `tasks.md` now preserves parent-task order cleanly and is ready for implementation.

### Re-review Findings

1. Cross-route active-tenant and deep-link coverage is now explicit.
   - `proposal.md` now includes shell-owned active-tenant consistency across tenant-scoped finance routes and direct finance deep links.
   - `design.md` decision 2 and migration step 2 now explicitly call for reuse across the listed finance routes, including direct deep-link entry.
   - `tasks.md` 1.3 now adds direct test-and-implement coverage for shared active-tenant reuse across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId`.
   - This closes the previous spec-to-task coverage gap.

2. The durable tmp-path dependency is removed.
   - `tasks.md` no longer makes the plan depend on `tmp/dashboard-redesign/reference-moneta-dashboard.png`.
   - `design.md` now records durable written visual acceptance criteria.
   - `tasks.md` 4.2 and 4.3 now point implementation and docs to written acceptance criteria instead of requiring durable references to a temporary repo asset.
   - This closes the previous artifact-durability gap.

3. No new blocking planning issues were found in the updated artifacts.

### Chunk Plan

Strict ordered implementation chunks, preserving parent-task order:

1. Chunk 1: parent task 1 only — Tenant and shell simplification, including cross-route active-tenant and deep-link behavior.
2. Chunk 2: parent task 2 only — Dashboard information architecture.
3. Chunk 3: parent task 3 only — Attention and diagnostics.
4. Chunk 4: parent task 4 only — Responsive/doc/verification follow-through.

Notes:

- Parent-task order is preserved.
- No parent task needs splitting.
- No non-consecutive parent tasks should be combined.

### Artifact Cleanup Status

- OpenSpec change directory cleanup: clean. Only standard planning/review artifacts and the expected spec delta are present.
- External artifact check: acceptable for planning gate. `tmp/dashboard-redesign/reference-moneta-dashboard.png` still exists as a temporary working asset, but the durable OpenSpec plan no longer depends on that tmp path.
- Cleanup verdict: clean enough for implementation planning and commit.

### Commit Status

- Pending planning artifacts should be committed because the plan is now clean and ready for implementation.
