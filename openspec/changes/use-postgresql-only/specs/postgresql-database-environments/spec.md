## ADDED Requirements

### Requirement: PostgreSQL Is The Sole Supported Database

All active Sumweave database-backed components SHALL use PostgreSQL and SHALL
NOT contain a SQLite runtime, test, configuration, or dependency fallback.

#### Scenario: PostgreSQL configuration initializes database components

- **WHEN** a Sumweave database-backed component receives valid PostgreSQL
  configuration and its database is available
- **THEN** it MUST construct its SQL and ORM access through the PostgreSQL
  drivers
- **AND** finance, agent runtime, auth, jobs, dispatch, and migration behavior
  MUST use PostgreSQL semantics only

#### Scenario: SQLite configuration is unsupported

- **WHEN** any active application, command, or test attempts to use a SQLite
  memory, file, URL, or filename DSN
- **THEN** initialization MUST fail rather than selecting a SQLite driver or
  compatibility path
- **AND** active Go module dependency graphs MUST NOT include a SQLite driver

### Requirement: Local PostgreSQL Is Provisioned By Docker Compose

The repository SHALL provide a canonical Docker Compose PostgreSQL environment
with one regular database and one dedicated test database as a required part of
local backend and database-backed test setup.

#### Scenario: Developer prepares the local backend environment

- **WHEN** a developer follows the standard local setup workflow
- **THEN** the workflow MUST start the repository-managed PostgreSQL service and
  wait until it is ready
- **AND** checked-in local configuration MUST point application and agent runtime
  storage at the regular PostgreSQL database
- **AND** checked-in test configuration MUST point application and agent runtime
  storage at the separate PostgreSQL test database
- **AND** the workflow MUST direct the developer to run `sumweave db-migrate`
  once for the regular environment and once for the test environment before
  starting PM2, another backend process, or database-backed tests

### Requirement: CI Provisions PostgreSQL Before Backend Tests

The GitHub Actions test workflow SHALL start and prepare PostgreSQL on the
Ubuntu runner before executing database-backed affected tests.

#### Scenario: Affected tests run in GitHub Actions

- **WHEN** the reusable tests workflow starts on an Ubuntu runner
- **THEN** it MUST start `postgresql.service` and confirm readiness with
  `pg_isready`
- **AND** it MUST provision separate regular and test databases
- **AND** it MUST run `sumweave db-migrate` against the regular environment and
  the test environment before affected test targets execute
- **AND** application schemas MUST be prepared by that production migration
  command and its existing AutoMigrate entrypoints rather than a duplicated SQL
  schema dump or replacement migration script

### Requirement: PostgreSQL Tests Use The Dedicated Test Database

Database-backed tests SHALL exercise PostgreSQL only through the dedicated test
database and SHALL create independent randomized domain data for each test.

#### Scenario: Database-backed test creates state

- **WHEN** a database-backed test needs users, tenants, sessions, jobs, provider
  records, or other persisted state
- **THEN** it MUST create fresh randomized identities and scope reads and
  assertions to the state it owns
- **AND** it MUST NOT depend on globally empty tables or use the regular local
  database

#### Scenario: Shared test database exposes a concrete conflict

- **WHEN** concurrent or repeated tests conflict through shared persisted state
- **THEN** the affected tests MUST remove static identities, create fresh
  tenant-scoped data, narrow their queries, clean up their owned rows, or
  serialize only the incompatible case
- **AND** the implementation MUST NOT introduce per-test PostgreSQL databases or
  schemas unless observed behavior shows that the simpler remedies are
  insufficient
