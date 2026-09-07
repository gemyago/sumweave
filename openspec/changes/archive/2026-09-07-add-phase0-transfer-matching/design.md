## Context

The approved Phase 0 PRD and
`docs/enrichment-pipeline/transfer-matching-phase0-design.md` define the required
behavior and implementation order. The repository already has manual two-row
transfer pairing, partner inspection, matched-transfer reporting, finance-owned
GORM persistence, appdispatch commands, lazy jobs observation, an atomic
bank-sync-window completion event, independent automatic classification, a
composite worker, and a canonical Bootstrap Finance SPA. It does not yet have an
automatic transfer matcher, durable user exclusion, explicit matching command, or
matching event subscriber.

This change crosses `finance/`, `apps/sumweave/`, and `apps/sumweave-ui/`.
Finance owns eligibility, matching, pair/exclusion persistence, and range
execution. The app owns authentication and HTTP glue, dispatch adapters, observed
jobs, event subscriptions, and process lifecycle. The UI owns local-calendar
range construction and initiating-flow feedback. `runtime/` remains unchanged.

One OpenSpec change is used because automatic and explicit entry points exercise
one matcher and because persistence ownership, correction behavior, reporting,
worker composition, and UI feedback must remain coherent. Implementation remains
serialized in the four source-design parent groups.

## Goals / Non-Goals

**Goals:**

- Match only mutually unique booked equal-and-opposite same-currency pairs on
  different visible accounts within an inclusive 72-hour elapsed window.
- Decide against one complete in-memory snapshot and require at least one accepted
  leg to lie inside the original half-open processing range.
- Save each pair atomically with narrow updates while preserving categories, tags,
  amounts, dates, descriptions, provider data, and account-balance effects.
- Preserve pair state through ordinary writes and silently protect manual unlink
  corrections from future automatic and explicit matching.
- Reuse appdispatch, lazy job observation, committed-window events, reporting, and
  the existing ledger/job UI rather than introducing parallel infrastructure.
- Keep automatic matching and category classification independent while allowing
  both to consume every committed bank-sync window.

**Non-Goals:**

- Cross-currency, fee, split, many-to-one, pending, refund, confidence, semantic,
  description, exchange-rate, or configurable-rule matching.
- Suggestions, approval queues, dry-run previews, result dashboards, pair audit
  history, provenance, original-kind history, or exclusion-management UI.
- New manual-create, CSV-import, periodic-history, or rematching triggers.
- Per-pair eligibility reloads, row locks, optimistic locking, or guarantees for
  overlapping attempts and edits after the initial load.
- Exposing the exclusion flag through HTTP requests or responses, changing the
  existing manual-link rules, or altering runtime infrastructure.

## Decisions

### Pair-owned persistence is narrow and dedicated

Add `TransferMatchingExcluded` to the internal transaction domain and GORM model,
mapped to `finance_transactions.transfer_matching_excluded boolean NOT NULL
DEFAULT false` through existing auto-migration. Add no table, index, foreign key,
version column, or custom SQL migration. Carry the field through persistence and
provider merge mappings, but do not add it to transaction API models.

Create a dedicated `TransferPairStore`; do not extend the legacy `Store`. It owns
one eligible bulk read and pair writes. Move `LedgerService` manual link/unlink
writes to this store while retaining current service validation and the existing
`TransferCandidateStore` for manual candidate and partner reads.

Automatic and manual links update only `kind`, `transfer_group_id`,
`transfer_matched_at`, and `updated_at` on both tenant-scoped existing rows. They
leave the exclusion flag untouched. Manual unlink atomically writes `kind =
regular`, clears both pair fields, sets `transfer_matching_excluded = true`, and
updates `updated_at` on both legs. Categories and tags remain untouched.

Use one ordinary database transaction per pair. Each update must affect one row;
otherwise rollback both. The automatic path reports a missing row as skipped and
continues. A database error rolls back that pair, fails the attempt, and preserves
earlier committed pairs. Pair operations never insert missing rows.

The shared transaction upsert omits `kind`, `transfer_group_id`,
`transfer_matched_at`, and `transfer_matching_excluded` from conflict assignments.
New rows still receive initial values. When an ordinary save returns a transaction,
the returned row must carry persisted pair values rather than stale incoming ones.
Provider refreshes and user edits therefore cannot intentionally reset pair state.

