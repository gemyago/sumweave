## Context

Signal Foundry already has the right product boundary: the backend app owns process wiring, visibility, command modes, and cross-product execution registration, while `finance/` and `runtime/` stay focused on their own domain logic. The problem is not ownership, it is dispatch.

Today, normal execution depends on a polling worker model and a separate scheduler tick command:

```text
request -> persist queued job -> poll -> execute
schedule -> enqueue-due tick -> persist queued job -> poll -> execute
```

That shape is conservative, but it does not provide immediate operator feedback for user-initiated workflows and it makes the local backend run path incomplete unless developers remember to start extra commands manually.

The requested direction is to move to Watermill-backed pub/sub dispatch while keeping Watermill internal to the app and keeping scheduled work tick-driven. This change also repositions jobs as an observation layer, not the default execution, idempotency, or locking source of truth.

## Goals / Non-Goals

**Goals:**

- Provide immediate pub/sub-style dispatch for on-demand work.
- Keep jobs as an observation metadata layer for execution visibility.
- Hide Watermill behind a tiny app-owned abstraction so product modules do not import it.
- Route both immediate and scheduled work through Watermill dispatch.
- Add a local `start-all` command that starts the real backend shape in one process.
- Preserve dedicated command modes for real environments.

**Non-Goals:**

- No Watermill dependency in `finance/`, `runtime/`, or public runtime contracts.
- No requirement that every dispatched workflow create a fully durable job row before it can execute.
- No forced default idempotency, locking, retry, or cancellation semantics through the jobs table.
- No inline execution of background work during API requests or scheduler scans.
- No requirement that scheduled work become self-running in dedicated environments; scheduled execution remains tick-based.
- No direct copy of the `community-manager` PostgreSQL-specific Watermill transport layer into Signal Foundry.

## Decisions

1. Treat Watermill as dispatch and jobs as observation.

   Watermill messages are the normal execution signal. Jobs metadata exists to observe executions: status, timestamps, progress, results, safe errors, requester/correlation context, and operator links. Jobs metadata can also support workflow-specific cancellation, locking, idempotency, or retry controls, but those are opt-in semantics for workflows that need them rather than a universal substrate requirement.

2. Add a tiny app-owned dispatch abstraction and keep Watermill internal.

   `apps/signal-foundry/` should own a small internal pub/sub abstraction with naming and types shaped around app execution needs rather than around Watermill. Product modules continue to depend on job/service abstractions only. Watermill packages stay confined to app infrastructure and command/startup wiring.

   That abstraction should be deliberately small so future non-job pub/sub use cases can share it without forcing jobs-specific concepts into every topic.

   The boundary for this change is concrete:

   - one app-owned durable execution-dispatch topic for background work
   - one versioned execution envelope carrying dispatch kind, canonical payload JSON, optional observable job identifier, correlation/requester context, and optional schedule-window context
   - one internal publisher seam used by API services and scheduler code
   - one internal typed handler registry used by the dedicated consumer path

   The app abstraction may expose multiple Go types internally, but `finance/`, `runtime/`, controllers, and other product modules must only see app-owned service or job interfaces and app-shaped DTOs. They must not import Watermill packages, Watermill message types, or transport configuration directly.

3. Use one SQL-backed Watermill transport family in the app database.

   The transport choice for this change is a durable SQL-backed Watermill transport owned by `apps/signal-foundry/` and stored in the same application database family already used by the backend. This is not an in-memory broker and not a local-only shortcut.

   The implementation must work for the repo's SQLite-first local development path and remain compatible with SQL production deployment shapes. `db-migrate` prepares any required transport tables and indexes. Startup commands only open and use that transport state; they do not create it.

4. Dispatch first as the normal execution path, with observation metadata attached when useful.

   On-demand work should follow a pub/sub-like shape:

   ```text
   validate request
     -> optionally create/update initial observation metadata when the workflow needs an immediate visible handle
     -> durably publish execute message
     -> return accepted/observable execution metadata when appropriate
   ```

   Some workflows may need an observation record before the consumer starts so the UI has an immediate handle. Others may create or update metadata from the consumer only. The design should not force every execution to be identified, claimed, or idempotently gated by a job row.

5. Publish success is the acceptance boundary for immediate work.

   For API- or service-triggered background work, validation and canonicalization happen before durable writes. If a workflow wants an immediate observable job handle, that observation write and the durable dispatch publish must happen in the same app-owned database transaction. If publish fails, the transaction rolls back and the caller does not receive an accepted response.

   In other words:

   ```text
   invalid request -> return error, publish nothing, write no acceptance metadata
   valid request + publish commit succeeds -> return accepted
   valid request + publish commit fails -> return error and leave no accepted observation state behind
   ```

   Workflows that do not need an immediate observable row may publish without creating one first, but accepted completion still depends on durable publish success.

6. Consumer outcome and observation outcome are related but not identical.

   The consumer acknowledges or retries based on the workflow handler outcome, not on the mere presence or absence of generic observation metadata. Observable workflows should update running/succeeded/failed metadata when those records exist, but a metadata-only write failure after the business work already succeeded must not automatically cause a blind replay of non-idempotent business work.

   If a workflow requires stronger coupling between business completion and metadata persistence, it must declare explicit idempotency, replay, or terminal-state guards as part of that workflow's own contract.

7. Make idempotency and locking explicit workflow choices, not defaults.

   Watermill delivery semantics and handler behavior are sufficient for most workflows. If a specific workflow needs deduplication, cancellation, locking, exactly-once guards, or safe retry coordination, that workflow should declare and implement those semantics explicitly using the jobs observation metadata or another narrow app-owned mechanism. The generic dispatch layer must not pretend that every workflow needs or gets the same idempotency behavior.

