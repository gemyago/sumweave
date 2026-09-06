## MODIFIED Requirements

### Requirement: Distinct Protected Finance Area
The UI SHALL provide a distinct protected Finance area alongside retained generic agent and administration surfaces.

#### Scenario: Finance navigation is tenant-aware and protected
- **WHEN** an authenticated operator uses the application navigation
- **THEN** the UI MUST provide a top-level Finance entry and protected tenant-aware routes including `#/finance`, `#/finance/tenants`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/connections`, `#/finance/connections/synthetic`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/rules`, `#/finance/imports`, and `#/finance/jobs/:jobId`
- **AND** unauthenticated access to those routes MUST redirect through the existing protected-route behavior
- **AND** all protected `#/finance*` routes, including account detail, transaction create/edit, finance job detail, synthetic setup, and rules routes, MUST render inside a dedicated finance-first shell with shared finance chrome rather than repeating the current in-page finance sub-navigation card on each page.

#### Scenario: Finance navigation stays focused on finance destinations
- **WHEN** finance screens are added to the SPA
- **THEN** they MUST remain visually and navigationally distinct from retained Chat, Providers, and Admin workflows
- **AND** the finance shell MUST expose only supported finance destinations for this slice, mapping the rail to real product routes such as dashboard, transactions, accounts, categories, rules, connections and sync, imports, and tenants
- **AND** unsupported reference items such as `Settings` MUST remain out of scope until backed by real product workflows.

## ADDED Requirements

### Requirement: Finance Rules Management And Classification Feedback
The Finance UI SHALL let the active tenant manage deterministic classification rules, optionally create a rule after manual category assignment, and explicitly classify an inclusive local-date range with existing job feedback.

#### Scenario: Operator manages ordered rules
- **WHEN** an authenticated tenant member opens `#/finance/rules`
- **THEN** the page MUST list rules in evaluation order with match type, condition, target category name, and position
- **AND** it MUST support create, edit, delete, move-up, and move-down controls with recoverable pending and error states
- **AND** successful mutations MUST refetch the ordered rule list.

#### Scenario: Category removal is blocked by rules
- **WHEN** category removal receives `category_referenced_by_classification_rules`
- **THEN** the UI MUST explain that the category cannot be removed until its rules are retargeted or deleted
- **AND** it MUST identify or link to the referencing rules without hiding the category.

#### Scenario: Manual assignment offers optional rule creation
- **WHEN** a manual transaction category assignment or category change succeeds
- **THEN** the transaction UI MUST offer a separate optional rule form prefilled with the current user-visible description and selected category
- **AND** the operator MUST be able to select exact or contains matching and edit the condition before an explicit save
- **AND** canceling or failing rule creation MUST leave the successful category assignment intact.

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
