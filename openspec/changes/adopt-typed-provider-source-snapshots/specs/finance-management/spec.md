## ADDED Requirements

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

## MODIFIED Requirements

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
