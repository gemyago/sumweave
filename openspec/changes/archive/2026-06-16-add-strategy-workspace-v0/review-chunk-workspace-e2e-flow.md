# Chunk Review: workspace-e2e-flow

- Scope: optional happy-path integration/e2e coverage for the strategy workspace flow only
- Result: unsupported/skipped

## Harness sufficiency check

- The repository does not currently ship a real product e2e harness for this flow.
- `tests/` contains only the agent `integration-cli` scenario harness, and `tests/AGENTS.md` explicitly treats that high-level harness as agent-test reference material rather than product QA architecture.
- `apps/signal-ui` has focused Vitest component/route tests with mocked strategy-workspace API calls, but no existing browser runner, backend orchestration fixture, or full-stack login + strategy + evaluation flow harness.
- This chunk also depends on deterministic local candle data for a truthful happy-path evaluation; that support is not established by an existing reusable test fixture in the repo.

## Documented skipped case

- Skipped implementing a new strategy-workspace happy-path e2e test because doing so would require inventing new harness/runtime setup beyond this optional chunk.
- The current repository support is sufficient for focused mocked UI tests and backend/service tests, but not for a meaningful end-to-end login → duplicate demo → edit/validate/save → run evaluation → open detail flow against real app wiring and persisted evaluation evidence.

## Safe follow-up boundary

- This optional chunk is safe to leave incomplete until the repo has both:
  1. an adopted product e2e harness for UI + backend orchestration, and
  2. a deterministic local candle-data fixture (or an explicitly supported failed-run flow fixture) usable in CI/local runs.

## Finalizing review

- Scope: review only the optional `workspace-e2e-flow` chunk against task 5.1
- Triggering input: implementation-finalizing validation of the documented unsupported/skipped decision
- Findings: none; the skipped decision is correctly scoped and supported by current repo boundaries
- Verification notes:
  - Task 5.1 explicitly allows a documented skipped/unsupported harness case when existing harness support is insufficient.
  - `tests/AGENTS.md` treats the current `integration-cli` high-level harness as template-derived reference material rather than adopted product QA architecture.
  - `apps/signal-ui/package.json` and `apps/signal-ui/project.json` expose Vitest-focused unit/component coverage only; there is no product browser e2e runner/target to extend without inventing new setup.
  - The current UI strategy/evaluation coverage is mock-driven page/API testing, which is useful but not a meaningful real-app login → duplicate → edit/save → run → evidence flow.
  - No established deterministic candle-data fixture or supported failed-run e2e fixture was found for this product path.
- Verdict: accepted as unsupported/skipped
- Artifact cleanup status: review artifact is correctly limited to this OpenSpec review note; `openspec/changes/add-strategy-workspace-v0/manager-status.md` remains intentionally dirty manager bookkeeping and is out of chunk scope
- Completion protocol status:
  - Non-coding review/documentation task; no lint/test run required by repo protocol
  - AGENTS.md updates: no changes needed
- Commit status: pending finalizing-sub-agent commit after this review note update
