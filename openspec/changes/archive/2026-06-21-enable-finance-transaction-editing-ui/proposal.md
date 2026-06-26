## Why

The finance product direction already expects focused transaction workflows, but the current implementation mixes manual transaction creation directly into the list screen and still leaves existing transactions effectively read-only from the UI. That combination is awkward on desktop, even worse on mobile, and makes the transaction workspace feel like an overloaded form-plus-list instead of a clear browse flow with dedicated create/edit entry points.

## What Changes

- Move manual transaction creation off the transactions list screen into a dedicated transaction editor screen that is optimized for mobile-friendly single-record entry.
- Reuse that same dedicated transaction editor screen for existing transaction edits so create and edit follow one consistent workflow.
- Keep the transactions list focused on filtering, sorting, state cues, and navigation into create/edit flows instead of embedding the creation form directly in the list page.
- Expose finance transaction detail and update API paths in the backend app so the dedicated editor can load one transaction directly, map to the existing finance-domain behavior, and keep tenant isolation intact.
- Show provider-origin context for synced transactions so user-edited reporting fields remain distinguishable from provider-original values and later edits do not imply silent provider overwrites.
- Keep transaction state cues such as pending, hidden, transfer, refund, and reconciliation visible while creating or editing so operators understand the reporting impact of the record they are changing.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Extend the finance transaction API contract so tenant-scoped transaction edits are supported for user-editable ledger fields while preserving provider-original values separately.
- `finance-operator-ui`: Strengthen the finance transactions workflow so operators browse transactions on one screen, create transactions on a dedicated mobile-friendly editor screen, and reuse that same screen for editing existing transactions.

## Impact

- Affects `finance/`, especially transaction update validation, provider-original preservation rules, and focused finance service tests.
- Affects `apps/signal-foundry/`, especially finance HTTP routes/controllers/models, OpenAPI surface, and API/controller tests for tenant-scoped transaction updates.
- Affects `apps/signal-ui/`, especially finance transaction API helpers, finance transaction routes/components/tests, responsive form layout, and `ui-wireframe.md` documentation for the transaction workflow.
- Updates OpenSpec finance specs so dedicated create/edit transaction screens are explicit in the finance operator UI acceptance criteria and transaction editing remains explicit in finance management.
