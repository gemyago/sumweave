## Context

`finance/domain/finance_management.go` defines category-only classification rules. The classifier loads the ordered list once per attempt and uses `ClassificationTransactionStore` to conditionally update an eligible transaction's category. Eligibility requires a visible, booked, uncategorized ordinary transaction. Transaction tags already use `finance_transaction_tags` and are editable in the UI.

`FinanceTransactionList.svelte` and `FinanceTransactionEditor.svelte` currently create a description/category snapshot immediately after a successful category save and render `FinanceRuleCreationForm.svelte` in full. Later tag saves do not update that snapshot. The shared list serves the ledger, dashboard, and account activity.

This iteration remains within the finance module, application API, and Svelte UI. Finance owns persistence and business behavior; existing observed commands and bank-window events continue to call the same classifier.

## Goals / Non-Goals

**Goals:**

- Rules assign one required category and zero or more existing tenant tags.
- Classification adds missing rule tags and preserves existing transaction tags.
- Category and tag assignment commit together for each eligible transaction.
- Manual category saves offer a small dismissible action that survives subsequent tag edits.
- Explicitly opening that action captures the latest saved transaction values into an editable rule draft.
- Rules management displays and edits tags with the existing catalog controls and Bootstrap conventions.

**Non-Goals:**

- Tag-only rules, applying multiple matching rules, or adding tags to already categorized transactions.
- Reclassification, removal of previously assigned tags when rules change, or assignment provenance.
- New tag creation inside the rule form, tag-removal UI/API, or new job infrastructure.
- New locking or concurrency policies beyond existing Phase 0 behavior.

## Decisions

### 1. Store rule tags as a finance-owned association

Add `TagIDs []string` to `domain.ClassificationRule` and create/update service parameters. Retain the required `CategoryID` and current ordering and matching fields.

Add exactly one GORM-auto-migrated table, `finance_classification_rule_tags`:

- `rule_id VARCHAR(255) NOT NULL`, first column of composite primary key.
- `tag_id VARCHAR(255) NOT NULL`, second column of composite primary key.
- Reverse lookup index `idx_finance_classification_rule_tags_tag_id` on `(tag_id, rule_id)` for hide protection.

Follow the existing explicit association-model style of `finance_transaction_tags`; tenant ownership comes from the referenced rule and is validated against each tag. No columns change on `finance_classification_rules`, `finance_transactions`, or `finance_transaction_tags`. No provenance, timestamps, or ordering are added to the association table.

Extend the dedicated rule store to load associations in a batch for the selected rules and return tag IDs in stable ID order. Create and replacement save the rule plus its complete tag set in one database transaction; replacement with an empty set clears rule associations. Deletion removes associations and closes the rule-position gap in the existing delete transaction. Reordering leaves associations intact. Do not add methods to the legacy `Store` for rule operations.

### 2. Validate tags and protect live references

Use a consumer-defined tag lookup dependency for rule-service validation. Reject duplicate/blank IDs and tags that are missing, hidden, or belong to a different tenant; reuse existing domain error conventions. Required dependencies are checked in constructors and wired in `finance.go`; regenerate Mockery mocks for changed interfaces.

Add rule-tag reference lookup to the dedicated rule store. `CatalogService.HideTag` rejects a referenced tag with `ErrTagReferencedByClassificationRules` and the referencing rule IDs. Mirror the existing category guard in the persistence `SaveTag` hide path so direct internal saves cannot bypass protection. Preserve existing tag assignments when hiding an unreferenced tag. There is no public tag-hide endpoint or UI to extend in this iteration.

Loaded rules can outlive deletion or retargeting during an attempt, so classification still validates their tags before assignment. A missing/hidden/wrong-tenant tag produces a finance terminal failure with code `classification_tag_unavailable`; an operational lookup failure remains an ordinary wrapped error for dispatch retries. Failed validation must not commit any part of that transaction's classification.

### 3. Apply category and tags as one conditional write

Replace the category-only assignment operation with a dedicated classification assignment accepting tenant ID, transaction ID, category ID, tag IDs, and update time through a params struct.

For each first matching rule:

1. Resolve and validate its required category and optional tags.
2. Begin a short database transaction and perform the existing conditional eligible-row category/updated-time update.
3. If no eligible row was updated, return skipped without inserting any tags.
4. Insert the rule tag associations into `finance_transaction_tags` with conflict-ignore on its existing composite key. Never clear existing assignments or save a stale whole transaction record.
5. Commit and increment `Classified` once for the transaction, irrespective of tag count. Any insertion or commit failure rolls back the category and tag writes together.

This preserves the current first-match policy and uncategorized-only selection for automatic and explicit runs. A retry excludes a transaction whose category and tags already committed; partial category-only success cannot strand the tag write. Rule editing/deletion never modifies historical transaction assignments.

### 4. Extend the existing rule API contract

In `apps/sumweave/internal/api/http/v1routes.yaml`:

- Add optional `tagIds` (array of unique nonblank string IDs) to `FinanceClassificationRuleRequest`. Omission and `[]` both mean no rule tags, including full replacement via PUT. Reject explicit null rather than treating it as a separate state.
- Add required `tagIds` to the `FinanceClassificationRule` read model; return `[]` for category-only rules.
- Preserve the POST generated-ID-only response and existing empty success responses for updates/deletes.
- Map tag errors through the existing finance response conventions: duplicate/unassignable tags to empty `400`, missing/cross-tenant tags to empty `404`, and membership denial to the existing empty `401`.

