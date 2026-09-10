# finance-operator-ui Specification

## Purpose
TBD - created by archiving change add-finance-management-slice. Update Purpose after archive.
## Requirements
### Requirement: Distinct Protected Finance Area
The UI SHALL provide a distinct protected Finance area alongside retained generic agent and administration surfaces.

#### Scenario: Finance navigation is tenant-aware and protected
- **WHEN** an authenticated operator uses the application navigation
- **THEN** the UI MUST provide a top-level Finance entry and protected tenant-aware routes including `#/finance`, `#/finance/tenants`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/connections`, `#/finance/connections/synthetic`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/rules`, `#/finance/imports`, and `#/finance/jobs/:jobId`
- **AND** unauthenticated access to those routes MUST redirect through the existing protected-route behavior
- **AND** all protected `#/finance*` routes, including account detail, transaction create/edit, finance job detail, synthetic setup, and rules routes, MUST render inside a dedicated finance-first shell with shared finance chrome rather than repeating the current in-page finance sub-navigation card on each page.

#### Scenario: Finance navigation stays focused on finance destinations
- **WHEN** finance screens are added to the SPA
- **THEN** they MUST remain visually and navigationally distinct from retained Chat, Providers, and Admin workflows
- **AND** the finance shell MUST expose only supported finance destinations for this slice, mapping the rail to real product routes such as dashboard, transactions, accounts, categories, rules, connections and sync, imports, and tenants
- **AND** unsupported reference items such as `Settings` MUST remain out of scope until backed by real product workflows.

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
- **AND** the PKO flow MUST start the Enable Banking redirect/SCA flow, handle the return state/code, and surface success or recoverable failure without exposing decrypted secrets or raw provider documents
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
- **WHEN** a finance sync, FX refresh, or import publishes job-observed work
- **THEN** the finance workflow MUST expose job status plus a route link to a finance-focused job detail or the generic admin job detail without losing operator context
- **AND** the returned dispatch ID MAY return `404` until worker delivery, which the
  initiating workflow MUST render as pending while arbitrary/deep-linked `404`
  responses remain errors

#### Scenario: Admin diagnostics expose sanitized operational state
- **WHEN** an authenticated operator opens `#/admin`, `#/admin/finance/fx`, or `#/admin/finance/providers`
- **THEN** the UI MUST show operational diagnostics such as failed jobs, missing FX coverage, stale connections, provider health, and manual sync affordances where supported
- **AND** admin diagnostics MUST make scheduler state and recent scheduled-run visibility observable without replacing tenant-facing bank-connection schedule management
- **AND** it MUST NOT display decrypted secrets or raw provider documents by default

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

### Requirement: Finance Rules Management And Classification Feedback
The Finance UI SHALL let the active tenant manage deterministic classification rules, optionally create a rule after manual category assignment, and explicitly classify an inclusive local-date range with existing job feedback.

#### Scenario: Operator manages ordered rules
- **WHEN** an authenticated tenant member opens `#/finance/rules`
- **THEN** the page MUST list rules in evaluation order with match type, condition, target category name, optional target tag names, and position
- **AND** it MUST support create, edit, delete, move-up, and move-down controls with recoverable pending and error states
- **AND** successful mutations MUST refetch the ordered rule list
- **AND** create/edit forms MUST provide a required category and optional existing-tag selection, including an empty tag set.

#### Scenario: Category removal is blocked by rules
- **WHEN** category removal receives `category_referenced_by_classification_rules`
- **THEN** the UI MUST explain that the category cannot be removed until its rules are retargeted or deleted
- **AND** it MUST identify or link to the referencing rules without hiding the category.

#### Scenario: Manual assignment offers optional rule creation
- **WHEN** a manual transaction category assignment or category change succeeds
- **THEN** the transaction UI MUST show a compact dismissible `Create rule from this transaction` action attached to that transaction
- **AND** it MUST NOT expand the rule form, move keyboard focus, or scroll automatically
- **AND** the behavior MUST be available in shared transaction lists and the full create/edit transaction editor
- **AND** the transaction assignment MUST stay independent of rule creation.

#### Scenario: Replacement manual-rule offer starts collapsed
- **WHEN** another successful category assignment starts a replacement offer in the same list or editor
- **THEN** the UI MUST replace the previous offer and any draft with a new collapsed offer attached to the newly saved transaction
- **AND** replacement MUST occur even when the saved description, category, and tags equal the previous offer's values.

#### Scenario: Explicit classification defaults to thirty local dates
- **WHEN** the rules page first presents explicit classification controls
- **THEN** it MUST show today and the preceding 29 local calendar dates as the selected inclusive start and end dates
- **AND** it MUST allow a wider range and reject a start date after the end date before submission.

