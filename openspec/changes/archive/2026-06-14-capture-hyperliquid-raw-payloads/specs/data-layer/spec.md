## ADDED Requirements

### Requirement: Raw Payload Normalized Record Links
The data layer SHALL persist audit links from raw payload evidence records to the canonical normalized records produced from those payloads without adding raw payload fields to shared canonical domain records.

#### Scenario: Raw payload links to normalized instruments
- **WHEN** a raw `meta` payload is normalized and persisted as one or more canonical instruments
- **THEN** the system MUST persist links from the raw payload identity to each normalized instrument identity
- **AND** returned canonical instrument records MUST NOT include raw payload body fields or persistence-only metadata

#### Scenario: Raw payload links to normalized candles
- **WHEN** a raw `candleSnapshot` payload is normalized and persisted as canonical candles
- **THEN** the system MUST persist links from the raw payload identity to each normalized candle replay identity or batch-linked canonical row identity
- **AND** returned canonical candle records MUST remain canonical market-data records without raw payload body fields

#### Scenario: Raw payload links to normalized trades
- **WHEN** a raw deterministic `recentTrades` payload is normalized and persisted as canonical trades
- **THEN** the system MUST persist links from the raw payload identity to each normalized trade replay identity or batch-linked canonical row identity
- **AND** returned canonical trade records MUST remain canonical market-data records without raw payload body fields

#### Scenario: Raw payload link rejects unknown payload
- **WHEN** a caller records a raw-to-normalized link for a raw payload identity that is not persisted
- **THEN** the system MUST reject the link with a validation or referential integrity error and MUST NOT persist a partial link

## MODIFIED Requirements

### Requirement: Raw Venue Payload Archive
The data layer SHALL persist raw venue payload evidence records using explicit, non-secret request/response metadata, UTC timestamps, response body hashes, and local blob references while storing ingestion run identity only when capture occurs inside a run.

#### Scenario: Raw payload captures public request and response evidence
- **WHEN** a caller stores raw public venue payload evidence with payload identity, source, venue, request type, request payload hash, compact request metadata, request timestamp, response timestamp, HTTP status, response body hash, response body reference, entity hint, and received time
- **THEN** the system MUST persist the raw payload evidence in UTC with enough metadata to identify the venue request without storing secret-bearing values

#### Scenario: Raw payload stores response body by reference
- **WHEN** a caller stores raw public venue response body bytes through the v0 local blob abstraction
- **THEN** the system MUST persist the bytes in the blob store and MUST store only `payload_body_ref` plus response body hash metadata in the database raw payload row
- **AND** the database raw payload row MUST NOT store the raw response body bytes

#### Scenario: Raw payload blob store is injected into lineage service
- **WHEN** app/runtime wiring enables raw payload capture
- **THEN** the system MUST construct the v0 local raw payload blob store from the configured data-layer blob path
- **AND** the lineage service MUST receive the blob store dependency for writing response bodies before SQL metadata persistence
- **AND** the database store constructor options MUST remain scoped to database concerns such as DSN and table prefix

#### Scenario: Raw payload blob path defaults under data dir
- **WHEN** the app/runtime configuration does not provide `dataLayer.rawPayloadBlobStore.path`
- **THEN** the system MUST choose a deterministic local raw-payload blob directory under the configured application `dataDir`
- **AND** relative configured blob paths MUST be resolved by the app/runtime wiring rather than by venue adapters

#### Scenario: Raw payload records applicable market-data scope
- **WHEN** a caller stores raw payload evidence for an instrument-scoped or range-scoped request
- **THEN** the system MUST persist applicable entity hint, instrument, timeframe, and half-open time range metadata
- **AND** fields that do not apply to the request type MUST be left empty rather than populated with misleading placeholder values

#### Scenario: Standalone raw payload capture is allowed outside ingestion run
- **WHEN** a caller stores raw payload evidence from a direct public venue read without an ingestion run identity
- **THEN** the system MUST persist the raw payload evidence without requiring a parent ingestion run

#### Scenario: Raw payload capture inside ingestion run validates parent run
- **WHEN** a caller stores raw payload evidence with a non-empty ingestion run identity
- **THEN** the system MUST retain the link back to the ingestion run for audit lookup
- **AND** the system MUST reject the raw payload if the referenced ingestion run is not persisted

#### Scenario: Repeated fetch appends raw payload evidence
- **WHEN** the same public venue request is fetched more than once and each HTTP exchange receives a distinct payload identity
- **THEN** the system MUST persist a separate raw payload evidence row for each fetch even when request payload hash, venue, entity hint, and normalized record natural keys match

#### Scenario: Retry with same raw payload identity is idempotent
- **WHEN** the same raw payload identity is stored more than once after a crash or retry
- **THEN** the system MUST retain one raw payload row for that identity while preserving the already-written blob reference and updating only mutable non-body metadata

#### Scenario: Secret-bearing metadata is not persisted
- **WHEN** a caller provides request or response metadata with credentials, authorization headers, cookies, signatures, API keys, or similarly secret-bearing values
- **THEN** the system MUST NOT persist those secret-bearing metadata values in the raw payload archive

### Requirement: Lineage Store Migration
The data layer SHALL migrate lineage persistence using explicit schemas compatible with the existing SQLite-first and PostgreSQL-oriented GORM store, including raw payload body references and raw-to-normalized link storage.

#### Scenario: AutoMigrate creates lineage schema
- **WHEN** the data-layer database store runs `AutoMigrate`
- **THEN** the system MUST create or update the ingestion run, raw venue payload evidence, normalization run, data batch, batch linkage, and raw-to-normalized link schema alongside the existing canonical data tables

#### Scenario: Lineage schema uses explicit columns
- **WHEN** the lineage persistence models are migrated
- **THEN** the system MUST use explicit table names, explicit column names, UTC timestamp fields, uniqueness constraints for stable lineage identities, and nullable ingestion-run references for standalone raw payload captures

#### Scenario: Raw payload schema stores body references only
- **WHEN** the raw payload evidence persistence model is migrated
- **THEN** the system MUST include explicit columns for request payload hash, request metadata, request timestamp, response timestamp, HTTP status, response body hash, `payload_body_ref`, entity hint, optional instrument/timeframe/time range, and optional ingestion run identity
- **AND** the schema MUST NOT include a database column that stores raw response body bytes
