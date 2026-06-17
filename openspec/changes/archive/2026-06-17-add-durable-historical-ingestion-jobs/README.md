# add-durable-historical-ingestion-jobs

Issue #22: durable historical raw candle backfill jobs with operator and AI orchestration

## Scope

- Add durable app-owned job orchestration for `historical_raw_candle_backfill`.
- Expose protected `/api/v1/jobs` create/list/detail endpoints.
- Add explicit strategy assistant job tools and workflow skill guidance.
- Add a minimal operator Jobs workspace plus Data-page backfill entry point.
- Keep synchronous evaluations unchanged; do not add real-money execution or continuous ingestion.

## Ordered Chunks

1. `jobs-foundation`: durable job store, service, worker lifecycle, historical backfill executor.
2. `jobs-http-api`: OpenAPI/routes/controllers for create/list/detail.
3. `jobs-agent-tools-skills`: strategy assistant job tools and workflow runbook skill.
4. `jobs-ui-workspace`: Jobs nav/list/detail and Data-page start action.
5. `jobs-integration-docs`: local product-flow smoke coverage and docs/status updates.