Regenerate backend models and validation using the application generation command. Update controller mappings and the hand-maintained Finance TypeScript API definitions/client. The generic agent API schema does not change.

### 5. Make the rule offer a compact local interaction

Represent each offer by an offer ID and source transaction ID, with a collapsed or editing phase. Keep one active offer per list/editor, matching the current scope. A successful category assignment starts a new collapsed offer; a later category assignment replaces it, including any prior draft. Use the shared list's saved transaction data or the full editor's saved `transaction` object as the source of defaults.

```text
category save succeeds
        |
        v
Create rule from this transaction   [Dismiss]
        |                         |
tag/description save              Dismiss
        |                         |
        v                         v
same compact action              no offer
        |
click action
        |
        v
editable rule draft from latest saved description + category + tags
        |
explicit Save rule / Cancel rule
```

- Show the compact text action below the transaction fields or full-editor save feedback, without a surrounding large panel, automatic expansion, autofocus, or scrolling.
- Tag and description edits/saves on the source transaction preserve the collapsed offer. Removing tags also updates the eventual defaults. A failed transaction save does not advance saved defaults.
- Clicking the action creates a fresh draft with `contains`, the latest saved description, required category, and all saved tag IDs. Pending transaction saves disable opening so the click cannot race a save.
- Once expanded, the rule draft is independent: rerenders and tag/description saves must not overwrite operator edits. Only a new category-assignment offer replaces it. The full editor's unsaved transaction form is never used as a rule source.
- Dismiss, cancel, and successful rule creation clear the offer. Subsequent tag-only or description-only saves do not resurrect it. A new successful category assignment can offer again. Clearing the category clears the offer/draft.
- The offer is local to its source transaction and tenant. Clear it when the tenant changes, the source leaves the rendered results, or the editor navigates to another record. No persistence across navigation or pages is introduced.
- Keep category-only rule creation available. Add an optional existing-tag checkbox group to both the transaction-derived form and Rules add/edit form; list target tag names as compact badges on rule rows.
- Catalog failures remain recoverable. Do not silently drop selected tags when catalog data is missing: preserve IDs, show unavailable selections, and require refresh or explicit removal before saving an invalid draft. An empty successfully loaded catalog is valid for category-only rules.
- Use native buttons/checkboxes and existing Bootstrap classes. Preserve keyboard focus on category/tag controls during offer appearance; opening the form may focus its first field with `preventScroll`. Keep narrow-screen touch targets and wrapping consistent with the shared transaction editor.

### 6. Verify behavior in the owning modules

Use TDD for rule/service/store/API changes, with randomized fixtures and Mockery mocks. Exercise persisted rule tag replacement/deletion, tenant validation, internal hide protection, additive assignment with overlapping tags, rollback on assignment failure, unchanged eligibility, first-match behavior, and retries. Keep migration verification to one shallow smoke test and assert persisted behavior rather than SQL strings.

UI behavioral tests cover category save -> compact action -> tag save -> open -> rule submission, latest saved description/tags, empty tag selection, dismissal, replacement, clearing, catalog errors, and recoverable rule-save failure. Cover both the shared list and full editor, plus Rules add/edit/list flows, without duplicating every permutation across all host pages.

During implementation, update the classification PRD/design, UI wireframe, and relevant E2E guidance alongside the owning work. Run the repository completion protocol, including PostgreSQL bootstrap before backend tests and affected lint/tests. Browser verification follows `docs/manual-e2e/README.md` and `finance-ui-shell-smoke-e2e.md`, exercising the changed flow on desktop and a narrow viewport. The UI module's required review-agent/fix/review loop is part of implementation completion. No user-operated setup or approval step is required by this design; if actual environment access blocks that verification, surface the concrete blocker for user resolution.

## Risks / Trade-offs

- Existing categorized transactions will not acquire newly configured rule tags. The rules page's classification copy must make the uncategorized-only scope clear.
- Ordinary transaction edits replace the selected tag set, while classification adds missing tags. Keep this distinction explicit in rule-form copy and the behavioral tests.
- Concurrent manual edits and catalog/rule changes remain an accepted Phase 0 risk. Conditional writes, atomic assignment, and pre-assignment validation cover the existing sequential guarantees; this iteration does not add provenance or locks.
- Only the newest offer remains active within a list/editor. Keep it attached to its transaction and reset on scope changes to avoid cross-record defaults.

## Migration Plan

Register the association model in finance's existing GORM migrator. Use the normal PostgreSQL bootstrap/migration workflow before restarting API and worker processes. Existing category-only rules have no association rows and read as `tagIds: []`; no backfill or reseed is needed. Ship the updated API, worker, and UI together through the existing release workflow. No custom SQL migration or backward-compatibility/rollback implementation is planned for this early-alpha change.

## Open Questions

None blocking. This proposal chooses required categories, additive optional tags, current uncategorized-only eligibility, and a compact offer whose draft is captured only when opened.
