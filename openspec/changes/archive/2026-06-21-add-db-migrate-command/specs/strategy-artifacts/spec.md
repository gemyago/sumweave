## ADDED Requirements

### Requirement: Strategy Workspace Schemas Are Prepared Explicitly
The backend application SHALL include strategy workspace and evaluation persistence schema initialization in the explicit backend database migration command.

#### Scenario: Migration creates strategy artifact and version registry tables
- **WHEN** a user runs `signal-foundry db-migrate` with valid data-layer database configuration
- **THEN** the command MUST create or update the strategy artifact store and strategy version registry tables using the configured data-layer DSN and strategy table prefix conventions
- **AND** the migration MUST preserve immutable strategy artifact semantics and version identity uniqueness

#### Scenario: Migration creates evaluation persistence tables
- **WHEN** a user runs `signal-foundry db-migrate` with valid data-layer database configuration
- **THEN** the command MUST create or update evaluation persistence tables for governor policy artifacts, audit records, execution records, and backtest records that are needed by strategy evaluation flows
- **AND** evaluation persistence MUST keep its configured table prefix conventions

#### Scenario: Strategy and evaluation startup rely on prepared schemas
- **WHEN** the documented standard setup has run `signal-foundry db-migrate`
- **THEN** strategy workspace and evaluation services MUST rely on the prepared schemas
- **AND** they MUST NOT create or update strategy or evaluation tables implicitly during startup
