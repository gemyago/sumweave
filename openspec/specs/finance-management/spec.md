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

#### Scenario: Provider sync v2 tracks per-connection state
- **WHEN** provider sync v2 records sync progress
- **THEN** sync state MUST be scoped to a bank connection
- **AND** it MUST keep attempt time, success time, successful window coverage, run or job identity, error summary, and aggregate sync stats where available

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

#### Scenario: Provider sync v2 real-bank connectors stay independent from legacy provider implementations
- **WHEN** finance composes the `monobank` or `enable-banking` provider sync v2 connectors
- **THEN** each connector MUST implement its own link and fetch behavior inside the v2 connector seam rather than wrapping legacy root `finance` bank-provider implementations
- **AND** the legacy root bank-provider implementations MUST remain removable after runtime cutover without changing the v2 connector contract

#### Scenario: Provider sync v2 Monobank connector stays token-based
- **WHEN** finance composes the `monobank` provider sync v2 connector
- **THEN** that connector MUST support token linking and requested-window fetch
- **AND** it MUST reject redirect start and redirect finish operations as unsupported for Monobank
- **AND** requested-window fetch MUST return provider account, transaction, and raw-payload observations without direct ledger persistence

#### Scenario: Provider sync v2 Enable Banking connector stays redirect-based
- **WHEN** finance composes the `enable-banking` provider sync v2 connector for PKO
- **THEN** that connector MUST support redirect start, redirect finish, and requested-window fetch
- **AND** it MUST reject token linking as unsupported for Enable Banking
- **AND** requested-window fetch MUST return provider account, balance, transaction, and raw-payload observations without direct ledger persistence

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

### Requirement: Provider Sync V2 Supports Synthetic Provider Storage And Generation
The finance module SHALL support a core-only `synthetic` bank provider through provider sync v2, including provider-owned linking configuration and dedicated synthetic-provider storage for generation history.

#### Scenario: Synthetic connection is linked from configured accounts
- **WHEN** core finance code links a synthetic bank connection with a list of account names and currencies
- **THEN** the system MUST create an active bank connection with product provider `synthetic` and connector `synthetic`
- **AND** the account configuration MUST require each generated account to have a non-empty display name and currency
- **AND** the account configuration MUST be persisted as provider-owned connection data without adding a user-facing UI or public HTTP linking surface

#### Scenario: Synthetic storage round-trips generation state
- **WHEN** core finance code saves synthetic-provider state for a linked synthetic connection
- **THEN** the system MUST persist configured accounts, generated-window history, repeat counters, and transaction sequence counters in synthetic-owned storage scoped to that connection
- **AND** loading synthetic-provider state for the connection MUST return the same provider-owned state without requiring provider-specific fields in the common provider sync state journal

#### Scenario: Common provider sync journal remains provider-agnostic
- **WHEN** provider sync v2 appends a sync state row for a synthetic connection
- **THEN** the row MUST describe the concrete attempted window, attempt outcome, job or run identity, error summary, and aggregate stats
- **AND** it MUST NOT require an opaque provider-specific state blob to support synthetic generation

#### Scenario: Synthetic fetch uses dedicated synthetic storage
- **WHEN** provider sync v2 executes a sync window for a synthetic connection
- **THEN** the synthetic provider MUST load configured accounts and generation history from synthetic-owned storage
- **AND** after generating observations it MUST persist updated generation history back to synthetic-owned storage
- **AND** it MUST NOT persist normalized finance ledger records directly from the connector

#### Scenario: Synthetic first run generates daily account transactions
- **WHEN** the synthetic provider fetches a requested sync window that is not recorded in its provider-specific state
- **THEN** it MUST return provider account observations for the configured accounts
- **AND** it MUST generate from 1 to 2 booked transaction observations for each configured account for each UTC day in the requested window
- **AND** generated transaction observations MUST include provider account IDs, provider transaction IDs, amount, currency, description, effective time, fingerprint, provider-original values, and raw payload evidence
- **AND** the updated provider-specific state MUST record the generated requested window and transaction sequence information needed by later fetches

