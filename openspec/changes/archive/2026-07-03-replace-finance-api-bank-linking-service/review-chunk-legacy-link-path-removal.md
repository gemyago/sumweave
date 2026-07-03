# Legacy Link Path Removal Review

## Round 1

- Phase: initial implementation phase
- Scope: section 3 `legacy-link-path-removal`
- OpenSpec apply: ran `openspec instructions apply --change replace-finance-api-bank-linking-service` before implementation work.
- TDD:
  - Updated finance service tests first so they no longer assert root `finance.Service` owns redirect-link or pending-start callback lookup behavior, and shifted remaining coverage onto direct sync/setup helpers or provider-level tests.
  - Re-ran focused finance and app tests after the test cleanup before removing root redirect-link methods and legacy pending-start helpers.
- Implementation:
  - Removed the unused root-service redirect start, redirect finish, and pending-start lookup path from `finance/provider_sync.go`, plus the now-dead pending-start persistence/restore helpers that only served the old API/callback route.
  - Kept unrelated root finance sync/list/delete behavior compiling and preserved fixture-oriented token-link setup coverage while restoring provider/common coverage with direct Enable Banking provider tests.
- Files changed:
  - `finance/provider_sync.go`
  - `finance/provider_sync_internal_test.go`
  - `finance/provider_sync_test.go`
  - `finance/providers_common_test.go`
  - `openspec/changes/replace-finance-api-bank-linking-service/tasks.md`
  - `openspec/changes/replace-finance-api-bank-linking-service/review-chunk-legacy-link-path-removal.md`
- Checks run:
  - `go test ./finance/...` from repo root
  - `go test ./internal/financeapp ./internal/api/http ./internal/api/http/v1controllers` from `apps/signal-foundry/`
  - `make lint` from `finance/`
  - `make test` from `finance/`
  - `make affected-lint-test` from repo root
- AGENTS.md impact: no changes needed; no commands, workflow, or architecture guidance changed.
- OpenSpec task updates:
  - `openspec/changes/replace-finance-api-bank-linking-service/tasks.md` now marks `3.1` complete.
- Artifact cleanup: clean; no ad-hoc repo artifacts added.
- Commit status: no commit created.
- Safe-to-continue: yes; the protected API/callback legacy redirect path is removed and remaining bank-link code outside that path still compiles.

## Round 2 (Finalization)

- Result: complete
- Requested scope verification: aligned with section 3 `legacy-link-path-removal`; changes are limited to removing legacy root-service token/redirect/pending-start flow ownership and retaining non-linking sync behavior.
- Safety check: no obvious regressions observed in finance sync or service internals from this chunk; token-link coverage is now expressed through direct provider-link setup and shared helper tests.
- Completion protocol:
  - `go test ./finance/...` from repo root passed.
  - `make affected-lint-test` from repo root passed.
- OpenSpec/implementation status:
  - `openspec instructions apply --change replace-finance-api-bank-linking-service` is recorded in this review file and implementation notes.
  - `openspec/changes/replace-finance-api-bank-linking-service/tasks.md` marks `3.1` complete.
- Artifact cleanup:
  - `git status` scoped to chunk artifacts shows only intended code files and OpenSpec artifacts.
  - No ad-hoc repository artifacts were introduced by this chunk.
- Commit status: committed (`4aeddf6`).
- Continue decision: safe to continue.
- Follow-up needed: none.
