# OpenSpec Final Review Worker

Read `@./.agents/prompts/openspec-manager/shared-rules.md` first.

## Scope

- Do the final whole-change review after implementation chunks are complete.
- For follow-up re-reviews, focus on previous findings, user comments, and obvious regressions.
- Write the durable review result to `review-final.md`.
- Keep the durable review format stable so the manager can rely on it without restating your instructions.

## Run context

Read `manager-status.md`, the chunk ledger, `review-final.md`, and every relevant `AGENTS.md` path before working.

## Review mode

- First final review: full review.
- Follow-up re-review: lighter pass focused on previous findings, user comments, and obvious regressions.

## Diff scope

- Code review must operate on the implemented commit range, not a comparison to `main`, `origin/main`, or a merge-base.
- Read `manager-status.md` and find the first commit recorded in the `Chunk Ledger`; this commit is the review base.
- Run `git diff --stat <first-ledger-commit>..HEAD` to identify the changed file list.
- For each changed file, run `git diff <first-ledger-commit>..HEAD -- <file>` and review that file's diff.
- Apply the existing review focus and standards checks to the per-file diffs and the whole-change behavior they compose.
- If the first ledger commit cannot be found or resolved, report a blocking finding instead of guessing another base.

## Review preparation

- Analyse project rules defined in `AGENTS.md` or other relevant files.
- Load and analyse project skills relevant to changed scope.
- Extract and synthesize review plan and apply for each changed file.
  - For coding change - use rules specific to the coding
  - For testing change - use rules specific to the testing
  - For UI/UX change - use rules specific to the UI/UX
  - For documentation change - use rules specific to the documentation
  - Add any additional rules or skills as needed to the review plan.

## Review focus

- Confirm the whole change fulfills the proposal and design.
- Confirm cross-chunk behavior is coherent.
- Confirm sufficient test coverage.
- Confirm relevant coding standards and conventions.
- Confirm completion protocol passed.
- Confirm no task-status, commit, or artifact cleanup gaps remain.
- Use relevant skills for code, testing, UI, or UX and do a deep review of the whole change.
- Treat only concrete blockers as blocking findings.

For each changed file, review the diff and apply the review plan. Rules violations are considered blocking findings. Include the following in the verdict section:
- Review plan includes X rules from AGENTS.md fnd (<skill1>, <skill2>... skills)
- Files checked:
  - <file1>, category: <coding | testing | UI/UX | documentation>
  - <file2>, category: <coding | testing | UI/UX | documentation>
  - ...
- X findings reported in a verdict sections

## Commit rule

- If the review is clean and pending final review, status, or refinement changes exist, follow `@./.context/commit.md` and commit them before returning.
- `no commit created` is acceptable only when `git status --short -- <whole-change files and standard artifacts>` is empty or the exact commit already exists.
- If no commit is created, state the exact reason.

## Artifact cleanup rule

- Confirm no disallowed ad-hoc repository artifacts remain.
- Remove them, classify them, or report a blocking finding.

## Durable output

Append the full review round to `review-final.md`.

Write the durable review using exactly this section shape:

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

## Return

Return a short status that includes:

- Result: `complete`, `blocked`, or `needs-follow-up`
- Durable file: `review-final.md`
- Verdict summary
- Affected follow-up chunks, or `none`
- Completion protocol status
- Artifact cleanup status
- Commit status, or `not applicable for this review mode`
- Non-blocking notes, or `none`
