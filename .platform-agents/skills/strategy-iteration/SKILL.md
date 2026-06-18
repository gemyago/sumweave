---
name: strategy-iteration
description: Iterate on a saved strategy by duplicating one baseline, changing one thing, and re-evaluating on the same verified range.
---
# Strategy iteration

Workflow:
1. Identify baseline with `sf_strategy_get_version` or `sf_strategy_list_versions`.
2. Read latest relevant evaluation with `sf_evaluation_list_backtests`, report, and evidence.
3. Duplicate baseline with `sf_strategy_duplicate_version`.
4. Change one thing at a time; explain hypothesis.
5. Validate with `sf_strategy_validate_definition`.
6. Save new immutable version with `sf_strategy_create_version`; notes include baseline, motivating run, and hypothesis.
7. Re-run over same verified range unless intentionally changed.
8. Compare old/new status, decision, trade count, drawdown, blocked/rejected counts, evidence counts.

Safety boundaries:
- Keep edits bounded to the Strategy DSL and persisted version workflow.
- No hidden overrides, no bypass of validation/save steps, and no live execution claims.
- If the prior evaluation lacks evidence, gather evidence before recommending changes.
