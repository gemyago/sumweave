# durable-jobs Specification

## Purpose

Define the app-owned, opt-in product visibility layer for finance background
work whose execution identity and lifecycle must be exposed through an API or
user interface. Appdispatch remains the publication and execution transport;
jobs are consumer-side projections, not a queue.
## Requirements
### Requirement: Finance durable job records

The backend application SHALL persist durable job records for finance background
work only when a product or operational requirement needs execution visibility.

#### Scenario: A finance workflow publishes observed work

- **WHEN** a finance workflow dispatches background work that requires
  product-visible execution state
- **THEN** the job materialized by the observed consumer MUST include its
  identifier, job type, status,
  requester/source metadata, lifecycle timestamps, attempt metadata, worker
  identifier, and sanitized error fields when applicable
- **AND** the job type MUST be a finance job type.

#### Scenario: Background processing needs no job visibility

- **WHEN** an internal background command or event reaction has no product or
  operational requirement for execution identity, status, or history
- **THEN** it MUST be allowed to use appdispatch without creating a job record.

#### Scenario: Job visibility does not replace transport

- **WHEN** a visible finance job is created
- **THEN** its execution command MUST be published through appdispatch before
  any consumer receives it
- **AND** publication MUST return a stable message ID that is also the future
  observed job ID
- **AND** publication MUST NOT create the job row.

#### Scenario: Job identity is shared with its message

- **WHEN** a job-observed consumer first receives a dispatch message
- **THEN** it MUST materialize the job lazily with `job.id == message.id`
- **AND** it MUST persist and claim the projection before invoking domain work.

#### Scenario: Job detail is temporarily absent before delivery

- **WHEN** a caller requests a known future observed-job ID before its consumer
  receives the message
- **THEN** the jobs API MAY return `404`
- **AND** a client that already received that ID from the dispatching workflow
  MAY treat the response as pending
- **AND** unknown or deep-linked IDs MUST remain ordinary `404` errors.

#### Scenario: Job state survives restart

- **WHEN** the backend process restarts after a job is queued or completed
- **THEN** list and detail reads MUST use the jobs store rather than in-memory
  worker state.

### Requirement: Metadata-only jobs API

The backend application SHALL expose authenticated `GET /api/v1/jobs` and
`GET /api/v1/jobs/{jobId}` endpoints using camelCase JSON.

#### Scenario: Operator lists finance jobs

- **WHEN** an authenticated operator requests the jobs list with supported
  metadata filters
- **THEN** the response MUST return a deterministic page of job metadata,
  including lifecycle, requester, attempt, worker, and safe-error information
- **AND** it MUST NOT expose arbitrary job input, progress, or result payloads.

#### Scenario: Operator reads a job

- **WHEN** an authenticated operator requests an existing job detail
- **THEN** the response MUST return that job's metadata and safe error
  information without exposing arbitrary job payloads.

#### Scenario: Jobs API requires authentication

- **WHEN** a caller without a valid authenticated identity calls either jobs
  endpoint
- **THEN** the system MUST reject the request as unauthorized.

### Requirement: Durable job execution and scheduling

The backend application SHALL dispatch observable finance background work as
semantic commands through appdispatch and execute it through a job-observed
consumer registration in the worker mode. Jobs MUST NOT define a separate
queue, publication path, or mandatory topic/group.

#### Scenario: API and scheduler dispatch without inline execution

- **WHEN** a finance API operation or scheduler tick starts background work
- **AND** that work requires job visibility
- **THEN** it MUST publish the semantic command and return its message ID
  without executing it inline or creating a job row.

#### Scenario: Job observation is selected by the consumer

- **WHEN** an appdispatch consumer is registered for a semantic command
- **THEN** it MAY opt into the jobs lifecycle decorator with safe type,
  requester, and schedule-occurrence metadata
- **AND** changing that registration MUST NOT require a producer or payload
  change.

#### Scenario: One observed consumer owns visibility

- **WHEN** several consumers could react to the same message
- **THEN** at most one registration MUST materialize a job projection for it
- **AND** an independently visible event reaction MUST publish a distinct
  semantic command.

#### Scenario: Job transport remains distinct from domain events

- **WHEN** the application wires durable jobs and domain-event consumers on the
  shared transport
- **THEN** each consumer MUST use its semantic topic and declared consumer group
  without a jobs-owned execution envelope or offset
- **AND** domain-event consumers MUST remain independent from job observation.

#### Scenario: Finance handlers use the generic jobs substrate

- **WHEN** the application wires durable job execution
- **THEN** finance job handlers MUST register against the generic persisted
  jobs substrate
- **AND** the finance module MUST NOT import the app jobs runtime package.

#### Scenario: Worker claims before execution

- **WHEN** a job-observed consumer receives a command for the first time
- **THEN** it MUST idempotently materialize a queued projection whose ID equals
  the message ID
- **AND** it MUST atomically claim the projection before invoking its handler.

#### Scenario: Duplicate delivery does not repeat a terminal job

- **WHEN** a job-observed consumer receives a duplicate for a terminal job
- **THEN** it MUST acknowledge the duplicate without invoking the handler again
- **AND** a running projection MUST remain pending or recoverable rather than
  executing concurrently.

#### Scenario: Business failure remains a job outcome

- **WHEN** a claimed handler returns a typed handled business failure
- **THEN** an observed consumer MUST persist a sanitized failed job and
  acknowledge the command
- **AND** an ordinary consumer MUST log the terminal failure without creating a
  job record.

The finance adapter maps only an explicitly classified finance-owned terminal
failure to this handled-failure path and preserves its sanitized code, summary,
and details. Unclassified finance service errors, including state-write and
infrastructure failures, retain transport retry/dead-letter handling along with
payload decoding, materialization, claim, panic, and terminal-state persistence
failures around the finance handler.

#### Scenario: Transport failure uses dispatch recovery

- **WHEN** delivery, infrastructure, or handler panic failure occurs
- **THEN** appdispatch MUST apply its bounded retry and dead-letter policy
- **AND** adding job observation MUST NOT convert that failure into an explicit
  job retry operation.

#### Scenario: Visibility persistence failure blocks acknowledgement

- **WHEN** job materialization, claim, or terminal-state persistence fails
- **THEN** the consumer MUST return an error without invoking later domain work
  or acknowledging the source message.

#### Scenario: Worker recovery uses one policy

- **WHEN** a worker finds a stale running job after a process interruption
- **THEN** it MUST recover only a durable claim whose `started_at` is older
  than the configured worker stale-running age
- **AND** it MUST condition its transition on that claim's owner and timestamp
  so a newer claim or terminal transition is not overwritten
- **AND** it MUST requeue or terminally fail the job according to one
  worker-level stale-running attempt policy
- **AND** handlers and individual job rows MUST NOT define separate maximums.

#### Scenario: Scheduled finance work is visible

- **WHEN** a recurring finance sync or FX schedule fires
- **THEN** its semantic command MUST be published without creating a job row
- **AND** once the job-observed consumer receives it, the resulting projection
  MUST be observable through the jobs list and detail endpoints.

### Requirement: Explicit durable jobs schema preparation

The explicit backend database migration command SHALL prepare the durable job
projection and finance-owned schedule storage using the configured application
database.

#### Scenario: Commands rely on prepared schemas

- **WHEN** the jobs worker or scheduler runs after `sumweave db-migrate`
- **THEN** it MUST use the prepared schema and MUST NOT migrate it implicitly.
