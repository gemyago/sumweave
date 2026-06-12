## Why

Signal Foundry has a canonical Data slice, but it still lacks a venue-shaped edge that can produce realistic market data without coupling data persistence to exchange-specific mechanics. A small venue edge v0 lets the system prove ingestion behavior with deterministic sandbox data first, then validate a real venue adapter against documented HTTP shapes before any live venue E2E work.

## What Changes

- Introduce a narrow venue edge capability for market-data oriented venue integration.
- Add a sandbox venue that produces seeded, deterministic synthetic instruments, candles, and trades with stable provenance.
- Add automated sandbox-to-data integration coverage using the existing data-layer ingestion and persistence contracts.
- Add the first real venue adapter against a documented market-data HTTP API shape, tested with Go's standard HTTP test server or equivalent local test HTTP server.
- Explicitly keep live real-venue E2E tests, order placement, fills, reconciliation, UI, and production ingestion jobs out of scope for v0.

## Capabilities

### New Capabilities

- `venue-edge`: Isolated market-data venue edge behavior, including deterministic sandbox generation, canonical record production, data-layer ingestion integration, and mocked-HTTP validation for a real venue adapter.

### Modified Capabilities

None.

## Impact

- Affects `runtime/` by adding a venue edge package or equivalent runtime area for market-data source behavior.
- Uses existing `runtime/domain` records and `runtime/data` ingestion/read services without changing data-layer requirements.
- May add runtime integration tests that use SQLite-backed data-layer storage and deterministic sandbox venue output.
- May add real-venue adapter tests backed by local HTTP test servers and documented response fixtures.
- Does not add backend API routes, operator UI, live network E2E, order placement, fills, or reconciliation in this change.
