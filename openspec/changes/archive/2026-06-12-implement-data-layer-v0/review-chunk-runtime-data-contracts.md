# Chunk Review: runtime-data-contracts

Implementation and review history for chunk `runtime-data-contracts`.

## 2026-06-12 Initial implementation

Verdict: complete for chunk scope.

### Implemented

- Added `runtime/data` ingestion and read services that return concrete structs and accept consumer-defined store interfaces.
- Implemented validation and normalization for instrument upsert, candles, and trades, including required provenance, non-negative price and size checks, and UTC canonicalization.
- Allowed ingestion to upsert instruments by venue and symbol from normalized candle and trade records before persisting dependent records.
- Added local-fake service tests for valid ingestion, validation failures, UTC normalization, lookup normalization, and no persistence on rejected records.

### Checks

- `go test ./data`
- `make affected-lint-test`

### OpenSpec updates

- Marked tasks `2.1`, `2.2`, and `2.3` complete in `tasks.md`.

### Artifact cleanup

- Clean. No ad-hoc repository artifacts were created.

## 2026-06-12 Chunk finalization review

## Verdict

- Clean. Chunk `runtime-data-contracts` matches tasks `2.1-2.3` and is safe to build persistence on top of.
- `runtime/data/service.go` and `runtime/data/service_test.go` provide concrete service constructors, validation and normalization flow, and local-fake tests without pulling GORM or app wiring into this chunk.
- `tasks.md` and `manager-status.md` correctly record tasks `2.1-2.3` as complete.

## Continue Decision

- safe to continue to next chunk

## Completion Protocol Status

- Root AGENTS protocol: pass — `make affected-lint-test` completed successfully.
- Runtime module protocol: pass — `go test ./data` passed and no AGENTS.md update was needed.
- Changelog/artifact protocol: pass — code files plus OpenSpec task/status artifacts were committed for the chunk.

## Artifact Cleanup Status

- clean

## Commit Status

- commit created with sha `1c3094f` and message `Review runtime data contracts chunk`

## Affected Follow-up Chunks

- `gorm-persistence`