#### Scenario: Synthetic repeated run generates only the last day
- **WHEN** the synthetic provider fetches a requested sync window already recorded in its provider-specific state
- **THEN** it MUST return provider account observations for the configured accounts
- **AND** it MUST generate from 1 to 3 booked transaction observations for each configured account only for the requested window's last UTC day
- **AND** repeated-run transaction provider IDs MUST be unique from transactions generated by earlier runs for the same connection, account, and day
- **AND** the updated provider-specific state MUST preserve prior generated-window history while advancing transaction sequence information

#### Scenario: Synthetic provider remains core-only
- **WHEN** the synthetic provider is implemented in this iteration
- **THEN** the system MUST NOT add synthetic provider controls to the finance UI
- **AND** it MUST NOT expose a public HTTP synthetic-linking workflow or product-facing OpenAPI enum unless a later change explicitly adds that surface

### Requirement: Provider Sync V2 Coordinates Bank Link Persistence
The finance module SHALL coordinate provider sync v2 bank-link workflows through product provider profiles, technical connectors, encrypted connection-secret persistence, and durable bank connection metadata.

#### Scenario: Redirect link start resolves product provider and connector
- **WHEN** a tenant member starts a provider sync v2 redirect/SCA link for product provider `pko`
- **THEN** the system MUST resolve the `pko` provider profile and technical connector `enable-banking` before calling connector start-link behavior
- **AND** the system MUST persist an unconsumed pending link start scoped to tenant, actor, product provider, technical connector, state, callback URL, authorization URL, expiration, and the connector's secret-safe start result
- **AND** the persisted redirect-start observations MUST remain in pending-start storage for later finish/retry use rather than becoming durable connection raw-payload evidence by themselves
- **AND** the system MUST reject unsupported product providers or redirect-link methods before storing secrets or calling a connector

#### Scenario: Redirect link finish creates a durable v2 connection
- **WHEN** a tenant member finishes a provider sync v2 redirect/SCA link with a valid state and code
- **THEN** the system MUST consume the matching unexpired pending start for the same tenant, actor, product provider, and technical connector before calling connector finish-link behavior
- **AND** it MUST pass the persisted connector start result to the same technical connector used during start
- **AND** successful finish MUST encrypt the returned connector secret through the finance connection-secret path and persist a bank connection with product provider, technical connector, provider reference, external ID, display name, state, and secret ID
- **AND** the same consumed state MUST NOT create duplicate bank connections

#### Scenario: PKO re-link preserves existing durable connection identity
- **WHEN** a tenant already has a linked `pko` bank connection and the tenant member successfully completes another `pko` redirect/SCA link
- **THEN** the system MUST update and return the existing tenant `pko` connection instead of creating a second `pko` connection
- **AND** the reused connection MUST keep its existing connection identity while refreshing mutable link metadata from the new successful finish

#### Scenario: Redirect finish failure remains retryable
- **WHEN** connector finish-link behavior or encrypted connection persistence fails after a pending redirect start is consumed
- **THEN** the system MUST restore or preserve the pending start so the tenant member can retry until the pending start expires
- **AND** failure handling MUST NOT persist plaintext connector credentials or secret-bearing raw payload evidence

#### Scenario: Token link creates a durable v2 connection
- **WHEN** a tenant member token-links product provider `monobank`
- **THEN** the system MUST resolve the `monobank` provider profile and technical connector `monobank` before calling connector token-link behavior
- **AND** successful token link MUST encrypt the returned token through the finance connection-secret path and persist a bank connection with product provider, technical connector, provider reference, external ID, display name, state, and secret ID
- **AND** token linking MUST reject unsupported product providers or token-link methods before storing secrets or calling a connector

#### Scenario: Linked connection identity feeds provider sync v2
- **WHEN** provider sync v2 coordinates a linked bank connection
- **THEN** the persisted bank connection MUST contain enough durable metadata to build `ProviderConnectionRef` with connection ID, product provider ID, technical connector ID, provider reference, and external ID
- **AND** provider sync v2 MUST use the persisted technical connector ID instead of deriving connector selection from product provider-specific branches

