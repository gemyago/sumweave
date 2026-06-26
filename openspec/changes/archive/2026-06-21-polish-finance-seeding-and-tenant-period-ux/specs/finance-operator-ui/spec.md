## ADDED Requirements

### Requirement: Active Tenant Workspace Context
The Finance area SHALL keep one active tenant workspace context across tenant-scoped finance routes and finance-context deep links.

#### Scenario: Sole joined tenant is selected automatically
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to exactly one finance tenant
- **THEN** the UI MUST automatically use that tenant as the active finance workspace without requiring an extra selection step

#### Scenario: Multiple joined tenants are selected once and reused
- **WHEN** an authenticated operator opens a tenant-scoped finance route and belongs to multiple finance tenants
- **THEN** the UI MUST require one explicit active-tenant selection when no active tenant has been chosen yet
- **AND** after selection, the UI MUST reuse that active tenant across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId` until the operator changes it

#### Scenario: Finance deep links preserve the requested route
- **WHEN** an authenticated operator opens `#/finance/accounts/:accountId` or `#/finance/jobs/:jobId` directly
- **THEN** the UI MUST apply the same active-tenant auto-selection or explicit-selection rules used by other finance routes before loading tenant-specific finance context
- **AND** once the active tenant is resolved, the UI MUST continue on the originally requested deep link instead of redirecting the operator to another finance page

### Requirement: Local Finance Dates And Synchronized Current-Month Controls
The Finance area SHALL present human-readable local dates while keeping the existing reporting request semantics deterministic.

#### Scenario: Finance views render local dates instead of raw ISO strings
- **WHEN** a finance page shows operator-facing dates or timestamps such as reporting periods, invite times, missing-FX diagnostics, connection schedule times, or similar finance metadata
- **THEN** the UI MUST render those values using a standard user-local date or date-time format rather than raw ISO strings
- **AND** the underlying API and persistence semantics MUST remain unchanged

#### Scenario: Current-month mode keeps visible date controls aligned
- **WHEN** the finance dashboard is in `current_month` mode on first load or after the operator reactivates that mode
- **THEN** the visible start and end date controls MUST show the current month's active reporting bounds
- **AND** the visible picker state MUST stay synchronized when the operator switches to previous month, next month, or a custom range
