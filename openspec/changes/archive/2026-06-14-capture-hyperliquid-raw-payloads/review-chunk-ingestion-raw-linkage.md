# Chunk Review: `ingestion-raw-linkage`

## Round 1

- Scope: parent task 3 (`ingestion-raw-linkage`)
- Triggering input: implementation completed for chunk scope.
- Findings:
  - Added optional raw payload lineage wiring in `IngestionFlow` with `WithRawPayloadLineage` and per-type link helpers.
  - Refactored repeated paging logic into shared generic helper `ingestRecords`, preserving existing read/persist behavior while allowing raw payload IDs to flow per page into linkage.
  - Updated `IngestInstruments`, `IngestCandles`, and `IngestTrades` to read `ReadResultMetadata.RawPayloadIDs` and persist raw-to-normalized links after successful row persistence.
  - Extended ingestion tests with fake read fixtures and a fake lineage sink; covered link creation for instruments/candles/trades, empty metadata no-link behavior, and wrapped linkage errors.
- Verdict: complete for chunk scope.
- Completion protocol status:
  - Focused checks: `go test ./runtime/venueedge`
  - Required repo check: `make affected-lint-test`
  - AGENTS.md update check: no changes needed
- Artifact cleanup status: clean
- Commit status: committed in the current resume-step commit
