## ADDED Requirements

### Requirement: Immutable Paper Governor Policy Artifacts
The system SHALL store GovernorPolicy v0 records as immutable, versioned, paper-only runtime artifacts derived from validated canonical governor policy definitions.

#### Scenario: Governor policy canonical bytes and hash are persisted
- **WHEN** a caller creates a GovernorPolicy artifact from a valid paper-only governor policy definition
- **THEN** the system MUST persist the artifact schema version, artifact kind, paper mode, canonical JSON bytes, lowercase hex SHA-256 hash of those canonical bytes, and creation time
- **AND** the persisted canonical JSON bytes MUST be derived from the canonical payload rather than from caller-supplied raw JSON formatting

#### Scenario: Reordered allowed action kinds canonicalize identically
- **WHEN** two valid GovernorPolicy v0 payloads differ only by the order of `allowedActionKinds`
- **THEN** the system MUST produce identical canonical JSON bytes and identical lowercase hex SHA-256 hashes

#### Scenario: Governor policy duplicate create is idempotent
- **WHEN** a caller creates a GovernorPolicy artifact whose canonical hash already exists
- **THEN** the system MUST return the existing artifact
- **AND** it MUST NOT insert a second policy row or mutate the existing artifact canonical JSON bytes, hash, schema version, artifact kind, paper mode, or creation time

#### Scenario: Governor policy artifact is immutable after creation
- **WHEN** a GovernorPolicy artifact has been created
- **THEN** the GovernorPolicy storage capability MUST NOT expose update or delete behavior for that artifact
- **AND** subsequent create, get, get-active, or duplicate create operations MUST NOT change the artifact canonical JSON bytes, hash, schema version, artifact kind, paper mode, or creation time

### Requirement: Governor Policy Paper Safety Gates
The system SHALL validate GovernorPolicy v0 artifacts as paper-only policies before storage or active selection.

#### Scenario: Valid paper policy maps to governor policy
- **WHEN** a GovernorPolicy v0 payload declares paper mode, a non-empty allowed action set, a supported minimum quality, and a non-negative maximum approved action count
- **THEN** the system MUST accept the payload and map it to the existing `governor.Policy` fields without adding live execution behavior

#### Scenario: Non-paper policy is rejected
- **WHEN** a GovernorPolicy v0 payload declares live mode, omits paper mode, or includes fields for venue order routing, wallet credentials, signing keys, leverage, private endpoints, or live execution adapters
- **THEN** the system MUST reject the payload with a validation error
- **AND** it MUST NOT produce canonical JSON bytes, store an artifact, or make the policy active

#### Scenario: Invalid governor policy fields are rejected
- **WHEN** a GovernorPolicy v0 payload has an empty allowed action set, unsupported action kind, unsupported minimum quality, or negative maximum approved action count
- **THEN** the system MUST reject the payload with a validation error matching the existing governor policy semantics

### Requirement: Governor Policy Retrieval And Active Selection
The system SHALL provide deterministic create, get, and get-active behavior for persisted GovernorPolicy v0 artifacts.

#### Scenario: Get governor policy by hash
- **WHEN** a caller requests a GovernorPolicy artifact by an existing canonical hash
- **THEN** the system MUST return the artifact with the same canonical JSON bytes and hash that were persisted at creation

#### Scenario: Missing governor policy is not found
- **WHEN** a caller requests a GovernorPolicy artifact by a canonical hash that does not exist
- **THEN** the system MUST return a not-found result without creating a placeholder or default policy

#### Scenario: Activated paper policy is returned
- **WHEN** a caller creates a valid GovernorPolicy artifact and requests that it become the active paper policy
- **THEN** the system MUST atomically persist or reuse the immutable artifact and make its canonical hash the active paper policy selector
- **AND** a subsequent get-active request MUST return that exact immutable artifact

#### Scenario: No active paper policy is not found
- **WHEN** no GovernorPolicy artifact has been activated for paper mode
- **THEN** get-active MUST return a not-found result without inventing a default policy

### Requirement: Governor Policy SQLite Persistence
The system SHALL support SQLite-backed GovernorPolicy artifact storage tests using the runtime database patterns.

#### Scenario: SQLite store creates and migrates governor policy schema
- **WHEN** the GovernorPolicy database store is opened against SQLite and migrated
- **THEN** the schema MUST support creating, getting, activating, retrieving the active policy, and idempotently recreating GovernorPolicy artifacts
- **AND** the schema MUST enforce canonical hash uniqueness

#### Scenario: SQLite store preserves governor policy immutability
- **WHEN** SQLite-backed tests create a GovernorPolicy artifact and then repeat duplicate create, get, and get-active operations
- **THEN** the stored artifact canonical JSON bytes, hash, schema version, artifact kind, paper mode, and creation time MUST remain unchanged
