# Transfer Matching Phase 0 — system design

Status: draft for review, revised on 2026-09-07. This design follows the
[Phase 0 PRD](transfer-matching-phase0-prd.md), including the review decision to
match in memory and accept concurrent-update races in Phase 0.
[Architecture](../ARCHITECTURE.md) remains authoritative.

## Design summary

Add a finance transfer-matching service that processes a tenant and ledger
timestamp range. Both explicit requests and committed bank-sync windows call
the same service. Automatically pair only booked, equal-and-opposite,
same-currency transactions within 72 hours when each leg has exactly one
eligible partner.

Reuse the existing transfer group, matching timestamp, reporting behavior, and
partner inspection UI. Add one transaction exclusion flag, a dedicated pair
store, one explicit command, and one independent event subscriber. Load the
eligible ledger slice once, decide pairs in memory, and save each pair in an
ordinary database transaction. Eligibility reflects the loaded data; concurrent
matching and edits are an accepted Phase 0 risk.

```text
Tenant ledger action                      Committed bank-sync window
        |                                            |
POST /transactions/match-transfers         Existing window-completed event
        |                                            |
Explicit matching command                 Matching consumer group
        |                                            |
Job-observed handler                      Ordinary event handler
        |                                            |
        +---------------------+----------------------+
                              |
                  TransferMatchingService.Match
                              |
                   Load eligible ledger slice
                              |
                   Match both legs in memory
                              |
                   Save both legs together
                              |
                      Log attempt counts

Explicit feedback: existing job lifecycle and ledger refresh
Classification: separate subscriber and separate explicit command
```

## Finance service and ownership

Expose `TransferMatchingService` through `finance.Finance`, with two operations:

- `Submit`: accepts authenticated actor, tenant, `rangeStart`, and
  `rangeEndExclusive`; checks tenant membership and range validity, publishes
  the explicit command, and returns its future job reference.
- `Match`: accepts tenant and the same timestamp bounds, plus dispatch and
  source-sync message IDs for logging; returns in-memory attempt counts and an
  error. Validate nonzero bounds and `rangeStart < rangeEndExclusive` here too,
  so event and command execution cannot accidentally select unbounded history.

Use parameter structs and consumer-defined interfaces. Inject the access
store, pair store, logger, clock, and ID generator; validate required
dependencies in the constructor. Command publication is optional for process
roots that only execute work, following the existing classification pattern.

Add a dedicated `TransferPairStore` with two responsibilities within pair
persistence: load the eligible ledger slice and save the two legs of a pair.
Use narrow field updates for automatic link, manual link, and unlink. Keep
domain types separate from GORM models and do not add methods to the legacy
`Store`.

Move the ledger service's pair saves to this store, retaining its current
manual validation. The existing `TransferCandidateStore` continues serving the
manual candidate browser and partner reads.

Finance owns the eligibility rule, pair state, and exclusion state. The app
owns HTTP, dispatch adapters, event subscriptions, jobs, and worker lifecycle.
No runtime-module changes are needed.

## API and ledger interaction

Add `POST /api/v1/finance/tenants/{tenantId}/transactions/match-transfers`:

```json
{
  "rangeStart": "2026-08-08T00:00:00+02:00",
  "rangeEndExclusive": "2026-09-07T00:00:00+02:00"
}
```

Require both timestamps in RFC 3339 format with an offset or `Z`, and a strictly
increasing range. Use existing authentication, tenant-membership checks, and
error mapping. Requester identity comes from authentication. Invalid inputs
return the existing invalid-input response; publication failures do not return
a successful job reference.

After durable publication, return `202` with only:

```json
{ "jobId": "dispatch-message-id" }
```

Add **Match transfers** to the tenant ledger actions. Opening it reveals a
compact range form using existing Bootstrap form conventions, with start and
inclusive end dates. Default to today and the preceding 29 calendar days in
the viewer's local time. Do not derive matching scope from account, search,
source, or current-page filters: the form submits a tenant-wide date range.

