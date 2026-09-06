## Context

The Phase 0 PRD defines deterministic tenant-managed classification while
`docs/enrichment-pipeline/classification-phase0-design.md` fixes the system shape
and implementation order. The current repository already has finance-owned GORM
persistence, focused services, provider-sync requested-window transactions,
appdispatch semantic commands, typed `appevents`, lazy job observation, a split
API/worker process model, and a canonical Bootstrap Finance SPA. It does not yet
have classification data, services, APIs, events, worker subscriptions, or UI.

This change crosses `finance/`, `apps/sumweave/`, and `apps/sumweave-ui/`. Finance
continues to own rules, matching, eligibility, category writes, and the fact that
a requested bank window committed. The app continues to own HTTP/auth glue,
appdispatch and typed event adapters, jobs observation, and worker lifecycle. The
UI continues to own local-calendar range construction and initiating-flow job
feedback. `runtime/` is unaffected.

One coherent OpenSpec change is used because the explicit and automatic entry
points are acceptance paths into one classifier and because the category and
bank-window guarantees must archive together with their UI contract. The overlay
is applied repeatedly to ordered implementation chunks inside this change; a
single change does not mean one implementation or review pass.

## Goals / Non-Goals

**Goals:**

- Add deterministic, ordered, tenant-isolated exact and contains rules.
- Classify only currently uncategorized, visible, booked ordinary/refund rows in a
  half-open effective-time range without changing any other transaction field.
- Use one classifier for explicit observed commands and ordinary committed-window
  events, with rules loaded once per attempt and current state rechecked on writes.
- Make each bank-window completion event atomic with that window's writes and
  successful checkpoint while keeping classification outcome independent of the
  bank-sync job.
- Add protected APIs and canonical Finance UI for rule management, optional rule
  creation after manual category assignment, and explicit classification feedback.
- Preserve repository completion gates, independent OpenSpec reviews, phase-by-
  phase deviation resolution, and repeated deterministic synthetic-sync E2E.

**Non-Goals:**

- Reclassification, category overwrite, assignment provenance, protected
  uncategorized state, disabled rules, automatic rule generation, confidence,
  LLM or semantic matching, regex/wildcards, or transfer detection.
- Triggering classification from manual creation, CSV import, or rule mutations.
- Persisting classification counts, progress, result payloads, or an automatic-
  classification job record.
- Resolving the accepted Phase 0 concurrency races beyond scoped selection and a
  conditional category update.
- Database compatibility or data migration beyond adding the new GORM-managed
  table in this early-alpha PostgreSQL-only product.

## Decisions

### Finance domain and persistence stay focused

Add `ClassificationRule`, `ClassificationMatchType`, request parameter, and
attempt-count domain types under finance. Add one explicit-column GORM model for
`finance_classification_rules` and include it in the existing migrator. Its
ordered-read index is `(tenant_id, position, id)` and category-reference index is
`(tenant_id, category_id)`.

Use a dedicated classification-rule store for ordered reads, append, replacement,
gap-closing delete, adjacent move, and category-reference lookup. Use a separate
classification transaction store for eligible ID-keyset reads and conditional
category assignment. Do not add methods to the legacy `persistence.Store`.
Multi-row ordering operations remain short GORM transactions. The classifier's
write operation includes tenant, null category, booked status, visible state, and
eligible-kind predicates and updates only `category_id` and `updated_at`.

Expose two focused services from `finance.Finance`:

- `ClassificationRuleService` owns membership checks, rule validation, tenant and
  visible-category validation, ordered CRUD/move, and reference lookup.
- `ClassificationService` owns explicit submission and trusted background
  execution. Submission applies membership and range validation before publishing;
  execution accepts one parameter object with tenant, range, and logging context.

Constructors receive narrow consumer-defined interfaces and required dependencies;
process-specific publication is supplied through an explicit option as in existing
finance command publishers. The catalog receives a required narrow rule-reference
dependency so every `HideCategory` call, public or internal, is guarded before the
logical hide. A typed finance error carries the referencing IDs for the API's
documented `409` body.

### Matching and attempt execution are in-memory and deterministic

At attempt start, load the tenant's ordered rules exactly once and retain the
slice unchanged for that attempt. The internal matcher trims surrounding space,
uses case-insensitive Unicode string comparison, preserves internal whitespace and
punctuation, performs literal substring matching, and returns the first rule.
Blank descriptions return no match.

Read up to 200 eligible transactions ordered by ID, using the last ID as the next
keyset cursor while retaining `effective_at >= rangeStart` and
`effective_at < rangeEndExclusive`. The initial database selection excludes rows
that cannot be classified and therefore does not count them. For each selected
row, no match increments `unmatched`; a failed eligibility/conditional update
increments `skipped`; a committed category assignment increments `classified`.
Validate the loaded rule's category remains visible and same-tenant before its
assignment. A missing category is a finance terminal failure. Log batch context,
completion or handled failure, counts, elapsed time, `tenantId`, `messageId`, and
`sourceSyncMessageId` where available, using camelCase keys.

