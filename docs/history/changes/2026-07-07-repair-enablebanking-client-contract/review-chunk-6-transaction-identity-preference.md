---
type: Historical record
title: "2026-07-07-repair-enablebanking-client-contract: review chunk 6 transaction identity preference"
description: "Historical review chunk 6 transaction identity preference record for 2026-07-07-repair-enablebanking-client-contract."
status: deprecated
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-07-repair-enablebanking-client-contract/review-chunk-6-transaction-identity-preference.md"
    title: "Original openspec/changes/archive/2026-07-07-repair-enablebanking-client-contract/review-chunk-6-transaction-identity-preference.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

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

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