Reuse classification's local-date boundary calculation: start of the first
selected day through start of the day after the last selected day. Use calendar
arithmetic for those boundaries and retain each boundary's correct offset
across DST. Reject reversed or invalid dates and allow historical ranges.
The example above includes August 8 through September 6, inclusive.

Show this scope explanation before submission:

> Searches all accounts in this tenant. A matching partner may be up to
> 72 hours outside the selected dates.

Use the existing `JobStatus` component and finance job detail route. Disable
duplicate submission while starting or observing the current run. Only the
initiating flow treats an initial `404` for its returned job ID as pending;
ordinary job navigation retains normal not-found behavior.

- Processing: show the existing queued/running lifecycle.
- Success: “Transfer matching completed.” Do not imply every transfer was
  found or claim a pair count that is not stored.
- Failure: “Transfer matching failed. Some pairs may already have been
  matched. Running it again preserves existing pairs.”
- On either terminal outcome, refresh the ledger through its existing refresh
  mechanism, since a failed pass may have committed pairs. Keep feedback
  scoped to the submitted tenant if the user changes tenants during execution.

Keep manual partner inspection, linking, and unlinking in the detail workflow.
Unlinking adds no warning, explanatory copy, or exclusion indicator. Do not
expose the exclusion flag in transaction API requests or responses.

## Required database schema changes

The complete schema change is one column on `finance_transactions`:

- Name: `transfer_matching_excluded`.
- PostgreSQL type: `boolean`.
- Nullability: `NOT NULL`.
- Default: `false`, including existing rows when the column is added.

Equivalent schema definition:

```sql
ALTER TABLE finance_transactions
    ADD COLUMN transfer_matching_excluded boolean NOT NULL DEFAULT false;
```

Implement this through the existing GORM auto-migrate path by adding the field
to `transactionModel` with explicit column, not-null, and default tags. The SQL
above describes the schema; it is not a separate migration script. Map the
column to `TransferMatchingExcluded` on the internal finance transaction type.

Reuse the existing `kind`, `transfer_group_id`, `transfer_matched_at`, and
`updated_at` columns. No new tables, indexes, foreign keys, uniqueness
constraints, or version columns are required. The bulk read uses the existing
`idx_finance_transactions_list_order` index, whose leading columns are
`tenant_id` and `effective_at`.

## Pair writes and exclusion state

An automatic pair updates only these fields on both existing rows:

- `kind = transfer`.
- One newly generated, shared `transfer_group_id`.
- One shared `transfer_matched_at`.
- `updated_at`.

Use one ordinary database transaction per pair, with both updates scoped by
tenant and transaction ID. Require each update to affect one existing row;
otherwise roll back both and record a skipped pair. A database error rolls back
that pair and fails the attempt. Previously committed pairs remain saved.

This transaction provides all-or-nothing writes. Matching uses the data loaded
at attempt start; there are no per-pair eligibility reads, candidate queries,
explicit `FOR UPDATE` reads, version checks, or optimistic-locking machinery.
The database handles its normal write locking as part of the updates.

Manual link retains the existing broader service validation and saves the same
pair fields. It allows unequal amounts, different currencies, and excluded
transactions. Manual unlink retains the existing pair validation and atomically
writes `kind = regular`, clears both pair fields, sets
`transfer_matching_excluded = true` on both legs, and updates `updated_at`.
Link operations leave the exclusion flag untouched. No Phase 0 operation
clears it.

Pair saves never insert missing rows and never write amounts, currencies,
effective timestamps, descriptions, categories, tags, or provider data.

### Ordinary transaction saves

For existing rows, ordinary transaction upserts preserve the stored `kind`,
`transfer_group_id`, `transfer_matched_at`, and `transfer_matching_excluded` by
omitting them from conflict-update assignments. New rows still receive their
initial values on insertion. Pair saves own subsequent changes to these fields;
current ordinary edits and provider refreshes do not intentionally change kind.

