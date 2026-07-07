# Review Chunk 3: connector alignment

## Round 1 - 2026-07-07

- Phase: finalization review
- Scope: connector alignment
- `openspec apply`:
  - attempted `direnv exec /Users/jenya/projects/signal-foundry openspec apply --change repair-enablebanking-client-contract --task 3.1 --task 3.2`
  - result: failed because the installed CLI reports `unknown command 'apply'`
- Implemented/verified in this chunk scope:
  - `finance/internal/enablebanking/connector.go` maps session accounts by `uid` (fallback `id`) and account metadata from typed `accounts_data` entries (`name`, `currency`, `iban`).
  - `finance/internal/enablebanking/connector.go` maps balances using typed `balance_type` semantics:
    - `interimavailable`, `availablebalance`, `expectedavailable`, and `available` to `available_balance_minor`
    - `closingbooked`, `booked`, `current`, `currentbalance` to `current_balance_minor`
    - preserves fallback behavior when one variant is missing.
  - `finance/internal/enablebanking/connector.go` maps transaction amount/sign from `transaction_amount` + `credit_debit_indicator` and maps stable IDs from `entry_reference` first, then `transaction_id`, then a provider fingerprint.
  - `finance/internal/enablebanking/connector.go` iterates transaction pages while honoring `continuation_key` from API responses.
  - Connector raw payload observations continue to capture provider output but are assembled from sanitized typed structures; session redaction for `secret` is enforced by `Enablebankingclient` serialization behavior and explicit assertions.
- Tests added/updated in this scope (and passing):
  - `finance/internal/enablebanking/connector_test.go`
    - session fetch balances/transactions path verifies account metadata, balance selection across types, continuation paging, DBIT/CRDT sign handling, `entry_reference` transaction identity, and time normalization.
    - raw-only-typed-field compatibility test verifies raw-only `accounts_data` (legacy-only) does not produce accounts/transactions when typed models omit those compatibility-only keys.
    - finish-link redaction test verifies raw observation payload excludes `secret`.
- OpenSpec task updates:
  - marked `3.1` complete in `tasks.md`
  - marked `3.2` complete in `tasks.md`
- Checks run:
  - `direnv exec /Users/jenya/projects/signal-foundry go test ./finance/internal/enablebanking ./finance/internal/providers ./finance`
  - `direnv exec /Users/jenya/projects/signal-foundry make affected-lint-test`
- Scope alignment:
  - changed files are confined to `finance/internal/enablebanking` and expected OpenSpec artifacts.
- Obvious regression check:
  - no runtime regression indicators in chunk 3 scope; package tests and full affected lint/test passed.

### Completion protocol

- Result: complete
- Continue decision: proceed to chunk 4
- Completion protocol status: passed
- Artifact cleanup status: clean
- Commit status: pending (no commit requested yet)
- Affected follow-up chunks: `4-manual-e2e-validation`
