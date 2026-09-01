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
- **Chunk reviewer:** independently performs a focused review and verification 
  (using repo standards and best practices) of one implemented chunk and returns 
  either commit & safe-to-continue signal or actionable findings.
- **Final reviewer:** independently reviews the complete implemented change and
  returns either a clean verdict or findings mapped to follow-up chunks.

## Workflow

```text
plan -> plan review -> (implement -> chunk review -> commit)* -> final review
     -> user review/corrections -> archive -> submit
```

1. **Plan:** invoke a planner, then a fresh plan reviewer. Send review findings
   back to a planner until the plan is clean. Chunks must be small and sequential,
   preserve parent-task order, and come from the clean plan review. Commit the
   clean plan before implementation.
2. **Implement:** for each chunk in order, invoke an implementer and then a fresh
   chunk reviewer. Send findings through another implement-and-review round until
   safe, then commit the chunk before starting the next one. Make sure to include 
   previous findings if it's a subsequent re-review round. The re-review should 
   only focus on previously identified issues and make sure new changes are not 
   adding new issues.
3. **Final review:** invoke a fresh final reviewer after all chunks. Convert any
   findings into ordered follow-up chunks and process them through the same
   implement, review, and commit loop until the whole change is clean. Make sure to 
   include previous findings if it's a subsequent re-review round. The re-review should 
   only focus on previously identified issues and make sure new changes are not 
   adding new issues.
4. **User review:** present the clean result and wait. For corrections, invoke a
   planner to add explicit tasks and update `design.md` only when the design must
   change. Process the resulting correction chunks normally, then invoke a final
   reviewer with the narrow assignment of verifying only that comment round.
   Repeat until approval.
5. **Finish:** natural-language approval completes review unless it also asks to
   pause. Delegate OpenSpec archive, commit archive changes, and submit the work
   unless the user opts out of submission.

For an existing change, inspect `proposal.md`, `design.md`, and `tasks.md`, then
resume at the earliest incomplete safe phase. Do not rerun `openspec propose`
when those artifacts are ready for review. Follow possible user instructions 
to identify a starting phase if provided.

Do not pass a gate without a clean review verdict, the applicable repository
completion protocol, and a clean relevant worktree after the required commit.
Keep all coordination artifacts in Crew Manager's existing `tmp/crew-manager/`
files; only OpenSpec's normal plan/task artifacts belong in the change directory.
