---
type: Reference
title: "Transaction classification"
description: "Ordered category and additive-tag rules, timestamp ranges, and accepted limits."
sources:
  - id: classification-phase0-design
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/docs/enrichment-pipeline/classification-phase0-design.md"
    title: "docs/enrichment-pipeline/classification-phase0-design.md"
  - id: service-classification-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/finance/service_classification.go"
    title: "finance/service_classification.go"
  - id: classification-transaction-store-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/finance/persistence/classification_transaction_store.go"
    title: "finance/persistence/classification_transaction_store.go"
  - id: models-go
    resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/finance/persistence/models.go"
    title: "finance/persistence/models.go"
  - id: documentation-snapshot
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/docs/product/classification.md"
    title: "Reviewed documentation checkpoint before this format migration"
---

# Transaction classification

Phase 0 is implemented. Rules assign a category and optional additive tags to
eligible uncategorized transactions. The same range classifier serves explicit
runs and committed bank-sync windows. [Architecture](../ARCHITECTURE.md) defines
product direction; [transfer matching](transfer-matching.md) is a separate operation.[^documentation-snapshot][^classification-phase0-design][^service-classification-go][^classification-transaction-store-go][^models-go]

Use the Rules screen to create, edit, delete, and move rules. A manual category
assignment can offer optional rule creation from the saved description/category
and tags. Cancelling that form does not undo the assignment. Clearing a category
makes a row eligible for later classification; there is no protected
"keep uncategorized" state or assignment provenance in Phase 0.

## Current behavior

Classification is a finance service that fetches the tenant's ordered rules
when execution starts, keeps them in memory, and applies the first matching
rule to eligible transactions.[^documentation-snapshot]

- Finance owns classification rules and their tag association table.
- Appdispatch and the worker execute background classification.
- Bank-sync window completion invokes the same range classifier.
- Existing jobs show explicit classification lifecycle to the user.
- Classified, unmatched, and skipped counts are logged from memory.
- Concurrent edits are an accepted Phase 0 risk.

## API shape

Finance paths below are relative to `/api/v1/finance/tenants/{tenantId}`. Apply
existing authentication and tenant-membership checks. Validate rule and category
ownership within that tenant; requester identity comes from authentication.[^documentation-snapshot]

### Rules management

- `GET /classification-rules` -> `200 { "items": [...] }` in evaluation order.
  Optional `categoryId` filters references for category-removal feedback.
- `POST /classification-rules` -> `201 { "id": "..." }`. Append a rule.
- `PUT /classification-rules/{ruleId}` -> `204`. Replace editable fields and
  preserve its position.
- `DELETE /classification-rules/{ruleId}` -> `204`. Delete and close the gap.
- `POST /classification-rules/{ruleId}/move` with
  `{ "direction": "up" }` or `{ "direction": "down" }` -> `204`. Swap adjacent
  positions in a normal database transaction. Moving past an edge is a no-op.

Create and edit bodies:

```json
{
  "matchType": "contains",
  "condition": "BIEDRONKA",
  "categoryId": "category-id",
  "tagIds": ["tag-id"]
}
```

Omitted `tagIds` or `[]` means no tags; replacement clears the prior rule tag
associations. Explicit null is invalid. Reads return tag IDs in stable ID order
and use `[]` for category-only rules.

A listed rule contains `id`, `matchType`, `condition`, `categoryId`, `tagIds`,
`position`, `createdAt`, and `updatedAt`. Positions are read-only and one-based. Fetch
category names through the existing category resource and refetch rules after
mutations. All rules are active.

Validate `matchType` as `exact | contains`, a nonblank condition, an existing
visible category, and optional existing visible tags belonging to the tenant.
Reject blank or duplicate tag IDs. Store the condition with surrounding whitespace
removed and preserve its case. Duplicate rules are allowed; order still determines
the winner.

Use ordinary database transactions for multi-row changes.
Concurrent management edits can race in Phase 0; ordered reads include `id` as
a deterministic tie-breaker if positions collide.

### Category lifecycle and manual assignment

`DELETE /categories/{categoryId}` -> `204` uses the existing logical
hide lifecycle, retaining records referenced by ledger transactions. Check live
rule references before removal, including through internal hide paths.

When blocked, the specialized `409` body contains:

```json
{
  "code": "category_referenced_by_classification_rules",
  "ruleIds": ["rule-id"]
}
```

This small payload lets the UI explain the reference restriction. Other errors
follow existing app conventions. Live rule tag references similarly block tag hiding. Deleting or changing a rule
does not clear previously assigned transaction tags. Concurrent category/rule changes remain part
of the accepted concurrency risk.

