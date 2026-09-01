## MODIFIED Requirements

### Requirement: Explicit Backend Database Migration Command
The backend application SHALL provide an explicit `sumweave db-migrate` command that prepares configured Sumweave-managed PostgreSQL schemas without starting long-running application processes.

#### Scenario: Command migrates all configured app schemas
- **WHEN** a user runs `sumweave db-migrate` with valid PostgreSQL environment configuration
- **THEN** the command MUST run all configured schema initialization steps for agent runtime storage, application database-backed auth and dispatch state, durable jobs persistence, and finance persistence
- **AND** it MUST complete without starting the HTTP server, jobs consumer mode, scheduler loop, provider sync, or AI/runtime request execution

### Requirement: Standard Environment Setup Uses Explicit Migration
The repository SHALL document PostgreSQL provisioning followed by explicit database migration for both the regular and test environments as standard setup steps before starting Sumweave backend processes or database-backed tests.

#### Scenario: Local setup migrates regular and test databases
- **WHEN** a developer follows documented local backend setup instructions
- **THEN** the instructions MUST direct them to start the repository-managed Docker Compose PostgreSQL service with separate regular and test databases
- **AND** the instructions MUST direct them to run `sumweave db-migrate` once with regular environment configuration and once with test environment configuration
- **AND** regular backend processes MUST use the migrated regular database while database-backed tests MUST use the migrated test database
