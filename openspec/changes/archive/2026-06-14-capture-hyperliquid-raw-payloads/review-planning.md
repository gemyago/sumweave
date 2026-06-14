# Planning Review

## Round 1

- Scope: `capture-hyperliquid-raw-payloads`
- Triggering input: OpenSpec proposal and design artifacts generated from the Notion task.
- Findings:
  - [required] Resolve the raw-capture handoff boundary before implementation. `design.md` leaves open whether raw payload identities reach ingestion through read-result metadata or an optional ingestion-only interface, but parent tasks 2 and 3 already depend on that choice. The current boundary only returns canonical results (`runtime/venueedge/types.go`, `runtime/venueedge/ingestion.go`), so the plan should explicitly name the minimal interface/type changes needed.
  - [required] Define v0 blob-store wiring/configuration. The proposal/design add local body-reference storage, but the current data-store construction only takes database concerns (`runtime/data/database_store.go`). The plan should say which layer owns blob-store path selection/injection and which constructor or wiring surface changes in v0.
  - [note] The proposal otherwise addresses the task and the parent-task order is logical once the two boundary decisions above are fixed.
- Verdict: revision required before implementation
- Chunking recommendation: use 3 sequential implementation chunks, one per parent task: `data-raw-evidence-storage` -> `hyperliquid-raw-capture` -> `ingestion-raw-linkage`. Do not combine them; task 1 is schema/storage heavy, task 2 depends on it, and task 3 depends on both. No scattered related work across non-consecutive parent tasks was found.
- Completion protocol status: non-coding review only; artifacts updated, lint/test not required
- Artifact cleanup status: no ad-hoc repository artifacts found; only standard OpenSpec files are present
- Commit status: not committed; `openspec/changes/capture-hyperliquid-raw-payloads/` is currently untracked in git

## Round 2

- Scope: `capture-hyperliquid-raw-payloads`
- Triggering input: follow-up re-review after planning revision.
- Findings:
  - None.
- Verdict: approved for implementation
- Chunking recommendation: keep 3 sequential implementation chunks, one per parent task: `data-raw-evidence-storage` -> `hyperliquid-raw-capture` -> `ingestion-raw-linkage`. Do not combine them. Parent task 1 remains the schema/storage/wiring foundation, task 2 depends on that storage boundary, and task 3 should wire ingestion only after the capture contract exists. No scattered related work across non-consecutive parent tasks was found.
- Notes:
  - The revision resolves the previous raw-capture handoff question by choosing optional read-result metadata on the existing venue-edge result structs while keeping canonical domain records and `MarketDataVenue` signatures unchanged.
  - The revision resolves v0 blob-store ownership by keeping blob-store path selection in app/runtime wiring, adding a data-layer blob-store dependency at `LineageService`, and keeping `DatabaseStoreOpts` database-only.
  - `tasks.md` is complete and ordered logically for the revised design.
- Completion protocol status: non-coding review only; artifacts updated, lint/test not required
- Artifact cleanup status: no ad-hoc repository artifacts found; only standard OpenSpec files are present
- Commit status: not committed; `openspec/changes/capture-hyperliquid-raw-payloads/` is currently untracked in git
