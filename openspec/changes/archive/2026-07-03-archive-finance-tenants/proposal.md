## Why

Finance tenants can be created and listed today, but there is no non-destructive way to retire a tenant that should no longer be used. That leaves stale tenants in the active workspace list and forces backend consumers to treat archival as an unsupported manual data problem.

## What Changes

- Add a backend/API-only finance tenant archive flow so an authenticated tenant member can archive a tenant without deleting its historical data.
- Add archived-tenant lifecycle rules so archived tenants leave the active tenant list.
- Extend the finance tenant API surface at a high level to cover archive behavior without returning tenant data from the archive mutation.
- Add manual e2e documentation for tenant management create, list, archive, and post-archive verification flows.
- Explicitly keep this change out of UI scope.

## Capabilities

### New Capabilities

### Modified Capabilities
- `finance-management`: Add tenant archival lifecycle and protected tenant archive API behavior.

## Impact

- Affects `finance/`, especially tenant domain models, tenant service/access rules, and tenant persistence/query behavior.
- Affects `apps/signal-foundry/`, especially `internal/api/http/v1routes.yaml`, generated finance route/models, finance controllers, and finance API tests.
- Affects `docs/manual-e2e/`, especially the manual API verification guide and index for tenant create/list/archive coverage.
- Updates OpenSpec finance requirements to make API-only tenant archival explicit and to keep UI work out of scope.
