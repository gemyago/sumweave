## Context

The finance slice already supports tenant creation, tenant listing, tenant selection, tenant invites, and tenant-scoped account/transaction workflows. The current tenant model and API only represent an active tenant lifecycle: `domain.Tenant` has no archive state, `ListTenantsForUser` returns joined tenants as active workspace options, and the app OpenAPI surface exposes create/list/get tenant routes but no archive command.

The requested feature is backend-only. There is no UI work requested, but the repository does keep manual API e2e notes under `docs/manual-e2e/`, so the change should also add human-run verification steps for tenant create/list/archive behavior.

## Goals / Non-Goals

**Goals:**

- Add a protected finance API flow that archives a tenant without hard-deleting its data.
- Keep archived tenants out of the normal active tenant list and active tenant selection flow.
- Keep the archive mutation response empty unless the backend must generate response data a client needs.
- Keep the initial permission model simple: any current tenant member can archive because finance tenant members are otherwise equal today.
- Add manual API e2e documentation for tenant management create/list/archive verification.
- Keep the feature explicitly API-only with no UI work.

**Non-Goals:**

- No UI route, button, dialog, or tenant workspace changes.
- No tenant restore/unarchive flow in this change.
- No hard-delete or cascading purge of tenant-owned records.
- No new role model, ownership tier, or archive approval workflow.
- No admin-only archived-tenant browsing surface unless implementation reveals a hard requirement that is not visible today.
- No extra archived-tenant access blocking beyond removing archived tenants from the active tenant list for this slice.

## Decisions

### 1. Use soft archive state on the tenant record

Tenant archival should be modeled as persisted soft state on the tenant itself, most likely a nullable archive timestamp that can be checked by queries and access guards.

Rationale:

- archive is a lifecycle change, not a data deletion request
- tenant-owned accounts, transactions, categories, tags, connections, invites, and related history should remain intact
- the finance module already uses nullable lifecycle timestamps for hidden records, so soft state matches existing patterns

Alternative considered:

- hard delete was rejected because it increases data-loss risk and would require much broader cascading behavior than requested

### 2. Add an explicit archive command endpoint instead of a generic tenant update route

The likely API shape is a dedicated command endpoint such as `POST /api/v1/finance/tenants/{tenantId}/archive` that returns no response body.

Rationale:

- the current finance tenant surface has create/list/get routes but no generic tenant update route
- the finance API already uses explicit command-style endpoints for non-CRUD actions such as redirect start/finish, sync, and import confirmation
- mutating APIs should stay no-response by default unless the backend generates data the client needs immediately
- a dedicated archive command keeps the first implementation narrow and avoids introducing broader tenant patch semantics that are not otherwise needed

Likely affected API surfaces:

- `apps/signal-foundry/internal/api/http/v1routes.yaml`
- generated tenant route params/models under `internal/api/http/v1routes/`
- finance controller methods and controller route tests
- finance controller archive handler shape and generated route contract

### 3. Treat archived tenants as inactive for default tenant discovery

After archive succeeds, archived tenants should stop appearing in the normal joined-tenant list and should no longer be returned as active tenant selection results.

Rationale:

- the main product reason for archive is to retire a tenant from normal use without losing history
- keeping archived tenants visible in the normal active list would not solve the stale-workspace problem

Implementation shape at a high level:

- tenant list queries filter to active tenants by default
- `GET /api/v1/finance/tenants/{tenantId}` stops resolving archived tenants as active selections

Alternative considered:

- leaving archived tenants visible but marked in list results was rejected for the first slice because there is no UI work requested and the simplest backend behavior is to remove archived tenants from the active workspace path entirely

### 4. Keep archive authorization aligned with the current equal-member tenant model

The first implementation should allow any current tenant member to archive the tenant.

Rationale:

- existing finance requirements say all tenant members are equal in the first implementation
- introducing owner/admin roles would expand scope beyond this request

### 5. Add manual API e2e notes alongside the backend change

Manual verification should be added under `docs/manual-e2e/` with API-only steps that cover:

- create tenant
- list tenants and capture the created tenant id
- archive the tenant through the new API
- list tenants again and verify the archived tenant is absent from the active list

Rationale:

- the user explicitly requested manual e2e coverage for the create/list/archive flow
- existing manual e2e guidance in this repo is already API-oriented for backend workflows
- documenting curlable API steps matches the requested no-UI scope

## Risks / Trade-offs

- Archived-tenant filtering changes list/get behavior for existing callers → keep the change explicit in spec and controller tests, and document the post-archive list expectation in the manual e2e guide.
- Future archived-tenant access restrictions may still be needed later → defer that scope until the product needs it instead of adding extra guard checks now.

## Migration Plan

1. Extend finance tenant persistence/domain shape with soft-archive state and auto-migrate the finance tenant table.
2. Add finance tenant archive service/store behavior plus active-tenant filtering for list/get flows.
3. Add the protected archive API route, generated models, no-response controller mapping, and focused backend tests.
4. Add manual API e2e documentation for tenant management create/list/archive and update the manual e2e index.

## Open Questions

- No blocker is currently identified for planning.
- Assumption: archive is one-way for this change; restore/unarchive can be proposed later if product usage proves it necessary.
- Assumption: archived tenants are removed from the default active tenant list rather than exposed through a separate archived listing API.
