# historical-data-backfill Specification

## Purpose
TBD - created by archiving change add-historical-raw-candle-backfill-cli. Update Purpose after archive.
## Requirements
### Requirement: Manual Historical Raw Candle Backfill Command
The system SHALL provide a manual operator CLI command for historical Hyperliquid raw candle backfill without adding a scheduler, backend route, UI workflow, private venue behavior, or AI call path.

#### Scenario: Valid command starts a Hyperliquid candle backfill
- **WHEN** an operator runs `signal-foundry data backfill-raw-candles` with venue `hyperliquid-perps`, symbol, asset class, timeframe, UTC-compatible start and end timestamps, run ID, and optional page size
- **THEN** the system MUST validate the inputs, build canonical runtime request values, and execute a Hyperliquid perps candle-only backfill for the half-open `[start, end)` range

#### Scenario: Invalid command input fails before venue reads
- **WHEN** the command is missing required flags or receives unsupported venue, invalid symbol, invalid asset class, unsupported timeframe, non-UTC-compatible timestamps, `start >= end`, negative page size, or empty run ID
- **THEN** the system MUST return a non-zero error without calling the venue or writing ingestion lineage

### Requirement: Backfill Ingestion Run Lifecycle
The historical raw candle backfill SHALL record deterministic ingestion-run lifecycle state for the operator-provided run ID.

#### Scenario: Run starts before first venue call
- **WHEN** a validated backfill begins
- **THEN** the system MUST record an ingestion run with the provided run ID, stable Hyperliquid backfill source, venue `hyperliquid-perps`, status `started`, current UTC start time, unset completion time, record count `0`, and empty error summary before the first venue read

#### Scenario: Run succeeds after candle persistence
- **WHEN** all requested venue candle pages are read, canonical candles are persisted, raw-to-candle links are recorded, and gap reporting completes
- **THEN** the system MUST update the same ingestion run to status `succeeded` with current UTC completion time, persisted canonical candle count, and empty error summary

#### Scenario: Run fails after start
- **WHEN** venue reading, canonical persistence, raw evidence capture, lineage linking, or readback fails after the ingestion run was started
- **THEN** the system MUST update the same ingestion run to status `failed` with current UTC completion time, best-known persisted count, and a concise error summary

### Requirement: Raw Evidence Linked To Canonical Candles
The historical raw candle backfill SHALL capture Hyperliquid raw candle payload evidence and link it to canonical candle rows while preserving idempotent canonical persistence.

#### Scenario: Raw payload is captured for each candle response
- **WHEN** the backfill requests Hyperliquid `/info` `candleSnapshot` pages
- **THEN** the system MUST capture raw request/response evidence for each HTTP exchange with the operator run ID as ingestion-run context before response decoding completes

#### Scenario: Canonical candles are persisted and linked
- **WHEN** a captured Hyperliquid candle response normalizes to canonical candles
- **THEN** the system MUST persist those candles through the existing venue ingestion flow and persist raw-payload-to-candle links for each persisted candle produced from that response

#### Scenario: Repeated backfills preserve canonical idempotency
- **WHEN** the same candle natural keys are backfilled more than once
- **THEN** the system MUST retain one canonical candle per natural key while preserving separate raw evidence rows for distinct HTTP exchanges

### Requirement: Historical Candle Completeness Report
The historical raw candle backfill SHALL produce a deterministic completeness report from persisted candle readback for the requested instrument, timeframe, and half-open range.

#### Scenario: Complete report summarizes requested range
- **WHEN** a backfill completes or reaches a reportable persisted state
- **THEN** the system MUST report requested venue, symbol, asset class, timeframe, start and end, expected candle count, persisted candle count, first persisted candle start when present, last persisted candle end when present, missing interval total, duplicate natural-key count when detected, and raw payload count when cheaply available

#### Scenario: Raw payload count is omitted when not cheaply available
- **WHEN** the completeness report cannot obtain a run-scoped raw payload count from existing cheap lineage reads or already-collected response metadata
- **THEN** the system MUST omit the raw payload count field deterministically and MUST NOT add broad audit scans or new audit APIs solely to compute it for this slice

#### Scenario: Missing intervals are reported, not filled
- **WHEN** expected candle interval boundaries have no corresponding persisted candle
- **THEN** the system MUST include those intervals in the gap report and MUST NOT synthesize, interpolate, or persist artificial candles for the missing intervals

#### Scenario: CLI output is deterministic and capped
- **WHEN** the CLI prints the backfill result
- **THEN** the output MUST include run ID, requested range, persisted count, expected count, gap count, first/last persisted boundaries, and raw payload count when present in stable order, MUST omit raw payload count deterministically when absent, and MUST cap printed missing intervals while still reporting the total missing interval count

### Requirement: Historical Trades Remain Unsupported
The historical backfill slice SHALL NOT add historical trade backfill for Hyperliquid v0.

#### Scenario: Trade backfill path is rejected if introduced
- **WHEN** a command or runner input attempts to request historical trades for Hyperliquid
- **THEN** the system MUST return a clear unsupported error explaining that historical trade backfill is unsupported because Hyperliquid `recentTrades` only exposes the latest venue window

