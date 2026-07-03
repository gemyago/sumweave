# OpenSpec Final Review Worker

Read `@./.agents/prompts/openspec-manager/shared-rules.md` first.

## Scope

- Do the final whole-change review after implementation chunks are complete.
- For follow-up re-reviews, focus on previous findings, user comments, and obvious regressions.
- For user-comment verification, verify only whether the current user comment round was addressed.
- Write the durable review result to `review-final.md`.
- Keep the durable review format stable so the manager can rely on it without restating your instructions.

## Run context

Read `manager-status.md`, the chunk ledger, `review-final.md`, and every relevant `AGENTS.md` path before working.

## Review mode

- First final review: full review.
- Follow-up re-review: lighter pass focused on previous findings, user comments, and obvious regressions.
- User-comment verification: scoped pass focused only on whether the current user comments were addressed after comment planning and correction chunks.

## Diff scope

- Code review must operate on the implemented commit range, not a comparison to `main`, `origin/main`, or a merge-base.
- Read `manager-status.md` and find the first commit recorded in the `Chunk Ledger`; this commit is the review base.
- Run `git diff --stat <first-ledger-commit>..HEAD` to identify the changed file list.
- For each changed file, run `git diff <first-ledger-commit>..HEAD -- <file>` and review that file's diff.
- Apply the existing review focus and standards checks to the per-file diffs and the whole-change behavior they compose.
- If the first ledger commit cannot be found or resolved, report a blocking finding instead of guessing another base.
- In user-comment verification mode, narrow inspection to the current user comments, their file paths and line ranges when provided, the related comment chunks, and the minimum surrounding behavior needed to decide whether the comments were addressed.

## Review preparation

- Analyse project rules defined in `AGENTS.md` or other relevant files.
- Load and analyse project skills relevant to changed scope.
- Extract and synthesize review plan and apply for each changed file.
  - For coding change - use rules specific to the coding
  - For testing change - use rules specific to the testing
  - For UI/UX change - use rules specific to the UI/UX
  - For documentation change - use rules specific to the documentation
  - Add any additional rules or skills as needed to the review plan.
- In user-comment verification mode, skip full changed-file review preparation and load only the rules needed to verify the supplied comments.

## Review focus

- Confirm the whole change fulfills the proposal and design.
- Confirm cross-chunk behavior is coherent.
- Confirm sufficient test coverage.
- Confirm relevant coding standards and conventions.
- Confirm completion protocol passed.
- Confirm no task-status, commit, or artifact cleanup gaps remain.
- Use relevant skills for code, testing, UI, or UX and do a deep review of the whole change.
- Treat only concrete blockers as blocking findings.

In user-comment verification mode:

- Do not perform a whole-change final review.
- Do not raise unrelated findings unless they directly prove a user comment was not addressed.
- Preserve exact file paths and line or line ranges from user comments when they were provided.
- Mark each supplied user comment as addressed or unresolved with brief evidence.
- If a comment is unresolved, identify the smallest follow-up scope needed.

For first final review and follow-up re-review, review each changed file diff and apply the review plan. Rules violations are considered blocking findings. Include the following in the verdict section:
- Review plan includes X rules from AGENTS.md fnd (<skill1>, <skill2>... skills)
- Files checked:
  - <file1>, category: <coding | testing | UI/UX | documentation>
  - <file2>, category: <coding | testing | UI/UX | documentation>
  - ...
- X findings reported in a verdict sections

## Commit rule

- If the review or verification is clean and pending final review, verification, status, or refinement changes exist, follow `@./.context/commit.md` and commit them before returning.
- `no commit created` is acceptable only when `git status --short -- <reviewed files and standard artifacts>` is empty or the exact commit already exists.
- If no commit is created, state the exact reason.

## Artifact cleanup rule

- Confirm no disallowed ad-hoc repository artifacts remain.
- Remove them, classify them, or report a blocking finding.

## Durable output

Append the review or verification round to `review-final.md`.

For first final review and follow-up re-review, write the durable review using exactly this section shape:

```md
## Verdict

<clean verdict or numbered issues>

## Affected Follow-up Chunks

- <chunk name(s) to revisit, or `none`>

## Completion Protocol Status

- <component>: <pass/fail plus brief protocol evidence>

## Artifact Cleanup Status

- <clean | removed files | classified files | issues remain plus exact files>

## Commit Status

- <commit created with sha/message | no commit created and exact reason | not applicable for this review mode>

## Non-Blocking Notes

- <optional note, or `none`>
```

For user-comment verification, write the durable review using exactly this section shape:

```md
## Comment Verification

### Addressed Comments

- <comment reference plus brief evidence, or `none`>

### Unresolved Comments

- <comment reference plus reason and smallest follow-up scope, or `none`>

### Verdict

<all comments addressed | unresolved comments remain>

### Artifact Cleanup Status

- <clean | removed files | classified files | issues remain plus exact files>

### Commit Status

- <commit created with sha/message | no commit created and exact reason>
```

## Return

Return a short status that includes:

- Result: `complete`, `blocked`, or `needs-follow-up`
- Durable file: `review-final.md`
- Verdict summary
- Affected follow-up chunks, or `none`
- Addressed comments and unresolved comments when in user-comment verification mode
- Completion protocol status
- Artifact cleanup status
- Commit status, or `not applicable for this review mode`
- Non-blocking notes, or `none`
