---
type: Historical record
title: "2026-06-26-clarify-provider-sync-attempt-journal: proposal"
description: "Historical proposal record for 2026-06-26-clarify-provider-sync-attempt-journal."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-26-clarify-provider-sync-attempt-journal/proposal.md"
    title: "Original openspec/changes/archive/2026-06-26-clarify-provider-sync-attempt-journal/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

The current provider sync state journal was specified and implemented as a succeeded-state journal. That matches the original orchestrator shape, but it makes the state semantics muddy: the in-memory state passed through planning mixes "last successful checkpoint" fields with the current attempt metadata.

We want the journal to describe what actually happened on each sync attempt. Every chunk attempt has a known window, and the latest journal row should mean "latest state" rather than "latest succeeded checkpoint". That keeps one journal/table/component while making the row semantics easier to reason about.

## What Changes

- Redefine provider sync state journal rows as append-only per-attempt state records rather than succeeded-only snapshots.
- Rename the journal load seam from `LoadLastSucceededSyncState` to `LoadLastState` so it explicitly returns the newest appended state for one connection.
- Rename the sync-state field and persistence columns from success-specific window naming to neutral attempt-window naming, using `Window`, `window_start`, and `window_end`, and require them on every journal row because each attempt always has a concrete window.
- Update sync orchestration so target-window planning happens before chunk execution, and each chunk attempt appends a state row that records that chunk's exact window plus either success or failure outcome.
- Keep failed latest attempts visible in the same journal, and let target-window policy decide how the latest loaded state should influence the next plan.
- Keep a single journal store and single journal table; do not split checkpoint and attempt persistence into separate components.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `finance-management`: redefine provider sync v2 state journaling as a latest-attempt journal with explicit per-attempt windows and nullable success outcome

## Impact

- Affects `finance/domain`, `finance/internal/providers`, and `finance/persistence` because the state contract, orchestration flow, store method names, and journal columns all need to align.
- Affects provider sync architecture/spec documentation because current terminology assumes succeeded-only journal rows.
- Does not add new HTTP APIs, UI behavior, connector behavior, or legacy v1 sync changes.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
