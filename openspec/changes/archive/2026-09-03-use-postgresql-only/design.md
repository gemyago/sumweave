## Context

PostgreSQL is now the only production persistence implementation in the core Go
product. The branch also contains a repository Compose service and bootstrap
path for `sumweave_local` and `sumweave_test`.

The rejected test design treated PostgreSQL as an optional, non-routine lane.
It kept ordinary CI database-free, introduced `postgres-verify` and focused
module targets, duplicated coverage profiles/configuration, instrumented the
bootstrap migration for coverage, renamed or split tests by database suffix,
and added contract scripts plus production interfaces solely to satisfy the
database-free coverage lane. That design is contrary to the correction request.

The corrected model is simpler: PostgreSQL is ordinary local/CI infrastructure.
Setup happens before tests, and PostgreSQL-backed tests participate in normal Go
package selection without a custom build tag or alternate test lane.

## Goals / Non-Goals

**Goals:**

- Preserve PostgreSQL-only production persistence and appdispatch behavior.
- Keep one canonical Compose/bootstrap setup for local development and CI.
- Make PostgreSQL available before ordinary local and CI tests.
- Remove every `postgres_test` reference from core Go source while preserving
  the three affected `!release` constraints.
- Restore the original single profile and coverage configuration per module.
- Restore pre-change test filenames and remove lane-driven file splits.
- Delete scripts, mocks, adapters, tests, docs, and rules that exist only to
  police the rejected split-lane design.
- Minimize the final diff against `main` while retaining required PostgreSQL
  fixture conversions and isolation fixes.

**Non-Goals:**

- Restore SQLite or any dual-dialect fallback.
- Preserve local SQLite data or migration compatibility.
- Add a second PostgreSQL verification workflow or target.
- Add per-test databases, schemas, migration frameworks, or SQL-string tests.
- Lower the existing 90% file or total coverage gates.
- Normalize dates or timestamps to UTC.
- Refactor production code merely to make it mockable.
- Change finance, jobs, appdispatch, HTTP, or deployment behavior beyond the
  PostgreSQL-only cutover.

## Decisions

### PostgreSQL-only production changes stay

Keep the direct PostgreSQL constructors, SQL/GORM predicates, appdispatch
publisher/subscriber/migrator, PostgreSQL DSN validation, local/test config, and
core dependency removals. Keep deletion of SQLite transports, connection
helpers, WAL/default handling, dialect branches, and SQLite drivers.

The correction must not revert the actual product cutover while deleting its
test-lane scaffolding. Production files that contain both kinds of change are
reduced to the PostgreSQL-only delta; testability seams introduced only for the
database-free lane are removed.

### One normal bootstrap path

`make postgres-bootstrap` remains the sole repository setup command. In normal
local and CI use it starts the Compose service, waits for readiness, prepares the
owner/migrator/runtime roles and `sumweave_local`/`sumweave_test` databases,
runs `sumweave db-migrate --env local` and `--env test`, and grants runtime
access.

Keep `compose.yaml`, `scripts/postgres/bootstrap.sh`, and the three SQL files
directly invoked by bootstrap. They are regular operational inputs, not test
contracts. Remove external/manual-verification and finance-coverage branches
from bootstrap. Do not combine the SQL files merely to reduce file count: that
would add churn without changing the operational path.

The reusable `.github/workflows/tests-run.yml` runs bootstrap once before its
existing test step. The Nx command remains unchanged. Local documentation tells
developers to bootstrap before `make test`, Nx tests, or
`make affected-lint-test`. Module test targets assume that prerequisite is
satisfied; they do not start Docker independently.

### Environment setup owns the shared test DSN

`SUMWEAVE_POSTGRES_TEST_DSN` is an input to database-backed Go test fixtures,
not an application configuration key. The committed root `.envrc` exports the
Compose runtime-role `sumweave_test` DSN for the standard local direnv shell.
Direct runtime, finance, and app module tests, root recursive tests, and local Nx
tests then inherit one prepared value from that shell. The reusable CI workflow
sets the same variable on its existing Nx test step, whose child module
Makefiles and Go processes inherit it.

The three core module Makefiles neither define a fallback nor prefix `go test`
with a DSN assignment. This removes three duplicated defaults and three
command-boundary re-exports without adding an env-file loader, Make include, Nx
wrapper, or Go test-config package.

