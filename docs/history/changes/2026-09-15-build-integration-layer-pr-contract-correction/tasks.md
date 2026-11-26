---
type: Historical record
title: "2026-09-15-build-integration-layer-pr-contract-correction: tasks"
description: "Historical tasks record for 2026-09-15-build-integration-layer-pr-contract-correction."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-15-build-integration-layer-pr-contract-correction/tasks.md"
    title: "Original openspec/changes/archive/2026-09-15-build-integration-layer-pr-contract-correction/tasks.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## 1. Canonical Direct-Client Contract Restoration

- [x] 1.1 Restore the complete `direct-finance-cli` modified requirements in
  this linked active correction change; first compare the two archived deltas
  against the intended complete requirements, then retain the two original
  transport scenarios plus redirect validation and the three original job-wait
  scenarios plus the in-flight deadline scenario, without changing source or
  either archive; run `openspec validate
  build-integration-layer-pr-contract-correction --strict` and verify the
  canonical spec is unchanged before archive.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
