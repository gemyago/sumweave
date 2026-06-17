## Context

The repository direction treats `runtime/` as the deterministic core, `apps/signal-foundry/` as the Go API/jobs app, and `apps/signal-ui/` as the operator UI. Current code already has `runtime/flows.HistoricalRawCandleBackfillRunner`, data lineage/raw evidence persistence, a CLI command for manual raw candle backfill, protected data browser APIs, strategy/evaluation APIs, strategy assistant tools, profile/skills support, and UI Data/Strategies/Evaluations pages.

The gap is app-level durable orchestration and visibility: an operator or assistant can discover missing candles but cannot start, list, inspect, or monitor the historical ingestion needed before synchronous evaluation.

## Goals / Non-Goals

**Goals:**

- Persist durable historical ingestion job state and worker metadata for `historical_raw_candle_backfill`.
- Let authenticated operators and the strategy assistant explicitly create, list, and inspect jobs.
- Execute queued jobs through the existing runtime backfill runner and preserve ingestion-run/raw-evidence behavior.
- Provide minimal UI visibility and a Data-page entry point without turning the data browser into implicit ingestion.
- Provide AI skills/runbooks that teach the explicit missing-data workflow.

**Non-Goals:**

- No `sf_data_ensure_candles` convenience tool, implicit repair/backfill, scheduler, continuous ingestion, or streaming job progress channel.
- No async evaluation jobs; existing synchronous evaluation APIs/tools remain unchanged.
- No real-money execution, manual order placement, private venue endpoints, wallet/signing, or autonomous promotion.
- No runtime public contract expansion for Watermill, job tables, or app orchestration internals.
- No cancellation, manual retry endpoint, scheduler, or general retry policy; the only automatic retry behavior in scope is bounded stale-running recovery after worker interruption.

## Decisions

1. Keep job orchestration app-owned.

   Implement the orchestration package under `apps/signal-foundry/internal/jobs` or a similarly explicit app-internal path. It owns job models, store, service, historical backfill executor, worker, lifecycle registration, and HTTP/tool DTO mapping. `runtime/flows` remains the deterministic backfill owner; it should not know about app job tables or worker transport.

2. Use a jobs table as the source of truth.

   Persist job records with explicit columns for id, job type, status, requester/source metadata, agent session/run identifiers, idempotency key, input hash, input/result JSON, bounded error fields, lifecycle timestamps, worker id, attempt count, last attempt time, and correlation id. Store GORM models separately from domain/DTO structs and use explicit column names. Create/list/get/read-after-restart behavior must query this store, not pub-sub state.

3. Use a conservative DB-backed worker for v0, with Watermill deferred unless proven lightweight in implementation.

   The issue suggested Watermill SQL pub-sub and community-manager patterns. An unauthenticated fetch of `gemyago/community-manager` returned 404 in this environment, and this repo has no Watermill dependency today. For this slice, prefer a simple durable DB-backed worker: create persists a queued job and signals a local worker loop; worker also polls queued jobs on start/interval so process restarts recover work. This avoids adding a bus whose offsets could be confused with source-of-truth state. If implementation proves Watermill SQL is already easy with SQLite/GORM, it may be used only as a wake-up mechanism; the jobs table still owns visibility and correctness.

4. Reuse the existing app data-layer database configuration.

   Existing strategy/evaluation app stores use `config.dataLayer.database.dsn`, `tablePrefix`, and `autoMigrate` with sub-prefixes. Job persistence should follow that convention, for example with a `jobs_` sub-prefix under the data-layer prefix, plus dedicated jobs config for worker enablement/concurrency and historical backfill interval limits. Avoid adding a second app database unless a later need appears.

5. Create returns immediately and worker transitions idempotently.

   Create validates/canonicalizes input, enforces venue `hyperliquid-perps`, asset class `future`, supported timeframe, UTC half-open range, no future end, interval cap, and page-size bounds; generates job id and ingestion run id; persists queued; signals the worker; returns the queued detail. Worker loads by id, claims queued jobs into running safely, increments attempt count on claim, sets worker id and last attempt time, skips terminal jobs on duplicate messages/polls, runs the existing backfill runner, persists succeeded/failed terminal state before acknowledging/continuing, and records bounded safe error summary/details.

6. Keep idempotency narrow and conflict on key/input mismatch.

   If an idempotency key is provided, the same requester/source + same job type + same key + same canonical input hash returns the existing job, regardless of whether it is queued, running, succeeded, or failed. If the same requester/source + same job type reuses a non-empty key with a different canonical input hash, creation must fail with a safe conflict (`idempotency_key_conflict`, HTTP 409 for the API/tool conflict result) and must not create, requeue, or mutate either job. Without a key, create a new job. Do not deduplicate by instrument/range because repeated ingestion runs can be useful raw evidence.

