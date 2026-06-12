# Chunk Review: hyperliquid-adapter

Implementation and review history for chunk `hyperliquid-adapter`.

## 2026-06-12 Initial implementation

Verdict: complete for chunk scope.

### Implemented

- Added a concrete Hyperliquid perps market-data adapter in [runtime/venueedge/hyperliquid_perps.go](/Users/jenya/projects/signal-foundry/runtime/venueedge/hyperliquid_perps.go) beside the existing Binance adapter, with injected HTTP client and configurable base URL.
- Mapped Hyperliquid `meta` universe entries into canonical `domain.Instrument` records with venue `hyperliquid-perps` and asset class `future`, plus canonical active/inactive state from the documented delisting flag.
- Mapped Hyperliquid `candleSnapshot` and `recentTrades` payloads into canonical `domain.Candle` and `domain.Trade` records with stable raw provenance under source `hyperliquid-perps-rest`.
- Kept Hyperliquid request bodies, timeframe mapping, HTTP error decoding, symbol handling, and opaque `startTime` page-token behavior isolated inside the adapter without changing `venueedge.MarketDataVenue` or the data-layer contract.

### Checks

- `direnv exec /Users/jenya/projects/signal-foundry/runtime go test ./venueedge`
- `direnv exec /Users/jenya/projects/signal-foundry/runtime /Users/jenya/projects/signal-foundry/bin/golangci-lint run ./venueedge`
- `curl -fsSL -X POST https://api.hyperliquid.xyz/info -H 'Content-Type: application/json' -d '{"type":"recentTrades","coin":"BTC"}' | head -c 400`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec updates

- Marked tasks `2.1`, `2.2`, and `2.3` complete in `tasks.md`.
- Updated `manager-status.md` to mark chunk `hyperliquid-adapter` complete and record the implementation/check status.

### Assumptions

- `recentTrades` uses a top-level `coin` field together with `type: "recentTrades"`; this matched a live public-endpoint sanity check, while the committed implementation baseline still follows the official-source direction already recorded in `design.md`.
- Trade pagination remains adapter-local in v0 through repeated `recentTrades` reads filtered by opaque `startTime` page tokens, without expanding the canonical contract.

### Artifact cleanup

- Clean. No ad-hoc repository artifacts were created.

## Completion Protocol Status

- Root/AGENTS protocol: pass — `make affected-lint-test` completed successfully.
- Runtime/module protocol: pass — `MarketDataVenue` stayed unchanged and no `AGENTS.md` updates were needed.
- `/opsx-apply` confirmation: pass — no literal `/opsx-apply` command was exposed in this environment, so the required chunk artifacts, task status, and review evidence were updated directly through the repository OpenSpec workflow.

## Commit Status

- no commit created; chunk changes remain uncommitted in the working tree

## 2026-06-12 Final reviewer check

## Verdict
Chunk `hyperliquid-adapter` is mostly aligned to scope `2.1-2.3`, but one correctness issue blocks a full-safe completion.

## Continue Decision
Do not continue to the next chunk until this is addressed.

## Completion Protocol Status
Root module protocol is still expected but not yet revalidated by this reviewer after this pass. `go test ./venueedge` is green in the runtime module, and existing status artifacts still report `make affected-lint-test` as previously passed.

## Artifact Cleanup Status
No ad-hoc files were added in this review pass. Existing chunk review append is in standard OpenSpec artifacts only.

## Commit Status
Chunk remains uncommitted and manager ledger still marks this chunk `complete` with `pending` commit.

## Findings

1) High: `ReadTrades` computes a next page token from `startTime`/`trade.EventTime` but does not send the start time back to Hyperliquid in the request payload. This makes trade paging purely client-side filtering over whatever `recentTrades` currently returns. In live behavior, `/info` `recentTrades` returns a fixed-size latest-trade list regardless of `startTime`, so pagination can repeatedly return the same page instead of advancing deterministically.
2) Medium: Because trade pagination is not coupled to API-side windowing and uses only `startTime`, calls with the same request can re-evaluate the same bounded response window, which is fragile for deterministic ingestion workflows expected by the venue contract.

## 2026-06-12 Scoped follow-up fix: hyperliquid-adapter-trade-pagination-fix

Verdict: fixed safely and minimally.

### Behavior change

- `ReadTrades` for Hyperliquid no longer generates or returns a `NextPageToken`.
- `ReadTrades` now rejects any non-empty `PageToken` with a validation error instead of pretending deterministic trade paging is supported.
- Hyperliquid trade reads remain bounded by canonical time-range filtering and optional `PageSize`, but v0 no longer claims pageable advancement for `recentTrades`.

### Rationale

- Local evidence confirmed `POST /info` `{"type":"recentTrades","coin":"BTC"}` and the same payload plus `startTime` return the same latest-trade window, so `recentTrades` does not provide trustworthy page advancement for this adapter.
- Explicitly unsupported paging is safer and more truthful than emitting tokens that can repeat the same response window.

### Checks

- `direnv exec /Users/jenya/projects/signal-foundry/runtime go test ./venueedge`
- `direnv exec /Users/jenya/projects/signal-foundry/runtime /Users/jenya/projects/signal-foundry/bin/golangci-lint run ./venueedge`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec updates

- Updated this chunk review to record the scoped trade-pagination fix and the new exact runtime behavior.
- Updated `manager-status.md` to move the follow-up fix chunk to complete and clear the blocker on chunk 2 finalization.

### Assumptions

- Hyperliquid v0 trade pagination should stay unsupported until the official/public API provides deterministic server-side trade window advancement for `recentTrades` or another documented endpoint replaces it.

## 2026-06-12 Follow-up Finalization Re-review: trade pagination fix

## Verdict
Chunk `hyperliquid-adapter` is aligned to its scope after `hyperliquid-adapter-trade-pagination-fix` and no longer advertises unsupported Hyperliquid trade paging as functioning pagination.

## Continue Decision
Safe to continue to the next chunk.

## Completion Protocol Status
- `make affected-lint-test`: pass (root-level run completed successfully).
- Runtime checks: `go test ./venueedge`: pass.
- Chunk scope `2.1-2.3` remains covered by `runtime/venueedge/hyperliquid_perps.go` and prior OpenSpec task markers.

## Artifact Cleanup Status
- Standard artifacts only: `openspec/changes/add-hyperliquid-venue-v0/review-chunk-hyperliquid-adapter.md` appended with this re-review.
- No ad-hoc files introduced in this pass.

## Commit Status
- Chunk changes are still present in working tree as uncommitted runtime and OpenSpec review artifacts; a commit is still needed before proceeding to archive/finalization milestones.

## Findings
- No blocking issues identified in this pass.
- Behavioral fix is consistent with verified `/info` `recentTrades` behavior that does not provide deterministic page advancement.
