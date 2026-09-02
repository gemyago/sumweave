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
databases and run inside each module's routine `make test` target. The
replacement model uses one dedicated PostgreSQL test database, separate from
the regular local database, but repository rules prohibit routine CI tests from
requiring PostgreSQL. The test contract therefore separates database-free
routine tests from explicitly tagged PostgreSQL persistence and composition
tests. Tagged tests create fresh users, tenants, sessions, jobs, and other
identities; any real conflict exposed by sharing the test database is corrected
in the affected test rather than by introducing a per-test database or schema
framework.

The requested CI reference is the
[`community-manager` test workflow](https://github.com/gemyago/community-manager/blob/main/.github/workflows/tests-run.yml#L14-L18):
start Ubuntu's bundled `postgresql.service`, wait with `pg_isready`, and invoke
database preparation before tests. Sumweave will adopt that service lifecycle
in a separate manually dispatched PostgreSQL verification workflow, not in the
reusable routine test workflow. Both local and manual-CI verification provision
separate regular and test databases through an explicit privileged bootstrap
connection and use the existing `sumweave db-migrate` command rather than a
second application-schema source.

## Goals / Non-Goals

**Goals:**

- Make PostgreSQL the only database dialect accepted by the core Go product
  modules.
- Remove SQLite runtime code, test usage, direct and transitive dependencies,
  active configuration, and active documentation in one change.
- Make a repository-managed Docker Compose PostgreSQL service a required,
  reproducible part of local backend and database-backed test setup.
- Provision separate regular and test PostgreSQL databases locally and in the
  non-routine PostgreSQL verification workflow.
- Use one portable privileged-administrator input for complete bootstrap of
  both the Compose service and a fresh externally managed PostgreSQL service.
- Run `sumweave db-migrate` once for the regular environment and once for the
  test environment during PostgreSQL bootstrap.
- Keep routine module tests, Nx tests, `make affected-lint-test`, and reusable
  pull-request CI database-independent and runnable without PostgreSQL.
- Provide an explicit serial PostgreSQL verification target and a manually
  dispatched workflow for the database-backed test surface.
- Keep PostgreSQL-backed tests simple by using the dedicated test database and
  fresh randomized domain identities, adjusting only tests with demonstrated
  shared-state conflicts.
- Preserve `sumweave db-migrate`, GORM AutoMigrate, and existing schema
  ownership and startup boundaries.
- Preserve the existing 90% per-file and total coverage gates across the
  database-free routine and tagged PostgreSQL lanes.

**Non-Goals:**

- Migrate, import, or preserve SQLite data.
- Introduce a dual-dialect transition release or compatibility layer.
- Replace GORM AutoMigrate or appdispatch's explicit migration with a new
  migration framework.
- Merge the application and agent-runtime database configuration boundaries.
- Change production PostgreSQL roles, deployment secrets, public APIs, finance
  behavior, durable-job semantics, or table shapes solely for this cutover.
- Normalize dates or timestamps to UTC or otherwise change timestamp semantics.
- Change template tool or integration-harness behavior merely because workspace
  synchronization updates an indirect dependency.
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

SQLite-only connection defaults, dialectors, and dialect branches will be
deleted. PostgreSQL query forms become the single implementation in runtime,
finance, auth, and jobs. Existing date and timestamp values retain their domain
semantics; the cutover does not add explicit UTC normalization.

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
database, and one database reserved for tests. The exact database contract is:

- databases: `sumweave_local` and `sumweave_test`;
- database-owner role: `sumweave_owner`;
- DDL role used only by migration setup: `sumweave_migrator`;
- DML/query role used by backend processes and ordinary tagged tests:
  `sumweave_runtime`;
- application table prefix: `sumweave_`;
- agent-runtime table prefix: `sumweave_runtime_`;
- finance-owned tables retain their `finance_` prefix.

The checked-in local-only passwords remain `sumweave_owner_local`,
`sumweave_migrator_local`, and `sumweave_runtime_local`. A fourth credential is
bootstrap-only: `POSTGRES_BOOTSTRAP_DSN` is a PostgreSQL connection to the
`postgres` maintenance database whose login can create and alter roles, create
databases, change database ownership, grant role/database/schema privileges,
and alter default privileges for `sumweave_migrator`. It is never an
application or test DSN.

Compose initializes its standard `postgres` administrator with the obvious
local-only password `sumweave_postgres_local`. When
`POSTGRES_MANAGED_EXTERNALLY` is unset, the root target defaults
`POSTGRES_BOOTSTRAP_DSN` to
`postgres://postgres:sumweave_postgres_local@127.0.0.1:55432/postgres?sslmode=disable`.
When `POSTGRES_MANAGED_EXTERNALLY=1`, `POSTGRES_BOOTSTRAP_DSN` has no default and
MUST be supplied; the target fails before mutation if it is absent or cannot
connect with the required privileges. Host/port decomposition remains available
only for deriving owner, migrator, and runtime DSNs: `POSTGRES_HOST` and
`POSTGRES_PORT` default to `127.0.0.1` and the Compose host port `55432`, while
the manually dispatched workflow sets port `5432`.

Readiness and every cluster-level operation run against
`POSTGRES_BOOTSTRAP_DSN`, not a later owner, migrator, or runtime login. The
bootstrap administrator idempotently creates or updates `sumweave_owner`,
`sumweave_migrator`, and `sumweave_runtime`; creates both databases owned by
`sumweave_owner`; grants migrator create/schema privileges and runtime
connect/usage; and applies existing-object plus
`ALTER DEFAULT PRIVILEGES FOR ROLE sumweave_migrator` grants. Application table
definitions remain outside this privileged cluster setup.

The resulting runtime test DSN is exported once per module command as
`SUMWEAVE_POSTGRES_TEST_DSN`; the app target also exports the same value through
the standard `APP_APPLICATION_DATABASE_DSN` and
`APP_AGENTRUNTIME_DATABASE_DSN` mappings. These are suite-level target inputs,
not per-test environment setup. Production credentials remain externally
injected.

The canonical root command is `make postgres-bootstrap`. It starts the Compose
service unless external mode is selected, waits and performs privileged cluster
setup through `POSTGRES_BOOTSTRAP_DSN`, and launches the following two commands
from `apps/sumweave` with migrator-role DSN overrides:

1. `go run ./cmd/sumweave db-migrate --env local`
2. `go run ./cmd/sumweave db-migrate --env test`

After each migration the bootstrap administrator grants the runtime role DML
access to existing tables and sequence access while default privileges cover
objects created later by the migrator. Cluster bootstrap SQL may create
databases, roles, and grants; it MUST NOT duplicate application table
definitions.

Checked-in `local.yaml` uses runtime-role DSNs for `sumweave_local`, and
`test.yaml` uses runtime-role DSNs for `sumweave_test`. Application and
agent-runtime DSNs remain separate config fields but select the same database in
each environment and retain their distinct table prefixes. The bootstrap target
temporarily overrides both DSNs with the matching migrator DSN because runtime
credentials do not own DDL. PM2 starts only after `make postgres-bootstrap`.

Starting Docker implicitly from `direnv` was rejected because merely entering
the repository should not mutate process state. A documented repo command or
setup target will start Compose and report readiness as an explicit local setup
step.

### Prepare both environments through `db-migrate`

Docker Compose provisions only cluster objects. Application schemas are
prepared by the two `sumweave db-migrate` invocations above. The second command,
against `sumweave_test`, is the one shallow serialized migration smoke check in
the non-routine verification path. Ordinary persistence tests MUST NOT call
AutoMigrate or otherwise modify schema; they use the schema prepared before the
tagged suites begin.

The finance full-coverage lane reuses that same successful test-environment
command instead of adding a migration test. Only the root
`postgres-test-finance` and `postgres-verify` targets set and export the
target-specific `SUMWEAVE_FINANCE_MIGRATION_COVER_DIR` variable, with the value
`$(CURDIR)/finance/.cover/postgres-migration`; target-specific inheritance
passes it to their shared `postgres-bootstrap` prerequisite. Ordinary
`make postgres-bootstrap` leaves that variable unset and remains an
uninstrumented local-setup command.

When that variable is set, bootstrap removes and recreates its
`raw/` subdirectory at the start of the invocation and does not retain a
readiness marker from an earlier run. It runs the existing one
`db-migrate --env test` command, with the same migrator-role DSNs and
configuration, from `apps/sumweave` as `go run -covermode=atomic` with
`-coverpkg=github.com/gemyago/sumweave/apps/sumweave/cmd/sumweave,github.com/gemyago/sumweave/finance/...` and sets `GOCOVERDIR` only on that command. On success it requires non-empty finance raw data in
`$(SUMWEAVE_FINANCE_MIGRATION_COVER_DIR)/raw` and writes the fresh
`$(SUMWEAVE_FINANCE_MIGRATION_COVER_DIR)/ready` marker. A missing variable,
failed migration, missing marker, empty raw input, or input not recreated by
this bootstrap fails before finance tests or profile checking. Instrumentation
changes neither the command configuration nor its migrator-role ownership, and
MUST NOT cause another `db-migrate --env test` invocation.

Finance `test-postgres` removes and recreates the separate ignored
`finance/.cover/postgres-test-raw` directory, requires the bootstrap `ready`
marker, and runs its current package list with `-coverpkg=./...` and
`-covermode=atomic`, passing `-test.gocoverdir=finance/.cover/postgres-test-raw`
through `go test ... -args`. `GOCOVERDIR` is not used for this test binary:
`-test.gocoverdir` is the required test-binary transport. Before the full
coverage check, `go tool covdata textfmt` reads exactly the bootstrap raw
directory and this raw-test directory, restricted to
`github.com/gemyago/sumweave/finance/...`, and writes the sole
`finance/.cover/postgres.out` profile. It merges matching source blocks rather
than duplicating their denominator.

The finance migrator remains responsible for applying the current model set,
but this early-alpha change does not preserve retired-schema compatibility
cleanup solely to retain detailed migration tests. Obsolete bank-connection
identity cleanup is removed rather than recreating legacy columns or indexes in
a test fixture. Error orchestration that still needs branch coverage is tested
database-free through the smallest consumer-defined seam and generated mocks;
those tests do not invoke GORM AutoMigrate or touch a schema. The successful
GORM executor path is owned by the bootstrap smoke.

No repository-owned replacement migration script or checked-in SQL schema dump
will create application tables. The existing command remains the single entry
point over agent runtime migrations, auth, appdispatch, durable jobs, and
finance AutoMigrate behavior. In particular, it continues to prepare the
job-projection persistence used by observed consumers, immutable dispatch IDs
and topic/consumer-group offsets, and the finance-owned bank-connection schedule
and daily FX refresh due-state tables used by `jobs enqueue-due`.

`sumweave start`, `sumweave start-all`, `sumweave jobs worker`, and
`sumweave jobs enqueue-due` continue to use those same prepared dispatch,
job-projection, and finance schemas without migrating on startup. Scheduler
publication still advances finance-owned due state without creating an observed
job row before worker delivery. The dialect cutover does not weaken these
migration-command or split-process guarantees.

Alternative considered: use a custom setup script to create both cluster
objects and application schemas. This would duplicate behavior already owned by
`db-migrate` and introduce another migration source of truth. Cluster-only setup
is retained because `db-migrate` cannot create its own database or login roles.

### Separate routine tests from tagged PostgreSQL verification

Routine `make test` targets in `runtime/`, `finance/`, and `apps/sumweave/`
remain the targets used by Nx, `make affected-lint-test`, and the reusable
`tests-run.yml` workflow. They run database-free unit and contract tests only,
without probing a port, reading a PostgreSQL DSN, starting Docker, or skipping
at runtime based on database availability. Persistence/composition test files
that require a real database use the `postgres_test` Go build tag and are absent
from the routine test binary. Database constructor validation that does not
connect, and business behavior isolated with project-approved generated mocks,
may remain routine; tests MUST NOT match generated ORM SQL strings.

The root Makefile owns the non-routine targets:

- `make postgres-test-runtime`
- `make postgres-test-finance`
- `make postgres-test-sumweave`
- `make postgres-verify`, which runs all three in that order

Each focused target depends on `postgres-bootstrap`, so invoking it directly
always prepares the schema first. Within one `make postgres-verify` invocation,
Make resolves the shared bootstrap prerequisite once and runs the three backend
targets serially. Each root focused target calls its module's exact
`make test-postgres` target; the database lane is never an implicit dependency
of routine `test`.

The executable coverage contract is identical in `runtime/`, `finance/`, and
`apps/sumweave/`:

- routine `make test` runs `go test` without `-tags`, writes
  `.cover/routine.out`, and checks that profile with
  `.testcoverage-routine.yaml`;
- non-routine `make test-postgres` runs `go test -tags=postgres_test` over the
  same package list and `-coverpkg` scope as that module's routine target,
  and checks `.cover/postgres.out` with the existing `.testcoverage.yaml`;
- runtime and app write `.cover/postgres.out` directly. Finance collects
  ordinary tagged-test raw data separately, then composes it with the fresh
  finance raw data emitted by bootstrap's single test migration into
  `.cover/postgres.out`;
- finance composition uses Go coverage data/profile tooling with one compatible
  coverage mode, exactly the two fresh raw directories above, and the
  `github.com/gemyago/sumweave/finance/...` package restriction; it fails before
  tests or profile checking if the bootstrap input is absent, stale, empty, or
  was not recreated by the current bootstrap;
- both configurations set `threshold.file: 90` and `threshold.total: 90`;
  neither may lower a threshold or add a package/directory/glob exclusion;
- `.testcoverage.yaml` remains the full production-file gate with only the
  module's already-approved generated/glue exclusions. Because untagged tests
  are included by a tagged `go test`, `.cover/postgres.out` combines routine
  and PostgreSQL behavior and MUST cover every non-excluded production file.
  Finance additionally includes the bootstrap migration execution in this same
  final profile; it does not exclude the migrator from the gate;
- `.testcoverage-routine.yaml` starts from those same thresholds and existing
  exclusions. It may additionally omit only exact, anchored individual source
  paths whose executable behavior necessarily requires PostgreSQL and whose
  tests carry `//go:build postgres_test`. Every such path MUST appear in
  `.cover/postgres.out` and pass the full 90% per-file gate there. Broad regular
  expressions, directories, packages, lowered overrides, and new generic
  coverage-ignore annotations are forbidden.

The replacement target-contract assertions MUST independently assert both
90/90 thresholds, unchanged full-configuration exclusions, exact anchored
routine-only paths, and one tagged/full-profile owner for every routine
omission; they MUST NOT compare routine and full YAML files byte-for-byte.
They MUST also assert target-scoped propagation of
`SUMWEAVE_FINANCE_MIGRATION_COVER_DIR`, bootstrap cleanup and readiness,
`GOCOVERDIR` only on the one instrumented test migration,
`-test.gocoverdir` on finance tests, separate raw directories, the exact
`covdata` package restriction and output, absent/stale-input failure, and
exactly one `db-migrate --env test` command. These assertions retain the exact
commands, profile paths, config paths, and one-to-one routine
omission/tagged-ownership rule while proving coverage capture does not add a
second migration invocation.
Implementers first keep or add database-free unit coverage where a generated
mock or pure boundary is legitimate; the exact routine omission list is only
for irreducibly PostgreSQL-backed source files. Thus routine CI retains a 90%
gate over all behavior it executes, while the tagged lane retains the existing
90% full-module contract rather than hiding moved persistence code.

The root routine aggregation MUST remain compatible with the core profile-name
change. Root `make test` continues to write the aggregate profile to
`.cover/profile.out` and MUST consume these inputs after the module tests run:
the unchanged `.cover/profile.out` from each template module
(`tools/firecrawl`, `tools/skills`, and `tools/workspacefs`), and
`.cover/routine.out` from each core module (`runtime`, `finance`, and
`apps/sumweave`). The first input supplies the single coverage-profile header;
the remaining inputs are appended without their headers. The existing root
aggregate actions MUST remain against that profile: `go tool cover -html` writes
`.cover/coverage.html`, and `go-test-coverage --profile .cover/profile.out`
performs the root aggregate check. This root check is in addition to, and does
not replace, each module's lane-specific 90% check.

Target-contract assertions MUST also verify the exact root aggregation input
paths and order, the aggregate profile and report/check paths, and that the
three template modules continue to emit `profile.out`. Verification MUST run
root `make test` with PostgreSQL unavailable and require successful completion
of the routine module tests, root aggregation, report generation, and aggregate
check; it MUST demonstrate that this path does not start, probe, or require
PostgreSQL.

`runtime/` and `finance/` tagged fixtures read one stable
`SUMWEAVE_POSTGRES_TEST_DSN`, supplied by the root target as the runtime-role DSN
for `sumweave_test`. App tagged tests load checked-in `test.yaml`; the local
default already selects that same DSN, while the root app target applies
suite-level standard `APP_` overrides when host or port inputs differ. All
ordinary tagged tests use the prepared fixed prefixes and runtime role; only
bootstrap has migration privileges.

The app fixtures that construct runtime database storage are an explicit
compatibility prerequisite to removing runtime SQLite support. Before the
runtime production cutover, the Engine, `cmd/sumweave` main and
runtime-resolution, application-composition database-runtime, and wireup
migration/jobs/HTTP cases move together to the tagged app lane. They use the
bootstrap-prepared application and `sumweave_runtime_` schemas and MUST NOT run
their own migration. Detailed schema-shape or repeat-migration assertions that
duplicate the task 1.1 bootstrap migration smoke are retired; database-free
configuration, validation, lifecycle, and file-storage cases stay routine. The
tagged app target and PostgreSQL-unavailable repository completion protocol must
both pass, with unchanged coverage gates, before runtime SQLite production code
is removed.

This early compatibility slice does not complete the app test parent task. The
later app test chunk owns every other database-backed command/config, auth,
jobs, finance-registration, controller, Engine, and wireup persistence or
composition case not included in the listed runtime-database fixture set. It
must not repeat or claim the prerequisite fixture conversion a second time.

All database-backed tests use the prepared test database, never the regular
local database. They create randomized IDs and fresh users, tenants, sessions,
jobs, provider records, and other domain state for each case. Reads and
assertions are scoped to the identity or tenant created by the test rather than
assuming globally empty tables. Test helpers open and close connections but do
not run migrations.

This avoids a generic per-test schema lifecycle before a concrete need exists.
If concurrent or repeated execution exposes a conflict, the smallest affected
test boundary is corrected by replacing a static identifier, creating a fresh
tenant, narrowing a query, cleaning up owned rows, or serializing only that
case. The root database lane itself remains serial because backend tasks must be
serialized and all three modules share one database.

Alternative considered: create a database or PostgreSQL schema for every test.
That provides strong isolation but adds lifecycle, permission, and cleanup
machinery not justified by observed failures. Alternative considered: replace
composition coverage with SQL mocks. That would stop exercising the production
database used by jobs, migrations, finance, and appdispatch.

### Keep routine CI database-independent and add manual PostgreSQL CI

The reusable `.github/workflows/tests-run.yml` remains the routine pull-request
lane and MUST NOT start PostgreSQL or call any `postgres-*` target. Its affected
lint and `test` targets therefore remain runnable on an Ubuntu runner where
PostgreSQL is unavailable.

A separate workflow triggered only by `workflow_dispatch` provides the hosted
PostgreSQL lane. It starts Ubuntu's `postgresql.service`, waits with
`pg_isready`, then uses the Ubuntu `postgres` OS account to set an obvious
ephemeral password on the fresh cluster administrator with
`sudo -u postgres psql --set ON_ERROR_STOP=1 --dbname postgres --command "ALTER ROLE postgres PASSWORD 'sumweave_postgres_ci'"`.
It exports
`POSTGRES_BOOTSTRAP_DSN=postgres://postgres:sumweave_postgres_ci@127.0.0.1:5432/postgres?sslmode=disable`
and invokes `POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432
POSTGRES_MANAGED_EXTERNALLY=1 make postgres-verify`. That explicit mode skips
Compose startup, while the supplied privileged DSN performs the complete fresh
service bootstrap: readiness, all three roles, both databases, ownership,
grants, default privileges, two migration invocations, and serial tagged tests.
The manual workflow is not called by `tests-run.yml`, is not a routine required
check, and does not alter the repository completion protocol. Implementers run
the non-routine target when changing the database surface in addition to the
mandatory routine completion protocol.

Alternative considered: provision PostgreSQL in the reusable routine workflow.
That directly violates the repository rule that routine CI tests must not
require PostgreSQL, so PostgreSQL verification is intentionally explicit.

### Remove SQLite from active dependency and documentation surfaces

After code and tests use PostgreSQL, the three core Go product modules will be
tidied and workspace sums synchronized so SQLite drivers and `modernc` packages
no longer appear in their dependency graphs. `go work sync` may update indirect
requirements in other workspace manifests; those mechanical changes are
accepted only when caused by the core removals. Template tools and the current
integration harness receive no test, runtime, or dependency-cleanup commitment.
If one independently imports SQLite, that implementation remains out of scope
and is reported rather than removed by this change.

Active architecture, module docs, manual E2E guides, OpenSpec capabilities, and
agent rules will describe PostgreSQL as the only database and Docker Compose as
the normal local prerequisite. Historical archived change artifacts remain
unchanged because they document prior decisions rather than supported behavior.

## Risks / Trade-offs

- [Local backend processes and database verification require PostgreSQL] →
  Provide explicit Compose setup instructions, fixed local-only credentials, a
  health check, and clear failure guidance; routine tests remain available
  without PostgreSQL.
- [Real PostgreSQL tests can exceed current short module timeouts] → Reuse one
  running test database, migrate it once during setup, and adjust only the
  affected module timeouts to measured values.
- [Tests can collide in the shared test database] → Run module database targets
  serially, require fresh randomized identities and tenant-scoped assertions,
  then narrowly serialize only concrete in-module conflicts.
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
- [Tagged tests can accidentally disappear from routine coverage] → Keep
  database-free behavior tests in the routine lane, keep tagged persistence
  cases explicit in module targets, and enforce both lanes without reducing
  coverage thresholds merely to accommodate the split.
- [Archived documents still contain the word SQLite] → Treat archives as
  immutable history and verify that core product code, configs, active specs,
  docs, and core Go dependency graphs contain no supported SQLite path.

## Migration Plan

This is one source and deployment cutover; the ordered implementation steps do
not create an intermediate supported release.

1. Add the canonical local Compose environment, portable privileged
   `POSTGRES_BOOTSTRAP_DSN` contract, root `postgres-*` targets, and checked-in
   local/test DSNs.
2. Preserve database-free routine targets and their lane-specific 90% coverage
   gate, add full tagged 90% coverage profiles, and move core database
   persistence and composition cases behind the explicit `postgres_test` tag.
3. Convert runtime tagged fixtures, then convert the app fixtures that construct
   runtime database storage as an independently reviewed prerequisite, to the
   prepared shared test database with runtime-role credentials, fixed prefixes,
   randomized identities, scoped reads, and no per-test migrations.
4. Simplify runtime to PostgreSQL-only only after that app compatibility
   prerequisite passes both the tagged app target and the PostgreSQL-unavailable
   repository completion protocol.
5. Convert finance tagged fixtures and the remaining app tagged persistence and
   composition fixtures without repeating the prerequisite app fixture slice.
6. Simplify finance, auth, jobs, and appdispatch to PostgreSQL-only
   implementations and delete SQLite-specific branches and files without
   changing timestamp normalization.
7. Add the manual-dispatch PostgreSQL workflow while leaving reusable routine CI
   free of PostgreSQL setup or dependencies.
8. Tidy the three core Go modules and synchronize workspace sums only as needed;
   verify those core dependency graphs contain no SQLite packages.
9. Update local defaults, PM2/setup guidance, active specs, architecture, manual
   E2E docs, and agent rules to describe the routine and non-routine paths.
10. Run the mandatory repository completion protocol without PostgreSQL, then run
   `make postgres-verify` as the additional database-surface check. Production
   runs the existing
   `db-migrate` command against its current PostgreSQL database before process
   startup; no data conversion occurs.

Rollback is a normal code rollback against the same production PostgreSQL
database. Because no PostgreSQL schema is replaced solely for this cutover, the
previous binary's PostgreSQL path remains the rollback path. Local developers
may reset the repo-scoped Compose volume if desired; no SQLite data is restored.

## Open Questions

There are no blocking design questions. The target names, roles, database names,
privileged bootstrap DSN, table prefixes, migration ownership, coverage
profiles/configurations, and routine/non-routine test boundaries above form the
implementation contract; individual test adaptations may follow nearest module
conventions without changing that contract.
