## Why

Provider sync v2 already has the core transport and planning pieces: connector fetch, observation contracts, diff planning, apply planning, target-window planning, and the attempt journal. What is still missing is the one requested-window execution path that ties those pieces together.

The current `WindowSyncExecutor` only resolves a connector and fetches a batch. It does not yet load persisted snapshot data, build a diff plan, build an apply plan, or hand those plans to storage for atomic persistence. That gap leaves the new v2 sync path architecturally incomplete and makes the executor name misleading.

The current architecture and spec language also still uses `candidate window`, which is accurate but a little opaque in code. The actual purpose of that wider range is to load the persisted comparison snapshot for matching. We should keep the wider-window concept, but name it more clearly in code and docs as a snapshot window.

## What Changes

- Introduce a `SnapshotWindowPolicy` seam for deriving the persisted snapshot lookup window from one requested sync window.
- Introduce a new executor-facing storage seam for provider sync v2 snapshot loading and apply persistence.
- Implement the v2 `WindowSyncExecutor` flow as:
  - resolve connector
  - fetch provider observations
  - derive snapshot window
  - load existing snapshot
  - build diff plan
  - build apply plan
  - hand plans to storage
  - return sync result
- Keep sync-state journaling in the orchestrator for now; this change focuses on one-window execution.
- Clarify in architecture and spec wording that the prior `candidate window` is the snapshot lookup window used to load persisted comparison data and may be wider than the requested sync window.

## Capabilities

### Modified Capabilities

- `finance-management`: define and implement provider sync v2 requested-window execution through snapshot loading, diff/apply planning, and executor-owned apply handoff.

## Impact

- Affected code: `finance/internal/providers`, with focused contract/test updates in finance provider sync v2 code.
- Affected docs/specs: finance provider sync architecture wording and the finance-management OpenSpec delta.
- Persistence implementation is intentionally deferred behind a new interface seam; this change adds the executor-side contract and behavior, not the storage implementation.
