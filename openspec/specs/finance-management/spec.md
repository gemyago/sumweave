# finance-management Specification

## Purpose
TBD - created by archiving change add-finance-management-slice. Update Purpose after archive.
## Requirements
### Requirement: Finance Module Boundary And Persistence Ownership
The system SHALL implement finance as a root `finance/` product module that remains architecturally independent from the trading runtime.

#### Scenario: App composes finance without reverse runtime imports
- **WHEN** finance functionality is wired into the product
- **THEN** `apps/signal-foundry/` MUST compose both `runtime/` and `finance/`
- **AND** `finance/` MUST NOT import `runtime/`
- **AND** finance business rules MUST live in `finance/` while auth, process lifecycle, generic jobs runtime, and HTTP route glue remain app-owned

#### Scenario: Finance persistence stays finance-owned and auto-migrated
- **WHEN** finance data is persisted
- **THEN** finance-owned tables MUST use `finance_` prefixes, GORM auto-migrate schema initialization, explicit column names, and UTC-first timestamps
- **AND** finance domain models MUST remain separate from GORM persistence models
- **AND** the storage design MUST stay compatible with SQLite local development and PostgreSQL-oriented production use

### Requirement: Tenant, Account, Category, And Tag Management
The finance module SHALL support tenant-based personal-finance ownership and tenant-local finance catalogs.

#### Scenario: Users create and join finance tenants
- **WHEN** an authenticated user creates a tenant or accepts an invite
- **THEN** the system MUST create or join a finance tenant with a user-friendly name and one display currency
- **AND** all tenant members MUST be equal in the first implementation

#### Scenario: New tenants receive tenant-local default categories and tags
- **WHEN** a finance tenant is created
- **THEN** the system MUST copy system default categories and default tags into that tenant
- **AND** the seeded category baseline MUST stay flat and cover common household finance needs across income, housing, utilities, food, transportation, health, insurance, education or childcare, pets, personal care, entertainment, shopping, home, travel, gifts or donations, taxes or fees, debt payments, and miscellaneous spending
- **AND** the seeded tag baseline MUST cover cross-category reporting uses such as tax, reimbursements, split or shared spending, business use, subscriptions, and travel
- **AND** transfer, reconciliation, and opening-balance semantics MUST remain explicit system transaction behavior rather than seeded user categories
- **AND** later changes to system defaults MUST NOT mutate existing tenant-local categories or tags

#### Scenario: Accounts remain tenant-owned and attachable
- **WHEN** a tenant member creates or links accounts
- **THEN** every finance account MUST belong to exactly one tenant
- **AND** the system MUST support manual, linked-bank, imported, and reconciliation-style account shapes
- **AND** bank-linking flows MUST be able to attach a linked provider account to an existing manual account instead of always creating a duplicate account

### Requirement: Ledger-Driven Transaction Semantics
The finance module SHALL treat transactions as the explainable ledger source of truth for balances and reporting.

#### Scenario: Balances are derived from transactions
- **WHEN** a user needs to correct an account balance
- **THEN** the system MUST use visible reconciliation or opening-balance transactions rather than directly mutating a balance field

#### Scenario: Synced transactions preserve provider truth and user edits
- **WHEN** provider-synced transactions are imported and later edited by a user
- **THEN** the system MUST retain provider-original values and raw payloads separately from user-edited presentation/reporting fields
- **AND** later syncs MUST NOT silently overwrite user corrections

#### Scenario: Reporting semantics stay explicit for special transaction kinds
- **WHEN** transactions are refunds, matched internal transfers, unmatched external transfers, reconciliations, pending items, or hidden/deleted items
- **THEN** refunds MUST reduce expense in the assigned category, matched internal transfers MUST be excluded from income/expense reporting, reconciliations MUST be visible but excluded from income/expense reporting, pending items MUST be visible but excluded from settled totals by default, and hidden/deleted items MUST be excluded from normal user views while retained for audit and sync idempotency

