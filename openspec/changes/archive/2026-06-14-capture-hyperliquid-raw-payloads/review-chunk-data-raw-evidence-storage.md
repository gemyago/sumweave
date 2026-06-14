# Chunk Review: `data-raw-evidence-storage`

## Round 1

- Scope: parent task 1 (`data-raw-evidence-storage`)
- Triggering input: implementation completed for chunk scope.
- Findings:
  - Added raw payload evidence fields and validation for request/response metadata, optional ingestion-run linkage, body refs/hashes, and optional market-data scope in `runtime/data`.
  - Added the v0 local raw payload blob store, shifted SQL persistence to `payload_body_ref` plus hashes/metadata only, and made `LineageServiceDeps` require blob-store injection while keeping `DatabaseStoreOpts` database-only.
  - Added raw-payload-to-normalized link persistence/readback for instruments, candles, and trades, plus backend config/wiring for `dataLayer.rawPayloadBlobStore.path` with default resolution under `dataDir`.
- Verdict: complete for chunk scope.
- Completion protocol status:
  - Focused checks: `go test ./runtime/data`, `go test ./apps/signal-foundry/internal/...`
  - Required repo check: `make affected-lint-test`
  - AGENTS.md update check: no changes needed
- Artifact cleanup status: clean
- Commit status: no commit created

## Round 2

- Scope: manager follow-up after committing the chunk.
- Triggering input: commit `915343b` (`add raw payload evidence foundation`).
- Findings: none.
- Verdict: chunk remains clean; commit recorded after the earlier review gate.
- Completion protocol status: unchanged; prior checks still pass.
- Artifact cleanup status: clean.
- Commit status: created `915343b` (`add raw payload evidence foundation`).
- Affected follow-up chunks: `hyperliquid-raw-capture`, `ingestion-raw-linkage`
