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
