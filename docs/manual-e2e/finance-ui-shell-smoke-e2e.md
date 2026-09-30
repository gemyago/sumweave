# Finance UI Shell Smoke Manual E2E

Follow preparation steps in [README.md](./README.md) first.

Use this guide after `adopt-finance-bootstrap-default`. It is the canonical smoke runbook for Bootstrap `#/login`, default authenticated landing on `#/finance`, the shared Finance shell, dashboard and route groups, responsive behavior, and a quick non-finance regression pass. This change supersedes `restructure-finance-ui-shell` for Finance/login styling; keep only the older behavior lessons such as tenant continuity and route preservation.

## 0. Scope note

- Treat `#/login` and tenant-facing `#/finance*` routes as the supported canonical Bootstrap surfaces.
- Treat retired `#/v2/*` finance/login hashes as unsupported; this smoke run does not cover or preserve them.
- Keep one quick non-finance regression pass in this run so Finance promotion does not silently spill into Chat, Providers, Admin, or other retained non-finance surfaces.

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
2. If the Finance shell asks for tenant selection, open **User menu** and choose **Active tenant** at the very top. Choose the seeded or existing tenant you want to use for the run; the requested Finance route must stay unchanged.
3. If no usable tenant exists yet, create one with [finance-tenants-management-e2e.md](./finance-tenants-management-e2e.md) or reseed the normal local finance data before continuing.
4. Confirm the Finance shell is the primary chrome on the route:
   - one compact Bootstrap navbar: brand then primary links left, user icon right, with no active tenant text in the bar
   - **Dashboard**, **Transactions**, and **Accounts** navbar links with correct active state and a full-width canvas, not a reserved sidebar column; below 576px use **Toggle Finance navigation** to reveal these links
   - **User menu** starts with **Active tenant** identity/selection, then groups: **Finance setup** (Categories, Rules, Connections & sync, Imports), **Workspace** (Tenants), **Preferences** (Theme and Sign out)
   - at most one **Active tenant** selector while the menu is open on multi-tenant tenant-scoped routes; Tenants owns its selection on-page instead
   - compact section breadcrumbs on wider screens; linked parent breadcrumbs and a non-link current leaf on detail routes, including phones

Expected:

- the selected tenant stays in shared shell state while the requested route resolves
- sole-tenant users do not see a redundant tenant switcher on tenant-scoped routes
- unsupported dead links such as `Settings` are not shown; **Rules** is supported

### User-menu interaction checks

1. Focus **User menu** and open it with Enter, then repeat with Space. The top tenant selector receives focus when present, otherwise the first destination does; the trigger reports expanded state.
2. Use Tab/Shift+Tab through native links, **Active tenant** (when present), Theme radios, and Sign out. There are no application-menu arrow-key semantics; native select/radio keyboard behavior remains available.
3. Press Escape: the menu closes and focus returns visibly to its trigger. Reopen, then click outside in an uncovered page gutter; it closes without navigating. Reopen and tab beyond its controls or Shift+Tab outside the container; leaving focus dismisses it.
4. Open each secondary destination: navigation closes the menu and restores trigger focus. An external route change also dismisses an open menu.
5. Switch between two existing tenants on Accounts or Transactions. Record the exact hash route (including query) beforehand: it must remain identical, the route data must reload for the new tenant, and the menu must stay open. Theme changes likewise retain the menu.
6. Check Light, Dark, and Auto and reload to confirm persistence. Assess menu headings, top tenant identity/selection, and idle user-icon contrast in both Light and Dark, plus trigger hover/focus states. Sign out through the menu and confirm the login redirect (then continue section 6).

## 3. Dashboard smoke

1. Stay on `#/finance`.
2. Confirm the dashboard hierarchy is visible for the selected tenant:
   - compact page header
    - visible reporting-period summary with **Previous month**, **Current month**, and **Next month** controls
    - full-width cash-flow card first, with title, Net-state, and settled-transaction-count badges in its header plus Net, Income, Expense, and Pending net KPI tiles
     - primary cash-flow visual after the KPIs; with the menu closed, useful summary should begin in the first viewport (existing FX warnings may push later KPIs/chart below it)
    - Transactions immediately after the cash-flow card
    - Top categories, then Largest balances, in their shared row
    - compact needs-attention or follow-up states
3. Click **Previous month** twice and **Next month** twice, waiting for each dashboard load to complete because all period actions disable during loading. Confirm every completed click advances exactly one browser-local calendar month. In browser network tools, confirm each request contains only explicit `startDate` and `endDate` timestamps and that the response period returns those same half-open `[startDate, endDate)` bounds: a month request ends at the next local month start. Inclusive end labels show the previous local calendar day for an exclusive end at local midnight; otherwise they show the exclusive end instant's local calendar date. If the shell offers another tenant, switch it and confirm the active client range reloads for that tenant with the same displayed bounds; then confirm the first following previous/next click advances one additional month without a stale cross-tenant response. Click **Current month** and confirm its request and response period use that browser-local month's explicit bounds. Use **Custom range** separately; for custom date-only values, confirm the selected start is reported from local start-of-day and the selected end at the following local day's start, including any imported row recorded at local midnight on the first day.

Expected:

- the page reads as the canonical Finance dashboard, not the older custom-shell/subnav layout
- shell chrome stays visually secondary to the money summary
- empty or reduced sections remain honest product states rather than fake placeholders

