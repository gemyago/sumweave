---
name: strategy-research-loop
description: Drive one bounded deterministic strategy evaluation loop using persisted product state.
---
# Strategy research loop

Goal: drive one bounded deterministic strategy evaluation loop using persisted product state.

Required tool order:
1. `sf_data_list_candle_availability`
2. `sf_data_get_candles` only for bounded samples/exact range checks
3. If missing data, read `historical-data-jobs` and do not evaluate yet
4. Read `strategy-dsl-v0` before definition work
5. Build one candidate definition
6. `sf_strategy_validate_definition`
7. `sf_strategy_create_version` only after clean validation and intended persistence
8. `sf_evaluation_run_backtest`
9. `sf_evaluation_get_backtest_report`
10. `sf_evaluation_get_backtest_evidence`

Strategy DSL example:

```json
{
  "kind": "moving-average-crossover",
  "instrument": {
    "venue": "hyperliquid-perps",
    "symbol": "BTC",
    "assetClass": "future",
    "active": true
  },
  "timeframe": "1h",
  "parameters": {
    "fastWindow": 3,
    "slowWindow": 8
  }
}
```

Final response requirements:
- strategy id/version/artifact
- data range/dataset
- run id/status/decision/metrics
- evidence counts
- interpretation
- next iteration

Failed-run requirements:
- run id/status
- failure reason/details
- whether local data/validation/policy/simulation caused it
- one safe next action

Safety boundaries:
- Use persisted product state and returned IDs only.
- Keep the loop bounded to one candidate, one verified range, and one evaluation at a time.
- Do not skip report or evidence reads before conclusions.
