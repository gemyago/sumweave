# finance-management Specification

## Purpose
TBD - created by archiving change add-finance-management-slice. Update Purpose after archive.
## Requirements
### Requirement: Finance Module Boundary And Persistence Ownership
The system SHALL implement finance as a root `finance/` product module that remains architecturally independent from the generic agent runtime.

#### Scenario: App composes finance without reverse runtime imports
- **WHEN** finance functionality is wired into the product
- **THEN** `apps/sumweave/` MUST compose both `runtime/` and `finance/`
- **AND** `finance/` MUST NOT import `runtime/`
- **AND** finance business rules MUST live in `finance/` while auth, process lifecycle, generic jobs runtime, and HTTP route glue remain app-owned

#### Scenario: Finance persistence stays finance-owned and auto-migrated
- **WHEN** finance data is persisted
- **THEN** finance-owned tables MUST use `finance_` prefixes, GORM auto-migrate schema initialization, explicit column names, and UTC-first timestamps
- **AND** finance domain models MUST remain separate from GORM persistence models
- **AND** the storage design MUST stay compatible with SQLite local development and PostgreSQL-oriented production use

### Requirement: Tenant, Account, Category, And Tag Management
The finance module SHALL support tenant-based personal-finance ownership, tenant profile updates, and tenant-local finance catalogs.

#### Scenario: Users create and join finance tenants
- **WHEN** an authenticated user creates a tenant or accepts an invite
- **THEN** the system MUST create or join a finance tenant with a user-friendly name and one display currency selected from the predefined valid tenant currency-code list
- **AND** the stored display currency MUST use the canonical uppercase code
- **AND** all tenant members MUST be equal in the first implementation

#### Scenario: Tenant members update finance tenants
- **WHEN** an authenticated tenant member updates a finance tenant
- **THEN** the system MUST allow updating the tenant name and display currency
- **AND** the replacement display currency MUST be selected from the predefined valid tenant currency-code list
- **AND** the stored display currency MUST use the canonical uppercase code
- **AND** the tenant `UpdatedAt` timestamp MUST advance while membership, invites, accounts, transactions, categories, tags, and archived state remain governed by their existing workflows

#### Scenario: Unsupported tenant display currencies are rejected
- **WHEN** a caller creates or updates a finance tenant with an empty, unknown, or unsupported display-currency code
- **THEN** the system MUST reject the request before persisting the tenant change
- **AND** arbitrary free-text values MUST NOT become tenant display currencies

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

#### Scenario: Tenant display currency updates affect reporting display currency only
- **WHEN** a tenant member changes the tenant display currency
- **THEN** later tenant display-currency reporting MUST use the updated display currency through the existing persisted FX-rate behavior
- **AND** the system MUST NOT mutate existing account, transaction, provider-original, or CSV-import native currency values as part of the tenant update

### Requirement: Ledger-Driven Transaction Semantics
The finance module SHALL treat transactions as the explainable ledger source of truth for balances and reporting.

#### Scenario: Balances are derived from transactions
- **WHEN** a user needs to correct an account balance
- **THEN** the system MUST use visible reconciliation or opening-balance transactions rather than directly mutating a balance field

#### Scenario: Synced transactions preserve provider truth and user edits
- **WHEN** provider-synced transactions are imported and later edited by a user
- **THEN** the system MUST retain provider-original values and current schema-derived provider snapshots separately from user-edited presentation/reporting fields
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
- **AND** provider references that identify local provider setup state MUST NOT be treated as decrypted credentials unless the provider explicitly marks them as secret material

#### Scenario: Supported bank linking methods are explicit
- **WHEN** a tenant member starts a bank-linking workflow
- **THEN** the system MUST expose monobank as a token-based bank connection option
- **AND** the system MUST expose PKO as an external redirect/SCA bank connection option implemented through Enable Banking
- **AND** the system MUST expose synthetic as a local redirect-style configured bank connection option
- **AND** token linking MUST reject any bank provider other than monobank before storing secrets or calling a provider
- **AND** redirect link start/finish MUST reject any bank provider other than PKO or synthetic before storing secrets or calling a provider
- **AND** local or fake-provider PKO redirect/SCA validation MUST NOT require a browser-trusted HTTPS certificate when the callback URL uses localhost or a loopback address

