## Why

The dashboard's current cash-flow visual repeats period totals as progress bars and does not show when income and expenses occurred. A grouped time-series chart would reveal daily and monthly cash-flow patterns and establish a reusable charting foundation for future finance visualizations.

## What Changes

- Add a protected, tenant-scoped cash-flow series API that accepts a full timestamp range and an explicit `day` or `month` grouping window.
- Aggregate settled income and expense buckets in PostgreSQL using the existing reporting, visibility, transfer, refund, and current-FX rules, while reporting omitted missing-FX contributions explicitly.
- Anchor generated buckets to the caller-supplied start timestamp; the API does not accept a separate timezone parameter.
- Replace the dashboard's aggregate cash-flow progress bars with a responsive grouped income-versus-expense bar chart backed by the separate series API.
- Add aligned six-month and twelve-month dashboard period actions alongside the existing month and custom-range controls.
- Add Apache ECharts through its tree-shakeable core API and a small reusable Svelte chart lifecycle component for this and future visualizations.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `finance-management`: Extend tenant reporting with a separate timestamp-bounded, PostgreSQL-aggregated cash-flow series API.
- `finance-operator-ui`: Replace the dashboard cash-flow summary visual with an adaptive ECharts time-series chart and add longer aligned period actions.

## Impact

- Finance domain reporting contracts and a new dedicated PostgreSQL cash-flow series store under `finance/`.
- The app-owned OpenAPI contract, generated Go route models, finance controller mapping, and hand-written typed Finance UI API client.
- The Finance dashboard, reporting-period controls, shared chart component surface, tests, wireframe, and user-visible empty/error states.
- One new frontend runtime dependency: Apache ECharts 6, imported selectively without a Svelte wrapper.
