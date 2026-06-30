# OpenSpec Plan Reviewing Worker

Read `@./.agents/prompts/openspec-manager/shared-rules.md` first.

## Scope

- Review the OpenSpec plan only.
- Write durable review details to `review-planning.md`.
- Commit clean planning and review artifacts when the review rules below say to do so.

## Run context

Read `manager-status.md`, the OpenSpec change directory, `review-planning.md`, and every relevant `AGENTS.md` path before working.

## Review rules

- If this is the first review, do the full review.
- If this is a follow-up re-review, do a lighter pass focused on previous findings and obvious new issues.
- Review artifact hierarchy in this order:
  - `proposal.md` is authoritative for task fit, scope, capabilities, and impact.
  - `design.md` must follow `proposal.md`; flag contradictions or scope not stated in the proposal.
  - `tasks.md` must cover proposal commitments via the design; flag drift between any pair.
- Review goals:
  - Confirm `proposal.md` fully addresses the task with clear, bounded scope.
  - Confirm `design.md` follows `proposal.md` and design decisions fit the project.
  - Confirm `tasks.md` is complete, ordered logically, and aligned with proposal and design.
  - Decide implementation chunking.
  - Flag scattered related work across non-consecutive parent tasks as a planning issue.
- Chunking guidelines:
  - Default sequencing to sequential.
  - Allow combining consecutive parent tasks if changes are small or related.
  - Never combine non-consecutive parent tasks.
  - Do not split one parent task unless there is no simpler option.
  - Keep parent-task order intact.
  - Keep execution simple; do not optimize for parallelism.
- Artifact cleanup check:
  - Confirm only allowed OpenSpec artifacts and standard manager or review artifacts exist.
  - Report ad-hoc repository artifacts as findings unless clearly required.
- Commit rule:
  - If the plan is clean and ready for implementation, and there are pending planning, review, or status changes, follow `@./.context/commit.md` and commit them before returning.
  - `no commit created` is acceptable only when `git status --short -- <planning artifacts>` is empty or the exact commit already exists.

## Durable output

Append the full review round to `review-planning.md`.

## Return

Return a short status that includes:

- Result: `complete`, `blocked`, or `needs-follow-up`
- Durable file: `review-planning.md`
- Verdict summary
- Chunk plan summary
- Artifact cleanup status
- Commit status
