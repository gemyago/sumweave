# Chunk Review: ui-strategy-workspace

## Round 1

- Scope: `apps/signal-ui` strategy/evaluation routes, navigation, API wrappers, page tests, and `ui-wireframe.md` updates for the UI strategy workspace chunk
- Triggering input: implementation-finalizing review of reported UI chunk
- Findings:
  1. Evaluation run validation does not actually enforce UTC timestamps. `validateRunForm()` accepts any `Date`-parsable string, so inputs without a `Z`/offset suffix can pass client validation even though this route is documented and labeled as UTC-only (`apps/signal-ui/src/pages/Evaluations.svelte:96-119`, `apps/signal-ui/src/pages/Evaluations.svelte:185-193`). This can submit ambiguous local-time ranges instead of the required UTC range.
  2. Evaluation detail does not react to in-app `runId` route changes. The page loads data only from `onMount()` and has no reactive follow-up when `params.runId` changes, so navigating from one evaluation detail link to another can leave stale summary/evidence on screen for the previous run (`apps/signal-ui/src/pages/EvaluationDetail.svelte:25-55`).
- Verdict: changes requested
- Artifact cleanup status: no stray temporary artifacts found; current dirty files are scoped to the UI chunk plus standard OpenSpec review bookkeeping, but the chunk gate is not clean because blocking UI issues remain and relevant chunk artifacts are still modified/untracked
- Completion protocol status:
  - `direnv exec /Users/jenya/projects/signal-foundry/apps/signal-ui make test` ✓
  - `make affected-lint-test` not re-run in this review round because the chunk is blocked on review findings
  - AGENTS.md updates: no changes needed
- Commit status: not committed; blocking review findings remain and the chunk is not yet safe to finalize

## Round 2

- Scope: follow-up UI fix iteration for UTC run validation and evaluation-detail route reactivity only
- Triggering input: implementation-finalizing review of the reported UI follow-up fix
- Findings:
  - None. `Evaluations.svelte` now rejects ambiguous timestamps unless they include an explicit timezone suffix, and `EvaluationDetail.svelte` reloads from `params.runId` changes while ignoring stale in-flight responses.
- Verdict: approved
- Artifact cleanup status: no stray temporary artifacts found; `openspec/changes/add-strategy-workspace-v0/manager-status.md` remains intentionally dirty as manager bookkeeping and was left untouched here
- Completion protocol status:
  - `direnv exec /Users/jenya/projects/signal-foundry/apps/signal-ui make test` ✓
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` ✓
  - AGENTS.md updates: no changes needed
- Commit status: pending finalizing-sub-agent commit of the UI chunk artifacts
