---
type: Historical record
title: "2026-09-11-add-monobank-snapshot-fx-transfer-matching: spec"
description: "Historical spec record for 2026-09-11-add-monobank-snapshot-fx-transfer-matching."
status: deprecated
superseded_by: /requirements/transfer-matching.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-09-11-add-monobank-snapshot-fx-transfer-matching/specs/transfer-matching/spec.md"
    title: "Original openspec/changes/archive/2026-09-11-add-monobank-snapshot-fx-transfer-matching/specs/transfer-matching/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## MODIFIED Requirements

### Requirement: Scoped Monobank Snapshot FX Candidate Rule
The transfer matcher SHALL use stored Monobank transaction snapshots only as a
third fixed candidate rule and SHALL merge its candidates with same-currency and
description-derived FX candidates before mutual uniqueness.

#### Scenario: Reciprocal snapshots qualify
- **WHEN** two eligible rows on different accounts and within 72 elapsed hours
  share one unambiguous Monobank connection and their validated operation
  currencies and negated operation amounts reciprocate their ledger currencies
  and amounts
- **THEN** the matcher MUST add them as snapshot FX candidates regardless of
  descriptions or provider transaction IDs.

#### Scenario: Evidence fails closed
- **WHEN** connection provenance, snapshot count, snapshot JSON, MCC,
  currency-code, amount/sign, provider-original, current-ledger, or negation
  requirements are not met
- **THEN** the snapshot rule MUST produce no evidence and MUST NOT disable the
  other two candidate rules.

#### Scenario: Existing storage is reused
- **WHEN** this rule loads matching evidence
- **THEN** it MUST use grouped existing provider-match and snapshot projections
  with one row per transaction
- **AND** it MUST add no schema, migration, backfill, resync, API, UI, job,
  scheduling, reporting, provider-call, or separate evidence-persistence change.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