Bootstrap owns service, role, database, migration, and grant preparation only.
It does not print, persist, or attempt to export test-process environment:
`make postgres-bootstrap` runs as a child and cannot mutate its caller's shell.
`apps/sumweave/internal/config/test.yaml` remains the source for the app's typed
`application.database.dsn` and `agentRuntime.database.dsn` values when code
loads the `test` profile. Existing `APP_` `AutomaticEnv` mapping remains the
only application-config override mechanism. The test YAML, config loader,
bootstrap script, Compose file, root Makefile, test helpers, and Nx project
files require no change for this correction.

### One ordinary test and coverage flow

Restore each core module Makefile to the pre-change one-target shape:

- `make test` runs ordinary `go test` over its existing package and `-coverpkg`
  scope, without a PostgreSQL build tag;
- `SUMWEAVE_POSTGRES_TEST_DSN` is required from the inherited local or CI
  execution environment;
- finance may retain `-p 1` only if the shared prepared database demonstrates a
  real in-module ordering conflict;
- the profile is `.cover/profile.out` and is checked by the existing
  `.testcoverage.yaml` at 90% per file and total;
- the root aggregate returns to consuming `.cover/profile.out` from all Go
  modules in its pre-change order.

There is no `test-postgres`, `postgres-test-runtime`,
`postgres-test-finance`, `postgres-test-sumweave`, or `postgres-verify`; no
`.cover/routine.out`, `.cover/postgres.out`, raw migration/test covdata, or
`.testcoverage-routine.yaml`; and no finance migration coverage environment
variable or readiness marker.

Ownership is intentionally non-overlapping. The ordinary-test switch owns every
caller and consumer: root/module Makefile targets, the historical command-level
variable wiring, profile/config selection, raw-covdata composition, and
readiness-marker checks. The latest environment correction removes that
historical Makefile wiring and places the test-process input at the local and CI
environment boundaries.
It also adds the one smallest shallow PostgreSQL smoke to the neutral pre-change
`finance/persistence/migrations_test.go` owner so the atomic switch satisfies the
unchanged coverage gate. The operational-bootstrap chunk owns the dormant
implementation branches inside `scripts/postgres/bootstrap.sh`: migration
coverage environment parsing, `GOCOVERDIR`, raw-directory handling, and marker
creation. The later finance persistence chunk removes the coverage-only seam and
suite while retaining or minimally refining that existing smoke; it must not add
a duplicate or restore caller-side coverage transport or bootstrap
instrumentation.

In the target state, no test or source file mentions or uses `postgres_test`.
PostgreSQL-backed tests are ordinary package members after bootstrap. The current
inventory has 84 such source occurrences across 76 core test files: 76 build
constraints, seven stale DSN diagnostics, and one GORM callback registration
key. Of those 76 files, 73 lose their constraint entirely; the two runtime agent
tests and app `engine_test.go`
with compound `postgres_test && !release` constraints retain only
`//go:build !release`. The diagnostics and callback key become neutral database
test wording, and the corresponding three `-tags=postgres_test` test arguments
plus the finance lint build-tag argument are removed rather than replaced.

The one shallow migration smoke executes through the ordinary test added
to `finance/persistence/migrations_test.go` with the ordinary-test switch. The
later persistence restoration may minimally refine that same smoke for the
restored concrete migrator, but must not create another owner or test. The smoke
replaces, rather than adds to, detailed schema-contract tests and does not require
bootstrap coverage instrumentation.

Task 2.6 keeps both ordered model groups and their existing wrapped error
contexts, but deduplicates the direct `WithContext(ctx).AutoMigrate(...)` call
and wrapping in one unexported concrete `Migrator` method. The existing
`TestMigrate` then adds one canceled-context assertion against the prepared real
PostgreSQL database, covering the shared failure path and first caller return.
This is the minimum same-owner refinement that clears the unchanged per-file
gate: it adds no interface, mock, second smoke, schema assertion, fixture, or
coverage exclusion, and leaves the second caller error return structurally
uncovered while preserving its production behavior.

### Restore filenames and collapse lane-only splits

Direct runtime renames are reversed exactly:

- `runtime/internal/agentprofiles/db_agent_profiles_service_postgres_test.go`
  → `db_agent_profiles_service_test.go`
