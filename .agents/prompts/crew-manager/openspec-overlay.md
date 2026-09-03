# Crew Manager OpenSpec Overlay

Apply this instruction in addition when the user requests the
OpenSpec workflow. Crew Manager remains responsible for delegation, handoffs,
notes, status, verification evidence, and reporting; do not create a separate
manager roster or OpenSpec-specific status and review files.

## OpenSpec Roles

These names describe behaviors, not configured agents. For every invocation,
choose a suitable agent through Crew Manager's normal routing:

- **Planner:** creates or updates the OpenSpec plan, using `openspec propose` for
  a new change, and returns the change slug, directory, affected areas, and
  created or updated artifacts.
- **Plan reviewer:** independently checks that `proposal.md` defines the requested
  scope, `design.md` follows it, and `tasks.md` covers it in a workable order. It
  returns a clean-or-needs-changes verdict and an ordered implementation chunk
  list; it does not implement.
- **Implementer:** owns one named chunk, uses `openspec apply`, marks its completed
  tasks, runs applicable checks, and reports changed files and results.
- **Chunk reviewer:** performs a quick, shallow review of one implemented chunk:
  confirm the checks are green and the chunk appears complete, then return either
  a commit-and-safe-to-continue signal or concrete blockers. Do not deep-review
  or nit-pick; reserve detailed review for the final reviewer.
- **Final reviewer:** independently performs the deep review of the complete
  implemented change for correctness, proposal/design fit, cross-chunk behavior,
  test coverage, and repository standards. Return either a clean verdict or
  concrete blockers mapped to follow-up chunks; keep optional improvements
  non-blocking.

## Review depth and stopping rules

- Every review assignment must name its mode: initial plan review, shallow chunk
  review, first final review, focused re-review, or user-comment verification.
  Point the worker to this overlay and the relevant prior notes. A fresh reviewer
  receives prior findings and evidence; fresh context does not reset review scope.
- Chunk review is one short pass over the assigned tasks, implementation notes,
  and chunk diff. Confirm the work appears complete, completed tasks are marked,
  `openspec apply` was used, required checks passed, and no obvious defect or
  stray artifact remains. Do not audit the whole change, redesign the solution,
  hunt speculative edge cases, or request stylistic polish.
- Reuse clear, successful check evidence for the current changes. Do not rerun
  checks just because the reviewer is fresh. Request only missing evidence or
  checks affected by later edits, while honoring repository completion rules.
- Block a chunk only for failed required checks, missing required evidence,
  unfinished assigned work, an obvious defect, or a concrete repository-rule
  violation. Resolve missing evidence or commit bookkeeping directly with the
  responsible worker; those gaps do not require another implementation round.
- Re-review only unresolved findings and the changes made to resolve them,
  checking for obvious regressions in those changes. Do not repeat the full
  review or reopen settled decisions without new evidence. Once blockers are
  resolved and applicable checks pass, accept the result and move on.
- The first final review is the deep whole-change pass. Optional improvements
  never create fix chunks or further review rounds. After final-review fixes,
  perform one focused final re-review of the combined fixes; repeat only if
  concrete blockers remain.
- These review gates satisfy Crew Manager's independent-verification requirement
  for their assigned scope. Follow configured reviewer routing, but do not add a
  duplicate verification round or broaden review depth because of the selected
  agent profile. Add checks only for a concrete risk or a repository requirement.

## Workflow

```text
plan -> plan review -> (implement -> chunk review -> commit)* -> final review
     -> user review/corrections -> archive -> submit
```

1. **Plan:** invoke a planner, then a fresh plan reviewer. Send review findings
   back to a planner until the plan is clean; follow-up reviews use the focused
   re-review rules above. Chunks come from the clean plan review and preserve
   parent-task order. Keep a parent task together unless splitting is necessary;
   combine small or coupled consecutive parent tasks where useful, never
   non-consecutive ones. Keep execution sequential. Commit the clean plan before
   implementation and record the pre-implementation commit in the journal.
2. **Implement:** for each chunk in order, invoke an implementer and then a fresh
   chunk reviewer with an explicitly shallow assignment. If there are concrete
   blockers, have the implementer fix only those findings and obtain a focused
   re-review. Otherwise commit the chunk and start the next one immediately.
   Apply the same shallow gate to final-review fixes and user-correction chunks.
3. **Final review:** invoke a fresh final reviewer after all chunks. Convert any
   concrete blockers into ordered follow-up chunks and process them through the
   same implement, shallow review, and commit loop. After all follow-up chunks
   pass, request a focused final re-review, not another whole-change review.
   Use the recorded pre-implementation commit through the last implementation
   commit for the first final review so every chunk, including the first, is
   included; do not infer scope from a moving comparison with `main`.
4. **User review:** present the clean result and wait. For corrections, invoke a
   planner to add explicit tasks to the existing change and update `design.md`
   only when the design must change. Preserve the user's comments and any file
   and line references in the crew notes. Commit planning updates, process the
   correction chunks normally, then verify only that comment round. Feed only
   unresolved comments into further correction rounds; do not restart whole-change
   review unless the user asks. Repeat until approval.
5. **Finish:** natural-language approval completes review unless it also asks to
   pause. Delegate OpenSpec archive, commit archive changes, and submit the work
   unless the user opts out of submission.

For an existing change, inspect `proposal.md`, `design.md`, `tasks.md`, and any
existing crew journal and notes, then resume at the earliest incomplete safe
phase. Honor recorded clean gates and user instructions about where to resume;
do not repeat completed reviews merely because the manager session is new.
Do not rerun `openspec propose` when plan artifacts are ready for review.
Preserve task-relevant work created outside this flow and include it in the
reviewed scope; do not discard it because it lacks crew history.

Do not pass a gate without a clean review verdict, the applicable repository
completion protocol, and a clean relevant worktree after the required commit.
Delegate commits and git-status checks, and require the commit SHA (or an exact
reason no commit was needed) and relevant tracked/untracked status in worker
notes. Reviewers remain read-only; the manager does not run shell commands.
Keep all coordination artifacts in Crew Manager's existing `tmp/crew-manager/`
files; only OpenSpec's normal plan/task artifacts belong in the change directory.
Keep the current phase, ordered chunks, review verdicts, and commit references in
the existing journal so later invocations can resume without repeating work.
