## Context

`finance/internal/providers/coordinator.go` currently only generates a run ID and returns an empty result. The provider sync v2 domain already stores both `ProviderID` and `ConnectorID` on `domain.ProviderConnectionRef`, and the provider contracts already separate product-facing provider profiles from technical connectors. That separation matters because PKO is a product provider that syncs through the `enable-banking` connector, while monobank syncs through its own `monobank` connector.

The missing piece for the coordinator is a finance-owned way to resolve the technical connector from `request.Connection.ConnectorID` before fetch orchestration begins. Without that seam, the new coordinator either cannot execute at all or would have to reintroduce legacy provider-specific branching that ignores the existing provider/connector model.

## Goals / Non-Goals

**Goals:**

- Resolve provider sync v2 connectors from `domain.ProviderConnectionRef.ConnectorID` rather than product-provider branching.
- Keep the provider-profile composition explicit so PKO continues to route through `enable-banking` and monobank continues to route through `monobank`.
- Fail early and clearly when a connector is unknown or not configured.
- Keep the coordinator dependency surface small and consumer-owned inside `finance/internal/providers/`.

**Non-Goals:**

- Implement candidate-window expansion.
- Implement existing-window snapshot loading.
- Implement atomic apply persistence, run persistence, or failed-run persistence.
- Replace or remove the legacy `finance/provider_sync.go` flow in this change.

## Decisions

1. Add a coordinator-owned `ConnectorRegistry` interface keyed by `domain.ProviderConnectorID`.

   The coordinator should depend on a small consumer-owned interface such as `Resolve(connectorID domain.ProviderConnectorID) (Connector, error)`. This keeps connector lookup at the same abstraction level as the coordinator and avoids coupling coordinator code to concrete connector constructors or app wiring packages.

   Alternative considered: switch directly on `request.Connection.ProviderID` or `request.Connection.ConnectorID` inside `Coordinate`. This is smaller in the short term, but it duplicates composition rules in orchestration code and makes every new connector a coordinator edit.

2. Resolve by technical connector ID, not by product provider ID.

   `Coordinate` should use `request.Connection.ConnectorID` as the source of truth for fetch selection. This preserves the existing domain distinction where PKO is the product provider and Enable Banking is the technical connector. It also prevents future multi-provider-to-one-connector compositions from leaking back into coordinator branching.

   Alternative considered: resolve from `ProviderID` and translate to a connector later. That would push composition knowledge into coordinator logic and make technical connector reuse harder.

3. Fail before fetch when the connector is missing or unconfigured.

   Unknown connector IDs, empty connector IDs, or connectors absent from registry wiring should return a bounded resolution error before any provider network call. The error can include the connector ID for diagnosability, but it must not include connection secrets or raw provider payload data.

   Alternative considered: silently fall back to legacy provider lookup or infer a connector from `ProviderID`. That hides configuration drift and risks calling the wrong integration.

4. Use a finance-owned static registry implementation for wiring.

   The implementation should be a simple in-memory registry that stores connectors by their declared `ConnectorID()`. Wiring can register the monobank and Enable Banking connectors once and pass the registry into `NewSyncCoordinator`. This keeps the runtime path cheap and avoids inventing a more dynamic plugin mechanism.

   Alternative considered: pass individual connectors as variadic coordinator options and scan them during each call. That works for two connectors, but a dedicated registry gives clearer validation and a better seam for tests.

## Risks / Trade-offs

- Unknown connector IDs will become immediate sync failures instead of silently taking a legacy branch. → Keep the error bounded and cover wiring with focused coordinator/registry tests.
- A second provider lookup abstraction may overlap conceptually with legacy `bankProviders`. → Keep the new registry scoped to provider sync v2 in `finance/internal/providers/` and avoid sharing legacy types.
- PKO depends on composition through Enable Banking, which can be easy to regress. → Test coordinator resolution using a PKO connection ref that carries `ConnectorID: enable-banking`.

## Migration Plan

- Add focused tests for registry resolution and coordinator behavior before implementation.
- Wire the v2 coordinator with a static connector registry containing the currently supported technical connectors.
- Keep the legacy sync path unchanged while the coordinator gains the new resolution seam.
- Rollback is removing the registry dependency and coordinator resolution branch; no data migration is required.

## Open Questions

- None. The remaining coordinator gaps from `docs/finance-provider-sync-coordinate-plan.md` stay intentionally out of scope for this change.
