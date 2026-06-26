## Why

Provider sync v2 already defines fetch, diff, and apply seams, but the coordinator still cannot choose the correct technical connector for a bank connection. This change is needed now so sync orchestration can start from the connection's stored connector identity instead of hardcoded or legacy provider-specific branching.

## What Changes

- Define provider sync v2 coordinator requirements for resolving a technical connector from the bank connection before fetch begins.
- Add a finance-owned connector registry contract so coordinator code can resolve `monobank`, `enable-banking`, and future technical connectors without direct constructor branching.
- Define failure behavior for unknown or unsupported connector IDs so the sync path fails early, audibly, and without invoking the wrong provider integration.
- Keep the product-provider versus technical-connector split explicit so PKO continues to sync through Enable Banking while monobank continues to sync through its own connector.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `finance-management`: extend provider sync v2 requirements to cover coordinator connector resolution and unsupported-connector handling.

## Impact

- Affected code: `finance/internal/providers/`, finance sync service orchestration, and connector registration wire-up.
- Affected systems: bank connection sync execution for provider sync v2, especially PKO via Enable Banking and monobank direct sync selection.
- Dependencies: existing provider connectors and provider sync v2 domain models will need a coordinator-facing registry contract, but this change does not add new persistence behavior.
