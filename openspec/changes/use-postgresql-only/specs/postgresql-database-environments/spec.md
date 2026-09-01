## ADDED Requirements

### Requirement: PostgreSQL Is The Sole Supported Database

All core product database-backed components SHALL use PostgreSQL. Components in
`finance/`, `runtime/`, and `apps/sumweave/` SHALL NOT contain a SQLite runtime,
test, configuration, or dependency fallback.

#### Scenario: PostgreSQL configuration initializes database components

- **WHEN** a Sumweave database-backed component receives valid PostgreSQL
  configuration and its database is available
- **THEN** it MUST construct its SQL and ORM access through the PostgreSQL
  drivers
- **AND** finance, agent runtime, auth, jobs, dispatch, and migration behavior
  MUST use PostgreSQL semantics only

#### Scenario: SQLite configuration is unsupported

- **WHEN** any core product application, command, or database-backed test
  attempts to use a SQLite memory, file, URL, or filename DSN
- **THEN** initialization MUST fail rather than selecting a SQLite driver or
  compatibility path
- **AND** the `finance`, `runtime`, and `sumweave` Go module dependency graphs
  MUST NOT include a SQLite driver

### Requirement: Local PostgreSQL Is Provisioned By Docker Compose

The repository SHALL provide a canonical Docker Compose PostgreSQL environment
with one regular database and one dedicated test database as a required part of
local backend and database-backed test setup.

#### Scenario: Developer prepares the local backend environment

- **WHEN** a developer follows the standard local setup workflow
- **THEN** `make postgres-bootstrap` MUST start the repository-managed
  PostgreSQL service and wait until it is ready
- **AND** readiness and all cluster-level setup MUST use the local Compose
  default for privileged `POSTGRES_BOOTSTRAP_DSN`
- **AND** it MUST idempotently ensure `sumweave_local` and `sumweave_test` plus
  the local owner, migrator, and runtime roles
- **AND** checked-in local configuration MUST point application and agent runtime
  storage at `sumweave_local` through runtime-role DSNs
- **AND** checked-in test configuration MUST point application and agent runtime
  storage at `sumweave_test` through runtime-role DSNs
- **AND** the target MUST run `sumweave db-migrate` through migrator-role DSNs
  once for the regular environment and once for the test environment
- **AND** it MUST grant the runtime role access to the resulting tables and
  sequences before PM2, another backend process, or a tagged database test runs

### Requirement: Routine Tests Are Database-Independent

Routine tests SHALL run without PostgreSQL or another database service being
available. This includes module `test` targets, Nx `test`,
`make affected-lint-test`, and the reusable pull-request test workflow.

#### Scenario: Routine affected tests run without PostgreSQL

- **WHEN** the reusable tests workflow starts on an Ubuntu runner
- **THEN** it MUST run only database-free tests through the routine affected
  `test` targets
- **AND** it MUST NOT start PostgreSQL, invoke a `postgres-*` target, probe a
  database port, or require a database DSN
- **AND** database-backed test files MUST be excluded from this lane at build
  time rather than skipped according to runtime database availability

### Requirement: PostgreSQL Verification Is Explicit And Non-Routine

The repository SHALL provide an explicit serial PostgreSQL verification target
and a separate manually dispatched GitHub Actions workflow for database-backed
tests.

#### Scenario: Developer runs PostgreSQL verification

- **WHEN** a developer runs `make postgres-verify`
- **THEN** the target MUST depend on `make postgres-bootstrap`
- **AND** it MUST run the `runtime`, `finance`, and `sumweave` database-backed
  targets serially with the `postgres_test` build tag
- **AND** focused root targets for each core module MUST use the same bootstrap
  prerequisite when invoked independently

#### Scenario: Both test lanes enforce coverage

- **WHEN** a core module runs its routine `make test` target
- **THEN** it MUST write `.cover/routine.out` without a PostgreSQL build tag and
  enforce 90% per-file and total coverage through
  `.testcoverage-routine.yaml`
- **AND** when its `make test-postgres` target runs, it MUST pass
  `-tags=postgres_test`, write `.cover/postgres.out`, and enforce 90% per-file
  and total coverage through the module's full `.testcoverage.yaml`
- **AND** the tagged profile MUST include untagged plus tagged tests and every
  non-excluded production file
- **AND** routine-only omissions MUST be exact anchored source-file paths paired
  with tagged ownership and full-profile coverage; package, directory, wildcard,
  lowered-threshold, and broad coverage-ignore exceptions MUST NOT be added

