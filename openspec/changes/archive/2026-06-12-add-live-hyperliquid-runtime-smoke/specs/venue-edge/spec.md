## ADDED Requirements

### Requirement: Opt-In Live Real Venue Smoke

The system SHALL support a manual opt-in live smoke path for supported public market-data venue adapters without making live venue access part of the default automated suite.

#### Scenario: Live smoke stays out of default test lanes

- **WHEN** normal repository or module automated test targets run
- **THEN** they MUST NOT require live venue network access and MUST NOT execute the manual live smoke path implicitly

#### Scenario: Live-tagged tests still compile in regular validation

- **WHEN** regular repository or module validation runs
- **THEN** the system MAY compile `live`-tagged tests in an offline check, but it MUST NOT turn that compile-only verification into an implicit live network execution

#### Scenario: Live smoke exercises a real public adapter through runtime ingestion

- **WHEN** a developer intentionally runs the manual live smoke for a supported public venue adapter such as Hyperliquid
- **THEN** the smoke MUST read real public market data through the concrete adapter and ingest it through the existing `venueedge` ingestion flow

#### Scenario: Live smoke stays outside private venue behavior

- **WHEN** the first live smoke path is implemented for Hyperliquid
- **THEN** it MUST stay scoped to public read-only market-data behavior and MUST NOT require wallet signing, `approveAgent`, account provisioning, transfers, or trading actions
