# Finance Provider Window Sync Executor Plan

This plan captures what is already present and what still needs to be built to implement `finance/internal/providers/window_sync_executor.go` in line with `docs/finance-provider-sync-architecture.md`.

## Session Layer Context

- `SyncOrchestrator` sits above this plan's `WindowSyncExecutor`.
- The orchestrator owns latest-succeeded-state loading, target-window planning, `<=30d` chunking, oldest-first chunk coordination, and append-only succeeded-state progression.
- This document stays focused on the one-requested-window executor below that session layer.

## What Already Exists

- `WindowSyncRequest` already carries the right inputs for orchestration: connection, secret, requested window, prior sync state, job ID, reason in `finance/internal/providers/window_sync_executor.go:14-21`.
- Connector fetch contract exists in `finance/internal/providers/contracts.go:67-89`.
- Snapshot shape for diffing exists in `finance/internal/providers/contracts.go:74-80`.
- Pure diff planning exists in `finance/internal/providers/diff.go:51-90`.
- Pure apply planning and merge policy exist in `finance/internal/providers/apply.go:31-129`.
- V2 domain models for batch, run, state, stats, and issues exist in `finance/domain/provider_sync_v2.go:71-136`.

## What The Window Sync Executor Still Needs

- Connector resolution.
  The executor needs a way to pick the right `Connector` from `request.Connection.ConnectorID`.
- Candidate window policy.
  The doc requires loading a wider window than the requested one, but there is no policy yet to compute that window. The tests only show an example `-72h/+72h`, not real behavior.
- Snapshot loading.
  The executor needs a repository method that loads `ExistingWindowSnapshot` for one connection and one candidate window.
- Atomic apply writer.
  The doc says apply is atomic. There is no executor-side dependency that can persist accounts, balances, transactions, matches, raw payloads, run, and state in one unit.
- Run/state persistence.
  `domain.ProviderSyncRun` and `domain.ProviderSyncState` exist, but there are no V2 persistence models or store methods for them.
- Clock and run metadata.
  The executor needs `now()` and probably a logger so it can stamp started/completed/failure state consistently.
- Failure handling.
  The executor needs a clear path for fetch failures, apply failures, and partial planning issues.

## Biggest Storage Gaps

- No V2 persistence for `ProviderSyncRun` / `ProviderSyncState`.
  The current store only has legacy `BankConnectionSyncRun`, not the new V2 shapes.
- No bulk match loading API.
  Diff planning expects all candidate matches, but persistence only supports exact lookup by provider ID or fingerprint in `finance/persistence/provider_sync_store.go:486-577`.
- No transaction window query.
  Current `ListTransactions` does not filter by date window in `finance/persistence/core_store.go:487-519`, but the new flow needs candidate-window loading.
- No executor-facing transaction boundary abstraction.
  The new flow needs one atomic write step; the current APIs are many individual saves.

## Concrete Window Sync Executor Flow To Build

1. Generate `runID`.
2. Resolve connector from `request.Connection`.
3. Compute candidate window from `request.RequestedWindow` and maybe `request.SyncState`.
4. Persist a `ProviderSyncRun` as `running` and update sync state `LastAttemptAt`.
5. Call `connector.Fetch(...)`.
6. Load `ExistingWindowSnapshot` for the candidate window.
7. Build `diffPlan` with `DiffPlanner`.
8. Build `applyPlan` with `ApplyPlanner`.
9. Persist all writes atomically:
   - provider accounts
   - balance observations
   - created/updated transactions
   - provider transaction matches
   - raw payload observations
   - completed run record
   - updated sync state
10. Return `WindowSyncResult{RunID, Stats, Issues}`.
11. On error, persist failed run/state and return the error.

## Important Design Note

- The old sync flow in `finance/provider_sync.go` has reusable ideas for account upsert, transaction save, match save, and lifecycle updates.
- But it should not be copied directly.
  That older flow writes item-by-item and does direct exact-match lookup, while the new architecture explicitly requires `fetch -> load existing window -> plan diff -> apply atomically` and conservative ambiguous handling.

- A future sync orchestrator can sit above the window sync executor to decide rolling coverage windows and chunking, but that does not change the executor's one-requested-window responsibility.

## Minimum New Contracts I’d Add

```go
type ConnectorRegistry interface {
	Resolve(connectorID domain.ProviderConnectorID) (Connector, error)
}

type CandidateWindowPolicy interface {
	Expand(requested domain.ProviderSyncWindow, state *domain.ProviderSyncState) domain.ProviderSyncWindow
}

type SyncRepository interface {
	LoadExistingWindow(ctx context.Context, connection domain.ProviderConnectionRef, window domain.ProviderSyncWindow) (ExistingWindowSnapshot, error)
	ApplySync(ctx context.Context, run domain.ProviderSyncRun, state domain.ProviderSyncState, diff ProviderDiffPlan, apply ApplyPlan) error
	MarkRunFailed(ctx context.Context, run domain.ProviderSyncRun, state domain.ProviderSyncState, err error, issues []domain.ProviderSyncIssue) error
}
```

## Short Answer

To build `Execute`, you do not need more diff/apply logic first. You need orchestration dependencies and persistence support: connector lookup, candidate window calculation, snapshot loading, atomic apply, and V2 run/state storage. The pure planning core is already there.
