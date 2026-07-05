# Finance UI Shell Smoke Manual E2E

Follow preparation steps in [README.md](./README.md) first.

Use this guide after `adopt-finance-bootstrap-default`. It is the canonical smoke runbook for Bootstrap `#/login`, default authenticated landing on `#/finance`, the shared Finance shell, dashboard and route groups, responsive behavior, and a quick non-finance regression pass. This change supersedes `restructure-finance-ui-shell` for Finance/login styling; keep only the older behavior lessons such as tenant continuity and route preservation.

## 0. Scope note

- Treat `#/login` and tenant-facing `#/finance*` routes as the supported canonical Bootstrap surfaces.
- Treat retired `#/v2/*` finance/login hashes as unsupported; this smoke run does not cover or preserve them.
- Keep one quick non-finance regression pass in this run so Finance promotion does not silently spill into Chat, Data, Jobs, Providers, or other non-finance surfaces.

## 1. Sign in and confirm default Finance landing

1. Open `http://127.0.0.1:5173/#/login`.
2. Confirm the page shows the canonical Bootstrap login card with labeled username/password fields, inline error space, and one primary submit action.
3. Sign in with the first local user from repo-root `.local-users` unless you intentionally prepared another user.
4. Confirm successful sign-in lands on `#/finance` by default.
5. Open `#/` while still authenticated and confirm the app resolves back to `#/finance`.

Expected:

- no pilot or parallel-product naming appears on canonical login
- login succeeds without console or network failures
- the default authenticated destination is `#/finance`

## 2. Confirm Finance shell and tenant context

1. Stay on `#/finance`.
2. If the Finance shell asks for tenant selection, choose the seeded or existing tenant you want to use for the run.
3. If no usable tenant exists yet, create one with [finance-tenants-management-e2e.md](./finance-tenants-management-e2e.md) or reseed the normal local finance data before continuing.
4. Confirm the Finance shell is the primary chrome on the route:
   - desktop left rail for supported Finance destinations
   - compact Finance utility header in the content column
   - shell-level theme and sign-out controls
   - at most one compact shell-level tenant selector when multiple tenants exist

Expected:

- the selected tenant stays in shared shell state while the requested route resolves
- sole-tenant users do not see a redundant tenant switcher on tenant-scoped routes
- unsupported dead links such as `Rules` or `Settings` are not shown

## 3. Dashboard smoke

1. Stay on `#/finance`.
2. Confirm the dashboard hierarchy is visible for the selected tenant:
   - compact page header
   - visible reporting-period summary with previous/current/next controls
   - balance-first summary with booked balance before secondary sections
   - compact income, expense, and pending summaries
   - one primary cash-flow or equivalent summary visual in the first viewport
   - account snapshot section
   - category or spending summary section
   - recent transactions section
   - compact needs-attention or follow-up states
3. Change the reporting range if that control is available and confirm the page updates without losing shell state.

Expected:

- the page reads as the canonical Finance dashboard, not the older custom-shell/subnav layout
- shell chrome stays visually secondary to the money summary
- empty or reduced sections remain honest product states rather than fake placeholders

## 4. Finance route-group smoke

1. Use the Finance shell navigation to visit the supported route groups:
   - `#/finance/transactions`
   - `#/finance/accounts`
   - `#/finance/categories`
   - `#/finance/connections`
   - `#/finance/imports`
   - `#/finance/tenants`
2. Confirm each route keeps the shared Finance shell active and renders Bootstrap-first headings, actions, forms, cards, lists, tables, alerts, and empty/loading/error states appropriate to the page.
3. From Accounts, open one account detail route and confirm `#/finance/accounts/:accountId` preserves Finance context.
4. From Transactions, open the dedicated create route and, if seeded data exists, one existing edit route. Confirm the browse page remains table-first and detail/editor flows stay on dedicated routes.
5. From Connections, start the synthetic flow if safe for the environment and confirm the app reaches `#/finance/connections/synthetic`; if no pending `state` exists, confirm the route shows guidance instead of crashing.
6. If Imports or Connections exposes a Finance job deep link, open it and confirm `#/finance/jobs/:jobId` stays inside Finance context after tenant resolution.

Expected:

- supported Finance routes stay on real product paths under `#/finance*`
- tenant-aware deep links preserve the requested destination instead of bouncing to another Finance page
- route groups do not fall back to generic app nav as their primary chrome

## 5. Responsive and visual smoke

1. Check the Finance shell at a desktop viewport such as `1280x900`.
2. Check the same routes at a narrow viewport such as `390x844`.
3. On both `#/finance` and `#/finance/transactions`, look for:
    - the desktop rail uses the wider workspace well
    - on narrow viewports, the full Finance nav stacks above the utility header and page content as a full-width section
    - the utility header wraps compactly below the stacked nav without introducing a menu-toggle-only state
    - tenant switching, when present, stays shell-owned rather than becoming a large page-level block
    - the dashboard still reads cleanly after the stacked shell chrome without overlap or clipped controls
    - headings, actions, and filter rows remain readable
    - no overlapping shell, toolbar, table, or inspector regions
    - no clipped content, horizontal overflow, or unusable action rows

Expected:

- desktop and narrow layouts remain readable and operable
- the stacked narrow-shell layout remains readable without overlap, clipping, or a missing navigation state

## 6. Non-finance regression smoke

1. Sign out if needed, then open a protected non-finance route such as `#/jobs`.
2. Confirm the app redirects to `#/login`.
3. Sign in again and confirm the remembered protected destination wins over the default Finance landing.
4. While authenticated on the non-finance route, open one or two other non-finance destinations such as `#/chat`, `#/data`, or `#/providers`.
5. Confirm those routes still use the existing generic app nav and non-finance styling stack.
6. Use the Finance link from the generic nav and confirm the app switches back to the Finance shell at `#/finance`.

Expected:

- default authenticated landing is Finance only when no remembered protected route exists
- non-finance routes remain on their existing shell/styling stack
- switching between non-finance routes and Finance changes shell chrome in the expected direction

## 7. If anything is wrong, report it

Capture:

- the signed-in username and selected tenant name
- the exact route and viewport size
- screenshots or short recordings of the failure
- console errors or warnings
- failed network requests and response details
- which login, shell, dashboard, route-group, responsive, or non-finance-regression expectation did not match