### Explicit classification is an appdispatch command observed as a job

Define `finance.classification.explicit.v1` with tenant ID, unchanged full
timestamp bounds, and existing requester metadata. The protected
`POST /api/v1/finance/tenants/{tenantId}/transactions/classify` operation accepts
`rangeStart` and `rangeEndExclusive`, requires offset-bearing RFC 3339 timestamps
with strict start-before-end ordering, publishes without inline execution, and
returns `202 {"jobId":"..."}`.

Register one observed handler with job type `finance.classification`. It maps the
message tenant and range unchanged into `ClassificationService.Classify`, uses the
existing terminal-failure adapter, and otherwise preserves appdispatch retry and
dead-letter semantics. Publication remains job-row-free; the initiating UI alone
may interpret a pre-delivery `404` for its returned ID as pending.

Add the routes to `v1routes.yaml`, regenerate apigen output, and implement
controller consumer interfaces around the focused finance services. Rule mutation
responses follow the design's minimal `201` ID and `204` contracts. Category
removal is added as `DELETE /categories/{categoryId}`; its documented reference
conflict is the narrow exception that needs a safe body and may use the controller's
direct HTTP response path.

### A committed bank window publishes one atomic typed event

Define finance-owned `BankSyncWindowCompleted` data with tenant ID, connection ID,
requested range start/exclusive end, and source sync message ID. Inject a narrow
`BankSyncWindowCompletionPublisher` into the provider requested-window apply path.
The app adapter converts that finance fact to the typed `appevents` event on
`finance.bank-sync-window-completed.v1` and calls `PublishInTx` with the exact SQL
transaction used by finance persistence.

The provider window transaction seam exposes only the transaction handle needed
by that publisher. Within `ApplySync`, save ledger/provider data, append the
successful state, and publish the completion event before commit. Event publication
failure rolls back all three. The event bounds come from the concrete requested
window and the source message ID comes from the bank-sync execution context, not
from a later whole-sync projection. Earlier window transactions remain committed
when a later window fails.

```text
provider requested window
        |
        v
finance apply transaction
  - ledger/provider writes
  - successful checkpoint
  - typed completion event
        |
        +---- commit ----> finance.classification.v1 subscriber
                                  |
                                  v
                         shared range classifier
```

### The worker owns observed and ordinary consumers without conflating them

Keep the explicit command on the existing job-observed router. Add an app-owned
ordinary classification event consumer backed by a separate appdispatch router in
consumer group `finance.classification.v1`; it creates no job state and maps the
event's bounds directly to the same classifier.

The worker root owns both consumers and closes both before publisher/database
shutdown. Long-running `jobs worker` runs them under one cancellation/error
lifecycle. Bounded `jobs worker --once` drains the observed-job consumer first so
bank-sync deliveries can publish completion events, then drains the ordinary
classification consumer with the same two-idle-poll convention. The CLI runtime
delegates `Run`, `RunOnce`, and `Close` to this composite root. This preserves the
existing command surface while making a single bounded E2E invocation process the
bank job and its newly committed classification event.

### The Finance UI adds one real rules destination

Add protected `#/finance/rules`, its shell rail/breadcrumb entry, active-tenant
participation, document title, and wireframe documentation. Implement typed
finance API client methods for rule CRUD/move, category deletion conflict data,
and explicit classification submission.

The rules page uses native Svelte and Bootstrap classes without route-local layout
CSS. It shows the visible evaluation order and target category names, uses local
create/edit/delete and edge-aware move controls, refetches after mutation, and
contains the explicit-run panel. The panel defaults to today plus the preceding 29
calendar dates. It converts selected local dates to start-of-first-day and
start-of-day-after-last-date with calendar arithmetic, then serializes full
offset-bearing RFC 3339 instants. Existing observed-dispatch/job-status behavior
shows pending, queued, running, success, failure, and a Finance job link; success
refreshes relevant ledger data and failure warns that partial assignments may have
committed.

After a successful transaction category assignment from the shared inline editor
or full editor, show a separate optional rule form using the current description
and category. Save is explicit; cancel or save failure cannot undo the category
request. Category removal surfaces referencing rules and directs the operator to
retarget or delete them.

### Ordered phases are overlay chunks with deviation gates

Implementation remains serialized for every backend-affecting chunk. The clean
plan reviewer may refine boundaries but must preserve these consecutive parent
task groups:

1. **Rules and matching foundation** — schema/domain, dedicated rule operations,
   category guard, matcher, bounded classifier, composition, and their tests.
2. **Explicit command and API** — submission command, observed handler, HTTP
   contracts/controllers/codegen, and parity/range tests.
3. **Committed-window automatic classification** — atomic event publication,
   ordinary subscriber, composite worker lifecycle, and crash/rollback/duplicate
   tests.
4. **Management UI** — rules route, shell/API integration, ordering and category
   feedback, manual-assignment rule offer, tests, and UI docs.