Use the existing shared persistence boundary for this field ownership. Where a
save returns transaction data, return the persisted pairing values through the
write's returned row. Carry the exclusion flag through domain/model mappings
and the provider merge. This keeps a later sync or ordinary save from resetting
pair state without adding a read-before-write check.

### Concurrency and retries

A subsequent run reloads current data and skips existing pairs and exclusions.
A retry after a partial failure therefore processes the remaining eligible
rows. Manual unlinking is respected by subsequent runs, and later amount/date
edits do not cause automatic rematching.

Overlapping attempts and manual changes can race after the initial load. An
in-flight pass can use stale amounts, dates, visibility, or exclusion state;
competing pair writes can overwrite pair metadata and leave relationships that
need manual correction. Atomic writes do not guarantee isolation between these
operations. Phase 0 accepts this risk, including an in-flight pass racing with
manual unlinking. Introduce optimistic locking only if observed issues justify
it; no concurrency-control framework is part of this design.

## Matching in memory

### Load the eligible ledger slice once

Run one tenant-scoped query for all eligible transactions in:

```text
loadStart        = rangeStart - 144 hours
loadEndExclusive = rangeEndExclusive + 144 hours

loadStart <= effectiveAt < loadEndExclusive
```

The query joins accounts to enforce tenant ownership and visibility. Both the
transaction and account must exist and be visible. Apply these transaction
predicates:

- Status is `booked`.
- Kind is `regular`, `expense`, `income`, or `transfer`.
- Both `transfer_group_id` and `transfer_matched_at` are null.
- Amount is nonzero and `transfer_matching_excluded` is false.

No source or category predicate applies. Hidden/deleted rows, missing or
hidden accounts, refunds, reconciliations, opening balances, and system kinds
are excluded. An unmatched transfer is eligible only when both pair fields
are absent.

Load only the matching fields: transaction ID, account ID, currency, signed
minor-unit amount, and effective timestamp. Eligibility is handled by the
query. Do not hydrate descriptions, tags, categories, or provider snapshots.
Finish this load before deciding or writing any pair. Do not limit it to a UI
page or silently truncate the result. Memory use grows with the selected range;
Phase 0 accepts that tradeoff and keeps the implementation to one bulk read.

### Why the load extends by 144 hours

A starting transaction can have a partner up to 72 hours away. That partner
can have a competing candidate another 72 hours away. Both windows must be
present to establish mutual uniqueness.

For example, A is a debit at hour 0 inside the requested range, B is an equal
credit at hour 72, and C is another equal debit at hour 144 on an account
different from B's. A sees only B, but B sees both A and C. Leave them unmatched.
Loading the requested range plus only 72 hours can miss C.

The single load therefore extends by 144 hours on each side. Matching still
uses a 72-hour tolerance, and every proposed pair must contain a transaction
inside the original range. The outer rows supply ambiguity evidence; they do
not widen which pairs the run may create. No recursive expansion is needed:
only the two proposed legs' candidate windows determine their uniqueness.

### Decide pairs

Build an in-memory index by currency and signed minor-unit amount, sorting each
bucket by effective timestamp and ID. For a transaction A, inspect the opposite
amount bucket within its inclusive 72-hour window, excluding its own account.
Use binary search to locate the time window and stop counting after two
eligible counterparts, since two already establishes ambiguity.

Amounts must be exactly opposite integers, with no floating point or unchecked
absolute-value arithmetic. The minimum signed 64-bit amount has no representable
opposite and therefore no candidate. Compare timestamps as instants; 72 hours
is an elapsed duration across DST, without explicit UTC normalization.

1. Select starting rows in memory using the original half-open range:
   `rangeStart <= effectiveAt < rangeEndExclusive`. Either sign can start a pair.
2. Find A's candidates in the loaded index. Zero means unmatched; more than one
   means ambiguous.
3. If B is the only candidate, find B's candidates in the same index. Accept the
   pair only if A is B's sole candidate. Otherwise it is ambiguous.
