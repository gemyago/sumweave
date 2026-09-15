## MODIFIED Requirements

### Requirement: Explicit Backend Database Migration Command

The backend application SHALL provide an explicit `sumweave db-migrate` command
that prepares configured Sumweave-managed PostgreSQL schemas without starting
long-running application processes.

#### Scenario: Command migrates all configured app schemas

- **WHEN** a user runs `sumweave db-migrate` with valid PostgreSQL configuration
- **THEN** the command MUST initialize agent runtime, auth users, auth refresh
  tokens, auth personal access tokens, appdispatch, job-projection, and finance
  schemas
- **AND** the access-token migration MUST run after auth users and refresh tokens
- **AND** it MUST complete without starting HTTP, workers, schedulers, provider
  sync, or runtime request execution.