Saving a manual category and creating a rule are separate requests. Offer the
prefilled rule form after category assignment succeeds. Cancelling or failing
rule creation leaves that assignment intact.

### Explicit classification

`POST /transactions/classify` accepts:

```json
{
  "rangeStart": "2026-08-07T00:00:00+02:00",
  "rangeEndExclusive": "2026-09-06T00:00:00+02:00"
}
```

Both fields are full ISO 8601 timestamps in RFC 3339 format, including an offset
or `Z`. Validate them and require `rangeStart < rangeEndExclusive`.

The UI defaults to today and the preceding 29 calendar days in the ledger
viewer's local time, showing both selected dates before submission. It computes
the start of the first day and the start of the day after the last selected day,
using calendar arithmetic so each boundary has the correct offset across DST.
The example includes all of September 5. Wider ranges are allowed.

The backend passes these instants directly to the command and classifier.
Select `effective_at >= rangeStart AND effective_at < rangeEndExclusive`.

Publish a classification command and return `202`:

```json
{ "jobId": "dispatch-message-id" }
```

The UI polls the existing `/api/v1/jobs/{jobId}` endpoint and uses its
`queued | running | succeeded | failed` lifecycle and existing safe errors.
The job is created on first delivery; only the initiating flow treats an initial
`404` for its returned job ID as pending, following existing behavior.

User feedback is lifecycle status, with a failure message explaining that some
transactions may already have been classified. Log counts for diagnostics and
refresh the ledger when processing finishes. A retry is another request;
already categorized transactions are skipped.

## Database schema shape

Classification uses `finance_classification_rules` and its finance-owned
`finance_classification_rule_tags` association:[^documentation-snapshot]

- `id`: primary key, using the existing string-ID convention.
- `tenant_id`: required tenant reference.
- `position`: positive integer defining order within the tenant.
- `match_type`: required `exact | contains` value.
- `condition`: required text; reject blank values in domain validation.
- `category_id`: required reference to an existing category.
- `created_at`, `updated_at`: required timestamps.

`finance_classification_rule_tags` has composite primary key `(rule_id, tag_id)`
and reverse index `(tag_id, rule_id)`. It has no timestamps or tenant column;
the referenced rule establishes tenant ownership. Rule create, replacement, and
deletion update these associations in the rule transaction.

Index `(tenant_id, position, id)` for ordered reads and
`(tenant_id, category_id)` for reference checks. Validate same-tenant category
ownership in the service. Use the existing GORM auto-migrate path and keep
persistence models separate from domain types.

Append at the end and maintain positions through ordinary CRUD transactions.
Classification assignment conditionally updates the existing transaction's
`category_id` and `updated_at`, then conflict-ignores inserts of the winning
rule's tags into `finance_transaction_tags` in the same short transaction. It
never clears existing transaction tags or saves a stale whole transaction.

Commands and events use existing appdispatch storage. Jobs retain
their existing lifecycle metadata. Finance persists rules and ledger category
changes.[^documentation-snapshot]

## Execution model

```text
Explicit request                   Committed bank-sync window
       |                                    |
Classification command             Window-completed event
       |                                    |
       +------------ Appdispatch -----------+
       |                                    |
Job-observed handler               Classification subscriber
       |                                    |
       +-----------------+------------------+
                         |
       Classify(tenantId, rangeStart, rangeEndExclusive)
                         |
                 Load rules into memory
                         |
                 Read ledger in batches
                         |
             Validate category and tags
                          |
             Assign category and missing tags
                         |
                 Log counts and errors

Explicit request feedback: existing job lifecycle
```

### Explicit command and bank-sync window completion event

`finance.classification.explicit.v1` carries `tenantId`, `rangeStart`,
`rangeEndExclusive`, and existing requester metadata. Its observed handler uses
job type `finance.classification` and calls the finance classifier.

For each successfully committed bank-sync window, publish a
`BankSyncWindowCompleted` domain event on `finance.bank-sync-window-completed.v1`:

```json
{
  "tenantId": "tenant-id",
  "connectionId": "connection-id",
  "rangeStart": "2026-08-07T00:00:00+02:00",
  "rangeEndExclusive": "2026-09-06T00:00:00+02:00",
  "sourceSyncMessageId": "sync-message-id"
}
```

The range is the start and exclusive end of the individual requested window
whose successful checkpoint is committed with the ledger writes. These are
ledger selection bounds, including when sync chooses and splits its target
range into multiple windows.