- `runtime/internal/llmproviders/db_providers_config_service_postgres_test.go`
  → `db_providers_config_service_test.go`
- `runtime/internal/sessions/database_metadata_postgres_test.go`
  → `database_metadata_test.go`
- `runtime/internal/sessions/database_service_postgres_test.go`
  → `database_service_test.go`
- `runtime/internal/sessions/database_storage_postgres_test.go`
  → `database_storage_test.go`

Where a `_postgres_test.go` file was added by extracting database cases from an
existing test, merge those cases back into the original filename and delete the
split file:

- runtime `agent/database_services_postgres_test.go` back into
  `agent_profiles_test.go`, `providers_config_test.go`, and `runner_test.go`;
- runtime `internal/sessions/factory_postgres_test.go` back into
  `factory_test.go`; rename the shared `sessions/postgres_test.go` fixture to a
  neutral `test_database_test.go` only if a shared fixture is still needed;
- app command `application_commands_postgres_test.go` and
  `finance_cmd_postgres_test.go` back into their matching `_test.go` files;
- app `main_file_test.go` and `engine_file_test.go` back into `main_test.go` and
  `engine_test.go` rather than preserving coverage-lane file splits;
- app `application_composition_postgres_test.go`, controller
  `finance_synthetic_link_state_postgres_test.go`, financeapp
  `register_postgres_test.go`, appdispatch `router_postgres_test.go`, jobs
  `store_postgres_test.go`, and wireup `command_roots_postgres_test.go`,
  `http_postgres_test.go`, `jobs_postgres_test.go`, and
  `migration_postgres_test.go` back into their corresponding pre-change test
  files or neutral responsibility-named files when no original owner existed;
- finance `root_service_postgres_test.go` back into
  `root_service_test_helper_test.go`.

Do not mechanically restore SQLite fixtures. Preserve PostgreSQL setup,
randomized IDs, tenant/user scoping, and the smallest demonstrated serialization
fixes while restoring file organization. The latest correction removes every
`postgres_test` source reference and preserves unrelated `!release` constraints.

### Provenance-based correction inventory

Use `main` as the byte-level baseline and the branch commits below as provenance.
This ledger is exhaustive for the rejected lane and its follow-on churn. For a
mixed file, revert only the named branch hunks; never check out the whole file.
If a changed hunk cannot be tied to a retained PostgreSQL requirement, restore it
to `main`. Do not perform opportunistic cleanup while applying this inventory.

**Independent pre-change work:** keep `b4e404b` (Crew OpenSpec overlay) and
`1a4566d` (root simplicity rule). They predate this change and are not rollback
targets.

**Operational bootstrap (`1a017af`):** keep `compose.yaml`, the one root
`postgres-bootstrap` entry point, `scripts/postgres/bootstrap.sh`,
`bootstrap-cluster.sql`, `configure-privileges.sql`,
`grant-runtime-access.sql`, and the local/test DSNs. Delete the bootstrap
contract test and restore branch-only command/config/test formatting from this
commit unless a later retained PostgreSQL hunk requires it.

**Rejected lane infrastructure (`d377e0f`, `d942c65`, `13d1262`, `9c13146`,
and `d7453f9`):** restore the root and three module Makefiles to one ordinary
profile plus the minimal tag/DSN delta; delete `.github/workflows/postgres-verify.yml`,
all three `.testcoverage-routine.yaml` files, all focused/separate targets,
workflow/target/bootstrap contracts, and routine coverage-config whitespace
cleanup. The ordinary-test chunk removes caller-side migration covdata variables,
profiles, composition, and marker checks; the following bootstrap chunk removes
the corresponding raw-covdata and marker-producing branches from
`scripts/postgres/bootstrap.sh`. Restore branch-only hunks in
`db_migrate_cmd_test.go`, config `load_test.go`/`values_test.go`, and wireup
`jobs_test.go`/`migration_test.go`. The ordinary-test chunk restores finance
`migrations_test.go` as the owner of exactly one smallest shallow smoke (tagged
at that historical stage and made normally selected by the latest correction).
The later persistence chunk retains or minimally refines only that smoke while
removing the `autoMigrator` seam, generated mocks, and replacement migration
test.

