# Chunk Review: finance-ui-docs

## Round 1

- Scope: task 3.3 / finance UI docs
- Triggering input: implementation not yet started
- Verdict: pending
- Notes: awaiting implementation sub-agent

## Round 2

- Scope: task 3.3 / finance UI docs
- Verdict: needs changes

### Findings

- `docs/manual-e2e.md` now correctly removes arbitrary provider entry instructions and scopes operators to monobank-token + PKO-Enable Banking flows, matching the implementation intent. However, the PKO return flow wording is underspecified: it only says “return to `#/finance/connections`” but does not describe the required callback shape used in practice (`{origin}/?code=...&state=...#/finance/connections`) nor the one-time query cleanup behavior. This is important for an operator hand-flow and should be made explicit so the documented runbook matches UI behavior exactly.

### Artifact cleanup status

- No unexpected documentation artifacts changed for task 3.3 beyond the intended docs files; update to `tasks.md` is expected for scope tracking.
- `apps/signal-ui/ui-wireframe.md` and `docs/manual-e2e.md` contain only documentation edits.
- `openspec/changes/fix-bank-provider-token-connection/manager-status.md` remains modified in this changeset, which is outside strict task 3.3 docs scope and appears to be process bookkeeping (`in progress` + running implementation-chunk row). If this repo policy treats manager status files as canonical run artifacts, keep; otherwise revert before gate.

### Completion protocol status

- Non-coding task: no mandatory lint/test protocol required per `AGENTS.md`.
- No code files were modified in this chunk.

### Commit status

- No commit exists yet for this chunk (`review-chunk-finance-ui-docs.md` is still untracked/modified in working tree).
- A commit is required before the gate can pass.

### Safe to continue to the next chunk?

- No. Not until the manual callback flow wording is tightened to the exact return URL/query-string contract.

## Round 3

- Scope: task 3.3 / finance UI docs (post callback wording fix)
- Verdict: clean

### Findings

- No blocking findings. The updated docs now explicitly constrain finance connection flows to:
  - monobank token linking
  - PKO via Enable Banking
- `docs/manual-e2e.md` no longer instructs arbitrary provider names and keeps the same limitation in the flow steps.
- The PKO callback contract is now explicitly documented as `/?code=...&state=...#/finance/connections`.
- Query cleanup behavior is documented: consumed query parameters are cleared while the `#/finance/connections` hash route remains active.

### Artifact cleanup status

- No out-of-scope file changes are present in this chunk besides expected docs updates:
  - `apps/signal-ui/ui-wireframe.md`
  - `docs/manual-e2e.md`
  - `openspec/changes/fix-bank-provider-token-connection/tasks.md`
- No unexpected working-tree artifacts were introduced.

### Completion protocol status

- Docs-only change set: non-coding protocol applies.
- No `make affected-lint-test` run was required for this docs-only chunk.

### Commit status

- No commit exists yet for this chunk in `git` history.
- A commit for this review/chunk is still required before gate passage.

### Safe to continue to the next chunk?

- Yes.