#### Scenario: PKO redirect link is completed from a pending start
- **WHEN** a tenant member returns from Enable Banking SCA for PKO with a valid state and code
- **THEN** the system MUST resolve the server-side pending PKO link start for the same tenant and actor
- **AND** the system MUST create the bank connection through the Enable Banking finish-link path
- **AND** the system MUST consume or expire the pending link start so the same state cannot create duplicate connections

#### Scenario: Synthetic local redirect link is completed from configured pending state
- **WHEN** a tenant member finishes a synthetic link with a valid pending state
- **THEN** the system MUST resolve the server-side pending synthetic link start for the same tenant and actor
- **AND** it MUST require configured synthetic account state with at least one valid account before creating the bank connection
- **AND** it MUST create an active synthetic bank connection whose `ProviderReference` is the synthetic state key
- **AND** it MUST consume or expire the pending link start so the same state cannot create duplicate connections

#### Scenario: Bank sync remains idempotent and job-backed
- **WHEN** a tenant member triggers or schedules a bank sync for a linked monobank, PKO, or synthetic connection
- **THEN** the system MUST execute the sync through durable finance jobs, persist normalized accounts/transactions, balance snapshots, provider-original identifiers, and current schema-derived provider snapshots, and deduplicate by provider/connection/account/provider-transaction identity plus safe fallback fingerprints when needed

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

#### Scenario: Synthetic pending link configuration is tenant and actor scoped
- **WHEN** an authenticated tenant member reads or updates synthetic link-state configuration for a pending state
- **THEN** the API MUST verify the state belongs to the selected tenant, authenticated actor, and provider `synthetic`
- **AND** the API MUST reject expired, consumed, wrong-tenant, wrong-actor, or non-synthetic pending states without exposing another user's setup data
- **AND** configuration responses MUST use camelCase JSON and MUST NOT include decrypted credentials

#### Scenario: Finance API covers required product areas
- **WHEN** the first finance slice is implemented
- **THEN** the API surface MUST cover tenants, tenant members/invites, accounts, bank connections, synthetic link-state configuration, transactions including list/detail/create/update flows, categories, tags, dashboard/reporting, FX diagnostics and sync, CSV import preview/confirm/status, and finance-related job deep-linking
- **AND** all operator-facing JSON fields MUST use camelCase

### Requirement: Finance Schema Is Prepared Explicitly
The backend application SHALL include finance-owned persistence schema initialization in the explicit backend database migration command.

#### Scenario: Migration creates finance-owned tables
- **WHEN** a user runs `sumweave db-migrate` with valid backend database configuration
- **THEN** the command MUST run the finance persistence migration for finance-owned tables before finance API, import, reporting, sync, or finance durable job flows rely on those tables
- **AND** finance-owned tables MUST keep finance persistence ownership, explicit column names, UTC-first timestamps, and compatibility with SQLite local development and PostgreSQL-oriented production use

#### Scenario: Finance startup relies on prepared schema in standard setup
- **WHEN** the documented standard setup has run `sumweave db-migrate`
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

### Requirement: Enable Banking Connector Uses Typed Client
The finance module SHALL implement the provider sync v2 Enable Banking connector through the generated typed Enable Banking client surface rather than connector-owned raw HTTP request and response mapping.

#### Scenario: Redirect link start uses typed auth creation
- **WHEN** the Enable Banking connector starts a PKO redirect/SCA link through the supported official branch
- **THEN** it MUST call the generated typed auth-creation client operation with a typed request model
- **AND** it MUST build the provider sync v2 start-link result from the typed auth response
- **AND** it MUST NOT read raw response maps or call raw transport helpers to derive start-link fields

