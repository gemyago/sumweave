# data-layer Specification

## Purpose
Provide canonical ingestion, persistence, query, and replay access for market and reference data used by deterministic runtime slices.
## Requirements
### Requirement: Canonical Data Domain

The system SHALL define canonical domain records for market and reference data that are independent from persistence models and reusable by downstream deterministic slices.

#### Scenario: Domain records do not expose persistence metadata

- **WHEN** analytics or strategy code imports canonical market data records
- **THEN** those records MUST be available from shared domain packages without GORM tags, table names, or database-only fields

#### Scenario: Time values are canonicalized

- **WHEN** the system accepts market data records with timestamps
- **THEN** it MUST normalize stored and returned timestamps to UTC

### Requirement: Instrument Reference Data

The system SHALL store and retrieve canonical instrument reference data, including venue, symbol, asset class, and active status.

#### Scenario: Instrument is upserted by venue and symbol

- **WHEN** ingestion provides reference data for the same venue and symbol more than once
- **THEN** the system MUST update the existing instrument record instead of creating duplicates

#### Scenario: Instrument lookup is deterministic

- **WHEN** a caller looks up an instrument by venue and symbol
- **THEN** the system MUST return at most one canonical instrument record

#### Scenario: Ingestion may upsert an instrument from a normalized market-data record

- **WHEN** ingestion receives a valid normalized candle or trade whose canonical instrument fields identify a venue and symbol not yet stored
- **THEN** the system MUST create or update the canonical instrument record before persisting the dependent market-data record

### Requirement: Validated Market Data Ingestion

The system SHALL validate and normalize incoming candle and trade records before persistence.

#### Scenario: Invalid candle is rejected

- **WHEN** ingestion receives a candle whose end time is not after its start time
- **THEN** the system MUST reject the record with a validation error and MUST NOT persist it

#### Scenario: Invalid price or size is rejected

- **WHEN** ingestion receives a market data record with a negative price or negative size
- **THEN** the system MUST reject the record with a validation error and MUST NOT persist it

#### Scenario: Normalized record is persisted

- **WHEN** ingestion receives a valid candle or trade record for a known or upsertable instrument
- **THEN** the system MUST persist the canonical normalized record with source provenance

### Requirement: Idempotent Persistence

The system SHALL make repeated ingestion of the same source record idempotent.

#### Scenario: Duplicate candle ingest does not create a second row

- **WHEN** ingestion receives the same candle natural key more than once
- **THEN** the system MUST retain one canonical candle record for that natural key

#### Scenario: Duplicate trade ingest does not create a second row

- **WHEN** ingestion receives the same trade source record identifier more than once for the same venue and instrument
- **THEN** the system MUST retain one canonical trade record for that source record identifier

### Requirement: Deterministic Data Queries

The system SHALL expose read services for market data by instrument, timeframe, and time range with deterministic ordering.

#### Scenario: Candle query returns ordered data

- **WHEN** a caller requests candles for an instrument, timeframe, and time range
- **THEN** the system MUST return matching candles ordered by start time ascending

#### Scenario: Trade query returns ordered data

- **WHEN** a caller requests trades for an instrument and time range
- **THEN** the system MUST return matching trades ordered by event time ascending with a stable tie-breaker

#### Scenario: Queries use explicit half-open boundaries

- **WHEN** a caller requests candles or trades for a time range
- **THEN** the system MUST treat the range as `[start, end)`, including records at `start` and excluding records at `end`

### Requirement: Replayable Data Access

The system SHALL provide replay-oriented reads that return a stable sequence for downstream deterministic slices.

#### Scenario: Replay is stable across repeated reads

- **WHEN** a caller requests the same replay range multiple times without intervening writes
- **THEN** the system MUST return records in the same order with the same record identities

#### Scenario: Replay respects range boundaries

- **WHEN** a caller requests a replay range with start and end timestamps
- **THEN** the system MUST use `[start, end)` semantics, including records whose relevant timestamp equals `start` and excluding records whose relevant timestamp equals `end`

### Requirement: Data Quality State

The system SHALL track data quality state and source provenance for persisted records.

#### Scenario: Persisted record includes quality state

- **WHEN** a valid market data record is persisted
- **THEN** the system MUST store a quality state and source metadata with the record

#### Scenario: Quality state is available in reads

- **WHEN** a caller reads persisted market data
- **THEN** the system MUST return the quality state needed to decide whether downstream slices can consume the record

### Requirement: Data Store Migration

