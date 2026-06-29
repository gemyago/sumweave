## Why

Provider sync v2 now has a requested-window executor that fetches observations, loads a persisted snapshot, builds a diff plan, builds an apply plan, and hands those plans to a storage seam. The current seam is named `SyncRepository`, but the shape is not a classic repository: it coordinates workflow-specific snapshot loading and apply persistence across accounts, balances, raw payloads, transaction matches, and ledger transactions.

At the same time, `finance/persistence` already has canonical persistence operations such as `SaveTransaction`, provider-account persistence, raw-payload persistence, and provider-transaction-match persistence. Implementing the executor seam directly in persistence as another broad writer risks duplicating those semantics, creating parallel transaction save paths, and broadening the legacy persistence store boundary that the finance module is trying to phase out.

We should align the boundary before building the concrete apply implementation: provider sync workflow composition belongs in `finance/internal/providers`, while `finance/persistence` remains the implementation of dedicated durable primitives and focused snapshot queries.

## What Changes

- Rename the executor-facing seam from `SyncRepository` to `WindowSyncStore`.
- Implement the concrete `WindowSyncStore` in `finance/internal/providers`, where it owns snapshot assembly and apply-plan coordination.
- Define the provider-owned persistence boundary explicitly as snapshot reads plus `WithTransaction(ctx, fn func(WindowSyncApplyStore) error) error` for apply writes.
- Keep `finance/persistence` on dedicated primitives and adapters; do not introduce a sync-workflow writer in persistence or widen the legacy `persistence.Store` contract for this slice.
- Reuse existing canonical persistence operations for entity saves, especially transaction persistence, instead of duplicating GORM upsert semantics.
- Make snapshot loading concrete: load connection provider accounts, provider-source transactions for the mapped finance accounts inside the snapshot window, and provider transaction matches for the same connection that reference those loaded transactions.
- Keep pure sync decisions in `DiffPlanner`, `ApplyPlanner`, and policies; the provider-owned store coordinates persistence but does not decide matching or merge policy.

## Capabilities

### Modified Capabilities

- `finance-management`: clarify provider sync v2 storage boundaries so requested-window apply is coordinated by a provider-owned `WindowSyncStore` backed by explicit snapshot-read and transactional-apply persistence capabilities.

## Impact

- Affected code: `finance/internal/providers` and focused additions/adapters in `finance/persistence` for missing primitive query capabilities and transaction-scoped apply wiring.
- Affected tests: provider workflow tests for snapshot/apply coordination and persistence tests for any new primitive queries.
- Affected docs/specs: finance-management OpenSpec wording for provider sync v2 storage/apply boundaries.
- No public finance API or UI behavior changes.
