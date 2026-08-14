# Database-Backed Application State

## Goal

Keep durable Sumweave state in databases so production app, worker,
scheduler, and migration workloads do not require persistent filesystem
volumes. Finance owns finance data; the retained agent runtime owns its own
runtime state.

Sumweave is early alpha. Local data may be dropped, recreated through
normal schema setup, and reseeded. No backward-compatible file migration is
required.

## Database Ownership

The application database is configured at `application.database` and is shared
by the finance application and its supporting infrastructure:

- finance tenants, accounts, transactions, imports, connections, reporting,
  and current provider snapshots
- authentication users and refresh tokens
- application dispatch transport
- durable jobs and schedules

The agent runtime database is configured separately at
`agentRuntime.database` and stores retained generic agent state such as
sessions, provider configuration, and agent profiles. Separate configuration
keys and table prefixes make ownership explicit even when both databases use
the same PostgreSQL server.

Configuration names are:

- application DSN: `APP_APPLICATION_DATABASE_DSN`
- agent runtime DSN: `APP_AGENTRUNTIME_DATABASE_DSN`
- application table prefix: `sumweave_`
- agent runtime table prefix: `sumweave_runtime_`

SQLite is supported for local development and tests only. The local defaults
are `data/application.db` and `data/agent-runtime.db` relative to
`apps/sumweave`. Production configuration must receive PostgreSQL DSNs
through its environment or secret management; it must not inherit local SQLite
paths.

## Authentication And Signing Keys

Authentication persistence uses the application database. User records and
refresh-token records are database-backed, with opaque refresh tokens stored
only as hashes. Refresh-token rotation consumes a token atomically so a token
can succeed only once under concurrent use.

JWT signing material is not persisted on the filesystem or generated per pod.
Production injects `APP_AUTH_JWTSIGNINGKEY` from a secret. Local configuration
may use an obvious placeholder such as `local-secret-key`; it is not a
production credential. Missing signing material must fail startup clearly.

## Agent Runtime And Filesystem Boundaries

The application always selects database-backed agent runtime persistence for
sessions, provider configuration, and profiles. Agent workspace files and
other temporary execution artifacts may use `dataDir` or an ephemeral path,
but they are not durable application state and do not require a persistent
volume.

Platform-agent skills are read-only runtime assets packaged with the image.
They are not database data. Workspace shell execution remains disabled by
default and must use an ephemeral writable workspace when explicitly enabled.

Optional file-based integrations, including application-terminated TLS,
provider private keys, and file logging, require separate deployment decisions
before they are enabled in a volume-free production workload. Prefer external
secret values and stdout logging.

## Finance Persistence

Finance persistence remains owned by the `finance/` module and is initialized
as part of the application database migration. Finance provider credentials
are encrypted with the configured system key. Current provider snapshots are
sanitized, schema-derived typed documents retained with finance records for
support and debugging; they are not raw response retention or a history
timeline.

Finance persistence uses explicit table prefixes and domain models separate
from persistence models. SQLite remains the local baseline; PostgreSQL is the
production database. The finance module must not import the generic agent
runtime.

## Migration Integration

Run `sumweave db-migrate` before starting processes that use persisted
tables. The command is idempotent and prepares the retained schemas in this
order:

1. database-backed agent runtime schema
2. application authentication schema
3. application dispatch schema
4. durable jobs and schedule schema
5. finance schema

Migration failures must identify the owning component. Migration tests remain
shallow: one schema smoke path and migration-failure context are sufficient.

No data migration from abandoned local files or obsolete databases is required.
For this early-alpha provider-snapshot change, recreate local and test
databases instead of retaining retired provider source-data tables or rows.

## Worker And Scheduler Lifecycle

Durable jobs use the application database. API, worker, and scheduler processes
share the same configured application database while retaining separate
process modes for deployment:

- `start` serves the API without executing durable jobs inline
- `jobs worker` consumes finance jobs
- `jobs enqueue-due` enqueues due finance schedules
- `start-all` combines the API, worker, and scheduler for local development

Worker shutdown must propagate `SIGINT` and `SIGTERM` through the root context
so active polling stops cleanly. Keep one worker replica until job claims,
leases, stale recovery, and duplicate execution are safe for horizontal
scaling.

## Verification

1. Run `db-migrate` against fresh local application and agent-runtime SQLite
   databases.
2. Reseed the first `.local-users` entry and verify login and token rotation.
3. Start the API and worker with no persistent data directory.
4. Verify finance data, current finance provider snapshots, agent sessions, provider
   configuration, and profiles survive process replacement through database
   reads.
5. Repeat migration and smoke behavior against PostgreSQL.
6. Run `make affected-lint-test` for implementation changes.

## Completion Criteria

This design is satisfied when:

- finance, authentication, dispatch, jobs, and schedules use the application
  database
- retained agent runtime state uses its configured agent runtime database
- JWT signing material is externally configured
- `db-migrate` prepares all retained schemas
- production workloads require no persistent filesystem volume
- local development continues to work with SQLite
- PostgreSQL smoke verification passes

## Provider snapshot upgrade cleanup

GORM auto-migrate creates `finance_provider_snapshots` but does not drop retired
tables. After confirming a persistent upgraded deployment is healthy, the new
snapshot table exists, and rollback to a legacy build is no longer needed, run:

```sql
DROP TABLE IF EXISTS finance_provider_evidence;
DROP TABLE IF EXISTS finance_raw_payloads;
```

Confirm both retired tables are absent. Fresh and recreated local/test databases
need no manual drop; this early-alpha change requires no row copy or backup.
