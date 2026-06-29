## 1. Boundary Naming

- [x] 1.1 Rename the executor-facing `SyncRepository` seam to `WindowSyncStore`; follow TDD flow by updating focused executor tests and stubs first, then update constructor options, required-dependency errors, and call sites.
- [x] 1.2 Update provider sync v2 wording in code comments and docs so the store is described as a provider sync workflow store, not a generic repository.

## 2. Provider-Owned Persistence Contract

- [x] 2.1 Define the provider-owned interfaces in `finance/internal/providers`: `WindowSyncSnapshotReader`, `WindowSyncApplyStore`, and `WindowSyncTransactor` with exact transaction boundary `WithTransaction(ctx, fn func(WindowSyncApplyStore) error) error`.
- [x] 2.2 Add focused provider-store tests with a fake persistence dependency before implementing snapshot loading or apply coordination.

## 3. Persistence Primitives And Adapters

- [x] 3.1 Add dedicated persistence read/query primitives for snapshot loading where existing methods do not match the required shape; do not add new sync-specific methods to the legacy `persistence.Store`, and follow TDD flow by adding persistence tests before implementation.
- [x] 3.2 Add a thin persistence transaction adapter that satisfies `WindowSyncTransactor` and exposes only `WindowSyncApplyStore` inside the callback.
- [x] 3.3 Keep existing canonical save methods as the transactional write path for transactions, provider accounts, balance snapshots, raw payloads, and transaction matches unless a method's semantics demonstrably do not match the sync apply need.

## 4. Provider-Owned Workflow Store

- [x] 4.1 Add the concrete `WindowSyncStore` implementation in `finance/internal/providers`.
- [x] 4.2 Implement `LoadExistingWindow` by loading connection provider accounts, provider-source transactions inside the snapshot window for the mapped finance accounts, and connection-scoped provider transaction matches for those loaded transaction IDs.
- [x] 4.3 Implement `ApplySync` by composing canonical persistence save methods inside one `WithTransaction` callback, reusing canonical transaction persistence instead of duplicating transaction upsert logic.

## 5. Composition And Verification

- [x] 5.1 Wire the provider-owned window sync store into finance provider sync composition without exposing the broad concrete persistence store to the executor.
- [x] 5.2 Run the finance module and repository completion checks required for code changes.
