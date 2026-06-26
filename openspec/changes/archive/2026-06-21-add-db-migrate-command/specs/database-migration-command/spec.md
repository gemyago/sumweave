## ADDED Requirements

### Requirement: Explicit Backend Database Migration Command
The backend application SHALL provide an explicit `signal-foundry db-migrate` command that prepares configured Signal Foundry-managed database schemas without starting long-running application processes.

#### Scenario: Command is discoverable on the existing app binary
- **WHEN** a user inspects the `signal-foundry` command tree
- **THEN** `db-migrate` MUST appear as a root-level Cobra subcommand
- **AND** it MUST use the same config loading, `--env`, logging, and database DSN conventions as the existing backend commands

#### Scenario: Command migrates all configured app schemas
- **WHEN** a user runs `signal-foundry db-migrate` with a valid environment configuration
- **THEN** the command MUST run all configured schema initialization steps for agent runtime database storage when enabled, data-layer persistence, durable jobs persistence, finance persistence, strategy workspace persistence, and evaluation workspace persistence
- **AND** it MUST complete without starting the HTTP server, jobs worker, scheduler loop, provider sync, or AI/runtime request execution

#### Scenario: Command is idempotent
- **WHEN** a user runs `signal-foundry db-migrate` more than once against the same local SQLite or configured database
- **THEN** each run MUST complete successfully without duplicating data, mutating immutable product records, or requiring manual cleanup

#### Scenario: Migration failure is explicit
- **WHEN** any configured migration step fails
- **THEN** the command MUST exit with an error
- **AND** the error MUST identify the component whose migration failed while avoiding secrets, raw SQL credentials, stack traces, or unbounded provider payloads

### Requirement: Standard Environment Setup Uses Explicit Migration
The repository SHALL document explicit database migration as a standard setup step before starting Signal Foundry backend processes.

#### Scenario: Local setup documents migration before PM2 startup
- **WHEN** a developer follows documented local setup or PM2 startup instructions
- **THEN** the instructions MUST direct them to run `signal-foundry db-migrate` before starting or restarting backend processes that depend on persisted tables

#### Scenario: Backend docs describe auto-migration as non-primary
- **WHEN** a developer reads backend architecture or configuration documentation
- **THEN** the docs MUST explain that `db-migrate` is the standard schema setup path
- **AND** startup-time auto-migration flags MUST be documented as compatibility or development convenience behavior rather than the primary migration mechanism

### Requirement: Startup Does Not Run Schema Migrations
Backend process startup SHALL rely on schemas prepared by the explicit migration command instead of creating or updating schemas implicitly.

#### Scenario: Startup command does not migrate schemas
- **WHEN** the documented standard environment setup has been followed
- **THEN** `signal-foundry start`, jobs worker commands, and scheduler commands MUST rely on schemas prepared by `signal-foundry db-migrate`
- **AND** those startup paths MUST NOT run app-owned database migration steps implicitly
- **AND** migration failures MUST be observable from the migration command rather than during server or worker startup
