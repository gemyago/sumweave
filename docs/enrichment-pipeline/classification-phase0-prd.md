# Classification Phase 0

## Goal

Provide a simple, deterministic foundation for transaction classification before introducing more advanced classification strategies.

Phase 0 focuses on user-manageable rules and predictable classification behavior. System design and implementation details are intentionally out of scope for this document for now.

The governing rule is simple: a transaction with a category is already classified and must be skipped, regardless of how that category was assigned. Phase 0 only assigns categories to transactions that currently have none.

## Product requirements

### 1. Manual transactions

- Fully manually entered transactions are not automatically classified when they are created.
- The user is responsible for selecting a category for a manually entered transaction.
- Uncategorized manual transactions may later be included in an explicit classification operation.
- Neither automatic nor explicit classification may overwrite an existing category.

### 2. Bank-synced transactions

- Transactions discovered through scheduled or user-triggered bank synchronization are subject to automatic classification.
- Automatic classification applies only to transactions eligible for ordinary category classification.
- If no classification rule matches, the transaction remains unclassified.

### 3. Transaction eligibility

The following eligibility requirements apply to both automatic and explicit classification:

- The transaction has no category.
- The transaction is booked/settled and is either a regular transaction or a refund.
- Hidden or deleted transactions are excluded.
- Pending/hold transactions are excluded; a bank-synced transaction becomes eligible when it is booked/settled, provided it still has no category.
- Transfers, including matched internal and unmatched/external transfers, are excluded from Phase 0 category classification. They may still be categorized manually where applicable.
- Reconciliation and opening-balance transactions are excluded.

CSV-imported transactions with a category are already classified. Uncategorized CSV-imported transactions may be included in an explicit classification operation; CSV import does not trigger automatic classification in Phase 0.

### 4. Deterministic classification rules

Phase 0 uses deterministic description-based rules only.

Initially supported rule conditions:

- exact description match
- partial description match / description contains

A rule maps a matching transaction to an existing category. Rules belong to one finance tenant, apply only to that tenant's transactions, and may target only categories belonging to that same tenant. Tenant members manage the shared rules using the existing tenant membership model.

Example:

```text
Description exactly matches "NETFLIX.COM"
→ Entertainment

Description contains "BIEDRONKA"
→ Groceries
```

Matching uses the transaction's current user-visible description, including any user edits, rather than a separate provider-original description.

Matching behavior:

- Compare descriptions and rule conditions case-insensitively, ignoring surrounding whitespace on both sides.
- Preserve internal whitespace and punctuation: `NETFLIX.COM` and `NETFLIX COM` are different descriptions.
- Exact match compares the whole description after this normalization.
- Contains match searches for a literal substring; there are no regular expressions, wildcards, or whole-word requirements.
- Reject empty or whitespace-only rule conditions. Empty or whitespace-only descriptions match no rule.

For example, `  netflix.com  ` matches an exact condition of `NETFLIX.COM`, while `NETFLIX.COM PAYMENT` only matches a contains condition of `NETFLIX.COM`.

Rules form one user-controlled ordered list. Evaluate them from top to bottom and apply the first matching rule. List position determines precedence for both exact and contains conditions.

### 5. Rules management UI

The product must provide an initial user-facing rules management UI.

Users must be able to:

- list classification rules
- create a rule
- edit a rule
- delete a rule
- inspect the target category of a rule
- inspect and change rule order using move-up/down controls
- create a rule from a manual transaction category assignment
- explicitly trigger classification of existing uncategorized transactions

New rules are appended to the end of the list and can then be moved up or down. All rules are active; Phase 0 supports editing and deleting rules, with no disable option.

When a user manually assigns or changes a transaction category, offer an optional action to create a rule from that transaction. Prefill the current description and selected category, and let the user choose exact or contains matching and edit the condition before saving. For example: `Always classify descriptions containing "BIEDRONKA" as Groceries`. Declining or cancelling rule creation leaves the category assignment intact. Creating the rule requires an explicit save and follows the same ordering and lifecycle behavior as any other rule.