#### Scenario: Root routine coverage aggregation remains compatible

- **WHEN** a developer runs the root `make test` target
- **THEN** it MUST consume `.cover/routine.out` from `runtime`, `finance`, and
  `apps/sumweave`
- **AND** it MUST continue to consume the unchanged `.cover/profile.out` from
  `tools/firecrawl`, `tools/skills`, and `tools/workspacefs`
- **AND** it MUST consume those profiles in the existing order: firecrawl,
  skills, workspacefs, runtime, finance, then sumweave
- **AND** it MUST write the single root aggregate profile to
  `.cover/profile.out`, preserving one profile header while appending the
  remaining profile bodies without duplicate headers
- **AND** it MUST generate `.cover/coverage.html` with `go tool cover` and run
  the existing root aggregate `go-test-coverage --profile .cover/profile.out`
  check
- **AND** target-contract assertions MUST verify these exact input paths, the
  aggregate profile/report/check paths, and the core/template profile
  ownership split
- **AND** the complete root routine target MUST succeed when PostgreSQL is
  unavailable without starting, probing, or requiring a PostgreSQL service

#### Scenario: Manual PostgreSQL workflow runs on GitHub Actions

- **WHEN** the manually dispatched PostgreSQL verification workflow starts
- **THEN** it MUST start `postgresql.service` and confirm readiness with
  `pg_isready`
- **AND** it MUST use the Ubuntu `postgres` OS account to make the fresh cluster
  administrator reachable through an ephemeral privileged
  `POSTGRES_BOOTSTRAP_DSN`
- **AND** it MUST invoke `make postgres-verify` with
  `POSTGRES_MANAGED_EXTERNALLY=1`, `POSTGRES_HOST=127.0.0.1`,
  `POSTGRES_PORT=5432`, and that administrator DSN so Compose startup is skipped
- **AND** it MUST otherwise use the same databases, roles, migrations, prefixes,
  grants, and serial tagged-test order as local verification
- **AND** it MUST NOT be called by the reusable routine test workflow or made an
  implicit prerequisite of a routine test target

#### Scenario: Fresh externally managed service is bootstrapped

- **WHEN** `make postgres-bootstrap` runs with
  `POSTGRES_MANAGED_EXTERNALLY=1` against a fresh PostgreSQL service
- **THEN** `POSTGRES_BOOTSTRAP_DSN` MUST be required and MUST identify a login
  able to create and alter roles, create databases, establish ownership, grant
  database/schema/object privileges, and alter migrator default privileges
- **AND** readiness, all three role creations, both database creations,
  ownership, grants, and default privileges MUST run through that connection
- **AND** the target MUST fail before migration when the privileged DSN is
  absent, unreachable, or insufficiently privileged
- **AND** after complete bootstrap, both explicit migrations and all three
  serial module database targets MUST succeed without Docker Compose

### Requirement: PostgreSQL Tests Use The Dedicated Test Database

Tagged database-backed tests SHALL exercise PostgreSQL only through the prepared
dedicated test database and SHALL create independent randomized domain data for
each test.

#### Scenario: Database-backed test creates state

- **WHEN** a database-backed test needs users, tenants, sessions, jobs, provider
  records, or other persisted state
- **THEN** it MUST create fresh randomized identities and scope reads and
  assertions to the state it owns
- **AND** it MUST NOT depend on globally empty tables or use the regular local
  database
- **AND** it MUST use the runtime role and fixed application, agent-runtime, and
  finance table prefixes prepared by bootstrap
- **AND** it MUST NOT run AutoMigrate or otherwise modify schema

#### Scenario: Shared test database exposes a concrete conflict

- **WHEN** concurrent or repeated tests conflict through shared persisted state
- **THEN** the affected tests MUST remove static identities, create fresh
  tenant-scoped data, narrow their queries, clean up their owned rows, or
  serialize only the incompatible case
- **AND** the implementation MUST NOT introduce per-test PostgreSQL databases or
  schemas unless observed behavior shows that the simpler remedies are
  insufficient

#### Scenario: Test schema is prepared

- **WHEN** the PostgreSQL bootstrap prepares `sumweave_test`
- **THEN** its single serialized `sumweave db-migrate --env test` invocation
  MUST be the shallow migration smoke check for the test lane
- **AND** application table definitions MUST NOT be duplicated in cluster
  bootstrap SQL or ordinary test fixtures
