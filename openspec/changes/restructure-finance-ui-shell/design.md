## Context

### Reference image analysis

- The reference dashboard is a finance-first workspace, not a generic app page with finance cards dropped into it.
- The dominant shell pattern is:
  - persistent left sidebar rail with logo, finance navigation, and footer utilities
  - compact top utility row
  - page-level header and actions
  - dashboard content arranged as summaries first, then charts, then operational lists and alerts
- The reference transactions screen keeps the same shell and swaps the body for a browse workspace:
  - page header plus primary actions
  - compact search/date/filter toolbar
  - summary chips
  - ledger table as the main browse surface
  - right-side transaction inspector for the selected row
- The image also includes `Rules`, `Settings`, notification, and help affordances, but those are only useful if real product routes or actions exist.

### Current UI analysis

- Finance still lives under the global horizontal `Nav` used by the rest of the product.
- Finance pages repeat finance-local chrome inside each route through `FinanceSubnav.svelte` and page-level tenant pickers.
- `#/finance` is a narrow, stacked summary page with period controls, KPI cards, alerts, missing FX, accounts, and category summaries, but it does not read like a finance dashboard shell.
- `#/finance/transactions` is card-first and route-local:
  - filters are stacked above results
  - results are individual cards instead of a ledger workspace
  - there is no finance-specific shell, utility bar, summary-chip row, table layout, or contextual inspector
- `apps/signal-ui/ui-wireframe.md` currently describes finance as subnav-first, stacked-summary-first, and generally separate-detail-route-first.

### Design tension to resolve

- The reference wants a denser dashboard and a transactions browse workspace.
- Repo UI guidance still matters:
  - preserve `DESIGN.md` tokens and terminal-native styling
  - avoid dead routes
  - keep create/edit flows in dedicated routes where that remains clearer
- This change should therefore adopt the reference information architecture and layout hierarchy, not perform a visual design-system rewrite.

## Goals / Non-Goals

**Goals:**

- Introduce a dedicated finance-only shell for `#/finance*` routes.
- Move active-tenant presentation into shared shell chrome instead of repeating it per page.
- Restructure `#/finance` into a recognizable finance dashboard with the reference hierarchy.
- Restructure `#/finance/transactions` into a browse-first ledger workspace with a responsive contextual inspector.
- Preserve dedicated transaction create/edit routes for full-record editing.
- Keep the existing Signal UI terminal/monospace design language and tokens.
- Add a manual smoke guide for the future shell and bake the sub-agent run/report/fix/rerun loop into the plan.

**Non-Goals:**

- No Svelte or Go implementation in this planning slice.
- No finance backend or OpenAPI expansion just to match unsupported reference widgets.
- No design-system rewrite away from the current `DESIGN.md` foundations.
- No unsupported `Rules`, `Settings`, notification, or help routes as placeholder links.
- No non-finance shell rewrite for Chat, Data, Jobs, Providers, Strategies, Evaluations, or Admin.

## Decisions

1. Finance routes will use a dedicated finance-first shell.

   - `#/finance*` should stop relying on the current in-page finance subnav card as the primary finance chrome.
   - The finance shell should own:
     - left navigation rail on desktop
     - compact top utility row
     - page content region
     - shared active-tenant control for tenant-aware finance routes
   - Non-finance routes stay on the existing global shell.
   - To avoid trapping operators in a finance island, the shell should keep a simple escape hatch back to the broader product chrome through supported branding or navigation affordances, not a second full-width global nav copy.

2. The left rail will expose supported finance destinations only.

    - The reference navigation should be mapped to real product routes, not copied literally.
    - The initial supported finance rail should cover the existing finance workflows as Dashboard, Transactions, Accounts, Categories, Connections & sync, Imports, and Tenants.
    - `Rules` and `Settings` remain future work until the product has real routes and behavior behind them.

3. The top utility row adopts the reference layout role, not unsupported feature parity.

   - The utility row should carry existing useful controls such as user/session context, sign out, theme, and shell-level tenant context where appropriate.
   - The layout may reserve the same hierarchy as the reference, but it should not ship fake notification or help buttons just to imitate the image.

