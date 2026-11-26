---
type: Historical record
title: "2026-09-11-add-monobank-snapshot-fx-transfer-matching: proposal"
description: "Historical proposal record for 2026-09-11-add-monobank-snapshot-fx-transfer-matching."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-11-add-monobank-snapshot-fx-transfer-matching/proposal.md"
    title: "Original openspec/changes/archive/2026-09-11-add-monobank-snapshot-fx-transfer-matching/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

Monobank records deterministic cross-currency account movements as reciprocal
ledger and operation amounts. They remain unmatched when descriptions and
provider transaction IDs differ, even though current stored snapshots contain
the narrow evidence needed to prove a pair.

## What Changes

- Add a third automatic candidate rule based on existing Monobank transaction
  snapshots, current ledger values, provider-original values, and unambiguous
  connection provenance.
- Project optional connector, provider-original, and exactly-one snapshot data
  through the existing one-row transfer-matching load.
- Validate the exact reciprocal operation-currency and amount relationship in
  memory, then combine it with existing candidates before mutual uniqueness.
- Document the scoped evidence and no-schema external-contract boundary.

## Non-Goals

- Schema, migration, backfill, resync, API, UI, job, scheduling, reporting,
  provider-call, or separate evidence-persistence changes.
- Rates, tolerances, descriptions, names, receipts, IDs, or external lookup.

## Impact

- Finance compact matching projection, Monobank internal extractor, matching
  service, focused PostgreSQL tests, and transfer-matching documentation.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
