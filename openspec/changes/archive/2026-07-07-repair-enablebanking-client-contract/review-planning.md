# Planning Review

## Round 1

- Scope: `repair-enablebanking-client-contract`
- Trigger: Initial plan review
- Findings:
  - Plan is internally consistent for typed client repair, app HTTP client wiring, connector mapping, and evidence separation.
  - `tasks.md` lacks a distinct manual e2e testing section.
  - Missing explicit manual e2e tasks for bank linking, sync, checking sync success, and adding mock Enable Banking transactions then confirming they appear after re-sync.
- Verdict: issues
- Completion protocol status: not applicable
- Artifact cleanup status: clean
- Commit status: none; review blocked on plan update

## Round 1 - 2026-07-07

### Verdict

issues

### Findings

- `proposal.md`, `design.md`, and `tasks.md` are aligned on the typed Enable Banking client repair, app HTTP client wiring, connector mapping, and separate provider evidence boundary.
- The plan does not include a distinct manual e2e testing section in `tasks.md`.
- `tasks.md` does not explicitly require manual e2e validation for bank linking.
- `tasks.md` does not explicitly require manual e2e validation for sync execution.
- `tasks.md` does not explicitly require manually checking whether sync succeeded.
- `tasks.md` does not explicitly require adding transactions in the mock Enable Banking integration and confirming they appear after re-sync.
- The missing manual validation tasks leave the plan not ready for implementation despite otherwise coherent technical scope.

### Recommended Plan Changes

- Add a new distinct `tasks.md` section after Connector Alignment, for example `## 4. Manual E2E Validation`.
- Add task `4.1 Manually validate the local mock Enable Banking bank-linking flow end-to-end, including starting the backend with required migrations, initiating bank linking, completing the redirect/callback, and confirming the linked account is persisted/visible through the existing finance API/UI path used for manual checks.`
- Add task `4.2 Manually run a finance sync for the linked mock Enable Banking account and record the exact command/API/UI path used to trigger sync.`
- Add task `4.3 Manually verify whether sync succeeded by checking the existing sync status/result surface and confirming expected accounts, balances, and transactions were imported without provider errors.`
- Add task `4.4 Add new transactions in the mock Enable Banking integration, re-run sync, and confirm the newly added transactions appear after re-sync without losing existing imported data.`
- Add task `4.5 Document the manual e2e evidence in the implementation or final review artifact, including environment used, sync status observed, and before/after transaction identifiers/counts.`
- Optionally add a small sentence to `proposal.md` Impact or `design.md` Goals that this repair must be validated with a manual mock Enable Banking link-and-resync flow. This is not strictly required if the tasks section is added, but it would make the validation expectation explicit outside `tasks.md`.

### Chunking Impact

- Add the manual e2e section as the final sequential parent task; implementation can remain chunks 1, 2, 3, then 4.
- Do not merge the manual e2e parent task into the code-change parent tasks; keep it as the acceptance/validation chunk after code work.

### Artifact Cleanup Status

- Change directory contains only expected OpenSpec artifacts plus standard manager/review artifacts; no ad-hoc cleanup needed.

### Commit Status

- No commit created because the plan has blocking review issues.

## Round 2 - 2026-07-07

### Verdict

clean

### Findings

- `proposal.md`, `design.md`, and `tasks.md` remain consistent for the typed Enable Banking client repair, app HTTP client wiring, connector mapping, and separate provider evidence boundary.
- The new `## 4. Manual E2E Validation` section is properly ordered after implementation-facing client, wiring, and connector work.
- Task 4.1 clearly covers manual local/mock Enable Banking bank linking, including migrations/backend startup, link initiation, redirect/callback completion, and linked-account visibility.
- Task 4.2 clearly requires triggering finance sync and recording the exact trigger path.
- Task 4.3 clearly requires sync success verification through the existing status/result surface and checks for accounts, balances, transactions, and provider errors.
- Task 4.4 clearly covers adding new mock Enable Banking transactions, re-triggering sync for the same linked account, and verifying new transactions appear while existing imports remain.
- Task 4.5 captures the needed manual e2e evidence in standard plan/review artifacts.

### Recommended Plan Changes

- None required.
- Optional only: implementation reviewers can reference `docs/manual-e2e/enable-banking-mock-aspsp-ui-e2e.md` when executing task 4, but the plan is sufficiently explicit without another plan edit.

### Chunking Impact

- Keep sequential chunks in parent-task order: 1, 2, 3, then 4.
- No need to split or merge parent tasks; manual e2e remains the final validation chunk.

### Artifact Cleanup Status

- Change directory contains only expected OpenSpec artifacts plus standard manager/review artifacts; no ad-hoc cleanup needed.

### Commit Status

- Pending commit gate because the plan is clean and planning/review artifacts have changed.
