# Evaluation backtest flow v0

Evaluation backtest flow v0 is the deterministic paper backtest path over local candle replay for saved ready strategy versions.

## Inputs

- `strategyId`
- `strategyVersion`
- `start`
- `end`
- `quantity`
- optional `governorPolicyHash`
- optional `note`

## Preconditions

- The strategy version is ready.
- The artifact resolves.
- Quantity is positive.
- The UTC range is valid.
- Replay candles exist locally.

## Missing-data behavior

If replay candles are unavailable, the system persists a failed run with `replay-data-unavailable`.

## Default governor policy payload

```json
{"mode":"paper","allowedActionKinds":["long","short"],"minimumQuality":"raw","maximumApprovedCount":50}
```

## Execution model

1. Strategy actions are produced from replayed local candles.
2. Audit traces and order intents are recorded.
3. Governor policy evaluates intents and produces approved decisions.
4. Local paper execution records are persisted.
5. Fills use replay candle close price with zero fee/slippage assumptions.
6. Simulator is `closed-candle-limit-v0`.

## Outputs

- detail
- report
- evidence
- metrics
- dataset reference / replay checksum
- policy reference

## Agent critique rule

Read report and evidence before conclusions.
