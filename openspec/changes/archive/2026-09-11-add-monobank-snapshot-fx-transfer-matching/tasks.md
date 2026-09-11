## 1. Snapshot projection and evidence

- [x] 1.1 Extend the compact transfer-matching projection with unambiguous
  connector, provider-original, and exactly-one transaction snapshot fields.
  Follow TDD flow: add PostgreSQL behavior tests for tenant scope, no duplicate
  rows, missing/multiple snapshots, and conflicting provenance, then implement
  grouped projections without generated-SQL assertions.
- [x] 1.2 Add a Monobank snapshot evidence extractor. Follow TDD flow: cover
  debit/credit reciprocity inputs, unknown connector/currency, missing/malformed
  fields, zero values, MCC rules, edits, same currency, signs, and int64
  negation boundaries before implementation.

## 2. Matching and documentation

- [x] 2.1 Add the third candidate index and combined mutual-uniqueness decision.
  Follow TDD flow: cover the supplied UAH/EUR pair, shuffled input, either
  starting leg, rejections, reciprocal ambiguity, cross-rule ambiguity, and
  atomic pair persistence before implementation.
- [x] 2.2 Update the Phase 0 PRD, design, and transfer-matching specification
  with the narrow snapshot rule and unchanged external/schema scope.
