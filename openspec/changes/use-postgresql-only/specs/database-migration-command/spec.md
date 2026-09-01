## MODIFIED Requirements

### Requirement: Explicit Backend Database Migration Command
The backend application SHALL provide an explicit `sumweave db-migrate` command that prepares configured Sumweave-managed PostgreSQL schemas without starting long-running application processes.

#### Scenario: Command migrates all configured app schemas
- **WHEN** a user runs `sumweave db-migrate` with valid PostgreSQL environment configuration
- **THEN** the command MUST run all configured schema initialization steps for
  agent runtime storage, application database-backed auth and dispatch state,
  job-projection persistence used by observed consumers, and finance persistence
- **AND** dispatch storage MUST support immutable unique message IDs and the
  topic/consumer-group offsets required for durable delivery
- **AND** finance persistence MUST include the authoritative bank-connection
  schedule and daily FX refresh due-state tables used by `jobs enqueue-due`
- **AND** it MUST complete without starting the HTTP server, jobs consumer mode, scheduler loop, provider sync, or AI/runtime request execution

### Requirement: Standard Environment Setup Uses Explicit Migration
The repository SHALL provide and document PostgreSQL provisioning followed by explicit database migration for both the regular and test environments before starting Sumweave backend processes or tagged database-backed tests.

#### Scenario: Local setup migrates regular and test databases
- **WHEN** a developer follows documented local backend setup instructions
- **THEN** the instructions MUST direct them to run `make postgres-bootstrap`
- **AND** the target MUST start and wait for the repository-managed Docker Compose PostgreSQL service with separate regular and test databases
- **AND** the target MUST run `sumweave db-migrate` from `apps/sumweave` once with regular environment configuration and once with test environment configuration through the migrator role
- **AND** regular backend processes MUST use the migrated regular database while database-backed tests MUST use the migrated test database
- **AND** routine tests MUST remain runnable without invoking this setup

#### Scenario: Split process modes use the same prepared schemas

- **WHEN** a developer runs `sumweave start`, `sumweave jobs worker`, or
  `sumweave jobs enqueue-due` after migration
- **THEN** each mode MUST use the same prepared dispatch, job-projection, and
  finance schemas
- **AND** scheduler publication MUST not create an observed job row before
  worker delivery

#### Scenario: Externally managed verification uses the same migration contract
- **WHEN** PostgreSQL verification runs with `POSTGRES_MANAGED_EXTERNALLY=1`
- **THEN** bootstrap MUST omit Docker Compose startup but require a privileged
  `POSTGRES_BOOTSTRAP_DSN` for readiness, role/database creation, ownership,
  grants, and migrator default privileges on a fresh service
- **AND** it MUST then run both explicit migrations and apply runtime grants
- **AND** cluster setup MUST NOT create or alter application tables outside the explicit migration command
