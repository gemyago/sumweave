# Planning Review

## Review Round 1 — 2026-07-04

- Reviewer: OpenSpec planning review worker
- Result: needs-follow-up
- Validation: `direnv exec /Users/jenya/projects/signal-foundry openspec validate restructure-finance-ui-shell --strict` ✓ passed

### What I reviewed

- `proposal.md`
- `design.md`
- `tasks.md`
- `specs/finance-operator-ui/spec.md`
- `docs/manual-e2e/finance-ui-shell-smoke-e2e.md`
- `docs/manual-e2e/README.md`
- `manager-status.md`

### Verdict summary

The plan is directionally strong and matches the requested finance-first shell, dashboard hierarchy, transactions ledger workspace, dedicated create/edit routes, and manual smoke loop. OpenSpec validation passes.

However, the plan is not implementation-ready yet because the task/spec coverage still has a few planning gaps.

### Required corrections

1. Rework the standalone final verification task.
   - `tasks.md` item `4.2` is a separate post-implementation verification task.
   - Repo/OpenSpec constraints for this change require tests and verification to be integrated into implementation tasks rather than added as a final phase.
   - Keep the smoke runbook, but fold the sub-agent run/report/fix/rerun loop into the relevant implementation tasks or their acceptance wording instead of leaving it as its own final task.

2. Add explicit dashboard coverage for missing-FX diagnostics.
   - The spec still requires missing-FX diagnostics on `#/finance`.
   - `tasks.md` 2.1 and 2.2 cover KPI/cards/sections and existing data sources, but they do not explicitly preserve or test missing-FX diagnostics or mention the related source coverage.
   - Update the tasks so dashboard implementation and tests clearly retain this requirement.

3. Close the finance-shell route coverage gap.
   - `proposal.md` and `design.md` describe a dedicated shell for `#/finance*`, but `tasks.md` 1.1 only proves shell chrome for a subset of finance routes.
   - Expand planning coverage for the remaining supported finance routes that are easy to miss during shell refactors, especially dedicated detail/editor routes such as `#/finance/accounts/:accountId`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/jobs/:jobId`, and clarify whether `#/finance/connections/synthetic` also uses the same shell.
   - If any of those routes are intentionally excluded, narrow the proposal/design/spec wording so the shell scope is unambiguous.

### Additional follow-up worth resolving before implementation

- `design.md` still leaves navigation labeling and dashboard-search behavior open. Those choices should be explicitly decided before implementation so reviewers and implementers share the same acceptance target.

### Chunk plan

- Sequential parent-task execution remains the right approach.
- Suggested implementation order after corrections:
  1. Finance Shell Foundation
  2. Finance Dashboard Composition
  3. Transactions Browse Workspace
  4. Documentation updates only where they are integrated with the corresponding implementation slices
- Do not split or parallelize further at planning time.

### Artifact cleanup

- No ad-hoc cleanup issue found in the reviewed change artifacts.

### Commit status

- No commit created, per user instruction.

## Review Round 2 — 2026-07-04

- Reviewer: OpenSpec planning review worker
- Result: complete
- Validation: `direnv exec /Users/jenya/projects/signal-foundry openspec validate restructure-finance-ui-shell --strict` ✓ passed

### What I reviewed

- `proposal.md`
- `design.md`
- `tasks.md`
- `specs/finance-operator-ui/spec.md`
- `docs/manual-e2e/finance-ui-shell-smoke-e2e.md`
- `docs/manual-e2e/README.md`
- `README.md`
- `manager-status.md`
- prior review in `review-planning.md`

### Verdict summary

Round 1 blockers are resolved. The plan now integrates the smoke run/report/fix/rerun loop into the implementation tasks, explicitly covers dashboard missing-FX diagnostics in task/spec acceptance, includes the full finance-shell route scope across list/detail/editor/job/synthetic routes, and closes the prior design decisions around finance rail labeling, dashboard search behavior, and the transactions inspector behavior.

The change is planning-ready for implementation.

### Correction check

1. Standalone final verification task
   - Resolved.
   - `tasks.md` no longer has a detached final verification phase; shell, tenant, dashboard, and transactions tasks each absorb the relevant smoke loop work.

2. Dashboard missing-FX diagnostics
   - Resolved.
   - `tasks.md` 2.1 and the spec dashboard scenario now explicitly retain missing-FX diagnostics.

3. Full `#/finance*` shell scope
   - Resolved.
   - `tasks.md` 1.1 and the spec route scenario now explicitly cover account detail, transaction create/edit, job detail, and synthetic setup routes inside the finance shell.

4. Navigation/search/inspector decisions
   - Resolved.
   - `design.md` now names the supported finance rail destinations, rejects inert dashboard-wide search, and defines the transactions inspector as a selected-row contextual panel on wide screens with route handoff for full edits.

### Chunk plan

- Keep parent tasks sequential in existing order: 1 → 2 → 3 → 4.
- Smoke verification should happen inside tasks 1-3 per the revised task wording; task 4 remains documentation alignment only.

### Artifact cleanup

- No ad-hoc cleanup issue found in the reviewed change artifacts.

### Commit status

- No commit created, per user instruction.