**Runtime lane conversion (`398f271`):** retain PostgreSQL fixture bodies, but
reverse all five direct renames and the extracted-file layout. The latest
correction removes the `postgres_test` constraints.
The complete touched test set is `runtime/agent/{agent_profiles_test.go,
database_services_postgres_test.go,providers_config_test.go,runner_test.go}`;
`runtime/internal/agentprofiles/db_agent_profiles_service_postgres_test.go`;
`runtime/internal/gormsumweave/dialector_test.go`;
`runtime/internal/llmproviders/db_providers_config_service_postgres_test.go`;
and `runtime/internal/sessions/{database_metadata_postgres_test.go,
database_service_postgres_test.go,database_storage_postgres_test.go,
factory_postgres_test.go,factory_test.go,postgres_test.go}`. The routine coverage
config hunk is deleted.

**App fixture extraction (`fe6a7a2`):** retain only PostgreSQL fixture content
while restoring original ownership for `cmd/sumweave/{main_test.go,
runtime_resolution_test.go}`, root `{engine_test.go}`, internal
`application_composition_test.go`, and wireup `{http_test.go,jobs_test.go,
migration_test.go}`. Delete or merge `main_file_test.go`, `engine_file_test.go`,
`application_composition_postgres_test.go`, `http_postgres_test.go`, and
`jobs_postgres_test.go`. Revert service/config factories in
`internal/{agent_runtime.go,database_migrate.go}` and
`internal/wireup/migration.go`, the shared `internal/mocks_test.go` additions,
`.mockery.yaml` additions, and `final_composition_coverage_test.go` expansion;
retain only direct PostgreSQL validation and construction.

**Finance lane conversion (`7a9d071`):** preserve required PostgreSQL fixtures,
randomized data, scoped assertions, and demonstrated isolation in every
pre-existing finance test touched by the commit. Remove
`finance/imports_parser_test.go` and
`finance/persistence/instant_predicate_test.go` when the restored owners
cover those behaviors; merge `root_service_postgres_test.go` into
`root_service_test_helper_test.go`; restore branch-only additions in
`finance/mocks_test.go`; and delete the routine config hunk. This rule covers all
other tests touched by that commit under `finance/`, including root service,
schedule, reporting, import, fixture, and `finance/persistence/*_test.go` files;
none is authorization for a wholesale restore of a converted PostgreSQL test.

**Residual app fixture/coverage commit (`276b97a`):**

- Revert production seams hunk-by-hunk in `cmd/sumweave/{finance_cmd.go,main.go,
  user_cmd.go}`, `internal/{agent_runtime.go,database_migrate.go}`,
  `internal/appdispatch/{appdispatch.go,migrator.go,router.go}`,
  `internal/appevents/events.go`, `internal/auth/refresh_store.go`,
  `internal/financeapp/register.go`, `internal/jobs/{service.go,store.go,
  worker.go}`, and `internal/wireup/{finance.go,http.go,jobs.go,migration.go}`.
- Delete coverage-only added tests/mocks:
  `appdispatch/{appdispatch_routine_test.go,migrator_unit_test.go,
  mock_message_publisher_test.go,
  mock_migration_runner_test.go}`, `appevents/{events_unit_test.go,
  mock_handler_registrar_test.go,mocks_test.go}`, auth
  `store_validation_test.go`, jobs `{durable_jobs_unit_test.go,
  mock_schema_migrator_test.go,store_migration_unit_test.go,
  worker_unit_test.go}`, wireup `{finance_module_test.go,mocks_test.go}`, and the
  branch-only shared `internal/mocks_test.go` additions.
- `appdispatch_routine_test.go` is not implicitly retained as cutover coverage:
  delete its branch-only SQL-mock cases and the file. Only if one named case is
  demonstrated to be the minimum test of retained PostgreSQL behavior may that
  case move to `appdispatch_test.go` or `transport_edges_test.go`; the routine
  file is deleted either way.
- Merge or neutrally rename retained PostgreSQL tests added/extracted in
  `cmd/sumweave/{application_commands_postgres_test.go,
  finance_cmd_postgres_test.go}`, controller
  `finance_synthetic_link_state_postgres_test.go`, appdispatch
  `router_postgres_test.go`, `application_composition_postgres_test.go`,
  financeapp `register_postgres_test.go`, jobs `store_postgres_test.go`, and
  wireup `{command_roots_postgres_test.go,migration_postgres_test.go}`.
