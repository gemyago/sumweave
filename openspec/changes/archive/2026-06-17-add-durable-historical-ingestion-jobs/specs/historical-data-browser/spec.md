## MODIFIED Requirements

### Requirement: Historical Data Browser Read-Only Safety
The historical data browser's browsing and inspection behavior SHALL remain read-only; only a clearly labeled historical backfill action may create a durable ingestion job.

#### Scenario: Browser does not implicitly mutate historical data
- **WHEN** an operator loads availability, selects availability entries, edits filters, loads candles, selects candles, views linked evidence, or browses raw payload metadata/detail
- **THEN** the system MUST NOT start backfills, schedule ingestion, fill gaps, edit, delete, repair, re-normalize, or mutate raw payload, lineage, candle, trading, strategy, analytics, backtest, paper trading, or execution state

#### Scenario: Explicit backfill action creates only a job
- **WHEN** an operator uses the Data page's clearly labeled `Start historical backfill` action with an explicit scope
- **THEN** the UI MUST call the durable jobs API to create a `historical_raw_candle_backfill` job and show a link to that job
- **AND** the Data page MUST NOT execute ingestion directly or pretend candles are available before the job succeeds and data is reloaded

#### Scenario: UI terminology maps normalized copy to canonical rows
- **WHEN** the UI labels persisted candle rows for operators
- **THEN** it MUST use the phrase "normalized candles" while mapping those rows to existing canonical persisted `domain.Candle` data
- **AND** formal normalization-run and data-batch browsing MUST remain a follow-up unless separately scoped

## ADDED Requirements

### Requirement: Historical Data Backfill Entry Point
The Data page SHALL provide an explicit operator entry point for starting historical raw candle backfill jobs from a selected or manually entered candle scope.

#### Scenario: Operator starts backfill from current data scope
- **WHEN** an authenticated operator has selected or entered venue, symbol, asset class, timeframe, start, and end on the Data page and activates `Start historical backfill`
- **THEN** the UI MUST submit those fields plus optional page size/idempotency key to `POST /api/v1/jobs/historical-data-backfills`
- **AND** successful creation MUST show the created job id/status and a route link to the job detail

#### Scenario: Backfill entry validates before submit
- **WHEN** required scope fields are missing, the UTC range is invalid, `start >= end`, or the range exceeds the documented job interval cap known to the client
- **THEN** the UI MUST show inline validation and MUST NOT call the jobs API

#### Scenario: Backfill entry preserves data browsing state
- **WHEN** a backfill job is created from the Data page
- **THEN** existing availability, candle, selected candle, and raw evidence UI state MUST remain honest and MUST NOT be replaced by optimistic synthetic candle data