### 6. Automatic and explicit classification

Automatic classification and explicit classification of existing transactions are separate product operations. Both skip every transaction that already has a category. There is no operation to overwrite or clear an existing category through rules in Phase 0.

#### Automatic classification

Automatic classification is primarily intended for newly synchronized bank transactions.

- An unclassified eligible transaction may receive a category when a rule matches.
- Any categorized transaction is skipped, whether its category was assigned manually, imported, or assigned automatically.
- Repeated bank synchronization must not change an existing category.
- A bank-synced transaction first observed as pending may be classified when it becomes booked/settled.
- A classification failure must preserve transactions already imported successfully.

#### Explicit classification

Users must be able to explicitly run classification over existing uncategorized transactions in the current tenant, including eligible manual, bank-synced, and CSV-imported transactions.

- Default the date range to the last 30 days, including today and the preceding 29 days, showing the actual start and end dates before the user starts the operation.
- Allow the user to choose a different date range, including a wider historical range.
- Apply the range to the transaction date shown in the ledger, including both selected boundary dates.
- Reject an invalid range whose start date is after its end date.
- Apply the same eligibility requirements throughout the selected range; choosing a wider range never permits overwriting a category.
- Show completion or failure feedback and counts of newly classified, unmatched, and skipped transactions. If processing fails after partial progress, make that partial outcome clear.

Classification must preserve a category assigned by a user while an operation is running. Retrying an operation skips transactions that already received a category, including those classified by a previous attempt.

### 7. Rule and category lifecycle

- Creating, editing, reordering, or deleting a rule does not change existing transaction categories or automatically start a historical classification run.
- Rule changes apply to subsequent classification operations, which still target only eligible uncategorized transactions.
- A category referenced by any classification rule cannot be deleted. The UI must explain which rules reference it; users must retarget or delete those rules before removing the category.
- Deleting a rule does not clear categories previously assigned by that rule.

## Classification ownership / provenance

Phase 0 does not require category-assignment provenance to decide eligibility: the presence of a category is sufficient to protect it. Distinguishing manual and automatic assignments for future reclassification is deferred.

If a user manually clears a category, the transaction becomes uncategorized and may be classified by a subsequent applicable operation. Phase 0 has no separate protected "keep uncategorized" state.

## Acceptance scenarios

- The first matching rule in the visible list wins, including when a contains rule appears before a matching exact rule. Move-up/down controls change that order.
- Matching ignores casing and surrounding whitespace, but preserves internal whitespace and punctuation and treats contains conditions literally.
- An existing category survives repeated syncs, explicit runs, and rule edits or deletion, regardless of how it was assigned.
- A pending transaction is skipped; once booked, it can receive a category if still uncategorized. A booked refund is eligible, while hidden/deleted transactions, transfers, reconciliations, and opening balances are skipped.
- Manual and CSV-imported uncategorized transactions can be classified in an explicit run; their existing categories are preserved.
- An explicit run defaults to the last 30 days, including today, and touches only eligible transactions within the chosen inclusive dates and current tenant.
- A manual category assignment offers optional rule creation with editable description, match type, and category; cancelling leaves the assignment intact, while saving appends the rule to the list.
- A user assigning a category while classification is running keeps that assignment, including after a retry.
- Deleting a referenced category is blocked until its rules are retargeted or deleted.

## Phase 0 non-goals

The following are not required for Phase 0:

- LLM-based classification
- embeddings or semantic similarity
- confidence scores
- automatic rule generation
- disabling rules
- merchant/counterparty normalization beyond what is necessary for deterministic description matching
- automatically creating categories during classification
- overwriting or clearing any existing category through classification rules
- category-assignment provenance or a protected uncategorized state
- transfer detection or matching as part of the category classifier

## Potential future extension

Transaction enrichment may later include transfer matching across tracked accounts. It may share classification orchestration and explicit-run workflows, while remaining separate from category classification. Reclassification of already categorized transactions may be considered in a later phase.