#### Scenario: Redirect link finish uses typed session creation
- **WHEN** the Enable Banking connector finishes a PKO redirect/SCA link with a provider code
- **THEN** it MUST call the generated typed session-creation client operation with a typed request model
- **AND** it MUST build the provider sync v2 link result from the typed session response
- **AND** it MUST NOT read raw response maps or call raw transport helpers to derive finish-link fields

#### Scenario: Fetch uses typed session, balance, and transaction operations
- **WHEN** the Enable Banking connector fetches a requested provider sync window for a linked PKO connection
- **THEN** it MUST call generated typed client operations for session/account, balance, and paged transaction data
- **AND** it MUST map typed responses into provider account, balance, transaction, provider-original, and fingerprint observations
- **AND** continuation handling MUST use the typed transaction response continuation key rather than connector-local raw response probing

#### Scenario: Connector raw access is forbidden
- **WHEN** the Enable Banking connector implements normal start, finish, or fetch behavior
- **THEN** connector code MUST NOT call generated raw transport helpers
- **AND** connector code MUST NOT read generated response `Raw` maps or connector-local raw maps for provider field extraction
- **AND** any response data not exposed by the generated typed client MUST be treated as unavailable to the connector rather than recovered through raw fallback behavior

#### Scenario: Unknown connectors fail before provider fetch
- **WHEN** provider sync v2 coordinates a bank connection whose `ConnectorID` is empty, unknown, or not configured in the runtime registry
- **THEN** the window sync executor MUST fail before any provider fetch call is attempted
- **AND** the failure MUST identify connector resolution as the cause without exposing secrets or raw provider payload content

### Requirement: Provider Sync V2 Supports Synthetic Provider Storage And Generation
The finance module SHALL support a configured `synthetic` bank provider through provider sync v2, including provider-owned linking configuration and dedicated synthetic-provider storage for generation history.

#### Scenario: Synthetic setup starts as a local redirect link
- **WHEN** a tenant member starts a synthetic bank connection link
- **THEN** the system MUST create a pending link start for product provider `synthetic` and connector `synthetic`
- **AND** the start result MUST include a generated state key and a local UI authorization URL that routes the operator to synthetic setup
- **AND** the generated state key MUST become the provider reference for pending and finished synthetic state

#### Scenario: Synthetic pending state is configured through API
- **WHEN** a tenant member configures pending synthetic setup
- **THEN** the account configuration MUST require at least one generated account with non-empty display name and currency before finish can succeed
- **AND** the account configuration MUST be persisted as provider-owned state keyed by provider reference
- **AND** duplicate configured accounts with the same display name and currency MUST remain distinct through stable synthetic account keys

#### Scenario: Synthetic storage round-trips generation state
- **WHEN** core finance code saves synthetic-provider state for a synthetic provider reference
- **THEN** the system MUST persist configured accounts, generated-window history, repeat counters, and transaction sequence counters in synthetic-owned storage scoped to that provider reference
- **AND** loading synthetic-provider state for the provider reference MUST return the same provider-owned state without requiring provider-specific fields in the common provider sync state journal

#### Scenario: Common provider sync journal remains provider-agnostic
- **WHEN** provider sync v2 appends a sync state row for a synthetic connection
- **THEN** the row MUST describe the concrete attempted window, attempt outcome, job or run identity, error summary, and aggregate stats
- **AND** it MUST NOT require an opaque provider-specific state blob to support synthetic generation

#### Scenario: Synthetic fetch uses dedicated synthetic storage
- **WHEN** provider sync v2 executes a sync window for a synthetic connection
- **THEN** the synthetic provider MUST load configured accounts and generation history from synthetic-owned storage by provider reference
- **AND** after generating observations it MUST persist updated generation history back to synthetic-owned storage by provider reference
- **AND** it MUST NOT persist normalized finance ledger records directly from the connector

#### Scenario: Synthetic first run generates daily account transactions
- **WHEN** the synthetic provider fetches a requested sync window that is not recorded in its provider-specific state
- **THEN** it MUST return provider account observations for the configured accounts
- **AND** it MUST generate from 1 to 2 booked transaction observations for each configured account for each UTC day in the requested window
- **AND** generated transaction observations MUST include provider account IDs, provider transaction IDs, amount, currency, description, effective time, fingerprint, provider-original values, and schema-derived provider snapshots
- **AND** the updated provider-specific state MUST record the generated requested window and transaction sequence information needed by later fetches

