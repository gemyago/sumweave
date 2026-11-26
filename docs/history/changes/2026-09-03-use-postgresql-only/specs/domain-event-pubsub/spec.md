---
type: Historical record
title: "2026-09-03-use-postgresql-only: spec"
description: "Historical spec record for 2026-09-03-use-postgresql-only."
status: deprecated
superseded_by: /requirements/domain-event-pubsub.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-03-use-postgresql-only/specs/domain-event-pubsub/spec.md"
    title: "Original openspec/changes/archive/2026-09-03-use-postgresql-only/specs/domain-event-pubsub/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## RENAMED Requirements

- FROM: `### Requirement: Explicit multi-driver pub/sub schema preparation`
- TO: `### Requirement: Explicit PostgreSQL pub/sub schema preparation`

## MODIFIED Requirements

### Requirement: Explicit PostgreSQL pub/sub schema preparation

The explicit database migration command SHALL prepare the topic-aware pub/sub
message and offset schema for the configured PostgreSQL application database.

#### Scenario: Migration prepares pub/sub storage

- **WHEN** `sumweave db-migrate` runs against a supported PostgreSQL application
  database
- **THEN** it MUST prepare durable topic messages and offsets keyed by topic and
  consumer group
- **AND** later publisher and router startup MUST NOT create or alter those
  tables implicitly.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
