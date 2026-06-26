## ADDED Requirements

### Requirement: Generic App Durable Jobs Substrate
The backend application SHALL provide one app-owned durable jobs substrate that can execute both historical-data and finance job types without moving job-runtime ownership into product modules.

#### Scenario: Typed product handlers register against generic persisted jobs
- **WHEN** the application wires durable job execution
- **THEN** product work such as `data.historical_raw_candle_backfill`, `finance.bank_connection_sync`, `finance.fx_rates_sync`, `finance.csv_import`, and `finance.account_import` MUST register as typed app-level handlers over generic persisted job rows
- **AND** persisted job rows MUST store generic `inputJson`, `resultJson`, and optional `progressJson` plus job type, status, requester/source, idempotency key, correlation id, worker metadata, attempts/max attempts, timestamps, and sanitized error fields
- **AND** product modules such as `runtime/` and `finance/` MUST NOT import the app jobs runtime package

#### Scenario: API process enqueues and worker mode executes
- **WHEN** a durable job is created from an API request or scheduler tick
- **THEN** the API process MUST persist the queued job and return without executing the durable work inline
- **AND** execution MUST happen through the dedicated jobs worker mode of the app binary

### Requirement: Durable Job Scheduling Registry
The backend application SHALL support database-backed recurring job schedules for finance and future product work.

#### Scenario: Scheduler tick enqueues due durable jobs
- **WHEN** `signal-foundry jobs enqueue-due` runs against stored schedule definitions
- **THEN** it MUST scan the database-backed schedule registry, enqueue due durable jobs exactly once per due window, update last-enqueue metadata, and leave the long-running product work to the jobs worker

#### Scenario: Scheduled work remains visible in job history
- **WHEN** a recurring finance sync or FX schedule fires
- **THEN** the resulting execution MUST create visible durable job records using the same jobs list/detail surfaces as manually triggered work

### Requirement: Manual Cancel And Retry Semantics
The backend application SHALL expose safe cancel and retry behavior only where the registered job handler supports it.

#### Scenario: Unsupported cancel or retry is rejected safely
- **WHEN** an authenticated operator requests cancel or retry for a job type or state that does not support the requested action
- **THEN** the system MUST reject the request with a safe validation/conflict response and MUST NOT mutate the job incorrectly

#### Scenario: Supported retry creates observable follow-up execution
- **WHEN** an authenticated operator retries a supported failed or canceled job
- **THEN** the system MUST create or requeue an observable durable execution through the generic jobs substrate and preserve audit visibility of the original terminal job outcome

## MODIFIED Requirements

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
