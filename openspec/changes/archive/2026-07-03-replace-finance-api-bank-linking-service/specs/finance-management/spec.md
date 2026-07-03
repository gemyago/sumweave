## ADDED Requirements

### Requirement: Finance Bank-Link API Uses Focused V2 Service
The finance module SHALL expose protected bank-linking workflows through a focused public bank-connection service backed by provider sync v2 link coordination rather than through the overloaded root finance service.

#### Scenario: Bank-link API preserves public HTTP contract
- **WHEN** an authenticated tenant member uses the existing finance bank-link HTTP endpoints for Monobank token linking or PKO redirect start/finish
- **THEN** the backend MUST keep the existing `/api/v1/finance/...` route paths, request JSON, response JSON, product provider choices, and callback flow unchanged
- **AND** the API controller MUST call a focused public bank-connection service instead of root `finance.Service` bank-link methods

#### Scenario: Focused bank-connection service coordinates v2 linking
- **WHEN** the focused bank-connection service handles Monobank token linking or PKO redirect start/finish
- **THEN** it MUST enforce tenant membership at the public finance boundary
- **AND** it MUST delegate product-provider resolution, technical connector calls, pending-start persistence, encrypted secret handoff, durable connection writes, and raw evidence persistence to provider sync v2 link coordination
- **AND** it MUST reject unsupported provider or linking-method combinations before storing secrets or calling a connector

#### Scenario: Legacy bank-link path is not retained behind a toggle
- **WHEN** the bank-link API path is migrated to the focused bank-connection service
- **THEN** the system MUST NOT keep a runtime toggle, selector, or fallback path that can route the same API operation back to the old root-service bank-link implementation
- **AND** old root-service bank-link methods and legacy bank-provider wiring MUST be removed from the active API path or deleted when no remaining in-scope caller needs them

#### Scenario: Internal provider coordination stays behind finance package boundary
- **WHEN** the backend application composes finance bank-linking dependencies
- **THEN** `apps/signal-foundry` MUST depend on public package `finance` service construction rather than importing `finance/internal/providers`, `finance/internal/monobank`, or `finance/internal/enablebanking`
- **AND** real v2 connector and provider-profile wiring MUST remain owned inside the finance module
