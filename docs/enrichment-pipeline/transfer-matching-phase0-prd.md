# Transfer Matching Phase 0

Status: implemented. The same-currency Phase 0 matcher was delivered on
2026-09-07 and its description-derived FX rule on 2026-09-09. The isolated
manual E2E and independent UI design-review gates are recorded separately and
remain to be run. Product requirements were approved on 2026-09-06; concurrency
requirements were revised on 2026-09-07 following design review: match against
data loaded at attempt start and accept concurrent-update races in Phase 0.
System design, API contracts, and storage changes are outside this PRD.
[Architecture](../ARCHITECTURE.md) remains the source of truth for product direction.

## Goal

Automatically recognize simple movements between accounts tracked in the same
finance tenant, so those movements do not inflate reported income and expenses.

Phase 0 links two existing ledger transactions using two fixed, deterministic
candidate rules. It leaves uncertain cases unchanged and retains manual linking
as the fallback. It never initiates a bank transfer or creates a missing
transaction.

## Existing foundation

- [Classification Phase 0](classification-phase0-prd.md) processes a tenant and
  transaction-date range after each committed bank-sync window. Explicit runs
  use existing job lifecycle feedback; automatic event handling has no job row.
- The ledger already supports manual linking, partner inspection, and unlinking.
  Linking marks both transactions as transfers with a shared group and matching
  timestamp, while preserving their categories and tags. Pair writes are atomic.
- Reporting already excludes booked, matched internal transfers from income and
  expenses. Unmatched transfers still contribute according to their amount sign.
- The current manual candidate list is a date-based browser across other
  accounts. Manual linking validates booked, nonzero, opposite-direction,
  unlinked transactions, but does not require equal amounts or equal currencies.
  Automatic matching needs its own stricter eligibility rule.
- Current unlinking clears the pair and returns both kinds to `regular`.
  There is currently no automatic-matching exclusion or rejected-pair memory.

Source references: [ledger pairing](../../finance/service_ledger.go),
[pair persistence](../../finance/persistence/core_store.go),
[candidate reads](../../finance/persistence/transfer_candidate_store.go),
[reporting](../../finance/reporting.go), and
[sync-window reaction](../../apps/sumweave/internal/financeapp/bank_sync_window_event.go).

## Product requirements

### 1. Eligible transactions

Both legs must:

- Belong to the same tenant and different, visible tracked accounts.
- Be booked/settled, visible, and not deleted.
- Have kind `regular`, `expense`, `income`, or unmatched `transfer`.
- Have neither an existing transfer group nor a matching timestamp.
- Have a nonzero amount and not be excluded from automatic matching by the user.

Pending transactions are excluded and may become eligible once booked, provided
all other matching requirements are met. Refunds, reconciliations, opening
balances, and system transactions are excluded. Refunds remain outside Phase 0
matching because a refund and a purchase can otherwise resemble an internal
movement.

Eligible bank-synced, CSV-imported, and manually entered transactions participate
under the same rules. An existing category does not prevent matching.

### 2. Two fixed candidate rules

Every candidate pair must have different accounts, opposite signs, and ledger
effective timestamps no more than 72 hours apart, inclusive. The same-currency
rule additionally requires:

- The same transaction currency.
- Exactly equal absolute amounts in minor units, with opposite signs.

The FX rule additionally requires matching typed evidence extracted from each
row's current ledger description. The initial complete ASCII description form is
`FX<digits> <BASE>/<QUOTE> <rate>`, where currencies are distinct uppercase
three-letter ISO codes and the positive decimal rate uses a comma or dot
separator. The evidence contains namespace `FX`, the deal reference, ordered
base/quote currencies, and an exact rate; comma/dot spellings and insignificant
trailing decimal zeros have the same value.

FX rows must have one unambiguous existing provider transaction mapping to the
same bank connection. Missing mappings or mappings to multiple distinct
connections disable only the FX rule; those rows remain eligible for the
same-currency rule. Both rows must agree on connection, namespace, reference,
ordered currency pair, and exact rate. Their ledger currencies must cover that
pair, and the absolute quote amount must equal the absolute base amount converted
by the rate and rounded to the quote currency's standard minor unit. Exact
integer and decimal arithmetic is used, with an exact positive halfway result
rounded upward; no binary floating point or amount tolerance applies.

Candidates from both rules are merged and deduplicated before applying mutual
uniqueness. A pair qualifies only when each leg has exactly one possible partner
across the combined candidate set and they are each other.

Use ledger amounts, currencies, and effective timestamps loaded at attempt
start, including user edits already saved. Compare timestamps as instants with
their supplied offsets. The 72-hour tolerance is a fixed elapsed duration,
including across DST changes.

