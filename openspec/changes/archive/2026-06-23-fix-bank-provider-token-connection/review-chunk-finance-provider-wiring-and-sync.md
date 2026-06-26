# Chunk Review: finance-provider-wiring-and-sync

## Round 1

- Scope: task 1.3 / provider wiring and sync
- Triggering input: implementation not yet started
- Verdict: pending
- Notes: awaiting implementation sub-agent

## Round 2

- Scope: task 1.3 / provider wiring and sync
- Verdict: needs changes
- Findings:
  - Task intent is satisfied in code/tests: `finance/provider_sync.go` now resolves sync provider for
    `pko` through `enable-banking` and keeps `monobank` on token behavior; app DI wiring and app-level
    test verify both configured providers can be linked and sync through job-backed flow.
  - Out-of-scope/churn item: `openspec/changes/fix-bank-provider-token-connection/manager-status.md`
    was modified by implementation (`in progress` / `running`). This file is outside the reported
    chunk scope and should be reverted or explicitly justified before gate.
- Artifact cleanup status: acceptable from code/test changes (no temporary build artifacts), but
  pending cleanup of the out-of-scope manager-status touch.
- Completion protocol status: ✓ completed here (`make affected-lint-test` passes with no lint/test issues).
- Commit status: no commit yet for this chunk.
- Commit required before gate: yes, a chunk commit is still required.
- Safe to continue to next chunk: not yet until out-of-scope manager-status change is removed/justified.

## Round 3

- Scope: task 1.3 / provider wiring and sync
- Verdict: clean
- Findings: none
- Check 1 (PKO sync route + monobank token route): `finance/provider_sync.go` now resolves sync via `bankProviderForSync`, routing
  `monobank` through `bankProviderForLink(..., bankLinkMethodToken)` and `pko` through
  `bankProviderForLink(..., bankLinkMethodRedirect)`, which maps to `enable-banking`. This preserves monobank token-backed behavior and ensures PKO uses Enable Banking for sync.
- Check 2 (configured URLs + product-level choices): `apps/signal-foundry/internal/financeapp/register_test.go` now creates both monobank and enable providers in DI when `MonoURL`/`EnableURL` are set, then links one monobank and one PKO connection and verifies persisted providers are `monobank` and `pko`.
- Check 3 (job-backed sync path): the same test runs `TriggerBankConnectionSync` for both connections, validates persisted job input from `jobsStore`, and calls `RunBankConnectionSync`, confirming both pass through job-backed scheduling path for execution.
- Out-of-scope changes: none observed in current diff beyond task files (`finance/provider_sync.go`, `finance/provider_sync_internal_test.go`, `apps/signal-foundry/internal/financeapp/register_test.go`, and `tasks.md`). No `manager-status.md` touch remains.
- Artifact cleanup status: acceptable. Only source/test/docs edits present; no extra files or generated artifacts were added.
- Completion protocol status: ✓ `make affected-lint-test` runs clean (all targeted modules in cache), so lint/test protocol appears satisfied.
- Commit status: no commit exists yet for this chunk in current working tree.
- Commit required before gate: yes, chunk-level commit should be created before OpenSpec gate.
- Safe to continue to next chunk: yes, functionally clean after scope cleanup.
