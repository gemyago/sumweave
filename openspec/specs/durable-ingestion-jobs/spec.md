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
The backend application SHALL create durable historical raw candle backfill jobs through an explicit product service call that returns immediately.

#### Scenario: Valid request creates queued job
- **WHEN** an authenticated operator or agent requests a historical backfill for venue `hyperliquid-perps`, asset class `future`, valid symbol, supported timeframe, UTC-normalized half-open `[start, end)` range, and non-negative page size
- **THEN** the service MUST validate and canonicalize input, enforce configured interval and page-size bounds, generate a job id and ingestion run id, persist a queued `historical_raw_candle_backfill` job, wake the worker when enabled, and return the queued job detail immediately without waiting for ingestion to complete

#### Scenario: Invalid request fails before job creation
- **WHEN** a create request has unsupported venue, unsupported asset class, invalid symbol, unsupported timeframe, non-UTC-compatible timestamps, `start >= end`, future end, range exceeding the configured interval cap, negative page size, or over-limit page size
- **THEN** the service MUST return a validation error without creating a job, calling the venue, or writing ingestion lineage

#### Scenario: Idempotency key reuses matching existing job
- **WHEN** the same requester/source submits the same job type with the same non-empty idempotency key and same canonical input hash more than once
- **THEN** the service MUST return the existing job rather than creating a duplicate
- **AND** create requests without idempotency key MUST create a new job even when instrument and range match an earlier job

#### Scenario: Idempotency key mismatch is a conflict
- **WHEN** the same requester/source submits the same job type with a previously used non-empty idempotency key but a different canonical input hash
- **THEN** the service MUST reject the create request with a safe conflict outcome, MUST NOT create a new job, and MUST NOT requeue or mutate the existing job
- **AND** the API mapping MUST use HTTP 409 with a stable `idempotency_key_conflict` code

### Requirement: Historical Ingestion Worker Execution
The backend application SHALL execute queued historical raw candle backfill jobs through a durable worker while keeping runtime flows deterministic.

#### Scenario: Worker succeeds through existing backfill runner
- **WHEN** the worker claims a queued historical raw candle backfill job
- **THEN** it MUST mark the job running idempotently, increment attempt count on claim, set worker id and last attempt time, execute the existing `runtime/flows.HistoricalRawCandleBackfillRunner` with the job-generated ingestion run id, persist a succeeded terminal result containing ingestion run id and completeness report counts, and only then acknowledge or stop processing the wake message/poll item

#### Scenario: Worker persists bounded failure
- **WHEN** validation that escaped creation, venue reads, canonical persistence, raw evidence capture, lineage linking, or report generation fails during execution
- **THEN** the worker MUST persist a failed terminal job with bounded safe error summary/details and MUST NOT expose SQL, GORM, stack traces, secrets, or raw response bodies in operator/agent-facing fields

#### Scenario: Duplicate delivery does not rerun terminal jobs
- **WHEN** a duplicate message, restart poll, or repeated worker loop observes a job already in `succeeded` or `failed`
- **THEN** the worker MUST skip execution and MUST NOT call the venue or backfill runner again for that terminal job

#### Scenario: Startup requeues stale running jobs below attempt cap
- **WHEN** worker startup observes a persisted `running` job left by a previous worker process with `attempt_count` below the configured maximum attempts
- **THEN** the startup recovery step MUST treat the job as stale before queued polling, set status back to `queued`, set `updated_at` and `queued_at` to the recovery time, clear `worker_id` and `started_at`, keep `completed_at` null, preserve `attempt_count` and `last_attempt_time`, and record a bounded safe recovery note/code such as `stale_running_requeued`
- **AND** the next worker claim MUST increment `attempt_count` before executing the backfill runner again

#### Scenario: Startup fails stale running jobs at attempt cap
- **WHEN** worker startup observes a persisted `running` job left by a previous worker process with `attempt_count` greater than or equal to the configured maximum attempts
- **THEN** the startup recovery step MUST mark the job `failed`, set `updated_at` and `completed_at`, preserve last attempt metadata, record bounded safe error code/details such as `stale_running_attempts_exhausted`, and MUST NOT call the venue or backfill runner for that job

#### Scenario: Active in-process jobs are not reclaimed by age alone
- **WHEN** the current worker process is actively executing a running job
- **THEN** the worker MUST NOT requeue or fail that job solely because `started_at`, `updated_at`, or `last_attempt_time` exceeds a stale timeout

#### Scenario: Worker concurrency is conservative
- **WHEN** default configuration is used
- **THEN** the system MUST run at most one historical raw candle backfill worker execution at a time

### Requirement: Protected Jobs HTTP API
The backend application SHALL expose authenticated app-owned jobs endpoints under `/api/v1/jobs` using camelCase JSON.

#### Scenario: Operator starts historical data backfill
- **WHEN** an authenticated operator calls `POST /api/v1/jobs/historical-data-backfills` with a valid backfill request
- **THEN** the API MUST create a queued historical raw candle backfill job with requested-by source `operator` and return job id, job type, status, requester source, created/updated timestamps, and canonical input

#### Scenario: Operator lists jobs with filters
- **WHEN** an authenticated operator calls `GET /api/v1/jobs` with optional status, job type, requested-by source, limit, and cursor filters
- **THEN** the API MUST return a deterministic paginated job list including status, job type, scope summary, requester/source, created/updated timestamps, started/completed timestamps when present, and compact result or error summary

#### Scenario: Operator reads job detail
- **WHEN** an authenticated operator calls `GET /api/v1/jobs/{jobId}` for an existing job
- **THEN** the API MUST return input, status, requester/audit metadata, lifecycle timestamps, worker/attempt metadata, result report including missing interval preview/raw payload count when present, and bounded error fields when failed

#### Scenario: Jobs API requires authentication
- **WHEN** a caller without a valid authenticated identity calls any `/api/v1/jobs*` endpoint
- **THEN** the system MUST reject the request as unauthorized

### Requirement: Operator Jobs Workspace UI
The operator UI SHALL provide a minimal protected Jobs workspace for job visibility and inspection.

#### Scenario: Jobs route is protected and navigable
- **WHEN** an authenticated operator uses the main navigation
- **THEN** the nav MUST include `Jobs`, route `#/jobs` MUST show the jobs list, route `#/jobs/:jobId` MUST show job detail, and unauthenticated access MUST redirect through the existing protected-route behavior

#### Scenario: Jobs list shows compact operational state
- **WHEN** an operator opens the Jobs workspace
- **THEN** the UI MUST show loading, empty, error, and success states; list rows MUST include status, job type, venue/symbol/timeframe/range summary, requested by/source, created/updated timestamps, started/completed timestamps when present, and result or error summary
- **AND** the UI MUST provide status, job type, and source filters plus refresh and open-detail actions

#### Scenario: Job detail shows execution audit information
- **WHEN** an operator opens a job detail route
- **THEN** the UI MUST show full input, requester/audit metadata, status timeline, worker/attempt metadata, result report, missing interval preview, raw payload count, bounded error fields if failed, and a link back to the Data page with the same scope filters

