## Context

The finance module currently has two bank integration paths. The legacy root service path owns link lifecycle and persistence through `StartBankConnectionLink`, `FinishBankConnectionLink`, `LinkTokenBankConnection`, pending link-start rows, encrypted connection secrets, and `BankConnection` rows. The provider sync v2 path owns product provider profiles, technical connector registries, real v2 connectors, and window sync execution.

The gap is between those paths. V2 connectors can start, finish, token-link, and fetch, but no v2 component coordinates link requests into durable connection records. The durable `BankConnection` model also stores only `Provider`, while v2 sync needs both product `ProviderID` and technical `ConnectorID` in `ProviderConnectionRef`.

## Goals / Non-Goals

**Goals:**

- Add a v2 link coordination layer that owns link start, finish, and token-link workflows.
- Resolve product provider profiles to technical connectors before invoking connector link methods.
- Persist enough pending-start data to finish redirect links through the same connector path.
- Persist `ConnectorID` on durable bank connections and pending starts.
- Keep credentials encrypted through the existing connection-secret path.
- Keep raw link evidence secret-safe and useful for troubleshooting.
- Keep public finance API and UI shapes unchanged for this slice.
- Keep persistence additions as dedicated link-focused stores/adapters, not more methods on the legacy broad store contract.

**Non-Goals:**

- No new bank providers or Enable Banking institution discovery.
- No public synthetic-provider linking surface.
- No removal of legacy root provider implementations in this change.
- No redesign of provider sync window execution, diff/apply, or sync-state journaling.
- No backwards-compatibility migration work beyond what finance-owned auto-migration needs in this early-alpha repo.

## Decisions

1. Name the new component `LinkCoordinator`.

   The component coordinates a workflow rather than acting as a provider connector or a generic repository. It should live in `finance/internal/providers` beside provider sync v2 contracts and registries because it consumes `ProviderProfile`, connector capabilities, `StartLinkResult`, and `LinkResult`.

   Alternative considered: keep this logic in `finance.Service`. That preserves the current shape, but it keeps product-provider resolution, connector resolution, secret handling, and connection persistence tangled with legacy provider types.

2. Resolve product providers through `ProviderProfileRegistry`, then resolve technical connectors through `ConnectorRegistry`.

   Link requests should accept the product provider ID the user selected. The coordinator resolves the profile, checks the connector's advertised capabilities, and calls the connector method that matches the requested link mode.

   This keeps PKO modeled as product provider `pko` composed through connector `enable-banking`, and keeps `enable-banking` from becoming a user-facing bank provider.

3. Persist connector identity explicitly.

   `domain.BankConnection` should gain `ConnectorID domain.ProviderConnectorID` or an equivalent durable field. The persistence model should add a `connector_id` column. Pending redirect starts should also persist the connector ID selected at start time.

   Deriving connector ID from provider ID forever would duplicate profile resolution rules in later sync and reauth paths. Since this repo is early alpha, carrying the durable field now is simpler and more honest.

4. Persist the complete secret-safe start result for redirect flows.

   `providers.StartLinkResult` contains `State`, `AuthorizationURL`, and raw payload observations. Enable Banking legacy finish derives provider reference from the stored start raw payload, so pending-start persistence must retain the connector-safe start result, not only state and URL.

   Store this as a small JSON envelope or equivalent dedicated fields on a pending link-start record scoped by tenant, actor, product provider, connector, state, expiration, and consumed time. The persisted form must contain only already-redacted payload data from the connector. These start-result observations exist only inside pending-start storage for later finish/retry; successful finish must not copy them into durable connection-linked raw payload rows.

5. Keep credential persistence outside connectors.

   Connectors may return `LinkResult.Secret` once. The coordinator owns handing it to the existing encrypted connection-secret writer and storing only the returned `SecretID` on the bank connection. Connectors and raw payload persistence must not store plaintext Monobank tokens, Enable Banking session secrets, bearer tokens, private keys, or signed request material.

6. Use dedicated link persistence interfaces.

   Define consumer-owned interfaces near `LinkCoordinator`, such as pending-start store, connection writer, secret writer, and raw-payload writer. Implement them with dedicated persistence adapters in `finance/persistence` rather than widening the old root `persistence.Store` contract.

   Existing `SaveBankConnection`, `SaveConnectionSecret`, and `SaveRawPayload` behavior can be reused internally by adapters if the semantics match, but the coordinator should depend only on the narrow link-focused interfaces.

7. Keep service access checks at the finance service boundary.

   The root `finance.Service` should continue to enforce tenant membership before calling the coordinator. The coordinator can still persist tenant and actor identifiers on pending starts, but it should not become the tenant authorization service.

8. Make retry behavior explicit for redirect finish.

   Finishing a redirect link should atomically consume a matching unexpired pending start before calling the connector. If connector finish or encrypted persistence fails after consumption, the coordinator should restore the pending start or otherwise leave it retryable until expiration. Once final connection persistence succeeds, the same state must not create another connection.

9. Preserve current PKO re-link behavior during service cutover.

   Routing `finance.Service` link methods through `LinkCoordinator` must keep the current PKO service behavior: when a tenant already has a durable PKO connection, a later successful PKO redirect re-link updates and returns that existing tenant PKO connection instead of creating a second PKO connection.

   This keeps the cutover narrow and avoids introducing new product policy for PKO replacement in the same change.

10. Durable raw-payload evidence comes from final link completion only.

   When token link or redirect finish succeeds, the coordinator should persist durable connection-linked raw payload evidence only from the connector's final `LinkResult.RawPayloads` (or equivalent final connector output). Pending-start start-result observations are for finish/retry continuity only and must not be duplicated into durable raw payload evidence on success.

11. Build v2 connection references from durable connection records.

   Add a small mapper that converts a persisted `BankConnection` into `domain.ProviderConnectionRef` using connection ID, product provider, connector ID, provider reference, and external ID. Sync v2 should consume this mapper instead of branching on product providers.

## Risks / Trade-offs

- Persisting `ConnectorID` changes the finance-owned schema. -> Accept because the module uses GORM auto-migrate and the project is early alpha.
- Storing the whole start result creates another JSON envelope. -> Keep the envelope narrow, versioned if useful, and limited to connector-redacted payload data.
- The coordinator overlaps with legacy service link logic during migration. -> Route service methods through the coordinator in one focused step, then leave legacy provider removal for a later change.
- Secret redaction depends on connector behavior. -> Add coordinator tests that assert persisted raw payloads and pending-start envelopes do not contain returned credentials.

## Migration Plan

1. Add failing tests for connection and pending-start persistence of connector identity and v2 start-result data.
2. Add the domain and persistence model fields/adapters needed for v2 link coordination.
3. Add failing coordinator tests for Monobank token link, PKO redirect start/finish, unsupported methods, retry behavior, and secret-safe persistence.
4. Implement `LinkCoordinator` with profile resolution, connector resolution, capability checks, encrypted secret handoff, connection persistence, and raw payload persistence.
5. Route existing finance service link methods through the new coordinator while preserving existing public parameter and result shapes.
6. Update docs/spec wording where needed so linked connection identity, PKO re-link behavior, and v2 sync resolution tell the same story.
