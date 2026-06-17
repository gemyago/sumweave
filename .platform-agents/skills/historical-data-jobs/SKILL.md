---
name: historical-data-jobs
description: Run bounded historical data backfill jobs before synchronous evaluation when local candles are missing.
---
# Historical data jobs

1. Check current local coverage first with `sf_data_list_candle_availability` and bounded candle/evidence reads.
2. Use `sf_jobs_list` to inspect duplicate queued/running jobs before starting anything new.
3. Start `sf_jobs_start_historical_data_backfill` only if the needed range is still missing, and prefer bounded incremental ranges over large catch-up requests.
4. Poll until terminal with `sf_jobs_get`; do not run evaluation while the job is queued or running.
5. After success, re-check local data availability for the exact scope, then run synchronous evaluation.
6. After failure, summarize the bounded failure honestly and only retry with a narrower or corrected request if needed.

Safety boundaries:
- Do not invent data, fills, or evaluation outcomes.
- Do not start repeated duplicates when a matching queued/running job already exists.
- Prefer bounded incremental ranges instead of repeated full-history backfills.
- Continuous ingestion is unavailable; jobs only cover explicit historical backfill requests.
- No live trading, order placement, wallet actions, or real-money execution.
