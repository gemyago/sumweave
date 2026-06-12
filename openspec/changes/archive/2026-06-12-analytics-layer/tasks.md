## 1. Domain Contract

- [ ] 1.1 Add minimal analytics domain types in `runtime/domain` for indicator kind, parameters, series identity, series, points, point time, and value range.
- [ ] 1.2 Add validation/canonicalization helpers for analytics domain values without persistence or API metadata.
- [ ] 1.3 Add focused `runtime/domain` tests for analytics identity, parameter validation, UTC point/value-range canonicalization, point ordering assumptions, and quality values.

## 2. Analytics Service

- [ ] 2.1 Create `runtime/analytics` with a service constructor that accepts a consumer-defined candle replay reader interface and returns a concrete service.
- [ ] 2.2 Add request/params types for candle analytics calculations, including instrument, timeframe, time range, indicator kind, and indicator parameters.
- [ ] 2.3 Implement request validation for required instrument identity, timeframe, time range, supported indicator kind, and positive window/lookback sizes.
- [ ] 2.4 Implement deterministic candle replay loading through the data-layer replay semantics without venue, AI, or network dependencies.
- [ ] 2.5 Implement point semantics so each output point uses the current candle end time, exposes the contributing half-open value range, and is ordered by point time then source replay identity.
- [ ] 2.6 Implement moving average over close prices, omitting warmup points until enough replayed candles exist.
- [ ] 2.7 Implement period return over close prices, omitting warmup points and failing the whole request without partial output when an otherwise-computable lookback close is zero or negative.
- [ ] 2.8 Implement analytics quality propagation so suspect inputs produce suspect outputs, raw inputs produce raw outputs, and all-validated inputs produce validated outputs.

## 3. Behavior Tests

- [ ] 3.1 Add analytics service tests with in-memory fake replay readers and randomized canonical candle inputs.
- [ ] 3.2 Verify repeated calculations return stable series identity, ordered points, UTC point timestamps, value ranges, values, and quality states.
- [ ] 3.3 Verify half-open range inputs are passed to the replay reader and no hidden lookback read occurs outside the requested range.
- [ ] 3.4 Verify moving average and period return warmup behavior omits only not-yet-computable points.
- [ ] 3.5 Verify invalid indicator parameters, unsupported indicator kinds, and zero/negative denominator return calculations fail without partial series output.
- [ ] 3.6 Verify the initial implementation requires no analytics persistence, migrations, live venue access, AI model calls, backend DI wiring, or new HTTP routes.

## 4. Deferred Backend Wiring Guardrail

- [ ] 4.1 Do not register analytics in `apps/signal-foundry/internal` unless the implementation introduces a current backend consumer.
- [ ] 4.2 If a current backend consumer is introduced, register the analytics service after the runtime behavior tests pass, keep wiring thin around the existing data-layer read service, expose no new HTTP routes, and add focused DI resolution tests.

## 5. Validation

- [ ] 5.1 Run formatting for touched Go files.
- [ ] 5.2 Run `make affected-lint-test` from the repository root and resolve failures.
- [ ] 5.3 Verify AGENTS.md updates are not needed because the change adds no new commands, workflows, or architecture rules.
