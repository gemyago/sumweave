# Planning Review

## Round 1

- Scope: add-durable-historical-ingestion-jobs
- Trigger: initial planning artifacts for GitHub issue 22
- Strengths: scope is bounded to historical raw candle backfill jobs; explicit operator/API/AI/UI paths are covered; non-goals exclude convenience ensure-candles, async evaluations, continuous ingestion, and real-money execution.
- Issues: none blocking for planning review; stale-running recovery remains an implementation decision documented in design/status.
- Verdict: revision requested by review feedback

## Round 2

- Scope: review-feedback revision for GitHub issue 22 planning artifacts only
- Changes: specified stale-running startup recovery, attempt-cap behavior, and metadata updates; specified idempotency-key/input-hash mismatch conflict semantics; updated tasks and manager status to match the revised plan.
- Verification: specs, design, tasks, and status artifacts are internally aligned; ordered chunk plan is preserved; implementation remains not started.
- Verdict: ready for OpenSpec review
