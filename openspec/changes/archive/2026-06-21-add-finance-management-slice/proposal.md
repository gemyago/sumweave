## Why

Issue #29 asks for one unified end-to-end Finance Management slice, but the repository currently has only finance POC command work plus a historical-data-specific jobs foundation. There is no productized `finance/` module, no tenant-aware finance API, no finance UI area, and no generic app-level durable jobs substrate that can safely run sync, FX, and import workflows.

The finance design doc is explicit that finance is adjacent to the trading runtime rather than inside it, and that generic jobs are the first cross-cutting dependency. This change turns that design into one cohesive implementation plan while preserving the existing product stack boundaries:

```text
apps/signal-foundry/ -> finance/
apps/signal-foundry/ -> runtime/
apps/signal-ui/      -> apps/signal-foundry/ HTTP API
finance/ must not import runtime/
```

## What Changes

- Create a new root `finance/` Go module for finance domain logic, persistence, connector abstractions, import logic, sync services, reporting queries, and fixture generation.
- Refactor the existing app durable jobs foundation into a generic app-level substrate with typed handler registration, worker mode, scheduler tick mode, database-backed schedules, generic JSON payload storage, and support for finance plus historical-data jobs.
- Add finance persistence, migrations, and services for tenants, memberships, invites, accounts, categories, tags, transactions, bank connections, FX rates, imports, and related audit/supporting records.
- Productize Enable Banking / PKO and monobank from the POC into finance connectors with encrypted credential storage, provider-original retention, raw payload storage, and idempotent sync behavior.
- Add finance HTTP APIs under `/api/v1/finance/...` for tenants, tenant members/invites, accounts, connections, transactions, categories, tags, dashboard/reporting, FX diagnostics/sync, imports, and finance job deep links, plus generic jobs list/detail/cancel/retry/status APIs where supported.
- Add a distinct Finance area in `apps/signal-ui` with tenant selection/create/invite/join/member flows, tenant-aware dashboard, accounts, transactions, explicit bank-linking flows, scheduled-sync controls/visibility, imports, finance job detail views, and utilitarian admin diagnostics for jobs, FX coverage, provider health, and scheduler state.
- Add minimal finance fixture scaffolding early, then complete a realistic fixture-generation CLI flow backed by `finance/` services after core services exist so local development, UI work, and end-to-end validation can use deterministic finance data.
- Add explicit docs/ops updates for architecture, local run instructions, worker/scheduler operation, fixture CLI usage, manual e2e guidance, and AGENTS updates if workflows or commands change.
- Add dedicated integration/e2e coverage and a planned fix-iterate loop so the final implementation is validated across API, jobs worker, scheduler tick, and UI workflows.

## Capabilities

### New Capabilities

- `finance-management`: Tenant-based finance domain, finance APIs, sync/import workflows, reporting semantics, and secure provider integration.
- `finance-operator-ui`: Distinct tenant-aware finance UI routes, finance workflows, and admin diagnostics.
- `finance-fixtures`: Deterministic finance fixture generation for local development and smoke/e2e flows.

### Modified Capabilities

- `durable-ingestion-jobs`: Promote the app jobs runtime from historical-backfill-only orchestration to a generic durable jobs substrate while preserving historical backfill behavior.
- `historical-data-browser`: Keep explicit historical backfill from the Data page, but deep-link created jobs into the generic admin jobs workspace.

## Impact

- Affects `apps/signal-foundry/` command wiring, config, DI, HTTP/OpenAPI routes, generic jobs runtime, worker/scheduler process modes, and finance composition.
- Adds a new root `finance/` module with substantial Go-domain, persistence, connector, import, and fixture code.
- Affects `apps/signal-ui/` routes, generated API client usage, finance pages/components/state, admin diagnostics views, and UI docs.
- Preserves `runtime/` as the trading runtime; finance stays independent except that historical backfill continues to run through app-registered generic job handlers.
- Requires architecture/runbook/manual updates for worker mode, scheduler ticks, fixture generation, and any AGENTS guidance touched by the new flow.
- Requires broad unit, integration, fake-provider, fixture-smoke, UI, and dedicated end-to-end validation coverage before implementation can be considered complete.
