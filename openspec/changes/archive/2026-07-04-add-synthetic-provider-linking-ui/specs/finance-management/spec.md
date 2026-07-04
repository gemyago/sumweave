## MODIFIED Requirements

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
- **THEN** the system MUST execute the sync through durable finance jobs, persist normalized accounts/transactions, balance snapshots, provider-original identifiers, and raw payloads, and deduplicate by provider/connection/account/provider-transaction identity plus safe fallback fingerprints when needed

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
- **AND** generated transaction observations MUST include provider account IDs, provider transaction IDs, amount, currency, description, effective time, fingerprint, provider-original values, and raw payload evidence
- **AND** the updated provider-specific state MUST record the generated requested window and transaction sequence information needed by later fetches

#### Scenario: Synthetic repeated run generates only the last day
- **WHEN** the synthetic provider fetches a requested sync window already recorded in its provider-specific state
- **THEN** it MUST return provider account observations for the configured accounts
- **AND** it MUST generate from 1 to 3 booked transaction observations for each configured account only for the requested window's last UTC day
- **AND** repeated-run transaction provider IDs MUST be unique from transactions generated by earlier runs for the same connection, account, and day
- **AND** the updated provider-specific state MUST preserve prior generated-window history while advancing transaction sequence information
