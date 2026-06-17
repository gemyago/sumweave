## Why

Historical candle ingestion already exists as a CLI/manual flow, and the strategy assistant can inspect data and run synchronous evaluations, but neither operators nor AI can start and monitor durable ingestion from the product surface. This change closes the missing-data loop with explicit, observable historical ingestion jobs while keeping evaluation and trading behavior unchanged.

## What Changes

- Add app-owned durable job orchestration for the initial `historical_raw_candle_backfill` job type, including persisted job state, requester/source metadata, worker attempt metadata, bounded errors, and idempotency-key handling.
- Add a conservative background worker that executes queued historical raw candle backfills through the existing `runtime/flows.HistoricalRawCandleBackfillRunner`, persists success/failure results, and keeps the jobs table as the source of truth.
- Add protected app HTTP endpoints: `POST /api/v1/jobs/historical-data-backfills`, `GET /api/v1/jobs`, and `GET /api/v1/jobs/{jobId}`.
- Add explicit strategy assistant tools: `sf_jobs_start_historical_data_backfill`, `sf_jobs_list`, and `sf_jobs_get`.
- Add AI skill/runbook guidance for the explicit flow: inspect availability, start job when needed, poll, re-check data, then run synchronous evaluation.
- Add a minimal UI Jobs workspace plus a Data-page “Start historical backfill” action/panel that links to the created job.
- Keep synchronous evaluations, manual CLI backfill, raw evidence semantics, real-money execution, continuous ingestion, and convenience “ensure candles” behavior unchanged/out of scope.

## Capabilities

### New Capabilities

- `durable-ingestion-jobs`: Durable app-owned job orchestration, worker execution, protected jobs API, and operator Jobs UI for historical raw candle backfills.

### Modified Capabilities

- `historical-data-backfill`: Clarify that the existing manual CLI backfill remains while the same deterministic runner can be invoked by durable app jobs.
- `ai-strategy-assistant-tools`: Add explicit job tools and workflow skills for historical ingestion orchestration before evaluation.
- `historical-data-browser`: Add an explicit Data-page entry point for starting historical backfill jobs while preserving read-only browsing by default.

## Impact

- Affects `apps/signal-foundry/internal/jobs` or equivalent app package, config wiring, lifecycle startup, GORM persistence against the existing app data-layer database, and generated app HTTP route glue from `internal/api/http/v1routes.yaml`.
- Affects existing app/runtime wiring used to construct the historical backfill runner, Hyperliquid public venue adapter, data services, strategy assistant tool registration, and bundled skills.
- Affects `apps/signal-ui` routes/nav, app API client code, Jobs list/detail pages, Data-page behavior, `ui-wireframe.md`, and tests.
- May add app-level dependencies only if justified by the worker design; no runtime public contract expansion or Watermill exposure from `runtime/` is planned.
- Requires persistence and integration tests for restart-visible jobs, worker success/failure, idempotency, HTTP mappings, tools, and UI states.
