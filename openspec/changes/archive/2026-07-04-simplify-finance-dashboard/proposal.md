## Why

- The current finance dashboard is overloaded with implementation-facing controls, duplicated tenant context, diagnostic copy, and secondary operational details before the operator can understand their money.
- The target dashboard hierarchy should make balance visible first, then compact income/expense summaries, visual cash-flow and spending summaries, and recent activity before secondary operational detail.
- Most users are expected to belong to a single tenant, so showing tenant selection everywhere creates friction and makes the product feel more complex than it is.
- We have enough finance sandbox data to redesign the dashboard using existing finance endpoints instead of expanding backend scope.

## What Changes

- Recompose `#/finance` into a simple finance dashboard focused on balance, cash flow, spending categories, account snapshot, recent transactions, and a compact attention strip.
- Remove large dashboard-level tenant and reporting control blocks from the primary first viewport.
- Hide tenant selection for single-tenant users and keep only one compact workspace switcher for multi-tenant users.
- Keep the shell-owned active tenant consistent across tenant-scoped finance routes and direct finance deep links instead of reintroducing route-level tenant pickers.
- Demote admin/diagnostic content such as missing FX, pending sync, and failed import signals into secondary attention states or admin routes.
- Update the finance UI OpenSpec delta, implementation task plan, and future wireframe expectations for the redesigned dashboard.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-operator-ui`: simplify the dashboard hierarchy and active-tenant presentation for finance routes.

## Impact

- Affected implementation areas: `apps/signal-ui/src/components/FinanceShell.svelte`, `apps/signal-ui/src/pages/Finance.svelte`, finance UI tests, `apps/signal-ui/ui-wireframe.md`, and finance manual smoke guidance.
- No new Go, OpenAPI, or persistence scope is proposed.
- The old dense dashboard layout can be replaced outright because the project is early alpha.
