## MODIFIED Requirements

### Requirement: Distinct Protected Finance Area
The operator UI SHALL provide a distinct protected Finance area rather than mixing finance workflows into trading/operator routes.

#### Scenario: Finance navigation is tenant-aware, protected, and Bootstrap-first
- **WHEN** an authenticated operator uses the application navigation
- **THEN** the UI MUST provide a top-level Finance entry and protected tenant-aware routes including `#/finance`, `#/finance/tenants`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/connections`, `#/finance/connections/synthetic`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/imports`, and `#/finance/jobs/:jobId`
- **AND** unauthenticated access to those routes MUST redirect through the existing protected-route behavior to canonical login
- **AND** all tenant-facing `#/finance*` routes MUST render inside the canonical Bootstrap Finance shell
- **AND** those routes MUST use Bootstrap-first markup and states instead of the older custom finance chrome or a parallel pilot product surface

#### Scenario: Finance routing stays distinct from trading routes
- **WHEN** finance screens are added to the SPA
- **THEN** they MUST remain visually and navigationally distinct from Data, Strategies, Evaluations, Chat, and other trading/runtime workflows
- **AND** the Bootstrap Finance shell MUST expose only supported Finance destinations for this slice, mapping navigation to real product routes such as dashboard, transactions, accounts, categories, connections and sync, imports, and tenants
- **AND** unsupported reference items such as `Rules` or `Settings` MUST remain out of scope until backed by real product workflows
- **AND** non-finance routes MUST remain on the existing stack unless a later change explicitly promotes them

### Requirement: Finance Dashboard And Workspace Flows
The Finance area SHALL expose the first end-user workflows required by the finance design.

#### Scenario: Dashboard shows Bootstrap period-aware finance summaries
- **WHEN** an authenticated tenant member opens `#/finance`
- **THEN** the UI MUST prioritize a Bootstrap-based dashboard hierarchy with compact header, active period context, primary balance summary, income summary, expense summary, pending delta, cash-flow visual, spending or category visual, account snapshot, recent transactions, and a compact needs-attention area
- **AND** the first viewport MUST avoid large implementation-facing copy, full-width tenant controls, full-width custom reporting forms, and admin diagnostics as primary content
- **AND** the dashboard MUST derive its first implementation from existing finance dashboard, account, transaction, category, and connection data sources instead of requiring a new API contract
- **AND** account, category, and recent-transaction dashboard sections MUST cap visible rows and link to dedicated browse/detail routes for full lists
- **AND** missing-FX, pending transaction, failed sync, or import follow-up signals MUST appear as secondary attention states rather than primary dashboard cards when present
- **AND** the dashboard MUST use honest empty or reduced states when selected-tenant data is unavailable
- **AND** the dashboard MUST use vanilla Bootstrap classes and native HTML or Svelte markup as the primary styling mechanism instead of the older Signal UI custom finance design-system classes

#### Scenario: Finance management routes use Bootstrap surfaces
- **WHEN** a tenant member opens tenant management, accounts, account detail, categories, connections, synthetic setup, imports, or finance job detail under `#/finance*`
- **THEN** each route MUST render inside the canonical Bootstrap Finance shell with Bootstrap headings, forms, cards, lists, tables, alerts, buttons, loading states, empty states, and recoverable error states as appropriate for the route
- **AND** each route MUST preserve its existing tenant, account, category, connection, synthetic setup, import, and job-detail API behavior without requiring new backend contracts
- **AND** bank-linking flows MUST continue to expose monobank token entry, PKO via Enable Banking redirect/SCA, and synthetic local configured setup as distinct supported choices
- **AND** synthetic setup MUST continue to load pending configuration when present, surface validation or API errors in place, and return to the connection list after successful finish

#### Scenario: Transactions use Bootstrap browse and editor flows
- **WHEN** a tenant member manages transactions under `#/finance/transactions`, `#/finance/transactions/new`, or `#/finance/transactions/:transactionId`
- **THEN** the UI MUST provide Bootstrap-first browsing, filtering, sorting, edit, hide, and category-assignment flows for transactions
- **AND** the transactions browse route MUST provide a search or date or filter toolbar, summary chips, a selectable ledger-style results table, and responsive selected-transaction context where the viewport supports it
- **AND** the transaction editor MUST be reused for both create and edit routes, with create mode initializing a blank editable record and edit mode prefilling the existing editable values
- **AND** the shared transaction editor MUST provide explicit save and cancel actions, show provider-original values when present so operator-edited reporting fields remain distinguishable from synced provider data, and remain usable in a mobile-friendly single-record layout
- **AND** full-record mutation flows MUST stay on dedicated create and edit routes even when the browse route offers contextual review or supported quick actions

### Requirement: Active Tenant Workspace Context
The Finance area SHALL keep one active tenant workspace context across tenant-scoped finance routes and finance-context deep links.

#### Scenario: Sole joined tenant is selected automatically without visible switcher
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to exactly one finance tenant
- **THEN** the UI MUST automatically use that tenant as the active finance workspace without requiring an extra selection step
- **AND** normal tenant-scoped finance routes such as `#/finance` MUST NOT render visible tenant selection controls or duplicate tenant workspace panels solely to show the already-resolved tenant
- **AND** the tenant management route MUST remain available for tenant administration
- **AND** the Bootstrap Finance shell MUST remain the single owner of any active-tenant presentation needed for the route

#### Scenario: Multiple joined tenants use one compact shared switcher
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to multiple finance tenants
- **THEN** the UI MUST require one explicit active-tenant selection when no active tenant has been chosen yet
- **AND** after selection, the UI MUST reuse that active tenant across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/connections/synthetic`, `#/finance/imports`, and `#/finance/jobs/:jobId` until the operator changes it
- **AND** the shared Bootstrap Finance shell MUST expose at most one compact workspace switcher for changing tenants on normal finance routes
- **AND** dashboard panels and route bodies MUST NOT reintroduce unrelated duplicate tenant picker or tenant workspace chrome

## REMOVED Requirements

### Requirement: V2 Bootstrap Finance Dashboard Pilot
**Reason**: The Bootstrap Finance dashboard is being promoted into the canonical Finance app instead of remaining a parallel pilot.
**Migration**: Use canonical `#/finance` for the Bootstrap dashboard and canonical `#/finance*` routes for related Finance workflows.
