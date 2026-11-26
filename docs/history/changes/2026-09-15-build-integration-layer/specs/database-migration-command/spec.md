---
type: Historical record
title: "2026-09-15-build-integration-layer: spec"
description: "Historical spec record for 2026-09-15-build-integration-layer."
status: deprecated
superseded_by: /requirements/database-migration-command.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-15-build-integration-layer/specs/database-migration-command/spec.md"
    title: "Original openspec/changes/archive/2026-09-15-build-integration-layer/specs/database-migration-command/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## MODIFIED Requirements

### Requirement: Explicit Backend Database Migration Command

The backend application SHALL provide an explicit `sumweave db-migrate` command
that prepares configured Sumweave-managed PostgreSQL schemas without starting
long-running application processes.

#### Scenario: Command migrates all configured app schemas

- **WHEN** a user runs `sumweave db-migrate` with valid PostgreSQL configuration
- **THEN** the command MUST initialize agent runtime, auth users, auth refresh
  tokens, auth personal access tokens, appdispatch, job-projection, and finance
  schemas
- **AND** the access-token migration MUST run after auth users and refresh tokens
- **AND** it MUST complete without starting HTTP, workers, schedulers, provider
  sync, or runtime request execution.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
