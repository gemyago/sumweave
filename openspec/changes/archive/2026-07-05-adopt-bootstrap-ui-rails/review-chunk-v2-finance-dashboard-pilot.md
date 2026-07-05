# V2 Finance Dashboard Pilot — implementation review

## Round 1 — 2026-07-05

- Phase: initial implementation phase
- Result: complete

### Scope completed

- Replaced the `#/v2/finance` placeholder with a real Bootstrap-first pilot dashboard inside the dedicated `V2FinanceShell` boundary.
- Kept canonical `#/finance` unchanged while reusing the existing finance auth path, tenant workspace state, dashboard API, transactions list, connections list, and finance formatting helpers.
- Kept the V2 route free of any route-local `<style>` block and normal `style=` layout/styling attributes.
- Expanded the Bootstrap shell slightly so it now also drives Bootstrap theme variables with `data-bs-theme={themeStore.effective}` while still owning the shell-level tenant, theme, sign-out, and handoff controls.
- Implemented the V2 dashboard with Bootstrap cards, rows, badges, tables, alerts, progress bars, buttons, and spacing utilities for:
  - compact header and canonical handoff actions
  - period context and previous/current/next/custom range controls
  - booked balance story
  - compact income/expense/pending summaries
  - cash-flow visual region
  - category or spending section
  - account snapshot
  - recent transactions
  - attention states
- Updated the route wireframe and finance shell smoke guide so the V2 finance pilot is described as a real dashboard instead of a placeholder boundary.

### OpenSpec task status updates made

- Marked `tasks.md` items `3.1`, `3.2`, and `3.3` as complete.
- Updated `manager-status.md` chunk ledger entry for `v2-finance-dashboard-pilot` to `complete`.

### `openspec apply` note

- Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-bootstrap-ui-rails --task 3.1 --task 3.2 --task 3.3`
- Current CLI still fails with `unknown command 'apply'`, so the approved chunk was implemented directly and the task artifact was updated in-repo.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry/apps/signal-ui npx vitest run src/pages/V2Finance.test.ts src/components/V2FinanceShell.test.ts src/App.test.ts`
- `direnv exec /Users/jenya/projects/signal-foundry pm2 restart signal-foundry-ui --update-env`
- Manual V2 finance smoke with Playwright-driven Chromium against the PM2 dev server:
  - signed in through `http://127.0.0.1:5173/#/v2/login`
  - checked `#/v2/finance` at `1280x900`
  - checked `#/v2/finance` at `390x844`
  - confirmed no failed network requests
  - confirmed only Vite dev-server debug console messages (`[vite] connecting...`, `[vite] connected.`)
  - confirmed no horizontal overflow and no oversized wrapped action buttons in either viewport
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### UI/UX smoke summary

- Desktop kept the compact V2 shell secondary to the booked-balance story and preserved the requested dashboard reading order.
- Mobile stacked the dashboard cleanly without horizontal overflow or unusable action rows.
- The cash-flow, category, account, recent-transaction, and attention sections remained visible and readable on both viewport sizes.

### Files changed in this chunk

- `apps/signal-ui/src/App.test.ts`
- `apps/signal-ui/src/components/V2FinanceShell.svelte`
- `apps/signal-ui/src/components/V2FinanceShell.test.ts`
- `apps/signal-ui/src/pages/V2Finance.svelte`
- `apps/signal-ui/src/pages/V2Finance.test.ts`
- `apps/signal-ui/ui-wireframe.md`
- `docs/manual-e2e/finance-ui-shell-smoke-e2e.md`
- `openspec/changes/adopt-bootstrap-ui-rails/manager-status.md`
- `openspec/changes/adopt-bootstrap-ui-rails/tasks.md`
- `openspec/changes/adopt-bootstrap-ui-rails/review-chunk-v2-finance-dashboard-pilot.md`

### Blockers / follow-up notes

- No blocker for this chunk.
- `make affected-lint-test` still reports one pre-existing Svelte deprecation warning from canonical `src/components/FinanceShell.svelte` using `<slot>`.

### Artifact cleanup

- Closed the browser after the V2 finance smoke run.
- Screenshots were written only to `/var/folders/kd/cj3zx62j0t36yr8t19v_99mw0000gn/T/opencode/` during smoke verification; no ad-hoc repo artifacts were kept.
