## MODIFIED Requirements

### Requirement: Durable job execution and scheduling

The backend application SHALL dispatch finance background work as commands on a
dedicated topic and consumer group of the app-owned durable pub/sub transport,
and execute it through the dedicated jobs consumer mode.

#### Scenario: API and scheduler dispatch without inline execution

- **WHEN** a finance API operation or scheduler tick starts background work
- **THEN** it MUST publish a job command without executing it inline.

#### Scenario: Job transport remains distinct from domain events

- **WHEN** the application wires durable jobs and domain-event consumers on the
  shared transport
- **THEN** job commands MUST use a jobs-owned execution topic and consumer group
- **AND** domain-event consumer groups MUST NOT advance or compete for the jobs
  consumer offset.

#### Scenario: Finance handlers use the generic jobs substrate

- **WHEN** the application wires durable job execution
- **THEN** finance job handlers MUST register against the generic persisted
  jobs substrate
- **AND** the finance module MUST NOT import the app jobs runtime package.

#### Scenario: Worker claims before execution

- **WHEN** the jobs consumer receives a command for a queued job
- **THEN** it MUST atomically claim the job before invoking its registered
  handler.

#### Scenario: Duplicate delivery does not repeat a non-queued job

- **WHEN** the jobs consumer receives a duplicate command for a terminal or
  otherwise non-queued job
- **THEN** it MUST acknowledge the duplicate without invoking the job handler
  again.

#### Scenario: Business failure remains a job outcome

- **WHEN** a claimed job handler returns a business execution error
- **THEN** the jobs consumer MUST persist the job as failed and acknowledge the
  command
- **AND** retrying that job MUST remain an explicit job operation rather than a
  domain-event or transport retry.

#### Scenario: Scheduled finance work is visible

- **WHEN** a recurring finance sync or FX schedule fires
- **THEN** the resulting work MUST be observable through the jobs list and
  detail endpoints.
