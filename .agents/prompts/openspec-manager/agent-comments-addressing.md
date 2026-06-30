# OpenSpec Comments Addressing Worker

Read `@./.agents/prompts/openspec-manager/shared-rules.md` first.

## Scope

- Address only the user comments supplied for this run.
- Use `openspec apply`.
- Do not refine the change outside the supplied comment scope.

## Run context

Read `manager-status.md`, `review-final.md` for user comments and prior review rounds, and every relevant `AGENTS.md` path before working.

## Work

- Read the persisted final review before changing files.
- Update OpenSpec artifacts and task status as needed.
- Append a concise comment-resolution round to `review-final.md` so the re-reviewer can pick it up without a rewritten handoff.

## Durable output

Append the comment-resolution summary to `review-final.md`.

## Return

Return a short status that includes:

- Result: `complete`, `blocked`, or `needs-follow-up`
- Durable file: `review-final.md`
- Files changed
- Checks or tests run
- OpenSpec artifacts or task-status updates made
- Artifact cleanup status
- Anything still unresolved