#### Scenario: Synthetic repeated run generates only the last day
- **WHEN** the synthetic provider fetches a requested sync window already recorded in its provider-specific state
- **THEN** it MUST return provider account observations for the configured accounts
- **AND** it MUST generate from 1 to 3 booked transaction observations for each configured account only for the requested window's last UTC day
- **AND** repeated-run transaction provider IDs MUST be unique from transactions generated by earlier runs for the same connection, account, and day
- **AND** the updated provider-specific state MUST preserve prior generated-window history while advancing transaction sequence information

### Requirement: Provider Sync V2 Coordinates Bank Link Persistence
The finance module SHALL coordinate provider sync v2 bank-link workflows through product provider profiles, technical connectors, encrypted connection-secret persistence, durable bank connection metadata, and typed provider source snapshots.

#### Scenario: Redirect link start resolves product provider and connector
- **WHEN** a tenant member starts a provider sync v2 redirect/SCA link for product provider `pko`
- **THEN** the system MUST resolve the `pko` provider profile and technical connector `enable-banking` before calling connector start-link behavior
- **AND** the system MUST persist an unconsumed pending link start scoped to tenant, actor, product provider, technical connector, state, callback URL, authorization URL, expiration, and the connector's secret-safe start result
- **AND** the persisted redirect-start observations MUST remain in pending-start storage for later finish/retry use rather than becoming durable connection provider snapshots by themselves
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
- **AND** failure handling MUST NOT persist plaintext connector credentials or secret-bearing provider snapshots

#### Scenario: Token link creates a durable v2 connection
- **WHEN** a tenant member token-links product provider `monobank`
- **THEN** the system MUST resolve the `monobank` provider profile and technical connector `monobank` before calling connector token-link behavior
- **AND** successful token link MUST encrypt the returned token through the finance connection-secret path and persist a bank connection with product provider, technical connector, provider reference, external ID, display name, state, and secret ID
- **AND** token linking MUST reject unsupported product providers or token-link methods before storing secrets or calling a connector

#### Scenario: Linked connection identity feeds provider sync v2
- **WHEN** provider sync v2 coordinates a linked bank connection
- **THEN** the persisted bank connection MUST contain enough durable metadata to build `ProviderConnectionRef` with connection ID, product provider ID, technical connector ID, provider reference, and external ID
- **AND** provider sync v2 MUST use the persisted technical connector ID instead of deriving connector selection from product provider-specific branches

#### Scenario: Link snapshots and pending state remain secret-safe
- **WHEN** provider sync v2 persists pending link-start data, final connection provider snapshots, logs, or returned API views
- **THEN** Monobank tokens, Enable Banking session secrets, bearer tokens, private keys, and signed request material MUST NOT be persisted or surfaced in plaintext
- **AND** persisted snapshots MUST keep enough non-secret typed provider context to debug link failures and connection identity
- **AND** successful token-link or redirect-finish flows MUST persist a durable connection snapshot only from the final connector result, without copying redirect-start observations out of the pending-start envelope

### Requirement: Finance Bank-Link API Uses Focused V2 Service
The finance module SHALL expose protected bank-linking workflows through a focused public bank-connection service backed by provider sync v2 link coordination rather than through the overloaded root finance service.

#### Scenario: Bank-link API preserves public HTTP contract
- **WHEN** an authenticated tenant member uses the existing finance bank-link HTTP endpoints for Monobank token linking or PKO redirect start/finish
- **THEN** the backend MUST keep the existing `/api/v1/finance/...` route paths, request JSON, response JSON, product provider choices, and callback flow unchanged
- **AND** the API controller MUST call a focused public bank-connection service instead of root `finance.Service` bank-link methods

