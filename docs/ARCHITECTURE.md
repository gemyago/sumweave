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
- `apps/sumweave/internal/appdispatch` is the app-owned durable PostgreSQL pub/sub
  transport. It stores multiple topics in one message table and tracks offsets
  independently by topic and consumer group. It is the
  generic internal delivery mechanism for semantic imperative commands and
  factual domain events. Publication assigns and returns one immutable, stable
  message ID and creates no job record. It is the only durable publication,
  scheduling, and delivery path for background commands and events; producers
  do not choose a jobs queue.
- `apps/sumweave/internal/appevents` is the typed domain-event API over
  `appdispatch` and is intentionally retained as a separate typed adapter.
- `apps/sumweave/internal/jobs` adds opt-in product and API visibility to
  selected background commands at consumer registration. A job-observed
  consumer lazily creates a metadata-only lifecycle projection on first
  delivery; its job ID is the dispatch message ID. Background processing that
  needs no product or operational visibility uses `appdispatch` without a job
  record.
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
override `APP_APPLICATION_DATABASE_DSN`) and PostgreSQL is the only supported
database. Local configuration uses the `sumweave_runtime` role against Compose's
`sumweave_local` database; agent runtime has its separate configuration key but
uses that same database with the `sumweave_runtime_` prefix. Run
`make postgres-bootstrap` before backend processes: it provisions the
`sumweave_owner`, `sumweave_migrator`, and `sumweave_runtime` roles; prepares
`sumweave_local` and `sumweave_test`; runs `sumweave db-migrate` once for each;
and grants the runtime role access to the migrated tables and sequences. After
that setup, local PM2 runs the API-only `start` and `jobs worker` commands as
separate processes; `jobs enqueue-due` remains a separate scheduler tick. The
combined `sumweave start-all` command is available for diagnostics. API-only
`start` can publish dispatch messages but does not start a message router or
execute background work.

The retained process modes are `start` for API-only serving, `jobs worker` for
the durable appdispatch consumer, and `jobs enqueue-due` for one scheduler tick.
`start-all` explicitly combines those three capabilities for diagnostics; the
worker and scheduler remain separate deployment processes.

The scheduler reads finance-owned bank-connection schedules and the finance-owned
daily FX refresh schedule. It publishes one semantic command per due occurrence,
advances the schedule, and stores the returned message ID as the future reference
in one application-database transaction. It does not use a generic schedule
registry, execute provider work, or create a job row.

Message delivery is at least once. Consumer handlers must tolerate duplicate
delivery. Routers recover panics, retry failures with the configured bounded
dispatch policy, and then publish the original message to
`app.dispatch.dead-letter.v1` with failure and source-topic metadata. A failed
dead-letter publication leaves the original message unacknowledged.

Dispatch retention is operational transport maintenance, not job retention.
Normal message rows are eligible for deletion after 7 days only when every
existing consumer-group offset for that topic has advanced beyond the row;
unacknowledged rows remain available for retry and are surfaced for operator
attention. Idempotency claims are retained for at least the same window so a
retry within the window returns the original message identity. Dead-letter
rows are retained for 30 days for diagnostics, then may be removed by the same
offset-safe maintenance process. This policy is internal to appdispatch: no
raw message or dead-letter entity is exposed through the jobs API, and it does
not define completed-job retention. No runtime cleanup is performed until an
operator maintenance command is required; workers never delete dispatch data
as part of delivery.

Each message may have at most one job-observed consumer registration. Other
consumers are ordinary appdispatch consumers; an event reaction that needs an
independent visible execution publishes a distinct semantic command. A job
projection is materialized lazily on first delivery, before the handler runs,
and uses the message ID as its job ID. Until that delivery, a request for the
known future job ID may return `404`; the UI treats that response as pending
only for an ID it just received from a dispatching workflow. Unknown or deep
linked IDs remain ordinary `404` errors.

Transport failure and job failure are separate concerns. A finance service must
explicitly return a finance-owned terminal failure for a terminal domain or
provider outcome. Only the finance adapter maps that typed outcome to a handled
failure, preserving its sanitized code, summary, and details for a failed
observed job before acknowledgement. Unclassified service, malformed payload,
materialization/claim, handler-panic, and terminal-state persistence failures
follow the `appdispatch` retry/dead-letter policy; terminal-state persistence is
retried with backoff while the delivery context remains alive, and shutdown leaves
the message unacknowledged. Ordinary consumers log explicit terminal failures and
create no job state.

Only running projections whose durable claim timestamp is at least the
worker-level `staleRunningAge` old are requeued or terminally failed (the
default age is five minutes). Recovery conditionally matches the claim owner
and timestamp, so it cannot overwrite a newer claim or terminal transition.
The uniform attempt policy defaults to three attempts; handlers and individual
rows do not define competing retry limits.

The topic-aware dispatch schema intentionally replaces the earlier alpha
single-topic schema. Recreate the local Compose PostgreSQL volume if a clean
environment is required; there is no SQLite data migration. Explicit jobs and
HTTP roots stop their message routers before closing the shared application
database. The migration root creates no publisher or router.

Run `make postgres-bootstrap` before ordinary module or Nx backend tests. Each
core module's `make test` selects tagged PostgreSQL tests with the prepared
`sumweave_test` schema; there is no separate verification lane or workflow.

Release builds run on the host with `make -C build dist`; Docker packages the
prepared binary and staged platform-agent root. The Helm chart deploys app,
singleton worker, scheduler, and migration processes because finance uses all
of them. It can also run an optional post-migration initial-user bootstrap Job
from credentials held in a consumer-managed Kubernetes Secret.
