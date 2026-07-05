# Finance UI Shell Smoke Manual E2E

Follow preparation steps in [README.md](./README.md) first.

Use this guide after `restructure-finance-ui-shell` lands. It is the smoke runbook for the canonical finance shell, dashboard, transactions workspace, and the parallel Bootstrap V2 pilot route boundary introduced by `adopt-bootstrap-ui-rails`.

## 0. Bootstrap V2 pilot boundary note

- In this chunk, `#/v2/login` is a real Bootstrap pilot login page and `#/v2/finance` is a real Bootstrap pilot dashboard boundary.
- Canonical `#/login` and `#/finance` remain the real operator paths in this change.
- Do not treat the V2 routes as promoted or feature-complete yet.

## 1. Sign in

1. Open `http://127.0.0.1:5173/#/login`.
2. Sign in with the first local user from repo-root `.local-users` unless you intentionally prepared another user.
3. Confirm the app redirects into the authenticated UI instead of staying on `#/login`.
4. Open `#/v2/login` and confirm the Bootstrap pilot login form loads without a crash.

Expected:

- login succeeds
- an authenticated app shell loads without console or network failures
- `#/v2/login` is reachable as a parallel public route and shows the Bootstrap pilot login form

## 2. Confirm the active finance tenant

1. Open `#/finance`.
2. If the finance shell asks for tenant selection, choose the seeded or existing tenant you want to use for the run.
3. If no usable tenant exists yet, create one with [finance-tenants-management-e2e.md](./finance-tenants-management-e2e.md) or reseed the normal local finance data before continuing.

Expected:

- the finance shell shows one active tenant context
- the selected tenant stays visible in shared finance shell chrome
- the route remains inside the intended finance destination while tenant selection resolves

## 3. Dashboard smoke

1. Stay on `#/finance`.
2. Verify the finance-only shell is present:
   - persistent left rail on desktop
   - compact top utility row
   - supported finance navigation only
3. Confirm the dashboard hierarchy is visible for the selected tenant:
   - compact page header
   - visible reporting period summary with previous/current/next controls
   - balance-first summary with booked balance before secondary sections
   - compact income, expense, and pending summary chips
   - one primary cash-flow or equivalent summary visual in the first viewport
   - account summary section
   - category or spending summary section
   - recent transactions section
   - alerts or insights section
   - sync activity section
4. Change the reporting range if that control is available and confirm the page updates without losing shell state.

Expected:

- the page reads as a finance dashboard, not the old subnav-plus-cards page
- finance shell chrome stays visually secondary to the dashboard money summary
- if the signed-in user belongs to one tenant, no visible tenant selector appears on tenant-scoped finance routes
- if the signed-in user belongs to multiple tenants, only one compact shell-level tenant selector appears
- unsupported dead links such as `Rules` or `Settings` are not shown
- empty sections, if any, are honest and styled as normal product states rather than fake data placeholders

## 3a. Bootstrap V2 finance boundary smoke

1. While still signed in, open `#/v2/finance`.
2. Confirm the canonical app nav is not the primary chrome for this route.
3. Confirm a Bootstrap-specific shell boundary is visible with:
   - finance navigation in the left column or top stack
   - shell-level theme control
   - shell-level sign-out action
   - shell-level tenant selector only when multiple finance tenants exist
   - visible handoff links back to canonical finance routes
4. Confirm the page content reads as a real pilot dashboard with:
   - compact header and period context
   - booked balance story before secondary sections
   - compact income, expense, and pending summaries
   - one primary cash-flow visual region
   - category or spending section
   - account snapshot
   - recent transactions
   - attention states

Expected:

- `#/v2/finance` is recognized as a protected route
- tenant state still comes from shared finance shell behavior
- the pilot remains parallel and does not replace canonical `#/finance`
- Bootstrap-first dashboard sections are visible without reusing canonical `FinanceShell.svelte` or `Nav.svelte` chrome

## 4. Transactions smoke

1. Open `#/finance/transactions`.
2. Confirm the same finance shell remains active and the transactions destination is highlighted.
3. Verify the transactions workspace shows:
   - page header
   - route actions such as import, sync, or new transaction when implemented for the route
   - search or date or filter toolbar
   - summary chips
   - ledger table with transaction columns
4. Select a transaction row.
5. On a desktop-width viewport, confirm a right-side contextual inspector opens for the selected row.
6. Verify the inspector shows the selected transaction context, such as merchant, amount, type, metadata, category or status controls, notes or tags state, and supported quick actions.
7. Open the dedicated create or edit route if those actions are present and confirm the route handoff still works.

Expected:

- the browse route is table-first rather than card-first
- the inspector reflects the selected row without breaking the list state
- dedicated create or edit routes remain the full-record mutation flow

## 5. Responsive and visual smoke

1. Check the finance shell at a desktop viewport such as `1280x900`.
2. Check the same routes at a narrow viewport such as `390x844`.
3. On both `#/finance` and `#/finance/transactions`, look for:
   - on narrow viewports, the rail is collapsed to a compact current-route summary and menu trigger before the main content
   - tenant switching, when present, stays compact and shell-owned rather than becoming a large page-level block
   - the dashboard first viewport still shows the balance story before long navigation or utility chrome
   - wrapped headings or buttons that should stay readable on one line
   - overlapping rail, toolbar, table, or inspector regions
   - clipped content, horizontal overflow, or unusable action rows
   - large empty gutters that waste screen space

Expected:

- desktop keeps the rail and uses the wider workspace well
- narrow layouts remain readable and operable even when the inspector cannot stay side-by-side
- the terminal-native design language still matches the existing app tokens

## 6. Implementation iteration expectation

During implementation, a sub-agent should run this guide after each meaningful shell, dashboard, or transactions slice.

If anything fails:

- report the exact route or hash
- capture a screenshot or snapshot
- capture browser console errors or warnings
- capture failed network requests with status and a short response summary
- note the selected tenant and viewport size

The implementer should fix the issue, then the sub-agent should rerun the same guide until the flow is clean.

## 7. If anything is wrong, report it

Capture:

- the signed-in username and selected tenant name
- the exact route and viewport size
- screenshots or short recordings of the failure
- console errors or warnings
- failed network requests and response details
- which dashboard or transactions section did not match the expected shell behavior
