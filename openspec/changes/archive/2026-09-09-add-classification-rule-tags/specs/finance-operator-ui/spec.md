## MODIFIED Requirements

### Requirement: Finance Rules Management And Classification Feedback
The Finance UI SHALL let the active tenant manage deterministic classification rules, optionally create a rule after manual category assignment, and explicitly classify an inclusive local-date range with existing job feedback.

#### Scenario: Operator manages ordered rules
- **WHEN** an authenticated tenant member opens `#/finance/rules`
- **THEN** the page MUST list rules in evaluation order with match type, condition, target category name, optional target tag names, and position
- **AND** it MUST support create, edit, delete, move-up, and move-down controls with recoverable pending and error states
- **AND** successful mutations MUST refetch the ordered rule list
- **AND** create/edit forms MUST provide a required category and optional existing-tag selection, including an empty tag set.

#### Scenario: Category removal is blocked by rules
- **WHEN** category removal receives `category_referenced_by_classification_rules`
- **THEN** the UI MUST explain that the category cannot be removed until its rules are retargeted or deleted
- **AND** it MUST identify or link to the referencing rules without hiding the category.

#### Scenario: Manual assignment offers optional rule creation
- **WHEN** a manual transaction category assignment or category change succeeds
- **THEN** the transaction UI MUST show a compact dismissible `Create rule from this transaction` action attached to that transaction
- **AND** it MUST NOT expand the rule form, move keyboard focus, or scroll automatically
- **AND** the behavior MUST be available in shared transaction lists and the full create/edit transaction editor
- **AND** the transaction assignment MUST stay independent of rule creation.

#### Scenario: Replacement manual-rule offer starts collapsed
- **WHEN** another successful category assignment starts a replacement offer in the same list or editor
- **THEN** the UI MUST replace the previous offer and any draft with a new collapsed offer attached to the newly saved transaction
- **AND** replacement MUST occur even when the saved description, category, and tags equal the previous offer's values.

#### Scenario: Explicit classification defaults to thirty local dates
- **WHEN** the rules page first presents explicit classification controls
- **THEN** it MUST show today and the preceding 29 local calendar dates as the selected inclusive start and end dates
- **AND** it MUST allow a wider range and reject a start date after the end date before submission.

#### Scenario: Local dates become half-open timestamp bounds
- **WHEN** the operator submits the displayed inclusive date range
- **THEN** the UI MUST send local start-of-day for the first date and local start-of-day after the last date as full RFC 3339 timestamps with their correct offsets
- **AND** it MUST use calendar arithmetic so daylight-saving transitions retain the correct boundary offsets.

#### Scenario: Explicit classification is observed
- **WHEN** the API returns a classification `jobId`
- **THEN** the initiating UI MUST treat only that ID's pre-materialization `404` as pending and poll the existing queued, running, succeeded, or failed lifecycle
- **AND** it MUST retain a Finance job-detail link, refresh the ledger after success, and explain on failure that some transactions may already have been classified.

#### Scenario: Classification UI is verified visually
- **WHEN** the classification management and feedback surfaces are ready for review
- **THEN** they MUST follow the canonical Bootstrap Finance shell and responsive behavior without route-local layout styles
- **AND** their common desktop and narrow-screen flows MUST pass the repository's required UI visual-review and manual smoke loops after concrete findings are resolved.

#### Scenario: Tag and description saves preserve the compact offer
- **WHEN** an operator edits and saves tags or description on the transaction with an active collapsed offer
- **THEN** the same compact offer MUST stay available without opening the form
- **AND** successful additions and removals MUST be reflected in the saved transaction used when opening the offer
- **AND** failed saves MUST NOT change the rule's source defaults.

#### Scenario: Opening captures the latest saved classification
- **WHEN** the operator clicks the compact action after saving the category and optional tags
- **THEN** the UI MUST open an editable rule draft initialized with `contains`, the latest saved description, category, and all saved tag IDs
- **AND** the operator MUST be able to change exact/contains matching, condition, category, and tags before explicitly saving
- **AND** unsaved transaction edits MUST NOT become rule defaults
- **AND** opening MUST be disabled while a transaction save is pending.

#### Scenario: Open draft is independent and recoverable
- **WHEN** the operator edits the opened rule draft, ordinary rerenders or tag/description saves occur, or a rule-save request fails
- **THEN** the UI MUST retain the draft's operator-edited values
- **AND** a save failure MUST show a recoverable error without changing the saved transaction
- **AND** a successful rule save MUST submit the displayed category and complete optional tag selection and clear the offer.

#### Scenario: Dismissal and cancellation are optional
- **WHEN** the operator dismisses the compact offer or cancels its rule form
- **THEN** the UI MUST clear the offer without changing the saved transaction
- **AND** later tag-only or description-only saves MUST NOT resurrect that offer
- **AND** a later successful category assignment MUST be able to start a new offer.

#### Scenario: Offer remains scoped to a categorized source transaction
- **WHEN** the source category is successfully cleared, the tenant or detail record changes, or the source transaction leaves the rendered list
- **THEN** the offer and any rule draft MUST be cleared
- **AND** clearing a category MUST NOT start a new offer.

#### Scenario: Tag catalog cannot silently erase selections
- **WHEN** the rule editor cannot resolve selected tag IDs because its catalog is unavailable or an assigned tag is no longer assignable
- **THEN** the UI MUST retain those selections and show recoverable catalog or selection feedback
- **AND** it MUST require a successful refresh or explicit removal of unavailable selections before saving an invalid draft
- **AND** a successfully loaded empty catalog MUST allow a category-only rule.
