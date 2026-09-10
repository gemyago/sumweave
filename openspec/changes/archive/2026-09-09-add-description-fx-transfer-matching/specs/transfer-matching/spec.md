## MODIFIED Requirements

### Requirement: Shared Automatic Transfer Eligibility
Automatic and explicit transfer matching SHALL use one finance-owned range service and SHALL consider only loaded tenant transactions that satisfy the Phase 0 eligibility rule.

#### Scenario: Eligible transaction is loaded
- **WHEN** a visible booked transaction belongs to a visible account in the selected tenant, has kind `regular`, `expense`, `income`, or `transfer`, has a nonzero amount, has neither transfer group nor matching timestamp, and is not excluded from automatic matching
- **THEN** the matching load MUST include its ID, account ID, currency, signed minor amount, effective timestamp, current ledger description, and optional unambiguous bank-connection provenance regardless of manual, CSV, or provider source and regardless of category.

#### Scenario: Ineligible transaction is excluded
- **WHEN** a transaction or account is missing, hidden, deleted, or outside the tenant, or the transaction is pending, zero amount, excluded, already paired, a refund, reconciliation, opening balance, or system kind
- **THEN** the matching load MUST exclude it
- **AND** query-excluded rows MUST NOT contribute to attempt outcome counts.

#### Scenario: Existing unmatched transfer is eligible
- **WHEN** a booked visible `transfer` transaction has neither `transferGroupId` nor `transferMatchedAt` and satisfies every other eligibility condition
- **THEN** automatic matching MUST treat it like the other eligible kinds.

#### Scenario: Connection provenance is unavailable or conflicting
- **WHEN** an otherwise eligible transaction has no provider transaction mapping or mappings to more than one distinct bank connection
- **THEN** the load MUST retain exactly one matching row with no usable connection provenance
- **AND** the row MUST remain eligible for the same-currency rule but MUST NOT form an FX candidate.

### Requirement: Fixed Mutual-Uniqueness Matching Rule
The transfer matcher SHALL combine candidates from the equal-and-opposite same-currency rule and the scoped description-derived FX rule, and SHALL accept a pair only when different-account rows lie no more than 72 elapsed hours apart and are each other's only eligible counterpart across the combined candidate set.

#### Scenario: Unique equal-and-opposite pair qualifies
- **WHEN** two eligible same-tenant transactions are on different visible accounts, use the same currency, have exact opposite nonzero minor amounts, are no more than 72 elapsed hours apart inclusive, and neither has another eligible counterpart under either rule
- **THEN** the matcher MUST accept that pair regardless of descriptions, categories, sources, provider data, or which signed leg appears first.

#### Scenario: Same-currency rule rejects a candidate
- **WHEN** otherwise eligible rows have unequal absolute amounts, occur more than 72 elapsed hours apart, share an account, or require negating the minimum signed 64-bit amount
- **THEN** they MUST NOT be same-currency counterparts.

#### Scenario: Ambiguity is mutual across rules
- **WHEN** either proposed leg has more than one eligible counterpart across the same-currency and FX rules in its full 72-hour window
- **THEN** the matcher MUST leave the proposal unchanged rather than selecting by rule, input order, first occurrence, or proximity
- **AND** one-to-many and many-to-one cases MUST be rejected symmetrically.

#### Scenario: Decisions are deterministic
- **WHEN** the same complete loaded input is supplied in any row order
- **THEN** the matcher MUST produce the same stable deduplicated pair set
- **AND** it MUST finish all pair decisions against the unchanged loaded input before saving any pair.

## ADDED Requirements

### Requirement: Description-Derived FX Evidence
The transfer matcher SHALL extract optional typed FX evidence once from each loaded current ledger description and SHALL keep description syntax separate from generic candidate decisions.

#### Scenario: Initial FX description format is recognized
- **WHEN** a complete description has form `FX<digits> <BASE>/<QUOTE> <rate>`, with distinct uppercase three-letter ISO currencies and a positive decimal rate using either comma or dot as its separator
- **THEN** extraction MUST return namespace `FX`, the deal reference, ordered base and quote currencies, and the mathematically exact decimal rate
- **AND** equivalent decimal spellings such as `4.125`, `4.12500`, and `4,12500` MUST compare as the same rate
- **AND** the rate MUST mean quote-currency units per one base-currency unit.

#### Scenario: Description has no usable FX evidence
- **WHEN** the description has an unknown shape, is incomplete or malformed, uses invalid or equal currencies, or contains a zero or invalid rate
- **THEN** extraction MUST return no FX evidence without failing the matching attempt
- **AND** the row MUST remain available to the existing same-currency rule.

#### Scenario: A compatible format is added later
- **WHEN** another description format can provide the same namespace, reference, ordered currency pair, and exact rate
- **THEN** support MUST require changes only within extraction and its tests
- **AND** candidate, account, currency, amount, uniqueness, persistence, and trigger behavior MUST remain generic.

### Requirement: Scoped Exact FX Candidate Rule
The transfer matcher SHALL recognize a cross-currency candidate only from matching scoped description evidence and exact current-ledger conversion at the quote currency's standard minor-unit precision.

#### Scenario: Unique internal FX exchange qualifies
- **WHEN** two eligible opposite-sign rows on different accounts and within 72 elapsed hours have the same bank connection, namespace, reference, ordered distinct currency pair, and exact rate; their ledger currencies cover that pair; and the quote amount equals the rounded base amount multiplied by the rate
- **THEN** the matcher MUST add them as FX candidates regardless of which currency is debited
- **AND** the ordered evidence pair MUST determine conversion direction.

#### Scenario: FX conversion uses exact currency-aware arithmetic
- **WHEN** the matcher checks FX amounts
- **THEN** it MUST use exact decimal and integer arithmetic with each ISO currency's standard minor-unit scale, rounding an exact halfway positive magnitude upward to quote minor units
- **AND** it MUST NOT use binary floating-point arithmetic.

#### Scenario: FX evidence disagrees
- **WHEN** candidate rows differ by bank connection, namespace, reference, currency pair, or exact rate; have same-sign values; do not cover the two evidence currencies; or have an amount inconsistent after quote-minor-unit rounding
- **THEN** those rows MUST NOT be FX counterparts.

#### Scenario: Reference scope prevents collision
- **WHEN** otherwise identical FX references occur in different bank connections or reference namespaces
- **THEN** the matcher MUST keep their candidates separate and MUST NOT create a cross-scope pair.
