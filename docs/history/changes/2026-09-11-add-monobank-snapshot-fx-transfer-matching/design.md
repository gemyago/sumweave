---
type: Historical record
title: "2026-09-11-add-monobank-snapshot-fx-transfer-matching: design"
description: "Historical design record for 2026-09-11-add-monobank-snapshot-fx-transfer-matching."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-11-add-monobank-snapshot-fx-transfer-matching/design.md"
    title: "Original openspec/changes/archive/2026-09-11-add-monobank-snapshot-fx-transfer-matching/design.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Context

The matcher already makes immutable in-memory decisions from one expanded,
uncapped tenant load. Existing provider matches identify a bank connection,
transactions retain provider-original values, and Monobank sync persists raw
transaction snapshots. This change reuses those facts without storing new
matching evidence.

## Design

The bulk projection aggregates provider matches by transaction before joining,
returning connection ID and connector only when exactly one tenant-owned
connection exists. It separately aggregates transaction snapshots by transaction
and connection, returning JSON only for exactly one tenant-owned transaction
snapshot. It joins the aggregate rather than snapshots directly, preserving one
row per transaction. Conflicting provenance clears all provider evidence;
missing or multiple snapshots clear only snapshot JSON.

The Monobank extractor decodes only `amount`, `operationAmount`, `currencyCode`,
`mcc`, and optional `originalMcc`. It accepts connector `monobank`, nonzero
same-sign amounts, MCC 4829, recognized operation currency distinct from the
current ledger currency, exact provider-original/snapshot equality, unchanged
current ledger amount/currency, and safe negations.

The matcher indexes usable evidence by connection ID, ledger currency, and
signed ledger amount. A row derives its counterpart key from operation currency
and negated operation amount. Candidate validation requires the reciprocal
operation relationship, distinct accounts, and the existing inclusive 72-hour
window. Candidates merge and deduplicate with same-currency and description FX
candidates before the existing reverse mutual-uniqueness decision and atomic
`LinkTransferPair` persistence.

No schema, migration, backfill, resync, API, UI, job, scheduling, reporting,
provider-call, or separate evidence persistence change is required.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