#### Scenario: Focused bank-connection service coordinates v2 linking
- **WHEN** the focused bank-connection service handles Monobank token linking or PKO redirect start/finish
- **THEN** it MUST enforce tenant membership at the public finance boundary
- **AND** it MUST delegate product-provider resolution, technical connector calls, pending-start persistence, encrypted secret handoff, durable connection writes, and provider snapshot persistence to provider sync v2 link coordination
- **AND** it MUST reject unsupported provider or linking-method combinations before storing secrets or calling a connector

#### Scenario: Legacy bank-link path is not retained behind a toggle
- **WHEN** the bank-link API path is migrated to the focused bank-connection service
- **THEN** the system MUST NOT keep a runtime toggle, selector, or fallback path that can route the same API operation back to the old root-service bank-link implementation
- **AND** old root-service bank-link methods and legacy bank-provider wiring MUST be removed from the active API path or deleted when no remaining in-scope caller needs them

#### Scenario: Internal provider coordination stays behind finance package boundary
- **WHEN** the backend application composes finance bank-linking dependencies
- **THEN** `apps/sumweave` MUST depend on public package `finance` service construction rather than importing `finance/internal/providers`, `finance/internal/monobank`, or `finance/internal/enablebanking`
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

### Requirement: Enable Banking Client Matches Official API And App Transport
The finance module SHALL implement Enable Banking through complete documented typed client operations that use the backend app's configured HTTP client instance.

#### Scenario: App wiring supplies provider HTTP client
- **WHEN** the backend app composes the finance module
- **THEN** finance provider connectors MUST receive an HTTP client created by the app HTTP client factory
- **AND** Enable Banking calls MUST use that injected client for transport, timeout, logging, correlation, and telemetry behavior
- **AND** app DI wiring MUST NOT pass `http.DefaultClient` directly for normal finance provider calls

#### Scenario: Client transport uses typed JSON request sending
- **WHEN** an Enable Banking client operation sends an HTTP request
- **THEN** it MUST use a typed JSON request helper with an injected HTTP client, typed request body, typed response target, standard JSON encode/decode behavior, and standard transport/status error handling
- **AND** it MUST attach the Enable Banking JWT `Authorization` header for signed requests
- **AND** normal successful operations MUST NOT decode or retain provider responses through `map[string]any`, raw-message fields, or successful raw response body envelopes

#### Scenario: Client models contain complete documented AIS response shapes
- **WHEN** the client decodes the supported account-information endpoints used by finance sync
- **THEN** its provider DTO graph MUST model every field and nested structure documented for session account resources, account details, account balances, transaction pages, and transaction items in the supported Enable Banking API reference
- **AND** `GET /aspsps` MUST decode the documented object response with an `aspsps` array
- **AND** session fetch MUST support documented account IDs and complete `accounts_data` resources
- **AND** account details MUST use the complete documented account resource
- **AND** balance decoding MUST use the complete documented balance resources under `balances`
- **AND** transaction requests MUST use the `transaction_status` query parameter when filtering by status
- **AND** transaction decoding MUST retain every documented transaction field and continuation handling MUST use `continuation_key`

#### Scenario: Documented provider values survive typed round trip
- **WHEN** a documentation-derived supported provider response is decoded and then encoded from its typed DTO
- **THEN** every documented field present in the response MUST retain an equivalent JSON value
- **AND** the client MUST model optional values so a valid documented zero, false, or empty value is not silently lost when its presence is meaningful
- **AND** JSON whitespace, object-key order, and unknown undocumented fields need not be preserved

#### Scenario: Provider DTOs remain separate from normalized finance values
- **WHEN** connector code maps an Enable Banking response into finance account, balance, transaction, provider-original, or fingerprint observations
- **THEN** it MUST derive normalized finance values without mutating serialized provider DTO fields
- **AND** normalized IDs, uppercase currencies, signed minor amounts, descriptions, and effective timestamps MUST NOT be stored as undocumented provider DTO fields