4. The dashboard will follow the reference hierarchy while staying honest about available data.

   - `#/finance` should be planned around:
     - finance dashboard header
     - visible reporting range and route actions
     - KPI strip
     - cash-flow visual
     - account summary
     - spending/category summary
     - recent transactions
     - sync activity
     - alerts or insights
    - Implementation should use existing finance/dashboard/account/transaction/connection data sources first.
    - If some reference-shaped widget has no existing API-backed data, the UI should render an honest reduced or empty state inside the new shell rather than forcing backend scope into this change.
    - The dashboard should not ship an inert finance-wide search field; add dashboard search only if it filters a real dashboard-local data set in this slice.

5. Transactions will become table-first, but full editing stays route-based.

   - `#/finance/transactions` should become a browse workspace with:
     - page header and supported actions
     - search/date/filter toolbar
     - summary chips
     - selectable ledger table
     - contextual inspector for the currently selected transaction on wide screens
    - The inspector is for browse/review context and supported quick actions.
    - The first inspector action set is limited to API-backed actions and route handoff; screenshot-only actions such as mark-reviewed, split, exclude, or delete stay out of scope unless already backed by the existing finance API during implementation.
    - Full create/edit remains on `#/finance/transactions/new` and `#/finance/transactions/:transactionId`.
   - This keeps the reference hierarchy without turning the browse screen into the only edit surface.

6. Responsive behavior should degrade from dense workspace to focused route behavior.

   - Desktop keeps the rail and ledger-plus-inspector shape.
   - Narrow screens should collapse or stack shell/toolbar regions cleanly.
   - If the inspector cannot fit, the browse route should still remain usable and continue to hand off full edits to dedicated routes.

7. Implementation planning must include docs and manual smoke iteration.

    - `apps/signal-ui/ui-wireframe.md` must be updated alongside implementation because the current finance wireframe is no longer the intended route behavior.
    - The manual smoke guide should act as the route-by-route finance-shell runbook after the change lands.
    - The smoke loop should be integrated into the shell, dashboard, and transactions implementation slices instead of treated as a detached final verification task.
    - A sub-agent should run the relevant guide sections during implementation, report route/screenshot/console/network evidence for failures, and rerun after fixes until each slice is clean.

## Risks / Trade-offs

- A finance-only shell could reduce cross-product discoverability.
  - Mitigation: keep a simple, explicit escape hatch back to the broader product instead of duplicating the old nav.
- The reference dashboard includes more widgets than the current API may naturally support.
  - Mitigation: prefer honest reduced panels or empty states over backend expansion or fake data.
- A ledger table plus inspector is denser than current finance UI guidance.
  - Mitigation: keep the inspector contextual and responsive, while preserving dedicated create/edit routes.
- Moving tenant choice into shared shell chrome could break deep-link behavior if routed carelessly.
  - Mitigation: preserve requested finance routes during tenant resolution and keep one shell-level source of truth.
- Replacing finance-local page chrome will touch many route tests at once.
  - Mitigation: land the shell first, then dashboard and transactions behavior in bounded slices.

## Migration Plan

1. Define the finance-shell route composition and supported rail destinations for `#/finance*`.
2. Centralize finance active-tenant chrome and route-preserving tenant resolution inside the shell.
3. Replace the current finance dashboard layout with the new shell-aligned dashboard hierarchy, using existing endpoint data first.
4. Replace the current transaction-card browse screen with a table-first ledger workspace and responsive inspector while keeping dedicated create/edit routes.
5. Update `apps/signal-ui/ui-wireframe.md` so the documented finance shell, dashboard, and transactions flows match the implemented routes.
6. Use the new manual smoke runbook inside the shell, dashboard, and transactions implementation slices so sub-agent browser verification reports findings, targeted fixes land, and the same guide sections rerun until clean.

Because the project is early alpha, this plan does not preserve the old finance subnav experience for backward compatibility.

## Open Questions

- No planning blockers remain.
