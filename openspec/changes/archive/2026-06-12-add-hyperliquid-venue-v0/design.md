## Context

`runtime/venueedge` currently proves the venue-edge shape with a deterministic sandbox venue and one concrete mocked-HTTP adapter for Binance Spot market data. That implementation validated the boundary, but it drifted from the earlier product direction captured in Notion, which selected Hyperliquid perps as the first MVP venue candidate and Binance as a reference venue rather than the default target.

The current repository architecture still helps here. The `venueedge.MarketDataVenue` contract is narrow and canonical, so another adapter can be added without changing the data-layer contract. The main design constraint is therefore not how to support many venues in the abstract, but how to add Hyperliquid quickly without turning `runtime/venueedge` into a generic framework or prematurely dragging execution/perps concerns into the current market-data-only slice.

## Goals / Non-Goals

**Goals:**

- Add Hyperliquid as a second concrete real venue adapter in `runtime/venueedge`.
- Keep Binance available during the transition so the team can compare friction and delivery speed with a working reference adapter still in tree.
- Preserve the current canonical venue-edge contract and data-layer boundary.
- Keep the Hyperliquid addition mocked-HTTP-testable and deterministic by default, with fixtures reduced from published Hyperliquid API docs/examples.
- Make a later Binance removal optional and separable from the Hyperliquid addition.

**Non-Goals:**

- No immediate removal of Binance.
- No live venue E2E or credentials in default tests.
- No order placement, fills, funding accounting, reconciliation, wallet signing, or execution-slice behavior in this change.
- No broad venue registry, plugin system, or multi-venue framework beyond the minimum needed to host two concrete adapters.
- No attempt to fully model Hyperliquid execution semantics in the market-data venue edge.

## Decisions

### Decision: Add Hyperliquid alongside Binance instead of replacing Binance now

This change should add a Hyperliquid adapter while leaving the existing Binance adapter intact.

Rationale: Binance already proves the mocked-HTTP adapter path and gives the team a known-good comparison point. Keeping it during the transition reduces rework risk and lets the team decide later, based on actual implementation drag, whether Binance should remain a reference adapter or be removed.

Alternative considered: replace Binance immediately. That would align the codebase to Notion faster, but it would also collapse a useful control sample and couple course-correction with deletion in the same change.

### Decision: Keep the current market-data-only venue-edge boundary

Hyperliquid should be added only at the existing market-data boundary: instruments, candles, trades, provenance, range handling, and pagination-like behavior where applicable.

Rationale: the promoted `venue-edge` spec and current `runtime/venueedge` package are explicitly market-data oriented. Hyperliquid's broader significance in product planning is execution and perps, but forcing those concerns into this change would conflate two slices and make the design harder to validate.

Alternative considered: expand the venue edge now to include funding, mark price, or private account state because Hyperliquid is a perps venue. That would better reflect the long-term execution target, but it would also change the contract before the current market-data shape has finished proving itself.

### Decision: Use `hyperliquid-perps` as the canonical venue identity

The adapter, tests, and related docs should use `hyperliquid-perps` as the canonical venue identity rather than a generic `hyperliquid` label.

Rationale: the existing Binance adapter already encodes market type in the venue name (`binance-spot`). Hyperliquid planning in Notion is specifically perps-first, so the venue identity should stay equally precise.

Alternative considered: use a generic `hyperliquid` venue name. That is shorter, but it would increase ambiguity if additional Hyperliquid market surfaces appear later.

### Decision: Reuse the existing adapter pattern instead of introducing a registry

The implementation should follow the same concrete-adapter style as Binance: injected HTTP client, configurable base URL, local HTTP test coverage, and direct mapping into canonical records.

Rationale: two adapters do not justify a registry or discovery framework. The current architecture already favors narrow interfaces and isolated vendor handling over early frameworking.

Alternative considered: add a venue registry or adapter factory now that there will be more than one real adapter. That may become useful later, but it is unnecessary complexity at two adapters.

### Decision: Hyperliquid remains mocked-HTTP-first in this change

Tests should use published Hyperliquid API docs/examples for the exact v0 request types under test through local HTTP test servers and fixtures. Default tests must remain offline. Captured live payloads may help local debugging, but they are not acceptable as the normative fixture source for committed tests.

Rationale: this preserves the determinism and low-flake stance already established for the venue edge while keeping fixture provenance aligned with the venue-edge spec. It also keeps progress focused on mapping and contract validation rather than credentials, connectivity, or venue availability.

Alternative considered: add opt-in live smoke tests immediately because Hyperliquid is the intended product venue. That might become useful later, but it is a separate readiness concern and should not gate the market-data adapter addition.

Implementation note: this planning change already fixes the v0 fixture families at the request-type level. Implementation should confirm committed fixtures stay derived from these published sources rather than reopening pre-implementation OpenSpec planning edits. The minimum expected coverage baseline is:

