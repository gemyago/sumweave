## ADDED Requirements

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
