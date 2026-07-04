# Finance UI Shell Smoke Manual E2E

Follow preparation steps in [README.md](./README.md) first.

Use this guide after `restructure-finance-ui-shell` lands. It is the smoke runbook for the finance shell, dashboard, and transactions workspace introduced by that change.

## 1. Sign in

1. Open `http://127.0.0.1:5173/#/login`.
2. Sign in with the first local user from repo-root `.local-users` unless you intentionally prepared another user.
3. Confirm the app redirects into the authenticated UI instead of staying on `#/login`.

Expected:

- login succeeds
- an authenticated app shell loads without console or network failures

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