8. Keep scheduler ticks narrow.

   Scheduler ticks only inspect schedule definitions and publish due scheduled-work messages. They must not scan jobs metadata to execute, retry, recover, or replay immediately initiated work. This keeps tick behavior predictable and prevents scheduled processing from becoming a hidden general-purpose job recovery loop.

9. Keep scheduled work tick-based, but make ticks publish dispatch messages.

   Scheduled work remains app-owned and tick-driven. `signal-foundry jobs enqueue-due` keeps scanning due schedules and advancing schedule metadata. The important change is what happens next:

   ```text
   due schedule found
     -> advance due-window metadata
     -> durably publish scheduled-work dispatch message
     -> record/update jobs observation metadata for visibility when appropriate
   ```

   This unifies scheduled and immediate work at the dispatch layer while preserving the distinction that only scheduler ticks initiate scheduled work.

10. Schedule advancement and scheduled publish must commit together.

   For scheduled work, advancing the due window and persisting the scheduled dispatch message must succeed or fail together in one app-owned transaction. A failed publish must leave the due window eligible for a later tick. A committed advancement must mean the matching scheduled dispatch was durably persisted exactly once for that due window.

   This keeps scheduler semantics clear:

   ```text
   due window claimed + publish commit succeeds -> next tick sees window as advanced
   due window claimed + publish commit fails -> next tick still sees window as due
   ```

11. Preserve dedicated commands, but add `start-all` as the standard local backend entrypoint.

   The command model should become:

   ```text
   signal-foundry start        # API only
   signal-foundry jobs worker  # dedicated jobs consumer mode
   signal-foundry jobs enqueue-due  # one scheduler tick
   signal-foundry start-all    # local API + consumer + scheduler loop
   ```

   `start-all` is the normal local workflow. Dedicated commands remain the production-like shape for split deployments, schedulers, or supervisors.

   To minimize churn, keep the external `jobs worker` command name even though its implementation becomes message-consumer based rather than poll-first.

12. `start-all` should run the same components as dedicated commands, not custom local-only logic.

   The all-in-one local mode should compose the same HTTP server, jobs consumer, and scheduler services used by the dedicated command paths. The scheduler portion of `start-all` should be a loop that periodically invokes the same scheduler tick behavior rather than a separate scheduling implementation.

13. `start-all` has explicit lifecycle and scheduler-loop semantics.

   `start-all` runs under one root context and starts three coordinated concerns: HTTP server, dedicated jobs consumer, and scheduler loop.

   The scheduler loop behavior is decided for this change:

   - it runs one scheduler tick immediately after startup succeeds
   - it then waits on one explicit scheduler-loop interval config owned by the app and runs the next tick after the previous tick has finished
   - it never overlaps ticks in the same process
   - per-tick failures are surfaced in logs/telemetry and the next interval still runs

   Component failure handling is also explicit:

   - if any component fails during startup, `start-all` exits non-zero and does not keep partial background behavior running
   - if the HTTP server or consumer exits unexpectedly after startup, `start-all` cancels sibling components and exits non-zero
   - normal signal/context shutdown cancels all three concerns and waits for coordinated termination

   `signal-foundry jobs enqueue-due` remains one tick only. It returns tick errors directly instead of looping.

14. The app transport implementation should fit both SQLite local and SQL production shapes.

   `community-manager` is a useful architectural reference, but its concrete Watermill SQL transport is PostgreSQL-specific. Signal Foundry must instead wire the chosen SQL-backed Watermill transport in a way that works with the repo's SQLite-first local development path and still leaves room for SQL production deployment. The transport choice remains an app detail behind the abstraction, but the transport family itself is now decided.

15. `db-migrate` remains the schema preparation gate.

    Any app-owned transport tables or offsets/state required by the chosen Watermill backing implementation must be prepared by `signal-foundry db-migrate`, not implicitly during `start`, `start-all`, jobs consumer startup, or scheduler startup.

## Risks / Trade-offs

- Watermill improves immediacy but weakens the old assumption that persisted job rows gate all execution. Workflows that need stronger semantics must opt into them explicitly.
- SQL-backed transport makes acceptance and scheduler semantics clearer, but it raises the bar on transaction wiring between business writes and durable publish persistence.
- SQLite and PostgreSQL transport details still differ. The abstraction should hide that, but the implementation and migration surface must explicitly account for it.
- Jobs metadata can become overloaded if treated as both observation and orchestration. The implementation should keep default behavior observational and add orchestration fields only for concrete workflow needs.
- `start-all` makes local development much better, but it also creates another long-running mode that needs strong tests and clear docs to avoid drifting from the dedicated command paths.
- Continuing the scheduler loop after a per-tick failure favors local availability over immediate fail-fast behavior; logs and telemetry need to make repeated tick failures obvious.

## Migration Plan

1. Introduce the app-owned execution-dispatch abstraction, SQL-backed transport wiring, and schema/setup path without exposing Watermill outside the app.
2. Convert immediate execution paths to durable publish-first acceptance with transactional observation writes where those workflows need an immediate handle.
3. Convert jobs execution mode from polling-first behavior to Watermill consumer behavior, with explicit workflow-owned observation, cancellation, locking, or idempotency semantics where required.
4. Update scheduler tick behavior so due schedules advance and publish scheduled-work dispatch messages atomically and never act as a recovery loop for immediate work.
5. Add `start-all` using the same underlying HTTP, consumer, and scheduler components as the dedicated commands, with the defined immediate-first-tick and non-overlapping loop behavior.
6. Update docs, AGENTS guidance, and local run instructions so `db-migrate` plus `start-all` becomes the standard local backend workflow.

## Open Questions

- No blocking design questions remain for this change.
