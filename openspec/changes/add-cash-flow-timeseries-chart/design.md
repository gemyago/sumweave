## Context

The Finance dashboard currently derives four period-wide cash-flow metrics from the dashboard response and renders them as Bootstrap progress bars. Those bars repeat the adjacent settled and pending totals and cannot show temporal patterns. The dashboard API already has broad summary responsibilities, so the new series remains an independent tenant-scoped read.

Transactions use `time.Time` in Go and PostgreSQL `timestamptz` in `finance_transactions.effective_at`. The UI already constructs browser-local calendar boundaries and serializes them as full RFC 3339 timestamps. This change treats those submitted instants as authoritative and accepts that PostgreSQL interval stepping does not preserve the browser's calendar timezone around daylight-saving transitions.

The finance module owns reporting rules and persistence. App-owned OpenAPI/controller code exposes those services, and the Svelte Finance UI consumes the generated contract. Canonical Finance UI remains Bootstrap-based and cannot add route-local styling. No database schema additions or changes are required; the existing tenant/effective-time transaction index and current FX-rate table support the read.

## Goals / Non-Goals

**Goals:**

- Provide a separate authenticated cash-flow series API with explicit timestamp range and `day` or `month` grouping.
- Produce ordered, zero-filled settled income and expense buckets in PostgreSQL.
- Preserve existing finance visibility, transaction-kind, transfer, refund, and current-FX reporting semantics.
- Make omitted missing-FX contributions explicit to every series consumer.
- Replace the redundant dashboard progress bars with a responsive grouped bar chart.
- Establish a small reusable Svelte lifecycle boundary around tree-shaken Apache ECharts 6.
- Add convenient six-month and twelve-month calendar-aligned dashboard periods.

**Non-Goals:**

- Adding the series to the existing dashboard response.
- Persisting aggregates, historical FX valuations, or a new reporting schema.
- Adding a timezone parameter or guaranteeing browser-local bucket boundaries across daylight-saving transitions.
- Charting pending movement, net lines, categories, account balances, forecasts, or drill-down interactions.
- Returning per-bucket missing-FX counts.
- Introducing a third-party Svelte wrapper around ECharts.

## Decisions

### Separate cash-flow series contract

Expose this protected route:

```text
GET /api/v1/finance/tenants/{tenantId}/cash-flow-series
    ?startDate=<RFC3339 timestamp>
    &endDate=<RFC3339 timestamp>
    &groupBy=day|month
```

The range is half-open: `[startDate, endDate)`. Both bounds and `groupBy` are required. The service rejects an empty or reversed range, an unsupported grouping, or a request that would produce more than 366 anchored buckets. Bucket-limit validation uses the selected fixed interval rather than trusting a client estimate. The grouping enum is mapped to fixed SQL branches; request text is never interpolated into SQL.

The response is deliberately limited to chart data:

```json
{
  "period": {
    "startDate": "2026-03-01T00:00:00Z",
    "endDate": "2026-09-01T00:00:00Z"
  },
  "groupBy": "month",
  "displayCurrency": "EUR",
  "complete": false,
  "missingFx": [
    {
      "provider": "frankfurter",
      "baseCurrency": "USD",
      "quoteCurrency": "EUR",
      "affectedTransactionCount": 2
    }
  ],
  "buckets": [
    {
      "startDate": "2026-03-01T00:00:00Z",
      "endDate": "2026-04-01T00:00:00Z",
      "incomeMinor": 420000,
      "expenseMinor": 175000
    }
  ]
}
```

Bucket bounds are full timestamps, the last bucket is clipped to `endDate`, and the store returns zero-valued buckets when no qualifying transaction exists. `complete` is false whenever a qualifying booked transaction cannot be converted. `missingFx` groups those omitted transaction contributions by configured provider and currency pair, with a distinct affected-transaction count; it is empty when the series is complete.

### PostgreSQL owns bucket generation and aggregation

Add a dedicated cash-flow series store rather than extending the legacy general persistence store. The reporting service authorizes tenant membership, validates the request, and delegates one bounded aggregation query.

```text
HTTP controller
      |
      v
Reporting service -- membership + request validation
      |
      v
Cash-flow series store
      |
      +-- generate_series(startDate, endDate, fixed interval)
      +-- visible booked transactions in each bucket
      +-- reporting-kind and matched-transfer rules
      +-- tenant display currency + current FX rate
      +-- ordered income/expense sums
      `-- grouped missing-FX diagnostics
