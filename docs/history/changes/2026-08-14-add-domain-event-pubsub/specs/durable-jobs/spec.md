---
type: Historical record
title: "2026-08-14-add-domain-event-pubsub: spec"
description: "Historical spec record for 2026-08-14-add-domain-event-pubsub."
status: deprecated
superseded_by: /requirements/durable-jobs.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-08-14-add-domain-event-pubsub/specs/durable-jobs/spec.md"
    title: "Original openspec/changes/archive/2026-08-14-add-domain-event-pubsub/specs/durable-jobs/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

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

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
