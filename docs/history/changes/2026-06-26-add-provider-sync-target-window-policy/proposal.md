---
type: Historical record
title: "2026-06-26-add-provider-sync-target-window-policy: proposal"
description: "Historical proposal record for 2026-06-26-add-provider-sync-target-window-policy."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-26-add-provider-sync-target-window-policy/proposal.md"
    title: "Original openspec/changes/archive/2026-06-26-add-provider-sync-target-window-policy/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

Provider sync v2 orchestration already depends on a `TargetWindowPolicy`, but the repo currently only has the interface and test mocks. There is no concrete implementation in `finance/internal/providers/`, so the orchestrator still cannot decide what window to sync outside tests.

The current architecture doc defines a rolling 30-day refresh idea, and the archived attempt-journal change deliberately moved latest-state interpretation into the policy seam. We now need to make the first concrete planning rule explicit: initial syncs should backfill much further, while subsequent syncs should either refresh the last 30 days or catch up from an older prior checkpoint.

## What Changes

- Add the first finance-owned provider-sync target-window policy implementation in `finance/internal/providers/`.
- Define the first planning rule set:
  - no prior state -> attempt to fetch the last 3 years ending at the planning time
  - prior checkpoint less than 30 days old -> fetch the last 30 days ending at the planning time
  - prior checkpoint older than 30 days -> fetch from that checkpoint until the planning time
- Keep latest-state interpretation explicit so the policy can derive that checkpoint from the latest journal row.
- Define bounded planning failure behavior for invalid persisted state windows instead of silently fabricating a target window.
- Update the finance-management OpenSpec delta so this behavior is an explicit acceptance criterion rather than only an architecture note.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `finance-management`: define and implement the first concrete provider sync v2 target-window planning policy

## Impact

- Affected code: `finance/internal/providers/` target-window planning logic and its focused tests.
- Affected docs/specs: provider sync planning expectations in OpenSpec and adjacent finance sync documentation.
- Out of scope: chunk splitting policy, window executor behavior, run persistence, and job or API wiring.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
