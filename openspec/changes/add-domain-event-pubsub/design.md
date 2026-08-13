## Context

Sumweave currently has two durable records for background work: a job row that
tracks lifecycle and an `appdispatch` message that triggers execution. The
transport has one topic (`app.dispatch.execution.v1`), one default consumer
group, and a handler loop that stops on the first unhandled error. Although the
package is named like a general application dispatch system, all production
publishers and consumers belong to durable jobs.

`community-manager` at commit `d71b74fb` provides the reference behavior for
this change: typed events choose their own topics, routers are created for named
consumer groups, offsets are tracked per topic and group, and Watermill
middleware supplies retry, panic recovery, and a poison queue. Sumweave also
supports SQLite for local development and already guarantees atomic job-row and
message creation, so the reference implementation must be adapted rather than
copied literally.

The resulting architecture needs two semantic layers over one durable SQL
transport:

- durable-job commands ask one logical worker group to perform work and retain
  observable job state;
- domain events announce facts that already happened and allow independent
  consumer groups to react.

Main now uses hand-written, command-specific roots under `internal/wireup`
instead of a dependency-injection container. `jobs.NewModule` constructs the
shared jobs capabilities, `BuildJobs` owns split worker/scheduler resources,
`BuildHTTP` owns API and local `start-all` resources, and `BuildMigration` owns
explicit schema preparation. The design must extend those roots without
reintroducing container registration or allowing components to read app config.

## Goals / Non-Goals

**Goals:**

- Provide real multi-topic, multi-consumer-group pub/sub semantics on SQLite and
  PostgreSQL without adding an external broker.
- Provide typed domain-event publishing and typed handler registration similar
  to `community-manager`.
- Preserve the durable-jobs lifecycle and its atomic job/message enqueue.
- Preserve explicit root ownership, cleanup error propagation, and API-only
  startup behavior.
- Isolate handler failures with bounded retry, panic recovery, and dead-letter
  delivery instead of terminating all subscriptions.
- Make delivery behavior testable at the transport, router, and jobs-adapter
  boundaries.

**Non-Goals:**

- Recast job execution commands as domain events.
- Introduce Kafka, NATS, Redis, or another external messaging service.
- Add a production finance event solely as a demonstration when no product
  consumer currently requires it.
- Make multiple job-worker replicas safe; leases and horizontally safe stale
  recovery remain separate work.
- Add event-table retention or an operator dead-letter UI in this change.
- Preserve existing early-alpha dispatch messages or offsets across the schema
  replacement.

## Decisions

### Use one transport with separate job and event APIs

`appdispatch` will become the low-level topic-aware SQL transport. Its published
message contract will contain a topic, opaque payload, message identifier, and
metadata; it will not impose job fields such as execution kind or observable job
ID. It will provide normal and transaction-bound publication plus a router
factory that creates a subscriber for an explicit consumer-group name.

A new `internal/appevents` package will define the small typed contract used by
publishers and consumers:

- an `Event` exposes `Topic() string`;
- a publisher JSON-encodes an event and delegates to `appdispatch`;
- `MakeHandler` JSON-decodes a concrete event type before invoking its handler;
- a router registers typed handlers by event topic.

The jobs package will own its execution envelope and continue using one
dedicated execution topic and consumer group. `jobs.NewModule` will receive
native transport inputs and construct the job publisher and worker without
starting consumption. This keeps job-specific concepts out of the domain-event
API while allowing both models to share transport, offset, migration, and
middleware code.

Alternative considered: retain job-only `appdispatch` and create a second full
events transport. This gives clean packages but duplicates SQL schemas,
publisher/subscriber adapters, locking, migrations, and tests. Alternative
considered: publish each job type as a domain-event topic. This obscures the
fact that a job message is an imperative command with one logical executor and
different retry semantics.

### Store all topics in one message table

The application dispatch message table will add a required `topic` column. The
offset table identity will become `(topic, consumer_group)`. Queries and indexes
will order messages deterministically within a topic, while each consumer group
advances its own offset. Two groups therefore receive the same message
independently; two instances using the same group coordinate as one logical
consumer.

PostgreSQL will adapt the single-table Watermill schema and offsets approach
from `community-manager`, including transaction-aware ordering where required.
SQLite will extend the existing custom transport so locking and offset updates
are scoped by topic and consumer group. The schema remains explicitly created
by `db-migrate`; constructors and startup paths never create it implicitly.

Alternative considered: one SQL table per topic. That follows Watermill's
default model but makes dynamic application topics into schema concerns and
increases migration and operational overhead.

### Use at-least-once delivery with consumer-owned idempotency

A subscriber acknowledges a message only after its handler succeeds or after
the failed message has been durably routed to the dead-letter topic. A process
failure before acknowledgement can redeliver the message. Event handlers must
therefore be safe for duplicate delivery, normally through domain idempotency
keys or persisted reaction state.

The transport will preserve message identifiers and original-topic metadata on
dead-letter messages so a failed reaction can be diagnosed and replayed by a
future operator workflow.

Exactly-once handling was rejected because it cannot be guaranteed across SQL
state and arbitrary external side effects such as provider or notification
calls.

### Put retry and panic recovery in the router

