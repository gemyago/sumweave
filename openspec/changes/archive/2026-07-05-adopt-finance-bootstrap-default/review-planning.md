# Planning Review

## Review Round 1 — 2026-07-05

- Reviewer: OpenSpec planning review worker
- Result: needs-follow-up
- Validation: `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` ✓ passed

### What I reviewed

- `proposal.md`
- `design.md`
- `tasks.md`
- `README.md`
- `.openspec.yaml`
- `specs/finance-operator-ui/spec.md`
- `specs/signal-ui-bootstrap-rails/spec.md`
- `manager-status.md`
- Active overlap artifacts for `restructure-finance-ui-shell`: `proposal.md`, `design.md`, `tasks.md`, `manager-status.md`, and `review-planning.md`

### Verdict summary

The plan matches the requested product direction: Bootstrap becomes canonical for login and tenant-facing `#/finance*` routes, successful login/default authenticated routing lands on `#/finance`, parallel pilot naming is retired, non-finance surfaces stay on the existing stack, and backend/API work is explicitly out of scope unless a true contract mismatch is discovered. OpenSpec strict validation passes.

The plan is not implementation-ready yet because the active overlap with `restructure-finance-ui-shell` is identified but not resolved into an actionable sequencing decision before implementation.

### Required correction

1. Resolve the active overlap before implementation sequencing.
   - `proposal.md` says `restructure-finance-ui-shell` should be reconciled or superseded before implementation sequencing, and `design.md` repeats that this must be decided before assigning chunks.
   - The same `design.md` also says there are no blockers, and `tasks.md` does not include or point to the required resolution.
   - The overlap is material: `restructure-finance-ui-shell` planned and reportedly implemented a custom finance-shell direction that preserves the existing design-system foundations, while this change promotes Bootstrap as the canonical Finance and login styling contract.
   - Update the plan/status to make the sequencing explicit, for example by stating that `adopt-finance-bootstrap-default` supersedes the custom-shell styling direction while reusing any behavior-only shell/tenant lessons, or by requiring the manager/user to close or archive the older change before implementation starts.

### Additional follow-up worth resolving before implementation

- Documentation work is split between task 3.2 (`ui-wireframe.md`) and task 5.1 (rules/manual smoke docs). This can be acceptable if 3.2 is only route/navigation documentation tied to that implementation slice, but the boundary should be clearer so future implementers do not duplicate or defer user-visible route documentation unexpectedly.
- Task 5.1 describes a documentation-only task as following a TDD flow. Consider rewording it as a documentation-first acceptance step rather than a TDD task, unless a concrete docs validation test is intended.

### Scope and consistency check

- `proposal.md` is appropriately authoritative and bounded to canonical login plus tenant-facing Finance surfaces.
- `design.md` follows the proposal and correctly excludes Chat, Data, generic Jobs, Providers, Strategies, Evaluations, Admin, broader app Bootstrap conversion, Go/backend/OpenAPI changes, wrappers, and fake unsupported workflows.
- `tasks.md` covers the proposal/design commitments across login/routing, shared Bootstrap Finance shell, tenant context, dashboard, remaining Finance route groups, route tests, page tests, visual verification, and documentation.
- The specs align with the proposal/design and enumerate the canonical route set, Bootstrap-first styling rules, default Finance landing, tenant-shell behavior, dashboard expectations, management routes, and transaction browse/editor behavior.

### Chunk plan

- Keep implementation sequential and preserve parent-task order.
- Do not combine non-consecutive parent tasks.
- Recommended chunks after the overlap correction:
  1. `canonical-login-routing`: parent task 1 only.
  2. `bootstrap-finance-shell-foundation`: parent task 2 only.
  3. `canonical-finance-dashboard-navigation`: parent task 3 only.
  4. `remaining-finance-route-surfaces`: parent task 4 only.
  5. `rules-manual-e2e-documentation`: parent task 5 only.
- If implementation proves small, tasks 1 and 2 may be combined because they are consecutive entry/shell foundation work, but default to separate chunks to limit route-wide churn.

### Artifact cleanup

- No ad-hoc cleanup issue found in `openspec/changes/adopt-finance-bootstrap-default`.
- The change directory contains standard OpenSpec artifacts plus standard manager/review artifacts only.

### Commit status

- No commit created because the plan needs follow-up before the planning gate is clean.

## Review Round 2 — 2026-07-05

- Reviewer: OpenSpec planning review worker
- Result: complete
- Validation: `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` ✓ passed

### What I reviewed

