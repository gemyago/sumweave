---
name: strategy-dsl-v0
description: Define, validate, and explain the supported Strategy DSL v0 shape and crossover semantics.
---
# Strategy DSL v0

Use this skill before creating, editing, validating, saving, or explaining a strategy definition.

Supported strategy kind: moving-average-crossover only.

Strategy shape:

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

Fields:
- kind must be moving-average-crossover.
- instrument.venue is required; for current evaluation work prefer hyperliquid-perps.
- instrument.symbol is required.
- instrument.assetClass must be valid; for Hyperliquid perps use future.
- instrument.active is boolean; use true for normal evaluation candidates.
- timeframe is one of 1m, 5m, 15m, 1h, 4h, 1d.
- fastWindow and slowWindow are positive integers.
- fastWindow must be less than slowWindow.

Algorithm semantics:
1. Compute moving average series for fastWindow.
2. Compute moving average series for slowWindow.
3. Align fast and slow analytics points by timestamp.
4. Emit no action when fewer than two aligned points exist.
5. Emit long when previous fast <= previous slow and current fast > current slow.
6. Emit short when previous fast >= previous slow and current fast < current slow.
7. Emit no action when no crossover occurs.
8. Candidate action decision time is the current aligned fast point timestamp.
9. Candidate action quality is propagated from aligned analytics points: suspect dominates raw, raw dominates validated.

Always call sf_strategy_validate_definition before saving. Use sf_strategy_create_version only after clean validation and intended persistence.

Safety boundaries:
- Stay within the supported v0 schema and semantics only.
- Validate before save, and save before evaluation.
