## 1. Topic-Aware SQL Transport

- [ ] 1.1 Replace the early-alpha dispatch schema with topic-aware messages and composite topic/consumer-group offsets for SQLite and PostgreSQL; follow TDD flow by updating the shallow migration smoke coverage before implementing the migration builders.
- [ ] 1.2 Generalize appdispatch messages and publishers around explicit topic, payload, metadata, and message identity while preserving transaction-bound publication; follow TDD flow by adding publish, commit, rollback, validation, and driver-selection cases before changing the publisher.
- [ ] 1.3 Implement topic-scoped SQLite subscription locking, acknowledgement, fan-out, same-group coordination, and restart resume; follow TDD flow by extending the SQLite transport behavior suite before updating its queries and subscription state.
- [ ] 1.4 Adapt the PostgreSQL single-table Watermill schema and offsets to topic-scoped ordering and consumer groups; follow TDD flow by adding focused adapter/query behavior tests before implementing the PostgreSQL adapters.

## 2. Routers And Typed Domain Events

- [ ] 2.1 Add an appdispatch router factory that creates routers for explicit consumer groups and registers handlers on multiple topics; follow TDD flow by proving topic routing, independent groups, same-group behavior, and clean lifecycle before implementing the router.
- [ ] 2.2 Add bounded retry, panic recovery, and durable dead-letter middleware without terminating unrelated subscriptions; follow TDD flow by covering transient success, exhausted failure, poison-publish failure, preserved diagnostic metadata, and continued routing before wiring the middleware.
- [ ] 2.3 Add the app-level typed domain-event contract, publisher adapter, and generic typed handler decoding; follow TDD flow by covering valid publication, empty-topic rejection, serialization errors, typed dispatch, and malformed payloads before implementing the package.

## 3. Durable Jobs Adaptation

- [ ] 3.1 Move the execution envelope, topic, and consumer-group ownership into the jobs package and adapt enqueue to the generalized transaction-bound publisher; follow TDD flow by updating atomic enqueue, rollback, idempotency, and transport-isolation expectations before changing the service.
- [ ] 3.2 Migrate the jobs worker onto the named router while preserving business-failure job outcomes and bounded one-shot execution; follow TDD flow by updating worker lifecycle, success, failure, cancellation, and `RunOnce` cases before replacing the consumer loop.
- [ ] 3.3 Make duplicate delivery of terminal or otherwise non-queued jobs acknowledge without executing; follow TDD flow by adding queued-claim, running, succeeded, failed, and canceled delivery cases before tightening worker claim handling.
- [ ] 3.4 Update dependency injection, explicit migrations, and command/start-all composition for the shared transport and jobs-owned router; follow TDD flow by adjusting runtime resolution and process-mode tests before changing application wiring.

## 4. Documentation And Contract Alignment

- [ ] 4.1 Document the separate job-command and domain-event models, shared transport, at-least-once guarantees, dead-letter behavior, schema reset, and process ownership; follow TDD flow by updating documentation assertions where present before aligning `AGENTS.md`, architecture, terminology, module docs, and migration guidance.