#### Scenario: Local dates become half-open timestamp bounds
- **WHEN** the operator submits the displayed inclusive date range
- **THEN** the UI MUST send local start-of-day for the first date and local start-of-day after the last date as full RFC 3339 timestamps with their correct offsets
- **AND** it MUST use calendar arithmetic so daylight-saving transitions retain the correct boundary offsets.

#### Scenario: Explicit classification is observed
- **WHEN** the API returns a classification `jobId`
- **THEN** the initiating UI MUST treat only that ID's pre-materialization `404` as pending and poll the existing queued, running, succeeded, or failed lifecycle
- **AND** it MUST retain a Finance job-detail link, refresh the ledger after success, and explain on failure that some transactions may already have been classified.

#### Scenario: Classification UI is verified visually
- **WHEN** the classification management and feedback surfaces are ready for review
- **THEN** they MUST follow the canonical Bootstrap Finance shell and responsive behavior without route-local layout styles
- **AND** their common desktop and narrow-screen flows MUST pass the repository's required UI visual-review and manual smoke loops after concrete findings are resolved.

#### Scenario: Tag and description saves preserve the compact offer
- **WHEN** an operator edits and saves tags or description on the transaction with an active collapsed offer
- **THEN** the same compact offer MUST stay available without opening the form
- **AND** successful additions and removals MUST be reflected in the saved transaction used when opening the offer
- **AND** failed saves MUST NOT change the rule's source defaults.

#### Scenario: Opening captures the latest saved classification
- **WHEN** the operator clicks the compact action after saving the category and optional tags
- **THEN** the UI MUST open an editable rule draft initialized with `contains`, the latest saved description, category, and all saved tag IDs
- **AND** the operator MUST be able to change exact/contains matching, condition, category, and tags before explicitly saving
- **AND** unsaved transaction edits MUST NOT become rule defaults
- **AND** opening MUST be disabled while a transaction save is pending.

#### Scenario: Open draft is independent and recoverable
- **WHEN** the operator edits the opened rule draft, ordinary rerenders or tag/description saves occur, or a rule-save request fails
- **THEN** the UI MUST retain the draft's operator-edited values
- **AND** a save failure MUST show a recoverable error without changing the saved transaction
- **AND** a successful rule save MUST submit the displayed category and complete optional tag selection and clear the offer.

#### Scenario: Dismissal and cancellation are optional
- **WHEN** the operator dismisses the compact offer or cancels its rule form
- **THEN** the UI MUST clear the offer without changing the saved transaction
- **AND** later tag-only or description-only saves MUST NOT resurrect that offer
- **AND** a later successful category assignment MUST be able to start a new offer.

#### Scenario: Offer remains scoped to a categorized source transaction
- **WHEN** the source category is successfully cleared, the tenant or detail record changes, or the source transaction leaves the rendered list
- **THEN** the offer and any rule draft MUST be cleared
- **AND** clearing a category MUST NOT start a new offer.

#### Scenario: Tag catalog cannot silently erase selections
- **WHEN** the rule editor cannot resolve selected tag IDs because its catalog is unavailable or an assigned tag is no longer assignable
- **THEN** the UI MUST retain those selections and show recoverable catalog or selection feedback
- **AND** it MUST require a successful refresh or explicit removal of unavailable selections before saving an invalid draft
- **AND** a successfully loaded empty catalog MUST allow a category-only rule.

### Requirement: Tenant Ledger Explicit Transfer Matching
The Finance transaction ledger SHALL let the active tenant submit an inclusive local-date range for transfer matching and observe the resulting work through existing Finance job feedback.

#### Scenario: Matching range defaults to thirty local dates
- **WHEN** the tenant ledger first presents the **Match transfers** action
- **THEN** it MUST show today and the preceding 29 local calendar dates as inclusive start and end values
- **AND** it MUST allow a wider historical range and reject invalid dates or a start after the end before submission.

#### Scenario: Matching scope is tenant-wide
- **WHEN** the operator reviews the matching form before submission
- **THEN** the UI MUST explain that matching searches all accounts in the tenant and may link a partner up to 72 hours outside the selected dates
- **AND** account, search, source, sort, and current-page ledger filters MUST NOT narrow the submitted matching scope.

#### Scenario: Local dates become half-open timestamp bounds
- **WHEN** the operator submits the inclusive date range
- **THEN** the UI MUST send local start-of-day for the first date and local start-of-day after the last date as full RFC 3339 timestamps with their correct offsets
- **AND** it MUST use calendar arithmetic so daylight-saving transitions retain correct boundary offsets.

