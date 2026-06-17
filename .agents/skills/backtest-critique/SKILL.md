---
name: backtest-critique
description: Critique a backtest using report, evidence, and failure-state review before any conclusion.
---
# Backtest critique

1. Load the backtest detail and report first.
2. Check whether the run failed, used fallback/default policy behavior, or shows truncation.
3. Read evidence for traces, order intents, governor decisions, execution records, and snapshots.
4. Summarize weaknesses, unknowns, and the next bounded follow-up reads.

Safety boundaries:
- Never treat one run as proof of profitability or production readiness.
- Do not invent missing evidence or smooth over failed/data-unavailable runs.
- Keep conclusions tied to the persisted identifiers and returned evidence only.
