## Why

PR #21 review found four correctness gaps in the newly archived cash-flow series: partial FX data can look like no activity, excessive requests can become server errors, PostgreSQL conversion can disagree with dashboard totals, and monthly validation can drift from query buckets. Correct them in a focused follow-up without altering the archived change.

## What Changes

- Show a cash-flow series partial-data warning and its missing-FX diagnostics before the all-zero activity state.
- Return a 4xx invalid-input response when explicit cash-flow parameter validation rejects a request that would exceed 366 buckets.
- Make PostgreSQL converted contributions use the same `float64` multiplication and `math.Round` tie behavior as existing dashboard reporting, with a cross-path regression.
- Count monthly bucket limits from the original range start so validation matches the SQL bucket anchor, including month-end cap behavior.
- Make no database schema additions or changes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `finance-management`: Make cash-flow series validation, month anchoring, and FX conversion behavior precise and client-safe.
- `finance-operator-ui`: Require incomplete all-zero cash-flow series to show their warning before the zero-activity state.

## Impact

- `finance/` cash-flow validation, reporting comparison coverage, and PostgreSQL series query tests.
- App finance controller error mapping and registered-route coverage.
- Finance dashboard cash-flow state ordering, dashboard coverage, and wireframe behavior documentation.
- No OpenAPI, generated code, dependency, database schema, or migration changes.
