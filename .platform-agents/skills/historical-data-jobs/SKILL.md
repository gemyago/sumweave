---
name: historical-data-jobs
description: Run the bounded historical raw candle backfill workflow before evaluation when local candles are missing.
---
# Historical data jobs

Supported job: historical raw candle backfill only.

Supported backfill scope: `hyperliquid-perps`, assetClass `future`, timeframes `1m`, `5m`, `15m`, `1h`, `4h`, `1d`, UTC half-open range.

Required workflow:
1. `sf_data_list_candle_availability` first.
2. `sf_jobs_list` for queued/running operator+agent jobs before starting a duplicate.
3. Start with `sf_jobs_start_historical_data_backfill` only when needed.
4. Use deterministic idempotency key: `backfill:<venue>:<symbol>:<assetClass>:<timeframe>:<start>:<end>`.
5. Poll `sf_jobs_get` until `succeeded` or `failed`; poll until terminal.
6. Do not run evaluation while job is `queued` or `running`.
7. On success, verify `persistedCount`, `expectedCount`, `missingIntervalCount`, re-check local availability, optionally sample candles.
8. On failure, include job id, status, error code/summary/details, input scope, attempt count; do not proceed to evaluation.

Safety boundaries:
- Do not invent data, fills, or evaluation outcomes.
- Do not duplicate a matching queued/running job.
- Do not imply continuous ingestion exists.
- Do not proceed to evaluation until the job is terminal and local data is verified.
