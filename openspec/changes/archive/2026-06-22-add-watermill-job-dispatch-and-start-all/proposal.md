## Why

Signal Foundry's current jobs path is split between queued job persistence, a polling worker loop, and a one-shot scheduler tick command. That is workable for conservative background processing, but it is the wrong operator experience for on-demand work such as manual bank sync, FX sync, CSV import confirmation, and future user-initiated workflows that should be dispatched immediately after the request is accepted.

The current local run shape is also awkward. PM2 starts only the API process, while background consumers and scheduler ticks require separate manual command paths. That makes the standard local backend workflow diverge from the actual product shape and increases the chance that due schedules or immediate workflows are not being processed during normal development.

We want to keep jobs as an observation layer for visible execution metadata while replacing polling-first execution with an app-owned pub/sub layer backed by Watermill. The bus should stay internal to `apps/signal-foundry/`, with a tiny abstraction seam so product modules do not depend on Watermill directly.

## What Changes

- Add an app-owned pub/sub dispatch layer in `apps/signal-foundry/` backed by a SQL-backed Watermill transport in the app database and hidden behind a small internal abstraction rather than exposing Watermill types to `finance/`, `runtime/`, or controller/module code.
- Make that abstraction concrete around one durable execution-dispatch topic, a versioned app-owned execution envelope, an internal publisher seam, and an internal typed handler registry so only app infrastructure translates to and from Watermill.
- Treat Watermill dispatch messages as the normal execution path for both immediate user-initiated workflows and tick-initiated scheduled workflows.
- Keep jobs as an observation metadata layer for status, history, progress, safe errors, audit context, and optional operator controls.
- Avoid forcing jobs metadata to be the default idempotency, locking, retry, or execution gating mechanism; workflows that need those semantics can opt into them explicitly.
- Change on-demand workflows to validate first, then durably publish immediate execution messages, creating or updating observation metadata in the same app transaction only when that workflow needs an immediate observable handle, rather than relying on queued-row polling as the normal execution path.
- Treat publish success as the acceptance boundary: requests return accepted only after durable publish succeeds; failed publish attempts must not leave behind accepted observation metadata, and consumer-side metadata failures must not blindly replay successful business work unless a workflow explicitly opts into that coupling.
- Keep scheduler execution tick-based, but unify the path so scheduler ticks advance a due window and durably publish the scheduled-work message together, rolling both back on failure, and never scan, execute, or retry immediately initiated workflows.
- Add a local `signal-foundry start-all` mode that starts the HTTP server, jobs consumer, and scheduler tick loop together for standard local development, using the same underlying components as dedicated commands.
- Define `start-all` scheduler behavior as one immediate tick followed by a non-overlapping fixed-delay loop on explicit scheduler-loop configuration, with coordinated shutdown and fail-fast handling for startup or component-exit failures.
- Preserve dedicated process commands for real environments, with API-only start and separate jobs/scheduler command paths remaining available.
- Update schema preparation, command docs, and ops guidance so `db-migrate` prepares any additional pub/sub transport state before `start-all`, `start`, jobs consumer, or scheduler execution begins.

## Capabilities

### New Capabilities

- `backend-process-modes`: Distinguish the local all-in-one backend start mode from dedicated API, consumer, and scheduler command modes.

### Modified Capabilities

- `durable-ingestion-jobs`: Replace polling-first job execution with Watermill-backed app dispatch while repositioning jobs as observation metadata rather than the forced execution/idempotency gate.
- `database-migration-command`: Ensure explicit migration/setup includes the app-owned pub/sub transport state needed by durable job dispatch and documents `start-all` as the standard local backend startup path.

## Impact

- Affects `apps/signal-foundry/` command wiring, startup composition, jobs execution flow, scheduler behavior, and schema setup/migration behavior.
- Introduces a new internal app abstraction around Watermill so app composition can later support non-job pub/sub processing without leaking Watermill into product modules, while pinning this change to one durable execution-dispatch topic and SQL-backed transport tables owned by the app.
- Preserves `finance/` and `runtime/` boundaries: both continue to interact through app-owned job or service abstractions instead of message-bus dependencies.
- Changes the documented local backend workflow from API-only startup toward `signal-foundry start-all`, while keeping dedicated commands for production-like environments and making `start-all` lifecycle semantics explicit.
- Requires focused tests and docs updates for dispatch boundary shape, publish/metadata/schedule failure semantics, scheduler tick boundaries, command behavior, startup composition, and migration/setup instructions.
