---
type: Historical record
title: "2026-06-24-add-provider-sync-v2-foundation: proposal"
description: "Historical proposal record for 2026-06-24-add-provider-sync-v2-foundation."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-24-add-provider-sync-v2-foundation/proposal.md"
    title: "Original openspec/changes/archive/2026-06-24-add-provider-sync-v2-foundation/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

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

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
