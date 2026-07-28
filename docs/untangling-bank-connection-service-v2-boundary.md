# Untangling BankConnectionService V2 Boundary

This note captures what the current `finance.BankConnectionService` actually does through provider sync v2, and which older finance-layer pieces it still depends on.

## Short Verdict

The service is v2 in its execution path, but not a pure v2 pass-through.

The actual bank-link operations delegate to `finance/internal/providers.LinkCoordinator`, but the public compatibility surface, config inputs, secret wiring, static profile assembly, and some persistence seams still come from older `finance` package structures that predate the v2 coordinator split.

## What Is Actually V2

The core link flow is v2-backed:

- `NewBankConnectionService` builds `internal/providers.LinkCoordinator`
- `LinkTokenBankConnection` delegates to `LinkCoordinator.LinkToken`
- `StartBankConnectionLink` delegates to `LinkCoordinator.StartRedirectLink`
- `FinishBankConnectionLink` delegates to `LinkCoordinator.FinishRedirectLink`
- `NewBankConnectionConnectorRegistry` composes the real internal Monobank and Enable Banking connectors
- `persistence.ProviderLinkPersistence` is the dedicated adapter used to satisfy the coordinator persistence contracts

This matches the OpenSpec intent that the public service in `finance` should expose the API-facing boundary, while the v2 coordination machinery stays under `finance/internal/providers`.

## Non-V2 Parts Still Used

### 1. Legacy public request and response DTOs

The new service still exposes older public `finance` DTOs from `finance/dto.go`, including:

- `LinkTokenBankConnectionParams`
- `StartBankConnectionLinkParams`
- `FinishBankConnectionLinkParams`
- `GetPendingBankConnectionLinkStartByStateParams`
- `ProviderLinkStart`
- `ProviderRawPayload`

Why this exists:

- The OpenSpec explicitly required the protected HTTP contract to remain unchanged.
- The service therefore preserves the existing public `finance` method signatures and maps them onto v2 coordinator requests and results instead of exposing v2 internal request types directly.

### 2. Public provider string handling and early config validation

The new service still reuses some older provider-link vocabulary from `finance/provider_sync.go`, including:

- `bankLinkMethod`
- `bankLinkMethodToken`
- `bankLinkMethodRedirect`
- `bankProviderMonobank`
- `bankProviderPKO`
- `bankConnectorEnableBanking`
- `configuredBankProviderName`
- `bankProviderNotConfiguredForBankError`
- `ErrBankProviderNotConfigured`
- `ErrUnsupportedBankProvider`
- `ErrUnsupportedBankLinkingMethod`

Why this exists:

- The public API still accepts provider values as plain strings.
- Today those values are effectively the same identifiers as the current `domain.ProviderID` values such as `monobank` and `pko`, so this is not a rich extra bank-alias layer.
- Actual flow support is still enforced by v2 connector capabilities inside `LinkCoordinator`.
- The remaining outer-layer logic is mostly early validation and legacy error shaping for unsupported or unconfigured providers.

### 3. Static provider-profile registry still assembled in the public service

`NewBankConnectionService` still constructs a specific static provider profile registry internally:

- `internalproviders.NewStaticProviderProfileRegistry(...)`
- `internalmonobank.Profile()`
- `internalproviders.PKOProfile()`

Why this exists:

- The v2 coordinator already depends on a `ProviderProfileRegistry` interface, so the seam for a future dynamic registry exists.
- The current wiring still hardcodes the known profile set in the outer `finance` constructor rather than accepting an injected registry.
- This keeps the current implementation simple, but it is one of the clearest remaining places to untangle when provider discovery becomes dynamic.

### 4. Legacy provider config structs reused as public constructor inputs

The new bank-connection constructor path still uses:

- `MonobankProviderConfig`
- `EnableBankingProviderConfig`
- `EnableBankingDefaultBaseURL`

These types live in the old provider implementation files under `finance/`, not under `finance/internal/providers`.

Why this exists:

- `apps/sumweave` cannot import `finance/internal/...` because of Go `internal` boundaries.
- The OpenSpec required package `finance` to provide public constructor inputs for wiring the real v2 connectors.
- Instead of introducing dedicated v2-facing config DTOs, the implementation reused the existing public config structs that were already available in `finance`.

### 5. Legacy connection-secret cipher seam

The service still depends on the older `connectionSecretCipher` abstraction and wraps it with `newBankConnectionSecretWriter`.

Why this exists:

- `LinkCoordinator` only needs a `ConnectionSecretWriter`.
- Secret encryption policy already existed at the outer `finance` layer.
- The new service bridges that older encryption abstraction into the v2 coordinator rather than moving crypto responsibilities into `finance/internal/providers`.

### 6. Broad legacy `*persistence.Store` usage

The service constructor still accepts `*persistence.Store`, and then uses older store methods and adapters such as:

- `IsTenantMember`
- `SaveConnectionSecret`
- `GetPendingBankConnectionLinkStartByState`
- `ErrPendingBankConnectionLinkStartNotFound`
- `NewSyntheticProviderStateStoreFromStore`

Why this exists:

- The OpenSpec explicitly said not to add new methods to the broad legacy `persistence.Store`.
- The implementation therefore reused the existing store and wrapped it through narrower adapter layers where needed instead of creating a fully separate v2-only storage graph.
- Tenant membership is also still a product-layer concern owned by the public service boundary, not by the v2 coordinator itself.

### 7. Legacy pending-start callback lookup surface

The service still exposes `GetPendingBankConnectionLinkStartByState`.

Why this exists:

- This is not part of the main v2 coordinator command flow.
- It remains because the Enable Banking callback bridge still needs a public lookup method to resolve the browser callback target after redirect completion.
- The OpenSpec explicitly allowed this one narrow extra method to remain on the focused bank-connection service.

### 8. Shared legacy helper/default reuse

The service still reuses:

- `pendingBankConnectionLinkStartTTL`
- `firstNonEmpty`

from `finance/provider_sync.go`.

Why this exists:

- They were already available as finance-level defaults/helpers.
- The new service reused them instead of redefining local copies.
- This is small in scope, but it is still another way the service is not fully isolated from older `finance` bank-link code.

## Why The Service Is Not “Tiny”

The service ended up doing more than direct delegation because it also owns:

- tenant authorization checks
- public API compatibility
- early provider validation and legacy error shaping
- static provider-profile registry assembly
- app-facing constructor/config composition
- secret writer bridging
- callback pending-start lookup

That shape is consistent with the OpenSpec. The spec did not ask for a raw public export of v2 internals. It asked for a focused public service in `finance` that keeps the old HTTP contract stable while delegating the actual linking work to v2 internals.

## Practical Bottom Line

If the expectation was “a very small wrapper around `LinkCoordinator` and almost nothing else”, the current implementation is larger than that expectation.

If the expectation was “a focused `finance` boundary that keeps old API semantics stable while routing actual linking through v2”, the current implementation is broadly aligned, but it still carries several older finance-layer seams to do that job, especially around constructor wiring and a few public compatibility checks.
