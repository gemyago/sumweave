## Context

The finance module already supports monobank token linking and PKO redirect/SCA linking through Enable Banking. Existing sync stores provider account mappings, balance snapshots, raw payloads, sync-run idempotency keys, and provider transaction matches. Existing domain docs also require provider-original retention and protection of user-edited transaction fields.

The current v1 sync path normalizes provider API data and applies it record-by-record inside the service layer. Matching first checks provider transaction ID and then a fallback fingerprint. When it finds a match, it rebuilds and saves the transaction from provider fields. That is not the right long-term foundation for pending-to-booked transitions, timestamp changes after settlement, ambiguous candidates, or user edits.

The v2 foundation should make each intermediate artifact inspectable before behavior is migrated.

```text
Provider Connector
        │
        ▼
ProviderSyncBatch
        │
        ▼
Existing Provider Window Snapshot
        │
        ▼
ProviderDiffPlan
        │
        ▼
Atomic Apply
        │
        ▼
ProviderSyncState + ProviderSyncRun + ProviderSyncStats
```

## Goals / Non-Goals

**Goals:**

- Define v2 provider sync domain types with clear `Provider` and `ProviderSync` naming.
- Define provider observations separately from ledger transactions.
- Define a generic connector contract that fetches observations and does not persist finance records directly.
- Define a provider profile shape so product providers can use technical connectors by composition.
- Define PKO as provider `pko` using connector `enable-banking`.
- Define per-connection sync state and run stats.
- Define a pure diff/dedup planning boundary.
- Make ambiguous matching conservative: create a new transaction and record diagnostics.
- Make user-edit preservation explicit through field comparison against previous provider-original values.
- Preserve existing v1 sync behavior until later migration chunks.

**Non-Goals:**

- Migrate monobank sync to v2.
- Migrate PKO/Enable Banking sync to v2.
- Add new finance UI or API workflows.
- Add a manual review workflow for ambiguous transactions.
- Auto-reconcile ledger balances from provider balance snapshots.
- Introduce arbitrary open-banking provider discovery.

## Decisions

1. Model provider observations, not final ledger records.

   Provider connectors should return account, balance, transaction, and raw payload observations. These records describe what the provider reported. They are not persistence models and are not automatically equivalent to finance ledger transactions.

   This keeps provider normalization separate from finance merge policy.

2. Track v2 sync state per connection.

   The first v2 foundation should use one sync state per bank connection. This is enough for the current monobank and PKO shape while keeping the model comprehensible. Provider-account or resource-level sync state can be added later if a provider requires it.

3. Use widened candidate windows for diffing.

   The requested sync window and the candidate lookup window should be separate concepts. Candidate lookup should be allowed to include a halo before and after the requested provider window so pending transactions that settle with shifted timestamps do not duplicate unnecessarily.

   ```text
   requested window:
       May 10 ───────────────── May 20

   candidate lookup window:
       May 03 ───────────────────────── May 27
   ```

4. Keep diff planning pure.

   The diff planner should consume a provider batch and an existing-window snapshot, then produce a plan. It should not perform persistence writes, network calls, ID generation side effects, or logging-dependent decisions. Deterministic input should produce deterministic output.

5. Prefer new transactions for weak or ambiguous matches.

   Strong matches should update existing provider-synced transactions. Weak or ambiguous candidates should create new transactions and increment `AmbiguousCreatedTransactions`. Creating duplicates is safer than silently merging the wrong financial event.

6. Preserve user edits by comparing current fields with previous provider-original values.

   On update, provider-original fields should always be refreshed from the new observation. User-facing fields such as description, amount, and effective timestamp should update only when the current value still equals the previous provider-original value. If a user changed the field, v2 should preserve it.

   ```text
   current display field == old provider-original field
        │
        ├─ yes: update display field from new provider observation
        └─ no:  preserve display field as user-edited
   ```

7. Separate product providers from technical connectors.

   PKO is the product-level provider because that is what the user linked. Enable Banking is the technical connector used to communicate with that provider.

   ```text
   provider:  pko
   connector: enable-banking
   profile:   PKO Bank Polski / PL / personal
   ```

   This should be composition/configuration, not inheritance. Future banks can use the same Enable Banking connector with different provider profiles.

8. Keep balance snapshots as observations.

   Provider balances should remain stored observations. They should not directly mutate ledger balances. Any future reconciliation workflow should be explicit and explainable through system transactions.

## Planned Data Shape

Domain types should be added under `finance/domain` with clear names such as:

- `ProviderID`
- `ProviderConnectorID`
- `ProviderConnectionRef`
- `ProviderSyncWindow`
- `ProviderSyncState`
- `ProviderSyncRun`
- `ProviderSyncStats`
- `ProviderSyncIssue`
- `ProviderAccountObservation`
- `ProviderBalanceObservation`
- `ProviderTransactionObservation`
- `ProviderTransactionOriginal`
- `ProviderRawPayloadObservation`
- `ProviderSyncBatch`

Internal provider package types should be added under `finance/internal/providers` with clear contracts such as:

- `Connector`
- `ConnectorCapabilities`
- `ProviderProfile`
- `StartLinkRequest`
- `StartLinkResult`
- `FinishLinkRequest`
- `LinkTokenRequest`
- `LinkResult`
- `FetchRequest`
- `ExistingWindowSnapshot`
- `DiffPlanner`
- `ProviderDiffPlan`
- `ProviderTransactionAction`
- `Applier`
- `SyncCoordinator`

The initial methods may be stubs, but they should document planned behavior in comments so the next implementation chunk can fill them in without rediscovering the architecture.

## Risks / Trade-offs

- Adding v2 types before provider migration creates temporary parallel terminology. The names must be explicit and the old v1 behavior must remain unchanged until migration.
- Creating new transactions for ambiguous matches can create visible duplicates. This is acceptable because duplicates are easier to detect and correct than incorrect silent merges.
- Per-connection sync state may be too coarse for some future providers. The early-alpha scope allows extending the model later without compatibility constraints.
- Field-diff user edit detection depends on reliable previous provider-original values. Existing synced rows without enough provider-original detail may need conservative behavior when migrated.

## Migration Plan

- Add compile-safe domain and internal provider foundation types.
- Add behavior comments and minimal compile coverage for the new package.
- Do not wire v2 into existing provider sync execution.
- Migrate providers in later chunks by first adapting one provider to produce `ProviderSyncBatch`, then applying the diff/apply path behind existing job execution.

## Open Questions

- What candidate-window halo should the first implementation use by default?
- Should ambiguous-created diagnostics be persisted only in sync stats, or also as individual sync issues?
