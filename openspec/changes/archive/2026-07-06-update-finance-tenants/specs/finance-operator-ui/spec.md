## ADDED Requirements

### Requirement: Tenant Management Supports Updates And Bounded Currency Selection
The Finance tenants route SHALL let operators create and update tenants using predefined valid display-currency choices instead of free-text currency fields.

#### Scenario: Tenant create uses a supported currency selector
- **WHEN** an authenticated operator opens `#/finance/tenants` to create a finance tenant
- **THEN** the create form MUST present display currency as a select control populated from the predefined valid tenant currency-code list
- **AND** the form MUST submit the selected currency code rather than arbitrary free text

#### Scenario: Selected tenant can be updated
- **WHEN** an authenticated tenant member has selected a tenant on `#/finance/tenants`
- **THEN** the UI MUST provide an edit form for the selected tenant name and display currency
- **AND** the display-currency control MUST use the same predefined valid tenant currency-code list as tenant creation
- **AND** saving the form MUST call the tenant update API and refresh the visible selected tenant state after success

#### Scenario: Tenant update failures are recoverable
- **WHEN** tenant update fails because validation, authentication, authorization, or network handling rejects the request
- **THEN** the UI MUST keep the operator on `#/finance/tenants`
- **AND** it MUST show a recoverable error state without losing the current selected tenant context
