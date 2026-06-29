## Context

The previous window-executor change intentionally deferred concrete persistence and introduced an executor-facing `SyncRepository` seam:

```go
type SyncRepository interface {
    LoadExistingWindow(ctx context.Context, connection domain.ProviderConnectionRef, window domain.ProviderSyncWindow) (ExistingWindowSnapshot, error)
    ApplySync(ctx context.Context, diffPlan ProviderDiffPlan, applyPlan ApplyPlan) error
}
```

That was enough to complete executor orchestration, but the name and likely implementation location are now misleading. `ApplySync` is not a single aggregate repository save. It is a provider sync workflow operation that needs snapshot reads, a transaction boundary, and several canonical persistence primitives.

The finance persistence package also already contains canonical entity persistence operations, including `SaveTransaction`. Duplicating those operations inside a new persistence-layer sync writer would make future ledger behavior harder to reason about. The finance module rules also explicitly say not to keep broadening the legacy `persistence.Store`; new sync-specific persistence behavior should land as dedicated stores/adapters, not as more methods on a god object.

## Goals / Non-Goals

**Goals:**

- Keep requested-window sync composition in `finance/internal/providers`.
- Rename the executor dependency to reflect workflow storage, not a generic repository.
- Fix the transaction-boundary contract now, not during implementation.
- Let the provider-owned sync store depend on narrow consumer-defined snapshot-read and apply-write interfaces.
- Reuse canonical persistence entity saves, especially `SaveTransaction`, inside the apply workflow.
- Add missing persistence primitives only when existing methods do not match the sync read/write shape.
- Keep diff and apply policy decisions outside persistence.

**Non-Goals:**

- No public finance API or UI changes.
- No rewrite of legacy provider sync v1.
- No broad refactor of `finance/persistence.Store`.
- No new sync-specific methods added to the legacy `persistence.Store` contract.
- No replacement of existing transaction persistence semantics.
- No attempt to make generic user-facing list methods serve provider sync snapshot queries when their shape does not match.

## Decisions

1. Rename the executor seam to `WindowSyncStore`.

   The executor dependency should describe the workflow it supports:

   ```go
    type WindowSyncStore interface {
        LoadExistingWindow(ctx context.Context, connection domain.ProviderConnectionRef, window domain.ProviderSyncWindow) (ExistingWindowSnapshot, error)
        ApplySync(ctx context.Context, diffPlan ProviderDiffPlan, applyPlan ApplyPlan) error
    }
   ```

   `WindowSyncStore` is intentionally workflow-shaped. It can load the exact snapshot shape the diff planner needs and apply the exact plans the apply planner produces.

2. Implement the concrete workflow store in `finance/internal/providers`.

   A concrete provider-owned store should live beside the provider sync workflow because it coordinates provider sync concepts: `ExistingWindowSnapshot`, `ProviderDiffPlan`, `ApplyPlan`, provider observations, and transaction match intent.

   This avoids making `finance/persistence` import or own provider workflow policy types beyond the primitive methods needed to persist/load durable records.

3. Depend on explicit provider-owned persistence interfaces.

   The provider-owned store should not depend on the broad concrete `*persistence.Store`. Instead, define narrow interfaces in `finance/internal/providers` with only the persistence capabilities required by provider sync v2.

   The boundary is:

   ```go
   type WindowSyncSnapshotReader interface {
       ListConnectionProviderAccounts(
           ctx context.Context,
           connectionID string,
       ) ([]domain.ConnectionProviderAccount, error)
       ListProviderTransactionsInWindow(
           ctx context.Context,
           financeAccountIDs []string,
           window domain.ProviderSyncWindow,
       ) ([]domain.Transaction, error)
       ListProviderTransactionMatchesByTransactionIDs(
           ctx context.Context,
           connectionID string,
           transactionIDs []string,
       ) ([]domain.ProviderTransactionMatch, error)
   }

   type WindowSyncApplyStore interface {
       SaveConnectionProviderAccount(
           ctx context.Context,
           account domain.ConnectionProviderAccount,
       ) (domain.ConnectionProviderAccount, error)
       SaveBalanceSnapshot(
           ctx context.Context,
           snapshot domain.BalanceSnapshot,
       ) (domain.BalanceSnapshot, error)
       SaveRawPayload(
           ctx context.Context,
           payload domain.RawPayload,
       ) (domain.RawPayload, error)
       SaveTransaction(
           ctx context.Context,
           transaction domain.Transaction,
       ) (domain.Transaction, error)
       SaveProviderTransactionMatch(
           ctx context.Context,
           match domain.ProviderTransactionMatch,
       ) (domain.ProviderTransactionMatch, error)
   }

   type WindowSyncTransactor interface {
       WithTransaction(ctx context.Context, fn func(WindowSyncApplyStore) error) error
   }

   type WindowSyncPersistence interface {
       WindowSyncSnapshotReader
       WindowSyncTransactor
   }
   ```

   `WithTransaction` is the resolved transaction-boundary contract for this change. The callback receives only `WindowSyncApplyStore`, so snapshot reads and any broader persistence surface are not exposed inside apply orchestration.

