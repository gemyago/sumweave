## Context

Production Sumweave already uses PostgreSQL, but local application configuration
and most database-backed tests use SQLite. Supporting both dialects currently
requires separate SQL connection selection in the app, runtime, and finance
modules; SQLite PRAGMA/WAL configuration; dialect-specific GORM predicates; and
a custom SQLite appdispatch subscriber and migration implementation alongside
the existing Watermill PostgreSQL path. The SQLite drivers also propagate their
`modernc` dependency tree into downstream workspace modules.

The repository already has an optional PostgreSQL 17 Compose environment that
can run agent runtime, auth, jobs, dispatch, and finance flows. This change
promotes that capability into the standard local environment rather than
creating another database setup. Production configuration remains externally
injected and continues to use its existing PostgreSQL databases and roles.

Database-backed tests currently rely on in-memory or temporary-file SQLite
databases. The replacement model uses one dedicated PostgreSQL test database,
separate from the regular local database. Tests should already create fresh
users, tenants, sessions, jobs, and other identities; any real conflict exposed
by sharing the test database will be corrected in the affected tests rather
than preemptively introducing a per-test database or schema framework.

The requested CI reference is the
[`community-manager` test workflow](https://github.com/gemyago/community-manager/blob/main/.github/workflows/tests-run.yml#L14-L18):
start Ubuntu's bundled `postgresql.service`, wait with `pg_isready`, and invoke
database preparation before tests. Sumweave will adopt that service lifecycle,
provision separate regular and test databases, and use its existing
`sumweave db-migrate` command for schema preparation rather than introducing a
second setup script for application tables.

## Goals / Non-Goals

**Goals:**

- Make PostgreSQL the only database dialect accepted by active Sumweave code.
- Remove SQLite runtime code, test usage, direct and transitive dependencies,
  active configuration, and active documentation in one change.
- Make a repository-managed Docker Compose PostgreSQL service a required,
  reproducible part of local backend and database-backed test setup.
- Provision separate regular and test PostgreSQL databases locally and in CI.
- Run `sumweave db-migrate` once for the regular environment and once for the
  test environment during repository setup.
- Keep PostgreSQL-backed tests simple by using the dedicated test database and
  fresh randomized domain identities, adjusting only tests with demonstrated
  shared-state conflicts.
- Preserve `sumweave db-migrate`, GORM AutoMigrate, and existing schema
  ownership and startup boundaries.

**Non-Goals:**

- Migrate, import, or preserve SQLite data.
- Introduce a dual-dialect transition release or compatibility layer.
- Replace GORM AutoMigrate or appdispatch's explicit migration with a new
  migration framework.
- Merge the application and agent-runtime database configuration boundaries.
- Change production PostgreSQL roles, deployment secrets, public APIs, finance
  behavior, durable-job semantics, or table shapes solely for this cutover.
- Rewrite archived OpenSpec changes that accurately record the historical
  SQLite implementation; active specifications and documentation will be
  updated.

## Decisions

### Use PostgreSQL constructors directly instead of detecting a dialect

All active `database/sql` connections will use the registered pgx driver and
all GORM construction will use the PostgreSQL dialector. Components will stop
classifying DSNs by filename, substring, or URL scheme. PostgreSQL's own DSN
parser remains authoritative for both URL and keyword/value PostgreSQL DSN
forms; a SQLite/file DSN fails as invalid PostgreSQL configuration rather than
selecting a fallback.

SQLite-only connection defaults, dialectors, and timestamp predicates will be
deleted. PostgreSQL query forms become the single implementation in runtime,
finance, auth, and jobs.

Alternative considered: keep dialect selection behind build tags for tests.
This retains a second behavior surface and its dependencies, so it does not
meet the PostgreSQL-only goal and would keep local tests unlike production.

### Retain the existing PostgreSQL appdispatch implementation

`appdispatch` will always construct the existing Watermill PostgreSQL schema,
offsets adapter, publisher, subscriber, and transactional migrator. Driver
enums and selection branches, SQLite migration query builders, and the custom
SQLite lease/subscriber implementation will be removed. Topic, consumer-group,
at-least-once, transaction-bound publication, retry, dead-letter, and lifecycle
contracts remain unchanged.

Alternative considered: replace SQL dispatch with an external broker while
removing SQLite. That expands product and operational scope without addressing
the current requirement; PostgreSQL dispatch already provides the required
behavior in production.

### Promote the existing Compose database into standard local setup

The repository will provide one canonical local PostgreSQL Compose definition,
based on the existing PostgreSQL 17 environment, with a health check,
repo-scoped persistent volume, obvious local-only credentials, one regular
database, and one database reserved for tests. Standard local setup will
explicitly start and wait for this service, run `sumweave db-migrate --env
local` against the regular database, and run `sumweave db-migrate --env test`
against the test database. PM2 continues to start application processes only
after the regular environment is prepared, while module tests use only the
prepared test database.

Local application and agent-runtime DSNs remain separate configuration fields
but point to the same PostgreSQL database for their selected environment, using
their existing table prefixes. Local config selects the regular database and
test config selects the test database. Local roles may own and migrate their
respective databases so the checked-in configs work with `db-migrate`, normal
commands, PM2, and tests. Production may continue overriding the two DSNs with
separate migration/runtime credentials.

Starting Docker implicitly from `direnv` was rejected because merely entering
the repository should not mutate process state. A documented repo command or
setup target will start Compose and report readiness as an explicit local setup
step.

### Prepare both environments through `db-migrate`

Docker Compose will provision the two empty local databases as infrastructure.
The GitHub Actions workflow will likewise start the Ubuntu PostgreSQL service,
wait with `pg_isready`, and ensure the same regular and test databases exist.
Application schemas are then prepared by invoking the existing `sumweave
db-migrate` command separately with regular and test environment configuration.

No repository-owned replacement migration script or checked-in SQL schema dump
will create application tables. The existing command remains the single entry
point over agent runtime migrations, auth, appdispatch, durable jobs, and
finance AutoMigrate behavior.

Alternative considered: use a custom setup script to create both cluster
objects and application schemas. This would duplicate behavior already owned by
`db-migrate` and introduce another migration source of truth. Alternative
considered: run the test suite in a PostgreSQL service container in GitHub
Actions. The requested reference uses the runner's bundled service, so CI will
follow that service lifecycle while retaining Sumweave's Nx commands.

### Share one dedicated test database with independent test data

All database-backed module tests will use the prepared test database, never the
regular local database. Tests will continue using randomized IDs and will create
fresh users, tenants, sessions, jobs, provider records, and other domain state
for each case. Reads and assertions must be scoped to the identity or tenant
created by the test rather than assuming globally empty tables.

This keeps the test setup close to the existing project conventions and avoids
building a generic per-test schema lifecycle before a concrete need exists. If
parallel execution reveals an actual conflict, the smallest affected test
boundary will be corrected—for example by replacing a static identifier,
creating a fresh tenant, narrowing a query, cleaning up owned rows, or
serializing only the incompatible case. Migration-specific tests may rerun the
idempotent migration command or be serialized when necessary, while the normal
test suite relies on the environment migrated during setup.

Alternative considered: create a database or PostgreSQL schema for every test.
That provides strong isolation but adds lifecycle, permission, and cleanup
machinery not justified by observed failures. Alternative considered: replace
composition coverage with SQL mocks. That would stop exercising the production
database used by jobs, migrations, finance, and appdispatch.

### Remove SQLite from active dependency and documentation surfaces

After code and tests use PostgreSQL, each active Go module will be tidied and
the workspace sums synchronized so the SQLite drivers and `modernc` packages no
longer appear in active module dependency graphs. This includes downstream
tooling and integration-test modules that currently inherit SQLite through the
runtime or app modules.

Active architecture, module docs, manual E2E guides, OpenSpec capabilities, and
agent rules will describe PostgreSQL as the only database and Docker Compose as
the normal local prerequisite. Historical archived change artifacts remain
unchanged because they document prior decisions rather than supported behavior.

## Risks / Trade-offs

- [Local development and backend tests now require Docker and PostgreSQL] →
  Provide explicit Compose setup instructions, fixed local-only credentials, a
  health check, and clear failure guidance when the service is unavailable.
- [Real PostgreSQL tests can exceed current short module timeouts] → Reuse one
  running test database, migrate it once during setup, and adjust only the
  affected module timeouts to measured values.
- [Parallel tests can collide in the shared test database] → Require fresh
  randomized identities and tenant-scoped assertions, then fix or narrowly
  serialize only concrete conflicts found by the suite.
- [Repeated local test runs leave rows in the test database] → Make tests
  independent of table emptiness and document how to recreate the test database
  through Compose when a clean slate is wanted.
- [CI's bundled PostgreSQL version may differ from the local PostgreSQL 17
  image] → Keep SQL within supported PostgreSQL behavior and treat CI/local
  version variation as useful compatibility coverage rather than relying on
  SQLite portability.
- [Removing DSN detection can make invalid configuration fail at a different
  layer] → Wrap pgx/GORM errors with existing component context and document
  PostgreSQL DSN examples; do not reintroduce home-grown dialect heuristics.
- [Archived documents still contain the word SQLite] → Treat archives as
  immutable history and verify that active code, configs, specs, docs, and Go
  dependency graphs contain no supported SQLite path.

## Migration Plan

This is one source and deployment cutover; the ordered implementation steps do
not create an intermediate supported release.

1. Add the canonical local Compose environment with separate regular and test
   databases, then wire GitHub Actions to start PostgreSQL and provision the same
   two-database shape.
2. Update setup instructions and CI to run `sumweave db-migrate` once for the
   regular environment and once for the test environment.
3. Convert all SQLite-backed tests and test configuration to the shared test
   database, using fresh randomized domain identities and resolving only
   demonstrated test conflicts.
4. Simplify runtime, finance, auth, jobs, and appdispatch to their PostgreSQL
   implementations and delete all SQLite-specific branches and files.
5. Update local defaults, PM2/setup guidance, active specs, architecture, manual
   E2E docs, and agent rules to make Compose PostgreSQL mandatory.
6. Tidy every affected Go module and synchronize the workspace dependency sums;
   verify active source and dependency graphs contain no SQLite packages.
7. Run the repository completion protocol with PostgreSQL provisioned, then
   deploy the resulting binary normally. Production runs the existing
   `db-migrate` command against its current PostgreSQL database before process
   startup; no data conversion occurs.

Rollback is a normal code rollback against the same production PostgreSQL
database. Because no PostgreSQL schema is replaced solely for this cutover, the
previous binary's PostgreSQL path remains the rollback path. Local developers
may reset the repo-scoped Compose volume if desired; no SQLite data is restored.

## Open Questions

There are no blocking design questions. Exact local command names and test
adaptations may follow the nearest module conventions during implementation
without changing the two-database or migration contracts above.
