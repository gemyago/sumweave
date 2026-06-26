# Final Review

## Round 1

- Trigger: user correction on workflow and finance e2e shape
- Exact user quote: `do 1 2 3`
- Exact user quote: `B, "manual" e2e should've been assumed by agent with fix/re-test loop, I assume that was done, if no - plan and execute it`
- Exact user quote: `wtf is @tests/finance-e2e/ that shouldn't exist`
- Verdict: pending correction
- Scope fit: yes, this is a workflow/correctness correction for the finance slice change
- Derived workflow action: remove the dedicated `tests/finance-e2e` module, run/record the manual finance e2e fix-retest loop instead, then create the missing chunk commits and finish whole-change final review
- Notes: user explicitly rejected the dedicated e2e module path and asked for manual e2e assumptions instead

## Round 2

- Trigger: correction implementation and whole-change manual finance e2e rerun
- Verdict: pass
- Scope fit: yes, correction stayed within the finance slice delivery and OpenSpec workflow artifacts
- Changes made:
  - removed `tests/finance-e2e/` and the `go.work` workspace entry instead of keeping a dedicated e2e module
  - removed the now-obsolete `go.work.sum` addition that came from the deleted module
  - updated project rules to prefer manual browser e2e over new dedicated test modules
- Manual finance e2e loop:
  - initial PM2/default-local attempt was blocked by the existing local jobs-store migration failure on the shared local DB and by stale port ownership
  - first isolated rerun exposed that fixture-only `scenario-provider` connections are not safe for real runtime sync smoke, so the loop was corrected to use a local monobank stub plus token-linked monobank data
  - final isolated rerun passed: login, tenant selection, dashboard route, manual account create/detail, CSV preview+confirm, worker-completed finance import job, token link + worker-completed bank sync job, and sanitized admin FX/provider diagnostics
- Verification:
  - `make affected-lint-test` passed
  - manual browser smoke passed on the corrected isolated stack
- Commit status: pending manager commit gate for the corrected chunk/final-review artifacts

## Round 3

- Trigger: missing chunk-commit reconciliation after the manual e2e correction
- Verdict: pass
- Scope fit: yes, this is still within the finance slice correction workflow and OpenSpec bookkeeping
- Changes made:
  - committed the finance imports/api chunk (`119a215`)
  - committed the finance ui diagnostics chunk (`5bd1633`)
  - committed the finance e2e workflow correction (`45e50c8`)
  - committed the missed jobs route refinement (`e8eee26`)
- Verification:
  - `git status --short` is now clean at the repo root
- Commit status: complete

## Round 4

- Trigger: user review comments on remaining code hygiene and architecture
- Exact user quote: `are you sure v1routes are generated but not created manually?`
- Exact user quote: `posted bunch of comments, I think it worth splitting them on a couple of chunks and consider this work as new chunks...`
- Exact user quote: `this is very bad pattern, we should always load config and use config but not env vars directly`
- Exact user quote: `wft coverage ignore is doing here and in many other places like this, don't we have a clear way of mock and test this?`
- Exact user quote: `fuck dude, use mockery, what are you doing...?`
- Exact user quote: `already commented, there must be no reason to skip coverage.`
- Exact user quote: `what a fuck this entire migrations thing is? our project was not using any migrations framework and this is what? building our own? drop this fucking shit and use auto-migrate as other storages, this solution is not portable. If we would have to add migrations - we will have to pick a proper framework for this.`
- Exact user quote: `Have you heard of a single responsibility principle?`
- Verdict: pending correction
- Scope fit: yes, these are follow-up implementation comments for the finance slice
- Derived workflow action: split the comments into new follow-up chunks, confirm generated-vs-manual route artifacts, remove env-var and coverage-ignore patterns, replace manual mocks with mockery, and resolve the persistence/migration and service-SRP concerns in separate work units

## Round 5

- Trigger: user correction on generated routes and migration policy
- Exact user quote: `do you think I'm fucking stupid or what? I just did "go generate ./apps/signal-foundry/..." and try to see a git diff...`
- Exact user quote: `some agent added that comment on migrations, change the rule to auto-migrate`
- Verdict: pending correction
- Scope fit: yes, this is still a follow-up correction on the finance slice implementation
- Derived workflow action: treat `v1routes` as generated output, stop claiming the finance additions were hand-authored, and carry the migration-policy change into the next follow-up plan

## Round 6