#### Scenario: Transaction edits stay limited to user-controlled reporting fields
- **WHEN** an authenticated tenant member updates an existing transaction
- **THEN** the system MUST allow edits to `description`, `amountMinor`, `effectiveAt`, and category assignment or category removal for that transaction
- **AND** any replacement category MUST belong to the same tenant as the transaction
- **AND** the system MUST preserve account identity, source, status, kind, currency, transfer linkage, hidden state, and provider-original values unless another dedicated workflow changes them

### Requirement: Reproducible Finance Reporting And FX Conversion
The finance module SHALL provide tenant display-currency reporting backed by persisted FX data.

#### Scenario: Dashboard reporting uses persisted FX rates
- **WHEN** a tenant requests finance summaries for a reporting period
- **THEN** the system MUST compute display-currency totals from persisted FX-rate records rather than only live rates
- **AND** the response MUST still expose native-currency account context where helpful

#### Scenario: Missing FX rates remain visible
- **WHEN** required FX rates are missing for part of the requested reporting scope
- **THEN** the system MUST surface explicit missing-rate warnings and diagnostics instead of silently fabricating or omitting converted totals

### Requirement: Finance Sync, Secrets, And Imports
The finance module SHALL support secure provider linking plus explicit async sync/import workflows.

#### Scenario: Provider credentials are encrypted and redacted
- **WHEN** bank connections or provider sessions are stored and later surfaced through logs, APIs, jobs, or admin screens
- **THEN** provider credentials and decrypted secrets MUST be encrypted at rest, MUST NOT be logged or returned by default, and MUST keep enough metadata to expose re-authentication needs

#### Scenario: Supported bank linking methods are explicit
- **WHEN** a tenant member starts a bank-linking workflow
- **THEN** the system MUST expose monobank as a token-based bank connection option
- **AND** the system MUST expose PKO as a redirect/SCA bank connection option implemented through Enable Banking
- **AND** token linking MUST reject any bank provider other than monobank before storing secrets or calling a provider
- **AND** redirect/SCA linking MUST reject any bank provider other than PKO before storing secrets or calling a provider
- **AND** local or fake-provider PKO redirect/SCA validation MUST NOT require a browser-trusted HTTPS certificate when the callback URL uses localhost or a loopback address

#### Scenario: PKO redirect link is completed from a pending start
- **WHEN** a tenant member returns from Enable Banking SCA for PKO with a valid state and code
- **THEN** the system MUST resolve the server-side pending PKO link start for the same tenant and actor
- **AND** the system MUST create the bank connection through the Enable Banking finish-link path
- **AND** the system MUST consume or expire the pending link start so the same state cannot create duplicate connections

#### Scenario: Bank sync remains idempotent and job-backed
- **WHEN** a tenant member triggers or schedules a bank sync for a linked monobank or PKO connection
- **THEN** the system MUST execute the sync through durable finance jobs, persist normalized accounts/transactions, balance snapshots, provider-original identifiers, and raw payloads, and deduplicate by provider/connection/account/provider-transaction identity plus safe fallback fingerprints when needed

#### Scenario: Provider sync v2 models observations separately from ledger records
- **WHEN** the finance module defines provider sync v2 data shapes
- **THEN** provider connectors MUST return normalized provider account, balance, transaction, and raw payload observations rather than directly returning ledger transaction persistence models
- **AND** provider observation types MUST be clearly named with `Provider` or `ProviderSync` prefixes so they remain distinct from user-facing finance ledger types
- **AND** provider connectors MUST NOT persist finance records directly

#### Scenario: Provider sync v2 tracks per-connection latest state journal
- **WHEN** provider sync v2 records sync progress
- **THEN** sync state journal rows MUST be scoped to a bank connection
- **AND** the system MUST append one latest-state row per attempted chunk window rather than only per succeeded chunk
- **AND** the system MUST be able to load the newest appended state row for that connection
- **AND** each state row MUST keep the attempted window bounds, attempt time, nullable success time, run or job identity, error summary, and aggregate sync stats where available

