---
type: Historical record
title: "2026-06-26-add-provider-sync-state-journal: proposal"
description: "Historical proposal record for 2026-06-26-add-provider-sync-state-journal."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-26-add-provider-sync-state-journal/proposal.md"
    title: "Original openspec/changes/archive/2026-06-26-add-provider-sync-state-journal/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

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

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
