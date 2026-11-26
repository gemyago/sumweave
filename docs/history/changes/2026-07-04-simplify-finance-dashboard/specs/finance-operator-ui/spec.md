---
type: Historical record
title: "2026-07-04-simplify-finance-dashboard: spec"
description: "Historical spec record for 2026-07-04-simplify-finance-dashboard."
status: deprecated
superseded_by: /requirements/finance-operator-ui.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-04-simplify-finance-dashboard/specs/finance-operator-ui/spec.md"
    title: "Original openspec/changes/archive/2026-07-04-simplify-finance-dashboard/specs/finance-operator-ui/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## MODIFIED Requirements

### Requirement: Finance Dashboard And Workspace Flows
The Finance area SHALL expose the first end-user workflows required by the finance design.

#### Scenario: Dashboard shows simple period-aware finance summaries
- **WHEN** an authenticated tenant member opens `#/finance`
- **THEN** the UI MUST prioritize a simple dashboard hierarchy with compact header, active period context, primary balance summary, income summary, expense summary, pending delta, cash-flow visual, spending or category visual, account snapshot, recent transactions, and a compact needs-attention area
- **AND** the first viewport MUST avoid large implementation-facing copy, full-width tenant controls, full-width custom reporting forms, and admin diagnostics as primary content
- **AND** the dashboard MUST derive its first implementation from existing finance dashboard, account, transaction, category, and connection data sources instead of requiring a new API contract
- **AND** account, category, and recent-transaction dashboard sections MUST cap visible rows and link to dedicated browse/detail routes for full lists
- **AND** missing-FX, pending transaction, failed sync, or import follow-up signals MUST appear as secondary attention states rather than primary dashboard cards when present
- **AND** the dashboard MUST use honest empty or reduced states when selected-tenant data is unavailable
- **AND** the dashboard MUST preserve the existing Signal UI terminal-native design tokens and styling foundations unless a separate design-system change is explicitly accepted

### Requirement: Active Tenant Workspace Context
The Finance area SHALL keep one active tenant workspace context across tenant-scoped finance routes and finance-context deep links.

#### Scenario: Sole joined tenant is selected automatically without visible switcher
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to exactly one finance tenant
- **THEN** the UI MUST automatically use that tenant as the active finance workspace without requiring an extra selection step
- **AND** normal tenant-scoped finance routes such as `#/finance` MUST NOT render visible tenant selection controls or duplicate tenant workspace panels solely to show the already-resolved tenant
- **AND** the tenant management route MUST remain available for tenant administration

#### Scenario: Multiple joined tenants use one compact shared switcher
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to multiple finance tenants
- **THEN** the UI MUST require one explicit active-tenant selection when no active tenant has been chosen yet
- **AND** after selection, the UI MUST reuse that active tenant across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId` until the operator changes it
- **AND** the shared finance shell MUST expose at most one compact workspace switcher for changing tenants on normal finance routes
- **AND** dashboard panels MUST NOT reintroduce unrelated duplicate tenant picker or tenant workspace chrome

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
