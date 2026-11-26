---
type: Historical record
title: "2026-09-06-correct-classification-pr-review-findings: proposal"
description: "Historical proposal record for 2026-09-06-correct-classification-pr-review-findings."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-06-correct-classification-pr-review-findings/proposal.md"
    title: "Original openspec/changes/archive/2026-09-06-correct-classification-pr-review-findings/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

PR #13 review found three correctness gaps in the completed Phase 0 transaction
classification flow: transient category reads are acknowledged as terminal,
classification tenant denials become server errors, and a replacement manual-rule
offer can retain stale form defaults. These focused corrections are needed before
the submitted change can be accepted.

## What Changes

- Keep actual unavailable classification-category states terminal, while returning
  transient category lookup failures as ordinary errors so appdispatch can retry
  and dead-letter them normally.
- Map an explicit classification submission's tenant access denial to the standard
  empty `401 Unauthorized` finance response.
- Refresh optional manual-rule form defaults when a new offer replaces the current
  offer, while preserving operator edits for the lifetime of the same offer.
- Add focused regressions for each corrected boundary and verify each corresponding
  PR #13 review thread after implementation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `transaction-classification`: Clarify retryable category lookup failures and the
  explicit classification tenant-denial HTTP contract.
- `finance-operator-ui`: Clarify draft refresh and edit preservation when a
  manual-assignment rule offer is replaced.

## Impact

- Finance classification execution and its focused service tests.
- Backend finance controller error mapping and registered-route tests.
- Optional rule-creation form/owners, focused Svelte tests, and the Finance UI
  wireframe's offer-lifetime behavior note.
- No schema, endpoint shape, generated API, dependency, or migration change.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
