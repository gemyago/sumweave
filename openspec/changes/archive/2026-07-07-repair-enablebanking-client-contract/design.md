## Context

The previous `use-enablebanking-typed-client` change moved `finance/internal/enablebanking.Connector` away from connector-local raw HTTP calls. That was the right upper-layer direction, but it assumed `finance/internal/enablebanking/client` was already a faithful typed client.

The current client is not faithful enough:

- `ListASPSPs` expects a top-level JSON array, while the official response is an object with an `aspsps` array.
- `GetSession` reads account objects from `accounts`, while the official response uses `accounts` as ID strings and `accounts_data` for account metadata.
- `CreateSession` carries fields like `state` and `providerReference` in the request model, while the official request body only needs the authorization `code` for this flow.
- Account balance decoding reads `type`, `currentBalanceMinor`, and `availableBalanceMinor`, while the official balance resource uses `balance_type` and `balance_amount`.
- Transaction requests send `status`, while the official query parameter is `transaction_status`.
- Transaction decoding reads `amount`, `id`, `description`, and `remittance_information_unstructured`, while the official transaction schema uses `transaction_amount`, `entry_reference`, `transaction_id`, `note`, and `remittance_information`.
- Generated models expose `Raw map[string]any` or raw slices, so schema models double as raw-provider evidence containers.
- `client.go` implements its own `DoRawObject` and `DoRawArray` path instead of the repo's generic typed JSON send style.
- `apps/signal-foundry/internal/financeapp/register.go` passes `http.DefaultClient` into finance instead of an app-created client from `httpclient.ClientFactory`.

The official API reference checked on July 7, 2026 documents the relevant endpoints and schemas at `https://enablebanking.com/docs/api/reference/`.

## Goals

- Make the Enable Banking client schema-faithful for the AIS endpoints used by finance.
- Keep generated request/response models typed and free of generic raw map fields.
- Preserve provider raw-payload observations through a separate evidence mechanism where finance still needs them.
- Use the app-wired HTTP client instance for finance provider calls.
- Keep finance independent from `runtime/` and from app `internal` package imports.
- Keep the connector observation-only and preserve provider sync v2 public behavior.

## Non-Goals

- Do not add payment endpoints.
- Do not add provider discovery UI or change configured PKO provider selection.
- Do not change public finance HTTP API responses.
- Do not introduce backward-compatibility shims for the current incorrect generated shapes.
- Do not move finance into `apps/signal-foundry/internal`.

## Decisions

1. Treat official Enable Banking schemas as the generated client source of truth.

   The client should model the current documented names directly: `url`, `authorization_id`, `session_id`, `accounts_data`, `balance_type`, `balance_amount`, `transaction_amount`, `entry_reference`, `transaction_id`, `continuation_key`, and `transaction_status`.

   The implementation should not recover data through guessed aliases such as `authorizationUrl`, `providerReference`, `amount`, `description`, or `status` unless the official docs explicitly define those fields for the endpoint.

2. Remove raw maps from typed schema models.

   Public client structs should describe provider schema fields only. They should not carry `Raw map[string]any`, `Raw []map[string]any`, or similar escape hatches.

   Provider evidence should be handled by a separate internal response evidence boundary. A likely shape is an internal envelope that carries the decoded typed value plus the successful response body bytes. The connector can persist evidence from that envelope without reading arbitrary raw maps for business mapping.

3. Use typed JSON request sending with an Enable Banking authorization hook.

   The client should use a small typed request helper shaped like `apps/signal-foundry/internal/infrastructure/httpclient.SendRequest` and `tools/firecrawl/internal/firecrawl/send_request.go`: generic body, generic target, injected `*http.Client`, standard JSON encoding/decoding, and standard status/transport error behavior.

   Because `finance/` cannot import `apps/signal-foundry/internal/...`, the implementation should either keep a local finance/client helper with the same semantics or extract a truly shared non-internal helper only if that remains small and product-neutral. The helper must allow Enable Banking to set `Accept: application/json` and `Authorization: Bearer <JWT>`.

4. Wire finance from the app HTTP client factory.

   `newFinanceModuleFromDI` should accept the app HTTP client factory and pass a factory-created `*http.Client` into `finance.Config`. Tests should prove the injected transport is used for Enable Banking calls. Direct `http.DefaultClient` use should remain only in narrow standalone constructors or tests where no app DI exists.

5. Align connector mapping with corrected client models.

   The connector should consume typed fields only:

   - start-link uses `StartAuthorizationResponse.URL` and `AuthorizationID`;
   - finish-link stores `AuthorizeSessionResponse.SessionID`;
   - fetch derives account IDs from session `accounts` or `accounts_data`;
   - account names, IBANs, and currency come from session account data or account details;
   - balances use `balance_type` and `balance_amount`;
   - transactions use `entry_reference` first for stable provider identity, then `transaction_id` only as a less-stable details identifier;
   - transaction amount and direction come from `transaction_amount` and `credit_debit_indicator`;
   - continuation uses `continuation_key`.

## Risks / Trade-Offs

- Existing tests currently assert incorrect or permissive shapes. They need to be rewritten around docs-derived fixtures, not patched with compatibility aliases.
- Removing raw maps from models may expose places where provider evidence was conflated with business mapping. That is useful friction; evidence should remain separate.
- Extracting a shared request helper could grow scope. Prefer the smallest implementation that gives Enable Banking the same typed send semantics and app-wired client behavior.
- Official docs can change. Tests should pin the supported contract with fixture names and comments referencing the API reference date.
