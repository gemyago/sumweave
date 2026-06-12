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
