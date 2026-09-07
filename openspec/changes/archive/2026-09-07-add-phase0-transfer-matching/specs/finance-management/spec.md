## MODIFIED Requirements

### Requirement: Ledger-Driven Transaction Semantics
The finance module SHALL treat transactions as the explainable ledger source of truth for balances and reporting.

#### Scenario: Balances are derived from transactions
- **WHEN** a user needs to correct an account balance
- **THEN** the system MUST use visible reconciliation or opening-balance transactions rather than directly mutating a balance field

#### Scenario: Synced transactions preserve provider truth and user edits
- **WHEN** provider-synced transactions are imported and later edited by a user
- **THEN** the system MUST retain provider-original values and current schema-derived provider snapshots separately from user-edited presentation/reporting fields
- **AND** later syncs MUST NOT silently overwrite user corrections

#### Scenario: Reporting semantics stay explicit for special transaction kinds
- **WHEN** transactions are refunds, matched internal transfers, unmatched external transfers, reconciliations, pending items, or hidden/deleted items
- **THEN** refunds MUST reduce expense in the assigned category, matched internal transfers MUST be excluded from income/expense reporting, reconciliations MUST be visible but excluded from income/expense reporting, pending items MUST be visible but excluded from settled totals by default, and hidden/deleted items MUST be excluded from normal user views while retained for audit and sync idempotency

#### Scenario: Transaction edits stay limited to user-controlled reporting fields
- **WHEN** an authenticated tenant member updates an existing transaction
- **THEN** the system MUST allow edits to `description`, `amountMinor`, `effectiveAt`, and category assignment or category removal for that transaction
- **AND** any replacement category MUST belong to the same tenant as the transaction
- **AND** the system MUST preserve account identity, source, status, kind, currency, transfer linkage, automatic-matching exclusion, hidden state, and provider-original values unless another dedicated workflow changes them

#### Scenario: Ordinary saves preserve transfer-owned state
- **WHEN** an existing transaction passes through an ordinary user, CSV, provider-refresh, sync-apply, or shared-upsert save path
- **THEN** the save MUST preserve its stored kind, transfer group, transfer matching timestamp, and automatic-matching exclusion rather than accepting stale incoming pair state
- **AND** any returned saved transaction MUST carry the persisted pair values while preserving categories, tags, and provider data

#### Scenario: Manual transfer correction retains ledger data
- **WHEN** a tenant member unlinks a transfer pair through the existing correction workflow
- **THEN** both rows MUST return to kind `regular`, clear pair metadata, preserve categories and tags, and become excluded from later automatic or explicit matching
- **AND** either excluded row MUST remain available for the existing broader manual linking workflow without an exclusion warning or indicator.
