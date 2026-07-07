# Planning Review

## Round 1 — 2026-07-06

### Verdict

Approved for implementation. The proposal, design, tasks, and spec deltas cleanly cover the requested tenant update support, bounded display-currency create/update behavior, affected backend and UI surfaces, and an ordered implementation path.

### Review Notes

- Tenant update support is explicit in `proposal.md`, `design.md`, `tasks.md`, and `specs/finance-management/spec.md`: joined tenant members can update tenant name and display currency, membership/invite/archive/account/transaction semantics remain unchanged, and `UpdatedAt` must advance.
- Currency selection is bounded for both create and update: backend validation is required at the finance service boundary, OpenAPI should express the enum where possible, and the UI uses select controls backed by the same static product-supported list. The design deliberately starts with known valid ISO 4217 codes including `USD`, `EUR`, `PLN`, and `UAH` without adding a runtime catalog endpoint.
- Backend surfaces are covered: `finance/` service contracts/validation/tests, `apps/signal-foundry` OpenAPI routes, generated apigen bindings/models, controller wiring, and controller tests.
- UI surfaces are covered: finance API/page layer, `FinanceTenants.svelte`, UI tests, shell selected-tenant refresh behavior, and `apps/signal-ui/ui-wireframe.md` documentation.
- Spec hierarchy is consistent: `design.md` follows `proposal.md`, and `tasks.md` implements the proposal through the design without adding unrelated scope.
- OpenSpec validation passed with `openspec validate update-finance-tenants --strict`.

### Findings

None.

### Strict Ordered Chunk Plan

- Chunk 1: Complete parent task 1 only — backend tenant contract.
  - Implement 1.1 before 1.2.
  - Add failing finance service tests first, then tenant currency catalog/validation and update service behavior.
  - Add failing registered-route controller tests next, then OpenAPI route/schema changes, apigen regeneration, controller wiring, and backend test/lint checks.
- Chunk 2: Complete parent task 2 only — tenant management UI.
  - Implement 2.1 before 2.2.
  - Add failing UI tests for bounded create currency selection before replacing the create free-text field.
  - Add failing UI/API tests for selected-tenant update, shared currency selector, update API call, shell tenant-state refresh, and recoverable errors before implementing UI/API changes.
- Chunk 3: Complete parent task 3 only — documentation alignment.
  - Update `apps/signal-ui/ui-wireframe.md` after backend and UI behavior exists.

Do not combine non-consecutive parent tasks. Keep chunks sequential unless the manager explicitly changes execution order.

### Artifact Cleanup

Clean. The change directory contains only OpenSpec artifacts and standard manager/review artifacts: proposal, design, tasks, README, `.openspec.yaml`, spec deltas, `manager-status.md`, `review-planning.md`, and `review-final.md`. No ad-hoc repository artifacts were found.

### Commit Recommendation

Plan is clean and ready for implementation. Commit pending planning and review artifacts per the planning review commit rule.

## Round 2 — 2026-07-06 User Review Comment Planning

### Comment Addressed

- `apps/signal-foundry/internal/api/http/v1routes.yaml` line 791: the tenant update mutating API should return only minimal required response data, not tenant entity data.

### Planning Update

- Updated `design.md` Decision 5 to replace the updated-tenant-summary response with success-only `204 No Content` because no response data is required; the UI already refreshes tenant state after successful update.
- Updated `tasks.md` with a single correction chunk, `minimal-mutating-response`, ordered as backend OpenAPI/controller/test/codegen alignment first, then UI API adapter/test alignment.

### Ordered Comment Chunk Plan

- Chunk 4: Complete parent task 4 only — minimal mutating response.
  - Implement 4.1 before 4.2.
  - Add/update failing registered-route controller tests first so tenant update success expects `204 No Content` with no tenant entity body, then edit `v1routes.yaml`, regenerate apigen output, and simplify controller wiring to stop listing/returning `FinanceTenantSummary` after update.
  - Add/update failing UI API/page tests next for a success-only update response, then change the Finance UI API adapter to resolve without entity data while preserving `financeShell.refreshTenants()` and `loadTenantDetails()` after success.

### Artifact Cleanup

Clean. No ad-hoc repository artifacts were found or created; the comment round only updates standard OpenSpec/workflow artifacts.

### Blockers

None.
