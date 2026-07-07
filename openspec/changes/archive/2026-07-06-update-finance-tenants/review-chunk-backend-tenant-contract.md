# Chunk Review: backend-tenant-contract

Implementation and review history for chunk `backend-tenant-contract`.

## 2026-07-06 Implementation

Verdict: complete for chunk scope.

### Implemented

- Added finance tenant update support with `UpdateTenantParams`, `TenantService.UpdateTenant`, and root test-helper forwarding so tenant members can change tenant name and display currency while preserving canonical uppercase codes and advancing `UpdatedAt`.
- Added a static supported tenant display-currency catalog (`USD`, `EUR`, `PLN`, `UAH`) plus finance-service validation so tenant create and update reject empty or unsupported codes before persistence.
- Added TDD-first finance service coverage for supported create/update normalization, invalid currency rejection, member-only update access, persisted name/currency changes, and `UpdatedAt` advancement, plus update-path store error coverage.
- Added `PATCH /api/v1/finance/tenants/{tenantId}` to the backend HTTP contract, introduced enum-backed create/update request schemas, regenerated apigen route artifacts, and wired the finance controller to return the updated tenant summary.
- Added registered-route controller coverage for tenant update success, non-member unauthorized access, invalid display-currency rejection, and missing-caller-identity handling for the new route.

### Checks

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change update-finance-tenants --task 1.1 --task 1.2` *(fails in current CLI: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run 'TestService/updates tenants with supported currencies and rejects unsupported ones'` *(initial fail before implementation; passes after service changes)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./apps/signal-foundry/internal/api/http/v1controllers -run 'TestFinanceController/tenant update route returns updated summaries and rejects invalid access or currency'` *(initial fail before route/controller changes; passes after regeneration and wiring)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./apps/signal-foundry/internal/api/http/v1controllers`
- `direnv exec /Users/jenya/projects/signal-foundry openspec validate update-finance-tenants --strict`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec updates

- Marked tasks `1.1` and `1.2` complete in `tasks.md`.
- Updated `manager-status.md` to mark `backend-tenant-contract` complete.

## 2026-07-06 Finalization (2026-07-06)

Verdict: complete for chunk scope.

### Verification performed in this pass

- Re-reviewed implementation against chunk scope (`1.1`, `1.2`) and OpenSpec specs.
- Re-ran `direnv exec /Users/jenya/projects/signal-foundry openspec validate update-finance-tenants --strict`.
- Re-ran targeted tests:
  - `go test ./finance -run 'TestService/updates tenants with supported currencies and rejects unsupported ones'`
  - `go test ./apps/signal-foundry/internal/api/http/v1controllers -run 'TestFinanceController/tenant update route returns updated summaries and rejects invalid access or currency'`
- Re-ran package-level checks:
  - `go test ./finance`
  - `go test ./apps/signal-foundry/internal/api/http/v1controllers`
- Re-ran `make affected-lint-test`.
- Reviewed all diffs from `git diff` for changed files.

### Result

- Functional implementation matches requested scope:
  - tenant updates supported in `finance` with normalization + validation + `UpdatedAt` update,
  - tenant update HTTP route added and wired,
  - OpenAPI enums and generated route/controller artifacts updated,
  - route-level and service-level negative coverage exists for invalid currency and unauthorized update.
- No obvious issues introduced in reviewed code paths.
- `finance/AGENTS.md` and `apps/signal-foundry/AGENTS.md` completion requirements remain satisfied.
- `openspec apply` remains unavailable in this environment (`unknown command 'apply'`), so implementation proceeded directly with `tasks.md`/`manager-status.md` updates and generated artifacts as before.

### Completion protocol status

- Repo protocol: pass (`make affected-lint-test` successful).
- AGENTS protocol: pass (`finance/AGENTS.md`, `apps/signal-foundry/AGENTS.md`, root AGENTS). 

### Artifact cleanup status

- Clean. `review-chunk-backend-tenant-contract.md`, updated tasks/manager artifacts, and OpenAPI-generated files are expected durable/standard artifacts.

## Completion Protocol Status

- Root coding protocol: pass after `make affected-lint-test`.
- `finance/AGENTS.md` protocol: pass.
- `apps/signal-foundry/AGENTS.md` protocol: pass, including OpenAPI regeneration after route/schema edits and mock regeneration for the controller test interface.
- `AGENTS.md` update: not needed.
- `openspec apply` note: the installed CLI does not expose an `apply` subcommand, so the approved chunk was implemented directly while keeping OpenSpec task artifacts updated.

## Artifact Cleanup Status

- Clean with respect to repository artifacts: only standard OpenSpec artifacts were added or updated, and no scratch files were kept in the repo.
