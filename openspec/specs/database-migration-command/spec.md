# database-migration-command Specification

## Purpose
TBD - created by archiving change add-db-migrate-command. Update Purpose after archive.
## Requirements
### Requirement: Explicit Backend Database Migration Command
The backend application SHALL provide an explicit `signal-foundry db-migrate` command that prepares configured Signal Foundry-managed database schemas without starting long-running application processes.

#### Scenario: Command migrates all configured app schemas
- **WHEN** a user runs `signal-foundry db-migrate` with a valid environment configuration
- **THEN** the command MUST run all configured schema initialization steps for agent runtime database storage when enabled, data-layer persistence, durable jobs persistence, app-owned pub/sub transport state required for durable job dispatch, finance persistence, strategy workspace persistence, and evaluation workspace persistence
- **AND** it MUST complete without starting the HTTP server, jobs consumer mode, scheduler loop, provider sync, or AI/runtime request execution

### Requirement: Standard Environment Setup Uses Explicit Migration
The repository SHALL document explicit database migration as a standard setup step before starting Signal Foundry backend processes.

#### Scenario: Local setup documents migration before local all-in-one startup
- **WHEN** a developer follows documented local backend setup instructions
- **THEN** the instructions MUST direct them to run `signal-foundry db-migrate` before starting `signal-foundry start-all` or other backend processes that depend on persisted tables

### Requirement: Startup Does Not Run Schema Migrations
Backend process startup SHALL rely on schemas prepared by the explicit migration command instead of creating or updating schemas implicitly.

#### Scenario: Startup commands do not migrate schemas
- **WHEN** the documented standard environment setup has been followed
- **THEN** `signal-foundry start`, `signal-foundry start-all`, jobs consumer commands, and scheduler commands MUST rely on schemas prepared by `signal-foundry db-migrate`
- **AND** those startup paths MUST NOT run app-owned database or app-owned pub/sub transport migration steps implicitly
- **AND** migration failures MUST be observable from the migration command rather than during server, consumer, or scheduler startup

