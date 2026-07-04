## MODIFIED Requirements

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

#### Scenario: Tenant management covers members and invites
- **WHEN** an authenticated operator opens `#/finance/tenants`
- **THEN** the UI MUST support tenant selection, tenant creation, invite creation, invite acceptance/join, and visible member lists for the selected tenant

#### Scenario: Dashboard shows period-aware finance summaries
- **WHEN** an authenticated tenant member opens `#/finance`
- **THEN** the UI MUST show reporting-period controls, KPI cards, charts or summary visuals, exact-value supporting tables/lists, sync/import alerts, and missing-FX diagnostics for the selected tenant

#### Scenario: Accounts and transactions use focused detail flows
- **WHEN** a tenant member manages accounts or transactions
- **THEN** the UI MUST provide focused list/detail routes for accounts and transactions, filtering/sorting/edit/hide/link flows for transactions, explicit visual state for pending/hidden/transfer/refund/reconciliation records, and category assignment controls
- **AND** the transactions list route MUST stay focused on browsing, filtering, sorting, and navigation into create/edit flows instead of embedding the create form directly in the list page
- **AND** the transaction editor MUST be reused for both `#/finance/transactions/new` and `#/finance/transactions/:transactionId`, with create mode initializing a blank editable record and edit mode prefilling the existing editable values
- **AND** the shared transaction editor MUST provide explicit save and cancel actions, show provider-original values when present so operator-edited reporting fields remain distinguishable from synced provider data, and remain usable in a mobile-friendly single-record layout
- **AND** the UI MUST prefer separate detail routes and stacked summaries over dense split-pane workspaces

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
