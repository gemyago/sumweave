## Why

The original product direction in Notion selected Hyperliquid as the first MVP venue candidate, while the current repository only implements Binance Spot as the first concrete venue-edge adapter. Adding Hyperliquid next resolves that drift without forcing an immediate rollback of the working Binance path, so the team can validate the intended venue while keeping current progress intact.

## What Changes

- Add a concrete Hyperliquid venue adapter alongside the existing Binance adapter instead of replacing Binance immediately.
- Extend venue-edge documentation and tests so the repository explicitly supports multiple concrete venue adapters during this transition period, with Hyperliquid mocked-HTTP fixtures derived from published Hyperliquid API docs/examples.
- Capture Hyperliquid as the preferred next real venue target while keeping Binance available as a reference adapter until the implementation follow-up note decides whether a separate removal change should be proposed.
- Keep live venue E2E, order placement, fills, reconciliation, and execution behavior out of scope unless a later change promotes them explicitly.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `venue-edge`: expand the existing venue-edge requirements so multiple concrete real-venue adapters can coexist, and so Hyperliquid can be added alongside Binance without changing the canonical data-layer boundary.

## Impact

- Affects `runtime/venueedge` by adding a Hyperliquid adapter and related fixtures/tests while preserving the current Binance adapter.
- May update `openspec/specs/venue-edge/spec.md` to describe coexistence of multiple real adapters and the transitional Binance-plus-Hyperliquid stance.
- Updates this change's planning artifacts so the Binance transition note has an explicit deliverable and completion condition.
- Does not add backend API routes, UI work, live trading, or execution-slice behavior in this change.