#### Scenario: Explicit matching is observed
- **WHEN** the API returns a matching `jobId`
- **THEN** the initiating flow MUST treat only that ID's pre-materialization `404` as pending, prevent duplicate submission while observing it, and show queued, running, succeeded, or failed lifecycle with a Finance job-detail link
- **AND** success MUST say “Transfer matching completed.” without claiming every transfer was found or exposing unstored counts.

#### Scenario: Terminal matching refreshes the initiating tenant
- **WHEN** the observed matching job succeeds or fails
- **THEN** the ledger MUST refresh for the tenant that initiated the run even if active tenant selection changed during observation
- **AND** failure MUST say “Transfer matching failed. Some pairs may already have been matched. Running it again preserves existing pairs.” and retain rerun availability.

#### Scenario: Manual correction remains silent
- **WHEN** the operator inspects, manually links, or unlinks transfer partners after automatic matching
- **THEN** the existing detail workflow MUST remain available without an automatic-exclusion warning, explanatory copy, or exclusion indicator.

#### Scenario: Matching UI is verified visually
- **WHEN** the ledger matching surface is ready for review
- **THEN** it MUST follow the canonical Bootstrap Finance shell and responsive behavior without route-local layout styles
- **AND** its common desktop and narrow-screen paths MUST pass the required independent UI design review and headed manual smoke loops after concrete findings are resolved.

### Requirement: Dashboard Cash-Flow Time-Series Visualization
The Finance dashboard SHALL present selected-period settled income and expense as a responsive grouped time-series chart backed by the separate cash-flow series API.

#### Scenario: Dashboard replaces the aggregate progress-bar visual
- **WHEN** a tenant dashboard and its cash-flow series load successfully
- **THEN** the dashboard MUST replace the existing settled and pending progress-bar visual with adjacent income and expense bars for every returned bucket
- **AND** the chart MUST use exact formatted values in its tooltip and visually distinct success and danger tones
- **AND** pending values MUST remain available through the existing summary rather than becoming a chart series

#### Scenario: Dashboard chooses grouping from the selected period
- **WHEN** the operator selects current, previous, or next month
- **THEN** the UI MUST request daily buckets using the same full timestamp bounds as the selected dashboard period
- **AND** when the operator selects six months, twelve months, or a custom inclusive range spanning more than 31 browser-local calendar dates, the UI MUST request monthly buckets
- **AND** a custom inclusive range spanning at most 31 browser-local calendar dates MUST request daily buckets

#### Scenario: Longer period actions use aligned local bounds
- **WHEN** the operator selects **Last 6 months** or **Last 12 months**
- **THEN** the UI MUST calculate a browser-local start at the first day of the earliest included month and an exclusive end at the first day of the next month
- **AND** those actions MUST include the current month and its preceding five or eleven calendar months respectively
- **AND** the selected range MUST remain visible and persist through the existing dashboard URL range behavior

#### Scenario: Series request lifecycle is independent and current
- **WHEN** the cash-flow series is loading, empty, incomplete, fails, or a newer tenant or range selection supersedes it
- **THEN** the chart card MUST show its own honest loading, zero-activity, partial-data warning, or recoverable error with retry without making the remaining dashboard unavailable
- **AND** an incomplete series with zero values in every bucket MUST render its partial-data warning and missing-FX diagnostics before the zero-activity state so omitted valuations cannot appear as genuine inactivity
- **AND** a response for an obsolete tenant, range, or grouping MUST NOT replace the current series

#### Scenario: ECharts integration is reusable and bounded
- **WHEN** the UI renders the cash-flow chart
- **THEN** it MUST use Apache ECharts 6 through selective core imports and the SVG renderer without a third-party Svelte wrapper
- **AND** a shared Svelte chart component MUST own initialization, responsive resize, option updates, and disposal while finance-specific series construction remains dashboard-owned

#### Scenario: Cash-flow chart remains usable and accessible
- **WHEN** the chart renders on desktop or a narrow viewport
- **THEN** the period-performance card and cash-flow chart card MUST each span the full available dashboard content width at every breakpoint
- **AND** the chart MUST use responsive tick-label reduction, theme-compatible presentation, and an accessible chart name and textual alternative containing every bucket's range and exact values
- **AND** hovering any income or expense bar MUST keep both bar series visibly rendered in their normal opaque semantic tones while the exact-value tooltip is displayed
- **AND** canonical Finance markup MUST remain Bootstrap-first without route-local style blocks or inline layout styles
- **AND** the changed desktop and narrow-screen flows MUST pass the repository's required independent visual-review and manual smoke loops after concrete findings are resolved

