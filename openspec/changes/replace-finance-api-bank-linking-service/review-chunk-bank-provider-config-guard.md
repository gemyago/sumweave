# Bank Provider Config Guard Review

## Round 1

- Phase: fixing phase
- Scope: final-review follow-up for unconfigured bank provider guarding in the focused bank-connection service and app DI path.
- OpenSpec apply:
  - Attempted `openspec apply --change replace-finance-api-bank-linking-service`, but the installed CLI does not expose `apply`.
  - Ran `openspec instructions apply --change replace-finance-api-bank-linking-service` before implementation work.
- TDD:
  - Added failing focused regressions first in `finance/bank_connection_service_test.go` and `apps/signal-foundry/internal/financeapp/register_test.go` asserting Monobank and PKO/Enable Banking return `ErrBankProviderNotConfigured` when the new bank-connection DI path is unconfigured.
  - Confirmed the failing state with `go test ./finance ./apps/signal-foundry/internal/financeapp` from the repo root before implementation.
- Implementation:
  - Added a minimal provider-config guard to `finance.BankConnectionService` so token and redirect link entrypoints reject unconfigured providers before coordinator or live connector use.
  - Kept the public bank-link method contract unchanged.
  - Tightened app DI helper shaping so Enable Banking config stays zero-valued when not actually configured.
- Files changed:
  - `finance/bank_connection_service.go`
  - `finance/bank_connection_service_test.go`
  - `apps/signal-foundry/internal/financeapp/register.go`
  - `apps/signal-foundry/internal/financeapp/register_test.go`
  - `openspec/changes/replace-finance-api-bank-linking-service/review-chunk-bank-provider-config-guard.md`
- Checks run:
  - `go test ./finance ./apps/signal-foundry/internal/financeapp` from repo root (failed first, then passed after implementation)
  - `gofmt -w finance/bank_connection_service.go finance/bank_connection_service_test.go apps/signal-foundry/internal/financeapp/register.go apps/signal-foundry/internal/financeapp/register_test.go`
  - `go test ./finance ./apps/signal-foundry/internal/financeapp && make affected-lint-test` from repo root
- OpenSpec task status updates made:
  - none; the existing completed tasks already cover this scoped regression fix, so no new follow-up task item was needed.
- AGENTS.md impact: no changes needed; no commands, workflow, or architecture guidance changed.
- Artifact cleanup: clean; only code files, this standard durable review artifact, and the pre-existing standard `review-final.md` artifact are pending.
- Commit status: no commit created.
- Safe-to-continue: yes; the blocking final-review regression is covered by focused tests and the repo completion checks pass.

## Round 2 (Finalization)

- Result: complete
- Requested scope verification: aligned with the follow-up fix scope for guarding unconfigured providers before coordinator calls.
- Safety check: Monobank token-link and PKO redirect link start/finish now fail fast with `ErrBankProviderNotConfigured` when DI is unconfigured, preserving previous contract behavior.
- Completion protocol checks:
  - `go test ./finance ./apps/signal-foundry/internal/financeapp` passed
  - `make affected-lint-test` passed
  - `gofmt` formatting checks pass (applied by implementation pipeline)
- OpenSpec/task status:
  - `openspec instructions apply --change replace-finance-api-bank-linking-service` is recorded as used.
  - `finance/bank_connection_service.go` and `apps/signal-foundry/internal/financeapp/register.go` now gate provider behavior by configured providers.
  - `finance/bank_connection_service_test.go` and `apps/signal-foundry/internal/financeapp/register_test.go` cover the regression (`ErrBankProviderNotConfigured`).
- Artifact cleanup:
  - `git status` scoped to this chunk’s changed files shows only intended code updates plus this durable review artifact and the pre-existing `review-final.md` blocker artifact.
- Commit status: committed.
- Safe-to-continue: yes.
- Follow-up needed: none.
