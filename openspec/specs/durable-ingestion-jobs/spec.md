# durable-ingestion-jobs Specification

## Purpose
TBD - created by archiving change add-durable-historical-ingestion-jobs. Update Purpose after archive.
## Requirements
### Requirement: Durable Historical Ingestion Job State
The backend application SHALL persist app-owned durable job records for historical ingestion visibility and worker coordination.

#### Scenario: Job record preserves visibility metadata
- **WHEN** a historical ingestion job is created
- **THEN** the persisted job MUST include id, job type `historical_raw_candle_backfill`, status, requester user id when available, requested-by source, optional agent session/run identifiers, optional idempotency key, canonical input JSON/hash, nullable result JSON, bounded error summary/details, UTC created/updated/queued/started/completed timestamps, worker id, attempt count, last attempt time, and correlation id

#### Scenario: Job state is queryable after restart
- **WHEN** the backend process restarts after a job was created or completed
- **THEN** list and detail reads MUST return job state from the jobs store rather than from in-memory worker or message-bus state

#### Scenario: Status transitions are explicit
- **WHEN** a job executes successfully or fails
- **THEN** normal execution status MUST move through `queued -> running -> succeeded` or `queued -> running -> failed`, preserving lifecycle timestamps and worker metadata
- **AND** the only additional automatic transitions MUST be startup stale-running recovery from `running -> queued` below the attempt cap or `running -> failed` at/above the attempt cap

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

### Requirement: Protected Jobs HTTP API
The backend application SHALL expose authenticated generic jobs endpoints under `/api/v1/jobs` using camelCase JSON.

#### Scenario: Operator starts historical data backfill
- **WHEN** an authenticated operator calls `POST /api/v1/jobs/historical-data-backfills` with a valid backfill request
- **THEN** the API MUST create a queued `data.historical_raw_candle_backfill` job with requester source `operator` and return the queued job detail immediately

#### Scenario: Operator lists jobs with generic filters
- **WHEN** an authenticated operator calls `GET /api/v1/jobs` with optional status, job type, source, requester, schedule, limit, and cursor filters
- **THEN** the API MUST return a deterministic paginated job list including status, job type, scope summary, requester/source, attempts, created/updated timestamps, started/completed timestamps when present, and compact result or error summary

#### Scenario: Operator reads generic job detail
- **WHEN** an authenticated operator calls `GET /api/v1/jobs/{jobId}` for an existing job
- **THEN** the API MUST return generic input/progress/result payloads, requester/audit metadata, lifecycle timestamps, worker/attempt metadata, schedule/correlation metadata when present, and bounded error fields when failed

#### Scenario: Operator can cancel or retry when supported
- **WHEN** an authenticated operator calls the supported cancel or retry jobs endpoints for a job whose handler allows that action
- **THEN** the API MUST perform the requested action through the generic jobs substrate and return the updated or follow-up observable job state

#### Scenario: Jobs API requires authentication
- **WHEN** a caller without a valid authenticated identity calls any `/api/v1/jobs*` endpoint
- **THEN** the system MUST reject the request as unauthorized

### Requirement: Operator Jobs Workspace UI
The operator UI SHALL provide a minimal protected admin jobs workspace for generic job visibility and inspection.

#### Scenario: Admin jobs routes are protected and navigable
- **WHEN** an authenticated operator uses the application navigation
- **THEN** the UI MUST provide an Admin entry, route `#/admin/jobs` MUST show the generic jobs list, route `#/admin/jobs/:jobId` MUST show generic job detail, and unauthenticated access MUST redirect through the existing protected-route behavior

#### Scenario: Jobs list shows cross-product operational state
- **WHEN** an operator opens the admin jobs workspace
- **THEN** the UI MUST show loading, empty, error, and success states; rows MUST work for both finance and historical-data jobs; and filters MUST include status, job type, source, and created-time controls plus refresh/open-detail actions

#### Scenario: Job detail supports product deep links
- **WHEN** an operator opens a generic job detail route
- **THEN** the UI MUST show generic request, timeline, worker, attempts, progress, result, and safe error sections
- **AND** it MUST provide context-specific deep links back to the originating product surface such as Data or Finance when the job payload carries that scope

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

### Requirement: Manual Cancel And Retry Semantics
The backend application SHALL expose safe cancel and retry behavior only where the registered job handler supports it.

#### Scenario: Unsupported cancel or retry is rejected safely
- **WHEN** an authenticated operator requests cancel or retry for a job type or state that does not support the requested action
- **THEN** the system MUST reject the request with a safe validation/conflict response and MUST NOT mutate the job incorrectly

#### Scenario: Supported retry creates observable follow-up execution
- **WHEN** an authenticated operator retries a supported failed or canceled job
- **THEN** the system MUST create or requeue an observable durable execution through the generic jobs substrate and preserve audit visibility of the original terminal job outcome

### Requirement: Durable Jobs Schema Is Prepared Explicitly
The backend application SHALL include durable jobs storage in the explicit backend database migration command.

#### Scenario: Migration creates durable jobs tables
- **WHEN** a user runs `signal-foundry db-migrate` with valid data-layer database configuration
- **THEN** the command MUST create or update the durable jobs table and durable schedule table used by historical-data and finance job execution
- **AND** it MUST use the configured data-layer database DSN and table prefix conventions

#### Scenario: Jobs commands rely on prepared durable schemas
- **WHEN** a user starts the jobs worker or runs a scheduler enqueue tick after `signal-foundry db-migrate` succeeds
- **THEN** jobs command startup MUST rely on the prepared durable jobs schema
- **AND** it MUST NOT create or update jobs tables implicitly during startup

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

