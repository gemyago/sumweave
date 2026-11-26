---
type: Historical record
title: "2026-07-05-adopt-finance-bootstrap-default: review chunk canonical finance dashboard navigation"
description: "Historical review chunk canonical finance dashboard navigation record for 2026-07-05-adopt-finance-bootstrap-default."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-05-adopt-finance-bootstrap-default/review-chunk-canonical-finance-dashboard-navigation.md"
    title: "Original openspec/changes/archive/2026-07-05-adopt-finance-bootstrap-default/review-chunk-canonical-finance-dashboard-navigation.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

# Canonical Finance Dashboard Navigation — implementation review log

## Round 1 — 2026-07-05

- Phase: initial implementation phase
- Result: complete
- Scope completed:
  - promote the Bootstrap dashboard composition to canonical `#/finance`
  - keep dashboard summaries balance-first with honest loading, empty, and error states
  - keep canonical Finance navigation/docs on `#/finance*` links without pilot naming
  - update chunk task/status artifacts for parent task 3
- `openspec apply` note:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-finance-bootstrap-default --task 3.1 --task 3.2`
  - current CLI still fails with `unknown command 'apply'`

### What changed

- Added a Bootstrap-first canonical dashboard composition for `#/finance` and switched the canonical route component to that composition without reintroducing custom route-local CSS.
- Kept the dashboard derived from the existing finance dashboard, transactions, and connections APIs while emphasizing the booked-balance story, reporting-period controls, capped account/category/recent-transaction sections, and compact attention diagnostics including missing-FX follow-up.
- Updated dashboard and shell navigation tests to assert canonical `#/finance*` links, absence of pilot naming in supported Finance navigation, and honest loading/tenant-required/empty/error behavior for the canonical dashboard.
- Updated the route/navigation portions of `apps/signal-ui/ui-wireframe.md`, plus OpenSpec tasks and manager status, to reflect the canonical Bootstrap Finance dashboard and navigation entry points.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-finance-bootstrap-default --task 3.1 --task 3.2` *(fails: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry npx vitest run src/pages/Finance.test.ts src/components/FinanceShell.test.ts src/App.test.ts` ✓
- `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` ✓
- `direnv exec /Users/jenya/projects/signal-foundry openspec status --change adopt-finance-bootstrap-default` ✓
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` ✓
- Manual smoke via `playwright-cli` against `http://127.0.0.1:5173` ✓
  - canonical login still lands on `#/finance`
  - canonical dashboard renders on desktop and mobile with balance-first summaries visible
  - dashboard action links still reach canonical `#/finance/accounts` and the period controls remain interactive

### UI verification

- Visual review artifacts saved under `tmp/ui-design-review/20260705-canonical-finance-dashboard-navigation/`.
- Desktop and mobile dashboard screenshots plus live snapshots looked clean; no layout, spacing, or wrapping issues were found in this chunk scope.

### Remaining follow-up

- Later chunks still own the remaining canonical Finance route-surface conversions and broader rules/design doc promotion work.

## Round 2 — 2026-07-05

- Phase: final chunk verification
- Result: complete
- Scope completed:
  - promoted the Bootstrap finance dashboard composition to canonical `#/finance` and replaced canonical route behavior without reintroducing local route CSS
  - preserved balance-first summary, reporting-period controls, capped sections, and concise attention states
  - aligned supported finance navigation/doc links on `#/finance*` without `v2` naming and updated chunk artifacts
  - marked parent task 3 entries in `tasks.md` and updated manager status/review log files
- `openspec apply` confirmation:
  - command remains unavailable in installed CLI (`unknown command 'apply'`)
  - chunk was implemented and validated directly with tests/lint/validation and manual smoke checks
- Completion protocol checks:
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` passed (from implementation report)
  - `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` passed (from implementation report)
  - no temporary/disposable artifacts were added in `openspec/changes/adopt-finance-bootstrap-default`
  - pending OpenSpec and standard review/status files were updated for this chunk
- Checks run:
  - `direnv exec /Users/jenya/projects/signal-foundry npx vitest run src/pages/Finance.test.ts src/components/FinanceShell.test.ts src/App.test.ts` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry openspec status --change adopt-finance-bootstrap-default` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` ✓
  - Manual smoke via `playwright-cli` against `http://127.0.0.1:5173` ✓
    - canonical finance routing and dashboard rendering verified on desktop and mobile
- Follow-up chunks:
   - `remaining-finance-route-surfaces` (parent task 4) remains to continue.

## Round 3 — 2026-07-05

- Phase: post-commit cleanup
- Result: complete
- Scope completed:
  - parent task 3 implementation commit exists and the working tree is clean
  - chunk review remains safe to continue from
- Commit status: complete (`0fcbd92 Implement canonical finance dashboard`)
- Artifact cleanup status: clean

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