Commit the window's ledger writes, successful checkpoint, and completion event
in one database transaction. Use the typed `appevents` adapter over appdispatch
and its transaction-bound publication. A publication failure rolls back that
window's writes and successful checkpoint and follows the existing bank-sync
retry policy. Previously committed windows and their events remain intact.

Each committed window triggers classification even if a later window fails or
the process crashes before the overall connection/schedule completion is saved.
A retry may resume at a later window: earlier committed windows already have
durable events. Classification can begin before the overall sync finishes.
Duplicate event deliveries use the same uncategorized-only classifier behavior.

The ordinary classification subscriber is registered in consumer group
`finance.classification.v1`. It maps the event's tenant and range directly to
the same classifier call as the explicit handler. Other event subscribers can
react independently through their own consumer groups.

The bank-sync job reports the overall sync outcome. Classification failures are
handled by the event subscriber's retry policy and logging; they preserve the
window's committed imports and do not change the bank-sync job outcome.

Finance owns matching, category assignment, and the window-completion fact. The
app owns typed event publication, subscriptions, and job adapters. Inject the
completion publisher through a narrow finance interface implemented by the app;
register the subscriber in the existing worker lifecycle.

### Shared classification contract

Both handlers invoke `Classify` with one parameter object containing `tenantId`,
`rangeStart`, and `rangeEndExclusive`. The classifier selects all eligible
transactions in that tenant and effective-date range, applying the same rules
and processing steps for either trigger. Event connection and message references
provide logging context.

The range may include eligible manual, bank-synced, and CSV transactions across
the tenant's accounts. Manual creation, CSV import, and rule edits keep their
existing behavior; a committed bank-sync window or explicit request starts a pass.

### Processing an attempt

1. Fetch the tenant's ordered rules once into memory when execution starts.
2. Read eligible scoped transactions in bounded batches, initially 200 rows.
3. Recheck eligibility, match the current description, and choose the first rule.
4. Validate the winning rule's category and tags are still visible and owned by
   the tenant. An unavailable category or tag is terminal; an operational lookup
   error remains retryable.
5. In one short transaction, conditionally update only the category and
   modification timestamp, then conflict-ignore the winning rule's tag inserts.
   Retain a `category_id IS NULL` predicate, tenant scope, and eligibility checks.
   If the conditional update affects no row, insert no tags.
6. Accumulate counts in memory and log batch summaries and completion/failure.

Use an in-memory keyset cursor ordered by transaction ID
within the effective-date range. This keeps pagination stable as transactions
receive categories. Concurrent inserts/edits can change what an attempt sees.

Use normal short database transactions for writes. Increment classified counts
only after confirmed commits.

Each attempt uses the rules loaded at execution start. Command and event
redeliveries fetch rules again and may use newer rules.

### Eligibility and matching

Eligible rows have no category, are booked, and have kind `regular`, `expense`,
`income`, or `refund`. These four kinds represent ordinary transactions and
refunds for classification. Exclude hidden/deleted rows, transfers,
reconciliation, and opening balances. Both entry points include eligible manual,
bank-synced, and CSV transactions. Date ranges use ledger `effectiveAt`.

Trim surrounding whitespace and compare case-insensitively. Exact matching
compares the whole description; contains matching searches a literal substring.
Preserve internal whitespace and punctuation; blank descriptions match nothing.
Use one matcher for both entry points, without regex or database wildcard rules.

Validate that a chosen category and every winning-rule tag still exist, are visible,
and belong to the tenant before assignment. An unavailable category returns the
existing terminal error; an unavailable tag returns terminal code
`classification_tag_unavailable`. Operational lookup failures remain ordinary
errors for dispatch retry. Concurrent catalog removal remains an accepted race.

### Retries, logging, and accepted risks

Use existing dispatch retries, failure mapping, and dead-letter behavior.
Explicit failures use existing failed-job feedback; automatic failures use logs
and ordinary dispatch handling.

A retry starts selection and counting again and reloads rules. Transactions
already categorized by earlier attempts are excluded from selection. Counts
describe only rows processed during that attempt:

- `classified`: category-and-additive-tag assignments with confirmed commits,
  counted once per transaction regardless of tag count.
- `unmatched`: eligible rows for which no rule matches.
- `skipped`: selected rows skipped during processing because an eligibility
  recheck fails or a conditional category update affects no row.

Rows filtered out by selection are excluded from all three counts. These are
diagnostic attempt counts, not totals for every transaction in the range; do not
scan or count excluded rows separately. Failed or rolled-back assignments do
not increment `classified`. A process crash can lose the latest in-memory
counts; ledger commits remain intact.

