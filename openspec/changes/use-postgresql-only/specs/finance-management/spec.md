## MODIFIED Requirements

### Requirement: Finance Module Boundary And Persistence Ownership
The system SHALL implement finance as a root `finance/` product module that remains architecturally independent from the generic agent runtime.

#### Scenario: App composes finance without reverse runtime imports
- **WHEN** finance functionality is wired into the product
- **THEN** `apps/sumweave/` MUST compose both `runtime/` and `finance/`
- **AND** `finance/` MUST NOT import `runtime/`
- **AND** finance business rules MUST live in `finance/` while auth, process lifecycle, generic jobs runtime, and HTTP route glue remain app-owned

#### Scenario: Finance persistence stays finance-owned and auto-migrated
- **WHEN** finance data is persisted
- **THEN** finance-owned tables MUST use `finance_` prefixes, GORM auto-migrate schema initialization, and explicit column names
- **AND** finance domain models MUST remain separate from GORM persistence models
- **AND** the storage design MUST use PostgreSQL only

### Requirement: Finance Schema Is Prepared Explicitly
The backend application SHALL include finance-owned PostgreSQL schema initialization in the explicit backend database migration command.

#### Scenario: Migration creates finance-owned tables
- **WHEN** a user runs `sumweave db-migrate` with valid PostgreSQL backend database configuration
- **THEN** the command MUST run the finance persistence migration for finance-owned tables before finance API, import, reporting, sync, or finance durable job flows rely on those tables
- **AND** finance-owned tables MUST keep finance persistence ownership, explicit column names, and PostgreSQL-only behavior

#### Scenario: Finance startup relies on prepared schema in standard setup
- **WHEN** the documented standard setup has run `sumweave db-migrate`
- **THEN** finance service registration and finance job handler registration MUST rely on the prepared finance schema
- **AND** they MUST NOT create or update finance tables implicitly during startup

#### Scenario: Finance migration receives shallow ordinary coverage

- **WHEN** finance migration execution requires direct Go coverage
- **THEN** one tagged migration smoke MAY execute the finance migrator against
  the bootstrap-prepared test database in the ordinary test flow
- **AND** tests MUST NOT require bootstrap coverage instrumentation, a separate
  finance coverage profile, detailed legacy schema fixtures, or a production
  AutoMigrate seam added only for mocking
- **AND** obsolete SQLite-era compatibility cleanup and detailed migration
  contracts MUST NOT be restored
