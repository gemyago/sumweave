## 1. Runtime flow boundary

- [x] 1.1 Define the paper backtest flow package surface and validation; must follow TDD flow (write test -> implement -> verify) by first adding tests for required run identity, instrument, timeframe, time range, moving-average parameters, governor policy, fixed positive quantity, and missing dependencies, then implementing the minimal `runtime/flows` request/result and constructor surface.

## 2. Slice orchestration

- [x] 2.1 Orchestrate replay data through analytics, strategy, and governor in deterministic order and wire one real-slice in-memory replay scenario through those stages; must follow TDD flow (write test -> implement -> verify) by first adding tests with instrumented dependencies plus the real existing replay/analytics/moving-average/governor services that assert the flow evaluates the strategy from the requested range, passes produced candidate actions unchanged into governor, preserves ordered decisions in the result, returns wrapped stage errors without calling downstream stages, and requires no backend, UI, storage, live-trading, or AI wiring.

## 3. Paper execution path

- [x] 3.1 Add approved-decision paper execution records using replay candle close prices and extend the real-slice in-memory replay scenario through local paper execution; must follow TDD flow (write test -> implement -> verify) by first adding tests that approved decisions create one deterministic command ID, order ID, client order ID, fill ID, flow-local reconciliation ID, command record, order record, fill record, and reconciliation record each, rejected or blocked decisions create no execution records, missing fill-price candles fail the run, and repeated runs return identical identifiers, records, and ordering.
