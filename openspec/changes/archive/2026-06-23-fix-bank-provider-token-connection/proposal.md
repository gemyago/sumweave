## Why

The Finance connections screen currently presents token linking as a generic free-text provider flow, which implies unsupported combinations and conflicts with the intended provider model. We need the operator workflow to match the product-supported bank integrations: monobank via token linking and PKO through Enable Banking redirect/SCA.

## What Changes

- Replace free-text bank provider entry in the Finance connections UI with explicit supported provider choices and provider-specific linking steps.
- Keep monobank as a token-based link flow and ensure token submission can only target `monobank`.
- Add the missing Enable Banking start/return HTTP and UI flow needed to link PKO through redirect/SCA.
- Ensure both supported providers can sync after linking, with sanitized errors, encrypted stored secrets, raw/provider-original retention, and existing job-backed sync behavior preserved.
- Do not introduce a hard local trusted-SSL requirement for the Enable Banking/PKO development and test path.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Make the bank-connection API contract explicitly support monobank token linking and Enable Banking redirect/SCA linking for PKO, while rejecting unsupported provider/method combinations.
- `finance-operator-ui`: Make the Finance connections workflow expose only supported provider-specific linking paths and remove generic free-text bank provider entry.

## Impact

- Affects `finance/` provider-linking validation and provider tests where provider/method support needs to be explicit.
- Affects `apps/signal-foundry/` finance OpenAPI routes, generated route models, controller methods, DI/config wiring, and controller tests for Enable Banking start/finish flows.
- Affects `apps/signal-ui/` finance API client, `FinanceConnections.svelte`, page tests, and UI docs/wireframe for provider-specific linking.
- Uses existing monobank and Enable Banking provider implementations instead of adding a new banking provider framework.