Descriptions are matching inputs only for the narrow FX evidence format above.
Names, provider snapshots, account identifiers from payment descriptions, and
other description text are not matching inputs. No user-defined rules,
adjustable tolerances, structured provider exchange-rate data, market-rate
lookup, or fee inference is included.

Examples:

- Account A has `-500.00 PLN`; account B has `+500.00 PLN` one day later.
  If neither has another candidate, link them automatically.
- Account A has two `-500.00 PLN` transactions near one `+500.00 PLN`
  transaction on account B. Leave all three unchanged: uniqueness must hold
  from both sides. Never choose the first or nearest candidate to break a tie.
- `-500.00 PLN` and `+495.00 PLN`, or `-500.00 PLN` and `+120.00 EUR`,
  remain unmatched. Manual linking remains available.
- Account A has `-100.00 USD` with `FX123 USD/PLN 4.125`, and account B has
  `+412.50 PLN` with `FX123 USD/PLN 4,12500`. If both have the same unambiguous
  bank connection and neither has another candidate under either rule, link
  them automatically.

Uniqueness is evaluated against all eligible transactions loaded for each
leg's full matching window, including outside the requested processing range.
Candidate paging or processing order must not manufacture a unique match by
hiding competing transactions.

### 3. Matching result and preservation

- Automatically link qualifying pairs without a suggestion-approval step.
- Reuse the existing two-transaction internal-transfer representation and
  partner inspection UI.
- Change only transfer kind, pair metadata, and modification timestamps.
  Preserve amounts, dates, descriptions, categories, tags, and provider data.
- Keep the two ledger rows and their account-balance effects. Their income and
  expense treatment follows existing matched-transfer reporting behavior.
- Save both legs together or neither; never leave a half-linked pair.
- Each attempt excludes pairs and user exclusions already present when its
  input is loaded. Sequential reruns and redeliveries preserve completed pairs.
- Decide eligibility and matching from the loaded data. Save each pair
  atomically; a missing leg leaves that proposed pair unchanged and allows the
  run to continue. Concurrent matching or manual edits after loading are an
  accepted Phase 0 risk; fresh eligibility checks and optimistic locking are
  deferred.

Once matched, a pair is not automatically reconsidered when another transaction
arrives or an amount/date is later edited. Corrections use manual unlinking.
Repeated bank sync must preserve the pair and user exclusion state.

### 4. Automatic matching and range behavior

Each successfully committed bank-sync window starts matching for its tenant and
ledger date range, using the existing durable window-completion trigger.

- Transactions in that range are starting points. Search across all eligible
  accounts and sources in the tenant for their other legs.
- A partner may fall outside the event range, within the 72-hour tolerance.
  Matching can therefore update a partner outside that range.
- At least one leg of every newly linked pair must be in the triggering range.
- If the second leg has not arrived, leave the first unchanged. A later sync
  covering the second leg can match back to the first, even across connections
  or sync windows and regardless of which direction arrives first.
- Manual creation and CSV import do not themselves start matching in Phase 0.
  Their eligible transactions can participate in a later applicable sync pass
  or explicit run.
- Matching failures preserve committed bank data and completed pairs. An early
  window's durable trigger survives a later sync failure or process crash.
- Automatic matching creates no separate user-facing job and does not alter
  the bank-sync job outcome. It uses existing background retry/error handling.

### 5. Relationship to category classification

Transfer matching and category classification remain separate enrichment
operations and can react independently to the same committed sync window.

- Matching does not assign, clear, or overwrite categories.
- A category assigned before matching does not block a pair. It remains stored
  after matching, while the matched transfer is excluded from income/expenses.
- Classification continues skipping transactions already marked as transfers
  and continues preserving every existing category.
- Classification does not have to finish before matching, and matching does
  not have to finish before classification. A temporary category on a newly
  detected transfer is acceptable; it need not be removed.
- Explicit classification continues to classify only. Explicit matching
  continues to match only.

### 6. Explicit matching and user feedback

Add a **Match transfers** action to the tenant ledger, using the established
explicit-classification range and job-feedback pattern.

- Default to today and the preceding 29 calendar days, show the actual dates,
  allow a wider historical range, and reject a reversed range.
- Apply the selected inclusive dates to the ledger effective date using the
  same local-date boundary behavior as explicit classification.
- Explain before starting that matching searches all tenant accounts and may
  link a partner up to 72 hours outside the selected date boundaries.
