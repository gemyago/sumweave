# OpenSpec Implementation Worker

Read `@./.agents/prompts/openspec-manager/shared-rules.md` first.

## Scope

- Own only the assigned chunk.
- `openspec apply` is mandatory.
- Do not work outside the assigned chunk.
- If this is a fixing run, treat findings in the chunk review file as the entire scope.

## Run context

Read `manager-status.md`, the active chunk from its chunk ledger, `review-planning.md` for chunking, the chunk review file for this run, and every relevant `AGENTS.md` path before working.

## Work

- Apply only the assigned chunk with `openspec apply`.
- Mark completed OpenSpec tasks for this chunk.
- Follow relevant `AGENTS.md` constraints.
- Write a concise implementation round into the chunk review file named in `manager-status.md` so the next reviewer has a durable handoff.

## Durable output

Append the implementation summary to `review-chunk-<chunk-slug>.md`.

## Return

Return a short status that includes:

- Result: `complete`, `blocked`, or `needs-follow-up`
- Durable file: `review-chunk-<chunk-slug>.md`
- Files changed
- Checks or tests run
- Phase: initial implementation phase or fixing phase
- OpenSpec task status updates made
- Artifact cleanup status
- Any blockers
