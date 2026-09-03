# database-migration-command Specification

## Purpose
Define explicit preparation of the Sumweave application database.
## Requirements
### Requirement: Explicit Backend Database Migration Command

The backend application SHALL provide an explicit `sumweave db-migrate`
command that prepares configured Sumweave-managed PostgreSQL schemas without
starting long-running application processes.

#### Scenario: Command migrates all configured app schemas

- **WHEN** a user runs `sumweave db-migrate` with valid PostgreSQL configuration
- **THEN** the command MUST initialize agent runtime, auth, appdispatch,
  job-projection, and finance schemas
- **AND** it MUST complete without starting HTTP, workers, schedulers, provider
  sync, or runtime request execution

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

