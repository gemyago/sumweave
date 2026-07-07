# Review Chunk 5: remove unsupported ListAccounts

## Round 1 - 2026-07-07

- Phase: fixing phase
- Scope: remove unsupported ListAccounts client surface only
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 5.1`
  - result: failed because the installed CLI reports `unknown command 'apply'`
- Implemented in this chunk scope:
  - deleted `finance/internal/enablebanking/client/list_accounts.go`
  - deleted `finance/internal/enablebanking/client/list_accounts_test.go`
  - removed `ListAccountsResponse` from `finance/internal/enablebanking/client/model_account.go` because it was only used by the unsupported surface
- OpenSpec task updates:
  - marked `5.1` complete in `tasks.md`
- Checks run:
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/enablebanking/client ./finance/internal/enablebanking`
  - `direnv exec /Users/jenya/projects/signal-foundry golangci-lint run ./finance/internal/enablebanking/client ./finance/internal/enablebanking`
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`
- Artifact cleanup status:
  - no ad-hoc repository artifacts added
- Blockers:
  - no product blockers in chunk 5 scope
  - tooling note only: `openspec apply` is unavailable in the installed CLI (`unknown command 'apply'`)
- Result: complete

## Round 2 - 2026-07-07

- Phase: fixing phase
- Scope: endpoint manifest cleanup for the removed unsupported `ListAccounts` surface only
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 5.1`
  - result: failed because the installed CLI reports `unknown command 'apply'`
- Implemented in this chunk scope:
  - removed the stale `GET /accounts` and `ListAccounts(ctx, ListAccountsParams)` entries from `finance/internal/enablebanking/client/ENDPOINTS.md`
- OpenSpec task updates:
  - no task checkbox changes needed; `tasks.md` already marked `5.1` complete and this follow-up finishes the remaining manifest cleanup for that task
- Checks run:
  - attempted `rg -n "GET /accounts|ListAccounts" finance/internal/enablebanking/client/ENDPOINTS.md` from the shell, but `rg` is not installed in this environment
  - verified with a focused content search for `^GET /accounts$|^Client method: ListAccounts\(` in `finance/internal/enablebanking/client/ENDPOINTS.md`, with no matches after cleanup
- Artifact cleanup status:
  - no ad-hoc repository artifacts added
- Blockers:
  - no product blockers in chunk 5 scope
  - tooling note only: `openspec apply` is unavailable in the installed CLI (`unknown command 'apply'`)
- Result: complete

## Round 3 - 2026-07-07

- Phase: follow-up verification
- Scope: confirm `finance/internal/enablebanking/client/ENDPOINTS.md` no longer advertises unsupported `GET /accounts` and `ListAccounts`
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 5.1`
  - result: blocked; the installed CLI reports `unknown command 'apply'`
- Verification run:
  - `finance/internal/enablebanking/client/ENDPOINTS.md` now includes only:
    - `GET /accounts/{accountId}/details`
    - `GET /accounts/{accountId}/balances`
    - `GET /accounts/{accountId}/transactions`
  - exact stale checks: no matches for `^GET /accounts$` and no matches for `ListAccounts\(` in this file
- OpenSpec task updates:
  - no task checkbox changes needed; task `5.1` remains complete and this scope finishes the remaining manifest follow-up
- Artifact cleanup status:
  - no ad-hoc repository artifacts added in this round; remaining worktree changes are standard review/change-artifact edits
- Blockers:
  - no product blockers in chunk 5 scope
  - tooling note only: `openspec apply` unavailable in the installed CLI
- Result: complete
