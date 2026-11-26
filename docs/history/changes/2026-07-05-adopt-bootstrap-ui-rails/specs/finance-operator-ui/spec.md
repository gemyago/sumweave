---
type: Historical record
title: "2026-07-05-adopt-bootstrap-ui-rails: spec"
description: "Historical spec record for 2026-07-05-adopt-bootstrap-ui-rails."
status: deprecated
superseded_by: /requirements/finance-operator-ui.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-05-adopt-bootstrap-ui-rails/specs/finance-operator-ui/spec.md"
    title: "Original openspec/changes/archive/2026-07-05-adopt-bootstrap-ui-rails/specs/finance-operator-ui/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## ADDED Requirements

### Requirement: V2 Bootstrap Finance Dashboard Pilot
The Finance area SHALL expose a parallel Bootstrap-based V2 dashboard pilot without replacing the canonical finance dashboard route in this change.

#### Scenario: V2 dashboard shows simple period-aware finance summaries
- **WHEN** an authenticated tenant member opens `#/v2/finance`
- **THEN** the UI MUST prioritize a simple Bootstrap-based dashboard hierarchy with compact header, active period context, primary balance summary, income summary, expense summary, pending delta, cash-flow visual, spending or category visual, account snapshot, recent transactions, and a compact needs-attention area
- **AND** the route MUST render inside a Bootstrap-specific protected finance shell rather than `FinanceShell.svelte`, with shell-level tenant selection when required, sign-out and theme controls, and compact finance-local navigation
- **AND** that finance-local navigation MAY link back to canonical `#/finance/*` destinations for non-pilot routes while this change keeps the pilot parallel
- **AND** the first viewport MUST avoid large implementation-facing copy, full-width tenant controls, full-width custom reporting forms, and admin diagnostics as primary content
- **AND** the dashboard MUST derive its first implementation from existing finance dashboard, account, transaction, category, and connection data sources instead of requiring a new API contract
- **AND** the route MUST reuse existing finance tenant-workspace behavior for tenant loading, remembered selection, and route continuity instead of introducing a second tenant-state contract
- **AND** account, category, and recent-transaction dashboard sections MUST cap visible rows and link to dedicated browse/detail routes for full lists
- **AND** missing-FX, pending transaction, failed sync, or import follow-up signals MUST appear as secondary attention states rather than primary dashboard cards when present
- **AND** the dashboard MUST use honest empty or reduced states when selected-tenant data is unavailable
- **AND** the dashboard MUST use vanilla Bootstrap classes and native HTML or Svelte markup as the primary styling mechanism for this pilot instead of the existing Signal UI terminal-native custom design-system classes
- **AND** custom CSS for this dashboard pilot MUST be limited to shared documented exceptions for shell containment, chart sizing, Bootstrap bridge variables, accessibility fixes, or browser fixes
- **AND** canonical `#/finance` MUST remain available and behaviorally unchanged until a later explicit promotion decision

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
