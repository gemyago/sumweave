## Why

The thin paper backtest flow now proves deterministic slice composition, but it remains in-memory and too narrow for a trustworthy paper/backtest product path. The five requested tickets add the next minimal durable runtime capabilities: auditable strategy intents, stronger paper/backtest governor safety checks, a persistent paper execution ledger with deterministic limit fills, projected paper state from fills, and persisted backtest/evaluation evidence.

## What Changes

- Add durable `DecisionTrace` and `OrderIntent` audit records that link strategy context, requested intent, governor outcome, and execution references.
- Expand governor evaluation to consume explicit paper/backtest intent context, enforce mode/scope/notional/exposure/kill-switch checks, and emit stable reason codes.
- Add a persistent paper execution ledger for commands, orders, and fills plus a deterministic limit-order fill simulator.
- Add deterministic paper position and portfolio snapshot projection from fills.
- Add minimal durable dataset reference, `BacktestRun`, and `EvaluationReport` scaffolding.
- Update the existing thin paper/backtest flow only enough to preserve audit and ledger linkage across the deterministic path.
- Keep the scope runtime-first: no UI, no public HTTP/OpenAPI routes, no live venue order submission, no private venue APIs, no AI calls.

## Capabilities

### New Capabilities
- `audit-records`: Defines durable decision trace and order intent audit records for deterministic paper/backtest decisions.
- `backtest-evaluation`: Defines durable backtest run and evaluation report scaffolding.

### Modified Capabilities
- `governor-layer`: Adds intent-based paper/backtest safety checks beyond action kind, quality, and approval count.
- `execution-layer`: Adds paper ledger persistence, deterministic limit-fill simulation, and paper state snapshots.
- `paper-backtest-flow`: Links the thin flow to audit, intent, governor, execution ledger, and backtest/evaluation records without taking slice ownership.

## Impact

- Affected runtime areas: `runtime/domain`, new or existing runtime audit/backtest packages, `runtime/governor`, `runtime/execution`, and `runtime/flows`.
- Persistence: new runtime GORM-backed stores with SQLite test coverage; explicit columns and UTC timestamps remain required.
- App/API/UI: no required `apps/signal-foundry` wiring, HTTP/OpenAPI changes, scheduled jobs, or UI screens in this change.
- External dependencies: no live network dependency, wallet signing, private venue API calls, or AI/agent runtime calls.
