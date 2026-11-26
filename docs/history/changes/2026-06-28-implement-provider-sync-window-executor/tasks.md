---
type: Historical record
title: "2026-06-28-implement-provider-sync-window-executor: tasks"
description: "Historical tasks record for 2026-06-28-implement-provider-sync-window-executor."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-28-implement-provider-sync-window-executor/tasks.md"
    title: "Original openspec/changes/archive/2026-06-28-implement-provider-sync-window-executor/tasks.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## 1. OpenSpec And Terminology

- [x] 1.1 Update the finance provider sync architecture doc and finance-management spec wording so the existing `candidate window` concept is clarified as the persisted snapshot lookup window, and must follow TDD flow where applicable by first updating any doc-linked expectations or terminology checks before applying the wording changes.

## 2. Executor Contracts

- [x] 2.1 Define `SnapshotWindowPolicy` and `SyncRepository` executor seams in `finance/internal/providers`, and must follow TDD flow by first expanding focused contract tests where needed before introducing the new interfaces and their expected responsibilities.
- [x] 2.2 Add the first concrete snapshot policy implementation as a requested-window passthrough policy, and must follow TDD flow by first adding focused policy tests for valid passthrough behavior before implementing that minimal default.

## 3. Window Executor Flow

- [x] 3.1 Implement requested-window execution in a follow-up step as fetch -> snapshot load -> diff -> apply-plan -> apply handoff -> result reporting, and must follow TDD flow by first adding failing executor tests that prove diff/apply planning outputs are passed into storage and surfaced through the result before implementing the executor flow.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
