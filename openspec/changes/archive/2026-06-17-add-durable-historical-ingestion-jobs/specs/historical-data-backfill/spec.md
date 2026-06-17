## MODIFIED Requirements

### Requirement: Manual Historical Raw Candle Backfill Command
The system SHALL continue to provide a manual operator CLI command for historical Hyperliquid raw candle backfill, independent from the app-owned durable jobs API and worker orchestration.

#### Scenario: Valid command starts a Hyperliquid candle backfill
- **WHEN** an operator runs `signal-foundry data backfill-raw-candles` with venue `hyperliquid-perps`, symbol, asset class, timeframe, UTC-compatible start and end timestamps, run ID, and optional page size
- **THEN** the system MUST validate the inputs, build canonical runtime request values, and execute a Hyperliquid perps candle-only backfill for the half-open `[start, end)` range

#### Scenario: Invalid command input fails before venue reads
- **WHEN** the command is missing required flags or receives unsupported venue, invalid symbol, invalid asset class, unsupported timeframe, non-UTC-compatible timestamps, `start >= end`, negative page size, or empty run ID
- **THEN** the system MUST return a non-zero error without calling the venue or writing ingestion lineage

## ADDED Requirements

### Requirement: Durable Job Backfill Runner Reuse
The app-owned durable jobs worker SHALL reuse the existing historical raw candle backfill runner rather than introducing a second ingestion implementation.

#### Scenario: Job executor invokes deterministic runner
- **WHEN** a durable historical raw candle backfill job is executed
- **THEN** the worker MUST pass the job-generated ingestion run id and canonical backfill input into `runtime/flows.HistoricalRawCandleBackfillRunner`
- **AND** existing ingestion lifecycle metadata, canonical candle persistence, raw payload capture, raw-to-candle links, and completeness reporting MUST remain authoritative

#### Scenario: Durable jobs do not change CLI behavior
- **WHEN** the durable jobs API and worker are enabled
- **THEN** the existing `signal-foundry data backfill-raw-candles` command MUST remain usable with an operator-provided run id and MUST NOT require a jobs table record