Log `tenantId`, `messageId`, `sourceSyncMessageId` where applicable, counts,
elapsed time, and ordinary errors. Log partial counts on handled failures when
available. Keep routine progress logs to identifiers, counts, and timing.

Concurrent transaction edits and stale whole-row saves are a recognized Phase 0
risk. The classifier's null-category predicate provides a basic guard, but a
later stale provider or manual save can still overwrite fields. Description,
eligibility, and category changes can also race between reads and writes.

## Verification and formal contracts

The [classification scenarios](../requirements/transaction-classification.md)
and [operator UI scenarios](../requirements/finance-operator-ui.md)
are acceptance requirements within this bundle. The
[synthetic API flow](../manual-e2e/synthetic-provider-flow-e2e.md) and
[UI flow](../manual-e2e/synthetic-provider-ui-e2e.md) are reproducible runbooks,
not proof that a run has completed.[^documentation-snapshot]

The accepted [shared safe error contract](../integrations/direct-access.md#shared-errors)
requires a JSON envelope. Ordinary blank or duplicate rule-tag validation returns
`400 invalid_request`, inaccessible rule tags return `404 not_found`, and tenant
denial returns `403 tenant_access_denied`. The specialized category-reference
`409` body and no-publication/no-mutation guarantees remain.

There is a current implementation gap on rule create and replacement: explicit
`tagIds: null` is rejected with an empty `400` before generated binding rather
than the required envelope. A non-size-limit request-body read failure in the
same wrapper also returns an empty `400`; body-size-limit failures take the
separate `413` path. These early rejections do not call the rule service or
publish work. Authentication precedes the wrapper; normal tenant checks remain
in the service. The [classification spec](../requirements/transaction-classification.md)
keeps the accepted error contract and records the gap.

Verify matching order, eligibility, tenant isolation, date boundaries, rule
CRUD, all four eligible transaction kinds, and category reference checks through
both the removal API and internal hide paths. Verify that rules load once per
attempt, retries reload them and skip existing categories, failed classification leaves
imported data intact, and the UI uses existing job feedback. Verify that the
command and event handlers pass identical range parameters to the same service,
each sync event uses its committed window's bounds, and date selection handles
exclusive boundaries and differing offsets across DST. Test the simple
conditional category-plus-additive-tags write, including overlap, category-only,
and rollback behavior, and the distinction between rows excluded by selection and
rows counted as skipped during processing. Verify unavailable tag terminal errors,
operational tag lookup retries, and first-rule-only tags.

Verify that an early committed window remains scheduled for classification when
a later window fails and retry resumes beyond it. Cover a crash after a window
commit but before overall sync completion, publication failure rolling back the
window's writes and successful checkpoint, and duplicate event delivery skipping
existing categories.

Use generated Mockery mocks where needed and one shallow migration smoke test.
Bootstrap PostgreSQL before backend tests and run required module checks when
code changes.

## Existing implementation references

Requirements below are in this bundle; repository implementation and test references are external.[^documentation-snapshot]

- [Classifier](../../finance/service_classification.go)
- [Conditional category and additive-tag write](../../finance/persistence/classification_transaction_store.go)

- [Transaction model and kinds](../../finance/domain/finance_management.go)
- [Finance persistence models](../../finance/persistence/models.go)
- [Existing transaction writes](../../finance/persistence/transaction_tag_store.go)
- [Category lifecycle](../../finance/service_catalog.go)
- [Bank-sync completion](../../finance/service_bank_sync.go)
- [Resolved sync window](../../finance/internal/providers/sync_orchestrator.go)
- [Window writes and checkpoint commit](../../finance/internal/providers/window_sync_store.go)
- [Typed domain events](../../apps/sumweave/internal/appevents/events.go)
- [Finance command contracts](../../finance/semantic_commands.go)
- [Finance dispatch/job adapter](../../apps/sumweave/internal/financeapp/register.go)
- [Worker composition](../../apps/sumweave/internal/wireup/jobs.go)
- [Jobs HTTP API](../../apps/sumweave/internal/api/http/v1controllers/jobs.go)

[^documentation-snapshot]: Reviewed documentation checkpoint, preserved as the source of this section; format and navigation changes do not establish execution results.

[^classification-phase0-design]: docs/enrichment-pipeline/classification-phase0-design.md.
[^service-classification-go]: finance/service_classification.go.
[^classification-transaction-store-go]: finance/persistence/classification_transaction_store.go.
[^models-go]: finance/persistence/models.go.