- instruments from Hyperliquid Docs `Info endpoint / Perpetuals`, specifically `Retrieve perpetuals metadata (universe and margin tables)` on `POST /info` with `type: "meta"`
- candles from Hyperliquid Docs `Info endpoint`, specifically `Candle snapshot` on `POST /info` with `type: "candleSnapshot"`
- trades from `POST /info` `type: "recentTrades"` as the v0 public trade-read baseline, with committed fixtures reduced from the official published Hyperliquid public trade field shape and request naming rather than captured live traffic

Published source references for those fixture families:

- instruments: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/info-endpoint/perpetuals>
- candles: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/info-endpoint>
- trades: request-type naming from Hyperliquid Docs `Rate limits and user limits` plus public trade field shape from Hyperliquid Docs `Websocket / Subscriptions`, which together remain the published baseline until Hyperliquid adds a dedicated `recentTrades` example page to the GitBook docs:
  - <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/rate-limits-and-user-limits>
  - <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/websocket/subscriptions>

### Decision: Keep `venueedge.MarketDataVenue` unchanged for Hyperliquid v0

The current `venueedge.MarketDataVenue` contract is already sufficient for Hyperliquid v0. Its three canonical reads cover the planned Hyperliquid scope: instruments, candles, and trades. Its existing request/result shapes already provide the half-open time ranges and opaque paging tokens needed for venue-local windowing without leaking Hyperliquid transport details into the data slice.

Rationale: `runtime/venueedge` is still a market-data-only boundary in the current architecture. Hyperliquid's broader product importance is execution-side, but this change does not need funding, mark price, account state, signing, wallet, or order-placement methods to land a first real market-data adapter cleanly.

Alternative considered: extend `MarketDataVenue` now for execution-adjacent perp concerns because Hyperliquid is perps-first. That would pull execution design into a chunk that is only meant to restore venue direction at the current market-data boundary, so it stays out of scope.

## Risks / Trade-offs

- Hyperliquid market-data needs may not line up cleanly with the current candle/trade contract -> Keep the contract narrow but allow Hyperliquid-specific mapping logic to stay inside the adapter, and treat any true contract mismatch as a design signal rather than hiding it.
- Keeping Binance may create extra maintenance overhead -> Limit the shared surface to what two adapters already need, and make Binance removal a follow-up if the maintenance cost becomes real.
- Hyperliquid's real long-term value is execution, not just market data -> Accept this temporarily and document that this change only restores product direction at the market-data edge, not the execution slice.
- Venue naming could lock in a poor canonical identifier -> Use `hyperliquid-perps` consistently unless the documented Hyperliquid market identity forces a later follow-up correction.
- Two concrete adapters may tempt early generalization -> Avoid registries, plugin systems, or cross-venue abstractions not directly exercised by the current tests.

## Migration Plan

1. Use the canonical venue identity `hyperliquid-perps` and the published v0 fixture-family references already captured in this change as the implementation baseline.
2. Add the OpenSpec delta for `venue-edge` so the spec explicitly allows more than one concrete real adapter.
3. Implement a Hyperliquid mocked-HTTP market-data adapter beside the existing Binance adapter.
4. Add adapter-specific unit tests and mocked-HTTP-to-data integration coverage using fixtures reduced from those published shapes without removing Binance tests.
5. Update the design follow-up notes with one explicit post-implementation Binance disposition: keep it as reference coverage for now, or recommend a separate removal change.
6. If Binance becomes a drag, remove it in a later dedicated change so rollback and reasoning stay clean.

Rollback strategy: revert the Hyperliquid adapter and its tests while leaving the existing Binance adapter and venue-edge contract intact.

## Open Questions

- Does Hyperliquid require adapter-local pagination or windowing around its documented public trade shape to satisfy the current canonical trade-read contract cleanly?
- Does Hyperliquid expose any data-shape mismatch that should trigger a follow-up venue-edge contract revision rather than adapter-only mapping?
- After Hyperliquid lands, should Binance remain as a long-lived reference adapter or should a separate removal change be proposed?

## Post-Implementation Verification

- Verified against the shipped adapter and tests in `runtime/venueedge`: this change preserved Binance-plus-Hyperliquid coexistence and kept the committed Hyperliquid fixture assumptions anchored to the documented `meta`, `candleSnapshot`, and `recentTrades` request families without needing additional OpenSpec deltas; the only notable v0 constraint is now explicit in code and coverage, where trade paging stays unsupported because `recentTrades` cannot be advanced deterministically by page token.

## Follow-up Notes

- Post-implementation Binance disposition: keep Binance as reference coverage for now, because the shipped tree still benefits from a second concrete adapter with passing mocked-HTTP and ingestion coverage while Hyperliquid v0 intentionally leaves trade paging narrower than Binance.
