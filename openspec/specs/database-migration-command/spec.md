# database-migration-command Specification

## Purpose
Define explicit preparation of the Sumweave application database.
## Requirements
### Requirement: Explicit Backend Database Migration Command
The backend application SHALL provide an explicit `sumweave db-migrate` command
that prepares configured Sumweave-managed PostgreSQL schemas without starting
long-running application processes.

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
The repository SHALL document `make postgres-bootstrap` as standard setup before
starting Sumweave backend processes. It provisions PostgreSQL and invokes the
explicit migration command.

#### Scenario: Local setup documents migration before local all-in-one startup
- **WHEN** a developer follows documented local backend setup instructions
- **THEN** the instructions MUST direct them to run `make postgres-bootstrap`
  before starting `sumweave start-all` or other backend processes that depend on
  persisted tables
- **AND** bootstrap MUST prepare separate `sumweave_local` and `sumweave_test`
  databases, run `sumweave db-migrate` once for each through the migrator role,
  and grant the runtime role access before backend processes or tagged tests run
- **AND** routine tests MUST remain database-independent

#### Scenario: Split process modes use the same prepared schemas

- **WHEN** a developer runs `sumweave start`, `sumweave jobs worker`, or
  `sumweave jobs enqueue-due` after migration
- **THEN** each mode MUST use the same prepared dispatch, job-projection, and
  finance schemas
- **AND** scheduler publication MUST not create an observed job row before
  worker delivery.

### Requirement: Startup Does Not Run Schema Migrations
Backend process startup SHALL rely on schemas prepared by the explicit migration command instead of creating or updating schemas implicitly.

#### Scenario: Startup commands do not migrate schemas
- **WHEN** the documented standard environment setup has been followed
- **THEN** `sumweave start`, `sumweave start-all`, jobs consumer commands, and scheduler commands MUST rely on schemas prepared by `sumweave db-migrate`
- **AND** those startup paths MUST NOT run app-owned database or app-owned pub/sub transport migration steps implicitly
- **AND** migration failures MUST be observable from the migration command rather than during server, consumer, or scheduler startup
