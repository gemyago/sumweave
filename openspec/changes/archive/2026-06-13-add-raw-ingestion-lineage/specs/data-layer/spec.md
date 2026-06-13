## ADDED Requirements

### Requirement: Raw Ingestion Run Lineage

The data layer SHALL persist minimal ingestion run lineage for replayability and auditability without requiring a scheduler, backend API, UI workflow, or venue adapter rewrite.

#### Scenario: Ingestion run is recorded idempotently

- **WHEN** a caller records an ingestion run with a stable run identity, source, venue, status, and UTC timestamps
- **THEN** the system MUST persist one ingestion run for that identity and update mutable status, count, and error fields on repeated writes rather than creating duplicates

#### Scenario: Ingestion run rejects missing required fields

- **WHEN** a caller records an ingestion run without a stable run identity, source, venue, status, or required timestamp
- **THEN** the system MUST reject the record with a validation error and MUST NOT persist it

### Requirement: Raw Venue Payload Archive

The data layer SHALL persist raw venue payload records linked to ingestion runs using explicit, non-secret metadata and UTC receive timestamps.

#### Scenario: Raw payload is linked to ingestion run

- **WHEN** a caller stores a raw venue payload for a known ingestion run with payload identity, source, venue, content type, body, checksum, and received time
- **THEN** the system MUST persist the raw payload and retain the link back to the ingestion run for audit lookup

#### Scenario: Raw payload rejects missing required fields

- **WHEN** a caller stores a raw venue payload without a stable payload identity, source, venue, ingestion run identity, content type, body, checksum, or received time
- **THEN** the system MUST reject the record with a validation error and MUST NOT persist it

#### Scenario: Raw payload rejects unknown ingestion run

- **WHEN** a caller stores a raw venue payload that references an ingestion run not persisted in the lineage store
- **THEN** the system MUST reject the record with a validation or referential integrity error and MUST NOT persist the raw payload

#### Scenario: Raw payload write is idempotent

- **WHEN** the same raw payload identity is stored more than once
- **THEN** the system MUST retain one raw payload row for that identity and update mutable payload metadata without duplicating the raw body record

#### Scenario: Secret-bearing metadata is not persisted

- **WHEN** a caller provides request or response metadata with credentials, authorization headers, cookies, signatures, or API keys
- **THEN** the system MUST NOT persist those secret-bearing metadata values in the raw payload archive

### Requirement: Normalization Run Lineage

The data layer SHALL persist normalization run records that describe attempts to transform raw venue payloads into canonical data batches.

#### Scenario: Normalization run links raw payload and outcome

- **WHEN** a caller records a normalization run for one or more raw payloads with a stable normalization identity, status, started time, completed time, record kind, counts, and optional error summary
- **THEN** the system MUST persist the normalization run and retain links to the source raw payload lineage needed for audit

#### Scenario: Normalization run write is idempotent

- **WHEN** the same normalization identity is recorded more than once
- **THEN** the system MUST retain one normalization run for that identity and update mutable status, count, and error fields rather than creating duplicates

#### Scenario: Normalization run rejects missing required fields

- **WHEN** a caller records a normalization run without a stable normalization identity, status, started time, record kind, counts, or at least one raw payload identity
- **THEN** the system MUST reject the record with a validation error and MUST NOT persist it

#### Scenario: Normalization run rejects unknown raw payload links

- **WHEN** a caller records a normalization run that references any raw payload identity not persisted in the lineage store
- **THEN** the system MUST reject the record with a validation or referential integrity error and MUST NOT persist the normalization run or partial raw-payload links

#### Scenario: Failed normalization preserves error summary

- **WHEN** normalization fails before producing a data batch
- **THEN** the system MUST persist the failed normalization status and error summary without requiring canonical candle or trade rows to be written

### Requirement: Data Batch Lineage

The data layer SHALL persist data batches that group canonical market records produced by a normalization run and expose stable batch identities for replay/audit reads.

#### Scenario: Data batch is linked to normalization run

- **WHEN** a caller records a data batch with a stable batch identity, normalization run identity, venue, optional instrument, record kind, time range, quality, and record count
- **THEN** the system MUST persist one batch for that identity and retain the link to the normalization run

#### Scenario: Data batch write is idempotent

- **WHEN** the same data batch identity is recorded more than once
- **THEN** the system MUST retain one data batch for that identity and update mutable quality, count, and summary fields rather than creating duplicates

#### Scenario: Data batch rejects missing required fields

- **WHEN** a caller records a data batch without a stable batch identity, normalization run identity, venue, record kind, time range, quality, or record count
- **THEN** the system MUST reject the record with a validation error and MUST NOT persist it

#### Scenario: Data batch rejects unknown normalization run

- **WHEN** a caller records a data batch that references a normalization run not persisted in the lineage store
- **THEN** the system MUST reject the record with a validation or referential integrity error and MUST NOT persist the data batch

#### Scenario: Canonical rows carry optional batch linkage

- **WHEN** lineage-aware ingestion persists valid canonical candles or trades for a data batch
- **THEN** the system MUST link the persisted canonical rows to the batch without adding raw payload fields, GORM tags, or database-only fields to shared canonical market data records

#### Scenario: Existing non-lineage ingestion remains valid

- **WHEN** a caller uses the existing candle or trade ingestion APIs without a data batch
- **THEN** the system MUST continue to persist and query canonical records with the existing deterministic behavior and no required batch linkage

### Requirement: Lineage Audit And Replay Lookup

The data layer SHALL provide batch-scoped read access sufficient to audit the lineage chain from canonical batch records back to normalization runs, raw payloads, and ingestion runs; direct run-level audit lookup is out of scope for this change.

#### Scenario: Batch audit returns full lineage chain

- **WHEN** a caller requests audit lineage for a persisted data batch
- **THEN** the system MUST return the batch, its normalization run, ordered linked raw payload metadata, and parent ingestion run metadata for each raw payload

#### Scenario: Batch audit orders multiple raw payloads by stable keys

- **WHEN** a caller requests audit lineage for a persisted data batch whose normalization run links multiple raw payloads
- **THEN** the system MUST order raw payload audit entries by raw payload received time in UTC ascending and then by stable raw payload identity ascending as the tie-breaker
- **AND** the system MUST NOT rely on database insertion order for the raw payload audit order

#### Scenario: Batch replay returns stable canonical identities

- **WHEN** a caller requests canonical candle or trade rows for a persisted data batch multiple times without intervening writes
- **THEN** the system MUST return the same rows in stable replay order with the same canonical replay identities

### Requirement: Lineage Store Migration

The data layer SHALL migrate lineage persistence using explicit schemas compatible with the existing SQLite-first and PostgreSQL-oriented GORM store.

#### Scenario: AutoMigrate creates lineage schema

- **WHEN** the data-layer database store runs `AutoMigrate`
- **THEN** the system MUST create or update the ingestion run, raw venue payload, normalization run, data batch, and batch linkage schema alongside the existing canonical data tables

#### Scenario: Lineage schema uses explicit columns

- **WHEN** the lineage persistence models are migrated
- **THEN** the system MUST use explicit table names, explicit column names, UTC timestamp fields, and uniqueness constraints for stable lineage identities
