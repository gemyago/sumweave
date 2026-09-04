# postgresql-database-environments Specification

## Purpose
TBD - created by archiving change use-postgresql-only. Update Purpose after archive.
## Requirements
### Requirement: PostgreSQL Is The Sole Supported Database

All core product database-backed components SHALL use PostgreSQL. Components in
`finance/`, `runtime/`, and `apps/sumweave/` SHALL NOT contain a SQLite runtime,
configuration, dependency, or test fallback.

#### Scenario: PostgreSQL configuration initializes database components

- **WHEN** a core database-backed component receives valid PostgreSQL
  configuration and the prepared database is available
- **THEN** it MUST construct SQL and ORM access through PostgreSQL drivers
- **AND** finance, runtime, auth, jobs, dispatch, and migration behavior MUST use
  PostgreSQL semantics only

#### Scenario: SQLite configuration is unsupported

- **WHEN** a core application, command, or database-backed test receives a
  SQLite memory, file, URL, or filename DSN
- **THEN** initialization MUST fail instead of selecting a compatibility path
- **AND** core production imports and dependency graphs MUST NOT include a
  SQLite driver

### Requirement: Local And CI PostgreSQL Use The Standard Bootstrap

The repository SHALL provide a canonical Docker Compose PostgreSQL environment
and `make postgres-bootstrap` command as a normal prerequisite for local backend
processes and ordinary backend tests.

#### Scenario: Developer prepares the local environment

- **WHEN** a developer follows the standard local setup
- **THEN** `make postgres-bootstrap` MUST start the repository Compose service
  and wait until it is ready
- **AND** it MUST idempotently prepare `sumweave_local`, `sumweave_test`, and the
  local owner, migrator, and runtime roles
- **AND** it MUST run `sumweave db-migrate` once for the regular environment and
  once for the test environment through the migrator role
- **AND** it MUST grant the runtime role access before a backend process or
  ordinary backend test runs

#### Scenario: Reusable CI prepares PostgreSQL before tests

- **WHEN** the reusable test workflow reaches its ordinary test step
- **THEN** it MUST first run the same `make postgres-bootstrap` setup against the
  repository Compose service
- **AND** it MUST then invoke its existing ordinary Nx test command
- **AND** it MUST NOT delegate database tests to a separate workflow or
  verification target

### Requirement: PostgreSQL Tests Use Normal Package Selection

Database-backed Go test files SHALL participate in normal package selection after
PostgreSQL setup. No core Go source file SHALL mention or use `postgres_test`,
active test instructions SHALL NOT claim that tag remains, and ordinary core
module test targets SHALL NOT pass it.

#### Scenario: Core module runs ordinary tests

- **WHEN** a developer or CI runs `make test` for `runtime`, `finance`, or
  `apps/sumweave`
- **THEN** the target MUST run ordinary `go test` without a PostgreSQL build tag
- **AND** it MUST run all normally selected package tests in one invocation
- **AND** it MUST write `.cover/profile.out` and enforce the existing 90% per-file
  and total thresholds through `.testcoverage.yaml`
- **AND** it MUST NOT create a routine/PostgreSQL profile or coverage-config split

#### Scenario: Root ordinary coverage is aggregated

- **WHEN** a developer runs root `make test`
- **THEN** the root target MUST consume each Go module's ordinary
  `.cover/profile.out` in the pre-change order
- **AND** it MUST retain the existing aggregate profile, HTML report, and
  `go-test-coverage` check
- **AND** no `postgres-verify`, focused `postgres-test-*`, or module
  `test-postgres` target SHALL exist

### Requirement: PostgreSQL Tests Use The Dedicated Test Database

Database-backed tests SHALL use the bootstrap-prepared dedicated test
database and independent randomized state. The standard local and CI execution
environments SHALL provide its runtime-role DSN to Go test processes through
`SUMWEAVE_POSTGRES_TEST_DSN`; module Makefiles SHALL only inherit that input.

#### Scenario: Local shell supplies the test-process DSN

- **WHEN** a developer enters the repository through its standard direnv setup
  and runs runtime, finance, app, root, or Nx tests after bootstrap
- **THEN** root `.envrc` MUST supply `SUMWEAVE_POSTGRES_TEST_DSN`
- **AND** child Make and Go test processes MUST inherit it without a module
  Makefile fallback or command-level DSN assignment

#### Scenario: Reusable CI supplies the test-process DSN

- **WHEN** the reusable workflow runs its existing ordinary Nx test step after
  bootstrap
- **THEN** that step's environment MUST supply
  `SUMWEAVE_POSTGRES_TEST_DSN`
- **AND** it MUST NOT depend on bootstrap exporting to the parent workflow or on
  a manually configured repository variable

#### Scenario: Database-backed test creates state

- **WHEN** a test needs users, tenants, sessions, jobs, provider records, or other
  persisted state
- **THEN** it MUST use the runtime-role test DSN and fixed application,
  agent-runtime, and finance prefixes
- **AND** it MUST create fresh randomized identities and scope assertions to the
  state it owns
- **AND** it MUST NOT use the regular local database or assume globally empty
  tables

#### Scenario: Shared test state conflicts

- **WHEN** repeated or concurrent tests demonstrate a shared-state conflict
- **THEN** the smallest affected fixture MUST randomize, scope, clean up, or
  serialize that concrete case
- **AND** the implementation MUST NOT introduce a generic per-test database or
  schema framework

#### Scenario: Migration behavior needs test coverage

- **WHEN** migration execution must be included in the ordinary Go coverage
  profile
- **THEN** one shallow migration smoke MAY run through the prepared test
  database
- **AND** it MUST replace detailed schema contracts rather than instrumenting
  bootstrap, adding raw covdata transport, or creating another test lane

