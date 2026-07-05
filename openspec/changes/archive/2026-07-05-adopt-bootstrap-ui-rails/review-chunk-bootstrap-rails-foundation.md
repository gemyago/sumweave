# Bootstrap Rails Foundation — implementation review

## Round 1 — 2026-07-05

- Phase: initial implementation phase
- Result: complete

### Scope completed

- Added `bootstrap` as a UI dependency and imported `bootstrap/dist/css/bootstrap.min.css` globally from `apps/signal-ui/src/main.ts` before the existing app styles.
- Added the parallel V2 route boundary in `App.svelte`:
  - public `#/v2/login`
  - protected `#/v2/finance`
  - unauthenticated `#/v2/finance` now remembers `/v2/finance` and redirects to `#/v2/login`
  - authenticated `#/v2/finance` renders inside a new Bootstrap-specific `V2FinanceShell.svelte` instead of the generic app nav or canonical `FinanceShell.svelte`
- Added the smallest replaceable placeholders needed to keep the app compiling and the routes reachable:
  - `src/pages/V2Login.svelte`
  - `src/pages/V2Finance.svelte`
- Reused finance tenant-workspace behavior through `createFinanceShellState` / `provideFinanceShellState` in the new V2 shell, while keeping the shell visuals Bootstrap-only.
- Updated focused route smoke coverage for:
  - V2 login route recognition
  - V2 finance shell recognition
  - V2 protected-route redirect and remembered destination
  - V2 protected-route classification in post-login routing helpers
- Updated the touched UI guidance/docs for the pilot boundary and no-promotion rule:
  - `apps/signal-ui/AGENTS.md`
  - `apps/signal-ui/DESIGN.md`
  - `apps/signal-ui/ui-wireframe.md`
  - `docs/manual-e2e/README.md`
  - `docs/manual-e2e/finance-ui-shell-smoke-e2e.md`

### OpenSpec task status updates made

- Marked `tasks.md` items `1.1` and `1.2` as complete.

### `openspec apply` note

- Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-bootstrap-ui-rails --task 1.1 --task 1.2`
- Current CLI still fails with `unknown command 'apply'`, so the approved chunk was implemented directly and the task artifact was updated in-repo.

### Dependency investigation summary

- Bootstrap is active and broadly maintained: GitHub page shows about 174k stars, 23k+ commits, active issues/PRs, and a current `v5.3.8` release.
- It fits the current stack because this chunk only needs compiled CSS imported by Vite/Svelte; no wrapper library or Bootstrap JS dependency was introduced.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec instructions tasks --change adopt-bootstrap-ui-rails`
- `direnv exec /Users/jenya/projects/signal-foundry openspec status --change adopt-bootstrap-ui-rails`
- `direnv exec /Users/jenya/projects/signal-foundry/apps/signal-ui npm install`
- `direnv exec /Users/jenya/projects/signal-foundry/apps/signal-ui npx vitest run src/App.test.ts src/lib/routing/post-login-destination.test.ts src/main.test.ts`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`
- Manual route smoke / visual spot check with Playwright CLI against a temporary Vite dev server:
  - opened `#/v2/login`
  - opened unauthenticated `#/v2/finance`
  - confirmed redirect back to `#/v2/login`
  - confirmed no console errors after the shell-mount guard fix

### UI/UX smoke summary

- `#/v2/login` placeholder rendered cleanly without wrapping or overlap in the quick Playwright snapshot.
- Unauthenticated `#/v2/finance` no longer mounts the tenant shell early, so the prior 401 console noise was removed.
- No route-local `<style>` blocks were added in the V2 placeholder pages or the V2 shell.

### Files changed in this chunk

- `apps/signal-ui/AGENTS.md`
- `apps/signal-ui/DESIGN.md`
- `apps/signal-ui/package.json`
- `apps/signal-ui/package-lock.json`
- `apps/signal-ui/src/App.svelte`
- `apps/signal-ui/src/App.test.ts`
- `apps/signal-ui/src/components/V2FinanceShell.svelte`
- `apps/signal-ui/src/lib/routing/post-login-destination.ts`
- `apps/signal-ui/src/lib/routing/post-login-destination.test.ts`
- `apps/signal-ui/src/main.ts`
- `apps/signal-ui/src/pages/V2Finance.svelte`
- `apps/signal-ui/src/pages/V2Login.svelte`
- `apps/signal-ui/ui-wireframe.md`
- `docs/manual-e2e/README.md`
- `docs/manual-e2e/finance-ui-shell-smoke-e2e.md`
- `openspec/changes/adopt-bootstrap-ui-rails/tasks.md`
- `openspec/changes/adopt-bootstrap-ui-rails/review-chunk-bootstrap-rails-foundation.md`

### Blockers / follow-up notes

- No blocker for this chunk.
- `make affected-lint-test` still reports one pre-existing Svelte deprecation warning from canonical `src/components/FinanceShell.svelte` using `<slot>`; this chunk did not add a new warning and the new V2 shell already uses `{@render ...}`.

### Artifact cleanup

- Temporary Vite dev server used for Playwright smoke was stopped.
- No ad-hoc repo artifacts were kept from the Playwright run.
