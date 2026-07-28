# durable-jobs Specification

## Purpose

Define the app-owned durable jobs substrate used by finance workflows.

## Requirements

### Requirement: Finance durable job records

The backend application SHALL persist durable job records for finance background
work and operational visibility.

#### Scenario: A finance workflow creates a job

- **WHEN** a finance workflow dispatches background work
- **THEN** the stored job MUST include its identifier, job type, status,
  requester/source metadata, timestamps, attempt metadata, worker identifier,
  correlation metadata, and sanitized error fields when applicable
- **AND** the job type MUST be a finance job type.

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

The backend application SHALL dispatch finance background work through its
app-owned pub/sub abstraction and execute it through the dedicated jobs
consumer mode.

#### Scenario: API and scheduler dispatch without inline execution

- **WHEN** a finance API operation or scheduler tick starts background work
- **THEN** it MUST publish the work without executing it inline.

#### Scenario: Finance handlers use the generic substrate

- **WHEN** the application wires durable job execution
- **THEN** finance job handlers MUST register against the generic persisted
  jobs substrate
- **AND** the finance module MUST NOT import the app jobs runtime package.

#### Scenario: Scheduled finance work is visible

- **WHEN** a recurring finance sync or FX schedule fires
- **THEN** the resulting work MUST be observable through the jobs list and
  detail endpoints.

### Requirement: Explicit durable jobs schema preparation

The explicit backend database migration command SHALL prepare the durable jobs
and schedule storage using the configured application database.

#### Scenario: Commands rely on prepared schemas

- **WHEN** the jobs worker or scheduler runs after `sumweave db-migrate`
- **THEN** it MUST use the prepared schema and MUST NOT migrate it implicitly.
