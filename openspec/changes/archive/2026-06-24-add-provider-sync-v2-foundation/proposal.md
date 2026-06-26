## Why

Finance provider sync currently mixes provider API normalization, sync orchestration, persistence application, and transaction matching inside the finance service layer. That shape was useful for the first slice, but it makes the next provider-sync work hard to reason about before implementation starts.

We need a v2 foundation that makes the data shape explicit: provider observations, per-connection sync state, existing-window snapshots, diff/dedup plans, merge policy, and apply results. This foundation should let later chunks migrate monobank and PKO/Enable Banking without silently overwriting user edits or hiding ambiguous matching behavior.

## What Changes

- Add new finance domain types for provider identities, provider observations, provider sync windows, sync state, sync runs, sync stats, and sync issues.
- Add a new `finance/internal/providers` package that defines v2 connector, profile, fetch, snapshot, diff, apply, and sync-coordinator contracts with stub behavior and documentation comments.
- Treat provider observations as distinct from ledger transactions so provider-reported data can be diffed before persistence writes happen.
- Define the v2 matching policy so strong matches update existing provider-synced transactions, while weak or ambiguous matches create new transactions and increment diagnostics.
- Define the v2 merge policy so provider-original fields are updated, but user-facing transaction fields are preserved when they differ from the previous provider-original values.
- Keep PKO modeled as the product-level provider `pko` while Enable Banking remains the technical connector `enable-banking`.
- Preserve existing v1 provider sync behavior; this change establishes the foundation only and does not migrate live providers.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Define the provider sync v2 foundation for normalized provider observations, per-connection sync state, diff/dedup planning, conservative ambiguous matching, and provider/connector separation.

## Impact

- Affects `finance/domain` by adding v2 provider sync domain data structures.
- Affects `finance/internal/providers` by introducing provider sync v2 contracts and documented stubs.
- Does not change current finance persistence behavior except where compile-time type definitions require supporting code organization.
- Does not change current app routes, UI flows, jobs, or provider behavior.
- Creates groundwork for later provider migration chunks.
