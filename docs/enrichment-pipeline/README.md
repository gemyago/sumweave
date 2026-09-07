# Enrichment Pipeline

This folder contains product requirements and system designs for transaction enrichment.

The enrichment pipeline is intended to cover two major parts:
1. Transactions classification (categories and maybe tags)
2. Transfer transactions matching

## Design documents

- [Classification Phase 0 PRD](classification-phase0-prd.md)
- [Classification Phase 0 system design](classification-phase0-design.md) — draft
  API contracts, database schema, execution model, and decisions for discussion.
- [Transfer Matching Phase 0 PRD](transfer-matching-phase0-prd.md) — implemented
  through automated chunks; automatic pairing rules, sync and explicit-run
  behavior, and manual correction.
- [Transfer Matching Phase 0 system design](transfer-matching-phase0-design.md) —
  implemented design; one bulk load, in-memory matching, atomic pair writes,
  exact schema change, API, and independent sync-event execution. The isolated
  [manual E2E guide](../manual-e2e/finance-transfer-matching-e2e.md) records the
  remaining manual verification gates.
