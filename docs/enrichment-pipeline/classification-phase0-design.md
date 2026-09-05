# Classification Phase 0 — system design

Status: draft for discussion. The [Phase 0 PRD](classification-phase0-prd.md)
defines behavior; [Architecture](../ARCHITECTURE.md) remains authoritative.

## Agreed foundation

Classification is a finance service that fetches the tenant's ordered rules
when execution starts, keeps them in memory, and applies the first matching
rule to eligible transactions.

- Add one finance table for classification rules.
- Use appdispatch and the existing worker for background execution.
- React to bank-sync completion events with the same range-based classifier.
- Use existing jobs to show explicit classification lifecycle to the user.
- Keep classified, unmatched, and skipped counts in memory and log them.
- Treat concurrent edits as a recognized Phase 0 risk.

## API shape

Finance paths below are relative to `/api/v1/finance/tenants/{tenantId}`. Apply
existing authentication and tenant-membership checks. Validate rule and category
ownership within that tenant; requester identity comes from authentication.

### Rules management

- `GET /classification-rules` → `200 { "items": [...] }` in evaluation order.
  Optional `categoryId` filters references for category-removal feedback.
- `POST /classification-rules` → `201 { "id": "..." }`. Append a rule.
- `PUT /classification-rules/{ruleId}` → `204`. Replace editable fields and
  preserve its position.
- `DELETE /classification-rules/{ruleId}` → `204`. Delete and close the gap.
- `POST /classification-rules/{ruleId}/move` with
  `{ "direction": "up" }` or `{ "direction": "down" }` → `204`. Swap adjacent
  positions in a normal database transaction. Moving past an edge is a no-op.

Create and edit bodies:

```json
{
  "matchType": "contains",
  "condition": "BIEDRONKA",
  "categoryId": "category-id"
}
```

A listed rule contains `id`, `matchType`, `condition`, `categoryId`, `position`,
`createdAt`, and `updatedAt`. Positions are read-only and one-based. Fetch
category names through the existing category resource and refetch rules after
mutations. All rules are active.

Validate `matchType` as `exact | contains`, a nonblank condition, and an existing
visible category belonging to the tenant. Store the condition with surrounding
whitespace removed and preserve its case. Duplicate rules are allowed; order
still determines the winner.

Use ordinary database transactions for multi-row changes.
Concurrent management edits can race in Phase 0; ordered reads include `id` as
a deterministic tie-breaker if positions collide.

### Category lifecycle and manual assignment

Proposed `DELETE /categories/{categoryId}` → `204` uses the existing logical
hide lifecycle, retaining records referenced by ledger transactions. Check live
rule references before removal, including through internal hide paths.

When blocked, return a documented `409` body:

```json
{
  "code": "category_referenced_by_classification_rules",
  "ruleIds": ["rule-id"]
}
```

This small payload lets the UI explain the reference restriction. Other errors
follow existing app conventions. Concurrent category/rule changes remain part
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

The only new table is `finance_classification_rules`:

- `id`: primary key, using the existing string-ID convention.
- `tenant_id`: required tenant reference.
- `position`: positive integer defining order within the tenant.
- `match_type`: required `exact | contains` value.
- `condition`: required text; reject blank values in domain validation.
- `category_id`: required reference to an existing category.
- `created_at`, `updated_at`: required timestamps.

Index `(tenant_id, position, id)` for ordered reads and
`(tenant_id, category_id)` for reference checks. Validate same-tenant category
ownership in the service. Use the existing GORM auto-migrate path and keep
persistence models separate from domain types.

Append at the end and maintain positions through ordinary CRUD transactions.
Category assignment updates the existing transaction's `category_id` and
`updated_at`.

Commands and events use existing appdispatch storage. Jobs retain
their existing lifecycle metadata. Finance persists rules and ledger category
changes.

## Execution model

```text
Explicit request                   Successful bank sync
       |                                    |
Classification command             Bank-sync-completed event
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
                 Assign categories
                         |
                 Log counts and errors

Explicit request feedback: existing job lifecycle
```

### Explicit command and bank-sync completion event

`finance.classification.explicit.v1` carries `tenantId`, `rangeStart`,
`rangeEndExclusive`, and existing requester metadata. Its observed handler uses
job type `finance.classification` and calls the finance classifier.

After a bank-connection sync succeeds, publish a `BankSyncCompleted` domain event
on `finance.bank-sync-completed.v1`:

```json
{
  "tenantId": "tenant-id",
  "connectionId": "connection-id",
  "rangeStart": "2026-08-07T00:00:00+02:00",
  "rangeEndExclusive": "2026-09-06T00:00:00+02:00",
  "sourceSyncMessageId": "sync-message-id"
}
```

The range describes the resolved transaction window covered by the successful
sync. Use the orchestrator's `TargetWindow` start and exclusive end, including
when the caller lets sync choose its window. These are ledger selection bounds.

Commit the completion event with the connection/schedule success state after
the sync's ledger writes have committed. Use the typed `appevents` adapter over
appdispatch and its transaction-bound publication. A publication failure follows
the existing bank-sync retry policy. Previously committed ledger data remains
intact.

