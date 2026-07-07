## Why

Finance tenants can currently be created and archived, but operators cannot correct a tenant name or display currency after creation. Tenant display currency is also accepted as free text in the API and UI, which allows unsupported or typo-prone currency values into a core reporting field.

## What Changes

- Add a tenant update workflow for changing tenant name and display currency for joined tenant members.
- Constrain tenant display-currency values on tenant create and update to a predefined list of valid currency codes.
- Replace the free-text tenant display-currency input in tenant create/update UI with a select control backed by the same product-supported currency list.
- Keep existing tenant membership, invite, archive, account, transaction, and reporting semantics unchanged except that future display-currency reporting uses the updated tenant display currency.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Tenant management gains update behavior and tenant display-currency validation.
- `finance-operator-ui`: Tenant management UI gains tenant update controls and bounded currency selectors for create/update.

## Impact

- Affects `finance/`, especially tenant service contracts, validation, and tests.
- Affects `apps/signal-foundry/internal/api/http/v1routes.yaml`, generated v1 route bindings/models, finance controllers, and controller tests.
- Affects `apps/signal-ui/src/lib/finance/`, `apps/signal-ui/src/pages/FinanceTenants.svelte`, and related UI tests/docs.
- No database migration is expected because tenant name, display currency, and timestamps already exist on persisted tenant records.
