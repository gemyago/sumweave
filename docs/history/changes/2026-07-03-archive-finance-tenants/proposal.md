---
type: Historical record
title: "2026-07-03-archive-finance-tenants: proposal"
description: "Historical proposal record for 2026-07-03-archive-finance-tenants."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-03-archive-finance-tenants/proposal.md"
    title: "Original openspec/changes/archive/2026-07-03-archive-finance-tenants/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

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

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
