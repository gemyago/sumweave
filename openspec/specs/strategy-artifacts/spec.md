# strategy-artifacts Specification

## Purpose
TBD - created by archiving change add-strategy-governor-v0-artifacts. Update Purpose after archive.
## Requirements
### Requirement: Immutable Versioned Strategy Artifacts
The system SHALL store StrategyArtifact v0 records as immutable, versioned runtime artifacts derived from validated canonical strategy definitions.

#### Scenario: Strategy artifact canonical bytes and hash are persisted
- **WHEN** a caller creates a StrategyArtifact from a valid canonical Strategy DSL v0 definition
- **THEN** the system MUST persist the artifact schema version, artifact kind, canonical JSON bytes, lowercase hex SHA-256 hash of those canonical bytes, and creation time
- **AND** the persisted canonical JSON bytes MUST be derived from the canonical payload rather than from caller-supplied raw JSON formatting

#### Scenario: Strategy artifact duplicate create is idempotent
- **WHEN** a caller creates a StrategyArtifact whose canonical hash already exists
- **THEN** the system MUST return the existing artifact
- **AND** it MUST NOT insert a second artifact row or mutate the existing artifact canonical JSON bytes, hash, schema version, artifact kind, or creation time

#### Scenario: Strategy artifact is immutable after creation
- **WHEN** a StrategyArtifact has been created
- **THEN** the StrategyArtifact storage capability MUST NOT expose update or delete behavior for that artifact
- **AND** subsequent create, get, or list operations MUST NOT change the artifact canonical JSON bytes, hash, schema version, artifact kind, or creation time

### Requirement: Strategy Artifact Retrieval
The system SHALL provide deterministic create, get, and list behavior for persisted StrategyArtifact v0 records.

#### Scenario: Get strategy artifact by hash
- **WHEN** a caller requests a StrategyArtifact by an existing canonical hash
- **THEN** the system MUST return the artifact with the same canonical JSON bytes and hash that were persisted at creation

#### Scenario: Missing strategy artifact is not found
- **WHEN** a caller requests a StrategyArtifact by a canonical hash that does not exist
- **THEN** the system MUST return a not-found result without creating a placeholder or default artifact

#### Scenario: List strategy artifacts in stable order
- **WHEN** a caller lists StrategyArtifact records
- **THEN** the system MUST return persisted artifacts in a stable deterministic order by creation time and canonical hash
- **AND** the list result MUST include each artifact's schema version, artifact kind, canonical hash, canonical JSON bytes, and creation time

### Requirement: Strategy Artifact SQLite Persistence
The system SHALL support SQLite-backed StrategyArtifact storage tests using the runtime database patterns.

#### Scenario: SQLite store creates and migrates strategy artifact schema
- **WHEN** the StrategyArtifact database store is opened against SQLite and migrated
- **THEN** the schema MUST support creating, getting, listing, and idempotently recreating StrategyArtifact records
- **AND** the schema MUST enforce canonical hash uniqueness

#### Scenario: SQLite store preserves strategy artifact immutability
- **WHEN** SQLite-backed tests create a StrategyArtifact and then repeat duplicate create, get, and list operations
- **THEN** the stored artifact canonical JSON bytes, hash, schema version, artifact kind, and creation time MUST remain unchanged

