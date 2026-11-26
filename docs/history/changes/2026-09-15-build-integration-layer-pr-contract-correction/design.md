---
type: Historical record
title: "2026-09-15-build-integration-layer-pr-contract-correction: design"
description: "Historical design record for 2026-09-15-build-integration-layer-pr-contract-correction."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-15-build-integration-layer-pr-contract-correction/design.md"
    title: "Original openspec/changes/archive/2026-09-15-build-integration-layer-pr-contract-correction/design.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Context

The original `build-integration-layer` change and its
`build-integration-layer-pr-corrections` follow-up are archived. The latter's
`## MODIFIED Requirements` delta restated only its new redirect and wait-deadline
scenarios. OpenSpec archive semantics replace the matching requirement with that
complete restatement, so the canonical `direct-finance-cli` spec lost five
unchanged scenarios.

## Decision

Create the smallest linked active correction change. Its direct-client delta
will restate each affected requirement in full: the unchanged Phase 1 scenarios
first, followed by the already-approved additive correction scenario. This
restores the canonical contract without changing its implementation or product
behavior.

## Non-Goals

- Do not modify either archived change or revise their historical artifacts.
- Do not change `swmd`, tests, API contracts, or release behavior.
- Do not reopen previously resolved threads or implement the two optional PR
  observations.

## Verification

Strictly validate this active change after its artifacts are complete. Before
archive, compare its two complete modified requirements with the archived
Phase 1 delta and the first correction delta to confirm all seven scenarios are
present exactly once: three transport scenarios and four observed-job-wait
scenarios. The canonical spec remains unchanged until archive.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
