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
