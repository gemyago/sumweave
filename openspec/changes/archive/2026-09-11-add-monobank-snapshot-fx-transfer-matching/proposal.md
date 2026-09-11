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
