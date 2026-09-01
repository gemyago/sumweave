## Why

Sumweave production already runs on PostgreSQL, while SQLite support adds a
second persistence and dispatch implementation that is used only by local
development and tests. Making PostgreSQL the only database removes dialect
drift and ensures local and CI verification exercise the same database behavior
as production.

## What Changes

- **BREAKING** Remove SQLite as a supported database from the finance module,
  generic runtime, Sumweave application, durable jobs, authentication, and
  application pub/sub transport; PostgreSQL becomes the only accepted database
  dialect.
- Remove SQLite-specific connection setup, WAL and busy-timeout handling, DSN
  detection, GORM dialectors, timestamp predicates, dispatch migrations and
  subscriber implementation, tests, and Go dependencies from every active
  module.
- Replace local file-database defaults with a repository-managed PostgreSQL
  Docker Compose environment that provisions two databases: one regular local
  database and one database used only by tests.
- Make repository setup run `sumweave db-migrate` for both the regular local
  environment and the test environment before backend startup or test
  execution; retain the existing AutoMigrate-backed schema preparation behind
  that command.
- Run database-backed tests against the dedicated PostgreSQL test database.
  Tests MUST use fresh randomized identities and tenant-scoped data rather than
  relying on an empty database; address concrete concurrency conflicts in the
  affected tests without introducing a per-test database or schema framework by
  default.
- Update GitHub Actions to follow the `community-manager` service pattern:
  start the Ubuntu PostgreSQL service, wait for readiness, provision the regular
  and test databases, and run `sumweave db-migrate` for both environments before
  lint and test execution.
- Keep the existing explicit `sumweave db-migrate` command and current
  auto-migration mechanisms for agent runtime, auth, dispatch, durable jobs, and
  finance schemas; this change does not introduce a new migration framework.
- Do not migrate or preserve local SQLite data and do not add a compatibility
  period or dual-dialect fallback. Production PostgreSQL data and schemas remain
  in place and continue through the existing migration command.
- Align active architecture, setup, manual E2E, OpenSpec, and agent guidance
  with the PostgreSQL-only contract.

## Capabilities

### New Capabilities

- `postgresql-database-environments`: Define PostgreSQL as the sole supported
  database and require separate regular and test databases for local development
  and CI.

### Modified Capabilities

- `database-migration-command`: Require the explicit migration command and
  standard environment setup to operate against prepared PostgreSQL databases
  only while retaining the existing schema initialization mechanisms.
- `domain-event-pubsub`: Remove multi-driver schema preparation and require the
  durable topic transport, offsets, publication, and subscription behavior on
  PostgreSQL only.
- `finance-management`: Replace SQLite-local/PostgreSQL-production
  compatibility requirements with PostgreSQL-only finance persistence and
  schema preparation.

## Impact

- Affects database construction and persistence code in `finance/`, `runtime/`,
  and `apps/sumweave/`, especially application SQL connections, GORM setup,
  auth, jobs, appdispatch, migrations, and explicit wireup roots.
- Replaces SQLite-backed unit and composition fixtures across the Go modules
  with PostgreSQL-backed tests using the dedicated test database and fresh
  randomized domain data.
- Affects local configuration, PM2 prerequisites, Docker Compose assets,
  repository setup instructions, Nx/GitHub Actions test execution, and active
  operational documentation.
- Removes the pure-Go SQLite driver and its transitive `modernc` dependency tree
  from product modules and downstream workspace module manifests.
- Does not change HTTP APIs, finance domain behavior, durable-job semantics,
  application/agent-runtime database configuration boundaries, or the deployed
  PostgreSQL data model beyond schemas produced by the existing migration path.