4. Deduplicate accepted pairs by their two IDs in a stable order. When both legs
   are starting rows, they still produce one pair.
5. Finish all decisions against the unchanged loaded index, then save each
   accepted pair using the ordinary pair transaction described above.

Never remove candidates while evaluating other starting rows, choose the first
or nearest candidate to break a tie, or query the database from inside the
matching loop. The algorithm is deterministic for the loaded input, independent
of input order. Check cancellation during evaluation and writes. On retry,
load the slice again and recompute from current data.

## Classification and reporting

Classification and matching remain independently executable. A category
already assigned does not prevent matching, and pairing never removes it.
Classification's conditional update skips transfer kinds and changes only
category/modification fields. A category assigned before matching is retained;
classification after matching skips the transfer. No ordering dependency or
combined enrichment job is introduced.

Reporting continues to use booked matched-transfer behavior: both ledger rows
affect account balances, while the matched internal transfer is excluded from
income and expenses. The exclusion flag has no reporting meaning.

## Dispatch, jobs, and worker lifecycle

Add `finance.transfer-matching.explicit.v1` with `tenantId`, `rangeStart`,
`rangeEndExclusive`, and existing requester metadata. Register one observed
handler with job type `finance.transfer-matching`. It calls `Match` and uses
existing finance failure mapping. An explicit retry is a new submission;
transport redelivery retains its existing dispatch identity.

Register an ordinary subscriber to the existing
`finance.bank-sync-window-completed.v1` topic in consumer group
`finance.transfer-matching.v1`. Pass the event's tenant and exact range to
`Match`; use the event message ID and `sourceSyncMessageId` for diagnostics.
Connection ID is logging context, not an account/source selection filter.

Keep the existing atomic publication of each committed window's ledger writes,
checkpoint, and completion event. No second sync event is needed. Matching
gets its own consumer group and offsets, so classification cannot consume its
delivery or gate its success. An early window remains actionable if a later
window or overall sync fails.

Wire the additional ordinary router into the existing worker root:

- Normal worker/start-all execution serves observed commands, classification
  events, and matching events independently. Cancel and join all router loops
  on shutdown or a router-level error.
- `jobs worker --once` drains observed commands first, then both ordinary event
  routers with their existing bounded idle-poll behavior. Attempt both event
  drains and combine their errors; one enrichment failure must not prevent the
  other drain from running. This is a bounded diagnostic drain, not an
  enrichment ordering guarantee.
- Stop all routers before closing their shared publisher/database. Include the
  new router in startup-failure cleanup.
- API-only `start` publishes explicit commands without executing matching.
  Scheduler behavior is unchanged.

Automatic matching creates no job projection and never changes the bank-sync
job outcome. Use existing dispatch retries and dead-letter handling. Ordinary
database/payload errors stay transport errors; do not invent a successful or
terminal business outcome to bypass that policy. Explicit work retains the
existing distinction between finance terminal failures and transport failures.

## Attempt counts and diagnostics

Keep counts in memory and log them on completion or failure:

- `matchedPairs`: newly committed pairs, counted once per pair.
- `unmatched`: eligible starting rows with no counterpart in the loaded data.
- `ambiguous`: starting rows rejected because either leg has multiple candidates.
- `skipped`: proposed pairs rolled back because an expected row was missing.

These counters have different units and do not sum to a transaction total.
Rows outside the original range supply matching evidence but do not contribute
starting-row outcomes. Rows excluded by the bulk read are not counted. Errors
may leave some planned pairs unattempted; only confirmed commits increment
`matchedPairs`. An unknown commit outcome can undercount diagnostics.

Log `tenantId`, `messageId`, `sourceSyncMessageId` when applicable, range bounds,
loaded row count, outcome counts, elapsed time, and ordinary errors. Use the
injected logger directly with camelCase keys. Counts describe one attempt;
they are not stored as job results. Retries load fresh data and restart counts.

## Implementation and verification

Implement backend work serially, with behavior tests integral to each step:

