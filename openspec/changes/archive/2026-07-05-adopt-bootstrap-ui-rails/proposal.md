## Why

The current UI is too dependent on bespoke CSS and route-local styling judgment, which makes it hard for humans and agents to keep the interface coherent across iterations. We need stricter rails now, but a parallel V2 route space is the safer way to prove those rails without fighting the existing shell, tests, and page structure in place.

## What Changes

- Add a parallel Bootstrap V2 route space for the pilot instead of replacing canonical routes in place.
- Add `#/v2/login` as the Bootstrap auth-form pilot while keeping `#/login` stable until promotion.
- Add `#/v2/finance` as the Bootstrap finance dashboard pilot while keeping `#/finance` stable until promotion.
- Implement `#/v2/finance` as a Bootstrap-specific protected shell boundary in `App.svelte` or equivalent route wiring, reusing finance auth, tenant state, and data helpers but not `FinanceShell.svelte`, `Nav.svelte`, or other V1 visual chrome.
- Let the V2 finance shell own shell-level tenant selection, sign-out and theme controls, and compact finance-local navigation for the pilot; non-pilot destinations may hand off to canonical `#/finance/*` routes until later V2 finance slices exist.
- Add Bootstrap as the primary styling dependency for the pilot; use Bootstrap directly rather than a Svelte Bootstrap wrapper.
- Treat custom CSS as an exception for the pilot, limited to app-shell containment, third-party widgets, Bootstrap bridge variables, and small documented layout gaps.
- Compose the V2 login page with standard Bootstrap form, alert, button, container, and responsive spacing classes.
- Compose the V2 finance dashboard with standard Bootstrap grid, cards, forms, buttons, nav/list/table utilities, and responsive classes while preserving existing finance data contracts.
- Update `apps/signal-ui/AGENTS.md`, `DESIGN.md`, `ui-wireframe.md`, and relevant `docs/manual-e2e/*` guidance during implementation so future agents follow the Bootstrap rails and the pilot smoke path stays documented.
- Keep non-pilot and canonical routes visually stable unless they must change to support shared Bootstrap setup.
- Defer redirecting or promoting canonical `#/login` and `#/finance` to a later explicit decision after the V2 pilot is visually reviewed.

## Capabilities

### New Capabilities

- `signal-ui-bootstrap-rails`: defines the UI styling contract for Bootstrap-first V2 pages, custom CSS limits, and the V2 login pilot behavior.

### Modified Capabilities

- `finance-operator-ui`: adds a parallel V2 Bootstrap finance dashboard route that can later be promoted to the canonical finance dashboard.

## Impact

- Affected code: `apps/signal-ui/package.json`, `src/main.ts` or equivalent global style entry, `src/app.css`, `src/styles/design-system.css`, `src/App.svelte`, new or revised V2 route/page components for login and finance dashboard, relevant shared behavior-only helpers, and focused UI tests.
- Affected docs/rules: `apps/signal-ui/AGENTS.md`, `apps/signal-ui/DESIGN.md`, `apps/signal-ui/ui-wireframe.md`, `docs/manual-e2e/README.md`, and `docs/manual-e2e/finance-ui-shell-smoke-e2e.md`.
- Affected dependencies: add `bootstrap` through npm; do not add `sveltestrap`, React-oriented component wrappers, or a second utility CSS framework for this change.
- Affected behavior: no backend, OpenAPI, finance API, authentication API, persisted data contract, or canonical-route promotion changes are proposed.
