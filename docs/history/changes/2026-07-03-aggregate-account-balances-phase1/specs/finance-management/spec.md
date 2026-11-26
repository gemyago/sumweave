---
type: Historical record
title: "2026-07-03-aggregate-account-balances-phase1: spec"
description: "Historical spec record for 2026-07-03-aggregate-account-balances-phase1."
status: deprecated
superseded_by: /requirements/finance-management.md
generated: { by: "codex/gpt-6.1-sol", at: "2026-11-26T08:20:34+01:00" }
sources:
  - id: original-record
    resource: "https://github.com/gemyago/sumweave/blob/3694d916ce230a3e3e0f6b98cb014b04abef5eec/openspec/changes/archive/2026-07-03-aggregate-account-balances-phase1/specs/finance-management/spec.md"
    title: "Original openspec/changes/archive/2026-07-03-aggregate-account-balances-phase1/specs/finance-management/spec.md"
---

> Historical source, retained for context. This record is deprecated and is
> not a current plan or workflow. Old commands, paths, task states, and reported
> test outcomes describe the original work; they were not re-executed by this
> documentation migration. Use [current requirements](../../../../../requirements/index.md) and their topic
> references for accepted contracts and delivered behavior.[^original-record]

## MODIFIED Requirements

### Requirement: Tenant, Account, Category, And Tag Management
The finance module SHALL support tenant-based personal-finance ownership and tenant-local finance catalogs.

#### Scenario: Users create and join finance tenants
- **WHEN** an authenticated user creates a tenant or accepts an invite
- **THEN** the system MUST create or join a finance tenant with a user-friendly name and one display currency
- **AND** all tenant members MUST be equal in the first implementation

#### Scenario: New tenants receive tenant-local default categories and tags
- **WHEN** a finance tenant is created
- **THEN** the system MUST copy system default categories and default tags into that tenant
- **AND** the seeded category baseline MUST stay flat and cover common household finance needs across income, housing, utilities, food, transportation, health, insurance, education or childcare, pets, personal care, entertainment, shopping, home, travel, gifts or donations, taxes or fees, debt payments, and miscellaneous spending
- **AND** the seeded tag baseline MUST cover cross-category reporting uses such as tax, reimbursements, split or shared spending, business use, subscriptions, and travel
- **AND** transfer, reconciliation, and opening-balance semantics MUST remain explicit system transaction behavior rather than seeded user categories
- **AND** later changes to system defaults MUST NOT mutate existing tenant-local categories or tags

#### Scenario: Accounts remain tenant-owned and attachable
- **WHEN** a tenant member creates or links accounts
- **THEN** every finance account MUST belong to exactly one tenant
- **AND** the system MUST support manual, linked-bank, imported, and reconciliation-style account shapes
- **AND** bank-linking flows MUST be able to attach a linked provider account to an existing manual account instead of always creating a duplicate account

#### Scenario: Account lists include ledger-derived balances
- **WHEN** an authenticated tenant member lists finance accounts
- **THEN** each returned account MUST include booked and pending balances derived from visible transactions for that account
- **AND** the balances MUST be computed by an aggregate read path that does not require loading full transaction history into application memory for account-list balance calculation
- **AND** the aggregate read MUST preserve existing ledger semantics by excluding hidden transactions, separating booked from pending transactions, and applying every visible transaction kind by signed `amountMinor`

#### Scenario: Account detail includes ledger-derived balances
- **WHEN** an authenticated tenant member loads one finance account by identifier
- **THEN** the returned account MUST include booked and pending balances derived from visible transactions for that account
- **AND** the balances MUST be computed by the same aggregate read path used by account-list and standalone account-balance reads
- **AND** the read MUST reject account identifiers that do not belong to the selected tenant

### Requirement: Protected Finance HTTP API
The backend application SHALL expose finance APIs through the existing app under `/api/v1/finance/...`.

#### Scenario: Finance APIs are authenticated and tenant-aware
- **WHEN** a caller without a valid authenticated identity calls finance endpoints
- **THEN** the system MUST reject the request as unauthorized
- **AND** when an authenticated caller accesses finance endpoints, tenant-scoped reads and writes MUST remain isolated to the caller's joined tenants

#### Scenario: Finance transaction detail API preserves tenant scope and editor context
- **WHEN** an authenticated tenant member calls `GET /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`
- **THEN** the system MUST return the selected transaction in camelCase JSON with `providerOriginal` included when present
- **AND** the read flow MUST reject transaction identifiers that do not belong to the selected tenant

#### Scenario: Finance transaction update API preserves tenant scope and reporting semantics
- **WHEN** an authenticated tenant member calls `PATCH /api/v1/finance/tenants/{tenantId}/transactions/{transactionId}`
- **THEN** the system MUST update only the transaction's user-editable reporting fields `description`, `amountMinor`, `effectiveAt`, and nullable `categoryId`
- **AND** the response MUST return the updated transaction in camelCase JSON with `providerOriginal` included when present
- **AND** the update flow MUST reject transaction or category identifiers that do not belong to the selected tenant

#### Scenario: Finance account list API returns balances
- **WHEN** an authenticated tenant member calls `GET /api/v1/finance/tenants/{tenantId}/accounts`
- **THEN** the system MUST return each account in camelCase JSON with ledger-derived `bookedBalanceMinor` and `pendingBalanceMinor` fields
- **AND** the read flow MUST preserve tenant isolation and existing account fields

#### Scenario: Finance account detail API returns balances
- **WHEN** an authenticated tenant member calls `GET /api/v1/finance/tenants/{tenantId}/accounts/{accountId}`
- **THEN** the system MUST return the selected account in camelCase JSON with ledger-derived `bookedBalanceMinor` and `pendingBalanceMinor` fields
- **AND** the read flow MUST reject account identifiers that do not belong to the selected tenant

#### Scenario: Finance API covers required product areas
- **WHEN** the first finance slice is implemented
- **THEN** the API surface MUST cover tenants, tenant members/invites, accounts, bank connections, transactions including list/detail/create/update flows, categories, tags, dashboard/reporting, FX diagnostics and sync, CSV import preview/confirm/status, and finance-related job deep-linking
- **AND** all operator-facing JSON fields MUST use camelCase

[^original-record]: Original historical source snapshot; migration metadata describes this retained copy.
