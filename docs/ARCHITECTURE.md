# Architecture

Sumweave is a finance-only system. Its product domain is tenant-aware
financial management: members, accounts, categories and tags, ledger
transactions and transfers, CSV imports, bank connections and sync, current
provider snapshots, balances, reports, and FX.

## Boundaries

- `finance/` owns finance domain services and persistence models and remains
  independent from `runtime/`.
- `apps/sumweave/` composes finance, auth, durable jobs, dispatch, HTTP,
  migrations, telemetry, and embedded UI delivery.
- `apps/sumweave/internal/appdispatch` is the app-owned durable SQL pub/sub
  transport. It stores multiple topics in one message table and tracks offsets
  independently by topic and consumer group for SQLite and PostgreSQL. It is the
  generic internal delivery mechanism for both imperative commands and factual
  domain events; it provides no user-facing execution model by itself.
- `apps/sumweave/internal/appevents` is the typed domain-event API over
  `appdispatch`.
- `apps/sumweave/internal/jobs` adds opt-in product and API visibility to
  selected background commands. A visible job records identity, lifecycle,
  attempts, and a sanitized outcome while its command still travels through
  `appdispatch`. Background processing that needs no product or operational
  visibility may use `appdispatch` without creating a job record.
- Its application config is app-internal at `internal/config`; `internal/wireup`
  owns explicit command roots. Command and Engine entrypoints pass typed startup
  options, while components receive native values or constructed collaborators.
- `apps/sumweave-ui/` is finance-first, with retained Admin, Chat, and Providers
  surfaces.
- `runtime/` is generic agent infrastructure only: sessions, profiles,
  providers, agent execution, and HTTP APIs.
- `.platform-agents/skills/` is an intentionally empty stageable root until
  finance-specific platform skills are introduced.

## Operations

The application database is configured at `application.database` (environment
override `APP_APPLICATION_DATABASE_DSN`). Local default storage is
`data/application.db`; SQLite is local development only. `db-migrate` prepares
agent, auth, topic-aware dispatch, jobs, and finance schemas. `start-all` runs
the API, worker, and scheduler together; split worker and scheduler commands
remain for deployment. API-only `start` constructs enqueue capabilities but
does not start a message router.

Message delivery is at least once. Consumer handlers must tolerate duplicate
delivery. Routers recover panics, retry failures three times with bounded
backoff, and then publish the original message to
`app.dispatch.dead-letter.v1` with failure and source-topic metadata. A failed
dead-letter publication leaves the original message unacknowledged.

Transport failure and job failure are separate concerns. Transient delivery or
infrastructure failures follow the `appdispatch` retry and dead-letter policy.
A handled business failure for observable work is persisted as a failed job and
the command is acknowledged. Non-observable background work has no job state;
its failure remains visible through logs and the dispatch failure policy.

The topic-aware dispatch schema intentionally replaces the earlier alpha
single-topic schema. Recreate or reseed an old local application database, then
run `sumweave db-migrate`; mixed old and new binaries are unsupported. Explicit
jobs and HTTP roots stop their message routers before closing the shared
application database. The migration root creates no publisher or router.

Run `sumweave db-migrate` before `sumweave start-all` locally.

Release builds run on the host with `make -C build dist`; Docker packages the
prepared binary and staged platform-agent root. The Helm chart deploys app,
singleton worker, scheduler, and migration processes because finance uses all
of them. It can also run an optional post-migration initial-user bootstrap Job
from credentials held in a consumer-managed Kubernetes Secret.
