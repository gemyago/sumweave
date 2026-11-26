---
type: Historical record
title: "2026-09-15-build-integration-layer-pr-contract-correction: proposal"
description: "Historical proposal record for 2026-09-15-build-integration-layer-pr-contract-correction."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-15-build-integration-layer-pr-contract-correction/proposal.md"
    title: "Original openspec/changes/archive/2026-09-15-build-integration-layer-pr-contract-correction/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

PR #28 has one new unresolved must-fix review thread at
`docs/requirements/direct-finance-cli.md:83`. Archiving the prior
`build-integration-layer-pr-corrections` delta replaced, rather than extended,
the affected requirements' scenario lists. The canonical direct-client spec
therefore no longer states already implemented and approved transport and
observed-job-wait behavior.

## What Changes

- Restore the retained non-loopback HTTP rejection and HTTPS verification/
  finite-timeout scenarios alongside the redirect-destination scenario.
- Restore the retained initial-404 grace, late-404 terminal, and terminal-job
  scenarios alongside the in-flight overall-deadline scenario.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `direct-finance-cli`: Correct the canonical requirement deltas so the prior
  Phase 1 scenarios and the first correction round's additive scenarios coexist.

## Impact

- OpenSpec only: one active change and the canonical `direct-finance-cli` spec
  when this change is later archived.
- No Go, UI, API, schema, runtime, documentation, archive, or GitHub changes.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
