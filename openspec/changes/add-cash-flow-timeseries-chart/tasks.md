## 1. Finance Cash-Flow Series

- [x] 1.1 Define the cash-flow grouping, request, response, bucket, completeness, and grouped missing-FX domain contracts plus bounded validation, and follow the TDD flow by adding focused randomized service tests before implementing request validation and tenant authorization.
- [x] 1.2 Implement the dedicated PostgreSQL cash-flow series store with anchored zero-filled non-zero-width buckets, settled reporting rules, per-transaction current-FX conversion, grouped missing-FX diagnostics, and no schema changes; follow the TDD flow with PostgreSQL integration cases for day/month grouping, clipped ends, exclusions, refunds, rounding, missing FX, ordering, and empty buckets.
- [x] 1.3 Inject the dedicated store as a required reporting dependency and wire it through finance composition without extending the legacy general store; follow the TDD flow by extending composition and service tests before production wiring.

## 2. Protected API Contract

- [x] 2.1 Add the protected `/api/v1/finance/tenants/{tenantId}/cash-flow-series` OpenAPI operation and exact response schemas, regenerate Go route code and controller mocks, and follow the TDD flow with registered-route controller tests for authentication, parameter validation, domain mapping, and errors.
- [x] 2.2 Extend the hand-written typed Finance UI API with cash-flow request serialization and strict response mapping, and follow the TDD flow with client contract tests for required fields, timestamps, grouping, completeness, missing-FX diagnostics, and malformed responses.

## 3. ECharts Dashboard Visualization

- [x] 3.1 Verify and record Apache ECharts 6 maintenance and stack compatibility, add it as a runtime dependency using selective core imports, and build a reusable SVG chart lifecycle component; follow the TDD flow with component tests for initialization, option updates, resize behavior, accessible naming, and cleanup.
- [x] 3.2 Add six-month and twelve-month aligned ranges plus automatic day/month grouping and independent stale-safe series loading, and follow the TDD flow with range and Finance dashboard tests for local calendar calculations, URL persistence, grouping requests, retry, and isolated loading/failure behavior.
- [x] 3.3 Replace only the cash-flow progress bars with the full-width grouped income-versus-expense chart, exact signed values and tooltips, responsive labels, loading/empty/partial/error states, and a complete textual alternative; update the Finance wireframe and shared widget sizing as needed, follow the TDD flow for observable UI behavior, then run the required Finance smoke and independent desktop/narrow visual-review loops and resolve concrete findings.

## 4. User-Review UI Corrections

- [x] 4.1 Make the period-performance card span the full available dashboard content width at every breakpoint and keep both income and expense bar series visibly opaque in their normal semantic tones while any bar is hovered and its tooltip is active; follow the TDD flow with focused dashboard option/layout coverage, then run the required Finance smoke and independent desktop/narrow visual verification for both corrections as one coupled UI chunk.
