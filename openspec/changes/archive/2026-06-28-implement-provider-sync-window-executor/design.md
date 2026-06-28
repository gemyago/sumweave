## Context

Provider sync v2 has been built incrementally:

- domain types and connector/profile contracts exist
- connector resolution exists
- target-window planning exists
- latest-attempt sync journaling exists
- pure diff/apply planning exists
- synthetic provider fetch exists

The missing link is the one-window executor implementation. `finance/internal/providers/window_sync_executor.go` still stops after connector fetch and returns fetched observations directly. That means the real v2 execution path is not yet:

`fetch -> load snapshot -> diff -> apply-plan -> apply`

This change closes that gap while keeping persistence implementation behind an interface seam.

## Goals / Non-Goals

**Goals:**

- Add one executor-facing repository seam for snapshot loading and apply persistence.
- Add one `SnapshotWindowPolicy` seam with a simple default implementation for the current phase.
- Implement `WindowSyncExecutor` requested-window logic against those seams.
- Keep `DiffPlanner` and `ApplyPlanner` as pure planning components.
- Keep journaling ownership in `SyncOrchestrator` for now.
- Clarify `candidate window` terminology as the snapshot lookup window in code-facing docs/specs.
- Keep the repository implementation deferred even though the executor logic is implemented now.

**Non-Goals:**

- No concrete persistence implementation for the new repository seam in this change.
- No provider sync v2 run table or run persistence in this change.
- No new public finance API or UI behavior.
- No migration of the legacy v1 provider sync service path.
- No broader provider-account creation/apply refactor beyond the executor seam.

## Decisions

1. Add a `SnapshotWindowPolicy` consumer-defined interface next to the executor.

   The executor needs a way to derive the window used to load persisted comparison data. That concern is distinct from target-window planning:

   - `TargetWindowPolicy` chooses what requested window should be synced next.
   - `SnapshotWindowPolicy` chooses what persisted snapshot window should be loaded for comparison while executing that requested window.

   The interface should stay close to the executor because it is executor-specific orchestration behavior.

2. Keep the first concrete snapshot policy simple and deterministic.

   The first executor implementation does not need speculative widening logic. A minimal default policy can return the requested window unchanged, while preserving the explicit seam for later widening when real-provider behavior demands it.

   This still matches the architecture/spec wording because the snapshot lookup window may be wider than the requested window, not must always be wider.

3. Add one executor-facing storage seam named `SyncRepository`.

   The later executor implementation needs two persistence-facing operations:

   - load the existing snapshot for one connection and one snapshot window
   - apply the diff/apply result

   That seam belongs in `finance/internal/providers` because it is owned by the consumer workflow, not by a generic persistence package.

   In this iteration we should only define the interface. The concrete persistence implementation should follow later once the executor-side contract is settled, but the executor logic itself should already depend on this seam now.

   The seam should be:

   ```go
   type SyncRepository interface {
       LoadExistingWindow(ctx context.Context, connection domain.ProviderConnectionRef, window domain.ProviderSyncWindow) (ExistingWindowSnapshot, error)
       ApplySync(ctx context.Context, diffPlan ProviderDiffPlan, applyPlan ApplyPlan) error
   }
   ```

4. Keep planner ownership inside the executor.

   `DiffPlanner` and `ApplyPlanner` are already pure and local. The executor implementation should create/use those planners directly instead of pushing planning into the repository. That keeps persistence from owning business logic.

5. Preserve orchestrator ownership of the attempt journal.

   `SyncOrchestrator` already appends latest-attempt state rows around each `windowExecutor.Execute(...)` call. This change should not split that responsibility. The executor focuses on one-window fetch/snapshot/diff/apply behavior; the orchestrator continues to own multi-window coordination and attempt journaling.

6. Do not add separate requested-window validation in this slice.

   The current requested-window sources are already finance-owned and tightly controlled by target-window planning plus chunk splitting. That means extra validation inside this planning slice would be defensive rather than necessary.

   If future flows introduce looser or user-supplied requested windows, or if the eventual executor implementation needs its own bounded guardrail, we can add that as a focused follow-up without making this design artificially broader now.

## Risks / Trade-offs

- [Default snapshot policy is not widened yet] -> Accept for now because the seam exists and the architecture wording still allows later widening without changing the executor contract.
- [Repository seam may later need run persistence or richer apply inputs] -> Accept for now; `ProviderDiffPlan` and `ApplyPlan` already carry the important execution intent, and the interface can evolve in a later change if run persistence is added.
- [Executor logic will compile and test before real persistence wiring exists] -> Accept because that is the point of the seam: finish orchestration behavior now, then plug in real storage later.

## Migration Plan

1. Add OpenSpec wording clarifying that the old candidate lookup window is the snapshot lookup window.
2. Introduce `SnapshotWindowPolicy` and `SyncRepository` as executor-owned seams.
3. Implement `WindowSyncExecutor` against those seams with focused tests and stubbed repository behavior.
4. Keep concrete repository implementation for a follow-up change.

## Open Questions

- Should the first real snapshot-window widening policy eventually live as a fixed buffer, provider-aware policy, or repository-owned expansion rule once real provider drift is observed?
