---
type: Historical record
title: "2026-07-03-split-finance-root-service: proposal"
description: "Historical proposal record for 2026-07-03-split-finance-root-service."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-03-split-finance-root-service/proposal.md"
    title: "Original openspec/changes/archive/2026-07-03-split-finance-root-service/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

The finance root `Service` is still the public home for tenant, catalog, ledger, reporting, FX, CSV import, and bank sync workflows, even though the module rules require each public service to be single-responsibility. Splitting this boundary now prevents new finance work from continuing to accumulate on a monolithic facade.

## What Changes

- **BREAKING** Replace the broad public `finance.Service` API with dedicated focused public services exposed by the `finance.New` / `finance.Finance` public contract.
- Promote tenant, catalog, ledger, reporting, FX, CSV import, and bank sync workflows to their own public service types.
- Keep bank-link workflows on the already-focused public bank connection service instead of moving them back through the root service.
- Add finance module contract tests that prove `finance.New` exposes the focused services through `finance.Finance` and that active callers stop depending on root `finance.Service`.
- Move app DI, HTTP controller dependencies, including FX diagnostics and FX sync controller routes, job handlers, fixture bootstrap paths, and CLI finance bootstrap code to the focused services they actually need.
- Shrink or remove `finance/service.go` so it no longer acts as a public god-service facade.
- Split the broad `serviceStore` dependency into service-owned store interfaces and dedicated stores where persistence support is missing.
- Keep the existing HTTP routes, JSON shapes, job types, database tables, and domain behavior unchanged unless a test proves an accidental mismatch.
- Do not preserve backward-compatible wrappers for early-alpha root-service callers.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Require finance public service boundaries to be focused by product responsibility and prohibit new app/controller/job usage of the root finance service.

## Impact

- Affected code: `finance/`, `finance/persistence`, `apps/signal-foundry/internal/financeapp`, finance HTTP controllers, finance job registration, finance CLI bootstrap, and finance fixtures.
- Affected API: no external HTTP route, OpenAPI, JSON, job type, or database schema change expected.
- Affected tests: finance service tests, `finance.New` / `finance.Finance` contract tests, finance provider sync/import/reporting tests, app registration tests, controller tests including FX routes, and fixture tests will move to focused service boundaries.
- Out of scope: UI changes, provider sync v2 algorithm changes, new finance features, and reviving package distribution code.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
