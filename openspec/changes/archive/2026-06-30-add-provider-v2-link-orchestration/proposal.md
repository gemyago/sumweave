## Why

Provider sync v2 now has real connector implementations and window execution, but bank-link lifecycle still lives in the legacy root finance service path. That leaves v2 without a single owner for resolving product providers, invoking connector link methods, encrypting returned credentials, and saving the durable connection metadata needed by later sync.

## What Changes

- Add a provider sync v2 link coordination layer for bank connection start, finish, and token-link workflows.
- Resolve product provider profiles before link operations so PKO remains product provider `pko` while using technical connector `enable-banking`.
- Persist pending redirect starts with enough connector-safe data to finish the link later, including the technical connector identity and the connector's secret-safe start result.
- Persist linked bank connections with explicit product provider and technical connector identity so sync v2 can build `ProviderConnectionRef` without provider-specific branching.
- Keep credential persistence on the existing encrypted connection-secret path and keep persisted raw payload evidence secret-safe.
- Route existing finance service linking methods through the new v2 link coordination layer when configured, while preserving the current public API and UI workflow shape.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: add provider sync v2 link coordination requirements for durable connection identity, pending redirect starts, encrypted secrets, and connector-safe link evidence.

## Impact

- Affected code: `finance/domain`, `finance/internal/providers`, `finance/persistence`, provider connector packages, root finance service link methods, and focused finance tests.
- Affected persistence: finance-owned bank connection and pending link-start models need explicit technical connector metadata and persisted v2 start-result data.
- Affected behavior: supported Monobank token and PKO redirect linking should be coordinated through v2 connectors without changing the user-facing provider choices.
- Out of scope: new public API or UI workflows, new bank providers, arbitrary Enable Banking discovery, and removing the legacy provider implementations in the same change.
