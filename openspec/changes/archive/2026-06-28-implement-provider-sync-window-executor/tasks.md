## 1. OpenSpec And Terminology

- [x] 1.1 Update the finance provider sync architecture doc and finance-management spec wording so the existing `candidate window` concept is clarified as the persisted snapshot lookup window, and must follow TDD flow where applicable by first updating any doc-linked expectations or terminology checks before applying the wording changes.

## 2. Executor Contracts

- [x] 2.1 Define `SnapshotWindowPolicy` and `SyncRepository` executor seams in `finance/internal/providers`, and must follow TDD flow by first expanding focused contract tests where needed before introducing the new interfaces and their expected responsibilities.
- [x] 2.2 Add the first concrete snapshot policy implementation as a requested-window passthrough policy, and must follow TDD flow by first adding focused policy tests for valid passthrough behavior before implementing that minimal default.

## 3. Window Executor Flow

- [x] 3.1 Implement requested-window execution in a follow-up step as fetch -> snapshot load -> diff -> apply-plan -> apply handoff -> result reporting, and must follow TDD flow by first adding failing executor tests that prove diff/apply planning outputs are passed into storage and surfaced through the result before implementing the executor flow.