#### Scenario: Provider sync v2 journal rows always describe a concrete attempted window
- **WHEN** provider sync v2 appends a state row for a chunk attempt
- **THEN** that row MUST store the concrete attempted window start and end even when the attempt fails
- **AND** window bounds MUST NOT depend on whether `SucceededAt` is populated

#### Scenario: Provider sync v2 latest state is interpreted through nullable success
- **WHEN** provider sync v2 loads the newest appended state row for a connection
- **THEN** `SucceededAt` populated on that row MUST mean the latest known attempt completed successfully
- **AND** `SucceededAt` absent on that row MUST mean the latest known attempt did not record success
- **AND** planning and retry logic MUST treat that distinction explicitly instead of assuming the newest row is always a succeeded checkpoint

#### Scenario: Provider sync v2 lets target-window policy interpret the latest loaded state
- **WHEN** provider sync v2 decides the next target window
- **THEN** target-window planning MUST consume the latest loaded sync state directly
- **AND** orchestration MUST NOT construct a synthetic future state by copying prior success fields into the current attempt before any chunk is known
- **AND** concrete attempt state rows MUST be created only when an exact chunk window is being executed

#### Scenario: Provider sync v2 treats empty journal state as no prior state
- **WHEN** a bank connection has no state rows in the provider sync state journal
- **THEN** the system MUST return no latest sync state for that connection
- **AND** planning the next sync session MUST treat that connection as having no prior journal state

#### Scenario: Provider sync v2 plans diffs before applying writes
- **WHEN** provider sync v2 compares provider observations with persisted data
- **THEN** it MUST load an existing-window snapshot for the connection using a candidate lookup window that may be wider than the requested provider sync window
- **AND** it MUST produce an explicit diff plan before persistence writes are applied
- **AND** the diff planner MUST be pure, deterministic, and free of persistence writes or provider network calls

#### Scenario: Provider sync v2 handles ambiguous transaction matches conservatively
- **WHEN** a provider transaction observation has only weak or ambiguous persisted transaction candidates
- **THEN** the diff plan MUST create a new transaction action instead of merging into an existing transaction
- **AND** the sync stats MUST count the action as an ambiguous-created transaction

#### Scenario: Provider sync v2 preserves user-edited transaction fields
- **WHEN** a provider transaction observation strongly matches an existing provider-synced transaction
- **THEN** provider-original fields MUST be refreshed from the new observation
- **AND** a user-facing transaction field MUST be updated from the provider observation only when the current field value still matches the previous provider-original value
- **AND** a user-facing transaction field MUST be preserved when it differs from the previous provider-original value

#### Scenario: Provider sync v2 separates product providers from technical connectors
- **WHEN** PKO sync is represented in provider sync v2
- **THEN** PKO MUST be modeled as product provider `pko`
- **AND** Enable Banking MUST be modeled as technical connector `enable-banking`
- **AND** the relation between PKO and Enable Banking MUST be composition through a provider profile rather than inheritance or user-facing connector exposure

#### Scenario: Scheduled sync management scope stays explicit
- **WHEN** finance scheduling features are surfaced
- **THEN** tenant-facing finance workflows MUST manage per-connection bank sync schedules, schedule state, and next/last sync visibility from finance connection screens
- **AND** admin/diagnostics workflows MUST provide sanitized cross-cutting visibility plus global FX schedule controls without becoming the primary tenant workflow for bank-connection schedule editing

#### Scenario: CSV import uses preview then durable execution
- **WHEN** a tenant member imports account or transaction CSV data
- **THEN** the system MUST require header detection, mapping confirmation, validation preview, duplicate/would-create visibility, and explicit confirmation before it enqueues `finance.csv_import` or `finance.account_import`
- **AND** durable import execution MUST expose progress, result summary, and rejected-row visibility