- `proposal.md`
- `design.md`
- `tasks.md`
- `README.md`
- `.openspec.yaml`
- `specs/finance-operator-ui/spec.md`
- `specs/signal-ui-bootstrap-rails/spec.md`
- `manager-status.md`
- Existing `review-planning.md` round 1 feedback

### Verdict summary

The plan is implementation-ready. The round 1 blocker is resolved: the proposal, design, tasks, and manager status now consistently state that `adopt-finance-bootstrap-default` supersedes `restructure-finance-ui-shell` for Finance/login styling, while permitting behavior-only reuse for shell route coverage, tenant context, route preservation, supported navigation mapping, and avoiding dead routes.

The previous documentation ambiguity is also resolved. Task 3.2 is limited to route/navigation documentation in `ui-wireframe.md` alongside the navigation implementation slice, while task 5.1 is a post-slice documentation acceptance step covering rules, design docs, manual e2e guides, and remaining non-route acceptance text. The docs-only task no longer claims a TDD flow.

### Scope and consistency check

- `proposal.md` remains authoritative and bounded to canonical Bootstrap login plus tenant-facing `#/finance*` surfaces, with non-finance surfaces and backend/API contracts explicitly out of scope.
- `design.md` follows the proposal and clearly resolves the active OpenSpec overlap in favor of this change.
- `tasks.md` covers the implementation commitments in strict parent-task order and keeps the supersession decision visible to implementers.
- The specs align with the proposal/design and cover canonical login, default Finance landing, Bootstrap-first Finance shell/routes, dashboard, management routes, transactions, tenant context, styling rails, and docs/rules updates.

### Chunk plan

- Keep implementation sequential and preserve parent-task order.
- Do not combine non-consecutive parent tasks.
- Recommended chunks:
  1. `canonical-login-routing`: parent task 1 only.
  2. `bootstrap-finance-shell-foundation`: parent task 2 only.
  3. `canonical-finance-dashboard-navigation`: parent task 3 only.
  4. `remaining-finance-route-surfaces`: parent task 4 only.
  5. `rules-manual-e2e-documentation`: parent task 5 only.
- If implementation turns out smaller than expected, tasks 1 and 2 may be combined because they are consecutive entry/shell foundation work, but the default should remain separate chunks.

### Artifact cleanup

- No ad-hoc cleanup issue found in `openspec/changes/adopt-finance-bootstrap-default`.
- The change directory contains standard OpenSpec artifacts plus standard manager/review artifacts only.

### Commit status

- Pending commit at review-write time; per review rules, planning/review/status artifacts should be committed after this clean review entry is written.

## Review Round 3 — 2026-07-05 Final-review correction planning

- Reviewer: OpenSpec planning worker
- Result: complete
- Validation: `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` ✓ passed; `openspec status --change adopt-finance-bootstrap-default` reports 4/4 artifacts complete.

### Final-review blockers addressed in the plan

1. Legacy `#/v2/*` finance/login route surface was preserved as compatibility-only despite the no-v2/no-pilot direction.
2. Responsive Finance shell docs described a narrow-screen menu toggle that the implemented Bootstrap shell does not provide.

### Correction chunk plan

- Use one focused follow-up implementation chunk: `final-review-route-doc-corrections`.
- Scope is task 6 only:
  - 6.1 retire the remaining legacy `#/v2/login` and `#/v2/finance` route surface, tests, protected-destination handling, and compatibility-only docs/rules.
  - 6.2 align responsive Finance shell docs/manual smoke acceptance to the actual stacked Bootstrap narrow layout, with responsive smoke evidence.
- Do not reopen broader Bootstrap conversion, backend/API work, or non-finance UI conversion.
- Do not implement a new responsive menu toggle unless the current stacked shell fails the responsive smoke acceptance.

### Affected components/directories

- `apps/signal-ui/src/App.svelte`
- `apps/signal-ui/src/lib/routing/post-login-destination.ts`
- `apps/signal-ui/src/App.test.ts`
- `apps/signal-ui/src/lib/routing/post-login-destination.test.ts`
- legacy V2-specific login/Finance tests if still present
- `apps/signal-ui/AGENTS.md`
- `apps/signal-ui/ui-wireframe.md`
- `docs/manual-e2e/finance-ui-shell-smoke-e2e.md`
- this OpenSpec change directory

### Artifact cleanup

- No new ad-hoc artifacts should be created.
- Durable evidence for the correction chunk should go in `review-chunk-final-review-route-doc-corrections.md`.
