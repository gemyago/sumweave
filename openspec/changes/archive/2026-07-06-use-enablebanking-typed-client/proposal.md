## Why

The Enable Banking provider sync v2 connector currently bypasses most of its generated typed client and performs raw HTTP calls plus connector-local map extraction for auth, session, account, balance, and transaction behavior. That defeats the purpose of the generated client surface, duplicates request/response mapping, and makes connector tests validate fragile raw JSON plumbing instead of finance connector behavior.

## What Changes

- Refactor `finance/internal/enablebanking.Connector` to depend on a narrow typed client interface built from the generated Enable Banking client methods.
- Use existing generated typed operations for redirect start, redirect finish, session fetch, balance fetch, and paged transaction fetch.
- Remove connector-owned raw path construction, ad hoc request maps, raw response access, and response field probing.
- Treat the generated client as the source of truth for Enable Banking response data; if data is not exposed by the generated typed client, the current connector usage is wrong and must not be replaced with raw-map fallback behavior.
- Keep provider sync v2 behavior unchanged for PKO composition and observation output shapes.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: Require the Enable Banking provider sync v2 connector to use the generated typed client surface rather than connector-local raw HTTP/mapping logic.

## Impact

- Affected code: `finance/internal/enablebanking`, `finance/internal/enablebanking/client`, and focused finance tests.
- Affected behavior: no public API, UI, database, provider-profile, or job behavior change expected.
- Affected risk: lowers duplicate mapping risk in the connector by making generated typed models the only connector input shape.
- Out of scope: new Enable Banking endpoints, provider discovery, UI changes, legacy provider resurrection, and changing the common provider sync v2 connector contract.
