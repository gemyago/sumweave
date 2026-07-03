## Context

The root `finance.Service` currently mixes tenant management, catalog, ledger, reporting, imports, FX, bank connection linking, and bank sync entrypoints. Recent provider sync v2 work added `finance/internal/providers.LinkCoordinator`, v2 Monobank and Enable Banking connectors, product provider profiles, durable connector metadata, and `ProviderLinkPersistence`, but the API-facing bank-link handlers still call root service methods that execute the legacy provider-linking path.

The app cannot import `finance/internal/providers` directly because of Go `internal` boundaries. The public boundary for API wiring therefore needs to live in package `finance`, while the v2 coordination machinery remains internal.

## Goals / Non-Goals

**Goals:**

- Stop adding bank-linking responsibility to root `finance.Service`.
- Introduce a focused public bank-connection service in package `finance`.
- Route existing finance HTTP bank-link endpoints through that focused service.
- Use v2 `LinkCoordinator` internally for Monobank token linking and PKO redirect start/finish.
- Keep the existing protected `/api/v1/finance/...` HTTP contract unchanged.
- Avoid feature toggles, dual-path selectors, or temporary compatibility modes.

**Non-Goals:**

- No new public API routes, route versions, request fields, response fields, or UI flows.
- No migration of bank sync execution, schedule management, connection listing, or deletion in this slice.
- No public synthetic-provider linking surface.
- No attempt to fully remove root `finance.Service` across unrelated finance domains.
- No new methods on the broad legacy `persistence.Store` contract.

## Decisions

1. Add `finance.BankConnectionService` as the public API-facing bank-link boundary.

   The app should depend on a focused service from package `finance`, not on `finance/internal/providers` and not on the overloaded root service. The new service should expose only the bank-link methods needed by the current protected API endpoints: token link, redirect start, redirect finish, and any pending-start lookup still required by the Enable Banking callback bridge.

   Alternative considered: add a coordinator field to root `finance.Service`. That is simpler mechanically, but it keeps the root service as the place where every finance workflow accumulates.

2. Make the new service delegate to `LinkCoordinator`.

   `BankConnectionService` should enforce tenant membership at its public boundary, map existing public parameter/result types, and delegate supported flows to `LinkCoordinator` for provider resolution, connector calls, pending-start persistence, secret handoff, and raw evidence persistence.

   Unsupported provider or linking-method combinations must be rejected at the public service boundary before any encrypted secret write or connector call. Focused failing tests in this slice should make that ordering explicit instead of leaving it implicit in coordinator internals.

   The coordinator remains in `finance/internal/providers` because it owns v2 provider-profile and connector concepts. The public service owns product/API-oriented authorization and return shape mapping.

3. Compose real v2 connectors inside `finance`.

   Because `apps/signal-foundry` cannot import finance internal packages, package `finance` should provide the constructor/options needed to compose real v2 Monobank and Enable Banking connectors from app configuration. App DI should pass config and stores into package `finance`, not construct internal provider registries itself.

4. Keep HTTP route and model shapes unchanged.

   The controller methods should continue accepting and returning the generated v1 route models. Their dependency should move from the root finance service interface to the new focused service interface for bank-link operations. No OpenAPI regeneration should be needed unless implementation discovers an existing generated interface must be split in a way that changes only Go code.

5. Do not keep the old link path active.

   This repository is early alpha, and the user explicitly prefers a clean replacement. The implementation should remove the protected API handlers and callback bridge from the old root-service link path, then delete any now-unreferenced legacy link-provider plumbing that becomes unreachable inside that bounded scope rather than hiding it behind a toggle.

6. Keep the slice narrow.

   This change should not pull listing, delete, schedule, or sync execution into the new service unless a link endpoint needs one of those operations to preserve current behavior. Those workflows can move in later slices once the new boundary proves itself.

## Risks / Trade-offs

- [Temporary API non-operation during implementation] -> Accept for this early-alpha slice, but final code must pass the repository completion protocol before reporting done.
- [Some legacy tests assume root `finance.Service` link methods] -> Update or remove those tests as part of the cutover so tests describe the new public bank-connection service boundary.
- [Controller dependency split may touch generated mocks] -> Regenerate or update controller mocks using the existing app patterns if signatures change.
- [Constructor wiring may grow inside package `finance`] -> Keep it specific to bank connections and real provider connector composition so it does not become another god service.
- [Pending Enable Banking callback bridge still needs pending-start lookup] -> Include only the minimum public method needed to keep the callback flow working, and keep it on the bank-connection service because it is part of redirect linking.

## Migration Plan

1. Add focused tests for the new public bank-connection service API and constructor composition.
2. Implement the service around `LinkCoordinator`, dedicated link persistence, encrypted secret persistence, and real v2 connector registries.
3. Rewire finance HTTP controller dependencies and app DI to use the new service for bank-link endpoints and callback pending-start lookup.
4. Remove or stop using old root-service bank-link methods and legacy bank provider composition from the protected API handlers and callback bridge.
5. Run the required repository completion checks.

## Open Questions

- None for this proposal.
