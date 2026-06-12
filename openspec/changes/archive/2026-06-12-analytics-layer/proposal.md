## Why

Signal Foundry has a canonical data layer, but the deterministic path still lacks the analytics slice that turns replayable market data into reusable strategy inputs. Adding the analytics layer now gives strategy work a stable boundary without putting AI, venue mechanics, or ad hoc calculations into the critical execution path.

## What Changes

- Introduce a new deterministic `analytics-layer` capability between `data-layer` and future `strategy` work.
- Add shared domain concepts for analytics outputs that can be consumed across deterministic runtime slices.
- Add a runtime analytics service that reads canonical candle replay data and computes indicator series with stable ordering, candle-end point timestamps, and explicit value-range semantics.
- Start with candle-derived indicators suitable for strategy inputs, including moving average and return-style calculations.
- Make invalid period-return denominators fail the request deterministically instead of returning partial output.
- Keep analytics computation independent from AI-assisted research, live venue calls, and vendor payloads.
- Do not require analytics persistence in the initial slice; results are computed deterministically from canonical data reads.

## Capabilities

### New Capabilities

- `analytics-layer`: Deterministic analytics outputs derived from canonical market data for downstream strategy consumption.

### Modified Capabilities

- None.

## Impact

- Affected code: `runtime/domain`, a new `runtime/analytics` package, and focused runtime tests.
- APIs: no external HTTP API changes are required for the initial layer.
- Dependencies: no new third-party service, AI model, or live venue dependency is required.
- Systems: analytics will consume existing data-layer read/replay contracts and preserve the deterministic product path `Data -> Analytics -> Strategy -> Governor -> Execution`.
- Deferred: `apps/signal-foundry` wiring is not part of the initial slice unless implementation work uncovers a current backend consumer that genuinely needs it.