### One bulk read establishes the immutable attempt snapshot

`TransferPairStore` loads matching projections with only transaction ID, account
ID, currency, signed minor amount, and effective timestamp. It performs one
tenant-scoped query over:

```text
loadStart        = rangeStart - 144 hours
loadEndExclusive = rangeEndExclusive + 144 hours

loadStart <= effectiveAt < loadEndExclusive
```

The query joins finance accounts and requires both account and transaction to be
visible and tenant-owned. It selects booked, nonzero, nonexcluded rows of kind
`regular`, `expense`, `income`, or unmatched `transfer`, with both pair fields
null. Source and category do not filter eligibility. Missing/hidden accounts,
pending or hidden rows, refunds, reconciliations, opening balances, system rows,
and linked transfers are excluded.

The load is complete before any pair decision or write. It is not paged or capped.
The existing tenant/effective-time transaction index remains sufficient. The
144-hour extension is required because a starting leg may see a partner 72 hours
away and that partner may see competing evidence a further 72 hours away. Outer
rows prove ambiguity but cannot form a pair when both legs are outside the
original range.

### The matcher is deterministic and mutually unique

Index loaded rows by currency and signed minor-unit amount; sort each bucket by
effective timestamp and ID. For each starting row in `rangeStart <= effectiveAt <
rangeEndExclusive`, binary-search the bucket for the exact opposite amount and an
inclusive 72-hour elapsed interval, excluding the same account. Stop candidate
counting after two because ambiguity is established. The minimum signed 64-bit
amount has no representable opposite and has no candidate. Compare timestamps as
instants with supplied offsets without explicit UTC normalization.

Accept A-B only when B is A's only candidate and A is B's only candidate. Never
choose a nearest/first tie-breaker or remove accepted rows while evaluating the
unchanged snapshot. Deduplicate accepted pairs by stable ordered IDs, so two
in-range starting legs still produce one write. Finish all decisions before saving
any pair. Input order does not affect the result, and cancellation is checked
during evaluation and writes.

`TransferMatchingService.Match` validates nonzero strictly increasing timestamp
bounds, loads once, decides in memory, and then writes accepted pairs. Its counts
are in-memory diagnostics: `matchedPairs` counts committed pairs, `unmatched`
counts eligible starting rows with no candidate, `ambiguous` counts starting rows
rejected because either leg is non-unique, and `skipped` counts missing-row pair
rollbacks. Loaded outer rows and query-excluded rows do not contribute starting-row
outcomes. Log range, loaded row count, counts, elapsed time, `tenantId`,
`messageId`, and optional `sourceSyncMessageId` with camelCase keys.

A retry reloads current data and naturally excludes completed pairs and manual
exclusions. No stale-write protection follows the load: overlapping matching,
manual link/unlink, or edits may race and require manual correction. This is the
approved Phase 0 concurrency boundary.

### Explicit matching is observed appdispatch work

Expose `TransferMatchingService` through `finance.Finance` with `Submit` and
`Match`. Required access store, pair store, logger, clock, and ID generator are
constructor-enforced through narrow consumer-defined interfaces. Command
publication is an explicit option for roots that submit work.

`Submit` checks membership and range validity, then publishes
`finance.transfer-matching.explicit.v1` with tenant, exact bounds, and requester
metadata. Every new user submission gets a fresh publication identity, including
an identical later range submission; redelivery of one already-published command
retains its existing identity. Publication creates no job row and does not execute
matching inline.

Add `POST /api/v1/finance/tenants/{tenantId}/transactions/match-transfers` with
required offset-bearing RFC 3339 `rangeStart` and `rangeEndExclusive`. Invalid or
reversed bounds use existing invalid-input mapping, tenant denial uses existing
finance authorization behavior, and publication failures return no successful
reference. A successful durable publication returns only `202 {"jobId":"..."}`.
Define the route in `v1routes.yaml`, regenerate apigen output, and test through
registered routes.

Register the command once through the existing job-observed router with job type
`finance.transfer-matching`. The handler passes tenant and range unchanged to
`Match` and uses existing finance terminal-versus-transport failure mapping. The
job is materialized lazily with the dispatch ID on first delivery. Only the
initiating flow may treat that known ID's initial `404` as pending.

