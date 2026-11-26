---
type: Requirements
title: "Database migration requirements"
description: "Explicit preparation of all configured PostgreSQL schemas."
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: accepted-contract
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/specs/database-migration-command/spec.md"
    title: "Accepted database-migration-command scenarios at the documentation checkpoint"
  - id: current-topic
    resource: "/architecture/persistence.md"
    title: "Current topic reference and implementation limits"
---

This document preserves the accepted requirements and granular scenarios for
this capability.[^accepted-contract] See the [current topic reference](../architecture/persistence.md)
for delivered behavior and known limitations. A requirement states a contract;
it does not record a completed test run.[^current-topic]

# database-migration-command Specification

## Purpose
Define explicit preparation of the Sumweave application database.
## Requirements
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

### Requirement: Standard Environment Setup Uses Explicit Migration

The repository SHALL provision PostgreSQL and run explicit migrations for the
regular and test environments before local backend processes or ordinary
backend tests.

#### Scenario: Local setup migrates regular and test databases

- **WHEN** a developer runs `make postgres-bootstrap`
- **THEN** it MUST start and wait for the repository Compose service
- **AND** it MUST run `sumweave db-migrate` from `apps/sumweave` once for the
  regular environment and once for the test environment through the migrator
  role
- **AND** regular backend processes MUST use the regular database while ordinary
  tests use the test database

#### Scenario: CI setup uses the same migration contract

- **WHEN** the reusable test workflow prepares to run ordinary tests
- **THEN** it MUST invoke `make postgres-bootstrap` before the existing test step
- **AND** it MUST NOT use a standalone verification workflow, external bootstrap
  mode, or coverage-instrumented migration command

#### Scenario: Split process modes use the same prepared schemas

- **WHEN** a developer runs `sumweave start`, `sumweave jobs worker`, or
  `sumweave jobs enqueue-due` after bootstrap
- **THEN** each mode MUST use the same prepared dispatch, job-projection, and
  finance schemas
- **AND** scheduler publication MUST NOT create an observed job row before
  worker delivery

### Requirement: Startup Does Not Run Schema Migrations
Backend process startup SHALL rely on schemas prepared by the explicit migration command instead of creating or updating schemas implicitly.

#### Scenario: Startup commands do not migrate schemas
- **WHEN** the documented standard environment setup has been followed
- **THEN** `sumweave start`, `sumweave start-all`, jobs consumer commands, and scheduler commands MUST rely on schemas prepared by `sumweave db-migrate`
- **AND** those startup paths MUST NOT run app-owned database or app-owned pub/sub transport migration steps implicitly
- **AND** migration failures MUST be observable from the migration command rather than during server, consumer, or scheduler startup

[^accepted-contract]: Preserved accepted scenario source; current delivery is described separately in the linked topic.

[^current-topic]: Current topic reference and implementation limits.
