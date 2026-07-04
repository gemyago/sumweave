# Review Chunk pending-synthetic-config

## Implementation Round 1 — 2026-07-04

- Implementer: openspec-implementation
- Scope: task `2.1` pending synthetic configuration service and persistence
- Status: complete

### OpenSpec apply

- Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change add-synthetic-provider-linking-ui --task 2.1`.
- Installed CLI still does not expose `apply` and returned `unknown command 'apply'`.
- Proceeded within the approved chunk scope and used `openspec instructions tasks --change add-synthetic-provider-linking-ui` for task context.

### What changed

- Added a dedicated persistence store for authorized synthetic pending-start lookup so tenant/actor/provider/state checks stay out of the legacy broad store.
- Added a focused `SyntheticLinkStateService` that refreshes pending synthetic config, validates account input, preserves stable synthetic account keys on resave, and persists provider-owned state by provider reference.
- Wired the new service into `finance.Finance` and added focused tests covering pending-state authorization, refresh, stable key assignment, and duplicate same-name/same-currency accounts remaining distinct after save/reload.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change add-synthetic-provider-linking-ui --task 2.1` *(fails: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry openspec instructions tasks --change add-synthetic-provider-linking-ui`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/...`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec task updates

- Marked `tasks.md` item `2.1` complete.

### Artifact cleanup

- Clean.
- No ad-hoc repository artifacts added.
- Created the standard chunk artifact `review-chunk-pending-synthetic-config.md` referenced by `manager-status.md`.

### Follow-up notes for reviewer

- The service currently allows saving zero configured accounts and reports `canFinish=false`; the later finish-flow chunk still owns enforcing non-empty configured state at link completion time.
- Pending synthetic config uses `pendingStart.ProviderReference` when present and otherwise falls back to the pending `state`, so this chunk works before the later synthetic start/finish lifecycle chunk tightens provider-reference assignment on start.

## Chunk Finalization Review — 2026-07-04

- Implementer artifact reviewed: `review-chunk-pending-synthetic-config.md`
- Scope under review: `pending-synthetic-config`
- Requested task: `2.1` pending synthetic configuration service and persistence

### Focus checks

- Confirmed implementation matches requested task in files:
  - `finance/synthetic_link_state_service.go`
  - `finance/persistence/synthetic_pending_start_store.go`
  - `finance/persistence/synthetic_pending_start_store_test.go`
  - `finance/synthetic_link_state_service_test.go`
  - `finance/finance.go`
- Verified OpenSpec progress artifacts:
  - `openspec/changes/add-synthetic-provider-linking-ui/tasks.md`
  - `openspec/changes/add-synthetic-provider-linking-ui/manager-status.md`
- Ran required verification:
  - `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change add-synthetic-provider-linking-ui --task 2.1` *(fails: `unknown command 'apply'`)*
  - `direnv exec /Users/jenya/projects/signal-foundry openspec instructions tasks --change add-synthetic-provider-linking-ui`
  - `make affected-lint-test`

### Findings

- Scope match: ✅ task `2.1` is fully implemented and aligned with chunk request.
- Safety / obvious issues: ✅ no blocking runtime issues seen in touched flow; authorization and synthetic pending-start filtering are correctly scoped to tenant/member/provider/state.
- Completion protocol: ✅ `make affected-lint-test` succeeds.
- OpenSpec task progress:
  - `tasks.md` marks `2.1` complete.
- Artifact cleanup: ✅ no ad-hoc repository artifacts added.

### Decision

- Verdict: `complete`
- Continue decision: `continue`
- Completion protocol status: `✓ pass`
- Artifact cleanup status: `✓ clean`
- Commit status: `✓ created` (`b0b12d8`)
- Follow-up chunk: `synthetic-link-lifecycle`
