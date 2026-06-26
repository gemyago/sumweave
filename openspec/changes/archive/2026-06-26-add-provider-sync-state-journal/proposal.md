## Why

The sync orchestrator is ready and already depends on a `SyncStateJournal`, but finance persistence still has no provider sync v2 journal implementation behind that seam. Without a real succeeded-state journal, the orchestrator cannot persist resumable coverage, and later window-executor work would have to invent storage around a contract that already exists.

## What Changes

- Add provider sync v2 succeeded-state journal persistence for append-only snapshots scoped to one bank connection.
- Add a dedicated finance sync journal store component that appends a succeeded sync state snapshot and loads the latest succeeded snapshot for one connection.
- Persist the sync-state fields the orchestrator already produces, including attempt/success timestamps, successful window coverage, run/job identifiers, aggregate stats, and lossless null handling for optional fields.
- Keep the orchestrator's `SyncStateJournal` seam backed by a dedicated finance persistence component rather than expanding a broader store into another cross-cutting responsibility.
- Keep the scope limited to succeeded-state journal persistence; v2 run persistence, failure journaling, candidate-window loading, and atomic apply remain separate follow-up work.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `finance-management`: make provider sync v2 succeeded-state journal persistence explicit for orchestrator resume behavior

## Impact

- Affects `finance/persistence` with a new provider sync state journal model, migration coverage, a dedicated journal store component, and focused tests.
- Affects `finance/internal/providers` only through runtime wiring of the existing `SyncStateJournal` seam to that dedicated journal store component.
- Does not change finance HTTP APIs, UI flows, bank-linking flows, provider fetch behavior, or the legacy v1 sync path.
