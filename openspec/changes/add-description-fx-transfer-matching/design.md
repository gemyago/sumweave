## Context

`TransferMatchingService` currently loads one immutable tenant slice through `TransferPairStore`, indexes equal-and-opposite same-currency amounts, applies mutual uniqueness, and reuses atomic pair writes. The compact row excludes descriptions and provider provenance. Existing provider transaction-match rows associate a ledger transaction with a bank connection, while the current ledger description already contains user edits and normalized remittance text.

The change stays inside `finance/` and retains both existing triggers. There are no API, UI, dispatch, job, or schema changes. The transfer-matching PRD and system design must be revised because they currently exclude descriptions and cross-currency matching.

## Goals / Non-Goals

**Goals:**

- Recognize deterministic two-leg internal FX exchanges from the current ledger description and current ledger values.
- Keep extraction isolated so a compatible description format changes only extraction and its tests.
- Evaluate ambiguity across same-currency and FX candidates without rule priority or input-order effects.
- Reuse the existing eligibility, range, trigger, correction, persistence, and reporting behavior.

**Non-Goals:**

- Structured provider exchange-rate fields, provider snapshots, network or market-rate lookup, or resync.
- Fee inference, amount tolerance beyond quote-minor-unit rounding, split transfers, confidence scoring, or fuzzy parsing.
- A parser registry, configurable rules, public contract changes, persistence changes, or new concurrency controls.

## Decisions

### Extract one typed evidence value per loaded description

Add a small finance-owned extractor selected by recognized text shape. Its first shape is anchored to the complete ASCII form:

```text
FX<digits> <BASE>/<QUOTE> <positive-rate>
```

`BASE` and `QUOTE` are three uppercase letters. The rate accepts digits with an optional comma or dot fractional part, is parsed as an exact positive decimal, and is rejected when zero or malformed. The result contains namespace `FX`, the deal reference, base and quote currency codes, and a canonical exact coefficient/scale representation with insignificant trailing fractional zeros removed. Thus comma and dot spellings and values such as `4.125` and `4.12500` compare by mathematical value. No evidence is returned for unknown text, same-currency pairs, invalid currency codes, or invalid rates.

Extraction runs once per loaded transaction before indexes are built. Candidate code consumes only the typed evidence and does not know the description syntax. A later compatible format is another branch in this extractor, returning the same evidence shape; no interface hierarchy or registry is introduced.

### Extend the compact load without duplicating ledger rows

Extend `persistence.TransferMatchingTransaction` with the current ledger `Description` and optional unambiguous `ConnectionID`. The existing single tenant-scoped, 144-hour-extended query remains the only matching load and preserves all eligibility predicates.

The query left-joins an aggregate of `finance_provider_transaction_matches` by ledger transaction ID. Before grouping, that aggregate joins `finance_bank_connections` by connection ID and restricts connections to the requested tenant. This makes the provenance scan use the existing tenant and connection indexes instead of grouping the database-wide match table on every attempt. Exactly one distinct tenant-owned connection ID yields usable provenance; no mapping or multiple distinct connection IDs yields no connection. Group or aggregate before projection so every eligible ledger transaction still appears exactly once. Missing, foreign-tenant, or conflicting provenance does not remove the row and therefore cannot alter same-currency matching. No schema or index change is required.

### Produce candidate sets from two rules, then decide once

Build the existing same-currency amount index and an FX evidence index from the unchanged loaded rows. For each row, union and deduplicate candidates from:

1. The unchanged equal-and-opposite same-currency rule.
2. The FX rule described below.

Apply the existing different-account and inclusive 72-hour checks in both rules. Mutual uniqueness operates on each leg's combined candidate set: accept only when each has exactly one candidate and those candidates are each other. Do not remove candidates while deciding, and complete all decisions before writes. Stable ID ordering and pair deduplication retain input-order independence. At least one accepted leg must remain inside the original half-open request range.

### Match scoped references and exact converted amounts

Two rows are FX candidates only when both have evidence and:

- connection ID, reference namespace, reference, ordered currency pair, and exact decimal rate agree;
- base and quote currencies are distinct, and the two ledger currencies cover that pair exactly;
- signs are opposite, accounts differ, and effective instants are within 72 hours; and
- the absolute quote amount equals the rounded conversion of the absolute base amount.

The ordered evidence pair determines which amount is base and which is quote regardless of which leg is the debit. Use checked arbitrary-precision integer arithmetic over the decimal coefficient, rate scale, and each ISO currency's standard minor-unit scale:

```text
quoteMinor = round(baseMinor * rateCoefficient * 10^quoteScale
                   / (10^rateScale * 10^baseScale))
```

Round an exact halfway result upward because amounts are compared as positive magnitudes. Resolve ISO codes and standard minor-unit scales through `golang.org/x/text/currency`, promoted to a direct finance dependency. Unknown currencies, unavailable scales, or any inconsistent evidence fail closed for the FX rule. No floating-point conversion or persisted extracted value is used.

### Preserve existing operational behavior

The existing automatic bank-sync-window consumer and explicit observed command both call the unchanged service entrypoint, so existing transactions participate immediately on the next explicit matching run. Accepted FX pairs use `LinkTransferPair` unchanged; category, tag, description, provenance, and reporting behavior are therefore preserved. Existing Phase 0 loaded-data concurrency risk remains accepted.

Update `docs/enrichment-pipeline/transfer-matching-phase0-prd.md` and `transfer-matching-phase0-design.md` to describe the two-rule candidate set, extraction and provenance projection, exact conversion, shared ambiguity, and unchanged external/schema scope.

## Risks / Trade-offs

- **A coincidental shared description can be false evidence** -> Require agreement on connection, namespace, reference, ordered pair, rate, direction, amount conversion, account separation, time window, and mutual uniqueness.
- **Conflicting provenance could multiply or mis-scope candidates** -> Aggregate distinct connection IDs in the bulk read, preserve one ledger row, and disable only FX matching when provenance is not unique.
- **Decimal overflow or floating-point drift could create a wrong match** -> Parse decimal digits exactly and perform conversion with arbitrary-precision integers.
- **A later transaction can reveal ambiguity after a pair was saved** -> Retain the documented Phase 0 loaded-snapshot and manual-correction behavior.
- **The initial parser is deliberately narrow** -> Reject unknown text and extend only the extractor when another concrete format is supported.

## Migration Plan

Deploy the finance code and documentation together. No database migration, data backfill, provider resync, or API rollout is required. Existing unmatched rows become eligible for the FX rule when an automatic or explicit match attempt next includes them. Rollback removes the FX candidate rule without changing already linked pairs, which remain correctable through existing manual unlinking.

## Open Questions

None. The issue defines the initial syntax, provenance scope, arithmetic, ambiguity, and Phase 0 boundaries.