- Restore `internal/appdispatch/import_boundary_test.go` fully to `main` and
  restore only the branch-added cases in `internal/telemetry/otel_test.go`.
  Restore branch-only formatting/build-tag churn in
  `cmd/sumweave/{application_commands_test.go,finance_cmd_test.go,
  main_file_test.go,mocks_test.go}`, `internal/api/http/ui_assets_test.go`,
  controller `mocks_test.go`, appdispatch `{appdispatch_test.go,mocks_test.go,
  transport_edges_test.go}`, `appevents/events_test.go`, auth
  `{mocks_test.go,refresh_store_test.go,user_store_test.go}`, financeapp
  `{mocks_test.go,register_test.go}`, jobs
  `{durable_jobs_workflow_test.go,mocks_test.go}`, telemetry
  `mock_slog_handler_test.go`, and wireup
  `{command_roots_test.go,http_test.go,jobs_test.go,migration_test.go}`.
- Restore regeneration-only changes in existing generated mocks under
  `internal/api/http/middleware`, `internal/app`,
  `internal/infrastructure/httpclient`, and every generated-mock file named in
  the preceding bullet. Regenerate only when a retained production interface
  independently requires it.

**PostgreSQL cutover (`29eb02b`, `29888a2`, `0fa93e4`, and `c11d789`):** keep
direct PostgreSQL dialectors, predicates, constructors, migrations, transport,
DSN rejection, fixture conversions, and SQLite production deletions. In files
also touched by earlier coverage work, remove downstream generated-mock,
test-seam, formatting, and split-file churn even when a cutover commit regenerated
or edited it. In particular, generated app mocks changed by `29888a2` do not
become required merely because regeneration occurred in that commit, and the
jobs worker periodic-cycle change/test edited by `0fa93e4` remains unrelated.

**Dependency cleanup (`b3aca72`):** keep SQLite removal in the three core module
manifests. Delete `scripts/core-dependencies-contract-test.sh`; restore
`tests/agent/integration-cli` and all `tools/{firecrawl,skills,workspacefs}`
module sums/manifests to `main`; retain only unavoidable `go.work.sum` effects
after core tidy/sync.

**Documentation and premature base specs (`5c2f0b0`):** keep truthful
PostgreSQL-only setup/product architecture and deletion of duplicate manual-E2E
Compose/role assets. Delete the documentation parser/contract scripts and revert
lane-only prose. During implementation, restore these directly edited base specs
exactly to `main`: `openspec/specs/database-migration-command/spec.md`,
`openspec/specs/domain-event-pubsub/spec.md`, and
`openspec/specs/finance-management/spec.md`. Until then, this active change's
delta specs are the sole location for the unarchived PostgreSQL requirements.
Plan-only ordering/spec commits do not authorize product changes.

### Remove coverage-driven production and generated-test overreach

Revert these production refactors to their pre-change concrete shape, then
reapply only the narrow PostgreSQL-only lines if the same file also owns a real
dialect change:

- `apps/sumweave/cmd/sumweave/{finance_cmd.go,main.go,user_cmd.go}`;
- `apps/sumweave/internal/{agent_runtime.go,database_migrate.go}`;
- `apps/sumweave/internal/appdispatch/{appdispatch.go,migrator.go,router.go}`
  only for test runner/interfaces/log-key formatting, while retaining the
  PostgreSQL transport and deleting SQLite behavior;
- `apps/sumweave/internal/appevents/events.go`;
- `apps/sumweave/internal/auth/refresh_store.go`;
- `apps/sumweave/internal/financeapp/register.go`;
- `apps/sumweave/internal/jobs/{service.go,store.go,worker.go}` only for
  mockability seams, while retaining PostgreSQL store construction and removing
  SQLite migration branches;
- `apps/sumweave/internal/wireup/{finance.go,http.go,jobs.go,migration.go}`;
- `finance/persistence/migrator.go` only for the `autoMigrator` seam, while
  retaining the early-alpha removal of retired SQLite-era schema cleanup.

