## ADDED Requirements

### Requirement: Distinct Protected Finance Area
The operator UI SHALL provide a distinct protected Finance area rather than mixing finance workflows into trading/operator routes.

#### Scenario: Finance navigation is tenant-aware and protected
- **WHEN** an authenticated operator uses the application navigation
- **THEN** the UI MUST provide a top-level Finance entry and protected tenant-aware routes including `#/finance`, `#/finance/tenants`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/connections`, `#/finance/transactions`, `#/finance/categories`, `#/finance/imports`, and `#/finance/jobs/:jobId`
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
- **THEN** the UI MUST provide focused list/detail routes for accounts, filtering/sorting/edit/hide/link flows for transactions, explicit visual state for pending/hidden/transfer/refund/reconciliation records, and category/tag assignment controls
- **AND** the UI MUST prefer separate detail routes and stacked summaries over dense split-pane workspaces

#### Scenario: Imports and bank-linking are step-by-step workflows
- **WHEN** a tenant member links a provider or imports CSV data
- **THEN** the UI MUST present step-by-step flows with clear validation, preview, confirmation, recovery messaging, and observable async job status rather than one-shot opaque submission
- **AND** bank-linking flows MUST explicitly cover Enable Banking redirect/SCA start-return handling, monobank token entry, attach-to-existing-account selection, re-authentication handling, and connection-detail schedule/sync visibility

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