7. Use protected app HTTP APIs with camelCase JSON.

   Add app-owned OpenAPI route definitions under `/api/v1/jobs`. All JSON fields use camelCase. Create/list/detail endpoints require authentication, derive operator identity from the existing auth middleware, and map controller errors into safe 4xx/5xx payloads. List supports status, job type, source, limit, and cursor at minimum; detail includes input, requester metadata, timestamps, worker/attempt metadata, result, missing interval preview, raw payload count, and bounded error fields.

8. Add explicit AI tools over the job service.

   Extend the strategy assistant tool registration with `sf_jobs_start_historical_data_backfill`, `sf_jobs_list`, and `sf_jobs_get`. Tool handlers call the job service directly, not HTTP loopback or raw SQL. The start tool must mark source `agent`, include agent session/run metadata when available from tool context, return job id/status, and instruct the assistant to poll `sf_jobs_get` before running evaluation.

9. Keep UI minimal and route-based.

   Add `#/jobs` and `#/jobs/:jobId` protected routes plus `Jobs` nav. The list is a stacked/summary-first workspace with filters and refresh/open actions. Detail uses a separate route, not a dense split pane. Data page adds an explicit “Start historical backfill” action/panel using the current data scope, submits the job create endpoint, and shows a link to the created job. The normal browse/select/load paths remain read-only and must not implicitly mutate data.

10. Document the operator/AI workflow.

    Add or update skills under `.agents/skills` so the assistant follows: check availability, avoid duplicate queued/running jobs, start bounded backfill only if needed, poll until terminal, re-check data, then run synchronous evaluation. Skills must state not to invent data, not to start repeated duplicates, prefer bounded incremental ranges, and continuous ingestion is unavailable.

11. Define stale-running recovery as bounded startup repair.

    A `running` job is stale when it was left by a previous app worker process and can no longer complete in memory. For the v0 single-process worker, startup treats every persisted `running` job observed before the worker begins polling as stale. During normal operation, the worker must not reclaim its own active in-memory execution merely because a timestamp is old; an optional configured stale timeout may only be used for jobs not owned by the current process/worker lease. Startup recovery runs before queued-job polling. If a stale job's `attempt_count` is below configured `max_attempts` (default 3, minimum 1), recovery requeues it: status becomes `queued`, `updated_at` and `queued_at` move to the recovery time, `worker_id` and `started_at` are cleared, `completed_at` remains null, `attempt_count` and `last_attempt_time` are preserved, and a bounded safe recovery note/code such as `stale_running_requeued` is recorded for audit until the next terminal result replaces it. The next claim increments `attempt_count`. If `attempt_count >= max_attempts`, recovery marks the job `failed`, sets `updated_at`/`completed_at`, preserves the last attempt metadata, records bounded safe error code/details such as `stale_running_attempts_exhausted`, and does not run the backfill again. Worker-startup recovery must be idempotent.

## Risks / Trade-offs

- SQLite writer contention while a backfill persists candles and job state → keep default historical backfill concurrency at 1 and make worker transitions short transactions.
- Simple DB polling is less immediate than pub-sub → acceptable for v0; create can also signal an in-process wake channel while polling covers restarts.
- Process crash after running starts may leave `running` jobs non-terminal → startup recovery requeues stale jobs below the attempt cap and fails jobs at/above the cap with bounded details; this can repeat ingestion calls after a crash, so runner and persistence idempotency remain important.
- Large historical ranges can overload venue/API/UI → enforce max interval config and compact missing interval previews.
- Agent duplicate starts could create noisy reruns → idempotency key plus list-before-start skill guidance mitigates without blocking deliberate reruns.
- Community-manager Watermill reference was not accessible unauthenticated during planning → not a blocker; design documents the DB-backed tradeoff and keeps Watermill optional as a wake-only implementation detail.

## Migration Plan

1. Add the jobs store schema and auto-migration behind existing data-layer auto-migrate conventions.
2. Add worker configuration with conservative defaults: enabled, max concurrent historical backfills 1, max intervals 10000.
3. On process startup, start the worker only when enabled; run stale-running recovery before polling queued jobs, requeueing below the attempt cap and failing at/above the cap.
4. Rollback can disable the worker and leave job rows for visibility; existing CLI backfill, data reads, and synchronous evaluations remain usable.

## Open Questions

- Whether to add cancellation in this slice is deferred unless implementation finds a safe low-cost path; acceptance does not require it.