### Every committed window reaches matching independently

Register an ordinary handler for existing
`finance.bank-sync-window-completed.v1` in consumer group
`finance.transfer-matching.v1`. Pass the event tenant and exact bounds to `Match`,
plus event message ID and `sourceSyncMessageId` for diagnostics. Connection ID is
log context, never an account/source filter. Automatic matching creates no job,
does not change bank-sync job status, and retains transport retry/dead-letter
behavior.

Classification remains in `finance.classification.v1`; distinct consumer-group
offsets guarantee both subscribers receive the same event independently. Neither
handler gates or consumes the other's delivery. Matching preserves categories;
classification keeps its existing transfer skip and category-preservation rules.
Matched transfers retain account-balance effects and use existing exclusion from
income and expenses.

Extend `WorkerRoot` from one observed worker plus one ordinary router to one
observed worker plus classification and matching routers. Long-running execution
runs all three under one cancellation/error lifecycle. Shutdown and startup-failure
cleanup stop every router before closing publisher/database resources.

For `jobs worker --once`, drain observed commands first, then attempt both ordinary
routers using their bounded idle-poll behavior. Attempt the classification and
matching drains even if one reports an error, and join their errors. This ordering
allows a bounded bank-sync delivery to emit the shared event before both
enrichments drain; it is not an enrichment ordering guarantee. Update
`apps/sumweave/AGENTS.md` when this implemented worker workflow lands.

### The ledger action reuses local dates and job feedback

Add **Match transfers** to `#/finance/transactions` as a compact Bootstrap range
form near ledger actions. It defaults to today and the preceding 29 local calendar
dates, shows inclusive start/end values, allows older ranges, and rejects a start
after end. Reuse or generalize classification's local-date helper to submit local
start-of-first-day through local start-of-day-after-last-date, preserving boundary
offset changes across DST by calendar arithmetic.

The action is tenant-wide regardless of account, search, source, or current-page
filters and shows before submission:

> Searches all accounts in this tenant. A matching partner may be up to 72 hours
> outside the selected dates.

Disable duplicate submission while starting or observing the active run. Reuse
`JobStatus` and the Finance job detail link. Show “Transfer matching completed.”
without claiming a pair count. Show “Transfer matching failed. Some pairs may
already have been matched. Running it again preserves existing pairs.” on failure.
Refresh the ledger after either terminal outcome because a failed attempt may have
committed earlier pairs, and scope the callback to the initiating tenant if the
active tenant changes. Keep existing manual partner/link/unlink UI unchanged and
show no exclusion warning or indicator.

## Ordered Implementation Chunks And Gates

Backend work is serialized. The four parent task groups remain in this order:

1. **Pair persistence and preservation** — schema/mappings, shared ordinary-save
   ownership, dedicated atomic pair writes, ledger manual-link migration, silent
   unlink exclusion, and preservation/rollback tests.
2. **Bulk load and matcher/service** — complete eligible projection read,
   deterministic mutual-uniqueness algorithm, range/count semantics, focused
   service composition, and acceptance-boundary tests.
3. **Command, API, event, and worker lifecycle** — explicit publication and lazy
   job observation, generated OpenAPI route/controller, independent matching event
   subscriber, normal/once/shutdown worker composition, and integration tests.
4. **Ledger feedback and verification readiness** — client/range behavior, ledger
   action and job feedback, visual checks, wireframe, implementation-status and
   manual E2E docs.

Each chunk uses `openspec apply`, marks only completed tasks, bootstraps PostgreSQL
before backend tests, runs applicable focused checks and `make affected-lint-test`,
receives a shallow independent review, resolves concrete blockers, and is committed
before the next chunk. Generated dependency mocks come from `go run
github.com/vektra/mockery/v3` at the owning module root. Generated API routes come
from `go generate ./internal/api/http/register.go`. Tests use randomized fixtures,
registered routes, behavior assertions rather than SQL-string matching, and at
most one shallow migration smoke.

After each clean chunk, compare implementation and evidence with the PRD, source
design, OpenSpec artifacts, and repository state. Convert any concrete deviation
or uncertainty into an in-order task and focused re-review before proceeding.
Accepted concurrency risk does not justify adding locking scope.

