# API And App Wiring Cutover Review

## Round 1

- Phase: initial implementation phase
- Scope: section 2 `api-and-app-wiring-cutover`
- OpenSpec apply: ran `openspec instructions apply --change replace-finance-api-bank-linking-service` before implementation work.
- TDD:
  - Updated controller route tests first to fail on the new split dependency shape, then prove token-link and redirect start/finish routes still keep their request and response contracts while invoking a dedicated bank-connection dependency instead of the root finance service mock.
  - Updated app wiring tests first to fail until DI exposed `*finance.BankConnectionService`, finance controller registration consumed it, and the Enable Banking callback route resolved pending starts through that focused service.
  - Confirmed the initial failing state with `go test ./internal/api/http/v1controllers ./internal/api/http ./internal/financeapp` from `apps/signal-foundry/` before implementation.
- Implementation:
  - Split finance controller bank-link calls onto a focused `bankConnectionService` dependency while leaving the rest of the controller on the root finance service.
  - Wired app DI to build and provide a real `*finance.BankConnectionService` from app config and secret-cipher inputs.
  - Updated route registration and controller DI registration so the Enable Banking callback bridge and finance HTTP handlers both use the focused bank-connection service.
  - Regenerated controller mocks with mockery for the new consumer-side interface.
- Files changed:
  - `apps/signal-foundry/.mockery.yaml`
  - `apps/signal-foundry/internal/api/http/register.go`
  - `apps/signal-foundry/internal/api/http/register_test.go`
  - `apps/signal-foundry/internal/api/http/v1controllers/finance.go`
  - `apps/signal-foundry/internal/api/http/v1controllers/finance_test.go`
  - `apps/signal-foundry/internal/api/http/v1controllers/mocks_test.go`
  - `apps/signal-foundry/internal/api/http/v1controllers/register.go`
  - `apps/signal-foundry/internal/financeapp/register.go`
  - `apps/signal-foundry/internal/financeapp/register_test.go`
  - `openspec/changes/replace-finance-api-bank-linking-service/tasks.md`
  - `openspec/changes/replace-finance-api-bank-linking-service/review-chunk-api-and-app-wiring-cutover.md`
- Checks run:
  - `go test ./internal/api/http/v1controllers ./internal/api/http ./internal/financeapp` from `apps/signal-foundry/` (failed first, then passed after implementation)
  - `make lint` from `apps/signal-foundry/`
  - `go test ./cmd/signal-foundry ./internal/...` from `apps/signal-foundry/`
  - `make affected-lint-test` from repo root
- AGENTS.md impact: no changes needed; no commands, workflow, or architecture guidance changed.
- Artifact cleanup: clean; no ad-hoc repo artifacts added, and only standard/generated mockery output changed.
- Commit status: no commit created.
- Safe-to-continue: yes; section 2 is complete and the next chunk can proceed to legacy link path removal.

## Round 2 (Finalization)

- Result: complete
- Requested scope verification: aligned with section 2 `api-and-app-wiring-cutover` and the change list in `tasks.md`.
- Safety check: no obvious regressions from this chunk; bank-link token/redirect routes preserve request/response contracts while using the focused bank-connection service dependency.
- Completion protocol:
  - `go test ./internal/api/http/v1controllers ./internal/api/http ./internal/financeapp` from `apps/signal-foundry/` (passed)
  - `make lint` from `apps/signal-foundry/` (passed)
  - `go test ./cmd/signal-foundry ./internal/...` from `apps/signal-foundry/` (passed)
  - `make affected-lint-test` from repo root (passed)
- OpenSpec/implementation status:
  - OpenSpec apply recorded in this review file (`openspec instructions apply --change replace-finance-api-bank-linking-service` noted).
  - `openspec/changes/replace-finance-api-bank-linking-service/tasks.md` now marks `2.1` and `2.2` complete.
  - `openspec/changes/replace-finance-api-bank-linking-service/manager-status.md` marks section 2 complete with commit `4df272b`.
- Artifact cleanup: `git status` scoped to chunk artifacts is limited to OpenSpec standard files and implemented code files; no ad-hoc repo artifacts found.
- Commit status: committed.
- Safe-to-continue decision: yes; section 2 is ready and section 3 can proceed.
- Follow-up needed: none.
