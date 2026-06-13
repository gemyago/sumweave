## 1. Shared Strategy Domain

- [x] 1.1 Add failing `runtime/domain` tests for strategy identity and candidate action time/input-range canonicalization, then implement only the minimal shared strategy identity and candidate action domain types and constructors those tests require; do not add evaluation request types or evaluation-only parameter wrappers to `runtime/domain`.

## 2. Strategy Service Foundation

- [x] 2.1 Add failing `runtime/strategy` tests for service construction and request validation, then implement the strategy service, consumer-defined analytics dependency interface, evaluation request types, and evaluation-only moving-average parameter wrapper for deterministic strategy evaluation.
- [x] 2.2 Add failing tests ensuring the strategy service requests fast and slow moving-average analytics with the exact instrument, timeframe, and `[start, end)` range, then implement analytics request orchestration without hidden pre-range lookback, strategy persistence, HTTP routes, or backend DI wiring.

## 3. Moving-Average Crossover Evaluation

- [x] 3.1 Add failing tests for first-aligned-point baseline behavior, bullish crossover emission, bearish crossover emission, and no-crossover cases, then implement aligned moving-average comparison with stable ascending action ordering.
- [x] 3.2 Add failing tests for decision-time semantics, combined input-range construction, and quality propagation across the previous/current aligned analytics points used by a crossover, then implement candidate action assembly and propagated quality rules while keeping the change scoped to touched runtime files only.