### Requirement: Protected Finance HTTP API
The backend application SHALL expose finance APIs through the existing app under `/api/v1/finance/...`.

#### Scenario: Finance APIs are authenticated and tenant-aware
- **WHEN** a caller without a valid authenticated identity calls finance endpoints
- **THEN** the system MUST reject the request as unauthorized
- **AND** when an authenticated caller accesses finance endpoints, tenant-scoped reads and writes MUST remain isolated to the caller's joined tenants

#### Scenario: Finance transaction detail API preserves tenant scope and editor context
- **WHEN** an authenticated tenant member calls `GET /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`
- **THEN** the system MUST return the selected transaction in camelCase JSON with `providerOriginal` included when present
- **AND** the read flow MUST reject transaction identifiers that do not belong to the selected tenant

#### Scenario: Finance transaction update API preserves tenant scope and reporting semantics
- **WHEN** an authenticated tenant member calls `PATCH /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`
- **THEN** the system MUST update only the transaction's user-editable reporting fields `description`, `amountMinor`, `effectiveAt`, and nullable `categoryId`
- **AND** the response MUST return the updated transaction in camelCase JSON with `providerOriginal` included when present
- **AND** the update flow MUST reject transaction or category identifiers that do not belong to the selected tenant

#### Scenario: Finance API covers required product areas
- **WHEN** the first finance slice is implemented
- **THEN** the API surface MUST cover tenants, tenant members/invites, accounts, bank connections, transactions including list/detail/create/update flows, categories, tags, dashboard/reporting, FX diagnostics and sync, CSV import preview/confirm/status, and finance-related job deep-linking
- **AND** all operator-facing JSON fields MUST use camelCase

### Requirement: Finance Schema Is Prepared Explicitly
The backend application SHALL include finance-owned persistence schema initialization in the explicit backend database migration command.

#### Scenario: Migration creates finance-owned tables
- **WHEN** a user runs `signal-foundry db-migrate` with valid backend database configuration
- **THEN** the command MUST run the finance persistence migration for finance-owned tables before finance API, import, reporting, sync, or finance durable job flows rely on those tables
- **AND** finance-owned tables MUST keep finance persistence ownership, explicit column names, UTC-first timestamps, and compatibility with SQLite local development and PostgreSQL-oriented production use

#### Scenario: Finance startup relies on prepared schema in standard setup
- **WHEN** the documented standard setup has run `signal-foundry db-migrate`
- **THEN** finance service registration and finance job handler registration MUST rely on the prepared finance schema
- **AND** they MUST NOT create or update finance tables implicitly during startup

### Requirement: Provider Sync V2 Window Sync Executor Resolves Technical Connectors
The finance module SHALL resolve the provider sync v2 fetch connector from the persisted bank connection's technical `ConnectorID` before the window sync executor performs provider fetch orchestration.

#### Scenario: Monobank connections resolve their direct connector
- **WHEN** provider sync v2 coordinates a bank connection whose `ConnectorID` is `monobank`
- **THEN** the window sync executor MUST resolve the `monobank` technical connector before fetch begins
- **AND** it MUST use that resolved connector instead of branching on product-provider-specific sync code

#### Scenario: PKO connections sync through Enable Banking
- **WHEN** provider sync v2 coordinates a bank connection whose product provider is `pko` and whose `ConnectorID` is `enable-banking`
- **THEN** the window sync executor MUST resolve the `enable-banking` technical connector for the fetch step
- **AND** PKO MUST remain modeled as a product provider composed through the Enable Banking connector rather than as a user-visible technical connector

#### Scenario: Unknown connectors fail before provider fetch
- **WHEN** provider sync v2 coordinates a bank connection whose `ConnectorID` is empty, unknown, or not configured in the runtime registry
- **THEN** the window sync executor MUST fail before any provider fetch call is attempted
- **AND** the failure MUST identify connector resolution as the cause without exposing secrets or raw provider payload content

