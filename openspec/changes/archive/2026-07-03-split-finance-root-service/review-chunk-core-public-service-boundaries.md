# Review Chunk: core-public-service-boundaries

## Implementation round 2026-07-03

- Result: complete
- Phase: initial implementation phase
- OpenSpec apply:
  - Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change split-finance-root-service --task 1`
  - Installed CLI still does not provide `apply` (`unknown command 'apply'`), so the approved chunk was implemented directly and the standard OpenSpec artifacts were updated.

### What changed

- Promoted tenant, catalog, and ledger workflows to focused public services:
  - `finance.TenantService`
  - `finance.CatalogService`
  - `finance.LedgerService`
- Added focused public constructors with narrow consumer-owned store interfaces and service-local option hooks for clock and ID injection.
- Rebound `finance.Service` core delegates through the new focused public services while keeping later non-core responsibilities untouched.
- Removed the obsolete broad tenant-scoped access helper file once the focused services owned their lookup logic.
- Added focused failing-then-passing finance tests proving tenant, catalog, and ledger workflows work without root `finance.Service`.
- Updated internal focused-service tests to use the new public boundaries and helper paths.

### Files changed

- `finance/service.go`
- `finance/service_tenants.go`
- `finance/service_catalog.go`
- `finance/service_ledger.go`
- `finance/service_internal_test.go`
- `finance/focused_public_services_test.go`
- `finance/service_access.go` (deleted)
- `openspec/changes/split-finance-root-service/tasks.md`
- `openspec/changes/split-finance-root-service/manager-status.md`
- `openspec/changes/split-finance-root-service/review-chunk-core-public-service-boundaries.md`

### TDD evidence

- Added `finance/focused_public_services_test.go` first.
- Ran `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run TestFocusedPublicServices` and observed the expected failure because `NewTenantService`, `NewCatalogService`, and `NewLedgerService` did not exist yet.
- Implemented the focused services and re-ran the targeted test successfully.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change split-finance-root-service --task 1` *(fails: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run TestFocusedPublicServices` *(initial failing TDD run)*
- `direnv exec /Users/jenya/projects/signal-foundry gofmt -w finance/service.go finance/service_tenants.go finance/service_catalog.go finance/service_ledger.go finance/service_internal_test.go finance/focused_public_services_test.go`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run TestFocusedPublicServices`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### Task status updates

- Marked `tasks.md` items `1.1`, `1.2`, and `1.3` complete.

### Artifact cleanup

- Clean. No ad-hoc repository artifacts were created.

### Blockers

- None for this chunk.

## Finalization round 2026-07-03

- Result: complete
- Phase: final review
- OpenSpec apply:
  - Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change split-finance-root-service --task 1`.
  - CLI does not currently expose `apply` (`unknown command 'apply'`), so the chunk was implemented directly and then documented.

### Completion check

- Ran `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` from repo root.
  - Result: pass (lint/test completed for signal-foundry, integration-cli, finance).
- Re-ran focused finance package tests implicitly by full lint/test pipeline:
  - `finance` tests and lint passed.

### Chunk review verdict

- Scope mapping: ✅ tenant, catalog, and ledger public service promotion completed in task items `1.1`, `1.2`, `1.3`.
- Boundary split correctness: ✅ root `Service` delegates now route through `TenantService`, `CatalogService`, and `LedgerService`; core tenant lookups now remain in focused services.
- Store-interface narrowing: ✅ each focused service has dedicated interfaces and constructors with override options for `now` and `newID`.
- Test coverage: ✅ added focused service tests proving workflows function without root `Service`; existing internal tests updated accordingly.
- Artifact cleanup: ✅ no ad-hoc artifacts introduced.

### Follow-up

- Next chunk remains: `section 2. Reporting, FX, Import, And Bank Sync Services`.

### Notes

- Continue decision: ✅ safe to continue
- Continue blockers: none.
