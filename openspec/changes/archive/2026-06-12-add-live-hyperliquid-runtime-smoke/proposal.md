## Why

The current runtime foundation has strong offline coverage for venue ingestion through the sandbox venue and mocked-HTTP real adapters, but it still lacks a human-triggered proof that the real Hyperliquid public API, the venue-edge mapper, the ingestion flow, and the SQLite-backed data layer all work together in one path.

Read-only live data is the immediate priority. Hyperliquid public market-data reads do not require wallet approval or funded accounts, so the team can gain real integration confidence now without introducing signing, `approveAgent`, testnet provisioning, or live trading scope.

## What Changes

- Add one opt-in manual `live` smoke path under `runtime/` for Hyperliquid public market data.
- Keep the smoke path read-only and scoped to the existing market-data contract:
  - instruments
  - candles
  - recent trades
- Run the live path through the real `runtime/venueedge` Hyperliquid adapter, the existing venue ingestion flow, an ephemeral SQLite store, and the canonical data-layer read services.
- Add a non-network compile-only check for `live`-tagged tests so the live lane cannot silently stop compiling while remaining excluded from normal execution.
- Keep the smoke path excluded from regular `make test`, `make affected-lint-test`, and CI by default.
- Add dedicated targets for:
  - compile-only validation of the `live` lane as a regular check
  - intentional manual execution of the `live` lane

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `venue-edge`: add an opt-in live public read smoke capability that stays outside the default automated suite.
- `data-layer`: add an opt-in live ingestion smoke that proves canonical persistence and readback through SQLite using real public venue data.

## Impact

- Affects `runtime/venueedge` test structure and runtime module test targets.
- Likely affects repo-level make targets so `live` tests have both a compile-only regular check and a manual execution entrypoint.
- Extends OpenSpec coverage for `venue-edge` and `data-layer`.
- Does not require wallets, funded addresses, `approveAgent`, account-state reads, or order placement.
- Does not add CI coverage or make live network access part of normal completion gates.
