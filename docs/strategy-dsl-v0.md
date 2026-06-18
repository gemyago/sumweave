# Strategy DSL v0

Strategy DSL v0 is the persisted immutable strategy definition format for Signal Foundry evaluation work.

## Supported kind

Only `moving-average-crossover` is supported in v0.

## JSON example

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

## Validation rules

- No unknown fields.
- `instrument` is required.
- `instrument.assetClass` must be valid.
- `timeframe` must be one of `1m`, `5m`, `15m`, `1h`, `4h`, `1d`.
- `fastWindow` and `slowWindow` must be positive integers.
- `fastWindow` must be less than `slowWindow`.

## Canonical artifact behavior

- `schemaVersion=strategy-artifact.v0`
- `artifactKind=strategy`
- `hash` is SHA-256 of canonical JSON

## Algorithm semantics

1. Compute moving average series for `fastWindow`.
2. Compute moving average series for `slowWindow`.
3. Align fast and slow analytics points by timestamp.
4. Emit no action when fewer than two aligned points exist.
5. Emit long when previous fast <= previous slow and current fast > current slow.
6. Emit short when previous fast >= previous slow and current fast < current slow.
7. Emit no action when no crossover occurs.
8. Candidate action decision time is the current aligned fast point timestamp.
9. Candidate action quality is propagated from aligned analytics points: suspect dominates raw, raw dominates validated.

## Operator and agent workflow

1. Check candle availability.
2. Validate the candidate definition.
3. Save a ready immutable strategy version.
4. Run evaluation for the saved version.
5. Critique the report and evidence before conclusions.
