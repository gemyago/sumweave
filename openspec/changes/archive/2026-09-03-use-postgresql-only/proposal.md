## Why

Sumweave production already uses PostgreSQL, while SQLite support adds a second
persistence and dispatch implementation used only by local development and
tests. PostgreSQL should be the sole product database, and the same database
behavior should be exercised by the normal local and CI test flow.

The first implementation of this change incorrectly made PostgreSQL testing a
separate verification product: it added a `postgres-verify` workflow, dedicated
test and coverage lanes, renamed/split test files around a `_postgres_test.go`
suffix, and added extensive contract scripts and coverage-only production
seams. This correction removes that machinery. PostgreSQL is instead a normal
development and CI prerequisite that is started and bootstrapped before tests.

## What Changes

- **BREAKING** Remove SQLite as a supported database from `finance/`,
  `runtime/`, and `apps/sumweave/`; PostgreSQL remains the sole persistence and
  appdispatch implementation in every environment.
- Keep the repository Compose service and one small operational
  `make postgres-bootstrap` path that prepares the regular and test databases,
  applies the existing migrations, and grants runtime access.
- Treat PostgreSQL setup as a normal local and CI prerequisite. The reusable CI
  workflow bootstraps it before the existing Nx test command.
- Treat the shared runtime-role test DSN as execution-environment setup rather
  than a Makefile fallback. The standard direnv environment supplies it for
  local module, root, and Nx tests, while the reusable CI workflow supplies it
  to its Nx test step; module recipes only inherit it.
- Remove every `postgres_test` reference from core Go source: delete the build
  constraint from every affected test, neutralize the stale DSN diagnostics and
  callback key that name it, and select PostgreSQL-backed tests normally. Keep
  the three affected `!release` constraints, one ordinary `make test`, and the
  existing single `.cover/profile.out` checked by `.testcoverage.yaml`.
- Remove `postgres-verify`, focused `postgres-test-*` and module
  `test-postgres` targets, the separate workflow, PostgreSQL-only coverage
  profiles/configuration, migration coverage transport, and contract scripts
  that exist only to enforce those lanes.
- Restore pre-change test filenames. Merge test splits created only for the
  separate lane back into their original files, while retaining the PostgreSQL
  fixtures, unrelated build constraints, randomized identities, and scoped
  assertions needed by the PostgreSQL-only implementation.
- Revert generated mocks, interfaces, adapters, and tests introduced only to
  maintain database-free routine coverage. Keep only production changes needed
  for PostgreSQL-only persistence and real PostgreSQL test compatibility. Revert
  the unrelated jobs worker periodic-cycle change and its new unit test
  unconditionally; any cancellation fix belongs in a separate approved change.
- Keep active setup and architecture documentation focused on the sole database,
  one bootstrap command, and one ordinary test command. Remove documentation and
  agent guidance claiming that `postgres_test` or the rejected verification lane
  remains, while preserving
  the independent agent-harness statement in `tests/AGENTS.md` that its routine
  harness tests remain database-independent. Land command-facing root/module
  AGENTS and docs atomically with the ordinary-test switch so no reviewed commit
  advertises a removed command.
- During implementation, restore the three base specs edited before this active
  change is archived—`database-migration-command`, `domain-event-pubsub`, and
  `finance-management`—to `main`; keep the PostgreSQL requirements in this
  change's deltas until archive.
- Use commit and hunk provenance for every rollback. Restore branch-only test,
  generated-mock, formatting, manifest, and prose churn to `main`, and preserve
  only the smallest observable PostgreSQL-only delta.
- Give every implementation chunk its own applicable TDD or restoration
  rationale and immediate checks; do not defer implementation verification to a
  standalone final task. Keep chunk review quick and shallow, then reserve the
  deep provenance and final-state audit for the independent final reviewer.
- Do not add timestamp normalization, a new migration framework, SQLite data
  conversion, or compatibility support.

## Capabilities

### New Capabilities

- `postgresql-database-environments`: Define PostgreSQL as the sole supported
  product database and as a normal prerequisite of local and CI tests.

### Modified Capabilities

- `database-migration-command`: Keep the explicit PostgreSQL migration command
  and use it from the normal local/CI bootstrap.
- `domain-event-pubsub`: Require the durable transport and schema preparation to
  use PostgreSQL only.
- `finance-management`: Require finance persistence and schema preparation to
  use PostgreSQL only.

## Impact

- Production persistence code and core dependency cleanup from the PostgreSQL
  cutover remain in `finance/`, `runtime/`, and `apps/sumweave/`.
- Local and CI environments require the repository PostgreSQL service and
  `make postgres-bootstrap` before tests.
- Core module test Makefiles remain close to their pre-change shape: one `test`
  target, one profile, and one coverage configuration, with no PostgreSQL test
  build tag, DSN fallback/recipe export, or alternate lane.
- The correction removes the standalone verification workflow and the test,
  coverage, parser, contract, mock, and production-seam expansion caused by the
  rejected lane split.
- Base specifications remain untouched during planning and are restored as an
  explicit unchecked implementation task before this change is archived.
- HTTP APIs, finance behavior, durable-job semantics, table shapes, and deployed
  PostgreSQL persistence are unchanged by the correction.
