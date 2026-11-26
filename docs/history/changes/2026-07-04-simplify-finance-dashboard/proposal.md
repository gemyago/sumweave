---
type: Historical record
title: "2026-07-04-simplify-finance-dashboard: proposal"
description: "Historical proposal record for 2026-07-04-simplify-finance-dashboard."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-04-simplify-finance-dashboard/proposal.md"
    title: "Original openspec/changes/archive/2026-07-04-simplify-finance-dashboard/proposal.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## Why

- The current finance dashboard is overloaded with implementation-facing controls, duplicated tenant context, diagnostic copy, and secondary operational details before the operator can understand their money.
- The target dashboard hierarchy should make balance visible first, then compact income/expense summaries, visual cash-flow and spending summaries, and recent activity before secondary operational detail.
- Most users are expected to belong to a single tenant, so showing tenant selection everywhere creates friction and makes the product feel more complex than it is.
- We have enough finance sandbox data to redesign the dashboard using existing finance endpoints instead of expanding backend scope.

## What Changes

- Recompose `#/finance` into a simple finance dashboard focused on balance, cash flow, spending categories, account snapshot, recent transactions, and a compact attention strip.
- Remove large dashboard-level tenant and reporting control blocks from the primary first viewport.
- Hide tenant selection for single-tenant users and keep only one compact workspace switcher for multi-tenant users.
- Keep the shell-owned active tenant consistent across tenant-scoped finance routes and direct finance deep links instead of reintroducing route-level tenant pickers.
- Demote admin/diagnostic content such as missing FX, pending sync, and failed import signals into secondary attention states or admin routes.
- Update the finance UI OpenSpec delta, implementation task plan, and future wireframe expectations for the redesigned dashboard.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-operator-ui`: simplify the dashboard hierarchy and active-tenant presentation for finance routes.

## Impact

- Affected implementation areas: `apps/signal-ui/src/components/FinanceShell.svelte`, `apps/signal-ui/src/pages/Finance.svelte`, finance UI tests, `apps/signal-ui/ui-wireframe.md`, and finance manual smoke guidance.
- No new Go, OpenAPI, or persistence scope is proposed.
- The old dense dashboard layout can be replaced outright because the project is early alpha.

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