## Implementation Completion E2E Gate

After all four chunks and checks pass, execute a deterministic manual E2E round
against an isolated/reseeded PostgreSQL database. This is a completion gate, not a
separate checkbox task:

- Start API-only, create the tenant, visible accounts, categories/rules, and
  matching fixtures through protected APIs/manual records, and keep the normal
  worker stopped until each pre-delivery assertion is made.
- Submit explicit matching, assert the returned job initially yields `404`, run
  `jobs worker --once`, then verify terminal job state, ledger refresh data, and
  the tenant-wide match result.
- Cover a unique pair, same-account and cross-tenant isolation, pending/refund/
  zero/linked/excluded rows, exact and unequal amounts, same and cross currency,
  exactly 72 hours and just beyond, both signs, an outside-range partner, two
  outside-range legs, one-to-many/many-to-one ambiguity including hour
  0/72/144, and no page/filter-induced uniqueness.
- Verify categories/tags and provider data survive linking; matched rows remain in
  balances but leave income/expense totals. Run classification before one match
  and after another to prove category preservation and independent execution.
- Unlink an automatic pair, rerun explicit matching to prove both legs stay
  excluded, then manually link an excluded leg to the intended partner and verify
  the broader correction workflow remains available and silent.
- Submit the same explicit range again and redeliver/retry work to verify fresh
  submissions get fresh job IDs while completed pairs remain unchanged. Exercise
  failure after an earlier pair commit and confirm rerun processes only remaining
  eligible rows.
- Use one deterministic synthetic provider sync only where needed to emit a real
  committed-window event over prepared manual fixtures. In one bounded worker run,
  verify classification and matching both receive the same window, automatic
  matching creates no job, a later sync-window failure cannot erase an earlier
  event or committed ledger rows, and repeated delivery is safe.
- Run the **Match transfers** flow in a headed browser at desktop and narrow
  viewports, including displayed default dates, scope copy, initial pending state,
  lifecycle feedback, terminal refresh, manual detail correction, and the required
  Finance shell smoke/regression path.
- Store scripts, logs, API bodies, and screenshots under `tmp/`. Fix every concrete
  data, API, worker, responsive, or visual finding in an ordered follow-up chunk,
  rerun affected automated checks, and repeat the relevant E2E round until clean.

Only then perform the overlay's first deep final review from the recorded
pre-implementation commit through the last implementation commit. Concrete
blockers become ordered fix chunks with shallow review and commits; a focused final
re-review verifies only those fixes. Present the clean result for natural-language
user approval before archive/submission.

## Risks / Trade-offs

- **Large historical ranges consume memory** -> Use one compact projection and the
  existing effective-time index; Phase 0 explicitly favors complete uniqueness
  evidence over paging.
- **Equal nearby movements can be unrelated** -> Require mutual uniqueness and
  retain manual unlink/correction; no confidence claim is made.
- **Concurrent attempts or edits can overwrite loaded assumptions** -> Keep atomic
  pair writes, reload on retry, log outcomes, and accept the approved Phase 0 race.
- **A pass can fail after some pair commits** -> Preserve committed pairs, expose
  explicit failure copy, and make sequential reruns exclusion/idempotence safe.
- **Two ordinary enrichment routers can fail independently** -> Give them distinct
  consumer groups, attempt both bounded drains, join errors, and stop all routers
  before shared resources.
- **DST changes alter boundary offsets** -> Build each boundary from its own local
  calendar day and test ranges whose offsets differ.

## Migration Plan

1. Commit the independently reviewed OpenSpec plan and record that SHA.
2. Implement, shallow-review, and commit the four chunks in order.
3. Run the existing explicit migration/bootstrap path to auto-migrate the boolean
   column before API or worker execution.
4. Complete automated checks, UI visual review, and the clean repeated isolated
   PostgreSQL E2E gate.
5. Complete deep final review and any focused fix re-review, then wait for user
   approval before archive/submission.

Early alpha permits a clean local database reseed if rollback is needed. No
compatibility path is maintained.

## Open Questions

No product-design question blocks plan review. Implementation discoveries must be
resolved at the required chunk deviation gate rather than deferred.
