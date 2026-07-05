## Why

- The accepted Bootstrap pilot proved the visual direction, but Finance still has split canonical and pilot surfaces plus rules that describe Bootstrap as temporary.
- The next product step is to make Bootstrap the default for the Finance app, send operators there after login, and keep non-finance workflows on the existing stack for now.

## What Changes

- Promote Bootstrap to the canonical/default styling contract for the Finance app under `#/finance*`.
- Upgrade every tenant-facing Finance route to Bootstrap-first shell, navigation, forms, cards, lists, tables, alerts, and empty/error states.
- Make canonical `#/login` use the Bootstrap login experience and route successful sign-in to `#/finance` when no protected destination was remembered.
- Remove the parallel pilot product route naming for promoted login and Finance surfaces; canonical routes become the only planned product surface for this change.
- Retire the legacy `#/v2/login` and `#/v2/finance` route surface entirely for this change; do not preserve compatibility-only routes, tests, or docs without explicit user approval.
- Keep Chat, Data, Jobs, Providers, Strategies, Evaluations, Admin, and other non-finance surfaces on the existing stack.
- Preserve existing Finance API/auth/tenant contracts; this is a UI routing and composition rollout, not a backend contract expansion.
- **BREAKING**: early-alpha Finance UI route composition and pilot-route expectations may be replaced in place without compatibility shims.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `signal-ui-bootstrap-rails`: Bootstrap rails become canonical for Finance and login instead of a parallel pilot, with post-login default routing to Finance.
- `finance-operator-ui`: all `#/finance*` routes render as canonical Bootstrap Finance app surfaces using existing finance data and tenant behavior.

## Impact

- Affected implementation areas: `apps/signal-ui/src/App.svelte`, canonical login/default routing, Finance shell components, Finance pages, route tests, Finance page tests, shared Bootstrap bridge styles, `apps/signal-ui/AGENTS.md`, `apps/signal-ui/DESIGN.md`, `apps/signal-ui/ui-wireframe.md`, and relevant manual e2e guides.
- Final-review correction areas: remove the remaining legacy `#/v2/*` finance/login route surface and align responsive Finance shell docs/manual smoke text with the implemented Bootstrap shell behavior.
- Affected dependencies: continue using the existing `bootstrap` npm dependency; do not add Svelte Bootstrap wrappers or another utility CSS framework.
- Backend/API impact: no planned changes to `apps/signal-foundry`, OpenAPI, finance data contracts, auth contracts, or persistence.
- Existing active OpenSpec overlap: this change supersedes `restructure-finance-ui-shell` for Finance/login styling direction. The older custom-shell and terminal-native styling direction MUST NOT remain the acceptance target for canonical Finance/login surfaces. Implementation may reuse only behavior lessons from that change, such as shared Finance shell route coverage, route-preserving tenant selection, supported destination mapping, and avoiding dead routes; the final canonical visual contract for `#/login` and `#/finance*` is Bootstrap-first with no `v2` or pilot product naming.
