## Context

Finance tenant creation currently normalizes `DisplayCurrency` to uppercase but does not verify it against a product-supported currency catalog. The tenant management page uses a free-text display-currency input when creating tenants and has no edit form for an existing tenant. Backend tenant persistence already stores `Name`, `DisplayCurrency`, and `UpdatedAt`, so this change can reuse the existing tenant row shape and `SaveTenant` path rather than adding schema.

## Goals / Non-Goals

**Goals:**

- Let joined tenant members update a finance tenant's display name and display currency.
- Require tenant create and update display-currency values to come from a predefined valid currency-code list.
- Make the tenant management UI use select controls for display-currency create and update flows.
- Keep API JSON camelCase and regenerate apigen route bindings after route/schema edits.

**Non-Goals:**

- No role hierarchy or owner-only tenant administration rules.
- No new server-side active-tenant preference.
- No migration or backfill for existing tenant rows.
- No changes to account, transaction, provider, or FX record native currency semantics.

## Decisions

1. Add a focused tenant update API at `PATCH /api/v1/finance/tenants/{tenantId}`.
   - Rationale: the existing tenant resource path already supports `GET`, while `POST /archive` covers the special archive command. `PATCH` matches the existing transaction update style and keeps update scoped to mutable tenant profile fields.
   - Alternative considered: overload `POST /api/v1/finance/tenants/{tenantId}/update`; rejected because the existing OpenAPI surface already uses standard methods for ordinary resource reads and updates.

2. Require both `name` and `displayCurrency` in the update request.
   - Rationale: the current UI edits a small tenant profile form and can submit the current value for unchanged fields. This avoids partial-update ambiguity and keeps service validation simple.
   - Alternative considered: allow partial updates; deferred until there is a real caller that needs single-field updates.

3. Keep the supported tenant display-currency list as a small static product catalog in code, with no external currency dependency.
   - Rationale: the feature needs a bounded list, not exchange-rate metadata or locale formatting. A static list is easy to test, works offline, and avoids a dependency for a small alpha feature.
   - The initial list should include the valid ISO 4217 codes already used by current finance flows and fixtures, at minimum `USD`, `EUR`, `PLN`, and `UAH`. Additional ISO 4217 codes can be appended deliberately as product support broadens.
   - Alternative considered: add a currency-list API endpoint. Rejected for this slice because the UI only needs static select options and no runtime configurability was requested.

4. Validate display currency at the finance service boundary and mirror the same choices in UI selects.
   - Rationale: backend validation protects API clients and test fixtures; UI selects prevent normal users from typing unsupported values. Mirroring is acceptable for this small feature because no runtime currency catalog exists.
   - OpenAPI should also express the enum so generated backend validation rejects unsupported values before controller code when possible.

5. Return no tenant entity data from the update endpoint.
   - Rationale: the user-review correction clarified that mutating APIs should return only minimal required response data, not entity data. The tenant update caller already has the submitted `name` and `displayCurrency`, and the UI can refresh tenant state through the existing tenant-list/read path after a successful mutation, so the smallest response contract is success-only `204 No Content`.

## Risks / Trade-offs

- [Risk] Backend and UI static currency lists can drift. → Mitigate with tests that cover accepted/rejected backend codes and UI options for the supported list.
- [Risk] Changing display currency can alter dashboard/reporting totals for future reads. → Mitigate by documenting that account/transaction native currencies stay unchanged and reporting continues through existing persisted FX behavior.
- [Risk] Existing tenants with unsupported display-currency values could become difficult to update. → Early alpha rules do not require compatibility; if discovered locally, users can reseed or fix data directly.
- [Risk] Generated apigen files can become stale after OpenAPI edits. → Include generation and check-api/lint in implementation tasks.

## Migration Plan

- No schema migration is required.
- Existing tenant rows are left as-is.
- Local/dev deployments pick up the new API after route generation and app restart.

## Open Questions

- None blocking planning. The implementation should pick the exact initial supported list in code, starting with the codes named above.
