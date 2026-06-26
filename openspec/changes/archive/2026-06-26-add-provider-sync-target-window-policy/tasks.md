## 1. Implement target-window policy behavior

- [x] 1.1 Add focused `finance/internal/providers/` tests for the concrete target-window policy, and must follow TDD flow by first writing failing tests proving a fresh run returns the last 3 years ending at planning time, a prior checkpoint within the last 30 days still returns the last 30 days ending at planning time, and a latest succeeded checkpoint older than 30 days starts the target window from `state.Window.End` before implementing and verifying focused tests.
- [x] 1.2 Add focused `finance/internal/providers/` tests for failed-latest and invalid-state behavior, and must follow TDD flow by first writing failing tests proving a latest failed state older than 30 days resumes from `state.Window.Start`, a latest failed state already within the last 30 days still returns the standard 30-day recent window, and zero or inverted persisted windows return a bounded planning error before implementing and verifying focused tests.

## 2. Update finance sync specifications and docs

- [x] 2.1 Update the `finance-management` OpenSpec delta and adjacent finance sync documentation so the first concrete target-window policy explicitly covers the 3-year initial backfill, 30-day recent refresh, older-checkpoint catch-up, and latest failed retry-from-window semantics, and must follow TDD flow where applicable by first updating any doc-linked expectations or validation artifacts before implementing and verifying the affected checks.