## 4. Finance route-group smoke

1. Use the Transactions and Accounts navbar links (open **Toggle Finance navigation** first on phones), then **User menu** for the secondary route groups:
   - `#/finance/transactions`
   - `#/finance/accounts`
    - `#/finance/categories`
    - `#/finance/rules`
   - `#/finance/connections`
   - `#/finance/imports`
   - `#/finance/tenants`
2. Confirm each route keeps the shared Finance shell active and renders Bootstrap-first headings, actions, forms, cards, lists, tables, alerts, and empty/loading/error states appropriate to the page.
3. From Accounts, open one account detail route and confirm `#/finance/accounts/:accountId` preserves Finance context, keeps Accounts active, and shows linked Finance/Accounts parents with a non-link Account detail leaf.
4. From Transactions, open the dedicated create route and, if seeded data exists, one existing edit route. Confirm the browse page remains table-first and detail/editor flows stay on dedicated routes. Transactions remains active; breadcrumbs name Record transaction or Transaction as the non-link leaf.
5. From Connections, open **Synced accounts** on one card. Confirm it lazily loads account name/currency/last sync and account links; retry a failed card-local load, then use **Sync now** and reopen the disclosure to confirm its cached details are refreshed. Start the synthetic flow if safe for the environment and confirm the app reaches `#/finance/connections/synthetic`; if no pending `state` exists, confirm the route shows guidance instead of crashing.
6. On synthetic setup, confirm the breadcrumb is Finance / Connections & sync / Synthetic setup, with linked parents, and Connections & sync is active inside User menu. If Imports or Connections exposes a Finance job deep link, open it and confirm `#/finance/jobs/:jobId` stays inside Finance context after tenant resolution.
7. In **Rules**, confirm an existing tagged rule names its target tags as compact badges. Add or edit a rule, select an optional existing tag, save, and confirm the list refreshes with that tag. If the tag catalog is unavailable, confirm the draft stays visible with a retry action and cannot silently drop a selected tag.
8. In **Transactions**, assign or change a category on one record. Confirm only the compact **Create rule from this transaction** / **Dismiss** action appears without moving focus or scrolling. Save a description or tag change first, then open the action and confirm the editable rule defaults use those latest saved values. Dismiss/cancel or fail a rule save and confirm the transaction edit remains saved and the rule interaction stays recoverable.

Expected:

- supported Finance routes stay on real product paths under `#/finance*`
- tenant-aware deep links preserve the requested destination instead of bouncing to another Finance page
- route groups do not fall back to generic app nav as their primary chrome

## 5. Responsive and visual smoke

1. Check the Finance shell at a desktop viewport such as `1280x900`.
2. Check the same routes at the standard phone acceptance viewport `390x844`, in Light and Dark. Use populated Dashboard data as well as an empty-settled-cash-flow state where available; do not establish containment from an empty fixture alone. Optionally add `320x844` stress checks and record these separately from standard 390px acceptance.
3. Without reloading, shrink the open dashboard from `1280x900` through approximately `1200px`, `992px`, `768px`, and `576px` to `390x844`, then expand back to `1280x900`. Repeat on `#/finance/transactions`.
4. On both `#/finance` and `#/finance/transactions`, look for:
    - no reserved desktop rail width; the full-width route canvas uses the reclaimed space
    - from 576px upward, brand and primary links are left-aligned on the same row with user icon right; no permanent header + tabbar stack remains
    - below 576px, the resting navbar is one brand/toggler/user row; Enter/Space on **Toggle Finance navigation** reveals all three links in a compact row, correct active state intact. Escape closes it and returns toggler focus; primary navigation and route changes close it too
    - no tenant name appears in the bar at any width; active tenant is immediately visible at the top of User menu. Redundant section breadcrumbs hide on phones, but detail breadcrumbs remain visible
    - the open menu stays right-aligned within the viewport; at short heights such as `390x360` (optionally `320x300`) it scrolls internally so Sign out stays reachable
    - tenant switching stays shell-owned rather than becoming a large page-level block
    - with the menu closed, Dashboard begins its useful summary in the first viewport without overlap or clipped controls; existing stale/missing-FX warnings remain visible even when they push later summary/chart content below it
    - headings, actions, and filter rows remain readable
    - no overlapping shell, toolbar, table, or inspector regions
    - no clipped content, horizontal overflow, or unusable action rows
    - cash-flow KPI tiles form a compact `2 x 2` grid at `390px` through large widths and use four columns at extra-large desktop widths; `100000.00 USD` (or an equivalent long value) remains fully visible without clipping or horizontal overflow
    - at optional 320px stress width, populated Dashboard stays contained; KPI money/currency may wrap, without clipping values or horizontal document scrolling
    - the navbar, dashboard controls, and chart reflow in both resize directions; check 575px/576px collapse boundary explicitly; after resize animation settles, chart SVG width matches its host

Expected:

- desktop and narrow layouts remain readable and operable
- primary navbar links/toggler and user disclosure remain usable without overlap, clipping, or a missing navigation state

## 6. Non-finance regression smoke

1. Sign out if needed, then open a protected non-finance route such as `#/chat`.
2. Confirm the app redirects to `#/login`.
3. Sign in again and confirm the remembered protected destination wins over the default Finance landing.
4. While authenticated on the non-finance route, open one or two other retained non-finance destinations such as `#/providers` or `#/admin`.
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
