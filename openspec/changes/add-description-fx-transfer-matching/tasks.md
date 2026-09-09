## 1. Matching Input And Provenance

- [x] 1.1 Extend the compact transfer-matching row and bulk query with the current description and one optional unambiguous connection ID. Follow TDD flow: first add focused PostgreSQL behavior cases for one mapping, no mapping, repeated same-connection mappings, and conflicting distinct connections while proving eligibility and one-row-per-transaction behavior remain unchanged; then implement the aggregate provenance projection and verify the finance module tests.

## 2. FX Evidence And Exact Conversion

- [x] 2.1 Add the typed FX evidence extractor and currency-aware exact conversion predicate. Follow TDD flow: first cover the complete `FX<digits> <BASE>/<QUOTE> <rate>` syntax, comma and dot rates, equivalent values with trailing zeros, malformed/unknown text, invalid or equal currencies, nonpositive rates, both debit directions, quote-minor rounding including halfway behavior, differing currency scales, and large values; then implement extraction plus arbitrary-precision conversion and make `golang.org/x/text/currency` a direct finance dependency.

## 3. Combined Candidate Decisions And Documentation

- [x] 3.1 Integrate FX candidates into the existing immutable transfer-matching decision. Follow TDD flow: first cover the illustrated USD/PLN pair, mismatched scope/reference/rate/pair/value cases, missing or conflicting provenance, same-currency regression, ambiguity spanning both rules, reverse-candidate ambiguity outside the requested range, stable shuffled input, extraction once per row, range scope, and unchanged atomic saves; then implement combined candidate union/deduplication and shared mutual uniqueness without changing triggers or pair persistence.
- [x] 3.2 Update the Phase 0 transfer-matching PRD and system design to match the implemented two-rule behavior. Follow TDD flow through the preceding executable acceptance coverage before revising the documents; remove superseded description/cross-currency exclusions and document extraction, provenance scope, exact arithmetic, shared ambiguity, unchanged schema and external contracts, and implementation status.
