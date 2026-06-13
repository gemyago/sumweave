# Planning Review

Planning review history for `strategy-layer`.

## Verdict

Status: revise before implementation.

1. [P1] Keep strategy evaluation request types out of the shared `domain` kernel. The proposal currently says shared domain should hold "evaluation requests" ([proposal.md](/Users/jenya/projects/signal-foundry/openspec/changes/strategy-layer/proposal.md:8)), and the design also pulls "strategy parameters" into `runtime/domain` as part of the shared set ([design.md](/Users/jenya/projects/signal-foundry/openspec/changes/strategy-layer/design.md:39), [design.md](/Users/jenya/projects/signal-foundry/openspec/changes/strategy-layer/design.md:41)). That is broader than the architecture allows: `domain/` is for small, stable cross-slice product concepts, not orchestration concerns ([docs/ARCHITECTURE.md](/Users/jenya/projects/signal-foundry/docs/ARCHITECTURE.md:56), [docs/ARCHITECTURE.md](/Users/jenya/projects/signal-foundry/docs/ARCHITECTURE.md:58)). The existing analytics slice already follows the right pattern by keeping `CalculateCandlesRequest` in the slice package while storing only the reusable identity/output types in `domain` ([runtime/analytics/service.go](/Users/jenya/projects/signal-foundry/runtime/analytics/service.go:33), [runtime/analytics/service.go](/Users/jenya/projects/signal-foundry/runtime/analytics/service.go:58)). The plan should explicitly keep service request types, and any evaluation-only parameter wrapper, in `runtime/strategy`, while reserving `runtime/domain` for the cross-slice strategy identity and candidate action records.

2. [P2] Rework parent task 4 so runtime-only guardrails are acceptance criteria, not a standalone implementation chunk. The current task asks for tests proving the absence of persistence, HTTP routes, and backend DI wiring ([tasks.md](/Users/jenya/projects/signal-foundry/openspec/changes/strategy-layer/tasks.md:15), [tasks.md](/Users/jenya/projects/signal-foundry/openspec/changes/strategy-layer/tasks.md:17)). That does not map cleanly to one coherent code change, and it encourages brittle negative tests or scattered late-stage edits across unrelated modules. The non-goals already capture this scope well ([design.md](/Users/jenya/projects/signal-foundry/openspec/changes/strategy-layer/design.md:17), [design.md](/Users/jenya/projects/signal-foundry/openspec/changes/strategy-layer/design.md:23)). Fold these guardrails into the acceptance criteria for tasks 1-3 and, if needed, keep only one small final verification step tied to actual touched runtime files.

The rest of the plan is directionally strong: it matches the architecture’s `Data -> Analytics -> Strategy -> Governor -> Execution` flow, keeps AI and venue mechanics out of the deterministic path, and sequences the runtime work sensibly once the two issues above are corrected.

## Complexity and sub-agents required

Medium complexity.

Recommended chunking after the task list is corrected:

1. Parent task 1 as one sequential implementation chunk.
2. Parent task 2 as one sequential implementation chunk.
3. Parent task 3 as one sequential implementation chunk.

Do not allocate a separate implementation sub-agent to the current parent task 4. Its content should be folded into chunk acceptance criteria and final review checks.

## Artifact Cleanup Status

No ad-hoc repository artifacts were found in this review pass. Existing OpenSpec files in `openspec/changes/strategy-layer/` are standard workflow artifacts and should remain.

## Commit Status

No commit was created in this review pass. Updated artifact: [review-planning.md](/Users/jenya/projects/signal-foundry/openspec/changes/strategy-layer/review-planning.md:1).

## Planning Redo Resolution

Status: revised for re-review.

- Addressed P1 by changing the proposal, design, spec, and tasks so `runtime/domain` additions are limited to cross-slice strategy identity and candidate action records.
- Addressed P1 by explicitly keeping strategy evaluation request types and evaluation-only moving-average parameter wrappers in `runtime/strategy`.
- Addressed P2 by removing the standalone runtime-only guardrail parent task and folding the no-persistence, no-HTTP-route, and no-backend-DI scope into design-level implementation guardrails plus task acceptance language.
- Preserved the deterministic strategy-layer design: `Data -> Analytics -> Strategy -> Governor -> Execution`, on-demand analytics consumption, exact `[start, end)` range semantics, stable action ordering, and quality propagation.

## Artifact Cleanup Status

Clean. No ad-hoc repository artifacts were created during the planning redo; only standard OpenSpec change artifacts and standard manager/review artifacts were updated.

## Commit Status

No commit was created in this planning redo pass. The `strategy-layer` OpenSpec change directory remains untracked and ready for the next workflow gate.

## Re-review Verdict

Status: approved for implementation.

No new planning findings in this follow-up pass.

- The prior P1 issue is resolved: the proposal, design, spec, and tasks now consistently keep evaluation request types and the evaluation-only moving-average parameter wrapper in `runtime/strategy`, while limiting `runtime/domain` to cross-slice strategy identity and candidate action records.
- The prior P2 issue is resolved: the old runtime-only guardrail work is no longer a standalone parent task and is instead expressed as scope guardrails plus acceptance language inside the runtime implementation tasks.
- The plan still matches the architecture direction in [docs/ARCHITECTURE.md](/Users/jenya/projects/signal-foundry/docs/ARCHITECTURE.md:12) and keeps the deterministic path as `Data -> Analytics -> Strategy -> Governor -> Execution`.
- `tasks.md` is complete and logically ordered for sequential implementation: shared domain first, then strategy service/request wiring, then moving-average crossover behavior and action assembly.
- No planning issue was found for scattered related work across non-consecutive parent tasks.

## Complexity and sub-agents required

Medium complexity.

Recommended implementation chunking remains:

1. Parent task 1 as one sequential implementation chunk.
2. Parent task 2 as one sequential implementation chunk.
3. Parent task 3 as one sequential implementation chunk.

Use one implementation sub-agent per parent task. Do not combine tasks, do not split parent tasks, and keep the existing parent-task order intact.

## Artifact Cleanup Status

Clean. No ad-hoc repository artifacts were found or created in this re-review pass. Existing OpenSpec files in `openspec/changes/strategy-layer/` remain standard workflow artifacts.

## Commit Status

No commit was created in this re-review pass. The `strategy-layer` OpenSpec change directory is still untracked as a pending OpenSpec work artifact.
