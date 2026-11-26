---
type: Architecture
title: "Persistence and migration"
description: "Database ownership, secrets, explicit migrations, and filesystem boundaries."
sources:
  - id: database-backed-state-plan
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/docs/database-backed-state-plan.md"
    title: "docs/database-backed-state-plan.md"
  - id: database-migrate-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/apps/sumweave/internal/database_migrate.go"
    title: "apps/sumweave/internal/database_migrate.go"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/architecture/persistence.md"
    title: "Reviewed documentation checkpoint before this format migration"
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:28:13+01:00" }
---

# Persistence and migration

## Goal

Keep durable Sumweave state in databases so production app, worker,
scheduler, and migration workloads do not require persistent filesystem
volumes. Finance owns finance data; the experimental agent runtime owns its separate
runtime state. Its current migration and startup dependencies remain operationally required.[^documentation-snapshot][^database-backed-state-plan][^database-migrate-go]

Sumweave is early alpha. Local data may be dropped, recreated through
normal schema setup, and reseeded. No backward-compatible file migration is
required.

## Database Ownership

The application database is configured at `application.database` and is shared
by the finance application and its supporting infrastructure:[^documentation-snapshot]

- finance tenants, accounts, transactions, imports, connections, reporting,
  and current provider snapshots
- authentication users, hashed refresh tokens, and digest-only personal access tokens
- application dispatch transport
- durable job projections and finance-owned schedules

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

PostgreSQL is the only supported database. Checked-in local configuration uses
the `sumweave_runtime` role and `sumweave_local` database exposed by the
repository-managed Compose service; checked-in test configuration uses the same
role and `sumweave_test`. Both use the fixed `sumweave_`,
`sumweave_runtime_`, and `finance_` table-prefix contract. Production receives
its PostgreSQL DSNs through environment or secret management. Bootstrap owns the
concrete local roles: `sumweave_owner` owns each database, `sumweave_migrator`
runs DDL, and `sumweave_runtime` runs backend processes and database-backed tests.

## Authentication And Signing Keys

Authentication persistence uses the application database. User records and
refresh-token records are database-backed, with opaque refresh tokens stored
only as hashes. Refresh-token rotation consumes a token atomically so a token
can succeed only once under concurrent use.[^documentation-snapshot]

JWT signing material is not persisted on the filesystem or generated per pod.
Production injects `APP_AUTH_JWTSIGNINGKEY` from a secret. Local configuration
may use an obvious placeholder such as `local-secret-key`; it is not a
production credential. Missing signing material must fail startup clearly.

## Agent Runtime And Filesystem Boundaries

The application always selects database-backed agent runtime persistence for
sessions, provider configuration, and profiles. Agent workspace files and
other temporary execution artifacts may use `dataDir` or an ephemeral path,
but they are not durable application state and do not require a persistent
volume.[^documentation-snapshot]

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
timeline.[^documentation-snapshot]

Finance persistence uses explicit table prefixes and domain models separate
from persistence models. It uses PostgreSQL in local, test, and production
environments. The finance module must not import the generic agent runtime.

## Migration Integration

Run `make postgres-bootstrap` before starting local processes that use persisted
tables. It starts and waits for Compose, provisions roles and databases, runs
`sumweave db-migrate` once with the migrator role for each of `local` and `test`,
then grants runtime access. The migration command is idempotent and prepares the
retained schemas in this order:[^documentation-snapshot]

1. database-backed agent runtime schema
2. application authentication schema (users, refresh tokens, and personal access tokens)
3. application dispatch schema
4. durable jobs schema (finance schedule tables migrate with finance)
5. finance schema

Migration failures must identify the owning component. Migration tests remain
shallow: one schema smoke path and migration-failure context are sufficient.

The topic-aware appdispatch schema replaces the earlier alpha single-topic
schema. Existing incompatible local databases must be recreated through the
normal Compose/bootstrap flow. Publishers and routers never create tables
implicitly; startup commands do not migrate app-owned schemas.

No data migration from abandoned SQLite files or obsolete databases is required.
For this early-alpha provider-snapshot change, recreate local and test
PostgreSQL databases instead of retaining retired provider source-data tables or
rows. The cutover does not change date or timestamp normalization.

## Worker And Scheduler Lifecycle

API, worker, and scheduler share the application database. Their process split
and delivery contract are defined in [background processing](background-processing.md);
[local development](../operations/local-development.md) owns startup commands.[^documentation-snapshot]

Worker shutdown must propagate `SIGINT` and `SIGTERM` through the root context
so active polling stops cleanly. Keep one worker replica until job claims,
leases, stale recovery, and duplicate execution are safe for horizontal
scaling.

## Verification procedure (not a record of a completed run)

1. Run `make postgres-bootstrap` to provision and migrate fresh local and test
   PostgreSQL databases.
2. Reseed the first `.local-users` entry and verify login and token rotation.
3. Start the API and worker with no persistent data directory.
4. Verify finance data and current provider snapshots survive replacement through
   database reads. When changing experimental runtime persistence, also verify
   sessions, model-provider configuration, and profiles.
5. Run ordinary module `make test` commands for database-backed PostgreSQL coverage.
6. Run `make affected-lint-test` for implementation changes after bootstrap.

## State ownership summary

The current persistence boundary is:[^documentation-snapshot]

- finance, authentication, dispatch, jobs, and schedules use the application
  database
- retained agent runtime state uses its configured agent runtime database
- JWT signing material is externally configured
- `db-migrate` prepares all retained schemas
- production workloads require no persistent filesystem volume
- local development uses the Compose PostgreSQL environment
- ordinary database-backed PostgreSQL tests use the prepared schema and existing timestamp semantics

## Provider snapshot upgrade cleanup

GORM auto-migrate creates `finance_provider_snapshots` but does not drop retired
tables. After confirming a persistent upgraded deployment is healthy, the new
snapshot table exists, and rollback to a legacy build is no longer needed, run:[^documentation-snapshot]

```sql
DROP TABLE IF EXISTS finance_provider_evidence;
DROP TABLE IF EXISTS finance_raw_payloads;
```

Confirm both retired tables are absent. Fresh and recreated local/test databases
need no manual drop; this early-alpha change requires no row copy or backup.

## References

Requirements below are in this bundle; implementation references are external:[^documentation-snapshot]

- [PostgreSQL environments](../requirements/postgresql-database-environments.md)
- [Migration command](../requirements/database-migration-command.md)
- [Migration implementation](../../apps/sumweave/internal/database_migrate.go)
- [Experimental runtime scope](../experiments/agent-runtime.md)

[^documentation-snapshot]: Reviewed documentation checkpoint, preserved as the source of this section; format and navigation changes do not establish execution results.

[^database-backed-state-plan]: docs/database-backed-state-plan.md.
[^database-migrate-go]: apps/sumweave/internal/database_migrate.go.
