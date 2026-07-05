# Final-review route/doc corrections — implementation review log

## Round 1 — 2026-07-05

- Phase: fixing phase
- Result: complete
- Scope completed:
  - retired the remaining legacy `#/v2/login` and `#/v2/finance` route surface from the UI route map, protected-destination logic, and compatibility-only tests
  - removed the dedicated V2 shell/page/test artifacts that only existed to preserve the retired route surface
  - updated Finance route docs and manual smoke text so responsive behavior describes the implemented stacked Bootstrap aside plus compact utility header instead of a menu toggle
  - marked tasks `6.1` and `6.2` complete and updated manager status for this correction chunk
- `openspec apply` note:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-finance-bootstrap-default --task 6.1 --task 6.2`
  - current CLI still fails with `unknown command 'apply'`

### What changed

- Removed legacy V2 login/finance route registration, V2-only shell selection, and V2 protected-destination preservation from `App.svelte` and the routing helper.
- Replaced V2 compatibility assertions with retirement assertions in App/routing tests, then deleted the no-longer-referenced V2 page and shell test files.
- Updated `apps/signal-ui/AGENTS.md`, `apps/signal-ui/ui-wireframe.md`, and `docs/manual-e2e/finance-ui-shell-smoke-e2e.md` so agents and manual smoke guidance treat legacy `#/v2/*` finance/login hashes as retired and describe the current narrow stacked Finance shell accurately.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-finance-bootstrap-default --task 6.1 --task 6.2` *(fails: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry npx vitest run src/App.test.ts src/lib/routing/post-login-destination.test.ts src/pages/Login.test.ts src/components/FinanceShell.test.ts` ✓
- `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` ✓
- `direnv exec /Users/jenya/projects/signal-foundry openspec status --change adopt-finance-bootstrap-default` ✓
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` ✓
- Manual smoke via `playwright-cli` against `http://127.0.0.1:5173` ✓
  - canonical login lands on `#/finance`
  - retired `#/v2/login` and `#/v2/finance` no longer render supported login/Finance shells
  - narrow responsive Finance shell shows the implemented stacked full-width nav/aside behavior with compact utility controls and no toggle-only state

### UI verification

- Manual desktop and narrow responsive checks found no new overlap, clipping, or shell-state issues in the corrected Finance/login scope.

### Artifact cleanup status

- Clean: only standard OpenSpec artifacts were added or updated in the change directory; no ad-hoc tracked artifacts were created.

## Round 2 — 2026-07-05

- Phase: final chunk verification
- Result: complete
- Scope completed:
  - task 6.1 and task 6.2 outcomes remain in place:
    - legacy `#/v2/login` and `#/v2/finance` route registration removed
    - post-login-destination logic no longer stores retired hash routes as protected
    - legacy V2 compatibility tests removed and replaced with retirement assertions
    - `apps/signal-ui/AGENTS.md`, `apps/signal-ui/ui-wireframe.md`, and `docs/manual-e2e/finance-ui-shell-smoke-e2e.md` now describe retirement and the stacked responsive shell
- `openspec apply` confirmation:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-finance-bootstrap-default --task 6.1 --task 6.2`
  - command still unavailable: `unknown command 'apply'`
- Completion protocol checks:
  - `direnv exec /Users/jenya/projects/signal-foundry npx vitest run src/App.test.ts src/lib/routing/post-login-destination.test.ts` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry openspec status --change adopt-finance-bootstrap-default` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` ✓
  - `playwright-cli` manual desktop/narrow smoke on `http://127.0.0.1:5173` ✓
  - no obvious regression observed for this correction scope
- Artifact cleanup status: clean
- Commit status: pending (no commit created for this finalization pass)
- Follow-up chunks:
  - none
