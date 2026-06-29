# Planning Review

## Round 1

- Scope: implement-bank-provider-v2-connectors planning artifacts
- Trigger: initial review of proposal/design/tasks
- Verdict: needs changes
- Findings:
  - [high] `finance/AGENTS.md / Boundary`; `design.md / Goals / Decisions`; `tasks.md / 1.*, 2.*` — the plan does not pin how Monobank tokens / Enable Banking credentials are handled so they never end up persisted as plaintext. Since linking is in scope, this repo-level security constraint needs an explicit design/task requirement before implementation.
  - [medium] `design.md / Decisions 6`; `tasks.md / 3.1` — the design says PKO must remain a product provider composed through technical connector `enable-banking`, and `enable-banking` must not become a user-facing provider choice, but no task explicitly verifies that profile/composition behavior stays unchanged.
  - [medium] `design.md / Risks / Trade-offs` (Enable Banking branches); `tasks.md / 2.1-2.2` — the plan says only “supported behavior branches needed today” should be ported for Enable Banking, but it never names those exact branches/modes. That leaves implementation scope ambiguous.
- Required follow-ups:
  - Add an explicit requirement for secret-bearing link results/state to use the existing secure/encrypted handling path and never introduce plaintext persistence.
  - Add task coverage proving PKO still resolves through `PKOProfile()`/`enable-banking` without exposing `enable-banking` as a product provider.
  - Name the exact Enable Banking auth/fetch branches that are in scope for v2, and which branches must return unsupported errors.
- Completion protocol status: not sufficient yet for implementation review to begin.
- Artifact cleanup status: no obvious stray logs/files detected.
- Commit status: pending changes remain (`openspec/changes/implement-bank-provider-v2-connectors/` untracked).

## Round 2

- Scope: revised planning artifacts
- Trigger: re-review after plan revision
- Verdict: clean
- Findings: none; earlier findings appear addressed.
- Suggested implementation chunk order:
  1. `tasks.md` 1.1
  2. `tasks.md` 1.2
  3. `tasks.md` 2.1
  4. `tasks.md` 2.2
  5. `tasks.md` 3.1
- Completion protocol status: sufficient for implementation to begin.
- Artifact cleanup status: planning artifact set looks clean.
- Commit status: pending.
