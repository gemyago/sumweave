# Review Chunk: service-go-file-decomposition

## Implementation round 2026-07-03

- Result: complete
- Phase: initial implementation phase

### What changed

- Attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change split-finance-root-service --task service-go-file-decomposition`, but the installed CLI still reports `unknown command 'apply'`, so the approved chunk was implemented directly.
- Added `finance/public_declarations_test.go` and strengthened `finance/root_service_caller_audit_test.go` so the chunk starts with compile-backed public declaration coverage plus a failing audit that required `finance/service.go` removal.
- Moved tenant params/default seeds into `finance/service_tenant_contract.go`, catalog params/not-found errors into `finance/service_catalog_contract.go`, ledger params/transfer helpers into `finance/service_ledger_contract.go`, tenant-access denial into `finance/access_guard.go`, and CSV import sentinel errors into `finance/service_csv_import.go`.
- Deleted the redundant declaration bucket `finance/service.go` without changing exported names or package behavior.

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change split-finance-root-service --task service-go-file-decomposition` *(fails: `unknown command 'apply'`)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run 'TestRootServiceCallerAudit|TestPublicDeclarationsRemainAvailable'` *(fails before implementation because `finance/service.go` still exists)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run 'TestRootServiceCallerAudit|TestPublicDeclarationsRemainAvailable|TestFocusedCoreServices'`
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance`
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`

### OpenSpec task updates

- Marked `tasks.md` item `5.1` complete.
- Marked chunk `service-go-file-decomposition` complete in `manager-status.md`.

### Artifact cleanup

- Removed the obsolete repository file `finance/service.go`.
- No ad-hoc repository artifacts were added.

### Notes for next reviewer

- Exported param types and sentinel errors keep their original package names; only file placement changed.
- `finance/public_declarations_test.go` is the durable compile-backed guard for future declaration moves, while the caller audit now also rejects the return of `finance/service.go`.

## Finalization round 2026-07-03

- Result: complete
- Verdict summary: chunk `service-go-file-decomposition` cleanly removes `finance/service.go` and keeps behavior/contracts stable by relocating declarations into focused files plus focused-file tests/audit checks.
- Continue decision: safe to continue
- Completion protocol status: pass
  - Read `finance/AGENTS.md`, `.agents/prompts/openspec-manager/agent-chunk-finalizing.md`, and `.agents/prompts/openspec-manager/shared-rules.md` before review
  - Confirmed applicable checks: `go test ./finance` and `make affected-lint-test` both pass after this chunk
- Artifact cleanup status: clean (no disallowed ad-hoc repository artifacts)
- Commit status: pending (no new chunk SHA recorded yet in `manager-status.md`)
- Affected follow-up chunks: none

### Checks run

- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance -run 'TestRootServiceCallerAudit|TestPublicDeclarationsRemainAvailable|TestFocusedCoreServices'` *(pass)*
- `direnv exec /Users/jenya/projects/signal-foundry go test ./finance` *(pass)*
- `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test` *(pass)*
- `git status --short -- openspec/changes/split-finance-root-service/manager-status.md openspec/changes/split-finance-root-service/review-chunk-service-go-file-decomposition.md openspec/changes/split-finance-root-service/tasks.md openspec/changes/split-finance-root-service/review-planning.md openspec/changes/split-finance-root-service/review-final.md` *(shows only standard OpenSpec/status files modified/untracked)*

### OpenSpec compliance checks

- `openspec apply` could not be used: installed CLI still reports `unknown command 'apply'`, so this chunk was implemented directly and documented in this file.
- Task `5.1` remains marked complete in `tasks.md`.
- `manager-status.md` now records this chunk as complete with chunk ledger status updated.
