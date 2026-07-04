# finance-operator-ui Specification

## Purpose
TBD - created by archiving change add-finance-management-slice. Update Purpose after archive.
## Requirements
### Requirement: Distinct Protected Finance Area
The operator UI SHALL provide a distinct protected Finance area rather than mixing finance workflows into trading/operator routes.

#### Scenario: Finance navigation is tenant-aware and protected
- **WHEN** an authenticated operator uses the application navigation
- **THEN** the UI MUST provide a top-level Finance entry and protected tenant-aware routes including `#/finance`, `#/finance/tenants`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/connections`, `#/finance/connections/synthetic`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/imports`, and `#/finance/jobs/:jobId`
- **AND** unauthenticated access to those routes MUST redirect through the existing protected-route behavior

#### Scenario: Finance routing stays distinct from trading routes
- **WHEN** finance screens are added to the SPA
- **THEN** they MUST remain visually and navigationally distinct from Data, Strategies, Evaluations, Chat, and other trading/runtime workflows

### Requirement: Finance Dashboard And Workspace Flows
The Finance area SHALL expose the first end-user workflows required by the finance design.

#### Scenario: Dashboard shows simple period-aware finance summaries
- **WHEN** an authenticated tenant member opens `#/finance`
- **THEN** the UI MUST prioritize a simple dashboard hierarchy with compact header, active period context, primary balance summary, income summary, expense summary, pending delta, cash-flow visual, spending or category visual, account snapshot, recent transactions, and a compact needs-attention area
- **AND** the first viewport MUST avoid large implementation-facing copy, full-width tenant controls, full-width custom reporting forms, and admin diagnostics as primary content
- **AND** the dashboard MUST derive its first implementation from existing finance dashboard, account, transaction, category, and connection data sources instead of requiring a new API contract
- **AND** account, category, and recent-transaction dashboard sections MUST cap visible rows and link to dedicated browse/detail routes for full lists
- **AND** missing-FX, pending transaction, failed sync, or import follow-up signals MUST appear as secondary attention states rather than primary dashboard cards when present
- **AND** the dashboard MUST use honest empty or reduced states when selected-tenant data is unavailable
- **AND** the dashboard MUST preserve the existing Signal UI terminal-native design tokens and styling foundations unless a separate design-system change is explicitly accepted

### Requirement: Admin Diagnostics And Finance Job Deep Links
The UI SHALL provide utilitarian admin diagnostics and connect finance workflows to generic jobs visibility.

#### Scenario: Finance screens deep-link to relevant job detail
- **WHEN** a finance sync, FX refresh, or import creates a durable job
- **THEN** the finance workflow MUST expose job status plus a route link to a finance-focused job detail or the generic admin job detail without losing operator context

#### Scenario: Admin diagnostics expose sanitized operational state
- **WHEN** an authenticated operator opens `#/admin`, `#/admin/finance/fx`, or `#/admin/finance/providers`
- **THEN** the UI MUST show operational diagnostics such as failed jobs, missing FX coverage, stale connections, provider health, and manual sync/retry affordances where supported
- **AND** admin diagnostics MUST make scheduler state and recent scheduled-run visibility observable without replacing tenant-facing bank-connection schedule management
- **AND** it MUST NOT display decrypted secrets or raw provider payloads by default

### Requirement: Active Tenant Workspace Context
The Finance area SHALL keep one active tenant workspace context across tenant-scoped finance routes and finance-context deep links.

#### Scenario: Sole joined tenant is selected automatically without visible switcher
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to exactly one finance tenant
- **THEN** the UI MUST automatically use that tenant as the active finance workspace without requiring an extra selection step
- **AND** normal tenant-scoped finance routes such as `#/finance` MUST NOT render visible tenant selection controls or duplicate tenant workspace panels solely to show the already-resolved tenant
- **AND** the tenant management route MUST remain available for tenant administration

#### Scenario: Multiple joined tenants use one compact shared switcher
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to multiple finance tenants
- **THEN** the UI MUST require one explicit active-tenant selection when no active tenant has been chosen yet
- **AND** after selection, the UI MUST reuse that active tenant across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId` until the operator changes it
- **AND** the shared finance shell MUST expose at most one compact workspace switcher for changing tenants on normal finance routes
- **AND** dashboard panels MUST NOT reintroduce unrelated duplicate tenant picker or tenant workspace chrome

### Requirement: Local Finance Dates And Synchronized Current-Month Controls
The Finance area SHALL present human-readable local dates while keeping the existing reporting request semantics deterministic.

#### Scenario: Finance views render local dates instead of raw ISO strings
- **WHEN** a finance page shows operator-facing dates or timestamps such as reporting periods, invite times, missing-FX diagnostics, connection schedule times, or similar finance metadata
- **THEN** the UI MUST render those values using a standard user-local date or date-time format rather than raw ISO strings
- **AND** the underlying API and persistence semantics MUST remain unchanged

#### Scenario: Current-month mode keeps visible date controls aligned
- **WHEN** the finance dashboard is in `current_month` mode on first load or after the operator reactivates that mode
- **THEN** the visible start and end date controls MUST show the current month's active reporting bounds
- **AND** the visible picker state MUST stay synchronized when the operator switches to previous month, next month, or a custom range

