## MODIFIED Requirements

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
