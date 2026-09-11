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
