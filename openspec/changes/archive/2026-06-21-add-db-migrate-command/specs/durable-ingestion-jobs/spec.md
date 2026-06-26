## ADDED Requirements

### Requirement: Durable Jobs Schema Is Prepared Explicitly
The backend application SHALL include durable jobs storage in the explicit backend database migration command.

#### Scenario: Migration creates durable jobs tables
- **WHEN** a user runs `signal-foundry db-migrate` with valid data-layer database configuration
- **THEN** the command MUST create or update the durable jobs table and durable schedule table used by historical-data and finance job execution
- **AND** it MUST use the configured data-layer database DSN and table prefix conventions

#### Scenario: Jobs commands rely on prepared durable schemas
- **WHEN** a user starts the jobs worker or runs a scheduler enqueue tick after `signal-foundry db-migrate` succeeds
- **THEN** jobs command startup MUST rely on the prepared durable jobs schema
- **AND** it MUST NOT create or update jobs tables implicitly during startup
