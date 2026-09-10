## 1. Finance Series Semantics

- [x] 1.1 Align PostgreSQL converted cash-flow contributions with existing Go `float64` multiplication and `math.Round` half-away-from-zero behavior, and follow the TDD flow with a PostgreSQL regression that compares the series bucket sum and dashboard settled totals for the reviewed half-unit and floating-boundary FX cases.
- [x] 1.2 Anchor monthly cash-flow bucket-limit validation to the original start timestamp rather than a prior clipped month boundary, and follow the TDD flow with month-end tests covering original-anchor boundaries plus accepted and rejected 366-bucket caps.

## 2. Cash-Flow API Validation

- [x] 2.1 Map every controller-side cash-flow parameter-validation failure to a 4xx invalid-input response before service delegation, and follow the TDD flow with a registered-route test for a valid grouping/range that exceeds 366 buckets and proves no service call occurs.

## 3. Incomplete-Series Dashboard State

- [x] 3.1 Render an incomplete cash-flow warning and its missing-FX diagnostics before the all-zero activity state while retaining the loaded-series textual alternative, update the Finance wireframe's chart-state wording, and follow the UI TDD and visual-verification flows with an all-zero incomplete response test, the required Finance shell smoke, and independent desktop/narrow visual review.
