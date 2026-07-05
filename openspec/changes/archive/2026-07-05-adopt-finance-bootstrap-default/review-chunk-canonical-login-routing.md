# Canonical Login Routing — implementation review log

## Round 1 — 2026-07-05

- Phase: initial implementation phase
- Result: complete
- Scope completed:
  - promote Bootstrap login composition to canonical `#/login`
  - switch default authenticated routing from `#/data` to `#/finance`
  - preserve remembered protected destinations for login redirects
  - update chunk task/status artifacts for parent task 1
- `openspec apply` note:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-finance-bootstrap-default --task 1.1 --task 1.2`
  - current CLI still fails with `unknown command 'apply'`

### What changed

- Replaced the canonical `Login.svelte` route-local custom form styling with Bootstrap-first card, form, alert, and button markup so `#/login` is now the canonical Bootstrap login surface without pilot naming.
- Changed the shared authenticated fallback route constant from `/data` to `/finance`, which updates post-login fallback routing and authenticated root navigation while still honoring remembered protected destinations.
- Updated login, routing, and app-level route tests to cover Bootstrap canonical login rendering, disabled/loading/error behavior, remembered destination precedence, and Finance as the default authenticated landing route.
- Updated the affected `ui-wireframe.md` route entries for `/` and `/login` so the documented default landing and canonical login behavior match this chunk.
- Updated OpenSpec task and manager status artifacts to mark parent task 1 complete.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-finance-bootstrap-default --task 1.1 --task 1.2` *(fails: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry npx vitest run src/pages/Login.test.ts src/pages/V2Login.test.ts src/lib/routing/post-login-destination.test.ts src/App.test.ts` ✓
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` ✓
- `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` ✓
- `direnv exec /Users/jenya/projects/signal-foundry openspec status --change adopt-finance-bootstrap-default` ✓
- Manual smoke via `playwright-cli` against `http://127.0.0.1:5173` ✓
  - canonical `#/login` renders and submits successfully
  - successful login without remembered destination lands on `#/finance`
  - logged-out access to `#/data` returns to `#/data` after login

### UI verification

- Visual review artifacts saved under `tmp/ui-design-review/20260705-canonical-login-routing/`.
- Desktop and mobile login snapshots looked clean; no layout, wrapping, or spacing issues were found in this chunk scope.

### Remaining follow-up

- None inside this chunk scope. Later chunks still own the canonical Finance shell/dashboard/page rollout and broader docs/rules updates.

## Round 2 — 2026-07-05

- Phase: final chunk verification
- Result: complete
- Scope completed:
  - canonical login composition switched to Bootstrap card layout on `#/login`
  - successful login now resolves to `#/finance` when no remembered destination exists
  - post-login destination preservation remains route-aware and protected-only
  - `#/` auth-root redirect updated to `#/finance`
  - durable routing/login/UI and parent-task artifacts updated for task 1
- `openspec apply` confirmation:
  - command is not available in current CLI (`openspec --help` has no `apply` subcommand)
  - chunk implementation and artifact updates were completed directly and validated via tests
- Completion protocol checks:
  - all task 1 checklist entries are marked in `tasks.md`
  - no ad-hoc artifacts present in `openspec/changes/adopt-finance-bootstrap-default`
  - completion protocol command `make affected-lint-test` run and passing
- Checks run:
  - `direnv exec /Users/jenya/projects/signal-foundry npx vitest run src/pages/Login.test.ts src/pages/V2Login.test.ts src/lib/routing/post-login-destination.test.ts src/App.test.ts` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry openspec validate adopt-finance-bootstrap-default --strict` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry openspec status --change adopt-finance-bootstrap-default` ✓
- Follow-up chunks: none for parent task 1
