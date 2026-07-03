## Context

`finance/service.go` currently defines the public root `Service`, a large `serviceStore` interface, shared options, tenant/catalog/ledger delegates, and public methods used by app wiring, HTTP controllers, jobs, fixture bootstrap, and tests. Some internal implementation is already split into `tenantService`, `catalogService`, and `ledgerService`, and bank-linking has a focused public `BankConnectionService`; however, callers still depend on root `Service` for most finance workflows, while FX, reporting, CSV import, and bank sync methods remain attached directly to the root type.

The finance module rules say outside code should use services exposed by `Finance`, the exposed shape should be minimal, and each public service must be single-responsibility. The persistence rule also says not to extend the legacy broad `persistence.Store` god object with new methods.

## Goals / Non-Goals

**Goals:**

- Make `finance.Finance` the app-facing composition root for focused finance services.
- Expose dedicated public services for tenant management, catalog management, ledger transactions, reporting, FX synchronization/diagnostics, CSV imports, bank-link workflows, and bank-sync workflows.
- Remove app, controller, job, CLI, and fixture reliance on broad `finance.Service`.
- Split service dependencies so each focused service accepts only the stores, clocks, ID generators, ciphers, providers, enqueuers, and schedulers it needs.
- Keep domain behavior and externally visible HTTP/job contracts unchanged.

**Non-Goals:**

- No UI work.
- No new finance product feature.
- No OpenAPI route or JSON shape change.
- No database schema migration solely for this split.
- No compatibility wrapper that keeps root `finance.Service` as the preferred public entrypoint.
- No full rewrite of provider sync v2 internals.

## Decisions

1. Expose focused services from the `finance.New` / `finance.Finance` public contract.

   `finance.New` should build a `finance.Finance` value whose public fields expose tenant, catalog, ledger, reporting, FX, CSV import, bank-link, and bank-sync services. Module-level tests should prove this is the public composition contract, that focused services are reachable through `Finance`, and that the contract no longer requires root `finance.Service` for active product workflows. App DI should then receive the specific service required by each controller or job handler. This follows the module rule that the outside world uses the `Finance` instance while avoiding a root facade with unrelated methods.

   Alternative considered: keep `Service` as a compatibility facade over focused services. That would reduce mechanical churn, but it preserves the public god-service shape the change is meant to retire.

2. Promote current delegates before rewriting behavior.

   Existing tenant, catalog, and ledger implementations can become exported service types with focused constructors and tests. Reporting, FX, CSV import, and bank sync should then receive the same treatment. This is a boundary refactor, not an algorithm rewrite.

   Alternative considered: rewrite the implementation internals while splitting boundaries. That increases risk and makes behavior regressions harder to identify.

3. Split store contracts by responsibility.

   Each service should define consumer-owned store interfaces near the service that consumes them. If a workflow needs persistence behavior not already available without depending on the broad store, add a dedicated store wrapper or narrow persistence interface instead of adding new methods to the legacy store contract.

   Alternative considered: keep `serviceStore` and pass it to every service. That would make the public service names cleaner while leaving the real dependency tangle intact.

4. Move controllers and jobs to service-specific dependencies.

   The finance HTTP controller currently has one broad finance dependency plus a bank connection dependency. It should be split so tenant/account/category/tag/transaction/reporting/import/bank-sync endpoints, plus FX diagnostics and FX sync endpoints, depend only on the focused interface they call. Finance job handlers should similarly depend on FX, CSV import, and bank-sync services.

   Alternative considered: split only package-level service types and leave app/controller interfaces broad. That would keep the practical call graph monolithic.

5. Use strict TDD for each migration chunk.

   For each service boundary, update or add failing tests at the public service and caller boundary first, then move implementation and wiring, then run the targeted package tests before advancing to the next chunk. Repository completion checks remain the final gate.

## Risks / Trade-offs

- [Large mechanical diff] -> Migrate one responsibility at a time and keep behavior tests focused on unchanged results.
- [Controller mock churn] -> Update generated or mockery-based mocks following existing app patterns in the same chunk that changes the interface.
- [Bank link versus bank sync naming confusion] -> Keep link start/finish/token concerns separate from connection listing, deletion, scheduling, and sync execution.
- [Hidden root-service callers remain] -> Add caller-audit checks in the final chunk and remove or make unexported any surviving root facade.
- [Fixture bootstrap needs multiple services] -> Introduce a small fixture-facing dependency shape instead of passing the entire finance module where only a few methods are required.

## Migration Plan

1. Promote tenant, catalog, and ledger services to public focused services and update their direct tests.
2. Move reporting, FX, CSV import, and bank sync onto public focused services with narrow dependencies.
3. Recompose the `finance.New` / `finance.Finance` contract so module tests and app wiring both receive focused services from the module entrypoint.
4. Split HTTP controller and job handler dependencies by focused service, including FX controller routes.
5. Update CLI and fixture bootstrap paths to use focused services.
6. Remove the root-service facade and broad service store dependency once no active caller needs them.

## Open Questions

- None.
