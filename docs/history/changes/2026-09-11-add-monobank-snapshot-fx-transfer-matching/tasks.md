---
type: Historical record
title: "2026-09-11-add-monobank-snapshot-fx-transfer-matching: tasks"
description: "Historical tasks record for 2026-09-11-add-monobank-snapshot-fx-transfer-matching."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-11-add-monobank-snapshot-fx-transfer-matching/tasks.md"
    title: "Original openspec/changes/archive/2026-09-11-add-monobank-snapshot-fx-transfer-matching/tasks.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## 1. Snapshot projection and evidence

- [x] 1.1 Extend the compact transfer-matching projection with unambiguous
  connector, provider-original, and exactly-one transaction snapshot fields.
  Follow TDD flow: add PostgreSQL behavior tests for tenant scope, no duplicate
  rows, missing/multiple snapshots, and conflicting provenance, then implement
  grouped projections without generated-SQL assertions.
- [x] 1.2 Add a Monobank snapshot evidence extractor. Follow TDD flow: cover
  debit/credit reciprocity inputs, unknown connector/currency, missing/malformed
  fields, zero values, MCC rules, edits, same currency, signs, and int64
  negation boundaries before implementation.

## 2. Matching and documentation

- [x] 2.1 Add the third candidate index and combined mutual-uniqueness decision.
  Follow TDD flow: cover the supplied UAH/EUR pair, shuffled input, either
  starting leg, rejections, reciprocal ambiguity, cross-rule ambiguity, and
  atomic pair persistence before implementation.
- [x] 2.2 Update the Phase 0 PRD, design, and transfer-matching specification
  with the narrow snapshot rule and unchanged external/schema scope.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
