## Why

Finance bank-linking now has a v2 `LinkCoordinator`, real v2 connectors, and dedicated link persistence, but the protected HTTP API still reaches those workflows through the overloaded root `finance.Service` and its legacy provider-linking path. Moving bank-linking onto a focused public service lets new finance work grow beside the legacy service instead of adding more responsibilities to it.

## What Changes

- Add a focused public finance bank-connection service for API-facing bank-link workflows.
- Route the existing protected finance bank-link HTTP handlers to that service instead of the root `finance.Service`.
- Use provider sync v2 link coordination internally for Monobank token linking and PKO redirect start/finish.
- Reject unsupported provider or linking-method combinations before any secret write or connector call.
- Keep existing `/api/v1/finance/...` routes, request shapes, response shapes, provider enums, and UI callback flow unchanged.
- Remove the old service-level bank-link implementation from the active API path rather than adding a runtime toggle or compatibility selector.
- Keep the root `finance.Service` out of new bank-linking work; this slice only commits to migrating the protected API handlers and callback bridge, then deleting any link-specific legacy helpers that become unreferenced as a direct result.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Require protected finance bank-link API flows to use a focused public bank-connection service backed by provider sync v2 link coordination, without changing the public HTTP contract.

## Impact

- Affected code: `finance/` public service boundary, `finance/internal/providers`, `finance/persistence`, `apps/signal-foundry/internal/financeapp`, and finance HTTP controller wiring/tests.
- Affected API: no route, request, response, or OpenAPI shape change expected.
- Affected behavior: Monobank token linking and PKO redirect linking should be v2-backed from the protected API path; unsupported provider/method combinations must fail before storing secrets or calling connectors.
- Out of scope: migrating bank sync execution, schedules, connection listing/deletion, UI changes, synthetic provider public API, and a broader decomposition of tenant/catalog/ledger/reporting services.