#### Scenario: Provider snapshots are encoded from schema models
- **WHEN** Enable Banking response data becomes a finance provider snapshot
- **THEN** connector business mapping MUST use typed schema fields only
- **AND** the snapshot document MUST be encoded from the unchanged typed provider DTO
- **AND** generated request and response structs MUST NOT expose generic raw maps, raw-message fields, or successful response-body fields for snapshot persistence

#### Scenario: Providers without separate balance documents do not invent them
- **WHEN** a supported provider reports account identity and balance fields in one typed account item rather than a separate typed balance response
- **THEN** the connector MUST retain that typed item as the `account` snapshot used to explain both mappings
- **AND** it MUST NOT duplicate the same document under `account_balance`
- **AND** `account_balance` MUST remain reserved for a distinct typed provider balance document

#### Scenario: Unsupported or undocumented operations stay out of the generated client surface
- **WHEN** an Enable Banking endpoint or response shape is not documented by the current official API reference and is not required for the supported PKO workflow
- **THEN** the client MUST NOT keep or add that operation as part of the supported generated surface
- **AND** finance code MUST fail through bounded unsupported-path errors rather than silently falling back to raw request construction

### Requirement: Provider Source Snapshots Are Typed And Distinct
The finance module SHALL retain current, sanitized provider source snapshots reconstructed from supported typed provider documents rather than successful raw HTTP response bytes.

#### Scenario: Snapshot document comes from a typed provider DTO
- **WHEN** a supported provider response contributes source data for a linked connection, account, balance, or transaction
- **THEN** the connector MUST encode the supported typed provider DTO into the snapshot document
- **AND** it MUST NOT populate the snapshot from successful raw HTTP response bytes, generic raw maps, or raw-message fields
- **AND** normalization into finance fields MUST NOT mutate the provider DTO used to construct the snapshot

#### Scenario: Snapshot identity distinguishes document kinds
- **WHEN** current provider source snapshots are persisted
- **THEN** each snapshot MUST identify its finance subject, provider object, snapshot kind, connection, and capture time
- **AND** supported kinds MUST distinguish `connection`, `account`, `account_balance`, and `transaction` documents
- **AND** an account snapshot and account-balance snapshot for the same provider account MUST coexist without replacing each other

#### Scenario: Latest snapshot replaces only the same snapshot identity
- **WHEN** a newer snapshot is captured for the same tenant, connection, finance subject, applicable finance account and transaction IDs, provider object, and snapshot kind
- **THEN** it MUST replace the current document for that exact identity
- **AND** snapshots with another provider object, finance subject, or kind MUST remain independently readable
- **AND** the system MUST NOT expose a snapshot history timeline as part of this change

#### Scenario: Transaction snapshot represents one provider transaction
- **WHEN** a provider transaction is imported or refreshed
- **THEN** its finance transaction MUST receive a `transaction` snapshot containing the complete supported typed transaction item
- **AND** transport pagination envelopes and continuation keys MUST NOT replace or stand in for that transaction snapshot

#### Scenario: Snapshot attachment matches its finance subject
- **WHEN** a provider snapshot is accepted for persistence
- **THEN** a `connection` subject MUST omit finance account and transaction IDs
- **AND** an `account` subject MUST identify its finance account and omit a finance transaction ID
- **AND** a `transaction` subject MUST identify both its finance account and finance transaction
- **AND** tenant ID, connection ID, subject, kind, provider object ID, document, and capture time MUST be present

#### Scenario: Connection deletion removes its current snapshots
- **WHEN** a tenant member deletes a bank connection
- **THEN** every provider snapshot owned by that connection MUST be deleted in the connection-owned metadata cleanup transaction

#### Scenario: Snapshots remain sanitized and tenant-authorized
- **WHEN** a snapshot is persisted or returned through a protected finance API
- **THEN** credential-like fields, bearer tokens, signed authorization material, private keys, and decrypted secrets MUST NOT be stored or returned
- **AND** account and transaction snapshot reads MUST enforce tenant membership and finance-object ownership