4. Keep persistence implementation dedicated and narrow.

   `finance/persistence` must not grow a sync-workflow repository and must not treat `*persistence.Store` as the new provider sync boundary. Instead, persistence should expose dedicated components or adapters that satisfy the provider-owned interfaces above.

   That means:

   - no new sync-specific methods on `*persistence.Store`
   - no `WindowSyncStore` implementation under `finance/persistence`
   - focused read/query additions in dedicated persistence code when a primitive is missing
   - a thin transaction adapter that enters a DB transaction and supplies a transaction-scoped `WindowSyncApplyStore`

5. Reuse canonical entity save methods.

   The provider-owned workflow store should call existing persistence methods when the semantics match. In particular, provider transaction apply should call the canonical transaction save method instead of reimplementing the transaction upsert.

   The apply workflow must run inside the `WithTransaction` callback so canonical save methods still execute atomically.

6. Make snapshot loading scope concrete.

   `LoadExistingWindow` must assemble `ExistingWindowSnapshot` in this order:

   1. Load all `ConnectionProviderAccount` rows for `connection.ConnectionID`.
   2. Derive the mapped `FinanceAccountID` values from those accounts.
   3. Load provider-source `Transaction` rows for those finance accounts whose `EffectiveAt` falls inside the snapshot window.
   4. Load `ProviderTransactionMatch` rows for the same connection whose `TransactionID` belongs to the transactions loaded in step 3.

   That query scope is sufficient for both strong-match lookup and weak-candidate detection used by the current diff planner. It also keeps match loading tied to the same concrete transaction set instead of inventing a separate fuzzy window rule for matches.

7. Add dedicated snapshot queries instead of bending display queries.

   Existing methods such as `ListTransactions` are close in spirit but have a user-facing shape: tenant/account/source/status filters, display ordering, and no snapshot window semantics. Provider sync snapshot loading needs finance-account/window-scoped comparison data and connection-scoped transaction matches for those loaded transactions.

   Add focused primitive queries for provider sync v2 snapshot loading rather than overloading user-facing list methods.

8. Keep persistence primitive and workflow responsibilities separate.

   Persistence may own GORM models, durable constraints, timestamps, transaction boundaries, and model/domain mapping. The provider-owned sync store may assemble a snapshot and sequence apply writes. It must not own matching strategy, weak/ambiguous candidate policy, merge policy, or stats/issues semantics.

## Risks / Trade-offs

- [More interfaces in providers] -> Accept because the interfaces are consumer-defined and narrow, and they prevent the old broad store from leaking through the provider workflow.
- [Snapshot queries still require persistence additions] -> Accept because those additions are primitive query capabilities, not provider workflow orchestration.
- [Dedicated persistence adapters add a little wiring] -> Accept because that wiring is cheaper than further broadening the legacy store boundary.
- [Workflow store in providers coordinates persistence] -> Accept because the coordination is provider-sync-specific and keeps persistence from becoming a second planner.
- [Naming churn from `SyncRepository`] -> Accept before a concrete implementation hardens the old name.

## Migration Plan

1. Rename `SyncRepository` and related constructor option/error names to `WindowSyncStore`.
2. Define the provider-owned persistence interfaces, including `WithTransaction(ctx, fn func(WindowSyncApplyStore) error) error`.
3. Add dedicated persistence read/query primitives and transaction adapters to satisfy those interfaces without broadening `*persistence.Store`.
4. Implement the provider-owned `WindowSyncStore.LoadExistingWindow` against the explicit snapshot query scope.
5. Implement the provider-owned `WindowSyncStore.ApplySync` by composing canonical save methods inside one transaction callback.
6. Wire the concrete provider-owned store into finance provider sync composition and verify the affected checks.

## Open Questions

- None for this slice.
