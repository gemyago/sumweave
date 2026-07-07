## MODIFIED Requirements

### Requirement: Tenant, Account, Category, And Tag Management
The finance module SHALL support tenant-based personal-finance ownership, tenant profile updates, and tenant-local finance catalogs.

#### Scenario: Users create and join finance tenants
- **WHEN** an authenticated user creates a tenant or accepts an invite
- **THEN** the system MUST create or join a finance tenant with a user-friendly name and one display currency selected from the predefined valid tenant currency-code list
- **AND** the stored display currency MUST use the canonical uppercase code
- **AND** all tenant members MUST be equal in the first implementation

#### Scenario: Tenant members update finance tenants
- **WHEN** an authenticated tenant member updates a finance tenant
- **THEN** the system MUST allow updating the tenant name and display currency
- **AND** the replacement display currency MUST be selected from the predefined valid tenant currency-code list
- **AND** the stored display currency MUST use the canonical uppercase code
- **AND** the tenant `UpdatedAt` timestamp MUST advance while membership, invites, accounts, transactions, categories, tags, and archived state remain governed by their existing workflows

#### Scenario: Unsupported tenant display currencies are rejected
- **WHEN** a caller creates or updates a finance tenant with an empty, unknown, or unsupported display-currency code
- **THEN** the system MUST reject the request before persisting the tenant change
- **AND** arbitrary free-text values MUST NOT become tenant display currencies

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

#### Scenario: Tenant display currency updates affect reporting display currency only
- **WHEN** a tenant member changes the tenant display currency
- **THEN** later tenant display-currency reporting MUST use the updated display currency through the existing persisted FX-rate behavior
- **AND** the system MUST NOT mutate existing account, transaction, provider-original, or CSV-import native currency values as part of the tenant update