Delete corresponding generated mock configuration additions, generated mock
files, database-free replacement tests, `final_composition_coverage_test.go`
expansion, and generic `coverage-ignore` churn that was introduced only to make
the rejected routine profile pass. Retain a new test only when it directly
protects changed PostgreSQL behavior and cannot be expressed in the restored
pre-change test owner.

Revert `jobs.Worker.runPeriodicRecoveryCycle`, its router factory, and
`worker_unit_test.go` unconditionally. That periodic cancellation behavior was
not introduced by the PostgreSQL cutover. A separately approved bug-fix change
may reintroduce it later; this correction must not retain it based on a new test.

### Remove contract machinery and unrelated manifest churn

Delete the standalone workflow and contract machinery:

- `.github/workflows/postgres-verify.yml`;
- `scripts/postgres/bootstrap-contract-test.sh`;
- `scripts/postgres/targets-contract-test.sh`;
- `scripts/postgres/workflow-contract-test.sh`;
- `scripts/postgres/documentation-contract-parser.py`;
- `scripts/postgres/documentation-contract-test.sh`;
- `scripts/core-dependencies-contract-test.sh`.

Tidy `runtime`, `finance`, and `apps/sumweave` in module-local mode first. Run
workspace synchronization only if required, then restore unrelated template and
integration-harness `go.mod`/`go.sum` files to `main` last so synchronization
cannot recreate the churn being removed. Inspect every core/non-core manifest
and `go.work.sum` after that final restoration. Keep core SQLite dependency
removals and only unavoidable workspace-sum changes; use `go mod why` to accept
ADK test-only SQLite metadata only when it is outside the production package
graph.

The final review found one narrower dependency correction: a module-local tidy
of `apps/sumweave` must remove only the stale indirect requirements
`github.com/ncruces/go-strftime`, `modernc.org/mathutil`, and
`modernc.org/memory`. Apply exactly that tidy diff, immediately prove all three
core modules are tidy and verified, and do not change any other manifest or
workspace sum.

The three base specs under `openspec/specs/` were edited prematurely while this
change is active. Restoring them is an implementation action, not part of plan
authoring. The unchecked documentation/spec implementation chunk restores each
file to `main`; archive will later apply the corrected delta normally.

### Keep documentation changes narrowly truthful

Keep PostgreSQL-only database configuration, canonical Compose bootstrap, PM2
prerequisites, and removal of obsolete duplicate manual-E2E Compose assets.
Remove statements that ordinary backend module or Nx tests are
database-independent or that a separate PostgreSQL lane/workflow exists. Update
only files that currently make one of those backend claims; avoid broader prose
rewrites.

Root and module `AGENTS.md` files must state that PostgreSQL is a normal
prerequisite and that ordinary tests include database-backed tests without a
custom build tag. The root
rule `Routine CI tests must not require PostgreSQL to be available` is replaced
with the corrected one-line rule; no second competing rule remains. Restore the
lane-only addition in `build/AGENTS.md` to `main` rather than adding a backend
test rule there. Preserve `tests/AGENTS.md` prose that the separate,
template-origin agent harness remains database-independent, while retaining its
manual browser E2E bootstrap guidance.

Those command-facing changes land atomically with the Makefile/test-flow switch,
not in the later prose cleanup. That commit owns root/runtime/finance/app
`AGENTS.md`, restoration of `build/AGENTS.md`, and the stale command claims in
`docs/ARCHITECTURE.md`, `docs/database-backed-state-plan.md`,
`docs/manual-e2e/postgres-local-verification.md`, and
`apps/sumweave/doc/architecture.md`. The later prose chunk owns only remaining
non-command setup/product/manual-E2E wording and exact base-spec restoration.
For this latest correction, replace only the four existing stale AGENTS claims;
do not add another root project rule.

## Risks / Trade-offs

- Ordinary backend tests now require PostgreSQL. The single bootstrap command
  and CI setup step make that dependency explicit.
- Shared test state can expose collisions. Preserve randomized/scoped fixture
  corrections and serialize only demonstrated conflicts rather than creating a
  generic isolation framework.
- Removing coverage-only seams may reveal gaps under the unchanged 90% gate.
  Prefer existing PostgreSQL integration coverage or one shallow smoke.
  For the concrete finance migrator, share only its duplicate direct GORM error
  handling and add one canceled-context assertion in that same smoke owner; do
  not recreate a second lane, broad exclusions, or mockable abstractions.