#### Scenario: Provider snapshot API exposes current source data
- **WHEN** an authenticated tenant member lists or reads provider snapshots for a finance account or transaction
- **THEN** the protected API MUST expose current snapshot metadata including ID, kind, provider object ID, and capture time
- **AND** a detail read MUST return the sanitized schema-derived document as provider source data
- **AND** the API MUST use provider-snapshot terminology and MUST NOT retain `/evidence` compatibility routes

### Requirement: Production Bank Sync Uses Provider Sync Orchestration
The finance module SHALL execute manual and scheduled bank-connection sync jobs through the provider sync orchestrator and requested-window executor as the single production sync path.

#### Scenario: Durable bank sync job enters the orchestrator
- **WHEN** the durable `finance.bank_connection_sync` handler runs for a linked bank connection
- **THEN** the focused bank-sync service MUST coordinate the request through the provider sync orchestrator
- **AND** the production path MUST NOT call a connector through the legacy `BankConnectionProvider.Sync` result-conversion and service-owned apply flow

#### Scenario: Persisted connection identity selects the connector
- **WHEN** orchestration loads a bank connection whose product provider is `pko` and whose persisted technical connector is `enable-banking`
- **THEN** it MUST build the provider connection reference with provider `pko` and connector `enable-banking`
- **AND** the requested-window executor MUST resolve `enable-banking` from that persisted connector identity without re-deriving it from product-provider-specific branches

#### Scenario: Persisted connector is invalid
- **WHEN** a linked bank connection has an empty, unknown, or unconfigured persisted connector ID
- **THEN** orchestration MUST fail before any provider fetch or finance apply is attempted
- **AND** the focused bank-sync service MUST record the failure through the existing connection and job diagnostics without exposing secret material

#### Scenario: Connector receives a bounded secret handoff
- **WHEN** an orchestrated connector requires the connection credential for provider fetch
- **THEN** the focused service MUST pass the persisted encrypted connection-secret record unchanged to orchestration
- **AND** finance composition MUST resolve plaintext only through the connector's configured cipher-backed dependency when plaintext is required
- **AND** plaintext credentials MUST NOT be persisted, logged, included in provider snapshots, or repackaged as a durable plaintext envelope

#### Scenario: Credentialless connector ignores the secret envelope
- **WHEN** a connector such as Enable Banking uses configured application credentials and the durable provider reference rather than a connection credential
- **THEN** it MUST ignore the persisted connection-secret envelope and identity metadata
- **AND** populated secret-record ID or reference fields MUST NOT make an otherwise valid fetch fail

### Requirement: Provider Sync Orchestrator Plans Target And Chunk Windows
The provider sync orchestrator SHALL resolve one target window from explicit job bounds or the latest connection journal state and SHALL execute validated chunk windows oldest first.

#### Scenario: Automatic sync plans from latest journal state
- **WHEN** a bank sync job supplies no explicit window start
- **THEN** target planning MUST derive coverage from the latest provider sync state journal entry using the documented first-sync, succeeded-checkpoint, failed-attempt, and recent-refresh policy
- **AND** `BankConnection.LastSuccessfulSyncAt` MUST remain an operational projection rather than the automatic coverage checkpoint

#### Scenario: Explicit target bounds are preserved
- **WHEN** a manual or scheduled job supplies an explicit window start or end through the existing sync job input
- **THEN** orchestration MUST use each supplied bound unchanged when resolving the target window
- **AND** an omitted end MUST resolve to the orchestration clock
- **AND** an omitted start MUST resolve from journal policy relative to the resolved end

#### Scenario: Target window is chunked oldest first
- **WHEN** the resolved target window is longer than 30 calendar days
- **THEN** the orchestrator MUST split it into contiguous half-open requested windows advancing by at most 30 calendar days without explicitly normalizing their timezone
- **AND** it MUST execute those requested windows from oldest to newest

#### Scenario: Target window is invalid
- **WHEN** resolved target bounds are zero, equal, reversed, or otherwise invalid
- **THEN** orchestration MUST fail before connector fetch and MUST NOT append a successful state

### Requirement: Requested-Window Apply Supports A Connection's First Sync
The provider-owned window sync store SHALL apply the first provider observations for a linked connection without requiring pre-existing provider-account mappings or transactions.

