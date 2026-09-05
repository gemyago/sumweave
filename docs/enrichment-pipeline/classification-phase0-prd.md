# Classification Phase 0

## Goal

Provide a simple, deterministic foundation for transaction classification before introducing more advanced classification strategies.

Phase 0 focuses on user-manageable rules and predictable reclassification behavior. System design and implementation details are intentionally out of scope for this document for now.

## Product requirements

### 1. Manual transactions

- Fully manually entered transactions are not automatically classified when they are created.
- The user is responsible for selecting a category for a manually entered transaction.
- Manual transactions may later be included in an explicit reclassification operation.
- Automatic classification must not silently overwrite a category explicitly selected by the user.

### 2. Bank-synced transactions

- Transactions discovered through automatic bank synchronization are subject to automatic classification.
- Automatic classification applies only to transactions eligible for ordinary category classification.
- Transactions that should be handled by another workflow, such as transfers, reconciliation transactions, or opening balances, must not be treated as ordinary classification candidates.
- If no classification rule matches, the transaction remains unclassified.

### 3. Deterministic classification rules

Phase 0 uses deterministic description-based rules only.

Initially supported rule conditions:

- exact description match
- partial description match / description contains

A rule maps a matching transaction to an existing category.

Example:

```text
Description exactly matches "NETFLIX.COM"
→ Entertainment

Description contains "BIEDRONKA"
→ Groceries
```

Matching should be normalized enough to avoid insignificant differences such as letter casing and surrounding whitespace affecting the result.

Rule evaluation must be deterministic. More specific rules should take precedence over less specific rules; in Phase 0 an exact match takes precedence over a partial match. If multiple rules of the same specificity match, the system must have a stable, user-understandable ordering rather than choosing arbitrarily.

### 4. Rules management UI

The product must provide an initial user-facing rules management UI.

Users must be able to:

- list classification rules
- create a rule
- edit a rule
- delete a rule
- inspect the target category of a rule
- inspect enough rule ordering/priority information to understand which rule wins when multiple rules can match
- explicitly trigger classification or reclassification

### 5. Classification vs reclassification

Normal automatic classification and explicit reclassification are separate product operations.

#### Automatic classification

Automatic classification is primarily intended for newly synchronized bank transactions.

- An unclassified eligible transaction may receive a category when a rule matches.
- A manually categorized transaction must not be overwritten automatically.
- A previously automatically classified transaction is not repeatedly changed just because normal bank synchronization runs again.

#### Explicit reclassification

Users must be able to explicitly re-run classification over existing transactions.

Phase 0 should support at least these scopes:

- unclassified transactions only
- unclassified transactions plus transactions that were previously automatically classified

Reclassification must not overwrite manually assigned categories by default.

## Classification ownership / provenance

The product needs to distinguish, at minimum, between a category assigned explicitly by a user and a category assigned automatically by classification.

This distinction is required so automatic classification and reclassification can preserve explicit user decisions.

The exact persistence model for this provenance is a system-design concern and is intentionally deferred.

## Phase 0 non-goals

The following are not required for Phase 0:

- LLM-based classification
- embeddings or semantic similarity
- confidence scores
- automatic rule generation
- merchant/counterparty normalization beyond what is necessary for deterministic description matching
- automatically creating categories during classification
- silently overwriting manually selected categories
- transfer detection or matching as part of the category classifier

## Potential future extension

Transaction enrichment may later include transfer matching across tracked accounts. It may share classification orchestration and re-run workflows, while remaining separate from category classification.

## Open product questions

The following should be resolved before moving to system design:

1. When a user manually changes a transaction category, should the UI offer to create a classification rule from that transaction, for example: `Always classify descriptions containing "BIEDRONKA" as Groceries`?
2. Should users be able to disable rules without deleting them, or is edit/delete sufficient for Phase 0?
3. How should users control ordering between multiple partial-match rules: explicit numeric priority, drag-and-drop ordering, creation order, or another simple mechanism?
4. Should explicit reclassification ever offer an advanced option to overwrite manually categorized transactions, or should manual assignments remain permanently protected from rule-based reclassification in Phase 0?