- Trigger: follow-up cleanup set finalization review for chunks 9.1, 9.2, and 9.3
- Verdict: pending correction
- Scope fit: yes, this review stays within the post-review cleanup follow-up for generated routes/config wiring, testability cleanup, and persistence/SRP cleanup
- Findings:
  - 9.1 config-backed finance fixture wiring is directionally correct, and targeted `go generate ./internal/api/http` confirms the finance/jobs route artifacts are generated output
  - 9.1 is still blocking because the generated validators for finance CSV map fields do not compile (`finance_csv_import_confirm_request_validation.go` and `finance_csv_import_preview_response_validation.go` instantiate `EnsureNonDefault[map[string]string]` even though `EnsureNonDefault` requires a `comparable` type)
  - 9.2 mockery adoption for controller tests is correct, but the broader coverage-ignore cleanup is incomplete because finance package exclusions still remain in `finance/imports.go`, `finance/persistence/store.go`, and `finance/fixtures/realistic.go`
  - 9.3 service SRP refactor is directionally good and `go test ./finance/...` passes, but the auto-migrate replacement is not yet schema-clean because key model tags do not preserve earlier migration guarantees such as invite-code uniqueness and several prior composite index shapes
- Verification:
  - `go generate ./internal/api/http` passed
  - `go test ./finance/...` passed
  - `go test ./cmd/signal-foundry ./internal/api/http/... ./internal/config/...` failed on the generated validator compile errors above
- Commit status: no commit, as requested; the follow-up set is not review-clean and is not safe to continue past yet

## Round 7

- Trigger: latest re-review of follow-up cleanup set chunks 9.1, 9.2, and 9.3 after the newest fixes
- Verdict: pending correction
- Scope fit: yes, this is still the same finance follow-up cleanup set review
- Findings:
  - 9.1 remains blocking: rerunning `go generate ./internal/api/http` still produces finance CSV validators that call `EnsureNonDefault[map[string]string]`, while the generated helper in `internal/api/http/v1routes/internal/validators.go` still constrains `EnsureNonDefault` to `comparable`; focused app-package tests still fail on that mismatch
  - 9.1 config-based finance fixture/runtime wiring still looks correct, but the generated route artifact set is not yet self-consistent or build-clean
  - 9.2's previous blocker appears resolved: the finance-package `coverage-ignore` exclusions called out in the prior round are gone from `finance/imports.go`, `finance/persistence/store.go`, and `finance/fixtures/realistic.go`, and the mockery-based controller test setup remains in place
  - 9.3's previous blocker appears resolved: finance auto-migrate models now carry the invite-code uniqueness and prior composite index tags, and `finance/persistence/migrations_test.go` now locks those shapes in with regression coverage
- Verification:
  - `go generate ./internal/api/http` changed the app route/helper worktree state (it dropped the local `validators.go` tweak and regenerated the finance route set)
  - `go test ./cmd/signal-foundry ./internal/api/http/... ./internal/config/...` failed on generated-validator compile errors in `internal/api/http/v1routes/internal/finance_csv_import_confirm_request_validation.go` and `internal/api/http/v1routes/internal/finance_csv_import_preview_response_validation.go`
  - after restoring the pre-generate `validators.go` helper tweak, `go test ./cmd/signal-foundry ./internal/api/http/... ./internal/config/...` passed again, so the remaining 9.1 blocker is specifically generator/source durability rather than the current local tree
  - `go test ./finance/...` passed
  - repo grep confirms there are no remaining `coverage-ignore` matches under `finance/**/*.go`
- Commit status: no commit, as requested; the follow-up set is still not review-clean and is not safe to continue past yet, although 9.2 and 9.3 now look clean

## Round 8

- Trigger: durable generator fix re-review for follow-up cleanup chunks 9.1, 9.2, and 9.3
- Verdict: pass
- Scope fit: yes, this stays within the finance follow-up cleanup set and specifically re-checks the generated route durability blocker plus the already-cleared 9.2/9.3 fixes
- Findings:
  - 9.1's blocking durability issue is now resolved: `register.go` runs `go run ./apigenpatch` as part of `go generate`, `apigenpatch/main_test.go` covers the patch flow and drift cases, and rerunning generation kept the finance CSV validator files and `validators.go` byte-identical with no manual restore step
  - 9.2 remains fixed: finance-package `coverage-ignore` exclusions are still absent, the mockery-generated controller mocks remain in place, and the focused app HTTP/controller test suite still passes
  - 9.3 remains fixed: finance persistence still uses GORM auto-migrate, the schema tags preserve the reviewed uniqueness/composite-index guarantees, and the focused finance regression suite still passes
- Verification:
  - `go generate ./internal/api/http` passed and preserved identical SHA-256 hashes for `internal/api/http/v1routes/internal/validators.go`, `finance_csv_import_confirm_request_validation.go`, and `finance_csv_import_preview_response_validation.go` before vs after regeneration
  - `go test ./cmd/signal-foundry ./internal/api/http/... ./internal/config/...` passed after regeneration
  - `go test ./...` passed under `finance/`
  - repo grep still finds no `coverage-ignore` matches under `finance/**/*.go`
- Commit status: no commit, as requested; this follow-up set is now review-clean and safe to continue past, while archive/clean-status submission gates remain pending outside this review step
