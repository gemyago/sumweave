---
name: backtest-critique
description: Critique a backtest only after detail, report, and evidence are all loaded.
---
# Backtest critique

Always read all three before critique:
1. `sf_evaluation_get_backtest_detail`
2. `sf_evaluation_get_backtest_report`
3. `sf_evaluation_get_backtest_evidence`

Required identity fields: runId, status, strategyId, strategyVersion, artifactHash, instrument, timeframe, tested range, dataset id/replay checksum, policy hash.

Failure handling: if failed, report failureReason/failureDetails. If `replay-data-unavailable`, switch to historical-data-jobs.

Metrics checklist: tradeCount, maxDrawdown, blockedGovernorDecisionCount, rejectedGovernorDecisionCount. Missing means unknown, not zero.

Evidence checklist: traces, order intents, governor decisions, execution records, position snapshots, portfolio snapshots.

Truncation rule: if evidence is truncated, say so and request next page/offset before detailed claims.

Response template:

```text
Run <runId> is <status> for <strategyId>/<version> over <range>.
Decision: <decision or unknown>.
Metrics: trades=<tradeCount>, maxDrawdown=<maxDrawdown>, blocked=<blocked>, rejected=<rejected>.
Evidence: traces=<n>, intents=<n>, governor=<n>, execution=<n>, positions=<n>, portfolios=<n>.
Interpretation: <what the evidence supports>.
Concern: <main limitation or failure mode>.
Next iteration: <smallest safe change or no-change recommendation>.
```

Safety boundaries:
- Never present one run as a production guarantee.
- Never smooth over missing, truncated, or failed evidence.
