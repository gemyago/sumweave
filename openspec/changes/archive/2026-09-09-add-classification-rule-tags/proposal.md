## Why

Tags are already a tenant-owned reporting dimension, but classification rules can assign only a category. Recurring transactions therefore still require repeated manual tagging even when their category is predictable.

The optional rule-creation flow also expands a large form immediately after a manual category save and snapshots values too early. This interrupts operators who only wanted to classify the transaction and prevents later saved tag or description edits from becoming useful rule defaults.

## What Changes

- Extend each classification rule with zero or more existing, visible tags from the same tenant while keeping its category required.
- Apply the first matching rule's category and missing tags in one atomic assignment, preserving existing transaction tags and the current uncategorized-only eligibility rules.
- Validate rule tag references throughout rule management and classification, and block internal tag hiding while a live rule references the tag.
- Replace the automatically expanded post-category rule form with a compact, dismissible action. Keep that action available through successful description and tag edits, then initialize an editable rule draft from the latest saved transaction values only when the operator opens it.
- Show and edit optional rule tags in Rules management and in transaction-derived rule editors across shared transaction lists and the full transaction editor.
- Preserve current rule ordering, first-match precedence, automatic and explicit classification triggers, jobs feedback, and retry behavior. Tag-only rules, tagging already categorized transactions, and reclassification remain outside this iteration.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `transaction-classification`: Optional rule tags, atomic additive assignment, tenant/tag validation, and tag-aware failure and retry behavior.
- `finance-management`: Protection against hiding tags referenced by live classification rules while retaining historical transaction assignments.
- `finance-operator-ui`: Compact persistent rule offers, latest-saved-value prefilling, and rule tag display and editing.

## Impact

- `finance/`: Rule domain/service/store behavior, a new GORM-managed rule-to-tag association table, atomic classifier assignment persistence, catalog tag-hide guards, dependency wiring, and associated tests.
- `apps/sumweave/`: Classification-rule OpenAPI request/read schemas, generated models and validation, controller mapping, and registered-route tests.
- `apps/sumweave-ui/`: Finance API types/client, shared transaction list, full editor, rule creation form, Rules page, and behavioral tests.
- Classification product/design documentation, UI wireframe, and relevant manual E2E guidance will be updated during implementation.
- No new dependencies, background job types, topics, or runtime module changes are required.
