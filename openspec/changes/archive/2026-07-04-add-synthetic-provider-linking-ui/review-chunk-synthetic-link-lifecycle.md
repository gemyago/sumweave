# Review Chunk synthetic-link-lifecycle

## Implementation Round 1 — 2026-07-04

- Implementer: openspec-implementation
- Scope: task `3.1` synthetic start/finish lifecycle
- Status: complete

### OpenSpec apply

- Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change add-synthetic-provider-linking-ui --task 3.1`.
- Installed CLI still does not expose `apply` and returned `unknown command 'apply'`.
- Proceeded within the approved chunk scope and used `openspec instructions tasks --change add-synthetic-provider-linking-ui` for task context.

### What changed

- Added failing-first connector tests for synthetic local start/finish behavior and link-coordinator tests for synthetic pending-start provider-reference persistence, PKO `code` validation, and unsupported method failures staying ahead of secret writes.
- Implemented synthetic connector local redirect start/finish support with deterministic state generation, the fixed `#/finance/connections/synthetic?state=...` authorization URL, configured-state validation, and active finish results keyed by the synthetic state/provider reference.
- Updated the link coordinator to persist synthetic pending-start `ProviderReference`, reject PKO redirect finishes without a non-empty `code` before consuming pending starts or writing secrets, and wired the finance service's synthetic connector through the app ID generator.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change add-synthetic-provider-linking-ui --task 3.1` *(fails: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry openspec instructions tasks --change add-synthetic-provider-linking-ui`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/synthetic ./finance/internal/providers`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/...`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec task updates

- Marked `tasks.md` item `3.1` complete.

### Artifact cleanup

- Clean.
- No ad-hoc repository artifacts added.
- Created the standard chunk artifact `review-chunk-synthetic-link-lifecycle.md` referenced by `manager-status.md`.

### Follow-up notes for reviewer

- HTTP/API surface for synthetic pending-state read/write and redirect-finish request validation still belongs to the next chunk `http-openapi-surface`.
- Synthetic finish now depends on persisted configured state under the state/provider-reference key, so manual/API callers must configure pending synthetic accounts before finishing.

## Chunk Finalization Review — 2026-07-04

- Implementer artifact reviewed: `review-chunk-synthetic-link-lifecycle.md` (current branch)
- Chunk target: `synthetic-link-lifecycle`
- Requested task: `3.1` synthetic start/finish lifecycle

### Focus checks

- Confirmed chunk artifacts and scope:
  - `openspec/changes/add-synthetic-provider-linking-ui/tasks.md` (item `3.1` marked complete)
  - `openspec/changes/add-synthetic-provider-linking-ui/manager-status.md` currently marks this chunk as in progress before finalization.
- Confirmed code scope against requested behavior:
  - `finance/finance.go`
  - `finance/internal/providers/link_coordinator.go`
  - `finance/internal/providers/link_coordinator_test.go`
  - `finance/internal/synthetic/connector.go`
  - `finance/internal/synthetic/connector_test.go`
- Verification run:
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/synthetic ./finance/internal/providers`
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/...`
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`
  - `direnv exec /Users/jenya/projects/signal-foundry openspec instructions tasks --change add-synthetic-provider-linking-ui`
  - `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change add-synthetic-provider-linking-ui --task 3.1` *(fails: `unknown command 'apply'`)*
- Artifact cleanup: standard review artifact updated only; no ad-hoc repository files added.

### Findings

- Scope match: ✅ task `3.1` is complete and aligned with requested behavior.
- Safety: ✅ no blocking issues detected in touched flow.
  - synthetic start persists `ProviderReference` so finish can use provider-owned synthetic state.
  - synthetic finish validates configured synthetic accounts before completion.
  - PKO redirect finish enforces non-empty `code` before any secret writes.
  - unsupported finish path test verifies pending start is not consumed when code is missing.
- Completion protocol: ✅ `make affected-lint-test` passes.
- OpenSpec progress:
  - `tasks.md` has item `3.1` complete.
  - `manager-status.md` reflects chunk completion and commit hash.

### Decision

- Verdict: `complete`
- Continue decision: `continue`
- Completion protocol status: `✓ pass`
- Artifact cleanup status: `✓ clean`
- Commit status: `✓ created` (`f8c3015`)
- Follow-up chunk: `http-openapi-surface`
