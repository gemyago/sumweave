## ADDED Requirements

### Requirement: Dashboard Cash-Flow Time-Series Visualization
The Finance dashboard SHALL present selected-period settled income and expense as a responsive grouped time-series chart backed by the separate cash-flow series API.

#### Scenario: Dashboard replaces the aggregate progress-bar visual
- **WHEN** a tenant dashboard and its cash-flow series load successfully
- **THEN** the dashboard MUST replace the existing settled and pending progress-bar visual with adjacent income and expense bars for every returned bucket
- **AND** the chart MUST use exact formatted values in its tooltip and visually distinct success and danger tones
- **AND** pending values MUST remain available through the existing summary rather than becoming a chart series

#### Scenario: Dashboard chooses grouping from the selected period
- **WHEN** the operator selects current, previous, or next month
- **THEN** the UI MUST request daily buckets using the same full timestamp bounds as the selected dashboard period
- **AND** when the operator selects six months, twelve months, or a custom inclusive range spanning more than 31 browser-local calendar dates, the UI MUST request monthly buckets
- **AND** a custom inclusive range spanning at most 31 browser-local calendar dates MUST request daily buckets

#### Scenario: Longer period actions use aligned local bounds
- **WHEN** the operator selects **Last 6 months** or **Last 12 months**
- **THEN** the UI MUST calculate a browser-local start at the first day of the earliest included month and an exclusive end at the first day of the next month
- **AND** those actions MUST include the current month and its preceding five or eleven calendar months respectively
- **AND** the selected range MUST remain visible and persist through the existing dashboard URL range behavior

#### Scenario: Series request lifecycle is independent and current
- **WHEN** the cash-flow series is loading, empty, fails, or a newer tenant or range selection supersedes it
- **THEN** the chart card MUST show its own honest loading, zero-activity, partial-data warning, or recoverable error with retry without making the remaining dashboard unavailable
- **AND** a response for an obsolete tenant, range, or grouping MUST NOT replace the current series

#### Scenario: ECharts integration is reusable and bounded
- **WHEN** the UI renders the cash-flow chart
- **THEN** it MUST use Apache ECharts 6 through selective core imports and the SVG renderer without a third-party Svelte wrapper
- **AND** a shared Svelte chart component MUST own initialization, responsive resize, option updates, and disposal while finance-specific series construction remains dashboard-owned

#### Scenario: Cash-flow chart remains usable and accessible
- **WHEN** the chart renders on desktop or a narrow viewport
- **THEN** the period-performance card and cash-flow chart card MUST each span the full available dashboard content width at every breakpoint
- **AND** the chart MUST use responsive tick-label reduction, theme-compatible presentation, and an accessible chart name and textual alternative containing every bucket's range and exact values
- **AND** hovering any income or expense bar MUST keep both bar series visibly rendered in their normal opaque semantic tones while the exact-value tooltip is displayed
- **AND** canonical Finance markup MUST remain Bootstrap-first without route-local style blocks or inline layout styles
- **AND** the changed desktop and narrow-screen flows MUST pass the repository's required independent visual-review and manual smoke loops after concrete findings are resolved
