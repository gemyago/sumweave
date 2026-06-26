## ADDED Requirements

### Requirement: Finance Schema Is Prepared Explicitly
The backend application SHALL include finance-owned persistence schema initialization in the explicit backend database migration command.

#### Scenario: Migration creates finance-owned tables
- **WHEN** a user runs `signal-foundry db-migrate` with valid backend database configuration
- **THEN** the command MUST run the finance persistence migration for finance-owned tables before finance API, import, reporting, sync, or finance durable job flows rely on those tables
- **AND** finance-owned tables MUST keep finance persistence ownership, explicit column names, UTC-first timestamps, and compatibility with SQLite local development and PostgreSQL-oriented production use

#### Scenario: Finance startup relies on prepared schema in standard setup
- **WHEN** the documented standard setup has run `signal-foundry db-migrate`
- **THEN** finance service registration and finance job handler registration MUST rely on the prepared finance schema
- **AND** they MUST NOT create or update finance tables implicitly during startup
