# Implementation Status

Sumweave contains an implemented finance foundation: tenancy, catalog,
ledger, transfers, imports, bank connections, provider snapshots, reporting,
balances, FX, and finance durable jobs. The Go app supplies auth, migrations,
dispatch, jobs, admin diagnostics, and HTTP delivery. The UI supplies finance,
admin, Chat, and provider configuration routes.

Phase 1 external integration access is implemented through self-service
personal access tokens, the explicit existing-route allowlist, caller-scoped
observed jobs, retry-safe finance triggers, and the separate `swmd` HTTP client.
See [External integrations](integration-layer/README.md) and the
[integration-layer E2E guide](manual-e2e/integration-layer-e2e.md).

Generic agent infrastructure remains intentionally, while the product surface
is limited to finance and its supporting application services.