- Use the same eligibility and matching behavior as automatic runs.
- Show processing, completion, or failure through existing jobs; refresh the
  ledger when processing finishes. Completion means the pass finished, not
  that every transfer was found or that any pair was necessarily linked.
- On failure, explain that some pairs may already have been matched and that
  running again preserves existing pairs.
- Log newly committed pair counts and unmatched, ambiguous, and conflict/skip
  outcomes for diagnostics. Durable result counts and a results dashboard are
  not required.

### 7. Manual correction

The existing detail-page partner view, manual link, and unlink actions remain
the correction workflow. Manual linking retains its existing broader rules.

When a user unlinks a pair, clear the relationship and return both transactions
to `regular`, as today. Preserve their current categories and tags. Exclude
both transactions from subsequent automatic and explicit matching, so the next
run cannot silently undo that correction. Respect this choice silently, using
the existing unlink interaction without additional warnings, explanatory copy,
or exclusion indicators.

An excluded transaction can still be manually linked to the correct partner.
Restoring automatic eligibility is deferred; Phase 0 needs no exclusion
management screen, original-kind history, or manual/automatic pair provenance.

## Acceptance scenarios

- A unique equal-and-opposite same-currency pair within 72 hours becomes a
  matched internal transfer, retains its categories/tags, and no longer
  contributes to income or expenses.
- Same-account, cross-tenant, unequal-amount, zero-amount, hidden, pending,
  refund, system, and already-linked cases remain unchanged. Cross-currency
  rows remain unchanged unless they meet every scoped FX rule requirement.
- A pair with either leg pending remains unmatched. Once both legs are booked,
  a subsequent applicable run can link them if all matching requirements hold.
- A gap of exactly 72 hours qualifies; a larger gap does not.
- One-to-many and many-to-one candidates remain unchanged, including when a
  competing candidate lies outside the run's date range or on another page, or
  when the competing candidate arises under the other automatic rule.
- A valid USD/PLN FX pair with mismatched connection, reference, currency pair,
  rate, or converted value remains unchanged. Missing or conflicting connection
  provenance likewise prevents FX matching without blocking same-currency
  matching.
- Either leg can arrive first. When the other arrives in a later connection
  sync or adjacent window, matching finds the older leg within the tolerance.
- An explicit range can match to a nearby partner outside its boundaries, but
  cannot create a pair whose two legs are both outside the range.
- Running classification before matching does not prevent a pair or erase its
  category; running classification after matching skips the transfer.
- A subsequent event delivery or explicit run preserves pairs present when it
  loads data. Overlapping attempts and manual edits have no concurrency
  guarantee in Phase 0.
- A write failure leaves neither leg linked; earlier completed pairs and
  imported transactions survive and retries can continue with remaining rows.
- Unlinking preserves categories/tags and excludes both legs from future runs;
  manually linking an excluded leg to its correct partner still works. This
  protection adds no warnings, explanatory copy, or exclusion indicators.

## Approved decisions and Phase 0 limits

Phase 0 uses direct automatic linking, a fixed 72-hour tolerance, and eligibility
regardless of category. Both legs must be booked; pending transactions are
excluded even when all other matching conditions hold. This keeps Phase 0
aligned with existing booked-transfer reporting and avoids a pending-pair
settlement lifecycle. Explicit unlinking is respected silently by excluding
both legs from subsequent matching.

Matching loads its inputs once per attempt and decides pairs in memory. Pair
writes remain atomic, while concurrent-update protection is deferred. A pass
already in flight can race with another match or manual unlink, overwrite pair
metadata, or use stale eligibility. Such races may require manual correction.
Consider optimistic locking later if observed issues justify it.

### Matching limitations

Equal amounts and nearby dates are a heuristic, not proof of account-to-account
movement. An unrelated payment and receipt can form a unique pair. A later
arrival can also reveal ambiguity that was absent when the first pair was
matched. Phase 0 accepts that limitation and relies on manual correction;
it does not claim a measured precision rate from repository analysis alone.

Not required in Phase 0:

- Pending-transaction matching or pending-pair settlement revalidation.
- Amount tolerances, fee inference, and any FX format other than the documented
  scoped description evidence.
- Split, many-to-one, or one-to-many transfers.
- Matching to accounts outside the tenant or creating missing legs.
- Transfer-rule management, LLMs, or confidence scores.
- A suggestions inbox, bulk approval, dry-run preview, or match audit history.
- New triggers for CSV import/manual creation, periodic full-history scans,
  or automatic rematching of existing pairs.
- A generic pipeline editor, stage dependencies, or combined enrichment job.

Any future tuning of the matching heuristic or tolerance requires a deliberate
change to this approved PRD.