Register an ordinary classification subscriber in consumer group
`finance.classification.v1`. It maps the event's tenant and range directly to
the same classifier call as the explicit handler. Other event subscribers can
react independently through their own consumer groups.

The bank-sync job reports sync completion. Classification failures are handled
by the event subscriber's retry policy and logging. A failed sync leaves its
committed transactions available for a later successful sync or explicit
classification request.

Finance owns matching, category assignment, and the sync-completion fact. The
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
existing behavior; a successful bank sync or explicit request starts a pass.

### Processing an attempt

1. Fetch the tenant's ordered rules once into memory when execution starts.
2. Read scoped transactions in bounded batches, initially 200 rows.
3. Check eligibility, match the current description, and choose the first rule.
4. Update only the category and modification timestamp. Retain a simple
   `category_id IS NULL` write predicate, tenant scope, and eligibility checks.
5. Accumulate counts in memory and log batch summaries and completion/failure.

Use an in-memory keyset cursor ordered by transaction ID
within the effective-date range. This keeps pagination stable as transactions
receive categories. Concurrent inserts/edits can change what an attempt sees.

Use normal short database transactions for writes. Increment classified counts
only after confirmed commits.

Each attempt uses the rules loaded at execution start. Command and event
redeliveries fetch rules again and may use newer rules.

### Eligibility and matching

Eligible rows have no category, are booked, and are ordinary transactions or
refunds. Exclude hidden/deleted rows, transfers, reconciliation, and opening
balances. Both entry points include eligible manual, bank-synced, and CSV
transactions. Date ranges use ledger `effectiveAt`.

The existing model includes `expense` and `income` alongside `regular` and
`refund`. Proposed interpretation: include all four as ordinary classification
candidates, subject to confirmation of the PRD terminology.

Trim surrounding whitespace and compare case-insensitively. Exact matching
compares the whole description; contains matching searches a literal substring.
Preserve internal whitespace and punctuation; blank descriptions match nothing.
Use one matcher for both entry points, without regex or database wildcard rules.

Validate that a chosen category still exists and is visible before assignment.
If it is unavailable, return a finance-owned terminal error and log the partial
attempt counts. Concurrent category removal remains an accepted race.

### Retries, logging, and accepted risks

Use existing dispatch retries, failure mapping, and dead-letter behavior.
Explicit failures use existing failed-job feedback; automatic failures use logs
and ordinary dispatch handling.

A retry starts selection and counting again and reloads rules. Transactions
already categorized by earlier attempts are skipped. Counts describe only that
attempt: classified commits, eligible unmatched rows, and ineligible/skipped
rows encountered. A process crash can lose the latest in-memory counts;
ledger commits remain intact.

Log `tenantId`, `messageId`, `sourceSyncMessageId` where applicable, counts,
elapsed time, and ordinary errors. Log partial counts on handled failures when
available. Keep routine progress logs to identifiers, counts, and timing.

Concurrent transaction edits and stale whole-row saves are a recognized Phase 0
risk. The classifier's null-category predicate provides a basic guard, but a
later stale provider or manual save can still overwrite fields. Description,
eligibility, and category changes can also race between reads and writes.

## Implementation and verification

Expose a small rule-management service and classification service through
`finance.Finance`. Keep matching internal. Add a dedicated rules store and a
focused scoped-read/category-update operation.

Implement rules and matching, then command handling and explicit submission,
then the bank-sync completion event/subscriber and the management/job-feedback UI.

Verify matching order, eligibility, tenant isolation, date boundaries, rule
CRUD, and category reference checks. Verify that rules load once per attempt,
retries reload them and skip existing categories, failed classification leaves
imported data intact, and the UI uses existing job feedback. Verify that the
command and event handlers pass identical range parameters to the same service,
the sync event uses the completed transaction window, and date selection handles
exclusive boundaries and differing offsets across DST. Test the simple
conditional category write.

Use generated Mockery mocks where needed and one shallow migration smoke test.
Bootstrap PostgreSQL before backend tests and run required module checks when
code changes.

Remaining product interpretations: inclusion of `expense`/`income` and logical
hiding as category removal.

## Existing implementation references

- [Transaction model and kinds](../../finance/domain/finance_management.go)
- [Finance persistence models](../../finance/persistence/models.go)
- [Existing transaction writes](../../finance/persistence/transaction_tag_store.go)
- [Category lifecycle](../../finance/service_catalog.go)
- [Bank-sync completion](../../finance/service_bank_sync.go)
- [Resolved sync window](../../finance/internal/providers/sync_orchestrator.go)
- [Typed domain events](../../apps/sumweave/internal/appevents/events.go)
- [Finance command contracts](../../finance/semantic_commands.go)
- [Finance dispatch/job adapter](../../apps/sumweave/internal/financeapp/register.go)
- [Worker composition](../../apps/sumweave/internal/wireup/jobs.go)
- [Jobs HTTP API](../../apps/sumweave/internal/api/http/v1controllers/jobs.go)
