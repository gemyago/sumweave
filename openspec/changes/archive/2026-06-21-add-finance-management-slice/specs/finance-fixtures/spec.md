## ADDED Requirements

### Requirement: Finance Fixture Generation CLI
The backend application SHALL provide a finance fixture-generation command through the existing `signal-foundry` binary.

#### Scenario: Fixture generation is deterministic and service-backed
- **WHEN** a developer runs `signal-foundry finance fixtures generate ...` with a seed and bounded date range
- **THEN** the command MUST generate deterministic finance fixture data through finance application/domain services rather than direct table writes
- **AND** the command MUST be suitable for local development, smoke testing, and automated finance e2e setup

### Requirement: Reusable Finance Fixture Scenarios
The finance module SHALL expose reusable scenario generation helpers for tests and local validation.

#### Scenario: Fixture scenarios cover realistic finance workflows
- **WHEN** fixture scenarios are generated
- **THEN** they MUST cover one or more tenants, multiple accounts and currencies, categories, tags, regular income/expense transactions, pending and booked items, refunds, matched and unmatched transfers, reconciliations, hidden/deleted records, user-edited provider-original shapes, FX-rate records, and representative finance job states

#### Scenario: Automated tests can reuse fixture scenarios without shelling out
- **WHEN** finance integration or e2e tests need realistic starting data
- **THEN** they MUST be able to call reusable scenario functions from `finance/` directly instead of requiring CLI shell execution
