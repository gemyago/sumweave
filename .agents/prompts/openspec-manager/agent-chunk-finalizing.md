# OpenSpec Chunk Finalizing Worker

Read `@./.agents/prompts/openspec-manager/shared-rules.md` first.

## Scope

- Do a quick, shallow review of the just-implemented chunk.
- Confirm the chunk is safe to continue before the manager moves on.
- Write the durable review result to `review-chunk-<chunk-slug>.md`.

## Run context

Read `manager-status.md`, the active chunk from its chunk ledger, and the chunk review file for this run before working. Read every relevant `AGENTS.md` path before reviewing.

## Review focus

- Confirm the chunk matches the requested tasks.
- Confirm no obvious issue was introduced.
- Confirm applicable completion protocol passed.
- Confirm `openspec apply` was used.
- Confirm completed OpenSpec tasks were marked.
- Confirm no disallowed ad-hoc repository artifacts remain.
- Decide whether it is safe to continue.
- Use relevant skills for code, testing, UI, or UX and do a quick review of the chunk.

## Commit rule

- If clean and pending changes exist for this chunk, follow `@./.context/commit.md` and commit them before returning.
- Include pending OpenSpec and standard review or status files.
- `no commit created` is acceptable only when `git status --short -- <chunk files and standard artifacts>` is empty or the exact commit already exists.
- If ad-hoc artifacts remain unremoved and unclassified, report `not safe to continue`.

## Durable output

Append the full chunk review round to `review-chunk-<chunk-slug>.md`.

## Return

Return a short status that includes:

- Result: `complete`, `blocked`, or `needs-follow-up`
- Durable file: `review-chunk-<chunk-slug>.md`
- Verdict summary
- Continue decision
- Completion protocol status
- Artifact cleanup status
- Commit status
- Affected follow-up chunks, or `none`
