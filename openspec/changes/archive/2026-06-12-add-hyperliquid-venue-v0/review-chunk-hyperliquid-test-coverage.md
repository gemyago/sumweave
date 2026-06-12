# Chunk Review: hyperliquid-test-coverage

Implementation and review record for chunk `hyperliquid-test-coverage`.

## 2026-06-12 Implementation

Verdict: complete for chunk scope `3.1-3.3`.

### Implemented

- Added [runtime/venueedge/hyperliquid_perps_test.go](/Users/jenya/projects/signal-foundry/runtime/venueedge/hyperliquid_perps_test.go) with mocked-HTTP unit coverage for Hyperliquid `meta`, `candleSnapshot`, and `recentTrades`.
- Covered documented success mapping for instruments, candles, and trades using reduced official-doc-style payloads.
- Covered truthful paging behavior: candle reads page via `startTime` tokens, while trade reads reject non-empty page tokens with the explicit v0 unsupported-paging validation error.
- Covered non-success statuses, Hyperliquid venue error payloads, and malformed payloads, including malformed-mapping cases for candle/trade symbol mismatches.
- Added mocked-HTTP-to-sqlite ingestion coverage that persists Hyperliquid instruments/candles/trades through `IngestionFlow` and reads them back deterministically through the existing data read service.
- Kept existing Binance adapter coverage passing by leaving [runtime/venueedge/binance_spot_test.go](/Users/jenya/projects/signal-foundry/runtime/venueedge/binance_spot_test.go) untouched and verifying the full `runtime/venueedge` package.
- Added minimal production cleanup in [runtime/venueedge/hyperliquid_perps.go](/Users/jenya/projects/signal-foundry/runtime/venueedge/hyperliquid_perps.go) by centralizing repeated Hyperliquid request-type strings into adapter-local constants.

### Checks

- `direnv exec /Users/jenya/projects/signal-foundry go test ./runtime/venueedge`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec updates

- Marked tasks `3.1`, `3.2`, and `3.3` complete in `tasks.md`.
- Updated `manager-status.md` to record chunk `hyperliquid-test-coverage` as complete in the working tree.
- Created this chunk review artifact with the implemented coverage and verification evidence.

### Assumptions

- Hyperliquid v0 test fixtures remain intentionally reduced from the official fixture families already pinned in `design.md`; this chunk did not expand into live-venue checks or parent task 4 docs work.
- Trade-ingestion coverage uses a single bounded `recentTrades` response rather than pretending pageable advancement exists, matching the current truthful adapter behavior.

### Completion Protocol Status

- Root/AGENTS protocol: pass — `make affected-lint-test` completed successfully.
- Runtime/module protocol: pass — `MarketDataVenue` stayed unchanged and no `AGENTS.md` updates were needed.
- `/opsx-apply` confirmation: pass — no literal `/opsx-apply` command was exposed in this environment, so the required chunk artifacts, task status, and review evidence were updated directly through the repository OpenSpec workflow.

### Commit Status

- No commit created in this chunk; code and artifact updates remain in the working tree.

## 2026-06-12 Review

## Verdict
- PASS for chunk scope `3.1-3.3`.
- The current Hyperliquid tests and small adapter refactor align with requested scope, and existing Binance coverage remains untouched in source while still passing in the venueedge package.

## Continue Decision
- Safe to continue to parent task 4 (`venue-edge-docs-verification`).

## Completion Protocol Status
- Reviewed chunk scope and related artifacts for scope drift and correctness.
- Confirmed gate checks with `make affected-lint-test` (pass, no issues) and `go test ./runtime/venueedge` (pass).
- No additional AGENTS updates were required for this chunk.

## Artifact Cleanup Status
- Durable evidence appended to this file only as requested.
- Existing OpenSpec artifacts were updated through `tasks.md` and `manager-status.md` in scope for chunk completion tracking.
- No ad-hoc files introduced.

## Commit Status
- No chunk commit has been made yet.
- `hyperliquid-test-coverage` is still uncommitted; next step is to commit once explicit approval confirms continuation.
