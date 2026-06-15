# Planning Review

## Round 1

- Scope: `add-durable-paper-backtest-audit`
- Trigger: review of proposal/design/tasks/specs created from the five Notion tickets.
- Verdict: changes requested.
- Findings:
  1. `tasks.md` chunk 1 is not fully dependency-safe. Task 1.2 asks the flow test to prove downstream governor/execution references are written back before chunk 2 defines the intent-based governor path and before chunk 3 defines the durable execution ledger references. Revise chunk 1.2 to stop at trace -> intent creation plus passing intent context into the next boundary, or move the downstream reference/write-back assertions into task 5.3 so chunk order stays strictly sequential.
  2. `design.md` leaves the persisted reason-code string style unresolved (`OK`/`MODE_NOT_ALLOWED` uppercase vs lower-kebab). That ambiguity is implementation-significant because the specs and tasks already assert concrete values. Resolve it in the planning artifacts before implementation; the simplest fix is to declare the uppercase ticket values as the canonical persisted strings everywhere in this change.
  3. The planning mostly stays minimal and covers the five ticket themes, but terminology is slightly inconsistent: `proposal.md` introduces `DatasetSnapshot` while `design.md`, `tasks.md`, and `specs/backtest-evaluation/spec.md` describe a dataset reference. Normalize this to one concept so chunk 5 does not imply an extra persistence object beyond the requested scaffold.
- Decision: not ready for implementation until the above revisions are made. After cleanup, keep the chunk order `1 -> 2 -> 3 -> 4 -> 5` and reserve the full cross-slice linkage proof for the final integration chunk.

## Round 2

- Scope: `add-durable-paper-backtest-audit`
- Trigger: planning revision after Round 1 review findings.
- Verdict: ready for implementation.
- Changes:
  1. Narrowed task 1.2 to trace -> intent creation and preparing stable intent context for the next governor boundary only; downstream governor/execution write-back proof remains in task 5.3.
  2. Declared uppercase snake-case governor reason-code strings as the canonical persisted format and reflected that in tasks and specs.
  3. Normalized dataset provenance terminology to compact dataset reference records and removed the `DatasetSnapshot` wording.
- Validation: `npx openspec validate add-durable-paper-backtest-audit --strict` succeeded on 2026-06-14.
- Decision: planning is clean; preserve strict chunk order `1 -> 2 -> 3 -> 4 -> 5` during implementation.
