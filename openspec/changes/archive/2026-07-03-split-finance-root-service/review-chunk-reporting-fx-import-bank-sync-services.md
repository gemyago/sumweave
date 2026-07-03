# Review Chunk: reporting-fx-import-bank-sync-services

## Implementation round 2026-07-03

- Result: needs-follow-up
- Phase: initial implementation phase
- OpenSpec apply:
  - Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change split-finance-root-service --task 2`
  - Installed CLI still does not provide `apply` (`unknown command 'apply'`), so the approved chunk was implemented directly and the standard OpenSpec artifacts were updated.

### What changed

- Promoted reporting workflows to focused public `finance.ReportingService` and moved dashboard loading/computation through that service.
- Promoted FX sync and diagnostics workflows to focused public `finance.FXService` with service-local option hooks for providers, default provider, job enqueuer, and schedule writer.
- Promoted CSV import preview/confirm/run/audit workflows to focused public `finance.CSVImportService` with explicit catalog and ledger collaborators instead of root `finance.Service` behavior.
- Promoted bank-sync list/delete/schedule/trigger/run/apply workflows to focused public `finance.BankSyncService` while leaving bank-link workflows on the existing bank-link paths.
- Rebound root `finance.Service` methods for section 2 responsibilities to the focused services so later chunks can rewire callers without behavior changes.
- Added focused failing-then-passing finance tests proving reporting, FX, CSV import, and bank-sync workflows work without root `finance.Service`.

### Files changed

- `finance/service.go`
- `finance/reporting.go`
- `finance/fx.go`
- `finance/imports.go`
- `finance/provider_sync.go`
- `finance/service_reporting.go`
- `finance/service_fx.go`
- `finance/service_csv_import.go`
- `finance/service_bank_sync.go`
- `finance/focused_public_services_test.go`
- `openspec/changes/split-finance-root-service/tasks.md`
- `openspec/changes/split-finance-root-service/manager-status.md`
- `openspec/changes/split-finance-root-service/review-chunk-reporting-fx-import-bank-sync-services.md`

### TDD evidence

- Extended `finance/focused_public_services_test.go` first with reporting, FX, CSV import, and bank-sync focused-service cases.
- Ran `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run TestFocusedPublicServices` and observed the expected failing compile because the new focused public services and options did not exist yet.
- Implemented the focused services, rebound the root service delegates, and re-ran the targeted test successfully.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change split-finance-root-service --task 2` *(fails: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run TestFocusedPublicServices` *(initial failing TDD run)*
- `direnv exec /Users/jenya/projects/signal-foundry gofmt -w finance/service.go finance/reporting.go finance/fx.go finance/imports.go finance/provider_sync.go finance/service_reporting.go finance/service_fx.go finance/service_csv_import.go finance/service_bank_sync.go finance/focused_public_services_test.go`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run TestFocusedPublicServices`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance`
- `direnv exec /Users/jenya/projects/signal-foundry make lint` *(pass in `finance/` after focused-service cleanup)*
- `direnv exec /Users/jenya/projects/signal-foundry make test` *(fails in `finance/`: file-coverage thresholds remain below 90% for `provider_sync.go`, `service_bank_sync.go`, and `service_csv_import.go`)*
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` *(fails for the same finance coverage-threshold blockers)*

### Task status updates

- Marked `tasks.md` items `2.1`, `2.2`, `2.3`, and `2.4` complete.

### Artifact cleanup

- Mostly clean. Added one dedicated coverage-focused test file:
  - `finance/focused_service_error_coverage_test.go`
  This file is required to exercise delegation and focused-service error branches; it is currently tracked as new work for this chunk but should be included in final diff.

### Blockers

- Finance test coverage gate still fails:
  - `provider_sync.go` below 90% file coverage
  - `service_bank_sync.go` below 90% file coverage
  - `service_csv_import.go` below 90% file coverage

## Implementation round 2026-07-03

- Result: blocked
- Phase: verification/update phase
- OpenSpec apply:
  - Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change split-finance-root-service --task 2`
  - Installed CLI still does not support `apply` (`unknown command 'apply'`), so OpenSpec artifacts were updated manually.

### What changed

- Reconfirmed section-2 code paths were implemented for reporting, FX, CSV import, and bank-sync and delegated from root service.
- Reaffirmed task ledger updates:
  - `openspec/changes/split-finance-root-service/tasks.md` now has `2.1`–`2.4` marked complete.
- Confirmed `openspec/changes/split-finance-root-service/review-chunk-reporting-fx-import-bank-sync-services.md` now includes this follow-up verification round and the blocker details.

### Files observed

- `finance/service.go`
- `finance/reporting.go`
- `finance/fx.go`
- `finance/imports.go`
- `finance/provider_sync.go`
- `finance/service_reporting.go`
- `finance/service_fx.go`
- `finance/service_csv_import.go`
- `finance/service_bank_sync.go`
- `finance/focused_public_services_test.go`
- `finance/focused_service_error_coverage_test.go`
- `openspec/changes/split-finance-root-service/tasks.md`
- `openspec/changes/split-finance-root-service/manager-status.md`
- `openspec/changes/split-finance-root-service/review-chunk-reporting-fx-import-bank-sync-services.md`

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run TestFocusedPublicServices`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run TestFocusedServiceErrorCoverage`
- `direnv exec /Users/jenya/projects/signal-foundry make lint`
- `direnv exec /Users/jenya/projects/signal-foundry make test` (finance file-coverage threshold fails on provider/bank-sync/csv-import files)
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` (same finance coverage blockers)

### Completion protocol status

- `make affected-lint-test` was run and reported failures.
- Because of coverage gate failures, repository module completion protocol is **not complete** for this chunk.

### Obvious issues review

- No regressions were observed in the covered section-2 integration paths.
- Root `finance.Service` currently delegates to focused services, preserving existing behavior for section-2 methods while leaving bank-link flows intact.

### Artifact cleanup status

- Not fully clean yet: one required new test file is untracked (`finance/focused_service_error_coverage_test.go`) and should be included or folded into an existing chunk test file.

### Blockers

- Finance test coverage gate still blocks `make affected-lint-test`:
  - `provider_sync.go`: 83.9% (266/317) `< 90%`
  - `service_bank_sync.go`: 85.9% (378/440) `< 90%`
  - `service_csv_import.go`: 89.2% (189/212) `< 90%`
