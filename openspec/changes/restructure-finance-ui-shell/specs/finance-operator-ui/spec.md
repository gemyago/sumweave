## MODIFIED Requirements

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
