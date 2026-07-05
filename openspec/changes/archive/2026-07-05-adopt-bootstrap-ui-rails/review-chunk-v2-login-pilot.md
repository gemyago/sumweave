# V2 Login Pilot — implementation review

## Round 1 — 2026-07-05

- Phase: initial implementation phase
- Result: complete

### Scope completed

- Replaced the `#/v2/login` placeholder with a real Bootstrap pilot login page.
- Kept canonical `#/login` unchanged while moving shared login behavior into `src/lib/auth/login-form.svelte.ts` so both routes now use the same auth submit flow, remembered-destination resolution, loading/disabled state, and inline auth error handling.
- Implemented the V2 page with Bootstrap card, form, input, alert, button, and spacing classes only.
- Kept the V2 route free of any route-local `<style>` block.
- Updated focused tests for:
  - V2 login rendering and canonical handoff link
  - V2 remembered-destination redirect to `/v2/finance`
  - V2 inline error alert behavior
  - V2 loading/disabled submit state
  - V2 no-route-local-style contract
  - App route wiring for public `#/v2/login`
- Updated the touched route docs/runbook text so `#/v2/login` is now described as the real pilot login page instead of a placeholder.

### OpenSpec task status updates made

- Marked `tasks.md` items `2.1` and `2.2` as complete.

### `openspec apply` note

- Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change adopt-bootstrap-ui-rails --task 2.1 --task 2.2`
- Current CLI still fails with `unknown command 'apply'`, so the approved chunk was implemented directly and the task artifact was updated in-repo.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry/apps/signal-ui npx vitest run src/pages/Login.test.ts src/pages/V2Login.test.ts src/App.test.ts`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`
- Manual V2 login smoke with Playwright CLI against the existing PM2 dev server:
  - opened `http://127.0.0.1:5173/#/v2/login`
  - checked desktop `1280x900`
  - checked mobile `390x844`
  - confirmed no console errors or warnings

### UI/UX smoke summary

- The Bootstrap login card remained centered and readable on both desktop and mobile widths.
- No wrapping/overflow issue was visible in the title, helper copy, or action row.
- The V2 login route loaded without console failures.

### Files changed in this chunk

- `apps/signal-ui/src/App.test.ts`
- `apps/signal-ui/src/lib/auth/login-form.svelte.ts`
- `apps/signal-ui/src/pages/Login.svelte`
- `apps/signal-ui/src/pages/V2Login.svelte`
- `apps/signal-ui/src/pages/V2Login.test.ts`
- `apps/signal-ui/ui-wireframe.md`
- `docs/manual-e2e/finance-ui-shell-smoke-e2e.md`
- `openspec/changes/adopt-bootstrap-ui-rails/tasks.md`
- `openspec/changes/adopt-bootstrap-ui-rails/review-chunk-v2-login-pilot.md`

### Blockers / follow-up notes

- No blocker for this chunk.
- `make affected-lint-test` still reports one pre-existing Svelte deprecation warning from canonical `src/components/FinanceShell.svelte` using `<slot>`.

### Artifact cleanup

- Closed the Playwright browser session after the smoke run.
- No ad-hoc repo artifacts were kept from the smoke run.
