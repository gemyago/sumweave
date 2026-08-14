# finance-operator-ui Specification

## Purpose
TBD - created by archiving change add-finance-management-slice. Update Purpose after archive.
## Requirements
### Requirement: Distinct Protected Finance Area
The UI SHALL provide a distinct protected Finance area alongside retained generic agent and administration surfaces.

#### Scenario: Finance navigation is tenant-aware and protected
- **WHEN** an authenticated operator uses the application navigation
- **THEN** the UI MUST provide a top-level Finance entry and protected tenant-aware routes including `#/finance`, `#/finance/tenants`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/connections`, `#/finance/connections/synthetic`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/imports`, and `#/finance/jobs/:jobId`
- **AND** unauthenticated access to those routes MUST redirect through the existing protected-route behavior
- **AND** all protected `#/finance*` routes, including account detail, transaction create/edit, finance job detail, and synthetic setup routes, MUST render inside a dedicated finance-first shell with shared finance chrome rather than repeating the current in-page finance sub-navigation card on each page

#### Scenario: Finance navigation stays focused on finance destinations
- **WHEN** finance screens are added to the SPA
- **THEN** they MUST remain visually and navigationally distinct from retained Chat, Providers, and Admin workflows
- **AND** the finance shell MUST expose only supported finance destinations for this slice, mapping the rail to real product routes such as dashboard, transactions, accounts, categories, connections and sync, imports, and tenants
- **AND** unsupported reference items such as `Rules` or `Settings` MUST remain out of scope until backed by real product workflows

### Requirement: Finance Dashboard And Workspace Flows
The Finance area SHALL expose the first end-user workflows required by the finance design.

#### Scenario: Tenant management covers members and invites
- **WHEN** an authenticated operator opens `#/finance/tenants`
- **THEN** the UI MUST support tenant selection, tenant creation, invite creation, invite acceptance/join, and visible member lists for the selected tenant

#### Scenario: Dashboard shows period-aware finance summaries
- **WHEN** an authenticated tenant member opens `#/finance`
- **THEN** the UI MUST show a finance-dashboard hierarchy with a page header, reporting-period controls, visible range context, route actions, KPI cards, charts or summary visuals, exact-value supporting tables or lists, sync or import alerts, and missing-FX diagnostics for the selected tenant
- **AND** the dashboard layout MUST include account, category or spending, recent transaction, and sync-activity sections, using honest empty or reduced states when the selected tenant has no data for a section
- **AND** the dashboard MUST adopt the reference information architecture while preserving the existing Sumweave UI terminal-native design tokens and styling foundations unless a separate design-system rewrite is explicitly accepted

#### Scenario: Accounts and transactions use focused detail flows
- **WHEN** a tenant member manages accounts or transactions
- **THEN** the UI MUST provide focused list or detail routes for accounts and transactions, filtering, sorting, edit, hide, and category-assignment flows for transactions, explicit visual state for pending, hidden, transfer, refund, and reconciliation records, and clear navigation into dedicated create or edit routes
- **AND** the transactions list route MUST stay focused on browsing, filtering, sorting, and navigation into create or edit flows instead of embedding the create form directly in the list page
- **AND** the transactions browse route MUST provide a search or date or filter toolbar, summary chips, a selectable ledger-style results table, and a responsive contextual inspector for the selected transaction on wide screens
- **AND** the transaction editor MUST be reused for both `#/finance/transactions/new` and `#/finance/transactions/:transactionId`, with create mode initializing a blank editable record and edit mode prefilling the existing editable values
- **AND** the shared transaction editor MUST provide explicit save and cancel actions, show provider-original values when present so operator-edited reporting fields remain distinguishable from synced provider data, and remain usable in a mobile-friendly single-record layout
- **AND** the UI MUST keep full-record mutation flows on dedicated routes even if the browse route adds a lightweight contextual inspector for review or supported quick actions

#### Scenario: Imports and supported bank-linking are step-by-step workflows
- **WHEN** a tenant member links a supported bank provider or imports CSV data
- **THEN** the UI MUST present step-by-step flows with clear validation, preview, confirmation, recovery messaging, and observable async job status rather than one-shot opaque submission
- **AND** bank-linking flows MUST expose monobank token entry, PKO via Enable Banking redirect/SCA, and synthetic local configured setup as distinct supported choices
- **AND** bank-linking flows MUST NOT allow free-text bank provider entry
- **AND** the monobank flow MUST submit tokens only for the monobank provider option
- **AND** the PKO flow MUST start the Enable Banking redirect/SCA flow, handle the return state/code, and surface success or recoverable failure without exposing decrypted secrets or raw provider payloads
- **AND** the synthetic flow MUST start local redirect setup, let the operator configure one or more synthetic accounts, save pending configuration, finish the link, and return to the connection list
- **AND** bank-linking flows MUST retain attach-to-existing-account selection, re-authentication handling, and connection-detail schedule/sync visibility

