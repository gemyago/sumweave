## 1. Hyperliquid Venue Design Alignment

- [x] 1.1 Use `hyperliquid-perps` as the canonical Hyperliquid venue identity across the adapter, tests, and change artifacts.
- [x] 1.2 Confirm the v0 Hyperliquid fixture-family selections already captured in `design.md` remain the implementation baseline for instrument, candle, and trade reads, including the official published source each mocked-HTTP fixture family must follow.
- [x] 1.3 Verify the current `venueedge.MarketDataVenue` contract is sufficient for Hyperliquid without expanding into execution concerns.

## 2. Hyperliquid Adapter Implementation

- [x] 2.1 Add a concrete Hyperliquid market-data adapter beside the existing Binance adapter in `runtime/venueedge`.
- [x] 2.2 Map Hyperliquid instrument, candle, and trade payloads into canonical domain records with stable provenance and canonical venue identity.
- [x] 2.3 Keep Hyperliquid transport, payload, symbol, and pagination details isolated inside the adapter without changing the data-layer contract.

## 3. Test And Integration Coverage

- [x] 3.1 Add mocked-HTTP unit tests for Hyperliquid using fixtures reduced from the published Hyperliquid docs/examples for the selected v0 request types, covering success responses, paging behavior, non-success statuses, venue error payloads, and malformed payloads.
- [x] 3.2 Add mocked-HTTP-to-data integration coverage that ingests Hyperliquid adapter records through the existing venue ingestion flow and reads them back deterministically.
- [x] 3.3 Keep the existing Binance adapter coverage passing so the transition remains additive rather than destructive.

## 4. Documentation And Transition Review

- [x] 4.1 Add one post-implementation verification note to `design.md` stating whether the delivered adapter preserved the planned Binance-plus-Hyperliquid coexistence and official-doc-derived fixture assumptions without further OpenSpec deltas.
- [x] 4.2 Update `design.md` follow-up notes with one explicit post-implementation Binance disposition: keep Binance as reference coverage for now, or recommend a separate removal change.
- [x] 4.3 Run `make affected-lint-test` from the repository root before implementation completion and resolve any failures.
