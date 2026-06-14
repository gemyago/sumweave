## Why

The deterministic runtime slices now exist as on-demand primitives, but the repository has no thin product workflow that proves one paper backtest path can run across them. This change adds the smallest orchestration capability needed to run a repeatable data-to-execution paper backtest without widening into backend, UI, live trading, or persistence work.

## What Changes

- Add a thin runtime orchestration package for a single deterministic paper backtest flow.
- Orchestrate existing data replay, analytics, strategy, governor, and execution primitives in the documented deterministic order.
- Return an in-memory run result containing strategy output, governor decisions, and ordered local paper execution records for approved decisions.
- Keep the flow deterministic by requiring explicit run inputs, fixed quantity, deterministic command/order/fill/reconciliation identifiers and records, deterministic local client order IDs, and replay-derived fill prices.
- Exclude backend routes, UI screens, live venue submission, execution ledgers, new storage migrations, and AI calls.

## Capabilities

### New Capabilities
- `paper-backtest-flow`: Defines the thin deterministic runtime paper backtest orchestration capability across existing slices.

### Modified Capabilities
- None.

## Impact

- Affected code: `runtime/flows/` or an equivalently thin runtime orchestration area, plus focused tests in that package.
- Affected runtime slices: consumes existing `runtime/data`, `runtime/analytics`, `runtime/strategy`, `runtime/governor`, `runtime/execution`, and `runtime/domain` contracts without moving slice ownership.
- APIs: introduces a narrow Go runtime package surface only; no HTTP/OpenAPI changes.
- Dependencies and storage: no new external dependencies, no database migrations, and no persistent execution ledger.
