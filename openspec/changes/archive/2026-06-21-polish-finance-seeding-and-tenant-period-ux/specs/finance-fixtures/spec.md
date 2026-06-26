## MODIFIED Requirements

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
