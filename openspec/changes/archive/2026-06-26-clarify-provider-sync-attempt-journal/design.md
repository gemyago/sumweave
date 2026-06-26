## Context

Current provider sync v2 work models `domain.ProviderSyncState` as the state seam between orchestration and persistence. The latest implementation and archived OpenSpec change defined the journal as succeeded-only:

- the journal store loads the latest succeeded snapshot
- only succeeded chunks append rows
- persisted window columns represent successful coverage

That works for resume semantics, but it also causes confusing mixed meaning in the orchestrator. The state object used before chunk execution carries current-attempt metadata (`AttemptedAt`, `JobID`) together with copied prior-success metadata (`SucceededAt`, `SuccessfulWindow`).

The proposed direction is to keep one state journal but make every row represent one concrete chunk attempt.

## Goals / Non-Goals

**Goals:**

- Keep one provider sync state journal table and one dedicated journal store component.
- Make the latest loaded state mean the newest appended attempt for a connection.
- Require each journal row to store the exact chunk window that was attempted.
- Make success explicit through nullable success fields instead of through "this row exists only because it succeeded".
- Keep the orchestration flow clear: plan first, then create and append per-chunk attempt states.

**Non-Goals:**

- Split attempt records and success checkpoints into separate tables or stores.
- Add `ProviderSyncRun` persistence in this change.
- Add HTTP, UI, or job-scheduler behavior.
- Preserve the old succeeded-only semantics for the journal load seam.

## Decisions

1. Keep one append-only journal, but redefine each row as one attempt state.

   The journal remains scoped by connection and append-only, but rows no longer mean "succeeded checkpoint only". A row now means "the state recorded for one attempted chunk window".

   This keeps the storage model simple while removing the need to overload a future row with hidden "current attempt plus copied old success window" semantics.

2. Rename the load seam to `LoadLastState`.

   The current method name is no longer accurate once failure rows are appended too. The dedicated journal store and orchestrator seam should therefore load the newest appended row for a connection and name that behavior directly.

   The orchestration layer can then inspect nullable success fields to decide whether the latest state is resumable success coverage or just the latest failed attempt.

3. Rename the state window itself and replace successful-window columns with required attempt-window columns.

   The domain field should be renamed from `SuccessfulWindow` to `Window`, and `successful_window_start` / `successful_window_end` should become `window_start` / `window_end`. They should be required on every row because orchestration always knows the requested chunk window before execution starts.

   This makes the row self-describing even on failure and removes the current "window only exists on success" ambiguity.

4. Append state rows from chunk execution, not from pre-planning.

   The orchestrator should still:

   - load the latest state for the connection
   - determine the target window
   - split it into requested chunk windows

   But it should pass the latest loaded state into target-window planning and let that policy decide how to interpret success vs failure. Then it should construct the concrete journal state per chunk, once the exact window is known, and append that row with either:

   - `SucceededAt` populated for success
   - `SucceededAt` left nil and `ErrorSummary` populated for failure

5. Keep planning state and execution state separate.

   The old `prepare next state` shape was confusing because it mixed current-attempt metadata with copied prior-success fields before any concrete chunk existed. The orchestrator should instead:

   - load the latest row
   - pass that loaded state directly into target-window policy
   - plan and split windows
   - create the concrete per-chunk attempt state only during actual execution

   This makes it obvious which seam owns planning interpretation and which seam persists the next attempt row.

6. Prefer explicit latest-attempt semantics over implicit fallback success semantics.

   This change intentionally does not hide failure by automatically returning an older succeeded row under a "load latest succeeded" name. The latest row should stay visible to orchestration and tests as the real current state for the connection.

## Risks / Trade-offs

- [Latest failed rows may be interpreted differently by different target-window policies] -> Accept because the user explicitly wants that logic to live in target-window policy rather than in orchestrator glue code.
- [Renaming window columns requires touching new persistence/tests immediately after the earlier journal change] -> Accept because the feature is still fresh, the repo is early alpha, and backward compatibility is intentionally not a constraint.
- [One state type still carries both attempt metadata and nullable success outcome] -> Accept because the user explicitly prefers one journal/state seam over split attempt/checkpoint types, and the semantics are much clearer once the window is always the attempted window.

## Migration Plan

1. Rename the journal seam and store method to `LoadLastState`.
2. Rename `domain.ProviderSyncState.SuccessfulWindow` to `Window` so the field represents the attempted window directly.
3. Update orchestration tests first so planning receives the latest loaded state directly, and appends one row per attempt.
4. Update persistence model/tests to rename window columns and make them required on every row.
5. Update finance provider sync architecture and OpenSpec language from "succeeded-state journal" to "attempt journal" / "latest state" terminology.

Rollback is removing the unarchived change before implementation, or reverting the resulting implementation commit after it lands.

## Open Questions

- Should the first target-window policy implementation retry from the latest failed row, fall back to the latest succeeded row, or use another rule when `SucceededAt` is nil on the latest loaded state?
