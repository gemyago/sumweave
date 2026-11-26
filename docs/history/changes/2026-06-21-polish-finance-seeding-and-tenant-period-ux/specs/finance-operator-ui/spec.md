---
type: Historical record
title: "2026-06-21-polish-finance-seeding-and-tenant-period-ux: spec"
description: "Historical spec record for 2026-06-21-polish-finance-seeding-and-tenant-period-ux."
status: deprecated
superseded_by: /requirements/finance-operator-ui.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-06-21-polish-finance-seeding-and-tenant-period-ux/specs/finance-operator-ui/spec.md"
    title: "Original openspec/changes/archive/2026-06-21-polish-finance-seeding-and-tenant-period-ux/specs/finance-operator-ui/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: Active Tenant Workspace Context
The Finance area SHALL keep one active tenant workspace context across tenant-scoped finance routes and finance-context deep links.

#### Scenario: Sole joined tenant is selected automatically
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to exactly one finance tenant
- **THEN** the UI MUST automatically use that tenant as the active finance workspace without requiring an extra selection step

#### Scenario: Multiple joined tenants are selected once and reused
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to multiple finance tenants
- **THEN** the UI MUST require one explicit active-tenant selection when no active tenant has been chosen yet
- **AND** after selection, the UI MUST reuse that active tenant across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId` until the operator changes it

#### Scenario: Finance deep links preserve the requested route
- **WHEN** an authenticated operator opens `#/finance/accounts/:accountId` or `#/finance/jobs/:jobId` directly
- **THEN** the UI MUST apply the same active-tenant auto-selection or explicit-selection rules used by other finance routes before loading tenant-specific finance context
- **AND** once the active tenant is resolved, the UI MUST continue on the originally requested deep link instead of redirecting the operator to another finance page

### Requirement: Local Finance Dates And Synchronized Current-Month Controls
The Finance area SHALL present human-readable local dates while keeping the existing reporting request semantics deterministic.

#### Scenario: Finance views render local dates instead of raw ISO strings
- **WHEN** a finance page shows operator-facing dates or timestamps such as reporting periods, invite times, missing-FX diagnostics, connection schedule times, or similar finance metadata
- **THEN** the UI MUST render those values using a standard user-local date or date-time format rather than raw ISO strings
- **AND** the underlying API and persistence semantics MUST remain unchanged

#### Scenario: Current-month mode keeps visible date controls aligned
- **WHEN** the finance dashboard is in `current_month` mode on first load or after the operator reactivates that mode
- **THEN** the visible start and end date controls MUST show the current month's active reporting bounds
- **AND** the visible picker state MUST stay synchronized when the operator switches to previous month, next month, or a custom range

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
