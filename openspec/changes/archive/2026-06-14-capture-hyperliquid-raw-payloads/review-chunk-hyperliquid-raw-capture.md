# Chunk Review: `hyperliquid-raw-capture`

## Round 1

- Scope: parent task 2 (`hyperliquid-raw-capture`)
- Triggering input: implementation completed for chunk scope.
- Findings:
  - Added optional raw-evidence recording to Hyperliquid `/info` reads without changing `MarketDataVenue` method signatures or canonical `domain.Instrument`, `domain.Candle`, and `domain.Trade` outputs.
  - Added optional raw-payload metadata on existing venue-edge read results plus mocked-HTTP coverage for `meta`, `candleSnapshot`, deterministic `recentTrades`, non-2xx/malformed responses, timestamps, request hashes, entity hints, optional ingestion-run context, and repeated-fetch fresh raw IDs.
- Verdict: complete for chunk scope.
- Completion protocol status:
  - Focused checks: `go test ./runtime/venueedge`
  - Required repo check: `make affected-lint-test`
  - AGENTS.md update check: no changes needed
- Artifact cleanup status: clean
- Commit status: no commit created