1. Add the boolean column, internal mappings, narrow atomic pair saves, and
   ordinary-save field preservation. Retain manual validation and add silent
   exclusion on unlink. Verify preservation and all-or-nothing writes.
2. Add the single bulk read and in-memory matcher. Verify complete windows,
   mutual uniqueness, deterministic decisions, deduplication, and range scope.
3. Add the explicit command/API and independent event subscriber. Wire normal,
   bounded, and shutdown worker paths. Test that both handlers pass identical
   range semantics to the same service.
4. Add the ledger action and existing job-feedback integration. Update the UI
   wireframe and manual E2E guide for matching and manual correction.

Cover these acceptance boundaries:

- Every eligible source/kind; categories and tags survive pairing. Exclude
  pending, hidden, deleted/missing, zero, excluded, refund/system, and linked
  rows. Enforce account visibility, different accounts, and tenant isolation.
- Exactly 72 hours versus just beyond, different offsets and DST, equal/opposite
  integer amounts, and same currency.
- One-to-many and many-to-one ambiguity, including the hour-0/hour-72/hour-144
  case on either side of the original range. Shuffling loaded rows gives the
  same pairs, and decisions use the complete input before any save.
- Partners outside either range boundary, both signs as starting rows, and
  both starting rows producing only one pair. Two outside legs never pair.
- One bulk selection call per attempt and no per-transaction/candidate reads.
  A retry reloads data; a completed pair or exclusion is absent from later loads.
- A second-leg write failure rolls back the first; a later-pair failure keeps
  earlier committed pairs. Missing-row updates roll back and count as skipped.
- Unlink preserves categories/tags and excludes both legs on subsequent runs;
  manual linking still accepts an excluded leg. Sync and ordinary saves retain
  pair state, and returned save data reflects the persisted values.
- Classification before/after matching and existing reporting/account-balance
  behavior. Do not assert concurrent matching/edit guarantees deferred by the PRD.
- Both event subscribers receive the same window independently. Matching
  creates no automatic job; failure leaves committed sync data and the bank-sync
  outcome intact. Include duplicate delivery and a later sync-window failure.
- API authentication, range validation, and publication response; local-date
  defaults, initial job `404`, terminal ledger refresh, and silent unlink UI.

Use generated Mockery mocks and focused PostgreSQL behavior tests. Keep any
migration coverage to one shallow smoke test. During implementation, bootstrap
PostgreSQL before backend tests and run the required affected lint/test checks
plus the applicable UI verification flow.

This revision changes documentation only. Update module worker-workflow and
implementation-status docs when the implementation lands.

## Existing implementation references

- [Classification service](../../finance/service_classification.go)
- [Classification persistence](../../finance/persistence/classification_transaction_store.go)
- [Ledger link/unlink](../../finance/service_ledger.go)
- [Current pair saves](../../finance/persistence/core_store.go)
- [Shared transaction upsert](../../finance/persistence/transaction_tag_store.go)
- [Manual candidate reads](../../finance/persistence/transfer_candidate_store.go)
- [Persistence models](../../finance/persistence/models.go)
- [Provider merge policy](../../finance/internal/providers/apply.go)
- [Sync-window writes](../../finance/internal/providers/window_sync_store.go)
- [Reporting](../../finance/reporting.go)
- [Semantic commands](../../finance/semantic_commands.go)
- [Window event adapter](../../apps/sumweave/internal/financeapp/bank_sync_window_event.go)
- [Observed finance handlers](../../apps/sumweave/internal/financeapp/register.go)
- [Worker composition](../../apps/sumweave/internal/wireup/jobs.go)
- [Finance HTTP contract](../../apps/sumweave/internal/api/http/v1routes.yaml)
- [Tenant ledger](../../apps/sumweave-ui/src/pages/FinanceTransactions.svelte)
- [Classification job feedback](../../apps/sumweave-ui/src/pages/FinanceRules.svelte)
- [Local-date boundaries](../../apps/sumweave-ui/src/lib/finance/classification-range.ts)