5. **Explicit feedback UI and synthetic verification readiness** — local date
   panel, job lifecycle feedback, automated DST/pending/failure tests, final
   wireframe/runbook updates, and UI review readiness.

Each phase uses `openspec apply`, marks only completed tasks, runs its applicable
checks, receives a shallow independent chunk review, resolves concrete blockers,
and is committed before continuing. After that clean chunk gate, the manager must
compare delivered behavior and evidence with the PRD, source design, OpenSpec
artifacts, and repository state. Any concrete deviation, uncertainty, failed check,
or newly discovered issue is resolved through an added or clarified in-order task
and focused re-review before the next phase starts. Accepted Phase 0 concurrency
risks are recorded, not silently expanded into extra scope.

### Verification is integral to implementation and overlay gates

Tests are written before implementation for each non-visual behavior and remain
inside their owning parent task. Use generated Mockery mocks, randomized fixtures,
registered HTTP routes, and one shallow migration smoke only. Do not test generated
ORM SQL strings. Required automated coverage includes matching order and text
normalization, all four eligible kinds, ineligible/excluded count behavior, tenant
and range isolation, conditional writes, rules loaded once, retry reload, API
validation, command/event range parity, category references, atomic event rollback,
early-window/later-failure durability, crash-after-commit behavior, duplicate event
delivery, and worker lifecycle.

Every code chunk runs `make affected-lint-test` after `make postgres-bootstrap`
where backend tests are involved. UI chunks also run the UI completion flow,
including an independent design review and focused fixes/re-review. Commands,
workflow, architecture, wireframe, and module instruction changes are assessed at
the chunk that introduces them; update `AGENTS.md` only if the established rules
or commands actually change.

After all implementation chunks and checks are green, run the synthetic manual E2E
as an implementation completion gate, not as a separate OpenSpec checkbox task:

- Extend the isolated API-only synthetic flow to create target categories and
  ordered rules, publish a fixed-window bank sync, observe the expected pre-worker
  `404`, drain the bounded worker, and verify matching synthetic transactions were
  automatically categorized while unmatched and pre-categorized rows are safe.
- Trigger an explicit range containing an uncategorized manual or CSV transaction,
  observe its own pre-delivery `404`, drain the worker, and verify terminal job and
  assignment behavior. Repeat sync/delivery to prove existing categories survive.
- Run the rules/manual-assignment/explicit-feedback browser flow at desktop and a
  narrow viewport, plus the required Finance shell smoke path.
- Record artifacts under `tmp/`, fix every concrete API, worker, data, responsive,
  or visual finding in an ordered follow-up chunk, rerun affected automated checks,
  then repeat the relevant E2E round until no concrete findings remain.

Only after the E2E gate is clean does a fresh final reviewer perform the overlay's
first deep review over the recorded pre-implementation commit through the last
implementation commit. Concrete blockers become ordered fix chunks with shallow
review and commits; one focused final re-review checks only those fixes. Optional
improvements remain non-blocking. The clean result is then presented for natural-
language user approval; archive and submission happen only after approval.

## Risks / Trade-offs

- **Concurrent rule edits can collide in position values** -> Keep transactional
  local moves, deterministic `(position, id)` reads, and accept the documented
  Phase 0 race rather than adding locking/versioning scope.
- **Concurrent transaction/category edits can race classification** -> Recheck
  visible category and use a scoped conditional category update; retain the
  explicitly accepted stale-save risk.
- **A partial attempt can commit before later failure** -> Never overwrite an
  existing category, log partial counts, and tell explicit-run users partial work
  may remain so retry is safe.
- **Worker consumers have different visibility semantics** -> Use distinct routers
  and consumer groups while one composite root owns startup, bounded drain, and
  shutdown ordering.
- **Offset changes across daylight-saving transitions** -> Build range boundaries
  from local calendar dates rather than fixed-hour duration arithmetic and test
  ranges whose two boundaries have different offsets.
- **The event publication seam touches finance persistence and app transport** ->
  Inject one narrow finance interface and expose only the active SQL transaction;
  do not import app packages into finance.

## Migration Plan

1. Commit the independently reviewed OpenSpec plan before implementation.
2. Implement and commit the five ordered phases with their deviation gates.
3. Run `sumweave db-migrate` through the existing bootstrap path to auto-migrate
   `finance_classification_rules` before starting API/worker processes.
4. Complete automated checks, visual review, and clean repeated synthetic E2E.
5. Complete deep final review and focused blocker re-review, then wait for user
   approval before archive/submission.

The new table and endpoints are additive. In early alpha, rollback is code rollback
plus a clean local database reseed when needed; no compatibility migration is
maintained. Events already committed remain harmless because classification is
uncategorized-only and duplicate-safe.

## Open Questions

No product-design question blocks planning. Implementation discoveries must be
resolved at the mandatory post-phase deviation gate rather than deferred across a
phase boundary.
