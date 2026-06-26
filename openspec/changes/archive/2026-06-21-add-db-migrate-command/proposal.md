## Why

Database schema creation currently happens implicitly during selected startup paths when `*.database.autoMigrate` is enabled, which makes standard environment setup hard to reason about and hides migration failures behind unrelated server or worker startup. We need an explicit operational command so developers and operators can migrate local or deployed databases intentionally before starting the app processes.

## What Changes

- Add an explicit `signal-foundry db-migrate` Cobra subcommand for running all app-owned GORM/finance migration steps that are currently tied to startup auto-migration.
- Make standard environment setup run the migration command before starting PM2/API/jobs processes, and document that requirement in the relevant repo and backend docs.
- Keep the command idempotent and safe to run repeatedly for local SQLite and configured database-backed stores.
- Remove implicit startup-time auto-migration from API, jobs, finance, strategy, and evaluation app startup wiring so schema changes happen through the explicit command.
- **BREAKING**: application process startup no longer creates or updates database schemas implicitly.

## Capabilities

### New Capabilities
- `database-migration-command`: Explicit backend database migration command and setup documentation for preparing Signal Foundry-managed tables.

### Modified Capabilities
- `durable-ingestion-jobs`: Durable jobs tables must be created by the explicit migration path before API, scheduler, or worker commands rely on them.
- `finance-management`: Finance persistence must be included in the explicit migration path before finance API/job flows rely on it.
- `strategy-artifacts`: Strategy artifact and version registry tables must be included in the explicit migration path before strategy workspace or evaluation flows rely on them.

## Impact

- Affects `apps/signal-foundry/cmd/signal-foundry` command wiring and tests.
- Affects `apps/signal-foundry/internal` migration wiring for agent runtime storage, data layer storage, jobs, finance, strategy workspace, and evaluation workspace persistence.
- Affects backend configuration defaults and injected settings related to `agentRuntime.database.autoMigrate` and `dataLayer.database.autoMigrate`.
- Affects setup documentation in repo-level and backend docs, including PM2/local startup instructions.
- No new external dependencies are expected.
