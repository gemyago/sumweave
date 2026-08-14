## 1. Topic-Aware SQL Transport

- [x] 1.1 Replace the early-alpha dispatch schema with topic-aware messages and composite topic/consumer-group offsets for SQLite and PostgreSQL; follow TDD flow by updating the shallow migration smoke coverage before implementing the migration builders.
- [x] 1.2 Generalize appdispatch messages and publishers around explicit topic, payload, metadata, and message identity while preserving transaction-bound publication; follow TDD flow by adding publish, commit, rollback, validation, and driver-selection cases before changing the publisher.
- [x] 1.3 Implement topic-scoped SQLite subscription locking, acknowledgement, fan-out, same-group coordination, and restart resume; follow TDD flow by extending the SQLite transport behavior suite before updating its queries and subscription state.
- [x] 1.4 Adapt the PostgreSQL single-table Watermill schema and offsets to topic-scoped ordering and consumer groups; follow TDD flow by adding focused adapter/query behavior tests before implementing the PostgreSQL adapters.

## 2. Routers And Typed Domain Events

- [x] 2.1 Add appdispatch router construction for explicit consumer groups and handlers on multiple topics; follow TDD flow by proving topic routing, independent groups, same-group behavior, construction without startup, cancellation, and clean closure before implementing the router.
- [x] 2.2 Add bounded retry, panic recovery, and durable dead-letter middleware without terminating unrelated subscriptions; follow TDD flow by covering transient success, exhausted failure, poison-publish failure, preserved diagnostic metadata, and continued routing before wiring the middleware.
- [x] 2.3 Add the `internal/appevents` typed domain-event contract, publisher adapter, and generic typed handler decoding; follow TDD flow by covering valid publication, empty-topic rejection, serialization errors, typed dispatch, and malformed payloads before implementing the package.

## 3. Durable Jobs Adaptation

- [x] 3.1 Move the execution envelope, topic, and consumer-group ownership into the jobs package and adapt enqueue to the generalized transaction-bound publisher; follow TDD flow by updating atomic enqueue, rollback, idempotency, and transport-isolation expectations before changing the service.
- [x] 3.2 Migrate the jobs worker onto the named router while preserving business-failure job outcomes and bounded one-shot execution; follow TDD flow by updating worker lifecycle, success, failure, cancellation, and `RunOnce` cases before replacing the consumer loop.
- [x] 3.3 Make duplicate delivery of terminal or otherwise non-queued jobs acknowledge without executing; follow TDD flow by adding queued-claim, running, succeeded, failed, and canceled delivery cases before tightening worker claim handling.
- [x] 3.4 Update `jobs.NewModule` and the explicit `wireup.BuildJobs` root to own jobs messaging resources and preserve split-command cleanup errors; follow TDD flow by extending module construction, jobs-root resolution, idempotent close, and operation-plus-cleanup error cases before changing the wireup.
- [x] 3.5 Update `wireup.BuildHTTP` and `start-all` to share the jobs module without starting consumption in API-only mode and to close messaging resources before the application database; follow TDD flow by extending HTTP-root and process-mode lifecycle tests before changing the wireup.
- [x] 3.6 Update `wireup.BuildMigration` and `DatabaseMigrator` to prepare the generalized pub/sub schema without constructing runtime publishers or routers; follow TDD flow by adjusting the shallow migration and narrow-root tests before changing migration composition.

## 4. Documentation And Contract Alignment

- [x] 4.1 Document the separate job-command and domain-event models, shared transport, at-least-once guarantees, dead-letter behavior, schema reset, explicit wireup roots, and process ownership; follow TDD flow by updating documentation assertions where present before aligning `AGENTS.md`, architecture, terminology, module docs, and migration guidance.