#### Scenario: First observed provider account creates its finance mapping
- **WHEN** a requested-window batch contains a provider account that has no connection provider-account mapping
- **THEN** atomic apply MUST create a linked finance account owned by the durable connection's tenant
- **AND** it MUST create the connection provider-account mapping before applying balances, transactions, and account snapshots for that provider account

#### Scenario: First provider transaction uses connection ownership
- **WHEN** a provider account has no existing finance transactions and its first provider transaction is applied
- **THEN** the transaction MUST receive its tenant identity from the durable bank connection
- **AND** apply MUST NOT require another transaction to infer tenant ownership

#### Scenario: Existing linked account preserves member edits
- **WHEN** a later observation refreshes an existing provider-account mapping
- **THEN** provider metadata and last-success information MUST be refreshed
- **AND** member-edited finance account fields MUST remain preserved under the existing linked-account refresh rules

#### Scenario: Multi-chunk account statistics remain accurate
- **WHEN** the same provider account is observed in more than one requested window
- **THEN** aggregate statistics MUST distinguish observed accounts from newly created finance accounts
- **AND** the existing `ImportedAccounts` job result MUST count created accounts rather than repeated observations

### Requirement: Successful Requested-Window Progress Is Atomic
The finance module SHALL commit the writes for a successful requested window and its successful provider sync state checkpoint in one database transaction.

#### Scenario: Window writes and checkpoint succeed together
- **WHEN** connector fetch, diff planning, and apply planning succeed for a requested window
- **THEN** accounts, balances, transactions, matches, typed provider snapshots, and the successful chunk state MUST commit atomically
- **AND** the state MUST record the attempted window, success time, run and job identity, and aggregate stats

#### Scenario: Successful checkpoint persistence fails
- **WHEN** the success journal row cannot be persisted during requested-window apply
- **THEN** all finance writes for that requested window MUST roll back
- **AND** the orchestrator MUST return a failure rather than report uncheckpointed progress

#### Scenario: Fetch or apply fails
- **WHEN** provider fetch, diff preparation, or transactional apply fails for a requested window
- **THEN** no partial finance writes for that window may commit
- **AND** the orchestrator MUST append a failed attempt state containing the requested window, job identity, and sanitized error summary

#### Scenario: A later chunk fails after earlier chunks succeeded
- **WHEN** an oldest-first orchestration commits one or more chunks and a later chunk fails
- **THEN** the earlier successful chunk states and finance writes MUST remain durable
- **AND** the next automatic target plan MUST derive its checkpoint from the failed window start before applying the existing recent-refresh rule

### Requirement: Orchestrated Sync Preserves Operational Bank-Connection State
The focused bank-sync service SHALL preserve existing connection, schedule, job-result, and deletion behavior around orchestrated provider sync execution.

#### Scenario: Whole orchestration succeeds
- **WHEN** every requested window in a bank sync job succeeds
- **THEN** the service MUST update existing last-started, last-successful, job ID, connection state, and schedule completion projections
- **AND** it MUST return the existing bank sync job result shape using aggregate orchestrator statistics

#### Scenario: Whole orchestration fails
- **WHEN** target planning or any requested window fails
- **THEN** the service MUST preserve successful chunk journal progress while marking the whole job and connection attempt as failed
- **AND** existing sanitized connection and schedule diagnostics MUST remain available to current API and UI consumers

#### Scenario: Connection deletion removes orchestration state
- **WHEN** a tenant member deletes a bank connection
- **THEN** connection-owned provider sync journal records MUST be removed in the existing connection metadata cleanup transaction
- **AND** a later cleanup failure MUST roll back the journal deletion
- **AND** the deletion path MUST NOT require a legacy sync adapter or leave orphaned orchestration state

#### Scenario: External contracts remain stable
- **WHEN** production execution moves to the provider sync orchestrator
- **THEN** existing finance sync HTTP paths, camelCase request and response JSON, durable job type, optional window input, and schedule operations MUST remain unchanged

