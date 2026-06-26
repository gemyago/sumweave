## Context

The backend app currently wires database schema creation through constructor/startup paths controlled by `agentRuntime.database.autoMigrate` and `dataLayer.database.autoMigrate`. That means `signal-foundry start`, jobs wiring, finance registration, strategy workspace wiring, and evaluation workspace wiring can create tables as a side effect of starting a process. Production already disables at least the data-layer auto-migrate flag, while local defaults keep it enabled, so the operational contract is inconsistent and easy to miss.

The intended product path is still the single Go app under `apps/signal-foundry/` composing the runtime and finance modules. A database migration command should live in that app binary beside the existing Cobra subcommands, while module-owned stores keep their schema ownership through their existing `AutoMigrate` or `Migrate` methods.

## Goals / Non-Goals

**Goals:**
- Provide one explicit `signal-foundry db-migrate` command for preparing Signal Foundry-managed tables before API, scheduler, or worker startup.
- Reuse existing store/service migration methods so schema ownership remains with the module that owns the persistence model.
- Cover app-owned and app-composed persistence currently initialized by startup auto-migration: agent runtime database storage when configured, data layer canonical tables, durable jobs tables and schedules, finance persistence, strategy artifacts/version registry, and evaluation persistence stores.
- Remove startup-time auto-migration from the app wiring so process startup assumes schemas were already prepared.
- Document the command as part of standard local and environment setup before PM2 or backend process startup.
- Keep the command idempotent and easy to test with local SQLite.

**Non-Goals:**
- Replace GORM AutoMigrate with a versioned migration framework in this change.
- Redesign database schemas, table prefixes, or persistence model boundaries.
- Introduce a separate migration binary or external dependency.
- Add UI behavior or tenant-facing migration controls.

## Decisions

### Add `db-migrate` to the existing Cobra binary

`signal-foundry db-migrate` should be registered from `cmd/signal-foundry` alongside `start`, `jobs`, `data`, `finance`, and `user`.

Alternatives considered:
- Separate binary: rejected because the repo already uses one backend app binary and shared config/env loading.
- Hidden startup hook only: rejected because the goal is explicit setup, observability, and failure isolation.

### Centralize orchestration in app-owned migration wiring

The command should call an app-owned migration orchestrator that constructs the configured stores and invokes their existing migration methods in a deterministic order. Store packages continue to own concrete schema details; the orchestrator owns only sequence and error context.

Alternatives considered:
- Move all migrations into `runtime/`: rejected because finance and app-owned jobs are composed by `apps/signal-foundry`, and `runtime/` must not own the whole app process.
- Keep duplicate migration calls in each command: rejected because drift would preserve the current implicit behavior problem.

### Drop startup auto-migration from app startup wiring

The setup path should be `db-migrate` followed by normal process startup. Startup-time auto-migration should be removed from app startup wiring, including API runtime construction, jobs store wiring, finance store wiring, strategy workspace wiring, and evaluation workspace wiring. Tests that need schemas should run the same migration path or invoke the owning store migration directly in isolated unit tests.

Alternatives considered:
- Keep startup auto-migration as compatibility: rejected because the project is early alpha and the implicit behavior is exactly what this change is meant to remove.
- Keep defaults unchanged and only add docs: rejected because users would still see startup as a valid schema setup mechanism.

### Fail fast with contextual errors

The command should stop on the first failed migration and wrap errors with the component being migrated. It should not start HTTP servers, durable workers, schedulers, or provider sync work.

Alternatives considered:
- Continue after failures and report an aggregate: rejected for the first implementation because dependent tables can make later failures noisy and less actionable.

## Risks / Trade-offs

- Some tests may currently depend on startup-time auto-migration -> Update those tests to run `db-migrate` or explicit store-level migrations before exercising startup behavior.
- Migration orchestration can duplicate DI construction details -> Keep it thin, reuse existing constructors where practical, and test command behavior through the Cobra command.
- Agent runtime database migration is conditional on `agentRuntime.storage.type=database` -> The command must no-op clearly for file-backed agent runtime storage while still migrating shared data-layer stores.
- Finance, strategy, and evaluation stores share the data-layer DSN with different table prefixes -> Tests should verify that the command prepares the expected store families without requiring live external services.

## Migration Plan

1. Add the command and migration orchestrator.
2. Wire the orchestrator to existing store migration methods with contextual errors.
3. Remove startup-time auto-migration from API/runtime, jobs, finance, strategy workspace, and evaluation workspace wiring.
4. Update setup documentation to run `signal-foundry db-migrate` before `pm2 start ecosystem.config.js` or direct backend startup.
5. Remove or neutralize auto-migration config settings that no longer affect startup behavior.
6. Rollback is a normal code rollback to the previous startup wiring if the explicit command is broken.

## Open Questions

- Should the command support a `--component` filter now, or wait until there is an operational need for partial migrations?
