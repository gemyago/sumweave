## 1. PostgreSQL Environment Provisioning

- [ ] 1.1 Establish the canonical local PostgreSQL Docker Compose environment with separate regular and test databases, readiness checks, fixed local-only credentials, and matching local/test config defaults; follow TDD flow by adding failing setup/config contract coverage before the environment assets and proving both environments select the intended database.
- [ ] 1.2 Prepare both local databases through the existing migration command and document `sumweave db-migrate --env local` plus `sumweave db-migrate --env test` as required setup; follow TDD flow by updating migration/setup assertions before changing the repository setup and PM2 guidance.
- [ ] 1.3 Provision the same regular/test database shape in the reusable GitHub Actions workflow using the Ubuntu PostgreSQL service and `pg_isready`, then run `sumweave db-migrate` for both environments before affected targets; follow TDD flow by adding a failing workflow contract assertion before changing the workflow.

## 2. Generic Runtime PostgreSQL Cutover

- [ ] 2.1 Convert runtime database-backed tests to the migrated test database with fresh randomized users, sessions, profiles, providers, and other owned identities; follow TDD flow by moving existing behavior cases first and resolving only demonstrated shared-state conflicts in the affected tests.
- [ ] 2.2 Convert runtime sessions, provider configuration, agent profiles, GORM predicates, dialector, and connection coverage to PostgreSQL and remove all runtime SQLite branches and files; follow TDD flow by moving each affected behavior to the PostgreSQL test database before simplifying its production implementation.

## 3. Finance PostgreSQL Cutover

- [ ] 3.1 Convert finance persistence and composition coverage to the migrated test database with fresh randomized tenants and tenant-scoped domain data; follow TDD flow by moving existing behavior cases first and resolving only demonstrated shared-state conflicts before replacing SQLite fixtures.
- [ ] 3.2 Simplify finance database construction and instant predicates to PostgreSQL-only behavior and remove finance SQLite connection code and dependencies; follow TDD flow by updating constructor, timestamp, persistence, and composition expectations before deleting each dialect branch.

## 4. Sumweave Application PostgreSQL Cutover

- [ ] 4.1 Convert command, config, auth, jobs, finance registration, controller, Engine, migration, and wireup coverage to the migrated test database with fresh randomized identities and scoped reads; follow TDD flow by moving existing behavior cases first and resolving only demonstrated shared-state conflicts before removing SQLite setup.
- [ ] 4.2 Make app SQL connections, authentication, durable jobs, and related query predicates PostgreSQL-only; follow TDD flow by updating their PostgreSQL-backed constructor and behavior cases before deleting DSN classification, SQLite dialectors, and SQLite query variants.
- [ ] 4.3 Make appdispatch publication, subscription, offsets, and schema migration unconditionally use the existing PostgreSQL Watermill adapters; follow TDD flow by preserving transport fan-out, same-group coordination, restart resume, transaction, retry, dead-letter, and shallow migration behavior on PostgreSQL before deleting the custom SQLite transport and multi-driver selection.

## 5. Dependency And Contract Cleanup

- [ ] 5.1 Remove SQLite drivers and their transitive dependency tree from every active Go module and synchronize workspace sums; follow TDD flow by adding a failing active-module dependency check before tidying product, tool, and integration-test modules and confirming only PostgreSQL drivers remain.
- [ ] 5.2 Align active architecture, module guidance, local setup, PM2 workflow, manual E2E guides, and database plans with mandatory Compose PostgreSQL, separate regular/test databases, two `db-migrate` setup runs, and no SQLite data migration; follow TDD flow by updating any documentation/config assertions before changing active docs and preserving archived OpenSpec records as history.

## 6. Test CI

<!-- Orchestrator: execute each distinct step in this group with a separate sub-agent. Use one sub-agent to create the PR, another to wait for each CI run, and a fresh sub-agent for each investigation/fix cycle. After a fix is pushed, delegate the next CI wait separately and repeat until green. -->

- [ ] 6.1 Create a pull request after tasks 1–5 and the local completion protocol succeed, then record the PR and its required checks; this orchestration-only step does not change implementation code.
- [ ] 6.2 Wait for the pull request CI run to finish and report the exact check results; this orchestration-only step does not make speculative fixes.
- [ ] 6.3 Investigate any failed CI check, reproduce it, implement and locally verify the scoped fix, push it to the pull request, and return to task 6.2 until all required checks are green; follow TDD flow for every implementation fix by reproducing the failure before changing code.
