---
type: Requirements
title: "Finance fixture requirements"
description: "Deterministic service-backed finance fixtures for local development and tests."
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: accepted-contract
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/specs/finance-fixtures/spec.md"
    title: "Accepted finance-fixtures scenarios at the documentation checkpoint"
  - id: current-topic
    resource: "/development/verification.md"
    title: "Current topic reference and implementation limits"
---

This document preserves the accepted requirements and granular scenarios for
this capability.[^accepted-contract] See the [current topic reference](../development/verification.md)
for delivered behavior and known limitations. A requirement states a contract;
it does not record a completed test run.[^current-topic]

# finance-fixtures Specification

## Purpose
Deterministic service-backed finance fixtures for local development and tests.
## Requirements
### Requirement: Finance Fixture Generation CLI
The backend application SHALL provide a finance fixture-generation command through the existing `sumweave` binary.

#### Scenario: Fixture generation is deterministic and service-backed
- **WHEN** a developer runs `sumweave finance fixtures generate ...` with a seed and bounded date range
- **THEN** the command MUST generate deterministic finance fixture data through finance application/domain services rather than direct table writes
- **AND** the command MUST be suitable for local development, smoke testing, and automated finance e2e setup

### Requirement: Reusable Finance Fixture Scenarios
The finance module SHALL expose reusable scenario generation helpers for tests and local validation.

#### Scenario: Fixture scenarios cover realistic finance workflows
- **WHEN** fixture scenarios are generated
- **THEN** the default realistic finance scenario MUST seed a deterministic full year of activity ending at the configured anchor period
- **AND** it MUST generate roughly 30-40 transactions per calendar month
- **AND** it MUST cover one or more tenants, multiple accounts and currencies, seeded default categories and tags in use, regular income and expense transactions, pending and booked items, refunds, matched and unmatched transfers, reconciliations, hidden or deleted records, user-edited provider-original shapes, FX-rate records, and representative finance job states

#### Scenario: Seeded reporting works immediately after fixture generation
- **WHEN** the default realistic fixture scenario is generated
- **THEN** the system MUST also persist the FX rates required by its seeded non-display-currency transactions for the seeded reporting windows
- **AND** finance dashboard and reporting flows for those seeded windows MUST NOT require a follow-up manual FX sync before returning display-currency totals

#### Scenario: Automated tests can reuse fixture scenarios without shelling out
- **WHEN** finance integration or e2e tests need realistic starting data
- **THEN** they MUST be able to call reusable scenario functions from `finance/` directly instead of requiring CLI shell execution

[^accepted-contract]: Preserved accepted scenario source; current delivery is described separately in the linked topic.

[^current-topic]: Current topic reference and implementation limits.