- CI bootstrap adds infrastructure time to every workflow execution. This is the
  accepted cost of testing the sole supported persistence implementation in the
  ordinary path.

## Ordered Implementation Chunks

1. Add CI bootstrap before the unchanged ordinary Nx test step and verify it in
   the same chunk.
2. Historically, collapse Makefiles/coverage to one then-tagged ordinary flow, remove
   all caller-side lane/coverage machinery, add the smallest shallow tagged
   smoke in `finance/persistence/migrations_test.go`, and make every
   command-facing AGENTS/doc truthful before running ordinary and repository
   checks.
3. Delete contract/parser scripts and remove bootstrap-owned external,
   migration-covdata, and readiness-marker branches; verify fresh and idempotent
   Compose bootstrap immediately.
4. Restore runtime test ownership and names, retaining and verifying tagged
   PostgreSQL fixtures.
5. Restore root finance test ownership and retained PostgreSQL fixtures.
6. Restore finance persistence/migration ownership, remove its mock seam and
   replacement suite, preserve both wrapped migration errors through one private
   concrete GORM routine, and add only one canceled-context error assertion to
   the task 2.2 smoke without adding duplicate migration-test ownership.
7. Roll back `cmd/sumweave` coverage seams with package-owned tests/mocks.
8. Restore root app Engine test ownership.
9. Roll back app-internal composition factories and coverage file splits.
10. Roll back appdispatch seams/tests, explicitly disposing of
    `appdispatch_routine_test.go`, while retaining PostgreSQL transport.
11. Roll back appevents coverage seams/tests.
12. Roll back auth coverage seams/tests while preserving PostgreSQL stores.
13. Roll back jobs schema/router seams, tests, and periodic behavior
    unconditionally while preserving its PostgreSQL store.
14. Restore financeapp and controller test ownership and remove their
    coverage-only mock churn.
15. Roll back wireup package seams/tests while retaining direct composition.
16. Restore provenance-confirmed ancillary config, HTTP/middleware, app,
    HTTP-client, and telemetry test/generated-mock churn in one practical
    restoration chunk with explicit package ownership.
17. Reconcile shared Mockery configs and remaining generated output.
18. Tidy core modules locally, synchronize only if required, restore non-core
    manifests last, and verify final dependency traversal and manifest diffs.
19. Align only remaining uncoupled prose and restore the three premature base
    specs to `main`; run strict OpenSpec validation and its scoped diff audit.
20. Apply the final app module-local tidy correction for only the three stale
    indirect requirements, then immediately verify tidy state, module integrity,
    bootstrap, and app/affected checks without broader manifest changes.
21. Remove all 84 `postgres_test` source occurrences across the 76 core test
    files, preserving only the three unrelated `!release` constraints; remove
    the corresponding three module test flags and finance lint flag; update only
    stale root/module AGENTS guidance; then run residue, bootstrap, module, root,
    affected, and strict OpenSpec checks in the same serialized correction chunk.
22. Move the shared test DSN out of the three module Makefiles: export it from
    root `.envrc` for standard local shells, set it on the existing reusable CI
    Nx test step, and let module/root/Nx children inherit it without changing
    bootstrap, app config mapping, test helpers, or Nx project definitions.

All backend chunks are serialized, and each task embeds its own applicable TDD
or restoration rationale plus immediate verification. Each production seam, its
generated mock, and its replacement tests are removed in the same package-owned
chunk so every reviewed intermediate commit remains buildable and reviewable.
There is no separate final-verification implementation task. After each chunk, a
fresh chunk reviewer performs only the quick, shallow gate: confirm reported
checks are green and the chunk appears complete, without deep review or
nit-picking. After all chunks, a fresh final reviewer performs the deep
`main...HEAD` provenance/residual-diff audit, confirms the accumulated checks,
reruns final-state completion checks when required, and returns a clean verdict
or ordered follow-up chunks.

## Open Questions

There are no blocking design questions. If untagging exposes a package-level
helper or test-name conflict, resolve it minimally within the existing package
and owner; do not add a package split, replacement build tag, or alternate lane.
The currently tagged files already compile together with untagged tests in the
ordinary tagged command, so no duplicate declaration is presently expected.
