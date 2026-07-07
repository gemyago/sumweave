## 1. Backend tenant contract

- [x] 1.1 Add tenant display-currency validation and update behavior in `finance/`, and must follow TDD flow by first adding failing tenant-service tests for supported currency create/update, unsupported currency rejection, member-only update access, name/currency persistence, and `UpdatedAt` changes before implementing the validation catalog and update service method.
- [x] 1.2 Add the tenant update HTTP contract in `apps/signal-foundry`, and must follow TDD flow by first adding failing registered-route controller tests for `PATCH /api/v1/finance/tenants/{tenantId}` success, unauthorized or non-member access, and invalid display-currency rejection before editing `v1routes.yaml`, regenerating apigen output, and wiring the controller method.

## 2. Tenant management UI

- [x] 2.1 Add bounded tenant currency options to the Finance tenant API/page layer, and must follow TDD flow by first adding failing UI tests proving tenant create uses a select with supported currency codes and submits the selected code before replacing the free-text input.
- [x] 2.2 Add selected-tenant update controls on the Finance tenants page, and must follow TDD flow by first adding failing UI/API tests proving the edit form pre-fills the selected tenant, uses the same currency select, calls the tenant update API, refreshes shell tenant state, and shows recoverable errors before implementing the UI and API wrapper changes.

## 3. Documentation alignment

- [x] 3.1 Update Finance UI behavior documentation, and must follow TDD flow by first identifying the tenant-management sections in `apps/signal-ui/ui-wireframe.md`, then documenting that tenant create/update currency is selected from supported codes and that selected tenant name/display currency can be updated from the tenants route.

## 4. User-review correction: minimal mutating response

- [x] 4.1 Change the tenant update HTTP contract to return success-only minimal data, and must follow TDD flow by first updating registered-route controller tests to expect `PATCH /api/v1/finance/tenants/{tenantId}` success as `204 No Content` with no tenant entity body before editing `v1routes.yaml`, regenerating apigen output, and simplifying controller wiring to avoid listing/returning the updated tenant summary.
- [x] 4.2 Align the Finance UI API adapter with the success-only update response, and must follow TDD flow by first updating API/page tests that mocked or asserted the tenant update return value, then changing `updateTenant` to resolve without entity data while preserving the existing post-success tenant refresh behavior.
