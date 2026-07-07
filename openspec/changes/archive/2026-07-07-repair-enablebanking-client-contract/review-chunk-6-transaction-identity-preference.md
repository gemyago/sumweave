# Review Chunk 6: transaction identity preference

## Round 1 - 2026-07-07

- Phase: fixing phase
- Scope: prefer `entry_reference` before `transaction_id` in connector transaction identity only
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 5.2`
  - result: failed because the installed CLI reports `unknown command 'apply'`
- Implemented in this chunk scope:
  - updated Enable Banking connector transaction identity selection to prefer normalized `entry_reference` before `transaction_id`
  - added focused connector coverage for a transactions response that contains both fields and now keeps `entry_reference` as `ProviderTransactionID`
- OpenSpec task updates:
  - marked `5.2` complete in `tasks.md`
- Checks run:
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/enablebanking`
  - `direnv exec /Users/jenya/projects/signal-foundry golangci-lint run ./finance/internal/enablebanking`
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`
- Artifact cleanup status:
  - no ad-hoc repository artifacts added
- Blockers:
  - no product blockers in chunk 6 scope
  - tooling note only: `openspec apply` is unavailable in the installed CLI (`unknown command 'apply'`)
- Result: complete

## Round 2 - 2026-07-07 (finalization pass)

- Scope: re-verify transaction identity preference only.
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 5.2`
  - still blocked: installed CLI reports `unknown command 'apply'`.
- Verification against code:
  - `finance/internal/enablebanking/client/normalizeTransactions` now sets `transaction.ID = transaction.EntryReference` when missing, and
  - `finance/internal/enablebanking/connector.go` resolves `transactionID` via `firstNonEmpty(transaction.ID, transaction.TransactionID, fallback)`.
- Focused coverage verified:
  - `finance/internal/enablebanking/connector_test.go` transaction paging fixture now includes both `entry_reference` and `transaction_id` and asserts `ProviderTransactionID` equals the entry reference.
- Checks rerun:
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/enablebanking`
  - `direnv exec /Users/jenya/projects/signal-foundry golangci-lint run ./finance/internal/enablebanking ./finance/internal/enablebanking/client`
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`
  - all passed
- `tasks.md`: chunk task `5.2` is marked complete.
- Artifact cleanup status:
  - no ad-hoc repository artifacts added in this round.
- Result: complete