Routers will apply Watermill middleware in the same behavioral order as the
reference implementation: recover panics as handler errors, retry failures a
bounded number of times with a short backoff, then publish a poison message to a
dedicated dead-letter topic. Exhausting one message must not terminate handlers
for other topics.

The first implementation will use small application constants matching the
reference behavior rather than adding configuration that has no current
operator need. The retry policy can become configuration when a real event
consumer requires different timing.

### Keep job failures distinct from transport failures

The jobs adapter will convert an executed handler's business failure into a
persisted `failed` job and return success to the router, causing the command
message to be acknowledged. An explicit job retry continues to create a new job
and message. Store, decoding, or dispatch infrastructure failures may return to
the router and use transport retry/dead-letter behavior.

Before executing, the worker must atomically claim a queued job. A redelivery
for a terminal or otherwise non-queued job will be acknowledged without running
the handler again. This removes the current path that can execute after
`ErrJobNotQueued` and makes at-least-once transport delivery safe at the jobs
boundary without claiming exactly-once external effects.

### Support transaction-bound publication

The low-level publisher will preserve `PublishInTx`, generalized to accept a
topic and payload. Job enqueue will continue creating the job row and command
message in one application-database transaction. Future domain-event producers
that mutate the same database can use the transaction-bound publisher through
their application orchestration seam, avoiding the reference implementation's
gap between committed state and a separately published event.

The ordinary typed event publisher remains available for facts produced after
external or already-committed operations. Transaction ownership stays in the
orchestration layer rather than leaking SQL dependencies into `finance/`.

### Start routers with the component that owns the reactions

There will be no global event router with an empty or catch-all handler set.
Each future reaction set will create a named router in its owning explicit
wireup root after its handlers are available, and that root will decide whether
to run it. In this change, the existing jobs worker is the production consumer
migrated onto the new router behavior; transport and router integration tests
prove independent event groups until a real finance event consumer is
requested. Constructing the API-only HTTP root will continue to construct jobs
for enqueue APIs without starting job or domain-event consumption.

This follows `community-manager`, where the Discord router is owned by the
Discord process, while avoiding a new Sumweave process mode with no product
work.

### Preserve explicit root resource ownership

Publishers, subscribers, and routers will expose native close behavior to their
owner. A router's blocking `Run` will finish cancellation and subscriber cleanup
before returning. Long-lived transport resources constructed by
`jobs.NewModule` will be closed through the owning `JobsRoot` or `HTTPRoot`
before their shared application database is released. Root and CLI cleanup will
remain idempotent and will join operation and cleanup errors instead of hiding
either one.

`BuildMigration` will continue constructing only migration dependencies. It
will use the generalized appdispatch migrator directly and will not construct a
jobs module, event publisher, router, finance service, or HTTP server.

Alternative considered: register transport closers as independent concurrent
shutdown hooks. This does not guarantee that routers stop before their shared
database closes, so ordered ownership at the explicit root boundary is safer.

## Risks / Trade-offs

- [At-least-once delivery can repeat external side effects] -> Require
  idempotent event handlers and make non-queued job redelivery a no-op.
- [A shared table can grow indefinitely] -> Keep topic/ordering indexes and
  document that retention is follow-up work before message volume warrants it.
- [A poison-message publish can itself fail] -> Return and log that
  infrastructure failure so the original message remains unacknowledged rather
  than being silently lost.
- [Shared storage could couple jobs and events operationally] -> Isolate
  offsets and locks by topic and consumer group, and keep semantic APIs and
  handler registries separate.
- [Replacing the schema discards local pending work] -> Use the repository's
  early-alpha compatibility policy, require `db-migrate` on a recreated/reseeded
  database, and do not add compatibility branches.
- [SQLite and PostgreSQL implementations can drift] -> Run the same transport
  behavior suite against both drivers where routine CI permits, retaining only
  shallow migration smoke coverage.
- [Router cleanup can race shared database shutdown] -> Make explicit roots
  close owned messaging resources before performing the remaining root
  shutdown and preserve both operation and cleanup errors.

## Migration Plan

1. Replace the dispatch migration with the topic-aware message and composite
   offset schema for both SQLite and PostgreSQL.
2. Generalize `appdispatch` publication and subscription around explicit topics
   and named consumer groups.
3. Add the typed domain-event publisher, handler, and router facade.
4. Migrate `jobs.NewModule` and the worker to the jobs-owned execution envelope,
   topic, and consumer group while preserving transaction-bound enqueue.
5. Extend `BuildJobs` and `BuildHTTP` with ordered messaging-resource ownership;
   keep API-only startup enqueue-only and preserve cleanup errors.
6. Update `BuildMigration` and `DatabaseMigrator` to prepare only the generalized
   topic-aware transport schema.
7. Add retry, panic recovery, dead-letter, fan-out, restart-resume, root cleanup,
   and duplicate job-delivery coverage.
8. Update architecture, terminology, migration, and process documentation.
9. Recreate/reseed local application databases, run `db-migrate`, and verify the
   worker and scheduler lifecycle.

Rollback during development is a code rollback followed by recreation of the
early-alpha application database. Mixed old/new binaries against the same
dispatch schema are unsupported.

## Open Questions

There are no blocking implementation questions. The first product domain event,
its consumer, message retention, and a dead-letter operator workflow are
intentionally deferred until a concrete finance use case requires them.
