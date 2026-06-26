## ADDED Requirements

### Requirement: App-Owned Execution Dispatch
The backend application SHALL dispatch background execution through an app-owned pub/sub abstraction while treating jobs as observation metadata by default.

#### Scenario: On-demand work dispatches immediately
- **WHEN** an authenticated operator, agent, or app-owned product flow triggers background work that should begin execution immediately
- **THEN** the backend MUST publish an execution-dispatch message through the app-owned pub/sub abstraction
- **AND** it MAY create or update jobs observation metadata for visibility when that workflow exposes job history
- **AND** it MUST return or continue without executing the background work inline in the request path

#### Scenario: Product modules stay free of Watermill dependency
- **WHEN** product modules such as `runtime/` or `finance/` trigger or observe durable job execution
- **THEN** they MUST interact through app-owned job or service abstractions rather than importing Watermill packages or Watermill-shaped message types directly

### Requirement: Jobs Observation Metadata
The backend application SHALL use jobs metadata primarily for observing background execution rather than as a mandatory dispatch, idempotency, or locking mechanism.

#### Scenario: Observation does not force idempotency
- **WHEN** a background workflow records jobs metadata for operator visibility
- **THEN** that metadata MUST NOT by default be treated as the universal idempotency key, execution lock, retry gate, or required claim record for the workflow
- **AND** workflows that need idempotency, cancellation, locking, or retry coordination MUST opt into those semantics explicitly

## MODIFIED Requirements

### Requirement: Historical Raw Candle Backfill Job Creation
The backend application SHALL dispatch historical raw candle backfill execution through an explicit product service call that returns immediately.

#### Scenario: Valid request dispatches historical backfill work
- **WHEN** an authenticated operator or agent requests a historical backfill for venue `hyperliquid-perps`, asset class `future`, valid symbol, supported timeframe, UTC-normalized half-open `[start, end)` range, and non-negative page size
- **THEN** the service MUST validate and canonicalize input, enforce configured interval and page-size bounds, publish an execution-dispatch message for the backfill work, create or update jobs observation metadata for operator/agent visibility, and return accepted execution metadata immediately without waiting for ingestion to complete

#### Scenario: Invalid request fails before dispatch
- **WHEN** a create request has unsupported venue, unsupported asset class, invalid symbol, unsupported timeframe, non-UTC-compatible timestamps, `start >= end`, future end, range exceeding the configured interval cap, negative page size, or over-limit page size
- **THEN** the service MUST return a validation error without publishing an execution-dispatch message, calling the venue, or writing ingestion lineage

#### Scenario: Historical backfill idempotency is explicit when requested
- **WHEN** a historical backfill request supplies a non-empty idempotency key
- **THEN** the historical backfill workflow MAY use that key plus canonical input metadata to avoid duplicate observable executions or report a safe conflict for mismatched input
- **AND** requests without an idempotency key MUST NOT be deduplicated solely because they resemble an earlier execution

### Requirement: Historical Ingestion Worker Execution
The backend application SHALL execute historical raw candle backfill messages through the app-owned Watermill consumer while keeping runtime flows deterministic.

#### Scenario: Consumer succeeds through existing backfill runner
- **WHEN** the jobs consumer receives a historical raw candle backfill execution message
- **THEN** it MUST execute the existing `runtime/flows.HistoricalRawCandleBackfillRunner` with the dispatch payload's canonical input, update jobs observation metadata when present, and persist a succeeded observable result containing ingestion run id and completeness report counts

#### Scenario: Consumer persists bounded failure metadata
- **WHEN** validation that escaped creation, venue reads, canonical persistence, raw evidence capture, lineage linking, or report generation fails during execution
- **THEN** the consumer MUST publish or persist bounded safe failure metadata for observable jobs and MUST NOT expose SQL, GORM, stack traces, secrets, or raw response bodies in operator/agent-facing fields

#### Scenario: Duplicate or repeated delivery uses workflow-specific guards only
- **WHEN** duplicate Watermill delivery or repeated dispatch occurs for historical backfill work
- **THEN** the workflow MUST only suppress duplicate execution when an explicit historical-backfill idempotency, cancellation, locking, or terminal-observation guard applies
- **AND** the generic jobs observation layer MUST NOT be treated as a universal duplicate-delivery gate for every workflow

### Requirement: Generic App Durable Jobs Substrate
The backend application SHALL provide one app-owned durable jobs substrate that can execute both historical-data and finance job types without moving job-runtime ownership into product modules.

#### Scenario: Typed product handlers register against generic persisted jobs
- **WHEN** the application wires durable job execution
- **THEN** product work such as `data.historical_raw_candle_backfill`, `finance.bank_connection_sync`, `finance.fx_rates_sync`, `finance.csv_import`, and `finance.account_import` MUST register as typed app-level handlers over generic persisted job rows
- **AND** persisted job rows MUST store generic `inputJson`, `resultJson`, and optional `progressJson` plus job type, status, requester/source, idempotency key, correlation id, worker metadata, attempts/max attempts, timestamps, and sanitized error fields
- **AND** product modules such as `runtime/` and `finance/` MUST NOT import the app jobs runtime package

#### Scenario: API process dispatches and dedicated consumer mode executes
- **WHEN** background work is triggered from an API request or scheduler tick
- **THEN** the API or scheduler path MUST publish an execution-dispatch message without executing the work inline
- **AND** execution MUST happen through the dedicated jobs consumer mode of the app binary
- **AND** jobs metadata MAY be updated during execution for observation, cancellation, locking, or idempotency only when the workflow requires those capabilities

### Requirement: Durable Job Scheduling Registry
The backend application SHALL support database-backed recurring job schedules for finance and future product work.

#### Scenario: Scheduler tick publishes due scheduled work onto the dispatch path
- **WHEN** `signal-foundry jobs enqueue-due` runs against stored schedule definitions
- **THEN** it MUST scan the database-backed schedule registry, update due-window metadata for due schedules, and publish scheduled-work execution messages through the app-owned pub/sub abstraction
- **AND** it MUST NOT scan, execute, retry, or recover immediately initiated jobs or non-scheduled work

#### Scenario: Scheduled work remains visible in job history
- **WHEN** a recurring finance sync or FX schedule fires
- **THEN** the resulting execution SHOULD create or update visible jobs observation metadata using the same jobs list/detail surfaces as comparable manually triggered work when that workflow is operator-visible
