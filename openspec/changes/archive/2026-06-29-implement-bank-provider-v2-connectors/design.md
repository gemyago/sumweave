## Context

Provider sync v2 already defines the shared connector contract, product-provider profiles, a connector registry, requested-window execution, diff/apply planning, and sync-state journaling. It currently has only one concrete connector implementation: `synthetic`.

The supported real bank integrations already exist elsewhere in the finance module:

- `finance/providers_monobank.go` handles Monobank token linking and transaction fetch.
- `finance/providers_enable_banking.go` handles PKO redirect/SCA linking through Enable Banking and session-based fetch.

That means the module currently has two parallel bank integration seams:

```text
legacy finance service path
  -> BankConnectionProvider
  -> normalize and persist in finance service

provider sync v2 path
  -> Connector
  -> ProviderSyncBatch
  -> diff/apply
```

The v2 seam now needs the supported real connectors so the technical connector IDs already stored on v2 connection refs can resolve to real behavior. Per review, this must be a clean v2 implementation, not an adapter over the legacy root `finance` provider path.

The finance module already has the required secure persistence path for provider credentials: link flows encrypt secrets into `domain.ConnectionSecret.Envelope` through the existing finance connection-secret save path (`Service.encryptAndSaveConnectionSecret(...)` / `SaveConnectionSecret(...)`). The v2 connector work must reuse that path rather than inventing connector-local secret persistence or allowing secret-bearing raw payloads to become plaintext evidence.

## Goals / Non-Goals

**Goals:**

- Add concrete v2 connectors for Monobank and Enable Banking.
- Preserve the current supported linking-method matrix:
  - Monobank: token link + fetch
  - Enable Banking: start link + finish link + fetch
- Keep PKO modeled through `PKOProfile()` as product provider `pko` composed with connector `enable-banking`.
- Keep the new connectors independent from legacy root `finance` provider implementations so the old path can be removed later without affecting v2 connector behavior.
- Keep Monobank tokens and Enable Banking credentials on the existing encrypted connection-secret path and out of plaintext-persisted raw payloads, state, or other connector-owned storage.
- Name the exact Enable Banking auth/fetch branches that v2 implements and make every other branch fail as unsupported instead of silently falling back.
- Return v2 observations and link results without direct ledger persistence from the connectors.

**Non-Goals:**

- Switch every runtime bank workflow to v2 in this change.
- Add new bank providers or multi-bank Enable Banking discovery.
- Change public finance API, OpenAPI enums, or UI workflows.
- Redesign the common sync journal, diff planner, or apply planner.

## Decisions

1. Add provider-specific v2 connectors in the existing bank internal packages.

   The concrete connectors should live next to the provider-specific internals they already depend on:

   - `finance/internal/monobank/connector.go`
   - `finance/internal/enablebanking/connector.go`

   This keeps technical connector behavior near the existing HTTP clients and profiles instead of introducing a new generic bank-adapter package.

2. Keep the v2 connectors fully independent from the legacy provider seam.

   The new connectors must not call or wrap:

   - `finance.NewMonobankProvider`
   - `finance.NewEnableBankingProvider`
   - `finance.MonobankProvider`
   - `finance.EnableBankingProvider`
   - root-package legacy normalization helpers or legacy provider result types

   The v2 connector packages should own link/fetch logic, normalization, and v2 mapping directly. If shared logic is needed, move it into provider-local helpers inside `finance/internal/monobank` or `finance/internal/enablebanking`, where the new connector owns it.

3. Keep the capability matrix explicit on the v2 connectors.

   The new connectors should advertise the currently supported methods through `ConnectorCapabilities`:

   - Monobank:
     - `SupportsTokenLink = true`
     - `SupportsFetch = true`
     - `SupportsStartLink = false`
     - `SupportsFinishLink = false`
   - Enable Banking:
     - `SupportsStartLink = true`
     - `SupportsFinishLink = true`
     - `SupportsFetch = true`
     - `SupportsTokenLink = false`

   Unsupported methods should keep returning bounded unsupported errors rather than silently no-oping.

4. Map provider responses directly into v2 observations.

   The v2 connectors should preserve the current finance bank-normalization semantics as their own implementation:

   - account identity, display fields, currency, IBAN, and masked PAN
   - balance observations derived from current and available balance fields
   - transaction status, amount, currency, description, effective time, fingerprint, and provider-original values
   - raw payload scope and provider object identity

   The only v2-only addition is connector-owned observation metadata such as `Connection` and `CapturedAt`.

5. Inject time for observation timestamps.

   V2 raw payload and balance observations require capture timestamps. The real-bank connectors should accept an optional clock function and default to `time.Now().UTC()`. This keeps tests deterministic and avoids hidden time coupling.

6. Keep provider composition unchanged.

   PKO remains a product provider profile that composes the technical Enable Banking connector. Monobank remains both the product provider and the technical connector. This change should not expose `enable-banking` as a user-facing provider choice or alter `PKOProfile()`.

7. Keep connector registration explicit and finance-owned.

   The new connectors should be usable with the existing static registry and `WithConnectors(...)` executor option. This change may add small constructor coverage or composition helpers if needed, but it should not invent a plugin system or dynamic discovery mechanism.

