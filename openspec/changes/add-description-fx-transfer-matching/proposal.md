## Why

Internal currency exchanges remain unmatched because automatic transfer matching only recognizes equal-and-opposite amounts in one currency. Bank feeds can omit structured exchange-rate data while retaining a shared deal reference, currency pair, and exact rate in each ledger description, leaving an otherwise deterministic pair to inflate income and expense reporting.

## What Changes

- Extract optional typed FX evidence from the existing ledger description, initially recognizing `FX<digits> <BASE>/<QUOTE> <rate>` with comma or dot decimal separators.
- Add an FX candidate rule for opposite-sign transactions on different accounts whose scoped reference evidence agrees and whose current ledger amounts satisfy the stated conversion at the quote currency's minor-unit precision.
- Scope description references to the same tenant, bank connection, and reference namespace by loading existing transaction provenance in bulk; missing or conflicting provenance disables only the FX rule for that row.
- Combine and deduplicate same-currency and FX candidates before the existing mutual-uniqueness decision so ambiguity across either rule remains unmatched and input order cannot select a winner.
- Preserve the existing eligibility, 72-hour window, range expansion, triggers, atomic pair writes, manual correction, category/tag preservation, inspection UI, and reporting behavior.
- Update the transfer-matching Phase 0 PRD and system design for the additional rule.
- Add no database schema, API, UI, job type, provider call, provider snapshot, or resync behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `transfer-matching`: Expand automatic matching with scoped, description-derived FX evidence and shared ambiguity handling across the existing and FX rules.

## Impact

- Finance matching service, compact matching projection, and dedicated pair-store query.
- Finance unit tests and focused PostgreSQL persistence behavior tests.
- Existing transfer-matching product requirements and system design documentation.
- No external contract or database schema changes; existing provider transaction-match provenance is reused.