```

The query generates `1 day` or `1 month` intervals starting exactly at the supplied `startDate`; it does not use `date_trunc`. It retains only generated starts before `endDate`, so an aligned exclusive end never creates a zero-width bucket, and clips each bucket end with `LEAST(..., endDate)`. PostgreSQL compares `timestamptz` values as instants, so outer filtering remains exact. Around a daylight-saving transition an internal interval boundary can differ from browser-local midnight, which is an accepted accuracy trade-off for this chart.

Only booked transactions contribute. Hidden transactions and transactions belonging to hidden accounts are excluded. Reconciliation and opening-balance transactions are excluded. Matched internal transfers are excluded, unmatched transfers follow their amount sign, and refunds reduce expense. Conversion uses the configured current FX provider and rounds each converted transaction before summing so the series agrees with existing dashboard totals when coverage is available.

When an FX rate is unavailable, the query excludes that converted contribution and never substitutes native minor units. It also marks the series incomplete and returns grouped missing-FX diagnostics. The dashboard keeps its existing range-level FX warning and adds a chart-local partial-data warning from the series response, so the chart remains honest even if the two requests complete independently.

### Dashboard range and grouping behavior

The existing current, previous, next, and custom range actions continue to submit explicit full timestamps. Add these actions:

- **Last 6 months**: local start of the month five months before the current month through local start of the next month.
- **Last 12 months**: local start of the month eleven months before the current month through local start of the next month.

Current, previous, and next month request `groupBy=day`. The six-month and twelve-month actions request `groupBy=month`. A custom inclusive date range spanning at most 31 browser-local calendar dates requests daily grouping; a longer custom range requests monthly grouping. This calendar-date decision avoids changing grouping merely because a selected range crosses a daylight-saving boundary.

The dashboard fetches the series independently of its existing dashboard summary request. A series failure leaves the remaining dashboard usable and produces a chart-card error with a retry action. The chart has its own loading, empty, partial-data, and error state. Tenant, range, grouping, and request revision together prevent a stale series response from replacing a newer selection.

### ECharts integration and chart presentation

Add Apache ECharts 6 as a runtime dependency and import only `echarts/core`, the bar chart, grid, tooltip, legend, accessibility support, and the SVG renderer. A small shared Svelte component owns initialization, resize observation, option updates, and disposal. It accepts chart options and an accessible name but contains no finance-specific series construction.

The Finance dashboard builds the grouped chart options:

- income and expense render as adjacent exact-value bars; ordinary expenses are positive, while a refund-dominant bucket may place net expense below zero;
- income uses the success tone and expense uses the danger tone;
- the horizontal axis uses localized bucket labels and reduces visible tick labels when space is constrained;
- tooltips show the complete localized bucket range and exact formatted currency values;
- the renderer uses a transparent background and theme-compatible axis, grid, and tooltip colors;
- a non-canvas text alternative identifies every bucket range and its exact income and expense values.

The chart becomes a full-width primary card below the period-net summary so a daily month remains legible. Any chart-height or containment exception is added to the shared stylesheet with the required widget-sizing comment; the Finance route does not add local styles.

## Risks / Trade-offs

- [DST-edge transactions can fall into an adjacent displayed bucket] -> Anchor every interval to the submitted start instant, return exact bucket bounds, and document that grouping is instant-based rather than timezone-calendar-based.
- [SQL reporting rules can drift from Go dashboard rules] -> Reuse shared domain constants where possible, cover every supported transaction kind in store integration tests, and assert that bucket sums agree with dashboard settled totals under complete FX coverage.
- [Missing FX can make bars partial] -> Never substitute native units; return completeness plus grouped diagnostics and show a chart-local partial-data warning.
- [ECharts can add unnecessary bundle weight] -> Use the documented tree-shakeable core imports and one renderer instead of importing the complete package.
- [Separate requests can finish out of order] -> Track tenant, range, grouping, and request revision before committing series state.
- [Thirty-one grouped pairs become dense on narrow screens] -> Give the chart a full-width card, use responsive label reduction, and verify desktop and narrow layouts through the required visual-review and manual smoke loops.

## Migration Plan

1. Add the dedicated store, reporting contract, protected OpenAPI route, controller mapping, generated Go route code, and hand-written Finance UI client mapping.
2. Add ECharts and the shared Svelte lifecycle component.
3. Replace the existing visual and add the longer period actions while retaining the current dashboard summary response.
4. Update tests and Finance UI documentation, then complete the repository lint, test, headed smoke, and visual-review flows.

Rollback removes the new endpoint, chart component, ECharts dependency, and longer period actions, then restores the existing progress-bar visual. There is no persisted-data migration or rollback step.

## Open Questions

None. The accepted first-version boundary uses PostgreSQL interval grouping without a timezone parameter and returns response-wide completeness plus grouped missing-FX diagnostics.
