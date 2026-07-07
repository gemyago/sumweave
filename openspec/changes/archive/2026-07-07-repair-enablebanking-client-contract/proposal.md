## Why

The generated Enable Banking client is currently typed in name but still behaves like a raw JSON adapter. It decodes responses into maps, guesses alternate field names, exposes `Raw` maps on response models, and contains a transport path separate from the repo's standard JSON request style.

That breaks the intent of the recent typed-client connector work: the connector can call typed methods, but those methods still do not reliably match the official Enable Banking API reference. Concrete drift includes ASPSP list shape, session account shape, balance fields, transaction query names, transaction amount fields, and the undocumented top-level account-list method.

The app layer also wires finance with `http.DefaultClient` instead of an app-created HTTP client, so provider calls can bypass the configured timeout, logging, correlation, and telemetry transport.

## What Changes

- Rebuild the Enable Banking client contract around the currently documented API surface used by finance: `GET /aspsps`, `POST /auth`, `POST /sessions`, `GET /sessions/{session_id}`, account details, balances, and transactions.
- Replace raw-map response extraction with strict typed request and response models that match the official schema field names.
- Remove `Raw` map fields from generated response structures; keep raw/evidence handling separate from schema models when provider payload evidence is still required.
- Refactor request sending to the standard typed JSON request shape used by the repo and Firecrawl, while preserving Enable Banking JWT authorization headers.
- Make the backend app compose finance with a configured HTTP client from the app HTTP client factory rather than `http.DefaultClient`.
- Update Enable Banking connector mapping only where required by the corrected client shapes, especially session account IDs, account detail enrichment, balances, transaction IDs, amount fields, and continuation handling.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `finance-management`: require the Enable Banking provider connector and typed client to match the official Enable Banking API contract and app HTTP wiring.

## Impact

- Affected code: `finance/internal/enablebanking/client`, `finance/internal/enablebanking`, `finance/finance.go`, `finance/finance_cfg.go`, `apps/signal-foundry/internal/financeapp`, and focused tests.
- Affected behavior: PKO through Enable Banking should use schema-faithful typed responses and the app-configured HTTP client for redirect start, redirect finish, and sync fetch.
- Out of scope: new bank providers, payment initiation, UI changes, public finance API changes, database schema changes, and broad HTTP infrastructure redesign beyond what is needed to wire the existing configured client.