#### Scenario: Link evidence and pending state remain secret-safe
- **WHEN** provider sync v2 persists pending link-start data, final connection raw payload evidence, logs, or returned API views
- **THEN** Monobank tokens, Enable Banking session secrets, bearer tokens, private keys, and signed request material MUST NOT be persisted or surfaced in plaintext
- **AND** persisted evidence MUST keep enough non-secret provider context to debug link failures and connection identity
- **AND** successful token-link or redirect-finish flows MUST persist durable raw payload evidence only from the final connector result, without copying redirect-start observations out of the pending-start envelope

### Requirement: Finance Bank-Link API Uses Focused V2 Service
The finance module SHALL expose protected bank-linking workflows through a focused public bank-connection service backed by provider sync v2 link coordination rather than through the overloaded root finance service.

#### Scenario: Bank-link API preserves public HTTP contract
- **WHEN** an authenticated tenant member uses the existing finance bank-link HTTP endpoints for Monobank token linking or PKO redirect start/finish
- **THEN** the backend MUST keep the existing `/api/v1/finance/...` route paths, request JSON, response JSON, product provider choices, and callback flow unchanged
- **AND** the API controller MUST call a focused public bank-connection service instead of root `finance.Service` bank-link methods

#### Scenario: Focused bank-connection service coordinates v2 linking
- **WHEN** the focused bank-connection service handles Monobank token linking or PKO redirect start/finish
- **THEN** it MUST enforce tenant membership at the public finance boundary
- **AND** it MUST delegate product-provider resolution, technical connector calls, pending-start persistence, encrypted secret handoff, durable connection writes, and raw evidence persistence to provider sync v2 link coordination
- **AND** it MUST reject unsupported provider or linking-method combinations before storing secrets or calling a connector

#### Scenario: Legacy bank-link path is not retained behind a toggle
- **WHEN** the bank-link API path is migrated to the focused bank-connection service
- **THEN** the system MUST NOT keep a runtime toggle, selector, or fallback path that can route the same API operation back to the old root-service bank-link implementation
- **AND** old root-service bank-link methods and legacy bank-provider wiring MUST be removed from the active API path or deleted when no remaining in-scope caller needs them

#### Scenario: Internal provider coordination stays behind finance package boundary
- **WHEN** the backend application composes finance bank-linking dependencies
- **THEN** `apps/signal-foundry` MUST depend on public package `finance` service construction rather than importing `finance/internal/providers`, `finance/internal/monobank`, or `finance/internal/enablebanking`
- **AND** real v2 connector and provider-profile wiring MUST remain owned inside the finance module

### Requirement: Finance Public Services Are Focused
The finance module SHALL expose public services by focused product responsibility instead of requiring app, controller, job, CLI, or fixture callers to depend on one broad root finance service.

#### Scenario: Finance module exposes focused services
- **WHEN** the finance module is composed through `finance.New`
- **THEN** the returned `Finance` instance MUST expose focused public services for tenant management, catalog management, ledger transactions, reporting, FX synchronization and diagnostics, CSV imports, bank-link workflows, and bank-sync workflows
- **AND** each exposed service MUST contain only the methods needed for its responsibility
- **AND** the existing bank-link service MUST remain separate from bank-sync listing, deletion, scheduling, and execution concerns

#### Scenario: App callers depend on focused services
- **WHEN** backend app registration wires finance HTTP controllers, finance jobs, callback handlers, CLI bootstrap, or fixture bootstrap
- **THEN** each caller MUST receive the focused finance service or consumer-defined interface required for its workflow
- **AND** active app code MUST NOT depend on a broad root `finance.Service` facade for unrelated finance methods

#### Scenario: Service dependencies stay narrow
- **WHEN** a focused finance service is constructed
- **THEN** it MUST accept only the store interfaces, providers, ciphers, clocks, ID generators, enqueuers, schedulers, and loggers required by that service
- **AND** store interfaces MUST be defined by the consuming service boundary
- **AND** new persistence needs MUST use dedicated responsibility-specific stores or narrow interfaces instead of extending the legacy broad persistence store as a god object

#### Scenario: External contracts stay unchanged during service split
- **WHEN** the root finance service is decomposed into focused services
- **THEN** existing finance HTTP routes, JSON field names, job types, database tables, and documented domain behavior MUST remain unchanged
- **AND** any required caller change MUST be limited to Go wiring and dependency interfaces inside the repository

