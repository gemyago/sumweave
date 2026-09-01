## Why

Sumweave production already runs on PostgreSQL, while SQLite support adds a
second persistence and dispatch implementation used only by local development
and tests. Making PostgreSQL the only product database removes dialect drift.
PostgreSQL behavior will be exercised through an explicit non-routine
verification path; routine CI remains database-independent and does not require
PostgreSQL to be available.

## What Changes

- **BREAKING** Remove SQLite as a supported database from the finance module,
  generic runtime, Sumweave application, durable jobs, authentication, and
  application pub/sub transport; PostgreSQL becomes the only accepted database
  dialect.
- Remove SQLite-specific connection setup, WAL and busy-timeout handling, DSN
  detection, GORM dialectors, dialect-specific predicates, dispatch migrations
  and subscriber implementation, tests, and Go dependencies from the core Go
  product modules: `finance/`, `runtime/`, and `apps/sumweave/`.
- Replace local file-database defaults with a repository-managed PostgreSQL
  Docker Compose environment that provisions two databases: one regular local
  database and one database used only by tests.
- Add a canonical repository bootstrap target that starts and waits for Compose
  by default, or accepts a required privileged `POSTGRES_BOOTSTRAP_DSN` for a
  fresh externally managed service, then creates the local roles plus both
  databases, runs `sumweave db-migrate` for the regular and test environments,
  and grants runtime access to the migrated objects.
- Keep routine module `make test`, Nx `test`, `make affected-lint-test`, and the
  reusable pull-request test workflow database-independent. Database-backed
  tests move behind an explicit PostgreSQL test build tag and non-routine root
  target instead of silently starting infrastructure.
- Run the tagged database-backed tests serially against the dedicated
  PostgreSQL test database. Tests MUST use the prepared schema, runtime role,
  fixed table-prefix contract, fresh randomized identities, and scoped data;
  only the bootstrap's serialized `db-migrate --env test` invocation acts as
  the shallow migration smoke check.
- Preserve the existing 90% per-file and total Go coverage gates in both lanes:
  routine targets write and check routine profiles, while tagged targets run
  the untagged plus `postgres_test` tests, write full PostgreSQL profiles, and
  check every production file under the existing full-coverage configuration.
  Routine-only configuration may omit only individually anchored,
  PostgreSQL-owned source files; it cannot add package/directory exclusions or
  reduce either threshold.
- Add a separate manually dispatched PostgreSQL verification workflow. It
  follows the `community-manager` service pattern by starting the Ubuntu
  PostgreSQL service, waiting with `pg_isready`, provisioning the same roles and
  databases, and invoking the same non-routine PostgreSQL verification target.
- Keep the existing explicit `sumweave db-migrate` command and current
  auto-migration mechanisms for agent runtime, auth, dispatch, durable jobs, and
  finance schemas; retain job-projection persistence, authoritative finance
  bank-schedule and FX due-state tables, and the same prepared schemas for
  all-in-one and split process modes. This change does not introduce a new
  migration framework.
- Do not add timestamp or date normalization as part of the dialect cutover.
- Do not migrate or preserve local SQLite data and do not add a compatibility
  period or dual-dialect fallback. Production PostgreSQL data and schemas remain
  in place and continue through the existing migration command.
- Align active architecture, setup, manual E2E, OpenSpec, and agent guidance
  with the PostgreSQL-only contract.

## Capabilities

### New Capabilities

- `postgresql-database-environments`: Define PostgreSQL as the sole supported
  product database, require separate regular and test databases, and separate
  database-independent routine tests from non-routine PostgreSQL verification.

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
- Replaces SQLite-backed persistence and composition fixtures in the core Go
  modules with tagged PostgreSQL-backed tests, while retaining database-free
  unit tests in routine module and Nx targets.
- Affects local configuration, PM2 prerequisites, Docker Compose assets,
  repository setup instructions, core Nx targets, a new manual GitHub Actions
  verification path, and active operational documentation. Routine CI keeps its
  existing no-PostgreSQL contract.
- Removes the pure-Go SQLite driver and its transitive `modernc` dependency tree
  from core product modules. Workspace manifests are synchronized only for
  indirect changes caused by those core removals; template tools and the
  integration harness are not adopted or behaviorally changed by this work.
- Does not change HTTP APIs, finance domain behavior, durable-job semantics,
  application/agent-runtime database configuration boundaries, or the deployed
  PostgreSQL data model beyond schemas produced by the existing migration path.
