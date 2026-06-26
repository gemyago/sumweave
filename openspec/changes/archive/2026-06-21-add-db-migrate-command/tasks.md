## 1. Command Contract

- [x] 1.1 Add the root-level `db-migrate` Cobra command registration; follow TDD flow by first adding command-tree tests that assert discoverability, shared persistent flags, and no server/worker startup side effects.
- [x] 1.2 Add command error handling and logging behavior; follow TDD flow by first adding command tests that assert migration failures return contextual errors without unsafe output.

## 2. Migration Orchestration

- [x] 2.1 Implement the app-owned migration orchestrator for data-layer, jobs, finance, strategy, and evaluation persistence; follow TDD flow by first adding SQLite-backed tests that prove each schema family is created through the explicit command path.
- [x] 2.2 Implement conditional agent runtime database migration; follow TDD flow by first adding tests that cover database-backed agent runtime migration and file-backed no-op behavior.
- [x] 2.3 Keep migration execution idempotent and non-executing; follow TDD flow by first adding tests that run `db-migrate` twice and assert no HTTP server, worker, scheduler, provider sync, or runtime request execution starts.

## 3. Startup And Configuration Alignment

- [x] 3.1 Remove startup-time auto-migration from app startup wiring; follow TDD flow by first adding tests that show `start`, jobs worker, scheduler, finance, strategy, and evaluation startup paths do not create schemas implicitly.
- [x] 3.2 Remove or neutralize obsolete auto-migration configuration bindings and defaults; follow TDD flow by first updating config tests that assert startup no longer depends on `*.database.autoMigrate`.

## 4. Documentation

- [x] 4.1 Document `signal-foundry db-migrate` in standard local setup and PM2 startup guidance; follow TDD flow by first adding or updating documentation coverage tests where this repo already checks backend docs.
- [x] 4.2 Update backend architecture/config documentation and AGENTS guidance for the new setup command; follow TDD flow by first adding or updating tests or doc assertions that keep the command from disappearing from relevant setup docs.
