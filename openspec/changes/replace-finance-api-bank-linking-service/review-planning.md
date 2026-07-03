# Planning Review

## Round 1

- Scope: replace-finance-api-bank-linking-service planning artifacts
- Triggering input: user requested an initial review of `proposal.md`, `design.md`, `tasks.md`, and `.openspec.yaml`
- Verdict: needs changes
- Strengths:
  - Proposal and design stay narrowly focused on moving API bank-link flows onto a dedicated public finance boundary without changing the HTTP contract.
  - Design aligns with repo/module rules by keeping app code out of `finance/internal/...` and by avoiding new methods on the legacy broad persistence store.
  - `.openspec.yaml` matches the repository's normal minimal change-metadata shape.
- Findings:
  - The plan does not explicitly carry the proposal/spec commitment that unsupported provider or linking-method combinations must fail before any secret write or connector call. Add that behavior to the service-level tasks and failing-test expectations instead of leaving it implicit.
  - Proposal scope says any remaining callers of the old root-service bank-link path must be migrated or deleted in this slice, but task 3.1 only guarantees removal from the active API path. Either narrow the proposal wording or add an explicit caller-audit/migrate-delete task so scope and implementation obligations match.
  - Task 3.2 introduces documentation/OpenSpec wording work that is not clearly committed in the proposal or design and is too open-ended as written. Either name the exact docs/artifacts that must change in proposal/design or remove/rewrite this task to a bounded follow-through item.
- Chunking recommendation:
  1. `focused-bank-connection-service` (section 1)
  2. `api-and-app-wiring-cutover` (section 2)
  3. `legacy-link-path-removal` (section 3)
- Artifact cleanup: clean; the change directory contains standard OpenSpec artifacts plus the repo's usual change `README.md`.
- Commit status: no commit created; the plan is not yet ready, and the working tree currently contains pending standard planning artifacts (`manager-status.md`, `review-planning.md`).

## Round 2

- Scope: replace-finance-api-bank-linking-service revised planning artifacts
- Triggering input: user requested a re-review after proposal, design, and tasks updates
- Verdict: ready
- Findings: none blocking; the prior gaps are now resolved.
- Checks:
  - Proposal now explicitly commits to pre-write/pre-connector rejection for unsupported provider or linking-method combinations.
  - Proposal, design, and tasks now consistently scope legacy cleanup to the protected API handlers and callback bridge, with only directly unreferenced helpers removed in-slice.
  - The open-ended documentation/OpenSpec cleanup task was removed, keeping the slice bounded to implementation work already described by the proposal and design.
- Chunking recommendation:
  1. `focused-bank-connection-service` (section 1)
  2. `api-and-app-wiring-cutover` (section 2)
  3. `legacy-link-path-removal` (section 3)
- Artifact cleanup: clean; only standard OpenSpec planning artifacts are present.
- Commit status: pending planning artifacts should be committed because the plan is now ready for implementation.
