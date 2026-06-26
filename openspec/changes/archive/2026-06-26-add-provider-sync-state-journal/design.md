## Context

`finance/internal/providers/sync_orchestrator.go` now models sync progress as a succeeded-state journal. It loads the latest succeeded snapshot for one connection, plans a target window, executes chunk windows oldest first, and appends the next succeeded snapshot after each completed chunk.

That architecture is already documented in `docs/finance-provider-sync-architecture.md` and partially reflected in `openspec/specs/finance-management/spec.md`. The missing piece is persistence. Current finance storage still centers on legacy bank-sync metadata such as `finance_bank_connection_sync_runs`, connection-owned provider accounts, balance snapshots, raw payloads, and provider transaction matches. There is no v2 store model or load/append API for `domain.ProviderSyncState`.

The finance module also has constraints we should preserve:

- finance owns its own GORM models and auto-migration
- domain types stay separate from persistence models
- persistence should remain SQLite-local and PostgreSQL-oriented
- the current change should stay narrower than full provider sync v2 execution

## Goals / Non-Goals

**Goals:**

- Persist provider sync v2 succeeded-state snapshots in an append-only journal scoped to a bank connection.
- Support `LoadLastSucceededSyncState(ctx, connection)` returning the newest succeeded snapshot or `nil` when the connection has no journal state yet.
- Support `AppendSyncState(ctx, state)` round-tripping the current `domain.ProviderSyncState` shape without silently dropping optional fields.
- Keep the persistence shape narrow enough that a dedicated sync journal store can satisfy the orchestrator's consumer-defined `SyncStateJournal` seam without turning a broader store into a catch-all component.
- Keep future executor and run-journal work unblocked without forcing that wider scope into this change.

**Non-Goals:**

- Persist `domain.ProviderSyncRun` or build a failed-run journal.
- Implement candidate-window snapshot loading, diff planning persistence, or atomic apply.
- Rework legacy `BankConnectionSyncRun` behavior.
- Add new finance API, UI, or durable-job wiring.

## Decisions

1. Use a dedicated append-only finance table for provider sync state snapshots.

   Add a finance-owned GORM model for succeeded sync state journal rows, using a plural finance-prefixed table name and explicit UTC timestamp columns. The table should be append-only from the application point of view: every succeeded chunk writes a new row instead of updating the prior row.

   Alternative considered: reuse `finance_bank_connections` mutable sync fields or expand `finance_bank_connection_sync_runs`. Rejected because the orchestrator contract and architecture explicitly require latest-succeeded snapshots that preserve earlier progress rather than one mutable row or a run-dedup table.

2. Scope rows by `connection_id` and reconstruct the `Connection` field from the caller's connection reference on load.

   The journal lookup already receives the canonical `domain.ProviderConnectionRef` argument. Persisting state rows by `connection_id` plus state-specific fields avoids duplicating provider, connector, provider reference, and external ID into every journal row while still returning a full `domain.ProviderSyncState` to the caller.

   Alternative considered: denormalize the full `ProviderConnectionRef` into each row. Rejected for now because it adds duplicate connection metadata without improving the orchestrator's current load or append behavior.

3. Store optional successful-window coverage as nullable start/end columns and aggregate stats as scalar columns.

   `SuccessfulWindow` should map to nullable `successful_window_start` and `successful_window_end` columns rather than JSON. Aggregate stats should map to one column per stat field. This keeps SQLite/PostgreSQL behavior straightforward, preserves queryability, and matches the fixed-shape nature of the current domain model.

   Alternative considered: persist the window or stats as JSON blobs. Rejected because the structure is stable, small, and better represented as explicit columns.

4. Add a dedicated sync journal store component instead of expanding `persistence.Store`.

   The repository rules explicitly discourage "god components", and the current `persistence.Store` already owns a wide range of finance persistence concerns. This change should therefore introduce a dedicated component such as `ProviderSyncStateJournalStore` or `SyncJournalStore` inside `finance/persistence` that owns only the v2 sync-state journal behavior while reusing the same database handle and migration model.

   The dedicated journal store should expose `LoadLastSucceededSyncState` and `AppendSyncState` with the existing orchestrator-facing signatures so it can satisfy `internal/providers.SyncStateJournal` without `internal/providers` importing finance persistence details.

   Alternative considered: add the methods directly on `persistence.Store`. Rejected because it keeps growing one broad component with unrelated responsibilities and fights the repo's component-boundary guidance.

5. Define "latest" by append order, implemented through descending journal-row creation time.

   The orchestrator cares about the newest appended succeeded snapshot, not the largest successful-window end or the largest `SucceededAt`. Load queries should therefore order by journal row creation timestamp descending, with a stable secondary tiebreaker, and return the first row for the requested connection.

   Alternative considered: order by `succeeded_at` or `successful_window_end`. Rejected because those are domain timestamps and coverage markers, not guaranteed append-order identifiers.

6. Keep the journal lossless for the current domain shape, even if some succeeded fields are expected to be empty.

   Persist all current `domain.ProviderSyncState` fields that can be represented in storage, including `ErrorSummary`, even though the orchestrator clears that field before appending succeeded snapshots today. This avoids hidden field loss and keeps the mapping aligned with the domain shape while later failure-oriented work is still undecided.

## Risks / Trade-offs

- [Append-only journal grows over time] -> Accept the growth for now because sync frequency is low and early-alpha compatibility concerns are intentionally relaxed; retention or compaction can be a later change if needed.
- [Ordering depends on journal-row creation time] -> Use store-controlled UTC timestamps and a deterministic secondary tiebreaker in tests so "latest" behavior stays explicit.
- [The journal only covers succeeded state, not failed attempts] -> Keep that limitation explicit in the proposal and spec so later run/failure persistence can be designed separately instead of leaking partial semantics into this change.
- [`finance_bank_connections` still has legacy mutable sync summary fields] -> Preserve them for existing v1 behavior and avoid widening this change into cross-flow sync metadata migration.

## Migration Plan

1. Add the new finance persistence model to `finance/persistence/models.go` and register it in `finance/persistence/migrations.go`.
2. Add dedicated journal store methods and focused tests for empty lookup, append/load round-trips, latest-row selection, connection isolation, null window handling, and database error wrapping.
3. Confirm through focused tests or compile coverage that the dedicated journal store can be used as the orchestrator `SyncStateJournal`.
4. Leave orchestrator runtime wiring, v2 run persistence, and window executor persistence for follow-up changes.

Rollback is removing the new journal model and dedicated journal store component before any dependent runtime wiring lands. Because finance uses GORM auto-migrate and the project is explicitly early alpha, this change does not need backward-compatibility planning.

## Open Questions

- Should later provider sync v2 work keep all succeeded snapshots forever, or eventually add journal compaction after the broader run/audit story exists?
- When `ProviderSyncRun` persistence lands, should sync state rows reference a dedicated run row, or should `RunID` remain a free string link across separate stores?
