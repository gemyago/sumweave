## ADDED Requirements

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
