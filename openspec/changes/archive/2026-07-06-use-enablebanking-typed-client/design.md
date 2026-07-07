## Context

`finance/internal/enablebanking/client` already exposes typed methods for the relevant Enable Banking calls, including auth creation, session creation/fetch, accounts, balances, and transactions. The v2 connector constructs that client, but its `apiClient` dependency exposes only `DoRawObject`, so connector code still builds paths, request maps, query strings, and response extraction locally.

That leaves two mapping layers in the same package: the generated typed client maps Enable Banking JSON into typed models, while the connector maps the same provider response again through raw maps. The connector should only own the finance-domain mapping from typed Enable Banking responses into `domain.ProviderSyncBatch` and link results.

## Goals / Non-Goals

**Goals:**

- Make `Connector` consume typed client operations instead of `DoRawObject`.
- Remove connector access to raw response maps and generated raw transport helpers.
- Keep tests focused on connector behavior and generated client method selection.
- Treat generated typed client models as the source of truth; connector behavior must use fields that exist on those models.

**Non-Goals:**

- No public HTTP/API/UI contract changes.
- No provider sync v2 registry, profile, executor, or persistence redesign.
- No new Enable Banking product provider or ASPSP discovery workflow.
- No new generated client methods or connector-only response fields invented by this change.

## Decisions

1. Replace the connector's raw `apiClient` seam with a typed Enable Banking client seam.

   The connector dependency should include only generated methods that already exist and are actually used by the supported workflow, such as `CreateAuth`, `CreateSession`, `GetSession`, `GetAccountBalances`, and `GetAccountTransactions`. Tests can then mock those typed methods and prove the connector does not build raw paths itself.

   Alternative considered: keep `DoRawObject` and only simplify helper functions. That preserves the duplicate mapping problem and keeps the generated client as a thin raw transport wrapper from the connector's perspective.

2. Forbid connector raw response access.

   The connector must not read `Raw` maps, call `DoRawObject`, or use map-probing helpers for Enable Banking provider fields. Generated typed models are the contract. If implementation finds that a value is not available on the generated typed model, the connector must stop relying on that value rather than bypassing the generated client.

   Alternative considered: keep `Raw` on typed responses for evidence handling. That still leaves an escape hatch for connector-owned provider parsing, which is the behavior this proposal is trying to remove.

3. Keep generated client shape authoritative.

   This change must not invent methods or fields just because the connector currently probes them from raw JSON. The existing generated typed client version defines what data is valid for the connector to consume. The connector can still perform finance-specific normalization such as currency casing, amount minor conversion, status mapping, and provider fingerprint fallback from typed values.

## Risks / Trade-offs

- [Typed models may not expose every field the connector currently probes] -> Treat that as evidence the connector is currently using unsupported data and remove the dependency instead of adding raw fallback behavior.
- [Existing raw-payload observation expectations may conflict with typed-only connector behavior] -> Resolve the conflict by keeping raw access out of the connector; any provider evidence contract must be owned by the generated client or a separate typed evidence boundary, not connector map parsing.
- [Tests could overfit method call order] -> Assert the typed operations required for each workflow and the resulting finance observations, while avoiding unrelated ordering assertions.
- [Connector normalization helpers may currently accept maps] -> Replace those helpers with typed-input normalization or remove them when the typed client already returns normalized values.

## Migration Plan

1. Add focused failing tests around the Enable Banking connector proving typed client methods are used for start, finish, and fetch workflows.
2. Remove connector dependencies on response values that are not exposed by the existing generated typed client models.
3. Refactor the connector to call typed client methods and map typed responses into existing provider sync v2 outputs.
4. Remove connector-local raw request/path helpers that become unused.
5. Run the finance module and repository completion checks required for code changes.

## Open Questions

- None for this proposal.