The system SHALL provide a database migration path for data-layer persistence using explicit schemas compatible with SQLite and PostgreSQL-oriented deployment.

#### Scenario: Startup migration is enabled

- **WHEN** the backend app starts with data-layer auto-migration enabled
- **THEN** the system MUST create or update the data-layer schema before serving dependent data services

#### Scenario: Startup migration is disabled

- **WHEN** the backend app starts with data-layer auto-migration disabled
- **THEN** the system MUST skip data-layer schema migration and leave existing tables unchanged

### Requirement: Opt-In Live Ingestion Smoke Through SQLite

The system SHALL support a manual opt-in smoke that persists real public venue data through the canonical data-layer ingestion and read path using an ephemeral SQLite store.

#### Scenario: Live venue data persists through the canonical path

- **WHEN** a developer intentionally runs the runtime live smoke against a supported public venue adapter such as Hyperliquid
- **THEN** the smoke MUST ingest canonical instruments, candles, or trades into SQLite through the existing ingestion and persistence services rather than bypassing the data layer

#### Scenario: Live smoke validates canonical readback

- **WHEN** live venue records are persisted by the smoke
- **THEN** the smoke MUST read them back through the canonical read services and verify structural invariants such as canonical venue identity, UTC-normalized timestamps, and deterministic ordering semantics where applicable

#### Scenario: Live smoke avoids brittle market snapshot assertions

- **WHEN** the live smoke validates persisted records from a real venue
- **THEN** it MUST focus on canonical persistence and readback invariants rather than exact record counts or fixed market values that are expected to change across runs

### Requirement: Raw Ingestion Run Lineage

The data layer SHALL persist minimal ingestion run lineage for replayability and auditability without requiring a scheduler, backend API, UI workflow, or venue adapter rewrite.

#### Scenario: Ingestion run is recorded idempotently

- **WHEN** a caller records an ingestion run with a stable run identity, source, venue, status, and UTC timestamps
- **THEN** the system MUST persist one ingestion run for that identity and update mutable status, count, and error fields on repeated writes rather than creating duplicates

#### Scenario: Ingestion run rejects missing required fields

- **WHEN** a caller records an ingestion run without a stable run identity, source, venue, status, or required timestamp
- **THEN** the system MUST reject the record with a validation error and MUST NOT persist it

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

### Requirement: Raw Payload Browser Read Models
The data layer SHALL expose read-only raw payload browser models that are safe for backend/API consumption without exposing GORM persistence models or raw response bodies in list results.

#### Scenario: Raw payload metadata list is filtered and ordered
- **WHEN** a caller lists raw venue payload metadata with venue and optional symbol, asset class, timeframe, half-open time range, ingestion run ID, entity hint, endpoint, request type, limit, and cursor filters
- **THEN** the system MUST return matching metadata rows only, ordered deterministically by received time ascending and raw payload identity ascending
- **AND** the list result MUST NOT include raw response body bytes or a response body preview

#### Scenario: Raw payload metadata list paginates deterministically
- **WHEN** the raw payload metadata list has more matches than the bounded requested limit
- **THEN** the system MUST return at most the requested limit and an opaque cursor that continues from the same stable order

#### Scenario: Raw payload detail returns bounded body preview
- **WHEN** a caller requests a raw payload detail by identity
- **THEN** the system MUST return raw payload metadata, response body size in bytes, a deterministic bounded response body preview, and whether the preview was truncated
- **AND** the system MUST read body bytes through the raw payload blob abstraction rather than from a database body column

#### Scenario: Missing raw payload detail is reported
- **WHEN** a caller requests a raw payload identity that is not persisted
- **THEN** the system MUST return a not-found result without fabricating metadata or body preview content

#### Scenario: Candle-linked raw payload metadata is read by full canonical candle key
- **WHEN** a caller requests raw payload evidence linked to a canonical candle using venue, symbol, asset class, timeframe, half-open candle start/end, provenance source, and provenance identity
- **THEN** the system MUST return only raw payload metadata linked to that exact provenance-bearing canonical candle, ordered deterministically
- **AND** the system MUST NOT require callers to depend solely on UI-facing database row identity
- **AND** the system MUST NOT fall back to matching only by venue, symbol, asset class, timeframe, and candle time range when provenance is omitted

#### Scenario: Candle-linked raw payload lookup rejects missing provenance
- **WHEN** a caller requests raw payload evidence linked to a canonical candle without a non-empty provenance source or without a non-empty provenance identity
- **THEN** the system MUST return a deterministic validation error rather than selecting among potentially ambiguous candle natural keys

