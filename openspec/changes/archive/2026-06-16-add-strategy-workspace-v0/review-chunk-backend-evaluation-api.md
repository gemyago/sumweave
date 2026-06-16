# Chunk Review: backend-evaluation-api

## Round 1

- Scope: `apps/signal-foundry/internal/app`, `apps/signal-foundry/internal/api/http`, generated `v1routes` evaluation artifacts, and supporting `runtime/flows`, `runtime/backtest`, and `runtime/execution` changes for the backend evaluation API chunk
- Triggering input: implementation-finalizing review of reported backend chunk
- Findings:
  1. `CreateEvaluation` does not preserve the selected governor policy reference for non-default policies. It resolves an explicit `GovernorPolicyHash`, but still hardcodes `GovernorPolicyID`/`GovernorPolicyVersion` to the default values in both the durable-flow request and the data-unavailable failure path, so stored/detail policy references can disagree with the requested artifact hash (`apps/signal-foundry/internal/app/evaluation_workspace.go:451-453`, `apps/signal-foundry/internal/app/evaluation_workspace.go:756-758`).
  2. Evaluation evidence is not reliably scoped to a single run. Traces and intents are filtered by `backtest_run_id`, but position snapshots are queried only by strategy/time range and portfolio snapshots only by mode/time range, so overlapping or repeated evaluations can leak snapshot evidence into the wrong run detail/evidence response (`apps/signal-foundry/internal/app/evaluation_workspace.go:949-977`).
  3. Report lookup failures are silently discarded. `ListEvaluations` and `buildDetail` ignore `reportForRun` errors, which can turn storage/query failures into incomplete "successful" payloads instead of surfacing an error (`apps/signal-foundry/internal/app/evaluation_workspace.go:507`, `apps/signal-foundry/internal/app/evaluation_workspace.go:817`).
- Verdict: changes requested
- Artifact cleanup status: no stray temporary artifacts found; generated `v1routes` evaluation artifacts are present; manager-owned `manager-status.md` remains separately modified for bookkeeping; the chunk gate is not clean because relevant source/generated artifacts are still dirty and the review found blocking issues
- Completion protocol status:
  - `go test ./apps/signal-foundry/internal/app ./apps/signal-foundry/internal/api/http/...` ✓
  - `make affected-lint-test` not re-run in this review round because the chunk is blocked on review findings
  - AGENTS.md updates: no changes needed
- Commit status: not committed; blocking review findings remain and relevant chunk artifacts are still modified/untracked

## Round 2

- Scope: follow-up fix iteration for the `backend-evaluation-api` chunk only
- Triggering input: implementation-finalizing review of the fix round addressing the three Round 1 blockers
- Findings: none; the fix round correctly preserves explicit governor policy selection in persisted runs/details, scopes evaluation evidence to the current run via linked fill/backtest references, and now surfaces report lookup failures from both list and detail/report paths
- Verdict: accepted
- Artifact cleanup status: no stray temporary artifacts found; generated `v1routes` evaluation artifacts remain intentionally present; `openspec/changes/add-strategy-workspace-v0/manager-status.md` is still separately modified for manager bookkeeping only
- Completion protocol status:
  - `go test ./apps/signal-foundry/internal/app ./apps/signal-foundry/internal/api/http/... ./runtime/flows ./runtime/backtest ./runtime/execution` ✓
  - `make affected-lint-test` ✓
  - AGENTS.md updates: no changes needed
- Commit status: pending at review time; ready to commit together with the chunk artifacts after this review note is appended
