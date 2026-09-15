## Context

The original `build-integration-layer` change and its
`build-integration-layer-pr-corrections` follow-up are archived. The latter's
`## MODIFIED Requirements` delta restated only its new redirect and wait-deadline
scenarios. OpenSpec archive semantics replace the matching requirement with that
complete restatement, so the canonical `direct-finance-cli` spec lost five
unchanged scenarios.

## Decision

Create the smallest linked active correction change. Its direct-client delta
will restate each affected requirement in full: the unchanged Phase 1 scenarios
first, followed by the already-approved additive correction scenario. This
restores the canonical contract without changing its implementation or product
behavior.

## Non-Goals

- Do not modify either archived change or revise their historical artifacts.
- Do not change `swmd`, tests, API contracts, or release behavior.
- Do not reopen previously resolved threads or implement the two optional PR
  observations.

## Verification

Strictly validate this active change after its artifacts are complete. Before
archive, compare its two complete modified requirements with the archived
Phase 1 delta and the first correction delta to confirm all seven scenarios are
present exactly once: three transport scenarios and four observed-job-wait
scenarios. The canonical spec remains unchanged until archive.