#### Scenario: Synthetic setup supports refresh and retry
- **WHEN** an authenticated tenant member opens synthetic setup with a valid pending state
- **THEN** the UI MUST load existing pending synthetic account configuration when present
- **AND** the UI MUST keep the operator on the setup route with actionable validation or API errors when saving configuration or finishing the link fails
- **AND** after a successful finish, the UI MUST clear consumed setup state from the active route and show the created synthetic connection in the connection list

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

#### Scenario: Sole joined tenant is selected automatically
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to exactly one finance tenant
- **THEN** the UI MUST automatically use that tenant as the active finance workspace without requiring an extra selection step
- **AND** the resolved tenant MUST become visible through one shared finance-shell tenant control rather than route-by-route duplicate tenant pickers

#### Scenario: Multiple joined tenants are selected once and reused
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to multiple finance tenants
- **THEN** the UI MUST require one explicit active-tenant selection when no active tenant has been chosen yet
- **AND** after selection, the UI MUST reuse that active tenant across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId` until the operator changes it
- **AND** the shared finance shell MUST keep the active tenant visible and changeable without each route reintroducing unrelated duplicate picker chrome

#### Scenario: Finance deep links preserve the requested route
- **WHEN** an authenticated operator opens `#/finance/accounts/:accountId`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, or `#/finance/jobs/:jobId` directly
- **THEN** the UI MUST apply the same active-tenant auto-selection or explicit-selection rules used by other finance routes before loading tenant-specific finance context
- **AND** once the active tenant is resolved, the UI MUST continue on the originally requested deep link instead of redirecting the operator to another finance page
- **AND** when explicit tenant selection is still required, the finance shell MUST keep the requested route context visible so the operator can resolve tenant choice without losing the intended destination

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

### Requirement: Tenant Management Supports Updates And Bounded Currency Selection
The Finance tenants route SHALL let operators create and update tenants using predefined valid display-currency choices instead of free-text currency fields.

#### Scenario: Tenant create uses a supported currency selector
- **WHEN** an authenticated operator opens `#/finance/tenants` to create a finance tenant
- **THEN** the create form MUST present display currency as a select control populated from the predefined valid tenant currency-code list
- **AND** the form MUST submit the selected currency code rather than arbitrary free text

#### Scenario: Selected tenant can be updated
- **WHEN** an authenticated tenant member has selected a tenant on `#/finance/tenants`
- **THEN** the UI MUST provide an edit form for the selected tenant name and display currency
- **AND** the display-currency control MUST use the same predefined valid tenant currency-code list as tenant creation
- **AND** saving the form MUST call the tenant update API and refresh the visible selected tenant state after success

#### Scenario: Tenant update failures are recoverable
- **WHEN** tenant update fails because validation, authentication, authorization, or network handling rejects the request
- **THEN** the UI MUST keep the operator on `#/finance/tenants`
- **AND** it MUST show a recoverable error state without losing the current selected tenant context

### Requirement: Finance Details Expose Current Provider Source Data
The Finance UI SHALL expose current schema-derived provider snapshots as provider source data for linked accounts and provider-synced transactions.

#### Scenario: Account detail lists distinct current snapshot kinds
- **WHEN** a tenant member expands provider source data on a linked finance account
- **THEN** the UI MUST lazily load current provider snapshot metadata for that account
- **AND** it MUST present account and account-balance snapshots as distinct rows when both are available
- **AND** each row MUST identify its snapshot kind, provider object, and capture time

#### Scenario: Transaction detail exposes its complete supported provider item
- **WHEN** a tenant member expands provider source data on a provider-synced transaction
- **THEN** the UI MUST lazily list the current transaction snapshot and allow an explicit detail reveal
- **AND** the revealed data MUST be the sanitized schema-derived provider transaction document returned by the protected API

#### Scenario: Source-data terminology is explicit
- **WHEN** account or transaction provider snapshots are presented
- **THEN** user-facing labels MUST use “Provider source data” or “Provider snapshot” terminology
- **AND** the UI MUST explain that the displayed document is the latest schema-derived provider snapshot rather than a raw HTTP response
- **AND** evidence and raw-payload terminology MUST NOT remain on the affected account or transaction surfaces

#### Scenario: Snapshot access stays bounded and recoverable
- **WHEN** provider source metadata or document loading is pending, empty, or fails
- **THEN** the UI MUST preserve its collapsed-by-default disclosure behavior and show bounded loading, empty, or recoverable error feedback
- **AND** it MUST NOT expose a provider snapshot history timeline
- **AND** it MUST NOT display decrypted credentials, authorization material, or other provider secrets