8. Use low-level provider clients, not legacy provider facades.

   The generated or provider-local HTTP clients under `finance/internal/monobank/client` and `finance/internal/enablebanking/client` are acceptable low-level dependencies because they are transport helpers, not the legacy finance service seam. The new v2 connectors should depend on those clients or on new provider-local helpers they own directly.

9. Keep secret-bearing link material on the existing encrypted finance path.

   Any credential-bearing value handled by the v2 link flows, including a submitted Monobank token or an Enable Banking `secret` returned from session creation, must be handed off only through the existing finance connection-secret encryption path (`Service.encryptAndSaveConnectionSecret(...)` / `SaveConnectionSecret(...)` writing `domain.ConnectionSecret.Envelope`).

   The connector implementation may add only the minimal contract/wiring needed to pass that credential once into the existing encrypted path. It must not:

   - add a second secret store
   - persist plaintext credentials in connector-owned state or sync state
   - include plaintext credentials in `ProviderRawPayloadObservation` evidence or any other persisted raw payload

   If an upstream response body contains a secret-bearing field, the persisted raw-payload form for link evidence must redact or omit that field while preserving non-secret troubleshooting context.

10. Pin the exact Enable Banking branches in scope for v2.

   The v2 Enable Banking connector should implement these named branches only:

   - **Legacy redirect auth branch** when signed requests are not configured:
     - start: `POST /auth` with legacy `{redirectUrl, state}` payload
     - finish: `POST /sessions` with legacy `{state, code, providerReference}` payload
   - **Legacy bearer-secret fetch branch** when signed requests are not configured:
     - fetch: `GET /accounts`, then per-account `GET /accounts/{id}/balances` and `GET /accounts/{id}/transactions` using the decrypted connection secret as bearer auth
   - **Signed official redirect auth branch** when signed requests are configured:
     - start: `POST /auth` with the official payload shape produced by `buildOfficialStartLinkPayload(...)`
     - finish: `POST /sessions` with `{code}`
   - **Signed official session fetch branch** when signed requests are configured:
     - fetch: `GET /sessions/{sessionID}`, then per-account `GET /accounts/{id}/balances` and paged `GET /accounts/{id}/transactions` using the requested-window query shape

   The connector must return bounded unsupported errors for branches outside that matrix, including:

   - token linking for Enable Banking
   - mixed-mode fallback between legacy bearer-secret flow and signed official session flow
   - any fetch attempt that lacks the required inputs for the configured branch (for example, no decrypted secret for legacy fetch or no session/external ID for signed fetch)
   - multi-bank discovery or any other Enable Banking auth/fetch mode not listed above

## Mapping Shape

The connector fetch mapping should produce:

- `domain.ProviderAccountObservation` for each normalized provider account
- `domain.ProviderBalanceObservation` when a current balance is present
- `domain.ProviderTransactionObservation` for each normalized provider transaction
- `domain.ProviderRawPayloadObservation` for every raw payload already captured by the provider integration

The resulting `domain.ProviderSyncBatch` should keep:

- `Connection` from `providers.FetchRequest.Connection`
- `RequestedWindow` from `providers.FetchRequest.RequestedWindow`
- all returned observations in provider-reported order unless a provider-specific invariant requires another stable order

For link flows, any persisted raw-payload evidence must already be secret-safe: credential-bearing fields such as Monobank tokens or Enable Banking session secrets must never appear in plaintext in the persisted payload form.

## Risks / Trade-offs

- Re-implementing the supported real-bank behavior inside the v2 seam is more work than wrapping the legacy providers, but it gives us a clean cutover path where the old root `finance` providers can be removed later.
- Recreating normalization in the connector packages risks drift unless coverage is explicit. Focused connector tests should pin provider-original fields, balance handling, raw-payload scopes, unsupported-method behavior, and representative error shaping.
- Enable Banking has both legacy and signed request paths plus tempting fallback behavior. Pinning the four supported v2 branches above and rejecting the rest keeps scope explicit, but it means tests must cover branch selection, missing-input unsupported errors, and the absence of silent fallback.

## Migration Plan

1. Add focused failing tests for Monobank connector capabilities, unsupported methods, secure token-link handoff into the existing encrypted connection-secret path, and fetch observation mapping.
2. Implement the Monobank v2 connector directly in `finance/internal/monobank` using provider-local code and low-level client dependencies only.
3. Add focused failing tests for Enable Banking connector capabilities, both named auth branches, token rejection, secret-safe finish-link evidence, and both named fetch branches plus unsupported mixed/missing-input branches.
4. Implement the Enable Banking v2 connector directly in `finance/internal/enablebanking` using provider-local code and low-level client dependencies only.
5. Add registry/profile coverage proving `PKOProfile()` still composes `pko` through connector `enable-banking`, the executor resolves that technical connector without product-provider branching, and `enable-banking` is not surfaced as a product provider.

## Open Questions

- None for planning. The existing product/provider composition and supported bank scope are already defined by the finance-management spec and prior finance bank-linking change.
